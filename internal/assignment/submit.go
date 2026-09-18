package assignment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// StepName identifies one stage of handing work in.
type StepName string

const (
	StepPreflight StepName = "preflight"
	StepUpload    StepName = "upload"
	StepSave      StepName = "save_submission"
	StepHandIn    StepName = "submit_for_grading"
	StepVerify    StepName = "verify"
)

// StepStatus is what happened to a step.
type StepStatus string

const (
	StepDone    StepStatus = "done"
	StepSkipped StepStatus = "skipped"
	StepPlanned StepStatus = "planned"
	StepFailed  StepStatus = "failed"
)

// Step is one stage and its outcome.
type Step struct {
	Name   StepName
	Status StepStatus
	Detail string
}

// Outcome says what happened overall.
type Outcome string

const (
	// OutcomeApplied means the work reached the state the caller asked for,
	// confirmed by reading it back.
	OutcomeApplied Outcome = "applied"
	// OutcomeNotApplied means nothing changed.
	OutcomeNotApplied Outcome = "not_applied"
	// OutcomePlanned is the result of a dry run.
	OutcomePlanned Outcome = "planned"
)

// SubmitRequest is what the caller wants done.
type SubmitRequest struct {
	AssignmentID string
	Files        []string
	// AcceptStatement records that the student agreed to the submission
	// statement. It is never inferred: agreeing on their behalf would be
	// signing something for them.
	AcceptStatement bool
	// DraftOnly saves the work without handing it in.
	DraftOnly bool
	DryRun    bool
}

// SubmitResult is what happened, as read back from Moodle.
type SubmitResult struct {
	Outcome Outcome
	// FinalStatus is the state Moodle reports after the fact, not the state
	// this code believes it produced.
	FinalStatus Status
	// HandedIn distinguishes work that is being marked from work that is
	// merely saved. This is the distinction other clients lose.
	HandedIn   bool
	Steps      []Step
	Assignment Summary
	Provenance site.Provenance
}

// Submitter hands work in.
type Submitter struct {
	backend Backend
	writer  Writer
	guard   safety.Guard
}

// NewSubmitter builds a Submitter.
func NewSubmitter(backend Backend, writer Writer, mode safety.Mode) *Submitter {
	return &Submitter{backend: backend, writer: writer, guard: safety.Guard{Mode: mode}}
}

