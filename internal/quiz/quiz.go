// Package quiz reads quizzes and the caller's attempts at them.
//
// Reading only. Starting an attempt is a write that also starts the clock on
// a timed quiz, and viewing one is recorded like any other *_view_* call, so
// neither is made here.
package quiz

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Quiz is one quiz in a course.
type Quiz struct {
	// ID is the quiz's own id where the route knows it. Reading pages sees
	// only the course module id, and reports that here as well, as the
	// assignment and forum routes do.
	ID       string
	CMID     string
	CourseID string
	// CourseShortName is nil when the route that answered did not say.
	CourseShortName *string
	Name            string
	// Opens and Closes are nil when the quiz sets no such date — or, on a
	// route that cannot see them, when the answer lists them as missing.
	Opens  *time.Time
	Closes *time.Time
	// TimeLimit is nil when the route could not see it; zero is no limit.
	TimeLimit *time.Duration
	// MaxAttempts is nil when the route could not see it; zero is unlimited,
	// which is how Moodle writes it.
	MaxAttempts *int
	// MaxGrade is what the quiz is marked out of, nil when not known.
	MaxGrade *float64
}

// Attempt is one of the caller's attempts.
type Attempt struct {
	ID string
	// Number counts from one, as Moodle does for quiz attempts.
	Number int
	// State is Moodle's own word: inprogress, overdue, finished or abandoned.
	State      string
	StartedAt  *time.Time
	FinishedAt *time.Time
	// Grade is the attempt's mark scaled to the quiz's maximum. Nil while the
	// attempt is not finished, when it still needs manual marking, or when
	// the marks are not released to this account yet.
	Grade *float64
}

// Detail is one quiz with the caller's standing in it.
type Detail struct {
	Quiz
	// Attempts is nil when the route could not read them, which is not the
	// same as an empty list: a quiz nobody has tried.
	Attempts []Attempt
	// BestGrade is nil when there is none yet or the route could not tell;
	// the provenance says which.
	BestGrade  *float64
	Provenance site.Provenance
}

// ListResult is a set of quizzes plus where it came from.
type ListResult struct {
	Quizzes    []Quiz
	Provenance site.Provenance
}

// Backend is one way of reading quizzes.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	List(ctx context.Context, courseIDs []string) (ListResult, error)
	// Show reads one quiz. The id is whatever List reported as the id on
	// this route.
	Show(ctx context.Context, quizID string) (Detail, error)
}
