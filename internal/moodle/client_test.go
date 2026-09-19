package moodle_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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

func TestTokenEndpointErrorKeepsMoodlesOwnWording(t *testing.T) {
	// /login/token.php reports failure with "error", where the REST endpoint
	// uses "message". Reading only "message" loses the wording Moodle chose
	// and leaves the user with a generic line instead of "Invalid login".
	server := testmoodle.New()
	defer server.Close()

	// The fake server's token endpoint rejects an empty password this way.
	client := newClient(t, server)
	_, _, err := client.LoginToken(context.Background(), "student1", "", moodle.MobileService)
	if err == nil {
		t.Fatal("an invalid login was accepted")
	}
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if !strings.Contains(e.Message, "Invalid login") {
		t.Errorf("Moodle's own wording was lost: %q", e.Message)
	}
	if e.Upstream == nil || e.Upstream.ErrorCode != "invalidlogin" {
		t.Errorf("upstream errorcode not preserved: %+v", e.Upstream)
	}
}

func TestAnExpiredBrowserSessionIsAnAuthenticationProblem(t *testing.T) {
	// Moodle answers a dead session with servicerequireslogin and says so in
	// the message. Leaving it unclassified makes it an "upstream error",
	// which points the user at the server instead of at their own credential:
	// the one thing to do about it is sign in again.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_course_get_courses", []any{})
	server.FailException("core_course_get_courses", "moodle_exception", "servicerequireslogin",
		"Web service is not available. (The session has been logged out or has expired.)")

	var out []any
	err := newClient(t, server).Call(context.Background(), "tok", "core_course_get_courses", nil, &out)
	if err == nil {
		t.Fatal("a dead session was treated as success")
	}
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if e.Upstream == nil || e.Upstream.ErrorCode != "servicerequireslogin" {
		t.Errorf("Moodle's own errorcode was not preserved: %+v", e.Upstream)
	}
}

func TestACourseTheUserIsNotOnIsAPermissionProblem(t *testing.T) {
	// require_login() raises this for an account that holds a valid session
	// but is not on the course — an undergraduate reading a graduate course.
	// It is not a defect in the site, so it must not read as one.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("gradereport_user_get_grade_items", map[string]any{})
	server.FailException("gradereport_user_get_grade_items", "core\\exception\\require_login_exception",
		"requireloginerror", "Course or activity not accessible.")

	var out map[string]any
	err := newClient(t, server).Call(context.Background(), "tok",
		"gradereport_user_get_grade_items", nil, &out)
	if err == nil {
		t.Fatal("a course the account is not on was treated as readable")
	}
	if e := errs.From(err); e.Code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", e.Code)
	}
}

func TestACapabilityTheUserLacksIsAPermissionProblem(t *testing.T) {
	// required_capability_exception carries the singular errorcode
	// "nopermission", which is a different code from the plural
	// "nopermissions" — the plural is classified and this one was not, so a
	// manager reading an assignment of a course they are not enrolled in came
	// back as an upstream fault. The message is Moodle's own untranslated
	// "error/nopermission", which reads as a broken site; the code is what
	// tells the truth, and only if it is classified.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("mod_assign_get_submission_status", map[string]any{})
	server.FailException("mod_assign_get_submission_status",
		"core\\exception\\required_capability_exception",
		"nopermission", "error/nopermission")

	var out map[string]any
	err := newClient(t, server).Call(context.Background(), "tok",
		"mod_assign_get_submission_status", nil, &out)
	if err == nil {
		t.Fatal("a capability the account lacks was treated as readable")
	}
	e := errs.From(err)
	if e.Code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", e.Code)
	}
	if e.Upstream == nil || e.Upstream.ErrorCode != "nopermission" {
		t.Errorf("Moodle's own errorcode was not preserved: %+v", e.Upstream)
	}
}

func TestAGroupTheUserIsNotInIsAPermissionProblem(t *testing.T) {
	// Asking a separate-groups activity about a group the account is not in
	// raises a plain moodle_exception, not required_capability_exception, so
	// none of the capability codes cover it. Unclassified it came back as an
	// upstream fault carrying Moodle's own untranslated "error/notingroup",
	// which points at the site when what is wrong is the group in the request.
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("mod_assign_get_submission_status", map[string]any{})
	server.FailException("mod_assign_get_submission_status",
		"moodle_exception", "notingroup", "error/notingroup")

	var out map[string]any
	err := newClient(t, server).Call(context.Background(), "tok",
		"mod_assign_get_submission_status", nil, &out)
	if err == nil {
		t.Fatal("a group the account is not in was treated as readable")
	}
	e := errs.From(err)
	if e.Code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", e.Code)
	}
	if e.Upstream == nil || e.Upstream.ErrorCode != "notingroup" {
		t.Errorf("Moodle's own errorcode was not preserved: %+v", e.Upstream)
	}
}

func TestAnActivityOnItsWayOutIsNotASiteFault(t *testing.T) {
	// Moodle 接受了刪除、正在處理，所以這個活動再也不會回來。報成上游錯誤
	// 等於建議重試——而重試唯一的結局是它不見了。實測 errorcode 是
	// activityisscheduledfordeletion。
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("mod_assign_get_submission_status", map[string]any{})
	server.FailException("mod_assign_get_submission_status",
		"core\\exception\\moodle_exception",
		"activityisscheduledfordeletion", "Activity deletion in progress...")

	var out map[string]any
	err := newClient(t, server).Call(context.Background(), "tok",
		"mod_assign_get_submission_status", nil, &out)
	if err == nil {
		t.Fatal("an activity being deleted was treated as readable")
	}
	e := errs.From(err)
	if e.Code != errs.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", e.Code)
	}
	if e.Upstream == nil || e.Upstream.ErrorCode != "activityisscheduledfordeletion" {
		t.Errorf("Moodle's own errorcode was not preserved: %+v", e.Upstream)
	}
}

