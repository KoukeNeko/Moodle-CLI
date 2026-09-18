package cli_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

// assignmentFixture is a site with one assignment and a submission state that
// the test can move, the way Moodle would.
type assignmentFixture struct {
	*fixture
	// status is what mod_assign_get_submission_status will report next.
	status map[string]any
}

// newAssignmentFixture sets up a signed-in site holding one assignment.
//
// drafts mirrors Moodle's submissiondrafts setting, which decides whether
// saving content is enough or a second call is needed.
func newAssignmentFixture(t *testing.T, drafts, statement bool, extraPlugins ...string) *assignmentFixture {
	t.Helper()
	f := newFixture(t)
	f.addSiteAndLogin()

	a := &assignmentFixture{fixture: f}
	a.status = lastAttempt("new", nil)

	f.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{map[string]any{
			"id": 2, "shortname": "CS204",
			"assignments": []any{map[string]any{
				"id": 7, "cmid": 12, "course": 2, "name": "Essay 1",
				"duedate": 1789000000, "cutoffdate": 0,
				"intro":       "<p>Write <strong>800 words</strong> on scheduling.</p>",
				"introformat": 1, "grade": 100, "allowsubmissionsfromdate": 0,
				// -1 means "as many attempts as you like", not minus one.
				"maxattempts": -1, "timelimit": 0,
				"teamsubmission": 0, "blindmarking": 0,
				"introattachments": []any{map[string]any{
					"filename": "rubric.txt", "filepath": "/", "filesize": 45,
					"fileurl":      "https://moodle.example.edu/webservice/pluginfile.php/16/mod_assign/introattachment/0/rubric.txt",
					"timemodified": 1789000000, "mimetype": "text/plain",
					"isexternalfile": false,
				}},
				"submissiondrafts":           boolToInt(drafts),
				"requiresubmissionstatement": boolToInt(statement),
				"configs":                    pluginConfigs(extraPlugins),
			}},
		}},
	})
	f.server.Handle(moodle.FunctionSubmissionStatus, func(url.Values) (any, error) {
		return a.status, nil
	})
	draftAreas := int64(500)
	f.server.Handle(moodle.FunctionUnusedDraftArea, func(url.Values) (any, error) {
		draftAreas++
		return map[string]any{"itemid": draftAreas}, nil
	})

	// The writes answer the way Moodle does: an empty warnings array, and the
	// state moves only as far as the call actually takes it.
	f.server.Handle(moodle.FunctionSaveSubmission, func(url.Values) (any, error) {
		next := "submitted"
		if drafts {
			// This is the whole point: saving content on a drafts assignment
			// leaves the work unsubmitted.
			next = "draft"
		}
		a.status = lastAttempt(next, []any{"report.pdf"})
		return []any{}, nil
	})
	f.server.Handle(moodle.FunctionSubmitForGrading, func(url.Values) (any, error) {
		a.status = lastAttempt("submitted", []any{"report.pdf"})
		return []any{}, nil
	})
	return a
}

// pluginConfigs mirrors the configs array Moodle sends, which is where the
// enabled submission plugins and the file limits both live.
func pluginConfigs(extra []string) []any {
	configs := []any{
		map[string]any{"plugin": "file", "subtype": "assignsubmission",
			"name": "enabled", "value": "1"},
		map[string]any{"plugin": "file", "subtype": "assignsubmission",
			"name": "maxfilesubmissions", "value": "3"},
		map[string]any{"plugin": "file", "subtype": "assignsubmission",
			"name": "maxsubmissionsizebytes", "value": "0"},
		// Switched off, so it must not appear in the payload.
		map[string]any{"plugin": "comments", "subtype": "assignsubmission",
			"name": "enabled", "value": "0"},
	}
	for _, plugin := range extra {
		configs = append(configs, map[string]any{
			"plugin": plugin, "subtype": "assignsubmission",
			"name": "enabled", "value": "1",
		})
	}
	return configs
}

func lastAttempt(status string, files []any) map[string]any {
	attempt := map[string]any{
		"submissionsenabled": true, "canedit": true, "cansubmit": true,
		"locked": false, "graded": false, "gradingstatus": "notgraded",
	}
	if status != "new" {
		attempt["submission"] = map[string]any{
			"id": 31, "status": status, "timemodified": 1789000100,
			"plugins": []any{map[string]any{
				"type": "file",
				"fileareas": []any{map[string]any{
					"area": "submission_files", "files": filesPayload(files),
				}},
			}},
		}
	}
	return map[string]any{"lastattempt": attempt}
}

