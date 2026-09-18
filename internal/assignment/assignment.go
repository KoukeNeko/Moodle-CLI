// Package assignment covers reading assignments and handing work in.
//
// Submitting is the reason this project exists. Moodle's own model has two
// steps that look like one: saving content, and handing it in for grading. A
// client that calls only the first and reports success leaves the student
// believing work was submitted when it is still a draft. Every path here ends
// by reading the state back from Moodle and reporting what Moodle says.
package assignment

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Status is where a submission stands.
type Status string

const (
	// StatusNew means nothing has been saved yet.
	StatusNew Status = "new"
	// StatusDraft means content is saved but not handed in. Work in this
	// state is not being marked.
	StatusDraft Status = "draft"
	// StatusSubmitted means the work has been handed in for grading.
	StatusSubmitted Status = "submitted"
	// StatusUnknown means Moodle reported something this build does not
	// recognise. It is deliberately not folded into one of the others.
	StatusUnknown Status = "unknown"
)

// Summary is one assignment.
type Summary struct {
	ID       string
	CourseID string
	CMID     string
	Name     string
	DueDate  *time.Time
	CutOff   *time.Time
	// SubmissionDrafts reports whether Moodle keeps a separate draft state.
	// When it does, saving content is not handing it in, and a second call is
	// required.
	//
	// Nil means the route that answered could not see the setting — reading a
	// page cannot. It is kept distinct from false because the difference is
	// whether saved work is already submitted, and guessing either way would
	// tell a student something untrue about coursework.
	SubmissionDrafts *bool
	// RequiresStatement reports whether the student must accept a submission
	// statement. Agreeing on the student's behalf would be signing something
	// for them, so the caller has to pass their consent explicitly.
	RequiresStatement bool
	MaxFiles          int
	MaxBytes          int64
	// Plugins are the submission plugins the assignment has switched on, such
	// as "file" and "onlinetext". Saving a submission has to carry data for
	// every one of them: Moodle saves them all in one call, and a plugin left
	// out is not left alone but saved empty.
	Plugins []string
}

// NeedsHandIn reports whether saving content leaves the work as a draft, and
// whether that is known at all.
func (s Summary) NeedsHandIn() (needs, known bool) {
	if s.SubmissionDrafts == nil {
		return false, false
	}
	return *s.SubmissionDrafts, true
}

// HasPlugin reports whether a submission plugin is switched on.
func (s Summary) HasPlugin(name string) bool {
	for _, plugin := range s.Plugins {
		if plugin == name {
			return true
		}
	}
	return false
}

// Plugin names this tool knows how to submit for.
const (
	PluginFile       = "file"
	PluginOnlineText = "onlinetext"
)

// Detail is everything an assignment says about itself.
//
// It is separate from Summary because a listing does not want the description
// of every assignment, and a single assignment's view is useless without it.
type Detail struct {
	Summary
	// Description is Moodle's own text, unchanged. It is usually HTML, which
	// DescriptionFormat identifies; rewriting it here would be guessing at how
	// the caller wants to render it.
	Description       string
	DescriptionFormat int
	// MaxGrade is the number the work is marked out of. Zero means the
	// assignment is not graded.
	MaxGrade float64
	// AllowFrom is when submissions open, nil when they are open already.
	AllowFrom *time.Time
	// TimeLimit is in seconds; zero means no limit.
	TimeLimit int
	// MaxAttempts is -1 when Moodle allows unlimited attempts.
	MaxAttempts    int
	TeamSubmission bool
	BlindMarking   bool
	// Attachments are the files the teacher attached to the description.
	Attachments []file.Ref
}

// State is the caller's current submission for an assignment.
type State struct {
	Status Status
	// CanEdit and CanSubmit are Moodle's own verdicts. They account for cut-off
	// dates, group membership and permissions, which a client cannot infer.
	CanEdit   bool
	CanSubmit bool
	// GradingStatus is Moodle's word for whether it has been marked.
	GradingStatus string
	ModifiedAt    *time.Time
	FileCount     int
	// OnlineText is what the student has already typed, when the assignment
	// takes online text. It is read so that submitting files can put it back
	// unchanged instead of blanking it.
	OnlineText *OnlineText
	// Files are what has actually been handed in, so a caller can check that
	// what Moodle holds is what they meant to send.
	Files []file.Ref
	// Provenance tells the caller which backend answered.
	Provenance site.Provenance
}

// OnlineText is the content of an online text submission.
type OnlineText struct {
	Text string
	// Format is Moodle's text format id, passed through rather than
	// interpreted: rewriting it would change how the text renders.
	Format int
}

// Content is everything one save_submission call carries.
//
// Moodle saves every enabled plugin in a single call, so this has to describe
// all of them at once. Sending only the part that changed would save the rest
// as empty.
type Content struct {
	// FileDraftID is the draft area holding the uploaded files.
	FileDraftID string
	// OnlineText is set only when the assignment enables that plugin.
	OnlineText *OnlineTextContent
}

// OnlineTextContent is the online text half of a submission.
type OnlineTextContent struct {
	Text   string
	Format int
	// DraftID must be a draft area of its own. Reusing the file area's id
	// makes Moodle move the uploaded files into the text plugin's storage,
	// after which the file plugin finds nothing and the save is rejected.
	DraftID string
}

// ListResult is a set of assignments plus where it came from.
type ListResult struct {
	Assignments []Summary
	Provenance  site.Provenance
}

// Backend reads assignments and their state.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	List(ctx context.Context, courseIDs []string) (ListResult, error)
	Show(ctx context.Context, assignmentID string) (Detail, error)
	Status(ctx context.Context, assignmentID string) (State, error)
}

// Writer performs the steps of handing work in.
//
// The steps are separate because their failure modes differ: an upload can be
// repeated harmlessly, saving cannot, and handing in is the step the student
// actually cares about.
type Writer interface {
	// UploadDraft puts local files in the user's draft area and returns the
	// item id that identifies it.
	UploadDraft(ctx context.Context, paths []string) (itemID string, err error)
	// NewDraftArea allocates an empty draft area. Allocating one is itself a
	// write, which is why it is a step of its own rather than a detail hidden
	// inside saving.
	NewDraftArea(ctx context.Context) (itemID string, err error)
	// SaveSubmission writes the submission's content.
	SaveSubmission(ctx context.Context, assignmentID string, content Content) error
	// SubmitForGrading hands the work in. acceptStatement records that the
	// student agreed to the submission statement.
	SubmitForGrading(ctx context.Context, assignmentID string, acceptStatement bool) error
}
