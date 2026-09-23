package cli_test

import (
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

func TestWSListIsOfflineVersionedAndSchemaValid(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.run("ws", "list", "--version", "v52", "--match", "core_course_get_contents", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "ws.list", stdout)
	if !strings.Contains(stdout, `"registry_digest"`) || !strings.Contains(stdout, `"v52"`) {
		t.Errorf("registry provenance is missing: %s", stdout)
	}
	if !strings.Contains(stdout, `"core_course_get_contents"`) {
		t.Errorf("filtered function is missing: %s", stdout)
	}
	if f.server.CallsTo(moodle.FunctionSiteInfo) != 0 {
		t.Error("an offline registry listing contacted Moodle")
	}
}

func TestWSDescribePublishesVersionSpecificSchemas(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.run("ws", "describe", "core_course_get_contents", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "ws.describe", stdout)
	for _, required := range []string{`"v45"`, `"v51"`, `"v52"`, `"parameters"`, `"returns"`} {
		if !strings.Contains(stdout, required) {
			t.Errorf("%s missing from description", required)
		}
	}
}

func TestWSCallValidatesAndExecutesARegisteredRead(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	f.server.HandleValue("core_course_get_contents", []any{})
	f.functions = append(f.functions, "core_course_get_contents")
	refreshFunctions(f)

	stdout, stderr, code := f.run("ws", "call", "core_course_get_contents",
		"--params-json", `{"courseid":2}`, "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "ws.call", stdout)
	if f.server.CallsTo("core_course_get_contents") != 1 {
		t.Fatal("registered read did not reach Moodle exactly once")
	}

	_, _, code = f.run("ws", "call", "core_course_get_contents",
		"--params-json", `{"courseid":"wrong"}`)
	if code != v1.ExitValidation {
		t.Fatalf("invalid params exit %d, want %d", code, v1.ExitValidation)
	}
	if f.server.CallsTo("core_course_get_contents") != 1 {
		t.Fatal("invalid parameters reached Moodle")
	}
}

func TestWSWriteDryRunAndReadOnlyAreEnforcedFromRegistry(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	name := "core_calendar_create_calendar_events"
	f.functions = append(f.functions, name)
	f.server.HandleValue(name, map[string]any{"events": []any{}, "warnings": []any{}})
	refreshFunctions(f)

	params := `{"events":[]}`
	stdout, stderr, code := f.run("ws", "call", name, "--params-json", params, "--dry-run", "--json")
	if code != v1.ExitOK {
		t.Fatalf("dry-run exit %d: %s", code, stderr)
	}
	validate(t, "ws.call", stdout)
	if !strings.Contains(stdout, `"needs_allow_write":true`) || f.server.CallsTo(name) != 0 {
		t.Fatalf("dry run wrote or hid its gate: %s", stdout)
	}

	_, _, code = f.run("ws", "call", name, "--params-json", params)
	if code != v1.ExitUsage {
		t.Fatalf("write without opt-in exit %d", code)
	}
	_, _, code = f.run("ws", "call", name, "--params-json", params, "--allow-write", "--read-only")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("read-only write exit %d", code)
	}
	if f.server.CallsTo(name) != 0 {
		t.Fatal("blocked writes reached Moodle")
	}
}

func TestWSUnknownCoreFunctionPointsAtThePluginEscapeHatch(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	_, stderr, code := f.run("ws", "call", "local_example_do_thing", "--params-json", `{}`)
	if code != v1.ExitNotFound {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr, "api call") {
		t.Errorf("missing escape-hatch hint: %s", stderr)
	}
}

func refreshFunctions(f *fixture) {
	declared := make([]any, 0, len(f.functions))
	for _, name := range f.functions {
		declared = append(declared, map[string]any{"name": name, "version": "2026091800"})
	}
	f.server.HandleValue(moodle.FunctionSiteInfo, map[string]any{
		"sitename": "Test Moodle", "username": "student1", "firstname": "Sam",
		"lastname": "Student", "userid": 4, "release": "5.2.3",
		"downloadfiles": 1, "uploadfiles": 1, "functions": declared,
	})
}
