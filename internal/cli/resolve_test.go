package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

func resolved(t *testing.T, stdout string) v1.ResolvedURL {
	t.Helper()
	var doc struct {
		Data v1.ResolvedURL `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Data
}

func TestResolveMakesNoRequest(t *testing.T) {
	// Reading a link must not announce to the site that you have it, and it
	// has to work with no site configured at all.
	f := newFixture(t)
	stdout, stderr, code := f.run("resolve",
		"https://moodle.example.edu/mod/assign/view.php?id=4", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "resolve", stdout)

	if len(f.server.Requests()) != 0 {
		t.Errorf("resolving a link made %d request(s)", len(f.server.Requests()))
	}
	data := resolved(t, stdout)
	if data.Kind != "activity" || data.CMID == nil || *data.CMID != "4" {
		t.Errorf("got %+v", data)
	}
	if data.Command == nil || !strings.Contains(*data.Command, "assignment show") {
		t.Errorf("command = %v", data.Command)
	}
}

func TestResolveSaysSoWhenItCannotHelp(t *testing.T) {
	// A confident wrong suggestion is worse than none.
	f := newFixture(t)
	stdout, _, code := f.run("resolve", "https://moodle.example.edu/badges/mybadges.php", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	data := resolved(t, stdout)
	if data.Kind != "unknown" {
		t.Errorf("kind = %q", data.Kind)
	}
	if data.Command != nil {
		t.Errorf("command = %q, want none", *data.Command)
	}
}

func TestResolveTellsTheReaderTheCmidIsNotTheActivityId(t *testing.T) {
	// The two are both small integers and are not interchangeable; someone
	// reading this output will otherwise try the wrong one.
	f := newFixture(t)
	human, _, code := f.run("resolve", "https://moodle.example.edu/mod/assign/view.php?id=4")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(human, "not the activity's own id") {
		t.Errorf("the output does not warn about the id:\n%s", human)
	}
}

func TestAssignmentCommandsAcceptAPastedAddress(t *testing.T) {
	// What a student has in their clipboard after looking at the assignment.
	a := newAssignmentFixture(t, true, false)
	// The fixture's assignment has id 7 and course module id 12.
	stdout, stderr, code := a.run("assignment", "status",
		"https://moodle.example.edu/mod/assign/view.php?id=12", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	var doc struct {
		Data v1.SubmissionState `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Data.AssignmentID != "7" {
		t.Errorf("assignment_id = %q, want 7 — the address carries the course module id",
			doc.Data.AssignmentID)
	}
}

func TestAnAddressForAnotherActivityIsRefusedClearly(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	_, stderr, code := a.run("assignment", "status",
		"https://moodle.example.edu/mod/forum/view.php?id=1")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if !strings.Contains(stderr, "forum") {
		t.Errorf("the error does not say what the address actually is:\n%s", stderr)
	}
}

func TestAnAddressForAnAssignmentYouCannotSeeIsNotFound(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	_, _, code := a.run("assignment", "status",
		"https://moodle.example.edu/mod/assign/view.php?id=999")
	if code != v1.ExitNotFound {
		t.Errorf("exit %d, want %d", code, v1.ExitNotFound)
	}
}
