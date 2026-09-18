package cli_test

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/api"
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

// fixture wires the CLI to a fake Moodle with an isolated config and keychain.
type fixture struct {
	t      *testing.T
	server *testmoodle.Server
	deps   cli.Deps
	// functions is what the fake site says it offers, so a test can check the
	// reported count against it instead of against a number that goes stale
	// the moment a feature is added.
	functions []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	server := testmoodle.New()
	t.Cleanup(server.Close)

	manager := testManager()
	f := &fixture{
		t:      t,
		server: server,
		deps: cli.Deps{
			ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
			Auth:       manager,
			Login:      testCoordinator(manager),
			Courses: func(session *auth.Session, capabilities *site.Capabilities) *course.Service {
				return course.NewService(
					moodle.NewCourseBackend(session.Client(), session.Token(), capabilities),
				)
			},
			Assignments: func(session *auth.Session, _ *site.Capabilities, mode safety.Mode) *assignment.Service {
				return assignment.NewService(
					moodle.NewAssignmentBackend(session.Client(), session.Token()),
					moodle.NewAssignmentWriter(session.Client(), session.Token()),
					mode,
				)
			},
			Grades: func(session *auth.Session, capabilities *site.Capabilities) *grade.Service {
				return grade.NewService(
					moodle.NewGradeBackend(session.Client(), session.Token(), capabilities),
				)
			},
			Calendar: func(session *auth.Session, _ *site.Capabilities) *calendar.Service {
				return calendar.NewService(
					moodle.NewCalendarBackend(session.Client(), session.Token()),
				)
			},
			API: func(session *auth.Session, mode safety.Mode, allowWrite bool) *api.Service {
				return api.NewService(
					moodle.NewRawCaller(session.Client(), session.Token()),
					mode, allowWrite,
				)
			},
			// No terminal: a test must never be able to answer a prompt by
			// accident, so confirmation has to be explicit.
			Interactive: func() bool { return false },
		},
	}
	// A working site by default; individual tests break what they need to.
	f.functions = []string{
		moodle.FunctionUserCourses,
		"mod_assign_get_assignments",
		moodle.FunctionGradeItems,
		moodle.FunctionCourseGrades,
		moodle.FunctionActionEvents,
	}
	declared := make([]any, 0, len(f.functions))
	for _, name := range f.functions {
		declared = append(declared, map[string]any{"name": name, "version": "2026091800"})
	}
	server.HandleValue(moodle.FunctionSiteInfo, map[string]any{
		"sitename": "Test Moodle", "username": "student1", "firstname": "Sam",
		"lastname": "Student", "userid": 4, "release": "5.2.3",
		"downloadfiles": 1, "uploadfiles": 1,
		"functions": declared,
	})
	server.AddToken("good-token")
	return f
}

func (f *fixture) run(args ...string) (stdout, stderr string, code int) {
	f.t.Helper()
	return runWith(f.t, f.deps, args...)
}

func (f *fixture) runWithStdin(stdin string, args ...string) (stdout, stderr string, code int) {
	f.t.Helper()
	return runWithInput(f.t, f.deps, stdin, args...)
}

// addSiteAndLogin gets the fixture to the signed-in state most tests need.
func (f *fixture) addSiteAndLogin() {
	f.t.Helper()
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		f.t.Fatalf("site add: exit %d, %s", code, stderr)
	}
	if _, stderr, code := f.run("auth", "login", "--token", "good-token"); code != 0 {
		f.t.Fatalf("auth login: exit %d, %s", code, stderr)
	}
}

func TestSiteAddThenList(t *testing.T) {
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	stdout, _, code := f.run("site", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "site.list", stdout)
	if !strings.Contains(stdout, `"name":"school"`) {
		t.Errorf("site missing from %s", stdout)
	}
	// The first site added becomes current, so a single-site user never has
	// to run `site use`.
	if !strings.Contains(stdout, `"current":true`) {
		t.Errorf("first site should be current: %s", stdout)
	}
}

func TestLoginVerifiesTheTokenBeforeStoringIt(t *testing.T) {
	// Storing a credential that does not work would make every later command
	// fail with a confusing error instead of the real one.
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	_, _, code := f.run("auth", "login", "--token", "wrong-token")
	if code != v1.ExitAuthentication {
		t.Fatalf("exit %d, want %d", code, v1.ExitAuthentication)
	}
	// Nothing was stored, so status must still say there is no account.
	_, _, statusCode := f.run("auth", "status")
	if statusCode != v1.ExitAuthentication {
		t.Errorf("auth status exit %d, want %d", statusCode, v1.ExitAuthentication)
	}
}

