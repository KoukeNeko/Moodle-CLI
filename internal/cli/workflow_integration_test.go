package cli_test

import (
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

func TestEnrolmentMethodsCallsTheTypedRead(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()
	name := "core_enrol_get_course_enrolment_methods"
	f.functions = append(f.functions, name)
	refreshFunctions(f)
	f.server.HandleValue(name, []any{})

	stdout, stderr, code := f.run("enrolment", "methods", "--param", "courseid=2", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	validate(t, "workflow.call", stdout)
	if !strings.Contains(stdout, `"function":"`+name+`"`) || f.server.CallsTo(name) != 1 {
		t.Fatalf("typed workflow did not execute exactly once: %s", stdout)
	}
}

func TestWorkflowWritesRequireConfirmationAndDryRunNeverSends(t *testing.T) {
	f := newFixture(t)
	params := `{"enrolments":[{"roleid":5,"userid":7,"courseid":2}]}`

	_, _, code := f.run("enrolment", "add", "--params-json", params)
	if code != v1.ExitUsage {
		t.Fatalf("write without confirmation exit %d, want %d", code, v1.ExitUsage)
	}

	f.addSiteAndLogin()
	name := "enrol_manual_enrol_users"
	f.functions = append(f.functions, name)
	refreshFunctions(f)
	stdout, stderr, code := f.run("enrolment", "add", "--params-json", params, "--dry-run", "--json")
	if code != v1.ExitOK {
		t.Fatalf("dry-run exit %d: %s", code, stderr)
	}
	validate(t, "workflow.call", stdout)
	if !strings.Contains(stdout, `"dry_run":true`) || f.server.CallsTo(name) != 0 {
		t.Fatalf("dry-run sent a write or omitted its status: %s", stdout)
	}

	_, _, code = f.run("enrolment", "add", "--params-json", params, "--yes", "--read-only")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("read-only write exit %d, want %d", code, v1.ExitPermissionDenied)
	}
	if f.server.CallsTo(name) != 0 {
		t.Fatal("read-only workflow reached Moodle")
	}
}
