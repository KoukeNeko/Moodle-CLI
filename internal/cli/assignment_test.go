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
	if doc.Data[0].NeedsHandIn == nil || !*doc.Data[0].NeedsHandIn {
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

func TestNoSubmissionSummaryIsNotAnAssignmentThatClosed(t *testing.T) {
	// Moodle omits lastattempt entirely for an account that is not a student
	// on the assignment — a TA on a course they do not take. The zero value
	// underneath reads as submissionsenabled=false, which would come out as
	// "this assignment is not accepting submissions": a wrong statement about
	// the assignment instead of a true one about the account.
	a := newAssignmentFixture(t, true, false)
	a.status = map[string]any{
		"gradingsummary": map[string]any{"participantcount": 31},
		"warnings":       []any{},
	}

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitUnavailable {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitUnavailable, stdout+stderr)
	}
	if !strings.Contains(stderr, "staff") {
		t.Errorf("the message does not say why there is nothing to report:\n%s", stderr)
	}
	if strings.Contains(stderr, "not accepting submissions") {
		t.Errorf("an absent summary was reported as a closed assignment:\n%s", stderr)
	}
}

func TestAnAbsentSummaryWithoutAGradingSummaryDoesNotClaimWho(t *testing.T) {
	// gradingsummary 缺席時，lastattempt 的缺席**不足以**說出這是誰：站台可以把
	// mod/assign:viewownsubmissionsummary 從 student 角色擋掉，學生拿到的回應就
	// 跟助教一字不差。實測（CS1001、course context 設 CAP_PROHIBIT）：
	//
	//   改之前：assignmentdata, lastattempt, warnings
	//   改之後：assignmentdata, warnings        ← 與助教相同
	//
	// 所以這一條只能講站台做了什麼，不能替帳號宣稱身分。
	a := newAssignmentFixture(t, true, false)
	a.status = map[string]any{"warnings": []any{}}

	_, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitUnavailable {
		t.Fatalf("exit %d, want %d", code, v1.ExitUnavailable)
	}
	if strings.Contains(stderr, "not accepting submissions") {
		t.Errorf("an absent summary was reported as a closed assignment:\n%s", stderr)
	}
	if strings.Contains(stderr, "you are staff") {
		t.Errorf("nothing in the reply says this account is staff:\n%s", stderr)
	}
	if !strings.Contains(stderr, "mod/assign:viewownsubmissionsummary") {
		t.Errorf("the message does not name what the site withheld:\n%s", stderr)
	}
}

func TestACourseTheAccountCannotReadIsNotAnEmptyCourse(t *testing.T) {
	// Moodle 對「這門課你看不到」的回答是一次**成功**的呼叫加一筆 warning，
	// 不是錯誤。把空清單照原樣傳上去，就變成對一門讀不到的課說「這裡沒有作業」。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{},
		"warnings": []any{map[string]any{
			"item": "course", "itemid": 2, "warningcode": "2",
			"message": "User is not enrolled or does not have requested capability",
		}},
	})

	stdout, stderr, code := a.run("assignment", "list", "--course", "2")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitPermissionDenied, stderr)
	}
	if stdout != "" {
		t.Errorf("a refusal wrote to stdout:\n%s", stdout)
	}
	if strings.Contains(stderr, "No assignments") {
		t.Errorf("a course the account cannot read was reported as an empty one:\n%s", stderr)
	}
	if !strings.Contains(stderr, "not enrolled") {
		t.Errorf("warning code 2 is about enrolment, and saying so is what "+
			"tells a manager they could still read the course:\n%s", stderr)
	}
}

