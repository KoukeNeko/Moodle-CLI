package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

// fixture wires the CLI to a fake Moodle with an isolated config and keychain.
type fixture struct {
	t      *testing.T
	server *testmoodle.Server
	deps   cli.Deps
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	server := testmoodle.New()
	t.Cleanup(server.Close)

	f := &fixture{
		t:      t,
		server: server,
		deps: cli.Deps{
			ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
			Auth: auth.NewManager(secret.NewMemory(), func(target site.Site) *moodle.Client {
				return moodle.NewClient(target)
			}),
		},
	}
	// A working site by default; individual tests break what they need to.
	server.HandleValue(moodle.FunctionSiteInfo, map[string]any{
		"sitename": "Test Moodle", "username": "student1", "firstname": "Sam",
		"lastname": "Student", "userid": 4, "release": "5.2.3",
		"downloadfiles": 1, "uploadfiles": 1,
		"functions": []any{
			map[string]any{"name": "core_enrol_get_users_courses", "version": "2026091800"},
			map[string]any{"name": "mod_assign_get_assignments", "version": "2026091800"},
		},
	})
	server.AddToken("good-token")
	return f
}

func (f *fixture) run(args ...string) (stdout, stderr string, code int) {
	f.t.Helper()
	return runWith(f.t, f.deps, args...)
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
	if !strings.Contains(stdout, `"function_count":2`) {
		t.Errorf("function count missing from %s", stdout)
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