func TestLoginThenStatus(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()

	stdout, _, code := f.run("auth", "status", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "auth.status", stdout)
	if !strings.Contains(stdout, `"valid":true`) {
		t.Errorf("status should report a valid credential: %s", stdout)
	}
	if !strings.Contains(stdout, `"user_id":"4"`) {
		t.Errorf("ids must be strings in the contract: %s", stdout)
	}
}

func TestSiteInspectReportsCapabilities(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()

	stdout, _, code := f.run("site", "inspect", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "site.inspect", stdout)

	var doc struct {
		Data struct {
			FunctionCount int `json:"function_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Data.FunctionCount != len(f.functions) {
		t.Errorf("function_count = %d, want %d", doc.Data.FunctionCount, len(f.functions))
	}
}

func TestDoctorReportsMobileServicesDisabled(t *testing.T) {
	// This is the acceptance criterion for the variant test site: the user
	// must be told an administrator has to act, not that the network failed.
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionPublicConfig, map[string]any{
		"sitename": "Test Moodle", "enablewebservices": 1, "enablemobilewebservice": 0,
	})
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}

	stdout, _, code := f.run("doctor", "--json")
	if code != v1.ExitUnavailable {
		t.Fatalf("exit %d, want %d", code, v1.ExitUnavailable)
	}
	validate(t, "doctor", stdout)
	if !strings.Contains(stdout, "mobile_services_disabled") {
		t.Errorf("doctor did not name the cause:\n%s", stdout)
	}
	if !strings.Contains(stdout, "administrator") {
		t.Errorf("doctor should say who can fix it:\n%s", stdout)
	}
}

func TestDoctorOnAHealthySite(t *testing.T) {
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionPublicConfig, map[string]any{
		"sitename": "Test Moodle", "enablewebservices": 1, "enablemobilewebservice": 1,
		"typeoflogin": 2, "showloginform": 1, "tool_mobile_qrcodetype": 2,
	})
	f.addSiteAndLogin()

	stdout, _, code := f.run("doctor", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	validate(t, "doctor", stdout)
	if !strings.Contains(stdout, `"ok":true`) {
		t.Errorf("doctor should pass on a healthy site:\n%s", stdout)
	}
	// A site that does not expose forums is not broken; that must be a
	// warning, never a failure.
	if strings.Contains(stdout, `"status":"failed"`) {
		t.Errorf("a missing optional feature must not fail the report:\n%s", stdout)
	}
}

func TestDoctorSeparatesUnreachableFromMisconfigured(t *testing.T) {
	// Both look like "it does not work" to a user; the report has to tell
	// them apart, which is the entire point of doctor.
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "dead", "http://127.0.0.1:1"); code != 0 {
		t.Fatal("site add failed")
	}
	stdout, _, code := f.run("doctor", "--site", "dead", "--json")
	if code != v1.ExitUnavailable {
		t.Fatalf("exit %d", code)
	}
	validate(t, "doctor", stdout)
	if !strings.Contains(stdout, `"name":"Site reachable"`) ||
		!strings.Contains(stdout, `"status":"failed"`) {
		t.Errorf("the unreachable site was not reported as such:\n%s", stdout)
	}
	// Checks that could not run must say so rather than claim to have passed.
	if !strings.Contains(stdout, `"status":"skipped"`) {
		t.Errorf("dependent checks should be skipped, not silently omitted:\n%s", stdout)
	}
}

func TestLogoutRemovesTheCredentialButNotTheMoodleToken(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()

	stdout, _, code := f.run("auth", "logout")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	// The wording matters: the token is often shared with the phone app, so
	// the user must know it still works elsewhere.
	if !strings.Contains(stdout, "not revoked") {
		t.Errorf("logout should say the token was not revoked:\n%s", stdout)
	}
	if _, _, statusCode := f.run("auth", "status"); statusCode != v1.ExitAuthentication {
		t.Errorf("status after logout: exit %d, want %d", statusCode, v1.ExitAuthentication)
	}
}

func TestExpiredTokenIsAuthenticationNotInternal(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.Fail(moodle.FunctionSiteInfo, testmoodle.FailInvalidToken)

	stdout, _, code := f.run("auth", "status", "--json")
	if code != v1.ExitAuthentication {
		t.Fatalf("exit %d, want %d:\n%s", code, v1.ExitAuthentication, stdout)
	}
	validate(t, "error", stdout)
	if !strings.Contains(stdout, "token_expired") {
		t.Errorf("the reason should say the token expired:\n%s", stdout)
	}
}

func TestSiteRemoveNeedsConfirmationWhenAccountsExist(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()

	_, _, code := f.run("site", "remove", "school")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if _, _, code := f.run("site", "remove", "school", "--yes"); code != v1.ExitOK {
		t.Fatalf("exit %d with --yes", code)
	}
	stdout, _, _ := f.run("site", "list", "--json")
	if strings.Contains(stdout, "school") {
		t.Errorf("site survived removal: %s", stdout)
	}
}

// courseFixture adds a course listing to the fake site.
func (f *fixture) withCourses(courses ...map[string]any) {
	f.t.Helper()
	list := make([]any, 0, len(courses))
	for _, item := range courses {
		list = append(list, item)
	}
	f.server.HandleValue(moodle.FunctionUserCourses, list)
}

func TestCourseListJSONSatisfiesSchema(t *testing.T) {
	f := newFixture(t)
	f.withCourses(
		map[string]any{"id": 2, "shortname": "CS204", "fullname": "Operating Systems",
			"startdate": 1757894400, "enddate": 0, "visible": 1},
	)
	f.addSiteAndLogin()

	stdout, _, code := f.run("course", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	validate(t, "course.list", stdout)
	if !strings.Contains(stdout, `"id":"2"`) {
		t.Errorf("ids must be strings in the contract:\n%s", stdout)
	}
}

func TestCourseListTreatsMoodleZeroDateAsNullNot1970(t *testing.T) {
	// Moodle sends 0 for "no end date". Rendering that as 1970-01-01 would be
	// a wrong answer that looks like a real one.
	f := newFixture(t)
	f.withCourses(
		map[string]any{"id": 2, "shortname": "CS204", "fullname": "Operating Systems",
			"startdate": 1757894400, "enddate": 0, "visible": 1},
	)
	f.addSiteAndLogin()

	stdout, _, code := f.run("course", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, `"end_date":null`) {
		t.Errorf("an unset end date must be null:\n%s", stdout)
	}
	if strings.Contains(stdout, "1970") {
		t.Errorf("Moodle's 0 leaked through as an epoch date:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"start_date":"2025-09-15T`) {
		t.Errorf("a real start date should be RFC 3339 UTC:\n%s", stdout)
	}
}

func TestCourseListPagination(t *testing.T) {
	f := newFixture(t)
	f.withCourses(
		map[string]any{"id": 1, "shortname": "A", "fullname": "A", "visible": 1},
		map[string]any{"id": 2, "shortname": "B", "fullname": "B", "visible": 1},
		map[string]any{"id": 3, "shortname": "C", "fullname": "C", "visible": 1},
	)
	f.addSiteAndLogin()

	stdout, _, code := f.run("course", "list", "--limit", "2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "course.list", stdout)

	var page struct {
		Data []struct {
			ShortName string `json:"short_name"`
		} `json:"data"`
		Meta struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("got %d courses, want 2", len(page.Data))
	}
	if page.Meta.NextCursor == nil {
		t.Fatal("meta.next_cursor should say there is more")
	}

	// The cursor must actually continue where the first page stopped.
	stdout, _, code = f.run("course", "list", "--cursor", *page.Meta.NextCursor, "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d on the second page", code)
	}
	var rest struct {
		Data []struct {
			ShortName string `json:"short_name"`
		} `json:"data"`
		Meta struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &rest); err != nil {
		t.Fatal(err)
	}
	if len(rest.Data) != 1 || rest.Data[0].ShortName != "C" {
		t.Fatalf("second page = %+v, want just C", rest.Data)
	}
	if rest.Meta.NextCursor != nil {
		t.Errorf("the last page should report no further cursor, got %q", *rest.Meta.NextCursor)
	}
}

func TestCourseListReportsWhichBackendAnswered(t *testing.T) {
	// An agent has to be able to tell a web service answer from a scraped one.
	f := newFixture(t)
	f.withCourses(map[string]any{"id": 2, "shortname": "CS204", "fullname": "OS", "visible": 1})
	f.addSiteAndLogin()

	stdout, _, _ := f.run("course", "list", "--json")
	if !strings.Contains(stdout, `"source":"ws"`) {
		t.Errorf("meta.source should name the backend:\n%s", stdout)
	}
}

func TestCourseListSaysWhatIsMissingWhenTheSiteCannotDoIt(t *testing.T) {
	// The site answers get_site_info but does not expose the course function.
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionSiteInfo, map[string]any{
		"sitename": "Test Moodle", "username": "student1", "userid": 4, "release": "5.2.3",
		"downloadfiles": 1, "uploadfiles": 1,
		"functions": []any{map[string]any{"name": "mod_assign_get_assignments"}},
	})
	f.addSiteAndLogin()

	stdout, _, code := f.run("course", "list", "--json")
	if code != v1.ExitUnavailable {
		t.Fatalf("exit %d, want %d:\n%s", code, v1.ExitUnavailable, stdout)
	}
	validate(t, "error", stdout)
	if !strings.Contains(stdout, moodle.FunctionUserCourses) {
		t.Errorf("the error should name the missing function:\n%s", stdout)
	}
}

