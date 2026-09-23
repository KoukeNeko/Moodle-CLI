package cli_test

import (
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

func TestAcademicConfigurationAndWorkloadValidation(t *testing.T) {
	f := newFixture(t)
	f.addSiteAndLogin()

	stdout, stderr, code := f.run("site", "academic", "configure", "--yes", "--json")
	if code != v1.ExitOK {
		t.Fatalf("configure exit %d: %s", code, stderr)
	}
	validate(t, "site.academic.configure", stdout)
	if !strings.Contains(stdout, `"undergraduate_minimum":21`) || !strings.Contains(stdout, `"graduate_minimum":6`) {
		t.Errorf("configured minima missing: %s", stdout)
	}

	f.functions = append(f.functions, moodle.FunctionCoursesByField)
	refreshFunctions(f)
	f.server.HandleValue(moodle.FunctionUserCourses, []any{
		map[string]any{"id": 2, "shortname": "UG101", "fullname": "Undergraduate 1"},
		map[string]any{"id": 3, "shortname": "UG102", "fullname": "Undergraduate 2"},
	})
	f.server.HandleValue(moodle.FunctionCoursesByField, map[string]any{"courses": []any{
		workloadCourse(2, "UG101", 12, "undergraduate", "2026-Fall"),
		workloadCourse(3, "UG102", 9, "undergraduate", "2026-Fall"),
	}})

	stdout, stderr, code = f.run("workload", "validate", "--require-minimum", "--json")
	if code != v1.ExitOK {
		t.Fatalf("valid workload exit %d: %s", code, stderr)
	}
	validate(t, "workload.validate", stdout)
	if !strings.Contains(stdout, `"credits":21`) || !strings.Contains(stdout, `"meets_minimum":true`) {
		t.Errorf("wrong workload result: %s", stdout)
	}
	if f.server.CallsTo(moodle.FunctionUserCourses) != 1 || f.server.CallsTo(moodle.FunctionCoursesByField) != 1 {
		t.Errorf("workload should use two requests, got enrol=%d details=%d",
			f.server.CallsTo(moodle.FunctionUserCourses), f.server.CallsTo(moodle.FunctionCoursesByField))
	}

	f.server.HandleValue(moodle.FunctionCoursesByField, map[string]any{"courses": []any{
		workloadCourse(2, "UG101", 3, "undergraduate", "2026-Fall"),
		workloadCourse(3, "UG102", 3, "undergraduate", "2026-Fall"),
	}})
	stdout, _, code = f.run("workload", "validate", "--require-minimum", "--json")
	if code != v1.ExitValidation {
		t.Fatalf("underload exit %d, want %d", code, v1.ExitValidation)
	}
	// A validation failure still publishes the calculated result, rather than
	// replacing it with prose that automation cannot inspect.
	validate(t, "workload.validate", stdout)
	if !strings.Contains(stdout, `"meets_minimum":false`) {
		t.Errorf("underload is not explicit: %s", stdout)
	}
}

func workloadCourse(id int, shortName string, credits float64, level, term string) map[string]any {
	return map[string]any{
		"id": id, "shortname": shortName, "fullname": shortName,
		"customfields": []any{
			map[string]any{"shortname": "credits", "valueraw": credits},
			map[string]any{"shortname": "academic_level", "valueraw": level},
			map[string]any{"shortname": "academic_term", "valueraw": term},
		},
	}
}
