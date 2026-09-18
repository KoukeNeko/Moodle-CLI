package moodle

import (
	"context"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

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

// Overview is not available from this route.
//
// Moodle's overview report lists every course's total on one page, but a
// student's own view of it is not reachable without the grade report's own
// navigation, and guessing at a URL that may not exist would report a missing
// page as a missing grade.
func (b *GradeHTMLBackend) Overview(context.Context) (grade.OverviewResult, error) {
	return grade.OverviewResult{}, errs.New(errs.CodeUnavailable,
		"reading pages cannot list every course's total").
		WithReason(errs.ReasonCapability).
		WithHint("ask for one course at a time with `moodle grade list --course`")
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
