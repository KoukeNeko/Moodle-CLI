package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/quiz"
)

// Quiz is one quiz on the wire.
type Quiz struct {
	ID       string `json:"id"`
	CMID     string `json:"cmid"`
	CourseID string `json:"course_id"`
	// CourseShortName is null when the route that answered did not carry it.
	CourseShortName *string `json:"course_short_name"`
	Name            string  `json:"name"`
	OpensAt         *string `json:"opens_at"`
	ClosesAt        *string `json:"closes_at"`
	// TimeLimitSeconds is 0 for no limit, and null when the route could not
	// see the setting.
	TimeLimitSeconds *int `json:"time_limit_seconds"`
	// MaxAttempts is 0 for unlimited, as Moodle writes it, and null when the
	// route could not see the setting.
	MaxAttempts *int     `json:"max_attempts"`
	MaxGrade    *float64 `json:"max_grade"`
}

// QuizAttempt is one of the caller's attempts on the wire.
type QuizAttempt struct {
	ID         string  `json:"id"`
	Number     int     `json:"number"`
	State      string  `json:"state"`
	StartedAt  *string `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	// Grade is scaled to the quiz's maximum, and null while the attempt is
	// unfinished, still being marked, or not yet released to this account.
	Grade *float64 `json:"grade"`
}

// QuizDetail is the quiz.show payload.
type QuizDetail struct {
	Quiz
	// Attempts is null when the route could not read them; an empty list is
	// a quiz this account has not tried.
	Attempts  []QuizAttempt `json:"attempts"`
	BestGrade *float64      `json:"best_grade"`
}

func newQuiz(item quiz.Quiz) Quiz {
	out := Quiz{
		ID:              item.ID,
		CMID:            item.CMID,
		CourseID:        item.CourseID,
		CourseShortName: item.CourseShortName,
		Name:            item.Name,
		OpensAt:         Timestamp(item.Opens),
		ClosesAt:        Timestamp(item.Closes),
		MaxAttempts:     item.MaxAttempts,
		MaxGrade:        item.MaxGrade,
	}
	if item.TimeLimit != nil {
		seconds := int(item.TimeLimit.Seconds())
		out.TimeLimitSeconds = &seconds
	}
	return out
}

// QuizList converts a listing into its envelope.
func QuizList(result quiz.ListResult, siteName, accountName string) Envelope {
	items := make([]Quiz, 0, len(result.Quizzes))
	for _, item := range result.Quizzes {
		items = append(items, newQuiz(item))
	}
	return NewEnvelope("quiz.list", items, MetaFrom(result.Provenance, siteName, accountName))
}

// QuizShow converts one quiz and the caller's attempts into its envelope.
func QuizShow(detail quiz.Detail, siteName, accountName string) Envelope {
	payload := QuizDetail{Quiz: newQuiz(detail.Quiz), BestGrade: detail.BestGrade}
	if detail.Attempts != nil {
		payload.Attempts = make([]QuizAttempt, 0, len(detail.Attempts))
		for _, attempt := range detail.Attempts {
			payload.Attempts = append(payload.Attempts, QuizAttempt{
				ID:         attempt.ID,
				Number:     attempt.Number,
				State:      attempt.State,
				StartedAt:  Timestamp(attempt.StartedAt),
				FinishedAt: Timestamp(attempt.FinishedAt),
				Grade:      attempt.Grade,
			})
		}
	}
	return NewEnvelope("quiz.show", payload, MetaFrom(detail.Provenance, siteName, accountName))
}