func TestACourseWithNoAccessRightsIsNotCalledUnenrolled(t *testing.T) {
	// 代碼 1 是 validate_context 擋下來的，跟「沒選課」是兩回事。講成沒選課，
	// 就會把一個連課程頁都打不開的帳號，說成只差一筆選課紀錄。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{},
		"warnings": []any{map[string]any{
			"item": "course", "itemid": 2, "warningcode": "1",
			"message": "No access rights in course context",
		}},
	})

	_, stderr, code := a.run("assignment", "list", "--course", "2")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d", code, v1.ExitPermissionDenied)
	}
	if strings.Contains(stderr, "not enrolled") {
		t.Errorf("a course the account cannot reach was reported as one it "+
			"merely is not enrolled on:\n%s", stderr)
	}
}

func TestOneUnreadableCourseAmongReadableOnesIsMarkedPartial(t *testing.T) {
	// 讀得到的那門課的資料不該被丟掉，但答案不完整這件事必須看得出來——
	// 契約裡的 meta.partial 就是為此存在的。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{map[string]any{
			"id": 15, "shortname": "CS5006",
			"assignments": []any{map[string]any{
				"id": 7, "cmid": 12, "course": 15, "name": "Essay 1",
				"duedate": 1789000000, "cutoffdate": 0,
				"intro": "", "introformat": 1, "grade": 100,
				"allowsubmissionsfromdate": 0, "maxattempts": -1, "timelimit": 0,
				"teamsubmission": 0, "blindmarking": 0,
				"introattachments": []any{}, "configs": pluginConfigs(nil),
				"submissiondrafts": boolToInt(true), "requiresubmissionstatement": 0,
			}},
		}},
		"warnings": []any{map[string]any{
			"item": "course", "itemid": 2, "warningcode": "2",
			"message": "User is not enrolled or does not have requested capability",
		}},
	})

	stdout, stderr, code := a.run("assignment", "list", "--course", "2", "--course", "15", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, %s", code, stderr)
	}
	validate(t, "assignment.list", stdout)

	var doc struct {
		Data []v1.Assignment `json:"data"`
		Meta struct {
			Partial bool `json:"partial"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Data) != 1 {
		t.Fatalf("got %d assignments, want the one readable course's", len(doc.Data))
	}
	if !doc.Meta.Partial {
		t.Error("a list missing a course the account cannot read was reported as complete")
	}
}

func TestAnActivityTheSiteWithheldIsNotAnEmptyCourse(t *testing.T) {
	// 可用性限制（日期／成績／分組）與活動層級的權限覆寫，站台的回答是把課程讀給你，
	// 再把作業一筆一筆拿掉，每一筆配一個 item=module 的 warning。全部被拿掉時，
	// 照原樣印出去就是對一門讀得到的課說「這裡沒有作業」。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{map[string]any{
			"id": 15, "shortname": "CS5006", "assignments": []any{},
		}},
		"warnings": []any{
			map[string]any{"item": "module", "itemid": 44, "warningcode": "1",
				"message": "No access rights in module context"},
			map[string]any{"item": "module", "itemid": 45, "warningcode": "1",
				"message": "No access rights in module context"},
		},
	})

	stdout, stderr, code := a.run("assignment", "list", "--course", "15")
	if code != v1.ExitPermissionDenied {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitPermissionDenied, stderr)
	}
	if stdout != "" {
		t.Errorf("a refusal wrote to stdout:\n%s", stdout)
	}
	if strings.Contains(stderr, "No assignments") {
		t.Errorf("a course whose activities were all withheld was reported as "+
			"an empty one:\n%s", stderr)
	}
	if strings.Contains(stderr, "not enrolled") {
		t.Errorf("the course itself was read, so enrolment is not the reason:\n%s", stderr)
	}
	if !strings.Contains(stderr, "44") || !strings.Contains(stderr, "45") {
		t.Errorf("the message does not name what the site withheld:\n%s", stderr)
	}
}

func TestAPartialListSaysSoInHumanOutput(t *testing.T) {
	// 站台把讀不到的課／被限制的活動從一次**成功**的回覆裡拿掉，契約用 meta.partial
	// 記下來——但那只有 --json 的讀者看得到。人類看到的是一張短了幾行的表，
	// 沒有任何東西告訴他這張表是不完整的。提示走 stderr，管線收到的仍然只有資料。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{map[string]any{
			"id": 15, "shortname": "CS5006",
			"assignments": []any{map[string]any{
				"id": 7, "cmid": 12, "course": 15, "name": "Essay 1",
				"duedate": 1789000000, "cutoffdate": 0,
				"intro": "", "introformat": 1, "grade": 100,
				"allowsubmissionsfromdate": 0, "maxattempts": -1, "timelimit": 0,
				"teamsubmission": 0, "blindmarking": 0,
				"introattachments": []any{}, "configs": pluginConfigs(nil),
				"submissiondrafts": boolToInt(true), "requiresubmissionstatement": 0,
			}},
		}},
		"warnings": []any{map[string]any{
			"item": "module", "itemid": 13, "warningcode": "1",
			"message": "No access rights in module context",
		}},
	})

	stdout, stderr, code := a.run("assignment", "list", "--course", "15")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Essay 1") {
		t.Fatalf("the readable assignment was dropped:\n%s", stdout)
	}
	if !strings.Contains(stderr, "incomplete") {
		t.Errorf("a filtered list was presented as the whole answer:\n%s", stderr)
	}
	if strings.Contains(stdout, "incomplete") {
		t.Errorf("the notice went into the data a pipe receives:\n%s", stdout)
	}
}

func TestACompleteListIsNotAnnouncedAsPartial(t *testing.T) {
	a := newAssignmentFixture(t, true, false)

	stdout, stderr, code := a.run("assignment", "list")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if stdout == "" {
		t.Fatal("nothing was listed")
	}
	if strings.Contains(stderr, "incomplete") {
		t.Errorf("a complete answer was called incomplete:\n%s", stderr)
	}
}

func TestAWithheldAssignmentIsNotSentBackToTheListThatHidesIt(t *testing.T) {
	// 被限制擋掉的作業，在這份清單裡跟「已經被刪掉」長得一模一樣，我們分不出來。
	// 但把人指回 `assignment list`——那裡同樣看不到它——等於讓他繞回原地。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionAssignments, map[string]any{
		"courses": []any{map[string]any{
			"id": 15, "shortname": "CS5006", "assignments": []any{},
		}},
		"warnings": []any{map[string]any{
			"item": "module", "itemid": 45, "warningcode": "1",
			"message": "No access rights in module context",
		}},
	})

	_, stderr, code := a.run("assignment", "show", "26")
	if code != v1.ExitNotFound {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitNotFound, stderr)
	}
	if strings.Contains(stderr, "moodle assignment list") {
		t.Errorf("the reader was sent to a list the assignment is also missing from:\n%s", stderr)
	}
	if !strings.Contains(stderr, "45") {
		t.Errorf("the message does not say the reply was short:\n%s", stderr)
	}
}

func TestASubmissionTheSiteWouldNotSaveNamesWhy(t *testing.T) {
	// save_submission 把理由放在 item，把固定的 "Could not save submission."
	// 放在 message——generate_warning() 的 detail 是最後一個參數。報 message
	// 就是把唯一沒有內容的那一半拿給讀的人看。而且這不是站台壞掉：請求合法、
	// 權限也有，是作業當下的狀態不接受，那是衝突。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionSaveSubmission, []any{
		map[string]any{
			"item": "The due date for this assignment has now passed", "itemid": 7,
			"warningcode": "couldnotsavesubmission", "message": "Could not save submission.",
		},
		map[string]any{
			"item": "File submissions are disabled", "itemid": 7,
			"warningcode": "couldnotsavesubmission", "message": "Could not save submission.",
		},
	})

	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--yes")
	if code != v1.ExitConflict {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitConflict, stderr)
	}
	if !strings.Contains(stderr, "due date") {
		t.Errorf("the reason the site gave is missing:\n%s", stderr)
	}
	if !strings.Contains(stderr, "File submissions are disabled") {
		t.Errorf("only the first of several reasons was shown:\n%s", stderr)
	}
}

func TestAGradingRefusalDoesNotShowMoodlesDebugLine(t *testing.T) {
	// submit_for_grading 的 item 是給開發者看的（"User id: 7, Assignment id: 27
	// Notices:"），不是給學生看的。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionSaveSubmission, []any{})
	a.server.HandleValue(moodle.FunctionSubmitForGrading, []any{
		map[string]any{
			"item": "User id: 7, Assignment id: 7 Notices:", "itemid": 7,
			"warningcode": "couldnotsubmitforgrading",
			"message":     "Could not submit assignment for grading.",
		},
	})

	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--yes")
	if code != v1.ExitConflict {
		t.Fatalf("exit %d, want %d\n%s", code, v1.ExitConflict, stderr)
	}
	if strings.Contains(stderr, "User id:") {
		t.Errorf("a debugging line was shown to the student:\n%s", stderr)
	}
}

func TestARefusalOnOneAssignmentDoesNotSuppressTheNext(t *testing.T) {
	// 站台的拒絕是針對某一門課、某一個活動、某一位使用者，不是針對「這支函式」。
	// 把拒絕記在函式頭上，下一個本來有權限的問題就會被一個過期的「不行」擋掉，
	// 而且完全不會去問站台。
	a := newAssignmentFixture(t, true, false)
	a.server.FailException(moodle.FunctionSubmissionStatus,
		"required_capability_exception", "nopermission", "error/nopermission")

	if _, _, code := a.run("assignment", "status", "7"); code != v1.ExitPermissionDenied {
		t.Fatalf("first call: exit %d, want %d", code, v1.ExitPermissionDenied)
	}
	// 站台這一次願意回答了——換一門課、換一個活動就是這樣。
	a.server.FailException(moodle.FunctionSubmissionStatus, "", "", "")
	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("the second call was refused without asking the site: exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Status:") {
		t.Errorf("the second call answered nothing:\n%s", stdout)
	}
}

func TestAnAccountThatIsBothStudentAndStaffIsStillAskedAboutItself(t *testing.T) {
	// 同一門課可以同時給一個帳號 student 與 teacher 兩個角色，Moodle 取聯集：
	// 實測站台會**兩個鍵都回**（lastattempt 與 gradingsummary 同時存在）。
	// 挑一個「人設」來套用就會答錯——問的是「我的繳交狀態」，而他確實有一筆。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionSubmissionStatus, map[string]any{
		"lastattempt":    lastAttempt("new", nil)["lastattempt"],
		"gradingsummary": map[string]any{"participantcount": 3, "submissionssubmittedcount": 1},
		"assignmentdata": map[string]any{},
		"warnings":       []any{},
	})

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "nothing submitted yet") {
		t.Errorf("an account that is also staff was not answered about itself:\n%s", stdout)
	}
	if strings.Contains(stderr, "you are staff") {
		t.Errorf("a participant was told they are staff:\n%s", stderr)
	}
}

func TestAGrantedExtensionIsShownBesideTheCutOffItOutlives(t *testing.T) {
	// 展延是發給個人的，Moodle 把它折進「還能不能交」，不會改作業上公布的日期。
	// 所以一個拿到展延的學生看到的是一個**已經過去**的截止日，而他其實還能交。
	// 實測：cutoff 在 2 天前、展延到 5 天後，站台回 canedit=true。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("new", nil)
	attempt, _ := state["lastattempt"].(map[string]any)
	attempt["extensionduedate"] = 1789600000
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Extension:") {
		t.Errorf("an extension this account holds was not shown:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", jsonOut)
	if !strings.Contains(jsonOut, `"extension_due_date":"`) {
		t.Errorf("the contract did not carry the extension:\n%s", jsonOut)
	}
}