func TestLoginWithPassword(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	// The fake site issues "token-for-<username>" and accepts it afterwards.
	stdout, stderr, code := f.runWithStdin("Student123!\n",
		"auth", "login", "--method", "password", "--username", "student1", "--password-stdin")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}
	if _, _, statusCode := f.run("auth", "status"); statusCode != v1.ExitOK {
		t.Errorf("status after password login: exit %d", statusCode)
	}
}

func TestLoginWithPastedCallback(t *testing.T) {
	// The fallback that works everywhere, including where no URL scheme can
	// be registered.
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	callback := "moodlemobile://token=" + base64.StdEncoding.EncodeToString(
		[]byte("anyhash:::good-token"))

	stdout, stderr, code := f.run("auth", "login", "--method", "manual", "--callback", callback)
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "manual") {
		t.Errorf("the method should be reported:\n%s", stdout)
	}
}

func TestLoginRejectsACallbackFromAnotherLogin(t *testing.T) {
	// A pasted callback carrying a hash for some other site or passport must
	// not be accepted when this process generated the passport.
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	callback := "moodlemobile://token=" + base64.StdEncoding.EncodeToString(
		[]byte("a-hash-from-somewhere-else:::stolen-token"))

	_, _, code := f.run("auth", "login", "--method", "manual",
		"--callback", callback, "--passport", "my-passport")
	if code != v1.ExitValidation {
		t.Fatalf("exit %d, want %d", code, v1.ExitValidation)
	}
}

