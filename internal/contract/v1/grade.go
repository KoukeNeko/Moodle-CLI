package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/grade"
)

// Grade is one row of a course's gradebook on the wire.
type Grade struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Kind is activity, category, course, manual or unknown. A consumer that
	// treats every row as an activity will present the course total as one.
	Kind   string  `json:"kind"`
	Module *string `json:"module"`
	CMID   *string `json:"cmid"`
	// Grade is null when the work has not been marked. Zero is a real mark.
	Grade *float64 `json:"grade"`
	Min   *float64 `json:"grade_min"`
	Max   *float64 `json:"grade_max"`
	// Display is Moodle's own rendering, which is the only truthful answer for
	// an item graded on a scale or in letters.
	Display string `json:"display"`
	// Percentage is null wherever one would be meaningless, in particular for
	// anything graded on a scale.
	Percentage     *float64 `json:"percentage"`
	Feedback       *string  `json:"feedback"`
	FeedbackFormat *int     `json:"feedback_format"`
	GradedAt       *string  `json:"graded_at"`
	// Hidden marks a grade the site is withholding. It is not the same as an
	// unmarked one, and reporting it as unmarked would tell a student their
	// marked work was never looked at.
	Hidden bool `json:"hidden"`
	Locked bool `json:"locked"`
}

// GradeReport is the grade.list payload.
type GradeReport struct {
	CourseID string  `json:"course_id"`
	Items    []Grade `json:"items"`
	// Total is the course total. It is null when the site reports none, and it
	// is kept apart from the items because its maximum counts only what has
	// been graded so far — it is never the sum of the items above.
	Total *Grade `json:"total"`
	// NotGradable is true only when the site confirmed this account is not a
	// graded participant on the course. False covers both "it is" and "the
	// site would not say", so a reader must not treat false as proof.
	NotGradable bool `json:"not_gradable"`
}

// GradeList converts a course's gradebook into its envelope.
func GradeList(result grade.CourseResult, siteName, accountName string) Envelope {
	payload := GradeReport{
		CourseID:    result.CourseID,
		Items:       []Grade{},
		NotGradable: result.NotGradable,
	}
	for _, item := range result.Items {
		payload.Items = append(payload.Items, newGrade(item))
	}
	if result.Total != nil {
		total := newGrade(*result.Total)
		payload.Total = &total
	}
	return NewEnvelope("grade.list", payload,
		MetaFrom(result.Provenance, siteName, accountName))
}

func newGrade(item grade.Item) Grade {
	out := Grade{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       string(item.Kind),
		Grade:      item.Grade,
		Min:        item.Min,
		Max:        item.Max,
		Display:    item.Display,
		Percentage: item.Percentage,
		GradedAt:   Timestamp(item.GradedAt),
		Hidden:     item.Hidden,
		Locked:     item.Locked,
	}
	out.Module = optional(item.Module)
	out.CMID = optional(item.CMID)
	out.Feedback = optional(item.Feedback)
	if item.Feedback != "" {
		format := item.FeedbackFormat
		out.FeedbackFormat = &format
	}
	return out
}

// CourseTotal is one course's total on the wire.
type CourseTotal struct {
	CourseID string `json:"course_id"`
	// Grade is null when nothing in the course has been graded yet.
	Grade *float64 `json:"grade"`
	// Display is Moodle's own rendering. The overview call carries no maximum,
	// so there is deliberately no percentage here to compute from.
	Display string `json:"display"`
}

// GradeOverview converts every course's total into its envelope.
func GradeOverview(result grade.OverviewResult, siteName, accountName string) Envelope {
	courses := make([]CourseTotal, 0, len(result.Courses))
	for _, item := range result.Courses {
		courses = append(courses, CourseTotal{
			CourseID: item.CourseID,
			Grade:    item.Grade,
			Display:  item.Display,
		})
	}
	return NewEnvelope("grade.overview", courses,
		MetaFrom(result.Provenance, siteName, accountName))
}
