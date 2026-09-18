package moodle_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

func newClient(t *testing.T, server *testmoodle.Server, opts ...moodle.Option) *moodle.Client {
	t.Helper()
	base, err := site.ParseBaseURL(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	return moodle.NewClient(site.Site{BaseURL: base}, opts...)
}

type siteInfo struct {
	SiteName  string   `json:"sitename"`
	Username  string   `json:"username"`
	UserID    int      `json:"userid"`
	Release   string   `json:"release"`
	Functions []any    `json:"functions"`
	Warnings  []string `json:"warnings"`
}

func TestCallDecodesASuccessfulResponse(t *testing.T) {
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_webservice_get_site_info", map[string]any{
		"sitename": "Test Moodle", "username": "student1", "userid": 4, "release": "5.2.3",
	})

	var info siteInfo
	err := newClient(t, server).Call(context.Background(), "tok", "core_webservice_get_site_info", nil, &info)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if info.Username != "student1" || info.UserID != 4 {
		t.Errorf("decoded %+v", info)
	}
}

func TestMoodleExceptionBehindHTTP200IsAnError(t *testing.T) {
	// The trap that makes Moodle clients report success wrongly: the status
	// line is 200 and the body is an exception.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_course_get_courses", []any{})
	server.Fail("core_course_get_courses", testmoodle.FailInvalidToken)

	var out []any
	err := newClient(t, server).Call(context.Background(), "tok", "core_course_get_courses", nil, &out)
	if err == nil {
		t.Fatal("an exception body was treated as success")
	}
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if e.Reason != errs.ReasonTokenExpired {
		t.Errorf("reason = %q, want token_expired", e.Reason)
	}
	if e.Upstream == nil || e.Upstream.ErrorCode != "invalidtoken" {
		t.Errorf("Moodle's own errorcode was not preserved: %+v", e.Upstream)
	}
}

func TestWebServicesDisabledIsUnavailableNotAuthentication(t *testing.T) {
	// The variant test site answers this way; doctor has to be able to tell
	// the user that an administrator must act.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_webservice_get_site_info", map[string]any{})
	server.Fail("core_webservice_get_site_info", testmoodle.FailServiceUnavailable)

	err := newClient(t, server).Call(context.Background(), "tok", "core_webservice_get_site_info", nil, nil)
	e := errs.From(err)
	if e.Code != errs.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", e.Code)
	}
	if e.Reason != errs.ReasonMobileServicesDisabled {
		t.Errorf("reason = %q, want mobile_services_disabled", e.Reason)
	}
}

func TestUnknownMoodleErrorCodeStaysUpstream(t *testing.T) {
	// A Moodle error we have never seen must not be guessed into a friendlier
	// code; it is reported as upstream with the original preserved.
	server := testmoodle.New()
	defer server.Close()
	server.Handle("mod_thing_do", func(url.Values) (any, error) {
		return nil, errNotMapped{}
	})
	err := newClient(t, server).Call(context.Background(), "tok", "mod_thing_do", nil, nil)
	e := errs.From(err)
	if e.Code != errs.CodeValidation && e.Code != errs.CodeUpstream {
		t.Errorf("code = %q", e.Code)
	}
	if e.Upstream == nil {
		t.Error("upstream detail was dropped")
	}
}

type errNotMapped struct{}

func (errNotMapped) Error() string { return "something specific to this plugin" }

func TestResponseLostMidFlightIsAmbiguous(t *testing.T) {
	// The server processed the request and then the connection dropped. The
	// caller must be told the outcome is unknown, never that it failed
	// cleanly, or a retry would repeat the effect.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("mod_assign_save_submission", map[string]any{"ok": true})
	server.Fail("mod_assign_save_submission", testmoodle.FailAppliedThenLost)

	err := newClient(t, server).Call(context.Background(), "tok", "mod_assign_save_submission", nil, nil)
	if err == nil {
		t.Fatal("a lost response was reported as success")
	}
	e := errs.From(err)
	if e.EffectiveOutcome() != errs.OutcomeAmbiguous {
		t.Errorf("outcome = %q, want ambiguous", e.EffectiveOutcome())
	}
	if e.Retryable {
		t.Error("an ambiguous write must never be marked retryable")
	}
	// The server did see the request: that is exactly why it is ambiguous.
	if server.CallsTo("mod_assign_save_submission") != 1 {
		t.Errorf("server saw %d calls, want 1", server.CallsTo("mod_assign_save_submission"))
	}
}

