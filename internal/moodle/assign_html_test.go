package moodle_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// pageSite stands in for a Moodle reached only with a browser session: its
// pages and the few AJAX functions a session may call.
type pageSite struct {
	server *httptest.Server
	// monthCalls counts month-view reads.
	monthCalls int64
	// calendarDown makes the month view refuse, as a site might.
	calendarDown bool
}

const (
	// dueAt is a deadline at 23:59 in Taipei on 5 October 2026.
	dueAt = int64(1791215940)
	// termStart and termEnd are the course's own dates.
	termStart = int64(1785513600) // 2026-08-01 Taipei
	termEnd   = int64(1801324800) // 2027-01-31 Taipei
)

func newPageSite(t *testing.T) *pageSite {
	t.Helper()
	s := &pageSite{}
	coursePage := `<html><body><ul>` +
		`<li class="activity modtype_assign"><div class="activityname">` +
		`<a href="/mod/assign/view.php?id=1436182"><span class="instancename">Practice #1` +
		`<span class="accesshide"> 作業</span></span></a></div></li>` +
		`<li class="activity modtype_assign"><div class="activityname">` +
		`<a href="/mod/assign/view.php?id=1436183"><span class="instancename">Bonus</span></a></div></li>` +
		`<li class="activity modtype_quiz"><div class="activityname">` +
		`<a href="/mod/quiz/view.php?id=1436146"><span class="instancename">gprof</span></a></div></li>` +
		`</ul></body></html>`
	assignPage, err := os.ReadFile("../webread/testdata/assign-page.html")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/my/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><script>M.cfg = {"sesskey":"abc123","userId":4};</script></html>`)
	})
	mux.HandleFunc("/course/view.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, coursePage)
	})
	mux.HandleFunc("/mod/assign/view.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(assignPage)
	})
	mux.HandleFunc("/mod/quiz/view.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body class="path-mod-quiz course-36728 cmid-1436146">`+
			`<div data-region="activity-information" data-activityname="gprof"></div>`+
			`<div class="box quizinfo"><p>允許作答幾次： 1</p></div></body></html>`)
	})
	mux.HandleFunc(moodle.PathAjax, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var calls []struct {
			Method string         `json:"methodname"`
			Args   map[string]any `json:"args"`
		}
		_ = json.NewDecoder(r.Body).Decode(&calls)
		switch calls[0].Method {
		case moodle.FunctionTimelineCourses:
			fmt.Fprintf(w, `[{"error":false,"data":{"courses":[{"id":36728,"shortname":"115_1_CS204",`+
				`"fullname":"OS","startdate":%d,"enddate":%d,"visible":true}],"nextoffset":0}}]`,
				termStart, termEnd)
		case moodle.FunctionCalendarMonth:
			atomic.AddInt64(&s.monthCalls, 1)
			if s.calendarDown {
				_, _ = io.WriteString(w, `[{"error":true,"exception":{"message":"x","errorcode":"servicenotavailable"}}]`)
				return
			}
			events := ""
			if calls[0].Args["month"] == float64(10) && calls[0].Args["year"] == float64(2026) {
				events = fmt.Sprintf(`{"modulename":"assign","instance":1436182,"eventtype":"due","timestart":%d},`+
					`{"modulename":"quiz","instance":1436182,"eventtype":"close","timestart":1},`+
					`{"modulename":"quiz","instance":1436146,"eventtype":"open","timestart":%d},`+
					`{"modulename":"quiz","instance":1436146,"eventtype":"close","timestart":%d}`,
					dueAt, dueAt-3600, dueAt)
			}
			fmt.Fprintf(w, `[{"error":false,"data":{"weeks":[{"days":[{"events":[%s]}]}]}}]`, events)
		default:
			_, _ = io.WriteString(w, `[{"error":true,"exception":{"message":"x","errorcode":"servicenotavailable"}}]`)
		}
	})
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func (s *pageSite) context(t *testing.T) (*moodle.PageReader, *moodle.AjaxSession,
	func(context.Context) ([]course.Summary, error)) {
	t.Helper()
	base, err := site.ParseBaseURL(s.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{BaseURL: base})
	cookie := moodle.SessionCookie{Value: "good"}
	ajax := moodle.NewAjaxSession(client, cookie)
	return moodle.NewPageReader(client, cookie), ajax,
		func(ctx context.Context) ([]course.Summary, error) {
			result, err := moodle.NewCourseAjaxBackend(ajax).List(ctx, course.ListQuery{})
			return result.Courses, err
		}
}

func (s *pageSite) backend(t *testing.T) *moodle.AssignHTMLBackend {
	t.Helper()
	base, err := site.ParseBaseURL(s.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{BaseURL: base})
	cookie := moodle.SessionCookie{Value: "good"}
	ajax := moodle.NewAjaxSession(client, cookie)
	return moodle.NewAssignHTMLBackend(moodle.NewPageReader(client, cookie), ajax,
		func(ctx context.Context) ([]course.Summary, error) {
			result, err := moodle.NewCourseAjaxBackend(ajax).List(ctx, course.ListQuery{})
			return result.Courses, err
		})
}

