package moodle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Assignment web service functions.
const (
	FunctionAssignments      = "mod_assign_get_assignments"
	FunctionSubmissionStatus = "mod_assign_get_submission_status"
	FunctionSaveSubmission   = "mod_assign_save_submission"
	FunctionSubmitForGrading = "mod_assign_submit_for_grading"
	FunctionUnusedDraftArea  = "core_files_get_unused_draft_itemid"
)

// PathUpload takes files into the caller's draft area.
const PathUpload = "/webservice/upload.php"

// assignmentsDTO is Moodle's reply to mod_assign_get_assignments. The field
// names were taken from real 4.5, 5.1 and 5.2 responses rather than guessed.
type assignmentsDTO struct {
	Courses []struct {
		ID          int64 `json:"id"`
		Assignments []struct {
			ID                         int64   `json:"id"`
			CMID                       int64   `json:"cmid"`
			Course                     int64   `json:"course"`
			Name                       string  `json:"name"`
			DueDate                    int64   `json:"duedate"`
			CutOffDate                 int64   `json:"cutoffdate"`
			SubmissionDrafts           int     `json:"submissiondrafts"`
			RequireSubmissionStatement int     `json:"requiresubmissionstatement"`
			AllowFrom                  int64   `json:"allowsubmissionsfromdate"`
			Intro                      string  `json:"intro"`
			IntroFormat                int     `json:"introformat"`
			Grade                      float64 `json:"grade"`
			TimeLimit                  int     `json:"timelimit"`
			MaxAttempts                int     `json:"maxattempts"`
			TeamSubmission             int     `json:"teamsubmission"`
			BlindMarking               int     `json:"blindmarking"`
			// IntroAttachments are the files attached to the description.
			IntroAttachments []fileDTO `json:"introattachments"`
			Configs          []struct {
				Plugin  string `json:"plugin"`
				Subtype string `json:"subtype"`
				Name    string `json:"name"`
				Value   string `json:"value"`
			} `json:"configs"`
		} `json:"assignments"`
	} `json:"courses"`
	// Warnings name what Moodle left out instead of refusing the whole call.
	// A course this account cannot reach, and an activity it cannot see inside
	// one it can, both come back here on an otherwise successful reply — so a
	// caller that ignores them reports a short list as though it were the
	// whole one.
	Warnings []struct {
		Item        string `json:"item"`
		ItemID      int64  `json:"itemid"`
		WarningCode string `json:"warningcode"`
		Message     string `json:"message"`
	} `json:"warnings"`
}

// submissionStatusDTO is Moodle's reply to mod_assign_get_submission_status.
type submissionStatusDTO struct {
	// GradingSummary comes back only for someone who can view grades, which
	// is how an account on a course it does not take — a TA — finds out it is
	// staff here rather than a participant.
	GradingSummary *struct {
		ParticipantCount int `json:"participantcount"`
	} `json:"gradingsummary"`
	// LastAttempt is a pointer because the key is absent, not false, when
	// Moodle has no submission summary to show this user at all: a teacher or
	// TA on a course they do not take. Reading the zero value would turn that
	// into "submissionsenabled: false" — "this assignment is closed" — which
	// is a different statement and a wrong one.
	LastAttempt *lastAttemptDTO `json:"lastattempt"`
	// PreviousAttempts is what the account handed in before a grader reopened
	// the assignment. Moodle sends it to the student themselves, not only to a
	// grader — measured — so an earlier attempt is knowable without the
	// grading call this tool cannot make.
	PreviousAttempts []struct {
		AttemptNumber int            `json:"attemptnumber"`
		Submission    *submissionDTO `json:"submission"`
	} `json:"previousattempts"`
}

