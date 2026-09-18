package v1_test

import (
	"encoding/json"
	"errors"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func TestExitCodeCoversEveryCode(t *testing.T) {
	// Every Code must map to a distinct, documented exit code. A new Code
	// with no mapping would silently become "internal".
	seen := map[int]errs.Code{}
	for _, code := range errs.Codes() {
		got := v1.ExitCode(errs.New(code, "boom"))
		if got == v1.ExitInternal && code != errs.CodeInternal {
			t.Errorf("code %q falls through to ExitInternal", code)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("codes %q and %q share exit code %d", prev, code, got)
		}
		seen[got] = code
	}
}

func TestExitCodeAmbiguousWinsOverCode(t *testing.T) {
	// An ambiguous outcome must report 12 whatever the code,
	// because that is the case a script has to special-case first.
	err := errs.New(errs.CodeNetwork, "connection reset").Ambiguous()
	if got := v1.ExitCode(err); got != v1.ExitAmbiguous {
		t.Fatalf("ambiguous outcome: got exit %d, want %d", got, v1.ExitAmbiguous)
	}
}

func TestExitCodeNilIsOK(t *testing.T) {
	if got := v1.ExitCode(nil); got != v1.ExitOK {
		t.Fatalf("nil error: got exit %d, want 0", got)
	}
}

func TestUnclassifiedErrorIsInternalNotBenign(t *testing.T) {
	// An error nobody classified must never look like a clean usage problem.
	env := v1.NewErrorEnvelope(errors.New("something went wrong"))
	if env.Error.Code != errs.CodeInternal {
		t.Fatalf("got code %q, want internal", env.Error.Code)
	}
	if got := v1.ExitCode(errors.New("x")); got != v1.ExitInternal {
		t.Fatalf("got exit %d, want %d", got, v1.ExitInternal)
	}
}

func TestErrorEnvelopeNullsRatherThanEmptyStrings(t *testing.T) {
	// A declared field is always present, and null when absent.
	raw, err := json.Marshal(v1.NewErrorEnvelope(errs.New(errs.CodeUsage, "bad flag")))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	body, ok := decoded["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %s", raw)
	}
	for _, field := range []string{"code", "reason", "outcome", "retryable", "message", "hint", "upstream"} {
		if _, present := body[field]; !present {
			t.Errorf("field %q missing from error body", field)
		}
	}
	if body["reason"] != nil {
		t.Errorf("reason should be null when unset, got %v", body["reason"])
	}
	if body["hint"] != nil {
		t.Errorf("hint should be null when unset, got %v", body["hint"])
	}
}

func TestMetaMissingIsNeverNull(t *testing.T) {
	raw, err := json.Marshal(v1.NewEnvelope("version", struct{}{}, v1.NewMeta(v1.SourceLocal)))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Meta struct {
			Missing []string `json:"missing"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Meta.Missing == nil {
		t.Fatalf("meta.missing marshalled as null: %s", raw)
	}
}

func TestSchemaLookup(t *testing.T) {
	for _, kind := range v1.SchemaKinds() {
		if _, err := v1.Schema(kind); err != nil {
			t.Errorf("listed kind %q has no schema: %v", kind, err)
		}
	}
	// A kind must never be able to escape the embedded directory.
	for _, bad := range []string{"", "../secret", "a/b", "version.schema"} {
		if _, err := v1.Schema(bad); err == nil {
			t.Errorf("Schema(%q) should have failed", bad)
		}
	}
}
