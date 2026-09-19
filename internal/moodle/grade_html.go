package moodle

import (
	"context"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

// PathOverviewGrades is the report listing one row per course. It is the only
// route to every course's total for an account without a web service token.
const PathOverviewGrades = "/grade/report/overview/index.php"

// PathUserGrades is the per-course grade report.
const PathUserGrades = "/grade/report/user/index.php"

// GradeHTMLBackend reads grades from Moodle's own report.
//
// Grades are exposed over neither the AJAX endpoint nor any other route a
// browser session can reach, so on a site with mobile web services off this is
// the only source.
//
// What it can report is narrower than the web service route, and deliberately
// so. The report prints what a person should see — a number, a letter, a word
// from a scale — already formatted by the site. Those strings are carried
// through as Moodle wrote them rather than parsed into numbers, because
// turning "Good" or "85.00 %" back into a quantity means guessing at a scale
// this route cannot see.
type GradeHTMLBackend struct {
	pages *PageReader
}

// NewGradeHTMLBackend builds the page-reading backend for grades.
func NewGradeHTMLBackend(pages *PageReader) *GradeHTMLBackend {
	return &GradeHTMLBackend{pages: pages}
}

func (b *GradeHTMLBackend) Name() site.BackendKind { return site.BackendHTML }

// Requirement is empty: a page needs no function to be exposed.
func (b *GradeHTMLBackend) Requirement() site.Requirement {
	return site.Requirement{}
}

func (b *GradeHTMLBackend) Course(ctx context.Context, courseID string) (grade.CourseResult, error) {
	page, err := b.pages.Get(ctx, PathUserGrades, map[string]string{"id": courseID})
	if err != nil {
		return grade.CourseResult{}, err
	}
	rows, err := webread.ParseGrades(page)
	if err != nil {
		return grade.CourseResult{}, err
	}

	result := grade.CourseResult{
		CourseID:   courseID,
		Items:      []grade.Item{},
		Provenance: site.NewProvenance(site.BackendHTML),
	}
	for _, row := range rows {
		item := grade.Item{
			Name: row.Name,
			Kind: grade.KindActivity,
			// Moodle's own rendering, unchanged. It is the only truthful
			// answer here: the report shows a scale's word or a letter the
			// same way it shows a number, and this route cannot tell which.
			Display:  row.Grade,
			Feedback: row.Feedback,
		}
		if row.Total {
			item.Kind = grade.KindCourse
		}
		// A number is offered only when the display really is one. Anything
		// else stays absent rather than being coerced: a scale position read
		// as a quantity is a wrong answer that looks right.
		if value, err := strconv.ParseFloat(strings.TrimSpace(row.Grade), 64); err == nil {
			item.Grade = &value
		}
		item.Min, item.Max = parseRange(row.Range)
		// Percentage is not computed from the parts here. The report already
		// printed one where it was meaningful, and it omits it where it is
		// not — which is exactly the distinction that would be lost by
		// dividing whatever numbers happen to be present.
		if percent, ok := parsePercentage(row.Percentage); ok {
			item.Percentage = &percent
		} else {
			item.UsesScale = row.Graded() && row.Percentage == ""
		}

		if item.Kind == grade.KindCourse {
			total := item
			result.Total = &total
			continue
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// Overview reads the report Moodle shows a student for every course at once.
//
// This used to refuse, saying reading pages could not list every course's
// total. That was wrong: grade/report/overview/index.php is exactly that list,
// and it answered on 4.5, 5.1 and 5.2 for the same browser session the command
// already held. A route that exists and a sentence saying it does not are the
// same kind of mistake as an empty list read as an absence.
//
// It is narrower than the web service route in one way worth stating: the
// report prints what a person should see, so the number is Moodle's own
// rendering and is carried through unchanged rather than parsed into a
// quantity this route cannot check.
func (b *GradeHTMLBackend) Overview(ctx context.Context) (grade.OverviewResult, error) {
	page, err := b.pages.Get(ctx, PathOverviewGrades, nil)
	if err != nil {
		return grade.OverviewResult{}, err
	}
	rows, err := webread.ParseOverview(page)
	if err != nil {
		return grade.OverviewResult{}, err
	}

	result := grade.OverviewResult{
		Courses:    []grade.CourseGrade{},
		Provenance: site.NewProvenance(site.BackendHTML),
	}
	for _, row := range rows {
		course := grade.CourseGrade{
			CourseID: row.CourseID,
			Display:  row.Grade,
		}
		// Only when the display really is a number. A letter or a scale's
		// word read as a quantity is a wrong answer that looks right.
		if value, err := strconv.ParseFloat(strings.TrimSpace(row.Grade), 64); err == nil {
			course.Grade = &value
		}
		result.Courses = append(result.Courses, course)
	}
	return result, nil
}

// parseRange reads "0–100" as Moodle prints it.
//
// The dash is an en dash, and a site may render the bounds as anything its
// grade type calls for, so a range that does not read as two numbers is left
// absent rather than half-filled.
func parseRange(raw string) (min, max *float64) {
	for _, separator := range []string{"–", "-", "—"} {
		low, high, found := strings.Cut(raw, separator)
		if !found {
			continue
		}
		lowValue, lowErr := strconv.ParseFloat(strings.TrimSpace(low), 64)
		highValue, highErr := strconv.ParseFloat(strings.TrimSpace(high), 64)
		if lowErr != nil || highErr != nil {
			return nil, nil
		}
		return &lowValue, &highValue
	}
	return nil, nil
}

// parsePercentage reads "85.00 %" as Moodle prints it.
func parsePercentage(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "%"))
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