type lastAttemptDTO struct {
	SubmissionsEnabled bool   `json:"submissionsenabled"`
	CanEdit            bool   `json:"canedit"`
	CanSubmit          bool   `json:"cansubmit"`
	Locked             bool   `json:"locked"`
	Graded             bool   `json:"graded"`
	GradingStatus      string `json:"gradingstatus"`
	// SubmissionGroup is the group this submission belongs to, 0 when the
	// assignment is not a group one. On a group assignment "handed in" is
	// about the group, and a reader has no way to know that from the status
	// alone: the assignment's own settings live in a different call.
	SubmissionGroup int64 `json:"submissiongroup"`
	// MembersWhoNeedToSubmit is populated only when every member has to
	// submit. An empty list is then "everyone has", which is a different
	// statement from "this setting is off" — Moodle sends an empty list for
	// both, so it is the count and not the emptiness that is reported.
	MembersWhoNeedToSubmit []int64 `json:"submissiongroupmemberswhoneedtosubmit"`
	// ExtensionDueDate is this account's own extension, granted per person.
	// It is not on the assignment: two students reading the same assignment
	// can have different answers, and Moodle folds it into whether a
	// submission is still open rather than into the dates it publishes.
	ExtensionDueDate int64 `json:"extensionduedate"`
	// TimeLimit is how long a started attempt may run, in seconds; zero when
	// the assignment sets none.
	TimeLimit int64 `json:"timelimit"`
	// Submission is absent entirely until a submission record exists, which
	// is different from one that exists and is empty: a record with status
	// "new" is a real state Moodle reports.
	Submission *submissionDTO `json:"submission"`
}

// submissionDTO is one attempt. The current one and every earlier one have
// the same shape, so they share this rather than drifting apart.
type submissionDTO struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	TimeModified int64  `json:"timemodified"`
	// TimeStarted is when a timed attempt's clock began, null when it has not
	// or the assignment has no time limit.
	TimeStarted *int64 `json:"timestarted"`
	Plugins     []struct {
		Type      string `json:"type"`
		FileAreas []struct {
			Area  string    `json:"area"`
			Files []fileDTO `json:"files"`
		} `json:"fileareas"`
		// EditorFields carry what the student has already typed.
		EditorFields []struct {
			Name   string `json:"name"`
			Text   string `json:"text"`
			Format int    `json:"format"`
		} `json:"editorfields"`
	} `json:"plugins"`
}

// fileDTO is how Moodle describes a file it is holding. The same shape comes
// back for assignment attachments and for submitted files.
type fileDTO struct {
	FileName     string `json:"filename"`
	FilePath     string `json:"filepath"`
	FileSize     int64  `json:"filesize"`
	FileURL      string `json:"fileurl"`
	TimeModified int64  `json:"timemodified"`
	MimeType     string `json:"mimetype"`
	// IsExternalFile marks a file held by another service, such as a linked
	// cloud drive; the site's own token does not necessarily open it.
	IsExternalFile bool `json:"isexternalfile"`
}

func fileRefs(items []fileDTO) []file.Ref {
	refs := make([]file.Ref, 0, len(items))
	for _, item := range items {
		refs = append(refs, file.Ref{
			Name:       item.FileName,
			Path:       item.FilePath,
			Size:       item.FileSize,
			URL:        item.FileURL,
			MIMEType:   item.MimeType,
			ModifiedAt: unixTime(item.TimeModified),
			External:   item.IsExternalFile,
		})
	}
	return refs
}

// AssignmentBackend reads assignments over the web service API.
type AssignmentBackend struct {
	client *Client
	token  string
}

// NewAssignmentBackend builds the web service backend for assignments.
func NewAssignmentBackend(client *Client, token string) *AssignmentBackend {
	return &AssignmentBackend{client: client, token: token}
}

func (b *AssignmentBackend) Name() site.BackendKind { return site.BackendWS }

func (b *AssignmentBackend) Requirement() site.Requirement {
	return site.Requirement{
		AnyFunction: []string{FunctionAssignments},
		Credential:  site.CredentialWSToken,
	}
}

