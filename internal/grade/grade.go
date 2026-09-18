// Package grade is the grade use cases and their model.
//
// Two questions are asked of a gradebook and they are not the same one: how am
// I doing in this course, and how am I doing overall. Moodle answers them with
// different calls that carry different information — the overview has no
// maximum to divide by — so they stay separate here rather than being merged
// into one shape that is half empty on either side.
package grade

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Kind says what a row in the gradebook is.
type Kind string

const (
	// KindActivity is a grade for an activity, such as an assignment.
	KindActivity Kind = "activity"
	// KindCategory is a subtotal.
	KindCategory Kind = "category"
	// KindCourse is the course total. It is not an activity and must not be
	// listed as one.
	KindCourse Kind = "course"
	// KindManual is a grade a teacher entered directly.
	KindManual Kind = "manual"
	// KindUnknown is anything this build does not recognise, kept distinct
	// rather than folded into one of the others.
	KindUnknown Kind = "unknown"
)

// Item is one row of a course's gradebook.
type Item struct {
	ID   string
	Name string
	Kind Kind
	// Module and CMID identify the activity a grade belongs to, so a caller
	// can get from a grade back to the thing that was graded.
	Module string
	CMID   string
	// Grade is nil when the work has not been marked. Zero is a real mark and
	// means something quite different.
	Grade *float64
	Min   *float64
	Max   *float64
	// Display is Moodle's own rendering. It is the only truthful answer when
	// the item uses a scale or letters, where a bare number means nothing.
	Display string
	// Percentage is computed only where it is meaningful: never for an item
	// graded on a scale, where the raw value is a position in a list rather
	// than a quantity.
	Percentage     *float64
	Feedback       string
	FeedbackFormat int
	GradedAt       *time.Time
	// Hidden reports a grade the site is withholding. That is different from
	// one that does not exist yet, and a caller that conflates them will tell
	// a student their marked work was never marked.
	Hidden    bool
	Locked    bool
	UsesScale bool
}

// Graded reports whether the item carries a mark.
func (i Item) Graded() bool { return i.Grade != nil }

// CourseResult is one course's gradebook plus where it came from.
type CourseResult struct {
	CourseID string
	Items    []Item
	// Total is the course total row, pulled out of the list because it is the
	// answer to a different question from the items above it.
	Total      *Item
	Provenance site.Provenance
}

// CourseGrade is one course's total, as the overview reports it.
type CourseGrade struct {
	CourseID string
	// Grade is nil when the course has nothing graded yet.
	Grade *float64
	// Display is Moodle's own rendering; the overview gives no maximum, so
	// there is nothing here to compute a percentage from.
	Display string
}

// OverviewResult is every course's total.
type OverviewResult struct {
	Courses    []CourseGrade
	Provenance site.Provenance
}

// Backend is one way of reading grades.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	Course(ctx context.Context, courseID string) (CourseResult, error)
	Overview(ctx context.Context) (OverviewResult, error)
}

// Percentage works out a percentage where one is meaningful.
//
// It is not meaningful for an item graded on a scale: the raw value is a
// position in a list of words, and dividing it by the number of options would
// produce a number that looks like a percentage and means nothing.
func Percentage(value, min, max *float64, usesScale bool) *float64 {
	if usesScale || value == nil || min == nil || max == nil {
		return nil
	}
	span := *max - *min
	if span <= 0 {
		return nil
	}
	percent := (*value - *min) / span * 100
	return &percent
}