func TestNoExtensionIsNotReportedAsOne(t *testing.T) {
	// Moodle 對「沒有展延」送的是 0，照讀會變成 1970 年——那是一個看起來很具體
	// 的錯答案。
	a := newAssignmentFixture(t, true, false)

	stdout, _, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "Extension:") {
		t.Errorf("an assignment with no extension reported one:\n%s", stdout)
	}
	if strings.Contains(stdout, "1970") {
		t.Errorf("zero was read as a date:\n%s", stdout)
	}
}

func TestAReopenedAttemptIsNotCalledADraft(t *testing.T) {
	// 評分者重開一次繳交機會，跟「存了草稿沒交」是兩件事：先前那一次**交過了**，
	// 而且通常已經被改過。講成 draft，等於對一個交過作業的人說他從來沒交。
	// translateStatus 正上方的註解本來就禁止這種折疊，reopened 卻是唯一的例外。
	a := newAssignmentFixture(t, true, false)
	a.server.HandleValue(moodle.FunctionSubmissionStatus,
		lastAttempt("reopened", []any{"attempt1.pdf"}))

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if strings.Contains(stdout, "draft") {
		t.Errorf("a reopened attempt was reported as a draft:\n%s", stdout)
	}
	if !strings.Contains(stdout, "reopened") {
		t.Errorf("the state Moodle reported was not passed on:\n%s", stdout)
	}
	if !strings.Contains(stdout, "earlier one was handed in") {
		t.Errorf("nothing says the earlier attempt was submitted:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", jsonOut)
	if !strings.Contains(jsonOut, `"status":"reopened"`) {
		t.Errorf("the contract folded reopened into another state:\n%s", jsonOut)
	}
	if !strings.Contains(jsonOut, `"handed_in":false`) {
		t.Errorf("a reopened attempt is not currently handed in:\n%s", jsonOut)
	}
}

func TestAGroupSubmissionSaysWhoseStateThatIs(t *testing.T) {
	// 團體作業上，「Handed in: yes」講的是**這一組**，不是問的人。作業本身的
	// teamsubmission 設定在另一支呼叫裡，status 這條路從來不會去問，所以讀的人
	// 沒有任何東西可以判斷。實測 ug1 已交、ug2 沒交，兩個人看到的都是這一組的狀態。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("submitted", []any{"report.pdf"})
	attempt, _ := state["lastattempt"].(map[string]any)
	attempt["submissiongroup"] = 18
	attempt["submissiongroupmemberswhoneedtosubmit"] = []any{11}
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "group submission") {
		t.Errorf("a group's state was presented as this account's:\n%s", stdout)
	}
	if !strings.Contains(stdout, "1 group member still has to submit") {
		t.Errorf("a group that is not finished was not reported as unfinished:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", jsonOut)
	if !strings.Contains(jsonOut, `"group_submission":true`) ||
		!strings.Contains(jsonOut, `"members_still_to_submit":1`) {
		t.Errorf("the contract did not carry the group state:\n%s", jsonOut)
	}
}

func TestAnIndividualSubmissionIsNotCalledAGroupOne(t *testing.T) {
	a := newAssignmentFixture(t, true, false)

	stdout, _, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "group submission") {
		t.Errorf("an individual submission was reported as a group's:\n%s", stdout)
	}
	if strings.Contains(stdout, "Waiting:") {
		t.Errorf("an individual submission was said to be waiting on others:\n%s", stdout)
	}
}

