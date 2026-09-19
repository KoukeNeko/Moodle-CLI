package cli_test

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// actionEvent builds one of Moodle's action events. The real reply carries a
// good deal more, including links with the caller's session key in them, which
// is why one is included here.
func actionEvent(id int, name string, at time.Time, overdue bool, extra map[string]any) map[string]any {
	event := map[string]any{
		"id": id, "name": name, "activityname": strings.TrimSuffix(name, " is due"),
		"modulename": "assign", "eventtype": "due",
		"instance": id + 1, "timesort": at.Unix(), "overdue": overdue,
		"url":     "https://moodle.example.edu/mod/assign/view.php?id=" + strconv.Itoa(id+1),
		"editurl": "https://moodle.example.edu/course/mod.php?update=2&sesskey=Hb7fBMHo4n",
		"course":  map[string]any{"id": 2, "shortname": "CS204"},
		"action": map[string]any{
			"name":       "Add submission",
			"url":        "https://moodle.example.edu/mod/assign/view.php?id=2&action=editsubmission",
			"itemcount":  1,
			"actionable": true,
		},
	}
	for k, v := range extra {
		event[k] = v
	}
	return event
}

func (f *fixture) withEvents(events ...map[string]any) {
	f.t.Helper()
	f.server.HandleValue(moodle.FunctionActionEvents, map[string]any{
		"events": toAny(events), "firstid": 1, "lastid": len(events),
	})
}

func calendarEvents(t *testing.T, stdout string) []v1.CalendarEvent {
	t.Helper()
	var doc struct {
		Data []v1.CalendarEvent `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Data
}

func TestOverdueWorkIsListedAndSaidPlainly(t *testing.T) {
	// The whole point of the list. Filtering out work whose deadline has
	// passed would make it look tidier and leave out the part that matters.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withEvents(
		actionEvent(3, "A3 statement is due", time.Now().AddDate(0, 0, -1), true, nil),
		actionEvent(1, "A1 direct submit is due", time.Now().AddDate(0, 0, 7), false, nil),
	)

	stdout, stderr, code := f.run("calendar", "upcoming", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "calendar.upcoming", stdout)

	events := calendarEvents(t, stdout)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 — overdue work must not be dropped", len(events))
	}
	if !events[0].Overdue {
		t.Error("the overdue event is not marked overdue")
	}

	human, _, _ := f.run("calendar", "upcoming")
	if !strings.Contains(human, "OVERDUE") {
		t.Errorf("the human output does not call out overdue work:\n%s", human)
	}
	if !strings.Contains(human, "past their deadline") {
		t.Errorf("the human output does not count the overdue work:\n%s", human)
	}
}

func TestTheWindowIsNotStartedAtNow(t *testing.T) {
	// Setting timesortfrom to now would silently drop everything overdue, so
	// the request must not carry it.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withEvents(actionEvent(1, "A1 is due", time.Now(), false, nil))

	if _, _, code := f.run("calendar", "upcoming"); code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	var sent url.Values
	for _, request := range f.server.Requests() {
		if request.Function == moodle.FunctionActionEvents {
			sent = request.Params
		}
	}
	if got := sent.Get("timesortfrom"); got != "" {
		t.Errorf("timesortfrom = %q; setting it hides overdue work", got)
	}
	if sent.Get("timesortto") == "" {
		t.Error("--days did not reach the site as timesortto")
	}
}

func TestALinkCarryingASessionKeyIsNeverPassedOn(t *testing.T) {
	// Moodle's calendar hands out edit links with the caller's sesskey in
	// them. Anything reaching the JSON contract can be logged and kept, so a
	// session key must not travel with it.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withEvents(actionEvent(1, "A1 is due", time.Now(), false, map[string]any{
		"url": "https://moodle.example.edu/mod/assign/view.php?id=2&sesskey=Hb7fBMHo4n",
	}))

	stdout, _, code := f.run("calendar", "upcoming", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "sesskey") {
		t.Fatalf("a session key reached the output:\n%s", stdout)
	}
	if url := calendarEvents(t, stdout)[0].URL; url != nil {
		t.Errorf("url = %q; a link carrying a secret should be dropped, not cleaned", *url)
	}
}

func TestTheEventCmidIsTheCourseModuleNotTheActivityId(t *testing.T) {
	// Moodle sends the course module id in a field called "instance", which
	// everywhere else means the activity's own id. Both are small integers, so
	// getting it wrong links to a different activity and nothing looks amiss.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withEvents(actionEvent(1, "A1 is due", time.Now(), false, map[string]any{
		"instance": 2,
	}))

	stdout, _, code := f.run("calendar", "upcoming", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if got := calendarEvents(t, stdout)[0].CMID; got != "2" {
		t.Errorf("cmid = %q, want 2", got)
	}
}

func TestAnEmptyCalendarSaysSo(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withEvents()

	stdout, _, code := f.run("calendar", "upcoming", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "calendar.upcoming", stdout)
	if events := calendarEvents(t, stdout); len(events) != 0 {
		t.Fatalf("got %d events", len(events))
	}

	// 空白的表格看起來像壞掉，所以要有一句話。但那句話不能是「沒有事情要做」：
	// 行事曆回的是**這個帳號看得到的**東西，隱藏課程或分組限制擋掉的截止日
	// 會從同一個空清單裡消失。對學生來說，這是唯一一個會讓他丟掉分數的錯答案。
	human, _, _ := f.run("calendar", "upcoming")
	if human == "" {
		t.Error("an empty calendar printed nothing at all")
	}
	if strings.Contains(human, "Nothing to do") {
		t.Errorf("a filtered calendar was reported as an empty one:\n%s", human)
	}
	if !strings.Contains(human, "this account can see") {
		t.Errorf("the answer does not say whose view it is:\n%s", human)
	}
}
