package site_test

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func TestParseResourceURLOnRealMoodleAddresses(t *testing.T) {
	// Every address here was taken from a running Moodle rather than written
	// from memory.
	cases := []struct {
		raw  string
		want site.Resource
	}{
		{
			"http://localhost:8521/mod/assign/view.php?id=4",
			site.Resource{Kind: site.ResourceActivity, Module: "assign", CMID: "4"},
		},
		{
			// The calendar's "do something about it" link.
			"http://localhost:8521/mod/assign/view.php?id=4&action=editsubmission",
			site.Resource{Kind: site.ResourceActivity, Module: "assign", CMID: "4"},
		},
		{
			"http://localhost:8521/mod/forum/view.php?id=1",
			site.Resource{Kind: site.ResourceActivity, Module: "forum", CMID: "1"},
		},
		{
			"https://moodle.example.edu/mod/forum/discuss.php?d=17",
			site.Resource{Kind: site.ResourceDiscussion, Module: "forum", DiscussionID: "17"},
		},
		{
			"https://moodle.example.edu/course/view.php?id=2",
			site.Resource{Kind: site.ResourceCourse, CourseID: "2"},
		},
		{
			"https://moodle.example.edu/user/view.php?id=4&course=2",
			site.Resource{Kind: site.ResourceUser, UserID: "4", CourseID: "2"},
		},
		{
			"https://moodle.example.edu/grade/report/user/index.php?id=2",
			site.Resource{Kind: site.ResourceGrades, CourseID: "2"},
		},
		{
			"http://localhost:8521/calendar/view.php?view=day&course=2&time=1789635573",
			site.Resource{Kind: site.ResourceCalendar, CourseID: "2"},
		},
		{
			"http://localhost:8521/webservice/pluginfile.php/16/mod_assign/introattachment/0/rubric.txt",
			site.Resource{Kind: site.ResourceFile, FileName: "rubric.txt"},
		},
		{
			"https://moodle.example.edu/pluginfile.php/16/mod_assign/introattachment/0/notes.pdf",
			site.Resource{Kind: site.ResourceFile, FileName: "notes.pdf"},
		},
		{
			// A Moodle page this build does not know. Not an error: the
			// address is fine, it is simply for something not covered.
			"https://moodle.example.edu/badges/mybadges.php",
			site.Resource{Kind: site.ResourceUnknown},
		},
	}

	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			got, err := site.ParseResourceURL(c.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestAnchorsAndTrailingJunkDoNotChangeTheAnswer(t *testing.T) {
	// The calendar hands out links with a fragment on the end.
	got, err := site.ParseResourceURL(
		"http://localhost:8521/calendar/view.php?view=day&course=2&time=1789635573#event_3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != site.ResourceCalendar || got.CourseID != "2" {
		t.Errorf("got %+v", got)
	}
}

func TestNonNumericIdsAreDropped(t *testing.T) {
	// Moodle ids are integers. Anything else is a malformed or hand-edited
	// address, and carrying it forward would let it reach a request.
	for _, raw := range []string{
		"https://moodle.example.edu/course/view.php?id=2;DROP TABLE",
		"https://moodle.example.edu/course/view.php?id=../../etc",
		"https://moodle.example.edu/course/view.php?id=-1",
		"https://moodle.example.edu/course/view.php?id=",
	} {
		t.Run(raw, func(t *testing.T) {
			got, err := site.ParseResourceURL(raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CourseID != "" {
				t.Errorf("course id = %q, want it dropped", got.CourseID)
			}
		})
	}
}

func TestAnActivityAddressWithoutAnIdIsRefused(t *testing.T) {
	// Reporting it as an activity with no id would push the problem into
	// whatever tried to use it.
	_, err := site.ParseResourceURL("https://moodle.example.edu/mod/assign/view.php")
	if code := errs.From(err).Code; code != errs.CodeUsage {
		t.Errorf("code = %q, want usage", code)
	}
}

func TestOnlyWebAddressesAreAccepted(t *testing.T) {
	for _, raw := range []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"/mod/assign/view.php?id=4",
		"",
		"   ",
	} {
		if _, err := site.ParseResourceURL(raw); err == nil {
			t.Errorf("%q was accepted", raw)
		}
	}
}

func TestParsingMakesNoAssumptionAboutWhoseSiteItIs(t *testing.T) {
	// Deciding what a link is and deciding whether to fetch it are separate
	// questions. Only the second needs to know whose site it is.
	got, err := site.ParseResourceURL("https://another.school.example/mod/assign/view.php?id=9")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != site.ResourceActivity || got.CMID != "9" {
		t.Errorf("got %+v", got)
	}
}