func TestAnEarlierAttemptIsNotLostWhenTheAssignmentIsReopened(t *testing.T) {
	// 評分者重開之後，當前那一次真的是空的——所以「Files: 0 / Handed in: no」
	// 每一行都是真的，合起來卻讀成「你從來沒交過」。學生交過的東西在
	// previousattempts 裡，而那是**學生本人也收得到**的（實測，不需要評分權限）。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("reopened", nil)
	state["previousattempts"] = []any{map[string]any{
		"attemptnumber": 0,
		"submission": map[string]any{
			"id": 32, "status": "submitted", "timemodified": 1789000000,
			"plugins": []any{map[string]any{
				"type": "file",
				"fileareas": []any{map[string]any{
					"area": "submission_files",
					"files": []any{map[string]any{
						"filename": "report.pdf", "filepath": "/", "filesize": 20,
						"fileurl":      "https://moodle.example.edu/webservice/pluginfile.php/1/a/b/report.pdf",
						"timemodified": 1789000000, "mimetype": "application/pdf",
						"isexternalfile": false,
					}},
				}},
			}},
		},
	}}
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Earlier:") {
		t.Errorf("work the student handed in went unmentioned:\n%s", stdout)
	}
	if !strings.Contains(stdout, "1 file") {
		t.Errorf("the earlier attempt's files were not counted:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", jsonOut)
	if !strings.Contains(jsonOut, `"earlier_attempts":[{"number":0`) {
		t.Errorf("the contract did not carry the earlier attempt:\n%s", jsonOut)
	}
}

