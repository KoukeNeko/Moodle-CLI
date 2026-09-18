package assignment_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// fake stands in for Moodle, recording what was called and letting a test set
// what the site reports afterwards.
type fake struct {
	summary assignment.Summary
	// states is returned in order, so a test can make the read-back differ
	// from the state before the write.
	states  []assignment.State
	stateAt int
	// statusFailFrom makes Status fail from the Nth call onwards (1-based),
	// so a test can make the read-back itself unavailable. 0 means never.
	statusFailFrom int

	uploadErr error
	saveErr   error
	handInErr error

	uploaded     int
	draftAreas   int
	saved        int
	savedContent assignment.Content
	handedIn     int
	acceptedFlag bool
}

func (f *fake) Name() site.BackendKind        { return site.BackendWS }
func (f *fake) Requirement() site.Requirement { return site.Requirement{} }

func (f *fake) List(context.Context, []string) (assignment.ListResult, error) {
	return assignment.ListResult{
		Assignments: []assignment.Summary{f.summary},
		Provenance:  site.NewProvenance(site.BackendWS),
	}, nil
}

func (f *fake) Show(_ context.Context, id string) (assignment.Detail, error) {
	if id != f.summary.ID {
		return assignment.Detail{}, errs.New(errs.CodeNotFound, "no such assignment")
	}
	return assignment.Detail{Summary: f.summary}, nil
}

func (f *fake) Status(context.Context, string) (assignment.State, error) {
	f.stateAt++
	if f.statusFailFrom > 0 && f.stateAt >= f.statusFailFrom {
		return assignment.State{}, errs.New(errs.CodeNetwork, "the site did not answer")
	}
	return f.states[min(f.stateAt-1, len(f.states)-1)], nil
}

func (f *fake) UploadDraft(context.Context, []string) (string, error) {
	f.uploaded++
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	return "9001", nil
}

func (f *fake) NewDraftArea(context.Context) (string, error) {
	f.draftAreas++
	return "9002", nil
}

func (f *fake) SaveSubmission(_ context.Context, _ string, content assignment.Content) error {
	f.saved++
	f.savedContent = content
	return f.saveErr
}

func (f *fake) SubmitForGrading(_ context.Context, _ string, accept bool) error {
	f.handedIn++
	f.acceptedFlag = accept
	return f.handInErr
}

// filePlugin is the minimum an assignment must have for this tool to submit
// to it at all.
var filePlugin = []string{assignment.PluginFile}

func editable(status assignment.Status) assignment.State {
	return assignment.State{Status: status, CanEdit: true, CanSubmit: true}
}

func tempFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(path, []byte("work"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSavingIsNotHandingIn(t *testing.T) {
	// The mistake this package exists to avoid: an assignment that keeps a
	// draft state needs a second call, and a client that stops after saving
	// leaves the student believing the work was handed in.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusSubmitted),
		},
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if err != nil {
		t.Fatal(err)
	}
	if f.handedIn != 1 {
		t.Errorf("submit_for_grading was called %d times, want 1", f.handedIn)
	}
	if !result.HandedIn {
		t.Error("the work was handed in but the result does not say so")
	}
	if result.FinalStatus != assignment.StatusSubmitted {
		t.Errorf("final status = %q", result.FinalStatus)
	}
}

func TestADraftIsNeverReportedAsHandedIn(t *testing.T) {
	// Moodle says the work is still a draft. Whatever this code believes it
	// did, the report follows Moodle.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusDraft), // read-back disagrees
		},
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if err != nil {
		t.Fatal(err)
	}
	if result.HandedIn {
		t.Error("a draft was reported as handed in")
	}
	if result.FinalStatus != assignment.StatusDraft {
		t.Errorf("final status = %q, want draft", result.FinalStatus)
	}
}

func TestAssignmentThatNeedsNoHandInSkipsThatStep(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "1", Name: "A1", SubmissionDrafts: false},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusSubmitted),
		},
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "1", Files: []string{tempFile(t)},
		})
	if err != nil {
		t.Fatal(err)
	}
	if f.handedIn != 0 {
		t.Error("submit_for_grading was called on an assignment that does not use drafts")
	}
	if !result.HandedIn {
		t.Error("Moodle reports it submitted, so the result should say so")
	}
}

func TestSubmissionStatementIsNeverAcceptedForTheStudent(t *testing.T) {
	// Ticking this on their behalf would be signing a declaration for them.
	f := &fake{
		summary: assignment.Summary{
			Plugins: filePlugin,
			ID:      "3", Name: "A3", SubmissionDrafts: true, RequiresStatement: true,
		},
		states: []assignment.State{editable(assignment.StatusNew)},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "3", Files: []string{tempFile(t)},
		})
	if err == nil {
		t.Fatal("the statement requirement was bypassed")
	}
	if code := errs.From(err).Code; code != errs.CodeUsage {
		t.Errorf("code = %q, want usage", code)
	}
	if f.uploaded != 0 || f.saved != 0 {
		t.Error("work was sent before the statement was accepted")
	}
}

