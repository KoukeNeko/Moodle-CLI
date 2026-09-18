package cli_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

const unreviewed = "local_somebodys_plugin_do_something"

func apiResult(t *testing.T, stdout string) v1.APICallResult {
	t.Helper()
	var doc struct {
		Data v1.APICallResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Data
}

// withPlugin adds a function nobody has reviewed to the fake site.
func (f *fixture) withPlugin(t *testing.T) {
	t.Helper()
	f.functions = append(f.functions, unreviewed)
	declared := make([]any, 0, len(f.functions))
	for _, name := range f.functions {
		declared = append(declared, map[string]any{"name": name, "version": "2026091800"})
	}
	f.server.HandleValue(moodle.FunctionSiteInfo, map[string]any{
		"sitename": "Test Moodle", "username": "student1", "firstname": "Sam",
		"lastname": "Student", "userid": 4, "release": "5.2.3",
		"downloadfiles": 1, "uploadfiles": 1, "functions": declared,
	})
	f.server.HandleValue(unreviewed, map[string]any{"done": true})
}

func TestAnUnreviewedFunctionIsTreatedAsOneThatWrites(t *testing.T) {
	// A site's plugins can expose anything. Guessing "probably a read" would
	// be guessing with someone else's coursework.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withPlugin(t)

	_, stderr, code := f.run("api", "call", unreviewed)
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if !strings.Contains(stderr, "not been reviewed") {
		t.Errorf("the refusal does not say why:\n%s", stderr)
	}
	if f.server.CallsTo(unreviewed) != 0 {
		t.Error("the call was made anyway")
	}
}

func TestAllowWriteIsTheWayThrough(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withPlugin(t)

	stdout, stderr, code := f.run("api", "call", unreviewed, "--allow-write", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "api.call", stdout)

	result := apiResult(t, stdout)
	if result.Reviewed {
		t.Error("an invented function was reported as reviewed")
	}
	if !result.Mutates {
		t.Error("an unreviewed function was reported as a read")
	}
	// The site's own shape is passed through unread: there is no typed
	// contract for a function nobody has looked at.
	if !strings.Contains(string(result.Response), `"done"`) {
		t.Errorf("the response was not passed through: %s", result.Response)
	}
}

func TestADryRunDescribesAWriteInsteadOfRefusingIt(t *testing.T) {
	// The refusal points at --dry-run, so --dry-run had better work. It sends
	// nothing, and refusing to describe the call would hide what someone ran
	// it to find out.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withPlugin(t)

	stdout, stderr, code := f.run("api", "call", unreviewed, "--dry-run", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "api.call", stdout)

	if f.server.CallsTo(unreviewed) != 0 {
		t.Fatal("a dry run reached the site")
	}
	result := apiResult(t, stdout)
	if !result.DryRun || !result.NeedsAllowWrite {
		t.Errorf("dry_run=%v needs_allow_write=%v", result.DryRun, result.NeedsAllowWrite)
	}

	human, _, _ := f.run("api", "call", unreviewed, "--dry-run")
	if !strings.Contains(human, "--allow-write") {
		t.Errorf("the plan does not say what a real run would need:\n%s", human)
	}
}

func TestAPlainReadNeedsNoPermission(t *testing.T) {
	// The escape hatch has to stay usable, or people will reach past it.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withCourses(map[string]any{
		"id": 2, "shortname": "CS204", "fullname": "Operating Systems",
		"startdate": 0, "enddate": 0, "visible": 1,
	})

	stdout, stderr, code := f.run("api", "call", moodle.FunctionUserCourses,
		"--param", "userid=4", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	result := apiResult(t, stdout)
	if result.Mutates {
		t.Error("a listing was reported as a write")
	}
	if !result.Reviewed {
		t.Error("a function in the registry was reported as unreviewed")
	}
}

func TestParametersReachTheSiteInMoodlesOwnNotation(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withPlugin(t)

	if _, _, code := f.run("api", "call", unreviewed, "--allow-write",
		"--param", "courseids[0]=2",
		"--params-json", `{"userid": 4, "courseids[0]": 99}`); code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	var sent url.Values
	for _, request := range f.server.Requests() {
		if request.Function == unreviewed {
			sent = request.Params
		}
	}
	if got := sent.Get("userid"); got != "4" {
		t.Errorf("userid = %q", got)
	}
	// --param goes in after the JSON so a single field can be overridden,
	// which is how someone iterates on a call.
	if got := sent.Get("courseids[0]"); got != "2" {
		t.Errorf("courseids[0] = %q, want --param to win over --params-json", got)
	}
}

func TestAFunctionTheSiteDoesNotHaveIsRefusedBeforeSending(t *testing.T) {
	// Moodle answers this with a generic access exception, which reads like a
	// permissions problem rather than a typo.
	f := newFixture(t)
	f.addSiteAndLogin()

	_, stderr, code := f.run("api", "call", "core_completely_made_up", "--allow-write")
	if code != v1.ExitUnavailable {
		t.Fatalf("exit %d, want %d", code, v1.ExitUnavailable)
	}
	if !strings.Contains(stderr, "does not offer") {
		t.Errorf("the error does not say the site lacks it:\n%s", stderr)
	}
}

func TestFunctionsListSaysWhatIsAssumedRatherThanKnown(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withPlugin(t)

	stdout, stderr, code := f.run("api", "functions", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "api.functions", stdout)

	var doc struct {
		Data []v1.APIFunction `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, item := range doc.Data {
		if item.Name != unreviewed {
			continue
		}
		found = true
		if item.Reviewed || !item.Mutates || item.Retry != "never" {
			t.Errorf("unreviewed function reported as %+v", item)
		}
		if item.Why != nil && *item.Why != "" && item.Reviewed {
			t.Error("an unreviewed function carries reasoning it never had")
		}
	}
	if !found {
		t.Fatalf("the plugin's function is missing from the listing")
	}

	human, _, _ := f.run("api", "functions", "--unreviewed")
	if !strings.Contains(human, "assumed to write") {
		t.Errorf("the listing presents an assumption as a finding:\n%s", human)
	}
}

func TestReadOnlyWithholdsTheEscapeHatchEntirely(t *testing.T) {
	// An arbitrary function caller is exactly what should not be available
	// when someone has asked for read-only.
	f := newFixture(t)
	f.addSiteAndLogin()
	f.withPlugin(t)

	_, stderr, code := f.run("api", "call", unreviewed, "--allow-write", "--read-only")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d", code, v1.ExitPermissionDenied)
	}
	if !strings.Contains(stderr, "read-only") {
		t.Errorf("the refusal does not mention read-only mode:\n%s", stderr)
	}
	if f.server.CallsTo(unreviewed) != 0 {
		t.Error("the call was made in read-only mode")
	}
}