func TestAFirstAttemptHasNoHistoryRatherThanNull(t *testing.T) {
	// 從來沒有被重開的作業，歷史是**空陣列**而不是 null：空陣列說「沒有更早的
	// 一次」，null 會被讀成「這條路線查不到」。
	a := newAssignmentFixture(t, true, false)

	stdout, _, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "Earlier:") {
		t.Errorf("an assignment that was never reopened reported a history:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(jsonOut, `"earlier_attempts":[]`) {
		t.Errorf("an absent history must be the empty array, not null:\n%s", jsonOut)
	}
}

func TestARunningTimeLimitIsShownWithoutBeingCalledADeadline(t *testing.T) {
	// 限時作業一旦開始，時限可能比截止日早幾個星期到期——而只讀截止日的學生
	// 完全不知道有計時器在跑。但它**不是死線**：Moodle 逾時之後照樣收件、標記
	// 成超時，真正關掉繳交的是 cutoff。講成 deadline 是往另一個方向的錯答案。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("draft", nil)
	attempt, _ := state["lastattempt"].(map[string]any)
	attempt["timelimit"] = 3600
	submission, _ := attempt["submission"].(map[string]any)
	submission["timestarted"] = 1789000000
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Timer:") {
		t.Errorf("a running time limit went unmentioned:\n%s", stdout)
	}
	if strings.Contains(stdout, "Deadline") || strings.Contains(stdout, "deadline") {
		t.Errorf("the timer was called a deadline, which it is not:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", jsonOut)
	if !strings.Contains(jsonOut, `"timer_ends_at":"`) {
		t.Errorf("the contract did not carry the timer:\n%s", jsonOut)
	}
}

