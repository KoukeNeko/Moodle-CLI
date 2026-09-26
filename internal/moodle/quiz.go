package moodle

import (
	"context"
	"html"
	"strconv"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/quiz"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Quiz web service functions. Only reads: starting an attempt starts the clock
// on a timed quiz, and mod_quiz_view_* records a view.
const (
	FunctionQuizzes   = "mod_quiz_get_quizzes_by_courses"
	FunctionBestGrade = "mod_quiz_get_user_best_grade"
	// FunctionQuizAttempts replaced FunctionUserAttempts in Moodle 5.1; 4.5
	// has only the older one, and 5.1 still offers it as deprecated.
	FunctionQuizAttempts = "mod_quiz_get_user_quiz_attempts"
	FunctionUserAttempts = "mod_quiz_get_user_attempts"
)

// quizzesDTO is Moodle's reply to mod_quiz_get_quizzes_by_courses.
//
// Everything past the name is VALUE_OPTIONAL: Moodle sends the settings only
// to an account allowed to view the quiz, so each is a pointer and a missing
// one stays unknown rather than becoming zero.
type quizzesDTO struct {
	Quizzes []struct {
		ID           int64    `json:"id"`
		CourseModule int64    `json:"coursemodule"`
		Course       int64    `json:"course"`
		Name         string   `json:"name"`
		TimeOpen     *int64   `json:"timeopen"`
		TimeClose    *int64   `json:"timeclose"`
		TimeLimit    *int64   `json:"timelimit"`
		Attempts     *int     `json:"attempts"`
		Grade        *float64 `json:"grade"`
		SumGrades    *float64 `json:"sumgrades"`
	} `json:"quizzes"`
}

// attemptsDTO is the reply to either attempts function; both return the same
// shape.
type attemptsDTO struct {
	Attempts []struct {
		ID         int64  `json:"id"`
		Attempt    int    `json:"attempt"`
		State      string `json:"state"`
		TimeStart  int64  `json:"timestart"`
		TimeFinish int64  `json:"timefinish"`
		// SumGrades is the raw mark, out of the quiz's own sumgrades, and null
		// while the attempt still needs marking.
		SumGrades *float64 `json:"sumgrades"`
	} `json:"attempts"`
}

// bestGradeDTO is Moodle's reply to mod_quiz_get_user_best_grade.
type bestGradeDTO struct {
	HasGrade bool     `json:"hasgrade"`
	Grade    *float64 `json:"grade"`
}

// QuizBackend reads quizzes over the web service API.
type QuizBackend struct {
	client       *Client
	token        string
	capabilities *site.Capabilities
}

// NewQuizBackend builds the web service backend for quizzes.
func NewQuizBackend(client *Client, token string, capabilities *site.Capabilities) *QuizBackend {
	return &QuizBackend{client: client, token: token, capabilities: capabilities}
}

func (b *QuizBackend) Name() site.BackendKind { return site.BackendWS }

func (b *QuizBackend) Requirement() site.Requirement {
	return site.Requirement{AnyFunction: []string{FunctionQuizzes}, Credential: site.CredentialWSToken}
}

func (b *QuizBackend) List(ctx context.Context, courseIDs []string) (quiz.ListResult, error) {
	dto, err := b.quizzes(ctx, courseIDs)
	if err != nil {
		return quiz.ListResult{}, err
	}
	result := quiz.ListResult{
		Quizzes:    []quiz.Quiz{},
		Provenance: quizWSProvenance(),
	}
	for i := range dto.Quizzes {
		result.Quizzes = append(result.Quizzes, dto.quiz(i))
	}
	return result, nil
}

// quizWSProvenance records the one field this route cannot fill.
//
// mod_quiz_get_quizzes_by_courses answers with the course's id and nothing
// else about it, unlike mod_assign_get_assignments which nests its quizzes
// under the course and carries its short name. A null short name with nothing
// in meta.missing would read as a course that has none, which Moodle does not
// allow.
func quizWSProvenance() site.Provenance {
	provenance := site.NewProvenance(site.BackendWS)
	provenance.Partial = true
	provenance.Missing = []string{"course_short_name"}
	return provenance
}

func (b *QuizBackend) Show(ctx context.Context, quizID string) (quiz.Detail, error) {
	id, err := strconv.ParseInt(quizID, 10, 64)
	if err != nil {
		return quiz.Detail{}, errs.New(errs.CodeUsage, "quiz ids are numbers")
	}
	// There is no call for one quiz's settings; the listing is it.
	dto, err := b.quizzes(ctx, nil)
	if err != nil {
		return quiz.Detail{}, err
	}
	index := -1
	for i, item := range dto.Quizzes {
		if item.ID == id {
			index = i
		}
	}
	if index < 0 {
		return quiz.Detail{}, errs.New(errs.CodeNotFound, "that quiz is not one you can see").
			WithHint("list them with `moodle quiz list`; the id there is the one to use")
	}
	detail := quiz.Detail{
		Quiz:       dto.quiz(index),
		Attempts:   []quiz.Attempt{},
		Provenance: quizWSProvenance(),
	}

	function := FunctionQuizAttempts
	if b.capabilities != nil && !b.capabilities.Has(function) {
		function = FunctionUserAttempts
	}
	var attempts attemptsDTO
	if err := b.client.Call(ctx, b.token, function, Params(map[string]any{
		"quizid": id, "status": "all", "includepreviews": false,
	}), &attempts); err != nil {
		return quiz.Detail{}, err
	}
	settings := dto.Quizzes[index]
	for _, raw := range attempts.Attempts {
		attempt := quiz.Attempt{
			ID:         strconv.FormatInt(raw.ID, 10),
			Number:     raw.Attempt,
			State:      raw.State,
			StartedAt:  unixTime(raw.TimeStart),
			FinishedAt: unixTime(raw.TimeFinish),
		}
		// Scaled the way the gradebook shows it. Without both totals the raw
		// mark would read as a grade out of something it is not.
		if raw.SumGrades != nil && settings.SumGrades != nil && *settings.SumGrades > 0 &&
			settings.Grade != nil {
			grade := *raw.SumGrades / *settings.SumGrades * *settings.Grade
			attempt.Grade = &grade
		}
		detail.Attempts = append(detail.Attempts, attempt)
	}

	var best bestGradeDTO
	if err := b.client.Call(ctx, b.token, FunctionBestGrade,
		Params(map[string]any{"quizid": id}), &best); err != nil {
		return quiz.Detail{}, err
	}
	if best.HasGrade {
		detail.BestGrade = best.Grade
	}
	return detail, nil
}

func (b *QuizBackend) quizzes(ctx context.Context, courseIDs []string) (quizzesDTO, error) {
	params := map[string]any{}
	if len(courseIDs) > 0 {
		ids := make([]any, 0, len(courseIDs))
		for _, raw := range courseIDs {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return quizzesDTO{}, errs.New(errs.CodeUsage, "course id "+raw+" is not a number")
			}
			ids = append(ids, id)
		}
		params["courseids"] = ids
	}
	var dto quizzesDTO
	err := b.client.Call(ctx, b.token, FunctionQuizzes, Params(params), &dto)
	return dto, err
}

// quiz maps one entry of the listing.
func (dto quizzesDTO) quiz(i int) quiz.Quiz {
	item := dto.Quizzes[i]
	out := quiz.Quiz{
		ID:       strconv.FormatInt(item.ID, 10),
		CMID:     strconv.FormatInt(item.CourseModule, 10),
		CourseID: strconv.FormatInt(item.Course, 10),
		// Escaped by this function as the forum listing is.
		Name:        html.UnescapeString(item.Name),
		MaxAttempts: item.Attempts,
		MaxGrade:    item.Grade,
	}
	if item.TimeOpen != nil {
		out.Opens = unixTime(*item.TimeOpen)
	}
	if item.TimeClose != nil {
		out.Closes = unixTime(*item.TimeClose)
	}
	if item.TimeLimit != nil {
		limit := time.Duration(*item.TimeLimit) * time.Second
		out.TimeLimit = &limit
	}
	return out
}
