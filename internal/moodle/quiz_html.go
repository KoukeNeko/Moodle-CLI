package moodle

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/quiz"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

// PathQuizView is the page showing one quiz.
const PathQuizView = "/mod/quiz/view.php"

// Contract fields a page does not state in a form this route can read. The
// quiz page prints its settings and each attempt's state and mark, but only
// as sentences in the site's language: "允許作答幾次： 1" and "已經完成" are
// the same facts as "Attempts allowed: 1" and "Finished", and reading one of
// them is reading none of the others.
var (
	quizPageBlindFields   = []string{"time_limit_seconds", "max_attempts", "max_grade"}
	quizDetailBlindFields = []string{"attempts", "best_grade"}
)

// QuizHTMLBackend reads quizzes from Moodle's own pages, for a site that
// issues no token: the quiz functions are not offered over AJAX.
//
// As with assignments and forums, the id it reports is the course module id,
// the only one a page carries.
type QuizHTMLBackend struct {
	pages *PageReader
	pageContext
}

// NewQuizHTMLBackend builds the page-reading backend for quizzes.
func NewQuizHTMLBackend(pages *PageReader, calendar *AjaxSession,
	courses func(context.Context) ([]course.Summary, error)) *QuizHTMLBackend {
	return &QuizHTMLBackend{pages: pages,
		pageContext: pageContext{calendar: calendar, courses: courses}}
}

func (b *QuizHTMLBackend) Name() site.BackendKind { return site.BackendHTML }

// Requirement is empty: a page needs no function to be exposed.
func (b *QuizHTMLBackend) Requirement() site.Requirement { return site.Requirement{} }

func (b *QuizHTMLBackend) List(ctx context.Context, courseIDs []string) (quiz.ListResult, error) {
	known, err := b.knownCourses(ctx)
	if len(courseIDs) == 0 {
		if err != nil {
			return quiz.ListResult{}, err
		}
		for _, item := range known {
			courseIDs = append(courseIDs, item.ID)
		}
	}

	result := quiz.ListResult{
		Quizzes:    []quiz.Quiz{},
		Provenance: site.NewProvenance(site.BackendHTML),
	}
	missing := append([]string{}, quizPageBlindFields...)
	var selected []course.Summary
	for _, courseID := range courseIDs {
		page, err := b.pages.Get(ctx, PathCourseView, map[string]string{"id": courseID})
		if err != nil {
			return quiz.ListResult{}, err
		}
		activities, err := webread.ParseCourseActivities(page)
		if err != nil {
			return quiz.ListResult{}, err
		}
		summary, named := known[courseID]
		if named {
			selected = append(selected, summary)
		} else if !contains(missing, "course_short_name") {
			missing = append(missing, "course_short_name")
		}
		for _, activity := range activities {
			if activity.Module != "quiz" {
				continue
			}
			item := quiz.Quiz{
				ID: activity.CMID, CMID: activity.CMID,
				CourseID: courseID, Name: activity.Name,
			}
			if named {
				short := summary.ShortName
				item.CourseShortName = &short
			}
			result.Quizzes = append(result.Quizzes, item)
		}
	}

	events, err := b.events(ctx, "quiz", selected, "")
	if err != nil || len(selected) < len(courseIDs) {
		// Without the calendar a null date would read as "none set".
		missing = append(missing, "opens_at", "closes_at")
	}
	for i := range result.Quizzes {
		result.Quizzes[i].Opens = events.at(result.Quizzes[i].CMID, "open")
		result.Quizzes[i].Closes = events.at(result.Quizzes[i].CMID, "close")
	}
	result.Provenance.Partial = true
	result.Provenance.Missing = missing
	return result, nil
}

func (b *QuizHTMLBackend) Show(ctx context.Context, quizID string) (quiz.Detail, error) {
	markup, err := b.pages.Get(ctx, PathQuizView, map[string]string{"id": quizID})
	if err != nil {
		return quiz.Detail{}, err
	}
	// The quiz page carries the same generated anchors as an assignment's:
	// the activity's name on its information block and the course on the
	// body.
	page, err := webread.ParseAssignPage(markup)
	if err != nil {
		return quiz.Detail{}, err
	}
	if page.Name == "" && page.CourseID == "" {
		return quiz.Detail{}, errs.New(errs.CodeUpstream,
			"this quiz page carries neither its name nor its course").
			WithReason(errs.ReasonProtocolDrift).
			WithHint("the site's pages may have changed shape, or this may not be a quiz page")
	}

	detail := quiz.Detail{
		Quiz: quiz.Quiz{
			ID: quizID, CMID: quizID,
			CourseID: page.CourseID, Name: page.Name,
		},
		Provenance: site.NewProvenance(site.BackendHTML),
	}
	missing := append(append([]string{}, quizPageBlindFields...), quizDetailBlindFields...)
	known, _ := b.knownCourses(ctx)
	summary, named := known[page.CourseID]
	var selected []course.Summary
	if named {
		short := summary.ShortName
		detail.CourseShortName = &short
		selected = append(selected, summary)
	} else {
		missing = append(missing, "course_short_name")
	}
	events, err := b.events(ctx, "quiz", selected, page.CourseID)
	if err != nil || !named {
		missing = append(missing, "opens_at", "closes_at")
	}
	detail.Opens = events.at(quizID, "open")
	detail.Closes = events.at(quizID, "close")
	detail.Provenance.Partial = true
	detail.Provenance.Missing = missing
	return detail, nil
}