func (b *AssignmentBackend) List(ctx context.Context, courseIDs []string) (assignment.ListResult, error) {
	params := Params{}
	if len(courseIDs) > 0 {
		ids := make([]any, 0, len(courseIDs))
		for _, raw := range courseIDs {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return assignment.ListResult{}, errs.New(errs.CodeUsage,
					fmt.Sprintf("course id %q is not a number", raw))
			}
			ids = append(ids, id)
		}
		params["courseids"] = ids
	}

	var dto assignmentsDTO
	if err := b.client.Call(ctx, b.token, FunctionAssignments, params, &dto); err != nil {
		return assignment.ListResult{}, err
	}

	var out []assignment.Summary
	for _, detail := range b.details(dto) {
		out = append(out, detail.Summary)
	}

	result := assignment.ListResult{
		Assignments: out,
		Provenance:  site.NewProvenance(site.BackendWS),
	}
	if len(dto.Warnings) > 0 {
		// The reply was filtered: Moodle answers a course it will not list with
		// a warning rather than an error. Returning the short list unchanged
		// would say "this course has no assignments" about one that was never
		// read.
		if len(out) == 0 {
			if unenrolled, unreachable := leftOutCourses(dto); len(unreachable) > 0 {
				return assignment.ListResult{}, errs.New(errs.CodePermissionDenied,
					fmt.Sprintf("course %s is not one this account can read",
						strings.Join(unreachable, ", "))).
					WithHint("the site left it out of the reply")
			} else if len(unenrolled) > 0 {
				// Not the same statement: an account can hold no enrolment on
				// a course and still read it — a manager does — but this call
				// lists a course only for an account enrolled on it.
				return assignment.ListResult{}, errs.New(errs.CodePermissionDenied,
					fmt.Sprintf("this account is not enrolled on course %s",
						strings.Join(unenrolled, ", "))).
					WithHint("the site lists a course here only for an account enrolled on it")
			} else if withheld := withheldActivities(dto); len(withheld) > 0 {
				// The course itself was read. Every assignment in it was left
				// out one at a time, which is what an availability restriction
				// or an activity-level override looks like from here.
				return assignment.ListResult{}, errs.New(errs.CodePermissionDenied,
					fmt.Sprintf("this account cannot read activity %s",
						strings.Join(withheld, ", "))).
					WithHint("the site left every assignment out of the reply")
			}
		}
		result.Provenance.Partial = true
	}
	return result, nil
}

// leftOutCourses splits the courses a reply left out by why it left them out.
//
// Moodle's warningcode carries the distinction: 2 for a course the account is
// not enrolled on, 1 for one it cannot reach at all. Reporting both as one
// would put a wrong reason in front of whoever reads it.
func leftOutCourses(dto assignmentsDTO) (unenrolled, unreachable []string) {
	for _, warning := range dto.Warnings {
		if warning.Item != "course" {
			continue
		}
		id := strconv.FormatInt(warning.ItemID, 10)
		if warning.WarningCode == "2" {
			unenrolled = append(unenrolled, id)
			continue
		}
		unreachable = append(unreachable, id)
	}
	return unenrolled, unreachable
}

// withheldActivities names the activities a reply left out one at a time.
//
// Moodle checks mod/assign:view per activity and reports each refusal with
// item "module" and the course module's id. That id is not the assignment id
// this tool prints elsewhere: an activity it will not open has no assignment
// record to name it by, and the course module id is what the site gave.
func withheldActivities(dto assignmentsDTO) []string {
	var out []string
	for _, warning := range dto.Warnings {
		if warning.Item != "module" {
			continue
		}
		out = append(out, strconv.FormatInt(warning.ItemID, 10))
	}
	return out
}

