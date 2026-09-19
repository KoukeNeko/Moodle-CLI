package webread_test

import (
	"os"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

func overviewPage(t *testing.T) string {
	t.Helper()
	// Recorded from a real Moodle 5.2 with mobile web services off, which is
	// the only site where this page is the route rather than a curiosity.
	raw, err := os.ReadFile("testdata/overview.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestOverviewIsReadFromARealReportPage(t *testing.T) {
	totals, err := webread.ParseOverview(overviewPage(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 {
		t.Fatalf("got %d course totals, want 1: %+v", len(totals), totals)
	}
	if totals[0].CourseID != "2" {
		t.Errorf("course id = %q; it comes from the row's own link", totals[0].CourseID)
	}
	if totals[0].Name != "Operating Systems" {
		t.Errorf("name = %q", totals[0].Name)
	}
	if totals[0].Grade != "85.00" {
		t.Errorf("grade = %q, want Moodle's own rendering", totals[0].Grade)
	}
}

func TestThePaddingRowsAreNotCourses(t *testing.T) {
	// The report pads itself to a fixed height with rows carrying the same
	// classes as a real one. Counting those would invent courses.
	totals, err := webread.ParseOverview(overviewPage(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, total := range totals {
		if total.CourseID == "" || total.Name == "" {
			t.Errorf("a padding row was read as a course: %+v", total)
		}
	}
}

func TestAReportWithoutTheTableIsDriftNotEmptiness(t *testing.T) {
	// A page this build can no longer read and an account with no graded
	// course must not look the same.
	_, err := webread.ParseOverview("<html><body><p>nothing here</p></body></html>")
	if err == nil {
		t.Fatal("a page without the report table was reported as no courses")
	}
}

func TestADashIsNotAGrade(t *testing.T) {
	markup := `<table id="overview-grade">
	  <tr class="header"><th>Course name</th><th>Grade</th></tr>
	  <tr><td><a href="/course/user.php?mode=grade&id=7&user=4">Ethics</a></td><td>-</td></tr>
	</table>`
	totals, err := webread.ParseOverview(markup)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 {
		t.Fatalf("got %d, want 1", len(totals))
	}
	if totals[0].Grade != "" {
		t.Errorf("grade = %q; the report's dash means nothing is marked, not a value",
			totals[0].Grade)
	}
}