// Submit hands work in and reports what Moodle says afterwards.
//
// The shape of the flow is forced by Moodle's model: saving content and
// handing it in are separate, and which one is enough depends on the
// assignment's settings. Reporting success after the first step alone is the
// mistake this method exists to avoid.
func (s *Submitter) Submit(ctx context.Context, req SubmitRequest) (SubmitResult, error) {
	result := SubmitResult{Provenance: site.NewProvenance(site.BackendWS)}

	summary, state, err := s.preflight(ctx, req)
	if err != nil {
		return result, err
	}
	result.Assignment = summary
	result.Steps = append(result.Steps, Step{
		Name: StepPreflight, Status: StepDone,
		Detail: fmt.Sprintf("%s, currently %s", summary.Name, state.Status),
	})

	// Whether handing in is a separate act is the assignment's decision, not
	// ours, and it is read from the site rather than assumed. The submitting
	// route always knows; a route that does not would have been refused
	// before reaching here.
	drafts, known := summary.NeedsHandIn()
	if !known {
		return result, errs.New(errs.CodeUnavailable,
			"cannot tell whether this assignment needs a separate hand-in").
			WithReason(errs.ReasonCapability).
			WithHint("submitting needs a web service token, or the result could not be reported truthfully")
	}
	needsHandIn := drafts && !req.DraftOnly

	if req.DryRun {
		result.Outcome = OutcomePlanned
		result.FinalStatus = state.Status
		result.Steps = append(result.Steps,
			Step{Name: StepUpload, Status: StepPlanned,
				Detail: fmt.Sprintf("%d file(s) to the draft area", len(req.Files))},
			Step{Name: StepSave, Status: StepPlanned, Detail: "attach the draft to the submission"})
		if needsHandIn {
			detail := "hand in for grading"
			if summary.RequiresStatement && !req.AcceptStatement {
				detail += "; this assignment needs --accept-statement, so a real run would stop here"
			}
			result.Steps = append(result.Steps, Step{
				Name: StepHandIn, Status: StepPlanned, Detail: detail,
			})
		} else {
			result.Steps = append(result.Steps, Step{
				Name: StepHandIn, Status: StepSkipped,
				Detail: skipReason(summary, req),
			})
		}
		result.Steps = append(result.Steps, Step{
			Name: StepVerify, Status: StepPlanned, Detail: "read the state back",
		})
		return result, nil
	}

	content, err := s.prepare(ctx, summary, state, req)
	if err != nil {
		result.Steps = append(result.Steps, Step{Name: StepUpload, Status: StepFailed})
		return result, err
	}
	result.Steps = append(result.Steps, Step{
		Name: StepUpload, Status: StepDone,
		Detail: fmt.Sprintf("%d file(s)", len(req.Files)),
	})

	if err := s.guard.Do(ctx, "mod_assign_save_submission", func(ctx context.Context) error {
		return s.writer.SaveSubmission(ctx, req.AssignmentID, content)
	}); err != nil {
		result.Steps = append(result.Steps, Step{Name: StepSave, Status: StepFailed})
		return result, s.reconcile(ctx, req, err, &result)
	}
	result.Steps = append(result.Steps, Step{Name: StepSave, Status: StepDone})

	if needsHandIn {
		if err := s.guard.Do(ctx, "mod_assign_submit_for_grading", func(ctx context.Context) error {
			return s.writer.SubmitForGrading(ctx, req.AssignmentID, req.AcceptStatement)
		}); err != nil {
			result.Steps = append(result.Steps, Step{Name: StepHandIn, Status: StepFailed})
			return result, s.reconcile(ctx, req, err, &result)
		}
		result.Steps = append(result.Steps, Step{Name: StepHandIn, Status: StepDone})
	} else {
		result.Steps = append(result.Steps, Step{
			Name: StepHandIn, Status: StepSkipped, Detail: skipReason(summary, req),
		})
	}

	// The answer comes from Moodle, not from the fact that the calls returned
	// without error.
	final, err := s.backend.Status(ctx, req.AssignmentID)
	if err != nil {
		result.Steps = append(result.Steps, Step{Name: StepVerify, Status: StepFailed})
		return result, errs.From(err).WithHint(
			"the submission may have gone through; check with `moodle assignment status`")
	}
	result.Steps = append(result.Steps, Step{
		Name: StepVerify, Status: StepDone, Detail: string(final.Status),
	})
	result.Outcome = OutcomeApplied
	result.FinalStatus = final.Status
	result.HandedIn = final.Status == StatusSubmitted
	return result, nil
}

// preflight reads everything the decision depends on, and writes nothing.
func (s *Submitter) preflight(ctx context.Context, req SubmitRequest) (Summary, State, error) {
	summary, err := s.find(ctx, req.AssignmentID)
	if err != nil {
		return Summary{}, State{}, err
	}
	state, err := s.backend.Status(ctx, req.AssignmentID)
	if err != nil {
		return Summary{}, State{}, err
	}

	// Order matters. Moodle also reports canedit=false once work is handed in,
	// so testing that first would answer "the due date may have passed" to
	// someone whose real situation is that they already submitted.
	if state.Status == StatusSubmitted {
		return Summary{}, State{}, errs.New(errs.CodeConflict,
			"this assignment has already been handed in").
			WithHint("check it with `moodle assignment status`")
	}
	if !state.CanEdit {
		return Summary{}, State{}, errs.New(errs.CodePermissionDenied,
			"this assignment cannot be edited").
			WithHint("the due date may have passed, or the submission may be locked")
	}
	if summary.RequiresStatement && !req.AcceptStatement && !req.DraftOnly && !req.DryRun {
		// Ticking this on the student's behalf would be signing a declaration
		// for them. A dry run is exempt: it writes nothing, and refusing to
		// describe the plan would hide exactly the requirement it should be
		// pointing out.
		return Summary{}, State{}, errs.New(errs.CodeUsage,
			"this assignment requires you to accept its submission statement").
			WithHint("re-run with --accept-statement once you have read it in Moodle")
	}
	if !summary.HasPlugin(PluginFile) {
		// This tool submits files. Saying so is better than uploading into a
		// draft area the assignment will ignore.
		return Summary{}, State{}, errs.New(errs.CodeUnavailable,
			"this assignment does not accept file submissions").
			WithReason(errs.ReasonCapability).
			WithHint("submit it in the browser instead")
	}
	if err := checkFiles(req.Files, summary); err != nil {
		return Summary{}, State{}, err
	}
	return summary, state, nil
}