func filesPayload(names []any) []any {
	out := make([]any, 0, len(names))
	for _, name := range names {
		out = append(out, map[string]any{
			"filename": name, "filepath": "/", "filesize": 27,
			"fileurl": "https://moodle.example.edu/webservice/pluginfile.php/16/" +
				"assignsubmission_file/submission_files/1/" + name.(string),
			"timemodified": 1789000100, "mimetype": "application/pdf",
			"isexternalfile": false,
		})
	}
	return out
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func workFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 work"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeCalls counts every request that can change something upstream.
func writeCalls(server *testmoodle.Server) int {
	return server.CallsTo(testmoodle.UploadFunction) +
		server.CallsTo(moodle.FunctionSaveSubmission) +
		server.CallsTo(moodle.FunctionSubmitForGrading)
}

func TestAssignmentListSaysWhichOnesNeedHandingIn(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	stdout, stderr, code := a.run("assignment", "list", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.list", stdout)

	var doc struct {
		Data []v1.Assignment `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 1 {
		t.Fatalf("got %d assignments, want 1", len(doc.Data))
	}
	if !doc.Data[0].NeedsHandIn {
		t.Error("an assignment that keeps drafts was reported as needing no hand-in")
	}
	if doc.Data[0].MaxBytes != nil {
		t.Error("Moodle's 0 for 'no size limit' became a limit of 0 bytes")
	}
	if doc.Data[0].MaxFiles == nil || *doc.Data[0].MaxFiles != 3 {
		t.Error("the file limit was not read from the assignment's configuration")
	}
}

func TestSubmitDryRunSendsNothingAtAll(t *testing.T) {
	// Asserted against the wire, not against the report. The enforcement sits
	// in one place so this is a property of the program rather than of each
	// feature's diligence.
	a := newAssignmentFixture(t, true, false)
	stdout, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--dry-run", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.submit", stdout)

	if got := writeCalls(a.server); got != 0 {
		for _, request := range a.server.Requests() {
			t.Logf("request: %s", request.Function)
		}
		t.Fatalf("a dry run made %d write request(s)", got)
	}

	var doc struct {
		Data v1.SubmitReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Data.DryRun || doc.Data.Outcome != "planned" {
		t.Errorf("dry_run=%v outcome=%q", doc.Data.DryRun, doc.Data.Outcome)
	}
	if doc.Data.HandedIn {
		t.Error("a dry run reported the work as handed in")
	}
}

func TestSubmitOnADraftsAssignmentHandsInAndSaysSo(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	stdout, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--yes", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.submit", stdout)

	if a.server.CallsTo(moodle.FunctionSubmitForGrading) != 1 {
		t.Error("the work was saved but never handed in")
	}
	var doc struct {
		Data v1.SubmitReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Data.HandedIn || doc.Data.FinalStatus != "submitted" {
		t.Errorf("handed_in=%v final_status=%q", doc.Data.HandedIn, doc.Data.FinalStatus)
	}
}

func TestDraftFlagLeavesTheWorkUnsubmittedAndSaysSoLoudly(t *testing.T) {
	// The competitor behaviour this project exists to avoid is reporting this
	// state as a successful submission.
	a := newAssignmentFixture(t, true, false)
	stdout, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--draft", "--yes", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	if a.server.CallsTo(moodle.FunctionSubmitForGrading) != 0 {
		t.Error("--draft still handed the work in")
	}
	var doc struct {
		Data v1.SubmitReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Data.HandedIn {
		t.Fatal("a draft was reported as handed in")
	}
	if doc.Data.FinalStatus != "draft" {
		t.Errorf("final_status = %q, want draft", doc.Data.FinalStatus)
	}

	// And the human output has to say it, not merely omit it.
	human, _, code := a.run("assignment", "submit", "7", workFile(t), "--draft", "--yes")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(human, "NOT handed in") {
		t.Errorf("the human output does not say the work is unsubmitted:\n%s", human)
	}
}

func TestSubmitRefusesWithoutConfirmation(t *testing.T) {
	// There is no terminal in a test, and assuming consent because stdin is a
	// pipe would make the safe default depend on how the command was invoked.
	a := newAssignmentFixture(t, true, false)
	_, stderr, code := a.run("assignment", "submit", "7", workFile(t))
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if writeCalls(a.server) != 0 {
		t.Error("work was sent without confirmation")
	}
	if !strings.Contains(stderr, "--yes") {
		t.Errorf("the refusal does not say how to proceed:\n%s", stderr)
	}
}

func TestSubmitRefusesToAcceptTheStatementForTheStudent(t *testing.T) {
	a := newAssignmentFixture(t, true, true)
	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--yes")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want %d", code, v1.ExitUsage)
	}
	if writeCalls(a.server) != 0 {
		t.Error("work was sent before the submission statement was accepted")
	}
	if !strings.Contains(stderr, "--accept-statement") {
		t.Errorf("the refusal does not name the flag:\n%s", stderr)
	}

	// With the flag, the student's acceptance reaches Moodle.
	if _, stderr, code := a.run("assignment", "submit", "7", workFile(t),
		"--accept-statement", "--yes"); code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	var accepted string
	for _, request := range a.server.Requests() {
		if request.Function == moodle.FunctionSubmitForGrading {
			accepted = request.Params.Get("acceptsubmissionstatement")
		}
	}
	if accepted != "1" {
		t.Errorf("acceptsubmissionstatement = %q, want 1", accepted)
	}
}

func TestSubmitOnAnAssignmentWithoutDraftsSkipsTheSecondCall(t *testing.T) {
	a := newAssignmentFixture(t, false, false)
	stdout, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--yes", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	if a.server.CallsTo(moodle.FunctionSubmitForGrading) != 0 {
		t.Error("an assignment that submits on save was also handed in explicitly")
	}
	var doc struct {
		Data v1.SubmitReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.Data.HandedIn {
		t.Error("Moodle reports it submitted, but the report says otherwise")
	}
}

func TestALostResponseIsReconciledNotRetried(t *testing.T) {
	// The save runs on the server and the reply never arrives. Sending it
	// again would be guessing with the student's coursework.
	a := newAssignmentFixture(t, true, false)
	a.server.Handle(moodle.FunctionSaveSubmission, func(url.Values) (any, error) {
		a.status = lastAttempt("draft", []any{"report.pdf"})
		return nil, nil
	})
	a.server.Fail(moodle.FunctionSaveSubmission, testmoodle.FailAppliedThenLost)

	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--draft", "--yes")
	if code != v1.ExitOK {
		t.Fatalf("a confirmed save was reported as a failure: exit %d, %s", code, stderr)
	}
	if got := a.server.CallsTo(moodle.FunctionSaveSubmission); got != 1 {
		t.Errorf("the save ran %d times; an ambiguous write must not be repeated", got)
	}
}

func TestAnUnresolvableLostResponseExitsAmbiguous(t *testing.T) {
	// The state cannot be read back either, so the honest answer is "unknown",
	// which is exit code 12 and never a plain failure to retry.
	a := newAssignmentFixture(t, true, false)
	a.server.Fail(moodle.FunctionSaveSubmission, testmoodle.FailAppliedThenLost)

	calls := 0
	a.server.Handle(moodle.FunctionSubmissionStatus, func(url.Values) (any, error) {
		calls++
		if calls > 1 {
			return nil, errTemporary{}
		}
		return a.status, nil
	})
	a.server.Fail(moodle.FunctionSubmissionStatus, "")

	stdout, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--draft", "--yes", "--json")
	if code != v1.ExitAmbiguous {
		t.Fatalf("exit %d, want %d (ambiguous)\n%s%s", code, v1.ExitAmbiguous, stdout, stderr)
	}
	// Even a failure emits exactly one document on stdout.
	validate(t, "error", stdout)
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("--json wrote diagnostics to stderr as well:\n%s", stderr)
	}
}

// errTemporary stands in for a site that stops answering.
type errTemporary struct{}

func (errTemporary) Error() string { return "the site did not answer" }

func TestAssignmentStatusSpellsOutWhatDraftMeans(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	a.status = lastAttempt("draft", []any{"report.pdf"})

	stdout, stderr, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.status", stdout)
	var doc struct {
		Data v1.SubmissionState `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Data.Status != "draft" || doc.Data.HandedIn {
		t.Errorf("status=%q handed_in=%v", doc.Data.Status, doc.Data.HandedIn)
	}

	human, _, _ := a.run("assignment", "status", "7")
	if !strings.Contains(human, "NOT handed in") {
		t.Errorf("the human output does not explain what draft means:\n%s", human)
	}
}

func TestUnknownStatusIsNotFoldedIntoSubmitted(t *testing.T) {
	// A site that reports something this build has never seen must not have it
	// guessed into a state the student would act on.
	a := newAssignmentFixture(t, true, false)
	a.status = lastAttempt("some_plugin_state", nil)

	stdout, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	var doc struct {
		Data v1.SubmissionState `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Data.Status != "unknown" {
		t.Errorf("status = %q, want unknown", doc.Data.Status)
	}
	if doc.Data.HandedIn {
		t.Error("an unrecognised state was reported as handed in")
	}
}

func TestSubmittingFilesSendsBackTheTextTheStudentTyped(t *testing.T) {
	// Moodle saves every enabled plugin in one save_submission call. Omitting
	// the online text does not leave it alone, it stores it as empty — so this
	// asserts on what went over the wire, not on what the report claims.
	a := newAssignmentFixture(t, true, false, "onlinetext")
	a.status = withOnlineText(lastAttempt("new", nil), "<p>my essay</p>", 1)

	if _, stderr, code := a.run("assignment", "submit", "7", workFile(t),
		"--draft", "--yes"); code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}

	var save *testmoodle.Request
	for i, request := range a.server.Requests() {
		if request.Function == moodle.FunctionSaveSubmission {
			save = &a.server.Requests()[i]
		}
	}
	if save == nil {
		t.Fatal("no save_submission request was made")
	}
	if got := save.Params.Get("plugindata[onlinetext_editor][text]"); got != "<p>my essay</p>" {
		t.Errorf("the student's text went out as %q", got)
	}
	if got := save.Params.Get("plugindata[onlinetext_editor][format]"); got != "1" {
		t.Errorf("text format = %q, want 1", got)
	}
	fileDraft := save.Params.Get("plugindata[files_filemanager]")
	textDraft := save.Params.Get("plugindata[onlinetext_editor][itemid]")
	if textDraft == "" || textDraft == fileDraft {
		t.Errorf("online text draft %q must differ from the file draft %q", textDraft, fileDraft)
	}
	// A plugin the assignment has switched off must not be sent at all.
	for key := range save.Params {
		if strings.Contains(key, "comments") {
			t.Errorf("data was sent for a disabled plugin: %s", key)
		}
	}
}

func withOnlineText(attempt map[string]any, text string, format int) map[string]any {
	submission := attempt["lastattempt"].(map[string]any)
	if submission["submission"] == nil {
		submission["submission"] = map[string]any{
			"id": 31, "status": "new", "timemodified": 1789000100,
			"plugins": []any{},
		}
	}
	entry := submission["submission"].(map[string]any)
	entry["plugins"] = append(entry["plugins"].([]any), map[string]any{
		"type": "onlinetext",
		"editorfields": []any{map[string]any{
			"name": "onlinetext", "text": text, "format": format,
		}},
	})
	return attempt
}

func TestReadOnlyModeWithholdsEveryWritingCommand(t *testing.T) {
	a := newAssignmentFixture(t, true, false)

	// It refuses, and says why: someone who set the restriction in their
	// environment would learn nothing from "unknown command".
	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--yes", "--read-only")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitPermissionDenied, stderr)
	}
	if !strings.Contains(stderr, "read-only") {
		t.Errorf("the refusal does not mention read-only mode:\n%s", stderr)
	}
	if writeCalls(a.server) != 0 {
		t.Error("a write reached the site in read-only mode")
	}

	// And it is gone from what an agent can discover, so it cannot be chosen
	// in the first place.
	stdout, _, code := a.run("commands", "--json", "--read-only")
	if code != v1.ExitOK {
		t.Fatalf("commands: exit %d", code)
	}
	var doc struct {
		Data []struct {
			Path    string `json:"path"`
			Mutates bool   `json:"mutates"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) == 0 {
		t.Fatal("no commands were described at all")
	}
	for _, command := range doc.Data {
		if command.Mutates {
			t.Errorf("%s can write and is still offered in read-only mode", command.Path)
		}
	}
}

func TestReadOnlyModeStillAllowsReads(t *testing.T) {
	// The restriction has to leave the tool useful, or nobody will use it.
	a := newAssignmentFixture(t, true, false)
	stdout, stderr, code := a.run("assignment", "list", "--json", "--read-only")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.list", stdout)
}

func TestReadOnlyDryRunIsStillARefusal(t *testing.T) {
	// --dry-run is this invocation's choice; --read-only is a standing
	// restriction. The restriction is not something a flag can talk its way
	// past, even a flag that would have written nothing.
	a := newAssignmentFixture(t, true, false)
	_, _, code := a.run("assignment", "submit", "7", workFile(t), "--dry-run", "--read-only")
	if code != v1.ExitPermissionDenied {
		t.Errorf("exit %d, want %d", code, v1.ExitPermissionDenied)
	}
	if writeCalls(a.server) != 0 {
		t.Error("a write reached the site")
	}
}

func TestAssignmentShowKeepsMoodlesTextAndReadsItForHumans(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	a.status = lastAttempt("draft", []any{"report.pdf"})

	stdout, stderr, code := a.run("assignment", "show", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.show", stdout)

	var doc struct {
		Data v1.AssignmentDetail `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	// The contract carries Moodle's own text untouched; deciding how to render
	// it belongs to whoever consumes it.
	if doc.Data.Description != "<p>Write <strong>800 words</strong> on scheduling.</p>" {
		t.Errorf("the description was rewritten: %q", doc.Data.Description)
	}
	if doc.Data.DescriptionFormat != 1 {
		t.Errorf("description_format = %d, want 1", doc.Data.DescriptionFormat)
	}
	// Moodle sends -1 for unlimited, which must not be reported as a limit.
	if doc.Data.MaxAttempts != nil {
		t.Errorf("max_attempts = %v, want null for unlimited", *doc.Data.MaxAttempts)
	}
	// The definition is not much use without knowing where you stand in it.
	if doc.Data.Submission.Status != "draft" || doc.Data.Submission.HandedIn {
		t.Errorf("submission = %+v", doc.Data.Submission)
	}

	human, _, code := a.run("assignment", "show", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(human, "<p>") || strings.Contains(human, "<strong>") {
		t.Errorf("raw HTML reached the terminal:\n%s", human)
	}
	if !strings.Contains(human, "Write 800 words on scheduling.") {
		t.Errorf("the description did not survive being made readable:\n%s", human)
	}
	if !strings.Contains(human, "NOT handed in") {
		t.Errorf("show does not say the work is unsubmitted:\n%s", human)
	}
}

func TestAssignmentShowOnAnUnknownIdIsNotFound(t *testing.T) {
	a := newAssignmentFixture(t, true, false)
	_, _, code := a.run("assignment", "show", "999")
	if code != v1.ExitNotFound {
		t.Errorf("exit %d, want %d", code, v1.ExitNotFound)
	}
}

func TestAssignmentShowListsItsAttachments(t *testing.T) {
	// A description that says "see the rubric" is no use without the rubric.
	a := newAssignmentFixture(t, true, false)
	stdout, stderr, code := a.run("assignment", "show", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.show", stdout)

	var doc struct {
		Data v1.AssignmentDetail `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data.Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(doc.Data.Attachments))
	}
	attachment := doc.Data.Attachments[0]
	if attachment.Name != "rubric.txt" || attachment.URL == "" {
		t.Errorf("attachment = %+v", attachment)
	}
	if attachment.External {
		t.Error("a file held by the site was marked external")
	}
}

func TestAssignmentStatusListsWhatMoodleActuallyHolds(t *testing.T) {
	// The count alone does not let anyone check that what arrived is what they
	// meant to send.
	a := newAssignmentFixture(t, true, false)
	a.status = lastAttempt("submitted", []any{"report.pdf"})

	stdout, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", stdout)

	var doc struct {
		Data v1.SubmissionState `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data.Files) != 1 || doc.Data.Files[0].Name != "report.pdf" {
		t.Errorf("files = %+v", doc.Data.Files)
	}
}