func TestMaintenanceIsTemporaryAndSaidToBe(t *testing.T) {
	// 維護模式會結束。報成上游錯誤，讀的人會去查一個沒有壞的站台；
	// 而 retryable=false 會叫腳本放棄一個十分鐘後就會回來的站台。
	// 實測 Moodle 送的碼是 sitemaintenance（REST 與 login/token.php 都是），
	// 不是我們表裡原本寫的 maintenanceinprogress。
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_enrol_get_users_courses", []any{})
	server.FailException("core_enrol_get_users_courses",
		"core\\exception\\moodle_exception", "sitemaintenance",
		"The site is undergoing maintenance and is currently not available")

	var out []any
	err := newClient(t, server).Call(context.Background(), "tok",
		"core_enrol_get_users_courses", nil, &out)
	if err == nil {
		t.Fatal("a site in maintenance was treated as answering")
	}
	e := errs.From(err)
	if e.Code != errs.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", e.Code)
	}
	if !e.Retryable {
		t.Error("a site that will answer again was reported as not worth retrying")
	}
}

func TestAFunctionTheSiteDoesNotOfferIsNotWorthRetrying(t *testing.T) {
	// retryable 不能從 code 推出來：CodeUnavailable 同時涵蓋「站台停機十分鐘」
	// 與「這個站台根本沒有這支函式」。對後者說可以重試，比什麼都不說更糟。
	server := testmoodle.New()
	defer server.Close()
	server.HandleValue("core_enrol_get_users_courses", []any{})
	server.FailException("core_enrol_get_users_courses",
		"webservice_access_exception", "enablewsdescription",
		"Web services are not enabled")

	var out []any
	err := newClient(t, server).Call(context.Background(), "tok",
		"core_enrol_get_users_courses", nil, &out)
	e := errs.From(err)
	if e == nil || e.Code != errs.CodeUnavailable {
		t.Fatalf("code = %v, want unavailable", e)
	}
	if e.Retryable {
		t.Error("a site that will never offer this was reported as worth retrying")
	}
}

// driftSite serves one hand-written response, for the shapes a real Moodle
// never sends but a network between here and there does.
func driftSite(t *testing.T, handler http.HandlerFunc) *moodle.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return moodle.NewClient(site.Site{BaseURL: base}, moodle.WithHTTPClient(server.Client()))
}

func TestABlockingIntermediaryIsNotACredentialProblem(t *testing.T) {
	// Moodle refuses a web service call with HTTP 200 and a JSON exception,
	// never with a bare 403 page. A 403 carrying markup came from something in
	// front of it — a WAF, a proxy — and "sign in again" sends the reader to
	// fix a credential nothing ever looked at.
	client := driftSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html><head><title>Access Denied</title></head>" +
			"<body>Request blocked. Reference #12.34</body></html>"))
	})

	var out map[string]any
	err := client.Call(context.Background(), "tok", "core_webservice_get_site_info", nil, &out)
	e := errs.From(err)
	if e == nil {
		t.Fatal("a blocked request was treated as an answer")
	}
	if e.Code == errs.CodeAuthentication {
		t.Error("a block by an intermediary was reported as a credential problem")
	}
	if !strings.Contains(e.Hint, "in front of the site") {
		t.Errorf("the hint does not say where the refusal came from: %q", e.Hint)
	}
}

func TestARedirectAwayFromTheEndpointIsNamed(t *testing.T) {
	// A web service endpoint never redirects; Moodle answers it in place. A
	// site behind single sign-on hands the request to a gateway, which
	// answers 200 with its own page — so the status line looks healthy and
	// only the hop gives it away. Reported as "not a Moodle endpoint", the
	// reader would go and check a URL that was right all along.
	client := driftSite(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sso" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign in</body></html>"))
			return
		}
		http.Redirect(w, r, "/sso", http.StatusFound)
	})

	var out map[string]any
	err := client.Call(context.Background(), "tok", "core_webservice_get_site_info", nil, &out)
	e := errs.From(err)
	if e == nil {
		t.Fatal("a sign-in page was accepted as an answer")
	}
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if !strings.Contains(e.Hint, "single sign-on") {
		t.Errorf("the hint does not name what happened: %q", e.Hint)
	}
	if strings.Contains(e.Hint, "not a Moodle web service endpoint") {
		t.Error("a correct URL was blamed")
	}
}

func TestDebuggingOutputBeforeJsonIsNamedNotQuoted(t *testing.T) {
	// A site with display_errors on prepends PHP's own warning, so valid JSON
	// arrives unparseable. Quoting the whole body hands the reader a wall of
	// markup; naming it tells an administrator what to switch off.
	client := driftSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("<br />\n<b>Notice</b>:  Undefined index: q in " +
			"<b>/var/www/html/lib/x.php</b> on line <b>42</b><br />\n{\"sitename\":\"X\"}"))
	})

	var out map[string]any
	err := client.Call(context.Background(), "tok", "core_webservice_get_site_info", nil, &out)
	e := errs.From(err)
	if e == nil {
		t.Fatal("a body with a PHP notice in front of it was decoded")
	}
	if e.Reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q, want protocol_drift", e.Reason)
	}
	if !strings.Contains(e.Hint, "debugging output") {
		t.Errorf("the hint does not name the cause: %q", e.Hint)
	}
	if strings.Contains(e.Hint, "Undefined index") {
		t.Error("the whole body was quoted instead of being named")
	}
}