func (s *Submitter) find(ctx context.Context, assignmentID string) (Summary, error) {
	list, err := s.backend.List(ctx, nil)
	if err != nil {
		return Summary{}, err
	}
	for _, item := range list.Assignments {
		if item.ID == assignmentID {
			return item, nil
		}
	}
	return Summary{}, errs.New(errs.CodeNotFound,
		fmt.Sprintf("no assignment with id %s", assignmentID)).
		WithHint("list them with `moodle assignment list`")
}

// checkFiles validates locally before anything is sent.
func checkFiles(paths []string, summary Summary) error {
	if len(paths) == 0 {
		return errs.New(errs.CodeUsage, "no files given").
			WithHint("pass the files to submit, for example `moodle assignment submit 3 report.pdf`")
	}
	if summary.MaxFiles > 0 && len(paths) > summary.MaxFiles {
		return errs.New(errs.CodeValidation, fmt.Sprintf(
			"this assignment accepts at most %d file(s), and %d were given",
			summary.MaxFiles, len(paths)))
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return errs.Wrap(errs.CodeUsage, err, fmt.Sprintf("cannot read %s", path))
		}
		if info.IsDir() {
			return errs.New(errs.CodeUsage, fmt.Sprintf("%s is a directory", path))
		}
		if summary.MaxBytes > 0 && info.Size() > summary.MaxBytes {
			return errs.New(errs.CodeValidation, fmt.Sprintf(
				"%s is %d bytes, over this assignment's limit of %d",
				filepath.Base(path), info.Size(), summary.MaxBytes))
		}
	}
	return nil
}

// prepare puts the files in a draft area and assembles everything the save
// call has to carry.
func (s *Submitter) prepare(ctx context.Context, summary Summary, state State, req SubmitRequest) (Content, error) {
	var content Content
	err := s.guard.Do(ctx, "upload.php", func(ctx context.Context) error {
		var err error
		content.FileDraftID, err = s.writer.UploadDraft(ctx, req.Files)
		return err
	})
	if err != nil {
		return Content{}, err
	}

	if !summary.HasPlugin(PluginOnlineText) {
		return content, nil
	}
	// The assignment also takes online text, and one save writes every plugin
	// at once. What the student already typed is read back and sent unchanged,
	// so handing files in never quietly erases it.
	text := OnlineTextContent{Format: 1}
	if state.OnlineText != nil {
		text.Text = state.OnlineText.Text
		text.Format = state.OnlineText.Format
	}
	err = s.guard.Do(ctx, "core_files_get_unused_draft_itemid", func(ctx context.Context) error {
		var err error
		text.DraftID, err = s.writer.NewDraftArea(ctx)
		return err
	})
	if err != nil {
		return Content{}, err
	}
	content.OnlineText = &text
	return content, nil
}

// reconcile works out what actually happened when a write's outcome is unknown.
//
// A lost response does not mean the request failed: Moodle may have applied it
// and the reply may have been dropped on the way back. Reading the state is the
// only way to tell, and when it still cannot be told the caller is told that
// rather than given a guess.
func (s *Submitter) reconcile(ctx context.Context, req SubmitRequest, cause error, result *SubmitResult) error {
	failure := errs.From(cause)
	if failure.EffectiveOutcome() != errs.OutcomeAmbiguous {
		return failure
	}

	state, err := s.backend.Status(ctx, req.AssignmentID)
	if err != nil {
		// Still unknown. Say so plainly and do not retry.
		return failure.WithHint(
			"the request may already have been applied, and the state could not be read back; " +
				"check with `moodle assignment status` before trying again")
	}

	result.FinalStatus = state.Status
	switch state.Status {
	case StatusSubmitted:
		result.Outcome = OutcomeApplied
		result.HandedIn = true
		return nil
	case StatusDraft:
		result.Outcome = OutcomeApplied
		result.Steps = append(result.Steps, Step{
			Name: StepVerify, Status: StepDone,
			Detail: "the connection dropped, but the work was saved as a draft",
		})
		return nil
	default:
		// Confirmed not applied: safe to say so, and safe for the caller to
		// try again.
		return failure.WithHint("nothing was saved; it is safe to try again")
	}
}

func skipReason(summary Summary, req SubmitRequest) string {
	if req.DraftOnly {
		return "asked to save as a draft only"
	}
	if drafts, known := summary.NeedsHandIn(); known && !drafts {
		return "this assignment accepts work as soon as it is saved"
	}
	return ""
}