// Show returns everything one assignment says about itself.
//
// Moodle has no "one assignment" call: the same listing is fetched and the one
// asked for is picked out of it.
func (b *AssignmentBackend) Show(ctx context.Context, assignmentID string) (assignment.Detail, error) {
	var dto assignmentsDTO
	if err := b.client.Call(ctx, b.token, FunctionAssignments, Params{}, &dto); err != nil {
		return assignment.Detail{}, err
	}
	for _, detail := range b.details(dto) {
		if detail.ID == assignmentID {
			return detail, nil
		}
	}
	// An assignment the site withheld is absent from this listing exactly as a
	// deleted one is. The warning names a course module, and the question asked
	// about an assignment, so which of the two this is cannot be settled from
	// here — but sending someone to `assignment list`, where it is equally
	// absent, walks them in a circle. Say that the reply was short instead.
	if withheld := withheldActivities(dto); len(withheld) > 0 {
		return assignment.Detail{}, errs.New(errs.CodeNotFound,
			fmt.Sprintf("no assignment with id %s in what this account can read", assignmentID)).
			WithHint(fmt.Sprintf("the site also withheld activity %s from the same reply, "+
				"so this may be one of them rather than one that does not exist",
				strings.Join(withheld, ", ")))
	}
	return assignment.Detail{}, errs.New(errs.CodeNotFound,
		fmt.Sprintf("no assignment with id %s", assignmentID)).
		WithHint("list them with `moodle assignment list`")
}

// details maps Moodle's reply onto this project's shape.
func (b *AssignmentBackend) details(dto assignmentsDTO) []assignment.Detail {
	var out []assignment.Detail
	for _, course := range dto.Courses {
		for _, item := range course.Assignments {
			summary := assignment.Summary{
				ID:       strconv.FormatInt(item.ID, 10),
				CourseID: strconv.FormatInt(course.ID, 10),
				CMID:     strconv.FormatInt(item.CMID, 10),
				Name:     item.Name,
				DueDate:  unixTime(item.DueDate),
				CutOff:   unixTime(item.CutOffDate),
				// These two decide whether saving content is enough, or
				// whether a second call is needed to hand the work in. This
				// route always knows; the page-reading one leaves it nil.
				SubmissionDrafts:  boolPtr(item.SubmissionDrafts == 1),
				RequiresStatement: item.RequireSubmissionStatement == 1,
			}
			for _, config := range item.Configs {
				if config.Subtype != "assignsubmission" {
					continue
				}
				// The enabled flags say which kinds of content the assignment
				// accepts, and a save has to carry data for every one of them.
				if config.Name == "enabled" {
					if config.Value == "1" {
						summary.Plugins = append(summary.Plugins, config.Plugin)
					}
					continue
				}
				if config.Plugin != "file" {
					continue
				}
				switch config.Name {
				case "maxfilesubmissions":
					summary.MaxFiles, _ = strconv.Atoi(config.Value)
				case "maxsubmissionsizebytes":
					summary.MaxBytes, _ = strconv.ParseInt(config.Value, 10, 64)
				}
			}
			out = append(out, assignment.Detail{
				Summary:           summary,
				Description:       item.Intro,
				DescriptionFormat: item.IntroFormat,
				MaxGrade:          item.Grade,
				AllowFrom:         unixTime(item.AllowFrom),
				TimeLimit:         item.TimeLimit,
				MaxAttempts:       item.MaxAttempts,
				TeamSubmission:    item.TeamSubmission == 1,
				BlindMarking:      item.BlindMarking == 1,
				Attachments:       fileRefs(item.IntroAttachments),
			})
		}
	}
	return out
}

// timerEnd is when a started time limit runs out, nil when none is running.
//
// Moodle's own timer takes the earlier of the limit and the assignment's
// closing date (mod/assign timelimit_panel), so a limit that would outlast the
// cut-off does not extend anything. This route has the limit but not those
// dates, so it reports only the limit's own end and leaves the comparison to
// whoever holds both.
//
// The clock starts server-side when the attempt is started, so timestarted
// being absent is the signal that it has not: a limit alone means "if you
// start", not "you have started".
func timerEnd(last *lastAttemptDTO) *time.Time {
	if last.TimeLimit <= 0 || last.Submission == nil {
		return nil
	}
	started := last.Submission.TimeStarted
	if started == nil || *started <= 0 {
		return nil
	}
	return unixTime(*started + last.TimeLimit)
}

