package wsregistry_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/wsregistry"
)

func load(t *testing.T) *wsregistry.Registry {
	t.Helper()
	registry, err := wsregistry.Load()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestSnapshotsFormAVersionedUnion(t *testing.T) {
	registry := load(t)
	if got := len(registry.Versions()); got != 3 {
		t.Fatalf("versions = %d, want 3", got)
	}
	if got := len(registry.All()); got < 755 {
		t.Fatalf("union has %d functions, want at least the largest snapshot", got)
	}
	function, ok := registry.Lookup("core_course_get_contents")
	if !ok {
		t.Fatal("core_course_get_contents missing")
	}
	if strings.Join(function.Versions, ",") != "v45,v51,v52" {
		t.Errorf("versions = %v", function.Versions)
	}
	for _, version := range function.Versions {
		variant := function.Variants[version]
		if !json.Valid(variant.Parameters) || !json.Valid(variant.Returns) {
			t.Errorf("%s schemas are not JSON", version)
		}
	}
}

// recordingFunctions are declared read by Moodle's services.php but are
// effects: each logs an event (which can complete an activity), and
// tool_mobile_get_content runs a plugin's mobile view, which can write. The
// exporter classifies them from an explicit list, test/e2e/export-ws-registry.php.
//
// core_get_fragment is not here on purpose. It is declared read and
// dispatches to any component's output_fragment_* callback, so whether a call
// writes depends on the callback, and treating all of it as a write is a
// decision for the maintainer.
var recordingFunctions = []string{
	"tool_mobile_get_content",
	"core_competency_competency_framework_viewed",
	"core_competency_competency_viewed",
	"core_competency_template_viewed",
	"core_competency_user_competency_plan_viewed",
	"core_competency_user_competency_viewed",
	"core_competency_user_competency_viewed_in_course",
	"core_competency_user_competency_viewed_in_plan",
	"core_blog_view_entries",
	"mod_lti_view_lti",
}

func TestFunctionsThatRecordSomethingAreWritesInEveryVersion(t *testing.T) {
	registry := load(t)
	for _, name := range recordingFunctions {
		function, ok := registry.Lookup(name)
		if !ok {
			t.Errorf("%s is not in the registry; was it renamed?", name)
			continue
		}
		if function.Effect != wsregistry.EffectWrite {
			t.Errorf("%s: union effect = %q, want write", name, function.Effect)
		}
		for _, version := range function.Versions {
			variant := function.Variants[version]
			if variant.Effect != wsregistry.EffectWrite || variant.Retry != "never" {
				t.Errorf("%s in %s: effect=%q retry=%q, want write and never",
					name, version, variant.Effect, variant.Retry)
			}
			if variant.Destructive {
				t.Errorf("%s in %s is marked destructive; recording is not deletion", name, version)
			}
		}
	}
}

type caller struct {
	called int
	name   string
	params map[string]any
}

func (c *caller) Call(_ context.Context, name string, params map[string]any) (json.RawMessage, error) {
	c.called++
	c.name, c.params = name, params
	return json.RawMessage(`[{"id":2}]`), nil
}

func capabilities(functions ...string) *site.Capabilities {
	out := site.NewCapabilities()
	out.Release = "5.2.3 (Build: 20260914)"
	for _, name := range functions {
		out.Functions[name] = site.FunctionInfo{Name: name}
	}
	return out
}

func TestTypedCallValidatesBeforeSending(t *testing.T) {
	transport := &caller{}
	service := wsregistry.NewService(load(t), transport, safety.Mode{}, false)
	_, err := service.Call(context.Background(), capabilities("core_course_get_contents"),
		"5.2.3", "core_course_get_contents", map[string]any{"courseid": "not-an-integer"}, false)
	if err == nil {
		t.Fatal("invalid parameters were accepted")
	}
	if errs.From(err).Code != errs.CodeValidation {
		t.Fatalf("code = %q", errs.From(err).Code)
	}
	if transport.called != 0 {
		t.Fatal("invalid parameters reached Moodle")
	}
}

func TestTypedReadCallsTheTransport(t *testing.T) {
	transport := &caller{}
	service := wsregistry.NewService(load(t), transport, safety.Mode{}, false)
	result, err := service.Call(context.Background(), capabilities("core_course_get_contents"),
		"5.2.3", "core_course_get_contents", map[string]any{"courseid": 2}, false)
	if err != nil {
		t.Fatal(err)
	}
	if transport.called != 1 || result.Effect != wsregistry.EffectRead {
		t.Fatalf("called=%d result=%+v", transport.called, result)
	}
}

func TestTypedCallOverABrowserSessionSaysATokenIsNeeded(t *testing.T) {
	// A browser session reports no release. The refusal used to be about a
	// registry snapshot for Moodle "", which pointed at the wrong problem.
	transport := &caller{}
	service := wsregistry.NewService(load(t), transport, safety.Mode{}, false)
	session := site.NewCapabilities()
	session.Credential = site.CredentialBrowserSession
	_, err := service.Call(context.Background(), session,
		"", "core_course_get_contents", map[string]any{"courseid": 2}, false)
	if err == nil || !strings.Contains(err.Error(), "web service token") {
		t.Fatalf("err = %v, want a refusal naming the missing token", err)
	}
	if transport.called != 0 {
		t.Fatal("the call reached Moodle")
	}
}

func TestTypedWriteRequiresExplicitPermissionAndDryRunSendsNothing(t *testing.T) {
	transport := &caller{}
	service := wsregistry.NewService(load(t), transport, safety.Mode{}, false)
	name := "core_calendar_create_calendar_events"
	params := map[string]any{"events": []any{}}

	plan, err := service.Call(context.Background(), capabilities(name), "5.2.3", name, params, true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.DryRun || !plan.NeedsAllowWrite || transport.called != 0 {
		t.Fatalf("plan=%+v calls=%d", plan, transport.called)
	}
	_, err = service.Call(context.Background(), capabilities(name), "5.2.3", name, params, false)
	if err == nil || errs.From(err).Code != errs.CodeUsage {
		t.Fatalf("write refusal = %v", err)
	}
}

func TestTypedMobileContentIsRefusedInReadOnlyModeAndWithoutAllowWrite(t *testing.T) {
	// The skeptic's case: this used to be a read, so a read-only session ran
	// whichever plugin's mobile view it named.
	name := "tool_mobile_get_content"
	params := map[string]any{"component": "mod_attendance", "method": "mobile_view_activity"}

	transport := &caller{}
	readOnly := wsregistry.NewService(load(t), transport, safety.Mode{ReadOnly: true}, false)
	_, err := readOnly.Call(context.Background(), capabilities(name), "5.2.3", name, params, false)
	if err == nil || errs.From(err).Code != errs.CodePermissionDenied {
		t.Fatalf("read-only: err = %v, want permission_denied", err)
	}

	withoutPermission := wsregistry.NewService(load(t), transport, safety.Mode{}, false)
	_, err = withoutPermission.Call(context.Background(), capabilities(name), "5.2.3", name, params, false)
	if err == nil || errs.From(err).Code != errs.CodeUsage {
		t.Fatalf("without --allow-write: err = %v, want usage", err)
	}
	if transport.called != 0 {
		t.Fatal("a refused call reached Moodle")
	}
}

func TestPluginFunctionStaysInTheUntypedAPI(t *testing.T) {
	service := wsregistry.NewService(load(t), &caller{}, safety.Mode{}, true)
	_, err := service.Call(context.Background(), nil, "5.2.3", "local_example_do_thing", nil, false)
	if err == nil || errs.From(err).Code != errs.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(errs.From(err).Hint, "api call") {
		t.Errorf("hint = %q", errs.From(err).Hint)
	}
}
