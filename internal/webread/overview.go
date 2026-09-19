package webread

import (
	"net/url"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Anchors on the overview report.
const (
	// idOverviewTable is the table holding one row per course. Moodle
	// generates it; the heading words above it are translated and the class
	// list is themeable, but this id is neither — checked on 4.5, 5.1 and 5.2.
	idOverviewTable = "overview-grade"
	// pathUserGrade is the link every course row carries, and the only place
	// on the page the course id appears.
	pathUserGrade = "/course/user.php"
)

// CourseTotal is one course's line in the overview report.
type CourseTotal struct {
	// CourseID comes from the row's own link. A row whose link this build
	// cannot read is dropped rather than reported without one: a total that
	// cannot be tied to a course is not something to show a student.
	CourseID string
	Name     string
	// Grade is Moodle's own rendering. It is empty when the report shows a
	// dash, which means nothing has been marked — not that the total is zero.
	Grade string
}

// ParseOverview reads grade/report/overview/index.php.
//
// This page is the reason a browser-session account can be told its totals at
// all: no web service and no AJAX endpoint on such a site will answer for the
// overview report, and the command used to say reading pages could not do it.
func ParseOverview(markup string) ([]CourseTotal, error) {
	document, err := parse(markup)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUpstream, err, "cannot read the overview report").
			WithReason(errs.ReasonProtocolDrift)
	}

	table, ok := document.find(byID(idOverviewTable))
	if !ok {
		return nil, errs.New(errs.CodeUpstream,
			"the overview report does not have the table this build reads").
			WithReason(errs.ReasonProtocolDrift).
			WithHint("the site's grade overview page has changed shape")
	}

	var totals []CourseTotal
	for _, row := range table.findAll(byTag("tr")) {
		if row.hasClass(classHeader) {
			continue
		}
		cells := row.findAll(byTag("td"))
		if len(cells) < 2 {
			continue
		}
		// The report pads itself out with empty rows to a fixed height. They
		// carry the same classes as a real one, so the link is what separates
		// a course from a spacer.
		id := courseIDFromLink(cells[0])
		if id == "" {
			continue
		}
		grade := strings.TrimSpace(cells[1].text())
		if grade == "-" {
			// The dash is the report's way of saying nothing is marked yet.
			// Carrying it through as a value would make it a grade.
			grade = ""
		}
		totals = append(totals, CourseTotal{
			CourseID: id,
			Name:     strings.TrimSpace(cells[0].text()),
			Grade:    grade,
		})
	}

	if len(totals) == 0 {
		// An account with no graded course is possible, and so is a page this
		// build can no longer read. The table was found, so the shape is still
		// there; an empty answer here is about the account.
		return []CourseTotal{}, nil
	}
	return totals, nil
}

// courseIDFromLink pulls the course id out of the row's own link.
func courseIDFromLink(cell node) string {
	for _, link := range cell.findAll(byTag("a")) {
		href := link.attr("href")
		if !strings.Contains(href, pathUserGrade) {
			continue
		}
		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}
		if id := parsed.Query().Get("id"); id != "" {
			return id
		}
	}
	return ""
}

// byID matches an element by its id attribute.
func byID(id string) func(node) bool {
	return func(n node) bool { return n.attr("id") == id }
}
