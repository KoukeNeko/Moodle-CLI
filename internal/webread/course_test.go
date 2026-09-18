package webread_test

import (
	"os"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

func coursePage(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/course.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestActivitiesAreReadFromARealCoursePage(t *testing.T) {
	activities, err := webread.ParseCourseActivities(coursePage(t))
	if err != nil {
		t.Fatal(err)
	}

	byCMID := map[string]webread.Activity{}
	for _, activity := range activities {
		byCMID[activity.CMID] = activity
	}

	// Both kinds are on the page; a caller filters, the parser does not.
	assign, ok := byCMID["2"]
	if !ok {
		t.Fatalf("the assignment at cmid 2 is missing from %v", activities)
	}
	if assign.Module != "assign" {
		t.Errorf("module = %q", assign.Module)
	}
	if assign.Name != "A1 direct submit" {
		t.Errorf("name = %q; the screen-reader label may have been included", assign.Name)
	}
	if forum, ok := byCMID["1"]; !ok || forum.Module != "forum" {
		t.Errorf("the forum at cmid 1 is missing or mistyped: %+v", forum)
	}
}

func TestTheNameExcludesTheLabelBesideIt(t *testing.T) {
	// Moodle puts "Assignment" and the completion state in the link for screen
	// readers. Both are translated, and neither is part of the name.
	activities, err := webread.ParseCourseActivities(coursePage(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, activity := range activities {
		if strings.Contains(activity.Name, "Assignment") && activity.Name != "Assignment" {
			t.Errorf("name %q carries the activity type label", activity.Name)
		}
	}
}

func TestEachActivityIsListedOnce(t *testing.T) {
	// A course page links the same activity several times — the title, the
	// icon, the completion control.
	activities, err := webread.ParseCourseActivities(coursePage(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, activity := range activities {
		seen[activity.CMID]++
	}
	for cmid, count := range seen {
		if count != 1 {
			t.Errorf("cmid %s appears %d times", cmid, count)
		}
	}
}

func TestAPageThatListsNothingReadableIsNotReportedAsEmpty(t *testing.T) {
	// A course with no activities and a page this build cannot read must not
	// look the same.
	page := `<html><body><div class="activityname">` +
		`<a href="/mod/assign/somethingelse.php?id=2">A1</a></div></body></html>`

	_, err := webread.ParseCourseActivities(page)
	if err == nil {
		t.Fatal("a page with unreadable activities was reported as empty")
	}
	if errs.From(err).Reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q, want protocol_drift", errs.From(err).Reason)
	}
}

func TestAnEmptyCourseIsEmptyNotAnError(t *testing.T) {
	activities, err := webread.ParseCourseActivities(
		`<html><body><div id="course-content">nothing here yet</div></body></html>`)
	if err != nil {
		t.Fatalf("an empty course was reported as unreadable: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("got %v", activities)
	}
}

func TestLinksThatAreNotActivitiesAreIgnored(t *testing.T) {
	page := `<html><body>` +
		`<a href="/course/view.php?id=2">the course</a>` +
		`<a href="/user/view.php?id=4&course=2">a person</a>` +
		`<a href="/mod/assign/view.php?id=9"><span class="instancename">Essay</span></a>` +
		`</body></html>`

	activities, err := webread.ParseCourseActivities(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 || activities[0].CMID != "9" {
		t.Errorf("got %v, want only the activity", activities)
	}
}