func TestOnlyTheQRExchangeClaimsToBeTheMoodleApp(t *testing.T) {
	// Moodle rejects the QR endpoint unless the caller says it is the app.
	// That claim must not leak into any other request.
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionQRTokens, map[string]any{
		"token": "good-token", "privatetoken": "",
	})
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	qr := f.server.URL() + "?qrlogin=KEY123&userid=4"
	if _, stderr, code := f.run("auth", "login", "--method", "qr", "--qr", qr); code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var sawApp, sawOwn bool
	for _, request := range f.server.Requests() {
		isApp := strings.Contains(request.UserAgent, "MoodleMobile")
		if request.Function == moodle.FunctionQRTokens {
			sawApp = isApp
			continue
		}
		if isApp {
			t.Errorf("%s was sent with the Moodle app User-Agent (%q)",
				request.Function, request.UserAgent)
		}
		if strings.Contains(request.UserAgent, "moodle-cli") {
			sawOwn = true
		}
	}
	if !sawApp {
		t.Error("the QR exchange did not claim to be the Moodle app, so Moodle would refuse it")
	}
	if !sawOwn {
		t.Error("no request identified this tool honestly")
	}
}

func TestAuthMethodsReportsWhatTheSiteSupports(t *testing.T) {
	f := newFixture(t)
	f.server.HandleValue(moodle.FunctionPublicConfig, map[string]any{
		"sitename": "Test Moodle", "enablewebservices": 1, "enablemobilewebservice": 1,
		"showloginform": 1, "tool_mobile_qrcodetype": 0,
	})
	if _, _, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatal("site add failed")
	}
	stdout, _, code := f.run("auth", "methods", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	validate(t, "auth.methods", stdout)
	if !strings.Contains(stdout, `"name":"qr"`) || !strings.Contains(stdout, `"availability":"unavailable"`) {
		t.Errorf("QR should be reported unavailable when the site has it off:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"name":"password"`) || !strings.Contains(stdout, `"availability":"available"`) {
		t.Errorf("password should be available on this site:\n%s", stdout)
	}
}

func TestAuthMethodsOnAnUnreachableSiteReportsUnknownNotUnavailable(t *testing.T) {
	// "I could not tell" is not "no": an offline site must not look like one
	// that forbids every login method.
	f := newFixture(t)
	if _, _, code := f.run("site", "add", "dead", "http://127.0.0.1:1"); code != 0 {
		t.Fatal("site add failed")
	}
	stdout, _, code := f.run("auth", "methods", "--site", "dead", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d:\n%s", code, stdout)
	}
	validate(t, "auth.methods", stdout)
	if strings.Contains(stdout, `"availability":"unavailable"`) {
		t.Errorf("an unreachable site must not mark methods unavailable:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"availability":"unknown"`) {
		t.Errorf("expected unknown verdicts:\n%s", stdout)
	}
}
