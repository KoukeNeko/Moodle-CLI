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
}

type lastAttemptDTO struct {
	SubmissionsEnabled bool   `json:"submissionsenabled"`
	CanEdit            bool   `json:"canedit"`
	CanSubmit          bool   `json:"cansubmit"`
	Locked             bool   `json:"locked"`
	Graded             bool   `json:"graded"`
	GradingStatus      string `json:"gradingstatus"`
	// Submission is absent entirely until a submission record exists, which
	// is different from one that exists and is empty: a record with status
	// "new" is a real state Moodle reports.
	Submission *struct {
		ID           int64  `json:"id"`
		Status       string `json:"status"`
		TimeModified int64  `json:"timemodified"`
		Plugins      []struct {
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
	} `json:"submission"`
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
	return assignment.ListResult{
		Assignments: out,
		Provenance:  site.NewProvenance(site.BackendWS),
	}, nil
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
		// No submission summary at all. Moodle shows one to a participant
		// when it has something to say, so its absence here means this
		// account is not a student on this assignment.
		if dto.GradingSummary != nil {
			return assignment.State{}, errs.New(errs.CodeUnavailable,
				"you are staff on this assignment, not a student on the course").
				WithHint(fmt.Sprintf(
					"there is no submission of yours here; the site counts %d to grade",
					dto.GradingSummary.ParticipantCount))
		}
		return assignment.State{}, errs.New(errs.CodeUnavailable,
			"this site did not report a submission status for you on this assignment")
	}
	state := assignment.State{
		Status:        assignment.StatusNew,
		CanEdit:       last.CanEdit,
		CanSubmit:     last.CanSubmit,
		GradingStatus: last.GradingStatus,
		Provenance:    site.NewProvenance(site.BackendWS),
	}
	if !last.SubmissionsEnabled {
		return state, errs.New(errs.CodeUnavailable,
			"this assignment is not accepting submissions").
			WithReason(errs.ReasonCapability)
	}
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
	case "draft", "reopened":
		return assignment.StatusDraft
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
	var warnings []struct {
		Item        string `json:"item"`
		WarningCode string `json:"warningcode"`
		Message     string `json:"message"`
	}
	if err := w.client.Call(ctx, w.token, FunctionSaveSubmission, Params{
		"assignmentid": assignID,
		"plugindata":   plugindata,
	}, &warnings); err != nil {
		return err
	}
	if len(warnings) > 0 {
		return errs.New(errs.CodeUpstream,
			fmt.Sprintf("Moodle refused the submission: %s", warnings[0].Message)).
			WithHint(warnings[0].Item)
	}
	return nil
}

func (w *AssignmentWriter) SubmitForGrading(ctx context.Context, assignmentID string, acceptStatement bool) error {
	assignID, err := strconv.ParseInt(assignmentID, 10, 64)
	if err != nil {
		return errs.New(errs.CodeUsage, fmt.Sprintf("assignment id %q is not a number", assignmentID))
	}

	var warnings []struct {
		Item        string `json:"item"`
		WarningCode string `json:"warningcode"`
		Message     string `json:"message"`
	}
	if err := w.client.Call(ctx, w.token, FunctionSubmitForGrading, Params{
		"assignmentid":              assignID,
		"acceptsubmissionstatement": acceptStatement,
	}, &warnings); err != nil {
		return err
	}
	if len(warnings) > 0 {
		return errs.New(errs.CodeUpstream,
			fmt.Sprintf("Moodle refused to accept the submission: %s", warnings[0].Message))
	}
	return nil
}

// boolPtr marks a setting this route can actually see, as against one a
// page-reading route has to leave unknown.
func boolPtr(value bool) *bool { return &value }