func TestAcceptedStatementIsPassedThrough(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{
			Plugins: filePlugin,
			ID:      "3", Name: "A3", SubmissionDrafts: true, RequiresStatement: true,
		},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusSubmitted),
		},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "3", Files: []string{tempFile(t)}, AcceptStatement: true,
		})
	if err != nil {
		t.Fatal(err)
	}
	if !f.acceptedFlag {
		t.Error("the student's acceptance was not passed to Moodle")
	}
}

func TestDryRunSendsNothing(t *testing.T) {
	// Not "reports that it sent nothing": sends nothing.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states:  []assignment.State{editable(assignment.StatusNew)},
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{DryRun: true}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)}, DryRun: true,
		})
	if err != nil {
		t.Fatal(err)
	}
	if f.uploaded != 0 || f.saved != 0 || f.handedIn != 0 {
		t.Errorf("a dry run sent something: uploads=%d saves=%d handins=%d",
			f.uploaded, f.saved, f.handedIn)
	}
	if result.Outcome != assignment.OutcomePlanned {
		t.Errorf("outcome = %q, want planned", result.Outcome)
	}
	if result.HandedIn {
		t.Error("a dry run must never report work as handed in")
	}
	// The plan has to say what it would do, or it is not useful.
	var planned int
	for _, step := range result.Steps {
		if step.Status == assignment.StepPlanned {
			planned++
		}
	}
	if planned < 3 {
		t.Errorf("only %d planned steps were reported", planned)
	}
}

func TestReadOnlyModeBlocksTheWrite(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2"},
		states:  []assignment.State{editable(assignment.StatusNew)},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{ReadOnly: true}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if err == nil {
		t.Fatal("read-only mode allowed a submission")
	}
	if f.saved != 0 {
		t.Error("the write ran despite read-only mode")
	}
}

func TestAlreadySubmittedIsAConflictNotASecondSubmission(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states:  []assignment.State{editable(assignment.StatusSubmitted)},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if err == nil {
		t.Fatal("work was submitted over an existing submission")
	}
	if code := errs.From(err).Code; code != errs.CodeConflict {
		t.Errorf("code = %q, want conflict", code)
	}
	if f.uploaded != 0 {
		t.Error("files were uploaded before the conflict was noticed")
	}
}

func TestAlreadyHandedInBeatsTheLockedReason(t *testing.T) {
	// Moodle reports canedit=false once work is submitted. Reporting that as
	// "this cannot be edited, the due date may have passed" would send the
	// student looking for a problem that is not theirs.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2"},
		states: []assignment.State{
			{Status: assignment.StatusSubmitted, CanEdit: false},
		},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	e := errs.From(err)
	if e == nil || e.Code != errs.CodeConflict {
		t.Fatalf("code = %v, want conflict", e)
	}
	if !strings.Contains(e.Message, "already been handed in") {
		t.Errorf("message = %q", e.Message)
	}
}

func TestLockedAssignmentIsRefusedBeforeAnythingIsSent(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2"},
		states:  []assignment.State{{Status: assignment.StatusNew, CanEdit: false}},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if code := errs.From(err).Code; code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", code)
	}
	if f.uploaded != 0 {
		t.Error("files were uploaded for an assignment that cannot be edited")
	}
}

func TestAmbiguousSaveIsResolvedByReadingTheStateBack(t *testing.T) {
	// The connection dropped after the save. Moodle may or may not have
	// applied it; reading the state is the only way to find out, and it must
	// not be retried blindly.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusDraft), // it did land
		},
		saveErr: errs.New(errs.CodeNetwork, "the response was lost").Ambiguous(),
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if err != nil {
		t.Fatalf("a confirmed save was still reported as a failure: %v", err)
	}
	if result.FinalStatus != assignment.StatusDraft {
		t.Errorf("final status = %q, want draft", result.FinalStatus)
	}
	if f.saved != 1 {
		t.Errorf("the save ran %d times; an ambiguous write must not be repeated", f.saved)
	}
}

func TestAmbiguousSaveThatCannotBeResolvedStaysAmbiguous(t *testing.T) {
	// The state could not be read either. Guessing here would be the worst
	// possible answer, so the caller is told it is unknown.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states:  []assignment.State{editable(assignment.StatusNew)},
		saveErr: errs.New(errs.CodeNetwork, "the response was lost").Ambiguous(),
	}
	// The preflight read works; the read-back after the lost write does not.
	f.statusFailFrom = 2
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)},
		})
	if err == nil {
		t.Fatal("an unresolved ambiguous write was reported as success")
	}
	e := errs.From(err)
	if e.EffectiveOutcome() != errs.OutcomeAmbiguous {
		t.Errorf("outcome = %q, want ambiguous", e.EffectiveOutcome())
	}
	if !strings.Contains(e.Hint, "assignment status") {
		t.Errorf("the hint should tell the user how to check, got %q", e.Hint)
	}
}

func TestMissingFileIsCaughtBeforeAnythingIsSent(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2"},
		states:  []assignment.State{editable(assignment.StatusNew)},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{"/no/such/file.pdf"},
		})
	if err == nil {
		t.Fatal("a missing file was accepted")
	}
	if f.uploaded != 0 {
		t.Error("an upload was attempted for a file that does not exist")
	}
}

