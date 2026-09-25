package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

func TestImportSessionFromStdinStoresVerifiedBrowserSession(t *testing.T) {
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	stdout, stderr, code := f.runWithStdin("MoodleSession=private-test-value\n",
		"auth", "import-session", "--stdin", "--account", "safari")
	if code != v1.ExitOK {
		t.Fatalf("import exit %d: %s%s", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "private-test-value") {
		t.Fatal("session value leaked into output")
	}
	stdout, stderr, code = f.run("auth", "status")
	if code != v1.ExitOK || !strings.Contains(stdout, "user 4") {
		t.Fatalf("stored session was not usable: exit %d: %s%s", code, stdout, stderr)
	}
}

func TestImportSessionRejectsUnsafeInputWithoutEcho(t *testing.T) {
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	for _, input := range []string{
		"", "OtherCookie=secret", "MoodleSession=secret; OtherCookie=secret", "MoodleSession=bad\nvalue",
	} {
		stdout, stderr, code := f.runWithStdin(input, "auth", "import-session", "--stdin")
		if code != v1.ExitUsage {
			t.Errorf("%q: expected usage exit, got %d: %s%s", input, code, stdout, stderr)
		}
		if strings.Contains(stdout+stderr, "secret") {
			t.Errorf("%q: echoed session value", input)
		}
	}
	stdout, stderr, code := f.run("auth", "import-session")
	if code != v1.ExitUsage || !strings.Contains(stderr, "--stdin") {
		t.Fatalf("noninteractive prompt: exit %d: %s%s", code, stdout, stderr)
	}
}

func TestImportSessionJSONUsesLoginContractWithoutSecret(t *testing.T) {
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	stdout, stderr, code := f.runWithStdin("secret-value",
		"auth", "import-session", "--stdin", "--json")
	if code != v1.ExitOK || strings.Contains(stdout+stderr, "secret-value") {
		t.Fatalf("JSON import exit %d or leaked session: %s%s", code, stdout, stderr)
	}
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		Kind          string `json:"kind"`
		Data          struct {
			UserID *string `json:"user_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Kind != "auth.login" || result.Data.UserID == nil || *result.Data.UserID != "4" {
		t.Fatalf("wrong login contract: %s", stdout)
	}
}

func TestSiteInspectWithABrowserSessionIsNotReportedAsExpired(t *testing.T) {
	// A browser session has no token. Inspect opened a token session with an
	// empty one, and the site's invalidtoken came back as "sign in again"
	// for a session that was perfectly valid.
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	if _, stderr, code := f.runWithStdin("MoodleSession=private-test-value\n",
		"auth", "import-session", "--stdin"); code != v1.ExitOK {
		t.Fatalf("import: %s", stderr)
	}
	stdout, stderr, code := f.run("site", "inspect")
	if code != v1.ExitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "browser session") {
		t.Errorf("inspect should say what a browser session can tell it:\n%s", stdout)
	}
}