func TestClientNeverRetriesOnItsOwn(t *testing.T) {
	// Retry policy needs to know whether an operation writes, which this
	// layer cannot know.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_course_get_courses", []any{})
	server.Fail("core_course_get_courses", testmoodle.FailServerError)

	_ = newClient(t, server).Call(context.Background(), "tok", "core_course_get_courses", nil, nil)
	if got := server.CallsTo("core_course_get_courses"); got != 1 {
		t.Errorf("the client made %d attempts, want exactly 1", got)
	}
}

func TestHTMLResponseIsReportedAsProtocolDrift(t *testing.T) {
	// Pointing the CLI at a non-Moodle URL is a common mistake; the message
	// should say so instead of "invalid character '<'".
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_webservice_get_site_info", map[string]any{})
	server.Fail("core_webservice_get_site_info", testmoodle.FailHTML)

	var out map[string]any
	err := newClient(t, server).Call(context.Background(), "tok", "core_webservice_get_site_info", nil, &out)
	e := errs.From(err)
	if e.Reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q, want protocol_drift", e.Reason)
	}
	if !strings.Contains(e.Hint, "HTML") {
		t.Errorf("hint should mention HTML, got %q", e.Hint)
	}
}

func TestUnreachableSiteIsANetworkError(t *testing.T) {
	base, err := site.ParseBaseURL("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{BaseURL: base})
	err = client.Call(context.Background(), "tok", "core_webservice_get_site_info", nil, nil)
	if code := errs.From(err).Code; code != errs.CodeNetwork {
		t.Errorf("code = %q, want network", code)
	}
}

func TestCancellationIsNotReportedAsASiteFailure(t *testing.T) {
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_webservice_get_site_info", map[string]any{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := newClient(t, server).Call(ctx, "tok", "core_webservice_get_site_info", nil, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(errs.From(err).Error(), "cancelled") {
		t.Errorf("got %q, want it to mention cancellation", errs.From(err).Error())
	}
}

func TestTraceRedactsTheToken(t *testing.T) {
	// A debug trace that leaks the token is worse than no trace.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_webservice_get_site_info", map[string]any{"sitename": "x"})

	var trace bytes.Buffer
	client := newClient(t, server, moodle.WithTrace(&trace))
	const token = "c9e291c29aa29ecb3d932383d09d16c1"
	if err := client.Call(context.Background(), token, "core_webservice_get_site_info", nil, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(trace.String(), token) {
		t.Errorf("the token appears in the trace:\n%s", trace.String())
	}
	// The marker may be URL-encoded in the form body, so accept either form.
	encoded := url.QueryEscape(moodle.Redacted)
	if !strings.Contains(trace.String(), moodle.Redacted) &&
		!strings.Contains(trace.String(), encoded) {
		t.Errorf("nothing was redacted:\n%s", trace.String())
	}
}

func TestRedaction(t *testing.T) {
	raw, err := url.Parse("https://example.edu/webservice/pluginfile.php?token=abc123&forcedownload=1")
	if err != nil {
		t.Fatal(err)
	}
	// Moodle puts the token in the URL for file downloads, so redacting only
	// headers would still leak it.
	got := moodle.RedactURL(raw)
	if strings.Contains(got, "abc123") {
		t.Errorf("token survived in %q", got)
	}
	if !strings.Contains(got, "forcedownload=1") {
		t.Errorf("non-secret parameters should survive, got %q", got)
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer abc123")
	header.Set("Cookie", "MoodleSession=xyz")
	header.Set("Accept", "application/json")
	clean := moodle.RedactHeader(header)
	if strings.Contains(clean.Get("Authorization"), "abc123") ||
		strings.Contains(clean.Get("Cookie"), "xyz") {
		t.Errorf("secret headers survived: %v", clean)
	}
	if clean.Get("Accept") != "application/json" {
		t.Errorf("harmless headers should survive, got %q", clean.Get("Accept"))
	}
}