func TestATimeLimitNotYetStartedIsNotRunning(t *testing.T) {
	// 時限存在不代表它在跑。Moodle 是在學生按下開始時才用伺服器時間寫
	// timestarted，所以它缺席就是「還沒開始」——報成正在倒數，會讓人以為
	// 自己錯過了一個根本還沒啟動的計時器。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("new", nil)
	attempt, _ := state["lastattempt"].(map[string]any)
	attempt["timelimit"] = 3600
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	stdout, _, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "Timer:") {
		t.Errorf("a time limit that has not started was reported as running:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(jsonOut, `"timer_ends_at":null`) {
		t.Errorf("a timer that is not running must be null:\n%s", jsonOut)
	}
}

func TestAnOfflineAssignmentIsNotAClosedOne(t *testing.T) {
	// 線下作業（所有繳交外掛都關掉，Moodle 把這件事快取成 nosubmissions）是一個
	// 刻意的設定，不是故障：老師照樣可以給分。實測站台上它有 76 分在成績簿裡，
	// 而 assignment status 卻整個拒絕回答（exit 9「這條路走不通」）。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("new", nil)
	attempt, _ := state["lastattempt"].(map[string]any)
	attempt["submissionsenabled"] = false
	attempt["canedit"] = false
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	stdout, stderr, code := a.run("assignment", "status", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d, want 0 — an offline assignment has a status\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "no online submission") {
		t.Errorf("nothing says why there is nothing to hand in:\n%s", stdout)
	}

	jsonOut, _, code := a.run("assignment", "status", "7", "--json")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	validate(t, "assignment.status", jsonOut)
	if !strings.Contains(jsonOut, `"online_submission":false`) {
		t.Errorf("the contract did not carry it:\n%s", jsonOut)
	}
}

