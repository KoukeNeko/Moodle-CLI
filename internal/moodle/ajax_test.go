package moodle_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// ajaxSite stands in for a Moodle with no web services, reachable only with a
// browser session.
type ajaxSite struct {
	server *httptest.Server
	// pageHits counts how often the sesskey page was fetched; it should be
	// once per session however many calls are made.
	pageHits int64
	calls    int64
	// available is the function this site offers over AJAX. Anything else gets
	// Moodle's own refusal.
	available string
	cookies   []string
	sesskeys  []string
	// courses is the payload the course function answers with.
	courses string
}

func newAjaxSite(t *testing.T) *ajaxSite {
	t.Helper()
	s := &ajaxSite{
		available: "core_course_get_enrolled_courses_by_timeline_classification",
		// progress 0 with hasprogress false is what a site that does not track
		// progress actually sends.
		courses: `{"courses":[{"id":2,"shortname":"CS204","fullname":"Operating Systems",` +
			`"progress":0,"hasprogress":false,"visible":true}]}`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/my/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&s.pageHits, 1)
		if c, err := r.Cookie(moodle.DefaultSessionCookieName); err != nil || c.Value != "good" {
			// Moodle serves the login page, which carries no sesskey.
			_, _ = w.Write([]byte(`<html><body>log in</body></html>`))
			return
		}
		_, _ = w.Write([]byte(`<html><script>M.cfg = {"sesskey":"abc123","wwwroot":"x"};</script></html>`))
	})
	mux.HandleFunc(moodle.PathAjax, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&s.calls, 1)
		if c, err := r.Cookie(moodle.DefaultSessionCookieName); err == nil {
			s.cookies = append(s.cookies, c.Value)
		}
		s.sesskeys = append(s.sesskeys, r.URL.Query().Get("sesskey"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("info") != s.available {
			_, _ = w.Write([]byte(`[{"error":true,"exception":{"errorcode":"servicenotavailable",` +
				`"message":"Web service is not available."}}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"error":false,"data":` + s.courses + `}]`))
	})
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func (s *ajaxSite) session(t *testing.T, cookie string) *moodle.AjaxSession {
	t.Helper()
	base, err := site.ParseBaseURL(s.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{Name: "nows", BaseURL: base})
	return moodle.NewAjaxSession(client, moodle.SessionCookie{Value: cookie})
}

type courseReply struct {
	Courses []struct {
		ID        int    `json:"id"`
		ShortName string `json:"shortname"`
	} `json:"courses"`
}

func TestASessionReadsDataFromASiteWithNoWebServices(t *testing.T) {
	// The whole reason this route exists: a site that issues no token at all.
	s := newAjaxSite(t)
	var reply courseReply
	err := s.session(t, "good").Call(context.Background(),
		"core_course_get_enrolled_courses_by_timeline_classification",
		map[string]any{"classification": "all"}, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Courses) != 1 || reply.Courses[0].ShortName != "CS204" {
		t.Errorf("courses = %+v", reply.Courses)
	}
	if s.cookies[0] != "good" {
		t.Errorf("the session reached the site as %q", s.cookies[0])
	}
	if s.sesskeys[0] != "abc123" {
		t.Errorf("sesskey = %q", s.sesskeys[0])
	}
}

func TestTheSesskeyIsFetchedOncePerSession(t *testing.T) {
	// It is a CSRF token read from a rendered page; fetching a page before
	// every call would triple the traffic for no reason.
	s := newAjaxSite(t)
	session := s.session(t, "good")
	for i := 0; i < 3; i++ {
		var reply courseReply
		if err := session.Call(context.Background(),
			"core_course_get_enrolled_courses_by_timeline_classification",
			map[string]any{"classification": "all"}, &reply); err != nil {
			t.Fatal(err)
		}
	}
	if s.pageHits != 1 {
		t.Errorf("the sesskey page was fetched %d times, want 1", s.pageHits)
	}
}

func TestAFunctionNotOfferedOverAjaxIsRememberedNotRetried(t *testing.T) {
	// The AJAX endpoint exposes a different and much smaller set than the web
	// service one, and there is no call that lists it. What is available is
	// learned by asking, so the refusals have to stick.
	s := newAjaxSite(t)
	session := s.session(t, "good")

	var junk map[string]any
	err := session.Call(context.Background(), "core_enrol_get_users_courses", nil, &junk)
	if err == nil {
		t.Fatal("a function the site does not offer was reported as working")
	}
	if code := errs.From(err).Code; code != errs.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", code)
	}
	if reason := errs.From(err).Reason; reason != errs.ReasonCapability {
		t.Errorf("reason = %q, want capability", reason)
	}
	if !session.Unavailable("core_enrol_get_users_courses") {
		t.Fatal("the refusal was not remembered")
	}

	before := s.calls
	if err := session.Call(context.Background(), "core_enrol_get_users_courses", nil, &junk); err == nil {
		t.Fatal("the second call was reported as working")
	}
	if s.calls != before {
		t.Errorf("the site was asked again after it had already refused")
	}
}

func TestARejectedSessionIsNotReadAsAMissingSesskey(t *testing.T) {
	// Moodle answers an unrecognised session with the login page, which has no
	// sesskey in it. Reporting that as a parsing problem would send the user
	// looking in the wrong place.
	s := newAjaxSite(t)
	_, err := s.session(t, "stale").Sesskey(context.Background())
	if err == nil {
		t.Fatal("a rejected session was accepted")
	}
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if !strings.Contains(e.Hint, "browser") {
		t.Errorf("the hint does not say where to get a new one: %q", e.Hint)
	}
}

func TestCoursesOverASessionReportTheirProvenance(t *testing.T) {
	// A caller has to be able to tell which route answered: the session route
	// reaches far less, so "this is all your courses" means something
	// different depending on how it was obtained.
	s := newAjaxSite(t)
	result, err := moodle.NewCourseAjaxBackend(s.session(t, "good")).
		List(context.Background(), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provenance.Source != site.BackendAJAX {
		t.Errorf("source = %q, want ajax", result.Provenance.Source)
	}
	if len(result.Courses) != 1 || result.Courses[0].ShortName != "CS204" {
		t.Errorf("courses = %+v", result.Courses)
	}
}

func TestACourseThatTracksNoProgressIsNullNotZero(t *testing.T) {
	// The endpoint sends progress 0 alongside hasprogress false. Reporting
	// that as 0% tells a student they have done none of a course that is not
	// counting.
	s := newAjaxSite(t)
	result, err := moodle.NewCourseAjaxBackend(s.session(t, "good")).
		List(context.Background(), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Courses[0].Progress != nil {
		t.Errorf("progress = %v, want null", *result.Courses[0].Progress)
	}
}

func TestProgressIsKeptWhenTheCourseTracksIt(t *testing.T) {
	s := newAjaxSite(t)
	s.courses = `{"courses":[{"id":2,"shortname":"CS204","fullname":"OS",` +
		`"progress":42,"hasprogress":true,"visible":true}]}`
	result, err := moodle.NewCourseAjaxBackend(s.session(t, "good")).
		List(context.Background(), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Courses[0].Progress == nil || *result.Courses[0].Progress != 42 {
		t.Errorf("progress = %v, want 42", result.Courses[0].Progress)
	}
}