// earlierAttempts maps what the account handed in before it was reopened.
//
// Moodle counts attempts from zero and sends them oldest first; both are kept
// as they arrive rather than renumbered, so what is printed matches what the
// site and its own pages say.
func earlierAttempts(dto submissionStatusDTO) []assignment.Attempt {
	var out []assignment.Attempt
	for _, previous := range dto.PreviousAttempts {
		attempt := assignment.Attempt{Number: previous.AttemptNumber}
		if previous.Submission != nil {
			attempt.Status = translateStatus(previous.Submission.Status)
			attempt.SavedAt = unixTime(previous.Submission.TimeModified)
			for _, plugin := range previous.Submission.Plugins {
				for _, area := range plugin.FileAreas {
					if area.Area == "submission_files" {
						attempt.FileCount += len(area.Files)
					}
				}
			}
		}
		out = append(out, attempt)
	}
	return out
}

func (b *AssignmentBackend) Status(ctx context.Context, assignmentID string) (assignment.State, error) {
	id, err := strconv.ParseInt(assignmentID, 10, 64)
	if err != nil {
		return assignment.State{}, errs.New(errs.CodeUsage,
			fmt.Sprintf("assignment id %q is not a number", assignmentID))
	}

	var dto submissionStatusDTO
	if err := b.client.Call(ctx, b.token, FunctionSubmissionStatus,
		Params{"assignid": id}, &dto); err != nil {
		return assignment.State{}, err
	}

	last := dto.LastAttempt
	if last == nil {
		// Mod/assign sends lastattempt only to an account holding
		// mod/assign:viewownsubmissionsummary. That is not a fact about being a
		// participant: a site can prevent the capability from the student role
		// and leave a student with no summary of their own — measured, and the
		// reply is then indistinguishable from a TA's.
		//
		// Gradingsummary is what separates the two: it comes back only to an
		// account that can view grades. Its absence settles nothing the other
		// way, though — a separate-groups activity withholds it even from staff
		// who hold the capability but no group — so without it we say only what
		// the site did and leave who this account is out of it.
		if dto.GradingSummary != nil {
			return assignment.State{}, errs.New(errs.CodeUnavailable,
				"you are staff on this assignment, not a participant on the course").
				WithHint("the site sends a submission summary only to an account " +
					"that can view its own")
		}
		return assignment.State{}, errs.New(errs.CodeUnavailable,
			"the site reports no submission summary for this account").
			WithHint("it sends one only to an account holding " +
				"mod/assign:viewownsubmissionsummary")
	}
	state := assignment.State{
		Status:               assignment.StatusNew,
		CanEdit:              last.CanEdit,
		CanSubmit:            last.CanSubmit,
		ExtensionDue:         unixTime(last.ExtensionDueDate),
		GroupSubmission:      last.SubmissionGroup != 0,
		MembersStillToSubmit: len(last.MembersWhoNeedToSubmit),
		GradingStatus:        last.GradingStatus,
		Earlier:              earlierAttempts(dto),
		TimerEndsAt:          timerEnd(last),
		OnlineSubmission:     boolPtr(last.SubmissionsEnabled),
		Provenance:           site.NewProvenance(site.BackendWS),
	}
	// submissionsenabled=false used to end the call here. It is false for an
	// offline assignment — every submission plugin switched off, which Moodle
	// caches as nosubmissions and documents as the way to mark work done
	// elsewhere. Refusing left a student unable to read a status for an
	// assignment that already had a grade: measured, 76/100 visible in the
	// gradebook while this call answered "no route to this data".
	if last.Submission == nil {
		// No submission record yet: nothing has been saved.
		return state, nil
	}
	state.Status = translateStatus(last.Submission.Status)
	state.ModifiedAt = unixTime(last.Submission.TimeModified)
	for _, plugin := range last.Submission.Plugins {
		for _, area := range plugin.FileAreas {
			if area.Area == "submission_files" {
				state.FileCount += len(area.Files)
				state.Files = append(state.Files, fileRefs(area.Files)...)
			}
		}
		if plugin.Type != assignment.PluginOnlineText {
			continue
		}
		for _, field := range plugin.EditorFields {
			if field.Name != "onlinetext" {
				continue
			}
			state.OnlineText = &assignment.OnlineText{
				Text: field.Text, Format: field.Format,
			}
		}
	}
	return state, nil
}

