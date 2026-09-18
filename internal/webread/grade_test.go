package webread_test

import (
	"os"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

func gradePage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/grades.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestGradesAreReadFromARealReport(t *testing.T) {
	grades, err := webread.ParseGrades(gradePage(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(grades) != 4 {
		t.Fatalf("got %d rows, want 3 activities and the total: %+v", len(grades), grades)
	}

	marked := grades[0]
	if marked.Name != "A1 direct submit" {
		t.Errorf("name = %q; the activity-type label may have been included", marked.Name)
	}
	if marked.Grade != "85.00" {
		t.Errorf("grade = %q; the action menu's text may have been included", marked.Grade)
	}
	if marked.Percentage != "85.00 %" {
		t.Errorf("percentage = %q", marked.Percentage)
	}
	if !strings.Contains(marked.Feedback, "starvation") {
		t.Errorf("feedback = %q", marked.Feedback)
	}
}

func TestUnmarkedWorkIsEmptyNotADash(t *testing.T) {
	// The report prints a dash where there is nothing to show. Carrying it
	// through as text would make unmarked work look like a mark of "-".
	grades, err := webread.ParseGrades(gradePage(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, grade := range grades {
		if grade.Name != "A2 submit button" {
			continue
		}
		if grade.Grade != "" {
			t.Errorf("grade = %q, want empty", grade.Grade)
		}
		if grade.Graded() {
			t.Error("an unmarked item reports itself as graded")
		}
		// The range is still known even when nothing is marked.
		if grade.Range == "" {
			t.Error("the range was dropped along with the empty grade")
		}
		return
	}
	t.Fatal("A2 is missing from the report")
}

func TestTheCourseTotalIsMarkedAsSuch(t *testing.T) {
	// Its maximum counts only what has been graded, so a consumer that treats
	// it as another activity will present a total as coursework.
	grades, err := webread.ParseGrades(gradePage(t))
	if err != nil {
		t.Fatal(err)
	}
	totals := 0
	for _, grade := range grades {
		if grade.Total {
			totals++
			if grade.Grade != "85.00" {
				t.Errorf("total = %q", grade.Grade)
			}
		}
	}
	if totals != 1 {
		t.Errorf("got %d total rows, want 1", totals)
	}
}

func TestTheHeadingRowIsNotReadAsAnItem(t *testing.T) {
	// The headings carry the same column keys as the data. Without telling
	// them apart there is an item called "Grade item" with a grade of "Grade".
	grades, err := webread.ParseGrades(gradePage(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, grade := range grades {
		if grade.Grade == "Grade" || grade.Name == "Grade item" {
			t.Errorf("the heading row was read as an item: %+v", grade)
		}
		if grade.Name == "Operating Systems" {
			t.Error("the category row was read as a graded item")
		}
	}
}

func TestAReportWithoutColumnKeysSaysTheShapeChanged(t *testing.T) {
	// A course with nothing graded and a page this build cannot read must not
	// look the same.
	page := strings.ReplaceAll(gradePage(t), "column-", "col-")

	_, err := webread.ParseGrades(page)
	if err == nil {
		t.Fatal("a report with no recognisable columns was parsed anyway")
	}
	if errs.From(err).Reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q, want protocol_drift", errs.From(err).Reason)
	}
}

func TestTheValuesComeFromColumnKeysNotHeadings(t *testing.T) {
	// The headings are translated and the keys are not, so a site in another
	// language must read the same.
	translated := strings.NewReplacer(
		"Grade item", "評分項目", "Grade</th>", "成績</th>",
		"Range", "範圍", "Percentage", "百分比", "Feedback", "回饋",
		"Course total", "課程總分", "Assignment", "作業",
	).Replace(gradePage(t))

	grades, err := webread.ParseGrades(translated)
	if err != nil {
		t.Fatal(err)
	}
	if len(grades) != 4 {
		t.Fatalf("got %d rows on a translated page, want 4", len(grades))
	}
	if grades[0].Grade != "85.00" {
		t.Errorf("grade = %q on a translated page", grades[0].Grade)
	}
	if !grades[3].Total {
		t.Error("the total was not recognised on a translated page")
	}
}
