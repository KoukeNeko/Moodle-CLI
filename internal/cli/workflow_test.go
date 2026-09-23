package cli

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/wsregistry"
)

// A reviewed workflow command must never silently drift away from the
// generated Moodle contract. This catches renamed/removed functions and the
// more dangerous case where a command advertised as a read actually writes.
func TestWorkflowSpecsMatchTheGeneratedRegistry(t *testing.T) {
	registry, err := wsregistry.Load()
	if err != nil {
		t.Fatal(err)
	}
	groups := [][]workflowSpec{
		courseWorkflowSpecs(), assignmentWorkflowSpecs(), gradeWorkflowSpecs(),
		forumWorkflowSpecs(), calendarWorkflowSpecs(), participantWorkflowSpecs(),
		enrolmentWorkflowSpecs(), groupWorkflowSpecs(), completionWorkflowSpecs(),
	}
	seen := map[string]string{}
	for _, specs := range groups {
		for _, spec := range specs {
			function, ok := registry.Lookup(spec.Function)
			if !ok {
				t.Errorf("%s references unknown core function %s", spec.Name, spec.Function)
				continue
			}
			wantWrite := function.Effect == wsregistry.EffectWrite
			if spec.Write != wantWrite {
				t.Errorf("%s maps to %s: command write=%t, registry effect=%s",
					spec.Name, spec.Function, spec.Write, function.Effect)
			}
			key := spec.Name + "\x00" + spec.Function
			if previous, duplicate := seen[key]; duplicate {
				t.Errorf("duplicate workflow %s -> %s (already in %s)", spec.Name, spec.Function, previous)
			}
			seen[key] = spec.Function
		}
	}
}

func TestTypedParamDecodesJSONLiterals(t *testing.T) {
	params, err := collectTypedParams([]string{
		"courseid=42", "enabled=false", `ids=[1,2]`, "name=Lab A", "code=007",
	}, `{"courseid":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if params["courseid"] != float64(42) || params["enabled"] != false {
		t.Fatalf("scalar params were not typed: %#v", params)
	}
	ids, ok := params["ids"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("ids = %#v", params["ids"])
	}
	if params["name"] != "Lab A" || params["code"] != "007" {
		t.Fatalf("plain strings changed: %#v", params)
	}
}
