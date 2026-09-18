package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
)

// The assignment payloads carry one distinction the rest of the contract does
// not: saved work and handed-in work are different states, and a consumer must
// be able to tell them apart without reading prose. Everything here reports
// what Moodle said after the fact, never what the client believes it did.

// Assignment is one assignment on the wire.
type Assignment struct {
	ID       string  `json:"id"`
	CourseID string  `json:"course_id"`
	CMID     string  `json:"cmid"`
	Name     string  `json:"name"`
	DueDate  *string `json:"due_date"`
	CutOff   *string `json:"cut_off_date"`
	// NeedsHandIn says whether saving content leaves the work as a draft. A
	// consumer that ignores it will report unsubmitted work as submitted.
	NeedsHandIn       bool `json:"needs_hand_in"`
	RequiresStatement bool `json:"requires_statement"`
	// SubmissionPlugins are the kinds of content the assignment accepts. A
	// caller checks for "file" here rather than assuming every assignment
	// takes one.
	SubmissionPlugins []string `json:"submission_plugins"`
	MaxFiles          *int     `json:"max_files"`
	MaxBytes          *int64   `json:"max_bytes"`
}

// AssignmentList converts a listing into its envelope.
func AssignmentList(result assignment.ListResult, siteName, accountName string) Envelope {
	items := make([]Assignment, 0, len(result.Assignments))
	for _, item := range result.Assignments {
		items = append(items, newAssignment(item))
	}
	return NewEnvelope("assignment.list", items,
		MetaFrom(result.Provenance, siteName, accountName))
}

func newAssignment(item assignment.Summary) Assignment {
	out := Assignment{
		ID:                item.ID,
		CourseID:          item.CourseID,
		CMID:              item.CMID,
		Name:              item.Name,
		DueDate:           Timestamp(item.DueDate),
		CutOff:            Timestamp(item.CutOff),
		NeedsHandIn:       item.NeedsHandIn(),
		RequiresStatement: item.RequiresStatement,
		SubmissionPlugins: item.Plugins,
	}
	if out.SubmissionPlugins == nil {
		out.SubmissionPlugins = []string{}
	}
	// Zero means "no limit" in Moodle, which is not the same as a limit of
	// zero files; it is reported as null rather than 0.
	if item.MaxFiles > 0 {
		value := item.MaxFiles
		out.MaxFiles = &value
	}
	if item.MaxBytes > 0 {
		value := item.MaxBytes
		out.MaxBytes = &value
	}
	return out
}

// SubmissionState is the assignment.status payload.
type SubmissionState struct {
	AssignmentID string `json:"assignment_id"`
	// Status is one of new, draft, submitted or unknown. An unrecognised value
	// from the site stays "unknown" rather than being folded into one of the
	// others.
	Status string `json:"status"`
	// HandedIn is the answer to the only question most callers have. It is
	// true only when Moodle reports the work as submitted.
	HandedIn      bool    `json:"handed_in"`
	CanEdit       bool    `json:"can_edit"`
	CanSubmit     bool    `json:"can_submit"`
	GradingStatus *string `json:"grading_status"`
	ModifiedAt    *string `json:"modified_at"`
	FileCount     int     `json:"file_count"`
}

// AssignmentStatus converts a submission state into its envelope.
func AssignmentStatus(assignmentID string, state assignment.State, siteName, accountName string) Envelope {
	payload := SubmissionState{
		AssignmentID:  assignmentID,
		Status:        string(state.Status),
		HandedIn:      state.Status == assignment.StatusSubmitted,
		CanEdit:       state.CanEdit,
		CanSubmit:     state.CanSubmit,
		GradingStatus: optional(state.GradingStatus),
		ModifiedAt:    Timestamp(state.ModifiedAt),
		FileCount:     state.FileCount,
	}
	return NewEnvelope("assignment.status", payload,
		MetaFrom(state.Provenance, siteName, accountName))
}

// SubmitStep is one stage of handing work in.
type SubmitStep struct {
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Detail *string `json:"detail"`
}

// SubmitReport is the assignment.submit payload.
type SubmitReport struct {
	Assignment Assignment `json:"assignment"`
	// Outcome is applied, not_applied or planned. It says what happened to the
	// request; final_status says where the work now stands.
	Outcome string `json:"outcome"`
	// FinalStatus is read back from Moodle after the writes, so a caller does
	// not have to trust that the calls did what they appeared to do.
	FinalStatus string `json:"final_status"`
	// HandedIn is false for a draft, whatever the individual steps reported.
	HandedIn bool `json:"handed_in"`
	// DryRun says nothing was sent. It is reported explicitly so a log cannot
	// be mistaken for a record of a real submission.
	DryRun bool         `json:"dry_run"`
	Steps  []SubmitStep `json:"steps"`
}

// AssignmentSubmit converts a submission result into its envelope.
func AssignmentSubmit(result assignment.SubmitResult, siteName, accountName string) Envelope {
	payload := SubmitReport{
		Assignment:  newAssignment(result.Assignment),
		Outcome:     string(result.Outcome),
		FinalStatus: string(result.FinalStatus),
		HandedIn:    result.HandedIn,
		DryRun:      result.Outcome == assignment.OutcomePlanned,
		Steps:       []SubmitStep{},
	}
	for _, step := range result.Steps {
		payload.Steps = append(payload.Steps, SubmitStep{
			Name:   string(step.Name),
			Status: string(step.Status),
			Detail: optional(step.Detail),
		})
	}
	return NewEnvelope("assignment.submit", payload,
		MetaFrom(result.Provenance, siteName, accountName))
}