func TestReadingPagesNamesTheCourseAndTheDeadline(t *testing.T) {
	// A course page names each assignment and nothing else. The course's
	// short name comes from the course listing and the deadline from the
	// calendar, which states it as a number rather than as prose in the
	// site's language.
	s := newPageSite(t)
	result, err := s.backend(t).List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assignments) != 2 {
		t.Fatalf("got %d assignments, want 2", len(result.Assignments))
	}
	first := result.Assignments[0]
	if first.Name != "Practice #1" || first.CourseShortName == nil || *first.CourseShortName != "115_1_CS204" {
		t.Errorf("assignment = %+v", first)
	}
	if first.DueDate == nil || first.DueDate.Unix() != dueAt {
		t.Errorf("due = %v, want %v", first.DueDate, time.Unix(dueAt, 0))
	}
	// An assignment with no due event has no deadline, and the calendar
	// was read, so null is an answer here rather than a gap.
	if result.Assignments[1].DueDate != nil {
		t.Errorf("an assignment without a due event got %v", result.Assignments[1].DueDate)
	}
	if !result.Provenance.Partial {
		t.Error("a page-read listing must say it is partial")
	}
	for _, field := range []string{"needs_hand_in", "submission_plugins", "cut_off_date"} {
		if !containsField(result.Provenance.Missing, field) {
			t.Errorf("%s should be listed as missing: %v", field, result.Provenance.Missing)
		}
	}
	if containsField(result.Provenance.Missing, "due_date") {
		t.Errorf("due_date was read, but is listed as missing: %v", result.Provenance.Missing)
	}
	// The course runs August to January; a month either side is ten reads,
	// not one per month since the epoch.
	if calls := atomic.LoadInt64(&s.monthCalls); calls < 6 || calls > 10 {
		t.Errorf("read %d months for a six-month course", calls)
	}
}

func TestWithoutTheCalendarADeadlineIsMissingNotNone(t *testing.T) {
	// A null due date from a route that could not read the calendar would
	// tell a student there is no deadline.
	s := newPageSite(t)
	s.calendarDown = true
	result, err := s.backend(t).List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsField(result.Provenance.Missing, "due_date") {
		t.Errorf("due_date should be missing: %v", result.Provenance.Missing)
	}
}

func TestShowReadsTheAssignmentFromItsPage(t *testing.T) {
	s := newPageSite(t)
	detail, err := s.backend(t).Show(context.Background(), "1436182")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Name != "Practice #1" || detail.CourseID != "36728" {
		t.Errorf("detail = %+v", detail.Summary)
	}
	if !strings.Contains(detail.Description, "Practice: gprof") {
		t.Errorf("description = %q", detail.Description)
	}
	if len(detail.Attachments) != 1 || detail.Attachments[0].URL == "" {
		t.Errorf("attachments = %+v", detail.Attachments)
	}
	if detail.DueDate == nil || detail.DueDate.Unix() != dueAt {
		t.Errorf("due = %v", detail.DueDate)
	}
	if !detail.Provenance.Partial || !containsField(detail.Provenance.Missing, "max_grade") {
		t.Errorf("provenance = %+v", detail.Provenance)
	}
}

func containsField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func TestForumThreadsAreReadFromTheForumPage(t *testing.T) {
	// The AJAX endpoint does not list a forum's threads to a browser
	// session; the forum's own page does.
	page, err := os.ReadFile("../webread/testdata/forum.html")
	if err != nil {
		t.Fatal(err)
	}
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("p"))
		_, _ = w.Write(page)
	}))
	defer server.Close()
	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{BaseURL: base})
	backend := moodle.NewForumHTMLBackend(
		moodle.NewPageReader(client, moodle.SessionCookie{Value: "good"}), nil)

	result, err := backend.Discussions(context.Background(), "1325022")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Discussions) != 2 || result.Discussions[0].ID != "260280" {
		t.Fatalf("discussions = %+v", result.Discussions)
	}
	// No paging bar, so one page is the whole forum.
	if len(asked) != 1 || asked[0] != "0" {
		t.Errorf("pages asked for: %v", asked)
	}
	// A reply count this route cannot read must not arrive as zero.
	if !result.Provenance.Partial || !containsField(result.Provenance.Missing, "replies") {
		t.Errorf("provenance = %+v", result.Provenance)
	}
}

func TestQuizzesAreReadFromPagesWithDatesFromTheCalendar(t *testing.T) {
	// The quiz functions are not offered over AJAX. The course page names the
	// quiz; the calendar says when it opens and closes; the settings and the
	// attempts are printed only as sentences in the site's language.
	s := newPageSite(t)
	backend := moodle.NewQuizHTMLBackend(s.context(t))

	list, err := backend.List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Quizzes) != 1 || list.Quizzes[0].Name != "gprof" {
		t.Fatalf("quizzes = %+v", list.Quizzes)
	}
	item := list.Quizzes[0]
	if item.Opens == nil || item.Opens.Unix() != dueAt-3600 || item.Closes == nil || item.Closes.Unix() != dueAt {
		t.Errorf("opens %v closes %v", item.Opens, item.Closes)
	}
	if item.MaxAttempts != nil || !containsField(list.Provenance.Missing, "max_attempts") {
		t.Errorf("max attempts should be unknown and listed missing: %v %v",
			item.MaxAttempts, list.Provenance.Missing)
	}

	detail, err := backend.Show(context.Background(), "1436146")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Name != "gprof" || detail.CourseID != "36728" || detail.Closes == nil {
		t.Errorf("detail = %+v", detail.Quiz)
	}
	// Nil, not empty: an empty list would say the account never tried it.
	if detail.Attempts != nil || !containsField(detail.Provenance.Missing, "attempts") {
		t.Errorf("attempts = %v, missing = %v", detail.Attempts, detail.Provenance.Missing)
	}
}