func TestSubmittingToAnOfflineAssignmentSaysWhyNot(t *testing.T) {
	// 擋是對的，但原因不是「可能過期了、可能被鎖了」——那三個猜測對一個老師
	// 刻意設定的線下作業每一個都是錯的。
	a := newAssignmentFixture(t, true, false)
	state := lastAttempt("new", nil)
	attempt, _ := state["lastattempt"].(map[string]any)
	attempt["submissionsenabled"] = false
	attempt["canedit"] = false
	a.server.HandleValue(moodle.FunctionSubmissionStatus, state)

	_, stderr, code := a.run("assignment", "submit", "7", workFile(t), "--dry-run")
	if code == v1.ExitOK {
		t.Fatal("a submission was planned for an assignment that takes none")
	}
	if !strings.Contains(stderr, "no online submission") {
		t.Errorf("the refusal does not name the real reason:\n%s", stderr)
	}
	if strings.Contains(stderr, "cut-off may have passed") {
		t.Errorf("a deliberate setting was reported as a missed deadline:\n%s", stderr)
	}
}

func TestAnonymousMarkingExplainsAMissingGradeWithoutPromisingAnonymity(t *testing.T) {
	// 匿名評分時 Moodle 會把已經改好的分數扣住，等揭露身分才放進成績簿
	// （`is_blind_marking() && !is_marking_anonymous()` 就不推送）。所以一個空的
	// 分數可能是「被扣住」而不是「沒人改」——學生需要知道這件事。
	//
	// 但**不能**講成「沒有評分者認得出你」：持有 mod/assign:viewblinddetails
	// 之類權限的角色看得到，那是一個我們守不住的承諾。
	a := newAssignmentFixture(t, true, false, "onlinetext")
	a.server.HandleValue(moodle.FunctionAssignments, blindAssignment(false))

	stdout, stderr, code := a.run("assignment", "show", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "withheld until identities are revealed") {
		t.Errorf("nothing explains why a grade may be missing:\n%s", stdout)
	}
	for _, overclaim := range []string{"cannot identify", "no grader", "nobody can"} {
		if strings.Contains(stdout, overclaim) {
			t.Errorf("a promise this tool cannot keep (%q):\n%s", overclaim, stdout)
		}
	}
}

func TestRevealedIdentitiesAreNotStillCalledAnonymous(t *testing.T) {
	a := newAssignmentFixture(t, true, false, "onlinetext")
	a.server.HandleValue(moodle.FunctionAssignments, blindAssignment(true))

	stdout, _, code := a.run("assignment", "show", "7")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "identities have been revealed") {
		t.Errorf("an assignment whose anonymity was lifted still reads as anonymous:\n%s", stdout)
	}
	if strings.Contains(stdout, "withheld until") {
		t.Errorf("a grade that can now be released was said to be withheld:\n%s", stdout)
	}
}

// blindAssignment is the fixture's assignment with anonymous marking on.
func blindAssignment(revealed bool) map[string]any {
	reveal := 0
	if revealed {
		reveal = 1
	}
	return map[string]any{
		"courses": []any{map[string]any{
			"id": 2, "shortname": "CS204",
			"assignments": []any{map[string]any{
				"id": 7, "cmid": 12, "course": 2, "name": "Essay 1",
				"duedate": 1789000000, "cutoffdate": 0,
				"intro": "", "introformat": 1, "grade": 100,
				"allowsubmissionsfromdate": 0, "maxattempts": -1, "timelimit": 0,
				"teamsubmission": 0, "blindmarking": 1, "revealidentities": reveal,
				"introattachments": []any{}, "configs": pluginConfigs(nil),
				"submissiondrafts": boolToInt(true), "requiresubmissionstatement": 0,
			}},
		}},
	}
}
