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
