package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// run executes the CLI with args and returns stdout, stderr and the exit code.
//
// Commands get an isolated configuration file and an in-process keychain, so
// a test never touches the developer's real setup.
func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runWith(t, cli.Deps{
		ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
		Auth: auth.NewManager(secret.NewMemory(), func(target site.Site) *moodle.Client {
			return moodle.NewClient(target)
		}),
	}, args...)
}

func runWith(t *testing.T, deps cli.Deps, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := cli.New(
		cli.BuildInfo{Version: "1.2.3", Commit: "abc123", BuildDate: "2026-09-18T00:00:00Z"},
		cli.Streams{Out: &out, Err: &errOut},
		deps,
	)
	code = app.Execute(context.Background(), args)
	return out.String(), errOut.String(), code
}

// compileSchema loads the embedded schemas as a set so $ref between them
// resolves without network access.
func compileSchema(t *testing.T, kind string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	root := v1.SchemaFS()
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	var target string
	for _, entry := range entries {
		raw, err := fs.ReadFile(root, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		obj, ok := doc.(map[string]any)
		if !ok {
			t.Fatalf("%s: schema is not an object", entry.Name())
		}
		id, _ := obj["$id"].(string)
		if id == "" {
			t.Fatalf("%s: schema has no $id", entry.Name())
		}
		if err := compiler.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
		if entry.Name() == kind+".schema.json" {
			target = id
		}
	}
	if target == "" {
		t.Fatalf("no schema for kind %q", kind)
	}
	schema, err := compiler.Compile(target)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validate(t *testing.T, kind, document string) {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(document))
	if err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, document)
	}
	if err := compileSchema(t, kind).Validate(value); err != nil {
		t.Fatalf("output does not satisfy the %s schema: %v\n%s", kind, err, document)
	}
}

func TestVersionJSONSatisfiesSchema(t *testing.T) {
	stdout, stderr, code := run(t, "version", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, want 0 (stderr: %s)", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty on success, got %q", stderr)
	}
	validate(t, "version", stdout)
}

func TestCommandsJSONSatisfiesSchema(t *testing.T) {
	stdout, _, code := run(t, "commands", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, want 0", code)
	}
	validate(t, "commands", stdout)
}

func TestUsageErrorIsExitTwoWithErrorEnvelopeOnStdout(t *testing.T) {
	// The Phase 1 acceptance criterion: a usage error exits 2 and, with
	// --json, puts a valid error envelope on stdout.
	stdout, _, code := run(t, "--json")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	validate(t, "error", stdout)
}

func TestUnknownCommandStillHonoursJSON(t *testing.T) {
	// Cobra fails before parsing flags when the command name is wrong, so the
	// format has to be decided from the raw arguments.
	stdout, _, code := run(t, "definitely-not-a-command", "--json")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	validate(t, "error", stdout)
}

func TestStdoutCarriesExactlyOneJSONDocument(t *testing.T) {
	// A consumer must be able to parse stdout as a single document, with no
	// progress text mixed in.
	for _, args := range [][]string{
		{"version", "--json"},
		{"commands", "--json"},
		{"--json"},
		{"schema", "no-such-kind", "--json"},
	} {
		stdout, _, _ := run(t, args...)
		decoder := json.NewDecoder(strings.NewReader(stdout))
		var first any
		if err := decoder.Decode(&first); err != nil {
			t.Errorf("%v: stdout is not a JSON document: %v (%q)", args, err, stdout)
			continue
		}
		var extra any
		if err := decoder.Decode(&extra); err == nil {
			t.Errorf("%v: stdout carries more than one JSON document", args)
		}
	}
}

func TestHumanOutputDoesNotGoToStdoutOnError(t *testing.T) {
	// Without --json, a failure must leave stdout untouched so a pipeline
	// reading stdout sees nothing rather than prose.
	stdout, stderr, code := run(t, "definitely-not-a-command")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty on a human-format error, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("stderr should carry the error, got %q", stderr)
	}
}

func TestSchemaCommandEmitsEveryPublishedKind(t *testing.T) {
	for _, kind := range v1.SchemaKinds() {
		stdout, _, code := run(t, "schema", kind)
		if code != v1.ExitOK {
			t.Errorf("schema %s: exit %d", kind, code)
			continue
		}
		if _, err := jsonschema.UnmarshalJSON(strings.NewReader(stdout)); err != nil {
			t.Errorf("schema %s: not valid JSON: %v", kind, err)
		}
	}
}

func TestVersionHumanOutputIsOneLine(t *testing.T) {
	stdout, _, code := run(t, "version")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if lines := strings.Count(strings.TrimSpace(stdout), "\n"); lines != 0 {
		t.Errorf("want a single line, got %q", stdout)
	}
	if !strings.Contains(stdout, "1.2.3") {
		t.Errorf("version string missing from %q", stdout)
	}
}