func TestTooManyFilesIsCaughtLocally(t *testing.T) {
	// Checking here saves a round trip and gives a clearer message than
	// Moodle's own refusal.
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", MaxFiles: 1},
		states:  []assignment.State{editable(assignment.StatusNew)},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t), tempFile(t)},
		})
	if code := errs.From(err).Code; code != errs.CodeValidation {
		t.Errorf("code = %q, want validation", code)
	}
	if f.uploaded != 0 {
		t.Error("files were uploaded despite exceeding the limit")
	}
}

func TestDraftOnlySkipsHandingIn(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{Plugins: filePlugin, ID: "2", Name: "A2", SubmissionDrafts: true},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusDraft),
		},
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "2", Files: []string{tempFile(t)}, DraftOnly: true,
		})
	if err != nil {
		t.Fatal(err)
	}
	if f.handedIn != 0 {
		t.Error("work was handed in despite --draft")
	}
	if result.HandedIn {
		t.Error("a draft-only save was reported as handed in")
	}
}

func TestSubmittingFilesDoesNotEraseTypedText(t *testing.T) {
	// Moodle saves every enabled plugin in one call, so leaving the online
	// text out of the payload does not leave it alone: it saves it as empty.
	// Whatever the student typed in the browser has to go back unchanged.
	f := &fake{
		summary: assignment.Summary{
			ID: "4", Name: "A4",
			Plugins: []string{assignment.PluginFile, assignment.PluginOnlineText},
		},
		states: []assignment.State{
			{
				Status: assignment.StatusNew, CanEdit: true, CanSubmit: true,
				OnlineText: &assignment.OnlineText{Text: "<p>my essay</p>", Format: 1},
			},
			editable(assignment.StatusSubmitted),
		},
	}
	if _, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "4", Files: []string{tempFile(t)},
		}); err != nil {
		t.Fatal(err)
	}

	text := f.savedContent.OnlineText
	if text == nil {
		t.Fatal("the online text plugin was left out of the save, which blanks it")
	}
	if text.Text != "<p>my essay</p>" {
		t.Errorf("the student's text was changed to %q", text.Text)
	}
	if text.Format != 1 {
		t.Errorf("the text format was rewritten to %d", text.Format)
	}
	// It needs a draft area of its own: reusing the file one makes Moodle move
	// the uploaded files into the text plugin's storage, and the save is then
	// rejected as empty.
	if text.DraftID == "" || text.DraftID == f.savedContent.FileDraftID {
		t.Errorf("online text draft %q must be separate from the file draft %q",
			text.DraftID, f.savedContent.FileDraftID)
	}
}

func TestAssignmentWithoutFileSubmissionsIsRefusedEarly(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{
			ID: "5", Name: "A5", Plugins: []string{assignment.PluginOnlineText},
		},
		states: []assignment.State{editable(assignment.StatusNew)},
	}
	_, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "5", Files: []string{tempFile(t)},
		})
	if code := errs.From(err).Code; code != errs.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", code)
	}
	if f.uploaded != 0 {
		t.Error("files were uploaded to an assignment that does not take them")
	}
}

func TestNoDraftAreaIsAllocatedWhenThereIsNoOnlineText(t *testing.T) {
	f := &fake{
		summary: assignment.Summary{ID: "1", Name: "A1", Plugins: filePlugin},
		states: []assignment.State{
			editable(assignment.StatusNew),
			editable(assignment.StatusSubmitted),
		},
	}
	if _, err := assignment.NewSubmitter(f, f, safety.Mode{}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "1", Files: []string{tempFile(t)},
		}); err != nil {
		t.Fatal(err)
	}
	if f.draftAreas != 0 {
		t.Error("a draft area was allocated for a plugin the assignment does not use")
	}
	if f.savedContent.OnlineText != nil {
		t.Error("online text was sent to an assignment that does not enable it")
	}
}

func TestDryRunDescribesTheStatementRequirementInsteadOfRefusing(t *testing.T) {
	// A dry run writes nothing, so refusing it would hide the very
	// requirement the user ran it to discover.
	f := &fake{
		summary: assignment.Summary{
			Plugins: filePlugin,
			ID:      "3", Name: "A3", SubmissionDrafts: true, RequiresStatement: true,
		},
		states: []assignment.State{editable(assignment.StatusNew)},
	}
	result, err := assignment.NewSubmitter(f, f, safety.Mode{DryRun: true}).
		Submit(context.Background(), assignment.SubmitRequest{
			AssignmentID: "3", Files: []string{tempFile(t)}, DryRun: true,
		})
	if err != nil {
		t.Fatalf("the dry run was refused: %v", err)
	}
	if f.uploaded != 0 || f.saved != 0 || f.handedIn != 0 {
		t.Fatal("the dry run sent something")
	}
	var said bool
	for _, step := range result.Steps {
		if step.Name == assignment.StepHandIn &&
			strings.Contains(step.Detail, "--accept-statement") {
			said = true
		}
	}
	if !said {
		t.Error("the plan does not mention that the statement must be accepted")
	}
}
