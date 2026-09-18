package webread

import (
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Anchors on the user grade report.
const (
	// prefixColumn names what a cell holds, as in "column-grade". Moodle
	// generates these from the report's column keys, so they do not change
	// with the site's language the way the header text does.
	prefixColumn = "column-"
	// classAggregate marks a row that totals the rows above it rather than
	// being an activity of its own.
	classAggregate = "baggt"
	// classRowTitle wraps an item's own name, without the activity-type label
	// Moodle shows beside it.
	classRowTitle = "rowtitle"
	// classActionMenu wraps the controls Moodle puts inside the grade cell.
	// Their text would otherwise be read as part of the grade.
	classActionMenu = "action-menu"
	// classHeader marks the column headings. They carry the same column keys
	// as the data, so without this the heading row reads as an item called
	// "Grade item" with a grade of "Grade".
	classHeader = "header"
	// classCategory marks the row naming the category or the course itself,
	// which heads the rows under it rather than being graded.
	classCategory = "category"
)

// Grade is one row of the user's grade report.
type Grade struct {
	Name string
	// Grade is Moodle's own rendering — "85.00", a letter, or a scale word.
	// It is empty when nothing has been marked, which the report shows as a
	// dash.
	Grade string
	// Range is the item's range as the report prints it, such as "0–100".
	Range string
	// Percentage is empty when the report shows none, which includes anything
	// graded on a scale.
	Percentage string
	Feedback   string
	// Total marks the course total rather than an activity. Its maximum counts
	// only what has been graded, so it is never the sum of the rows above.
	Total bool
}

// Graded reports whether this row carries a mark.
func (g Grade) Graded() bool { return g.Grade != "" }

// ParseGrades reads grade/report/user/index.php.
//
// Every value is taken from a cell Moodle labels with a generated column key.
// The words in the header row are translated; the keys are not.
func ParseGrades(markup string) ([]Grade, error) {
	document, err := parse(markup)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUpstream, err, "cannot read the grade report").
			WithReason(errs.ReasonProtocolDrift)
	}

	rows := document.findAll(byTag("tr"))
	if len(rows) == 0 {
		return nil, errs.New(errs.CodeUpstream, "the grade report has no rows").
			WithReason(errs.ReasonProtocolDrift)
	}

	var grades []Grade
	anchored := false
	for _, row := range rows {
		cells := map[string]node{}
		for _, cell := range row.findAll(func(n node) bool {
			return n.Data == "td" || n.Data == "th"
		}) {
			if key := cell.classWithPrefix(prefixColumn); key != "" {
				cells[key] = cell
			}
		}
		if len(cells) == 0 {
			// Header and spacer rows carry no column keys.
			continue
		}
		anchored = true

		name, ok := cells["itemname"]
		if !ok {
			continue
		}
		// The heading row and the category row carry the same column keys as
		// the data. Both are told apart by generated classes rather than by
		// their words, which are translated.
		if name.hasClass(classHeader) || name.hasClass(classCategory) {
			continue
		}
		grade := Grade{
			Name:       itemName(name),
			Grade:      dashToEmpty(gradeValue(cells["grade"])),
			Range:      dashToEmpty(cellText(cells["range"])),
			Percentage: dashToEmpty(cellText(cells["percentage"])),
			Feedback:   dashToEmpty(cellText(cells["feedback"])),
			Total:      name.hasClass(classAggregate),
		}
		if grade.Name == "" {
			continue
		}
		grades = append(grades, grade)
	}

	if !anchored {
		// A report with no graded items is possible; a page this build cannot
		// read is not the same thing, and the column keys tell them apart.
		return nil, errs.New(errs.CodeUpstream,
			"the grade report carries no recognisable columns").
			WithReason(errs.ReasonProtocolDrift).
			WithHint("the site's pages may have changed shape")
	}
	return grades, nil
}

// itemName reads the item's own name.
//
// Moodle prints the activity type beside it — "Assignment A1" — and both the
// label and the icon's alt text are translated. The name itself sits in its
// own element.
func itemName(cell node) string {
	if title, ok := cell.find(byClass(classRowTitle)); ok {
		return title.text()
	}
	return cell.text()
}

// gradeValue reads the grade, skipping the controls Moodle puts in the cell.
//
// The action menu's text — "Actions", "Grade analysis" — is translated and
// would otherwise end up inside the number.
func gradeValue(cell node) string {
	if cell.Node == nil {
		return ""
	}
	return cell.textExcept(classActionMenu)
}

func cellText(cell node) string {
	if cell.Node == nil {
		return ""
	}
	return cell.text()
}

// dashToEmpty turns the report's placeholder into an absent value.
//
// Moodle prints a dash where there is nothing to show. Carrying that through
// as text would make an unmarked item look like one marked "-".
func dashToEmpty(value string) string {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case "-", "–", "—", " ", "":
		return ""
	}
	return trimmed
}
