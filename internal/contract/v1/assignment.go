package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/file"
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

// AssignmentDetail is the assignment.show payload.
type AssignmentDetail struct {
	Assignment
	// Description is Moodle's own text, passed through unchanged, with the
	// format it declared. Rewriting it here would be guessing at how the
	// caller wants to render it.
	Description       string `json:"description"`
	DescriptionFormat int    `json:"description_format"`
	// MaxGrade is null when the assignment is not graded, which is not the
	// same as being marked out of zero.
	MaxGrade  *float64 `json:"max_grade"`
	AllowFrom *string  `json:"allow_from"`
	// TimeLimitSeconds is null when there is no limit.
	TimeLimitSeconds *int `json:"time_limit_seconds"`
	// MaxAttempts is null when Moodle allows unlimited attempts.
	MaxAttempts    *int `json:"max_attempts"`
	TeamSubmission bool `json:"team_submission"`
	BlindMarking   bool `json:"blind_marking"`
	// Attachments are the files the teacher attached to the description.
	Attachments []File          `json:"attachments"`
	Submission  SubmissionState `json:"submission"`
}

// File is a file Moodle is holding.
type File struct {
	Name string `json:"name"`
	// Path is the folder within the file area, "/" at the top: two files in
	// one area can share a name if they sit in different folders.
	Path     string  `json:"path"`
	Size     int64   `json:"size"`
	MIMEType string  `json:"mime_type"`
	URL      string  `json:"url"`
	Modified *string `json:"modified_at"`
	// External marks a file held by another service, such as a linked cloud
	// drive. The site's own token does not necessarily open it, so a download
	// that works for the rest may fail for this one.
	External bool `json:"external"`
}

func newFiles(refs []file.Ref) []File {
	out := make([]File, 0, len(refs))
	for _, ref := range refs {
		out = append(out, File{
			Name:     ref.Name,
			Path:     ref.Path,
			Size:     ref.Size,
			MIMEType: ref.MIMEType,
			URL:      ref.URL,
			Modified: Timestamp(ref.ModifiedAt),
			External: ref.External,
		})
	}
	return out
}

// AssignmentShow converts one assignment and the caller's standing in it into
// its envelope. The two belong together: the definition is not much use
// without knowing where you stand in it.
func AssignmentShow(detail assignment.Detail, state assignment.State, siteName, accountName string) Envelope {
	payload := AssignmentDetail{
		Assignment:        newAssignment(detail.Summary),
		Description:       detail.Description,
		DescriptionFormat: detail.DescriptionFormat,
		AllowFrom:         Timestamp(detail.AllowFrom),
		TeamSubmission:    detail.TeamSubmission,
		BlindMarking:      detail.BlindMarking,
		Attachments:       newFiles(detail.Attachments),
		Submission:        newSubmissionState(detail.ID, state),
	}
	if detail.MaxGrade > 0 {
		grade := detail.MaxGrade
		payload.MaxGrade = &grade
	}
	if detail.TimeLimit > 0 {
		limit := detail.TimeLimit
		payload.TimeLimitSeconds = &limit
	}
	// Moodle uses -1 for "as many as you like", which must not be reported as
	// a limit of minus one attempt.
	if detail.MaxAttempts > 0 {
		attempts := detail.MaxAttempts
		payload.MaxAttempts = &attempts
	}
	return NewEnvelope("assignment.show", payload,
		MetaFrom(state.Provenance, siteName, accountName))
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
	// Files is what Moodle actually holds, so a caller can check that what was
	// received is what they meant to send.
	Files []File `json:"files"`
}

// AssignmentStatus converts a submission state into its envelope.
func AssignmentStatus(assignmentID string, state assignment.State, siteName, accountName string) Envelope {
	return NewEnvelope("assignment.status", newSubmissionState(assignmentID, state),
		MetaFrom(state.Provenance, siteName, accountName))
}

func newSubmissionState(assignmentID string, state assignment.State) SubmissionState {
	return SubmissionState{
		AssignmentID:  assignmentID,
		Status:        string(state.Status),
		HandedIn:      state.Status == assignment.StatusSubmitted,
		CanEdit:       state.CanEdit,
		CanSubmit:     state.CanSubmit,
		GradingStatus: optional(state.GradingStatus),
		ModifiedAt:    Timestamp(state.ModifiedAt),
		FileCount:     state.FileCount,
		Files:         newFiles(state.Files),
	}
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