// translateStatus maps Moodle's word onto ours.
//
// An unrecognised value becomes StatusUnknown rather than being folded into
// one of the others: reporting "draft" for something Moodle called something
// else would be a confident wrong answer about whether work was handed in.
func translateStatus(raw string) assignment.Status {
	switch raw {
	case "new":
		return assignment.StatusNew
	case "draft":
		return assignment.StatusDraft
	case "reopened":
		return assignment.StatusReopened
	case "submitted":
		return assignment.StatusSubmitted
	default:
		return assignment.StatusUnknown
	}
}

// AssignmentWriter performs the steps of handing work in.
type AssignmentWriter struct {
	client *Client
	token  string
}

// NewAssignmentWriter builds the writer.
func NewAssignmentWriter(client *Client, token string) *AssignmentWriter {
	return &AssignmentWriter{client: client, token: token}
}

// uploadedFile is one entry of the upload endpoint's reply.
type uploadedFile struct {
	ItemID   json.Number `json:"itemid"`
	FileName string      `json:"filename"`
	// The endpoint reports failure in the same array rather than as an
	// exception.
	Error     string `json:"error"`
	ErrorCode string `json:"errorcode"`
}

func (w *AssignmentWriter) UploadDraft(ctx context.Context, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", errs.New(errs.CodeUsage, "no files to upload")
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for index, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return "", errs.Wrap(errs.CodeUsage, err, fmt.Sprintf("cannot read %s", path))
		}
		part, err := form.CreateFormFile(fmt.Sprintf("file_%d", index), filepath.Base(path))
		if err != nil {
			file.Close()
			return "", errs.Wrap(errs.CodeInternal, err, "cannot build the upload")
		}
		if _, err := io.Copy(part, file); err != nil {
			file.Close()
			return "", errs.Wrap(errs.CodeInternal, err, fmt.Sprintf("cannot read %s", path))
		}
		file.Close()
	}
	if err := form.Close(); err != nil {
		return "", errs.Wrap(errs.CodeInternal, err, "cannot finish the upload")
	}

	// The token goes in the query here, which is why redaction covers URL
	// parameters and not only headers.
	endpoint := w.client.Site().Endpoint(PathUpload) + "?token=" + w.token
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternal, err, "cannot build the upload request")
	}
	request.Header.Set("Content-Type", form.FormDataContentType())

	raw, err := w.client.send(request, "upload.php")
	if err != nil {
		return "", err
	}

	var files []uploadedFile
	if err := json.Unmarshal(raw, &files); err != nil {
		// A failure here can also arrive as a single object rather than an
		// array, which is the exception shape.
		return "", decode(raw, "upload.php", nil)
	}
	if len(files) == 0 {
		return "", errs.New(errs.CodeUpstream, "the upload returned no files").
			WithReason(errs.ReasonProtocolDrift)
	}
	if files[0].Error != "" {
		return "", errs.New(errs.CodeUpstream, files[0].Error)
	}
	itemID := files[0].ItemID.String()
	if itemID == "" || itemID == "0" {
		return "", errs.New(errs.CodeUpstream, "the upload returned no draft area").
			WithReason(errs.ReasonProtocolDrift)
	}
	return itemID, nil
}

// NewDraftArea allocates an empty draft file area.
func (w *AssignmentWriter) NewDraftArea(ctx context.Context) (string, error) {
	var reply struct {
		ItemID json.Number `json:"itemid"`
	}
	if err := w.client.Call(ctx, w.token, FunctionUnusedDraftArea, Params{}, &reply); err != nil {
		return "", err
	}
	if reply.ItemID.String() == "" {
		return "", errs.New(errs.CodeUpstream, "the site returned no draft area").
			WithReason(errs.ReasonProtocolDrift)
	}
	return reply.ItemID.String(), nil
}

