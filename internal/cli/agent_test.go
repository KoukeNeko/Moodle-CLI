package cli_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

// These cover what makes the JSON contract usable by an unattended caller:
// asking for only the fields it reads, never being left waiting on a prompt,
// and learning from the binary itself whether a command may run unattended.

func dataKeys(t *testing.T, stdout string) [][]string {
	t.Helper()
	var doc struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("not a list envelope: %v\n%s", err, stdout)
	}
	if doc.Meta == nil {
		t.Errorf("meta was dropped along with the unwanted fields:\n%s", stdout)
	}
	var out [][]string
	for _, row := range doc.Data {
		var names []string
		for name := range row {
			names = append(names, name)
		}
		sort.Strings(names)
		out = append(out, names)
	}
	return out
}

func TestFieldsKeepOnlyWhatWasAskedFor(t *testing.T) {
	f := newFixture(t)
	f.withCourses(
		map[string]any{"id": 2, "shortname": "CS204", "fullname": "OS", "startdate": 1757894400, "visible": 1},
		map[string]any{"id": 3, "shortname": "CS205", "fullname": "Networks", "visible": 1},
	)
	f.addSiteAndLogin()

	stdout, stderr, code := f.run("course", "list", "--json", "--fields", "short_name,start_date")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	rows := dataKeys(t, stdout)
	if len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	for _, names := range rows {
		if strings.Join(names, ",") != "short_name,start_date" {
			t.Errorf("row fields = %v", names)
		}
	}
	// A field the row holds as null stays null rather than disappearing.
	if !strings.Contains(stdout, `"start_date":null`) {
		t.Errorf("the course without a start date lost the field:\n%s", stdout)
	}
}

func TestAnUnknownFieldIsRefusedWithTheKnownOnes(t *testing.T) {
	// An empty listing has no rows to learn names from; the schema still
	// knows them, so a typo is refused rather than answered with nothing.
	f := newFixture(t)
	f.withCourses()
	f.addSiteAndLogin()

	stdout, _, code := f.run("course", "list", "--json", "--fields", "shortname")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want usage:\n%s", code, stdout)
	}
	validate(t, "error", stdout)
	if !strings.Contains(stdout, "short_name") {
		t.Errorf("the refusal should list the real field names:\n%s", stdout)
	}
}

func TestFieldsWithoutJSONIsAUsageError(t *testing.T) {
	_, stderr, code := run(t, "version", "--fields", "version")
	if code != v1.ExitUsage || !strings.Contains(stderr, "--json") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestNoInputNeverReadsACallbackFromStdin(t *testing.T) {
	// Without --no-input the manual method waits for a pasted callback.
	// With it, nothing is read even when stdin holds something; the refusal
	// names the flag that supplies the answer instead.
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	_, stderr, code := f.runWithStdin("moodlemobile://token=abc\n",
		"auth", "login", "--method", "manual", "--no-input")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stderr, "--callback") {
		t.Errorf("the refusal should name --callback:\n%s", stderr)
	}
}

func TestNoInputRefusesTheHiddenSessionPrompt(t *testing.T) {
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	_, stderr, code := f.run("auth", "import-session", "--no-input")
	if code != v1.ExitUsage || !strings.Contains(stderr, "--no-input") ||
		!strings.Contains(stderr, "--stdin") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestNoInputRefusesToAskBeforeSubmitting(t *testing.T) {
	// Even with a person at a terminal, --no-input means nobody is asked.
	a := newAssignmentFixture(t, true, false)
	a.deps.Interactive = func() bool { return true }
	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--no-input")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if writeCalls(a.server) != 0 {
		t.Error("work was sent without confirmation")
	}
}

func TestSchemaDescribesWhetherACommandMayRunUnattended(t *testing.T) {
	cases := map[string]struct{ safety, idempotency, kind string }{
		"assignment submit": {"write", "non_idempotent", "assignment.submit"},
		"course list":       {"read", "idempotent", "course.list"},
		"site add":          {"local", "idempotent", "site.add"},
		"file download":     {"local", "idempotent", "file.download"},
		"ws call":           {"write", "non_idempotent", "ws.call"},
	}
	for path, want := range cases {
		args := append([]string{"schema"}, strings.Fields(path)...)
		stdout, stderr, code := run(t, append(args, "--json")...)
		if code != v1.ExitOK {
			t.Fatalf("%s: exit %d: %s", path, code, stderr)
		}
		validate(t, "command.schema", stdout)
		var doc struct {
			Data struct {
				Safety      string         `json:"safety"`
				Idempotency string         `json:"idempotency"`
				Kind        string         `json:"kind"`
				Input       map[string]any `json:"input_schema"`
				Output      map[string]any `json:"output_schema"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatal(err)
		}
		got := doc.Data
		if got.Safety != want.safety || got.Idempotency != want.idempotency || got.Kind != want.kind {
			t.Errorf("%s: %s/%s/%s, want %+v", path, got.Safety, got.Idempotency, got.Kind, want)
		}
		// A kind without a published schema reports null, not a guess.
		if _, err := v1.Schema(want.kind); err == nil && got.Output == nil {
			t.Errorf("%s: output_schema is missing", path)
		}
	}
}

func TestSchemaDescribesPositionalArguments(t *testing.T) {
	stdout, _, code := run(t, "schema", "assignment", "submit", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	var doc struct {
		Data struct {
			Input struct {
				Required   []string                  `json:"required"`
				Properties map[string]map[string]any `json:"properties"`
			} `json:"input_schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	args := doc.Data.Input.Properties["args"]
	// An assignment and at least one file; the files repeat, so no maximum.
	if args["minItems"] != float64(2) || args["maxItems"] != nil {
		t.Errorf("args = %v", args)
	}
	if doc.Data.Input.Properties["dry-run"]["type"] != "boolean" {
		t.Errorf("--dry-run is not described as a boolean: %v", doc.Data.Input.Properties["dry-run"])
	}
}

func TestSchemaOfAGroupNamesItsCommands(t *testing.T) {
	_, stderr, code := run(t, "schema", "assignment")
	if code != v1.ExitUsage || !strings.Contains(stderr, "submit") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestSchemaDescribesACommandReadOnlyWithholds(t *testing.T) {
	// Read-only mode hides every write. An agent running under it still has
	// to be able to learn that a command is one, rather than hear it does not
	// exist.
	stdout, stderr, code := run(t, "schema", "assignment", "submit", "--json", "--read-only")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, `"safety":"write"`) {
		t.Errorf("a withheld write was not described as one:\n%s", stdout)
	}
}