func (w *AssignmentWriter) SaveSubmission(ctx context.Context, assignmentID string, content assignment.Content) error {
	assignID, err := strconv.ParseInt(assignmentID, 10, 64)
	if err != nil {
		return errs.New(errs.CodeUsage, fmt.Sprintf("assignment id %q is not a number", assignmentID))
	}
	draftID, err := strconv.ParseInt(content.FileDraftID, 10, 64)
	if err != nil {
		return errs.New(errs.CodeInternal,
			fmt.Sprintf("draft item id %q is not a number", content.FileDraftID))
	}

	plugindata := map[string]any{"files_filemanager": draftID}
	if text := content.OnlineText; text != nil {
		// Moodle saves every enabled plugin in this one call. Leaving the
		// online text out does not leave it alone: it saves it as empty, which
		// would delete whatever the student typed in the browser.
		textDraft, err := strconv.ParseInt(text.DraftID, 10, 64)
		if err != nil {
			return errs.New(errs.CodeInternal,
				fmt.Sprintf("draft item id %q is not a number", text.DraftID))
		}
		plugindata["onlinetext_editor"] = map[string]any{
			"text":   text.Text,
			"format": text.Format,
			"itemid": textDraft,
		}
	}

	// Moodle answers with an array of warnings; an empty array means success.
	var warnings []writeWarning
	if err := w.client.Call(ctx, w.token, FunctionSaveSubmission, Params{
		"assignmentid": assignID,
		"plugindata":   plugindata,
	}, &warnings); err != nil {
		return err
	}
	if len(warnings) > 0 {
		// save_submission puts its reason in "item" and a fixed "Could not
		// save submission." in "message" — generate_warning() takes the detail
		// last and Moodle passes the notice there. Reporting "message" showed
		// the reader the one part that says nothing.
		return errs.New(errs.CodeConflict,
			"the site would not save the submission: "+reasons(warnings)).
			WithHint("check the current state with `moodle assignment status`")
	}
	return nil
}

// writeWarning is the shape both write calls answer with.
type writeWarning struct {
	Item        string `json:"item"`
	WarningCode string `json:"warningcode"`
	Message     string `json:"message"`
}

// reasons joins what the site said, rather than showing the first and
// discarding the rest: a submission can be refused by several plugins at once,
// and hiding all but one of them hides part of the work needed to fix it.
func reasons(warnings []writeWarning) string {
	seen := map[string]bool{}
	var out []string
	for _, warning := range warnings {
		text := strings.TrimSpace(warning.Item)
		if text == "" {
			text = strings.TrimSpace(warning.Message)
		}
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	if len(out) == 0 {
		return "it gave no reason"
	}
	return strings.Join(out, "; ")
}

func (w *AssignmentWriter) SubmitForGrading(ctx context.Context, assignmentID string, acceptStatement bool) error {
	assignID, err := strconv.ParseInt(assignmentID, 10, 64)
	if err != nil {
		return errs.New(errs.CodeUsage, fmt.Sprintf("assignment id %q is not a number", assignmentID))
	}

	var warnings []writeWarning
	if err := w.client.Call(ctx, w.token, FunctionSubmitForGrading, Params{
		"assignmentid":              assignID,
		"acceptsubmissionstatement": acceptStatement,
	}, &warnings); err != nil {
		return err
	}
	if len(warnings) > 0 {
		// Unlike save_submission, this one puts a debugging line in "item" —
		// "User id: 7, Assignment id: 27 Notices:" — so there is nothing here
		// to pass on. Measured; the notices it promises are usually empty.
		failure := errs.New(errs.CodeConflict,
			"the site would not accept the submission for grading").
			WithHint("check the current state with `moodle assignment status`")
		failure.Upstream = &errs.Upstream{
			ErrorCode: warnings[0].WarningCode,
			Message:   strings.TrimSpace(warnings[0].Item),
		}
		return failure
	}
	return nil
}

// boolPtr marks a setting this route can actually see, as against one a
// page-reading route has to leave unknown.
func boolPtr(value bool) *bool { return &value }
