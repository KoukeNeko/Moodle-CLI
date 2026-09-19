package moodle

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Grade web service functions.
const (
	FunctionGradeItems = "gradereport_user_get_grade_items"
	// FunctionGradableUsers answers the one question an empty gradebook leaves
	// open: whether this account is a graded participant at all.
	FunctionGradableUsers = "core_grades_get_gradable_users"
	FunctionCourseGrades  = "gradereport_overview_get_course_grades"
)

// gradeItemsDTO is Moodle's reply to gradereport_user_get_grade_items. The
// field names were taken from real 4.5, 5.1 and 5.2 responses. Fields that
// appear only once an item is graded, such as the weights, are simply absent
// before that.
type gradeItemsDTO struct {
	UserGrades []struct {
		CourseID   int64 `json:"courseid"`
		UserID     int64 `json:"userid"`
		GradeItems []struct {
			ID             int64        `json:"id"`
			ItemName       *string      `json:"itemname"`
			ItemType       string       `json:"itemtype"`
			ItemModule     *string      `json:"itemmodule"`
			CMID           *int64       `json:"cmid"`
			ScaleID        *int64       `json:"scaleid"`
			GradeRaw       *float64     `json:"graderaw"`
			GradeMin       *float64     `json:"grademin"`
			GradeMax       *float64     `json:"grademax"`
			GradeFormatted string       `json:"gradeformatted"`
			GradedAt       *json.Number `json:"gradedategraded"`
			GradeIsHidden  bool         `json:"gradeishidden"`
			GradeIsLocked  *bool        `json:"gradeislocked"`
			Feedback       string       `json:"feedback"`
			FeedbackFormat int          `json:"feedbackformat"`
		} `json:"gradeitems"`
	} `json:"usergrades"`
}

// courseGradesDTO is Moodle's reply to the overview call. It is much thinner
// than the per-course report: there is no maximum here, only the site's own
// rendering and the raw number.
type courseGradesDTO struct {
	Grades []struct {
		CourseID int64        `json:"courseid"`
		Grade    string       `json:"grade"`
		RawGrade *json.Number `json:"rawgrade"`
	} `json:"grades"`
}

// GradeBackend reads grades over the web service API.
type GradeBackend struct {
	client       *Client
	token        string
	capabilities *site.Capabilities
}

// NewGradeBackend builds the web service backend for grades.
func NewGradeBackend(client *Client, token string, capabilities *site.Capabilities) *GradeBackend {
	return &GradeBackend{client: client, token: token, capabilities: capabilities}
}

func (b *GradeBackend) Name() site.BackendKind { return site.BackendWS }

func (b *GradeBackend) Requirement() site.Requirement {
	return site.Requirement{
		AnyFunction: []string{FunctionGradeItems},
		Credential:  site.CredentialWSToken,
	}
}

// userID is the caller's own id.
//
// It has to be sent explicitly: without it Moodle reads the call as a request
// for everyone's grades and refuses it as "View grades of other users", which
// reads like a permissions problem with the account rather than a missing
// parameter.
func (b *GradeBackend) userID() (int64, error) {
	if b.capabilities == nil || b.capabilities.UserID == "" {
		return 0, errs.New(errs.CodeInternal, "the signed-in user is not known")
	}
	id, err := strconv.ParseInt(b.capabilities.UserID, 10, 64)
	if err != nil {
		return 0, errs.New(errs.CodeUpstream,
			fmt.Sprintf("the site reported user id %q, which is not a number", b.capabilities.UserID)).
			WithReason(errs.ReasonProtocolDrift)
	}
	return id, nil
}

func (b *GradeBackend) Course(ctx context.Context, courseID string) (grade.CourseResult, error) {
	id, err := strconv.ParseInt(courseID, 10, 64)
	if err != nil {
		return grade.CourseResult{}, errs.New(errs.CodeUsage,
			fmt.Sprintf("course id %q is not a number", courseID))
	}
	user, err := b.userID()
	if err != nil {
		return grade.CourseResult{}, err
	}

	var dto gradeItemsDTO
	if err := b.client.Call(ctx, b.token, FunctionGradeItems, Params{
		"courseid": id, "userid": user,
	}, &dto); err != nil {
		return grade.CourseResult{}, err
	}
	if len(dto.UserGrades) == 0 {
		return grade.CourseResult{}, errs.New(errs.CodeNotFound,
			fmt.Sprintf("no gradebook for course %s", courseID)).
			WithHint("check the course id with `moodle course list`")
	}

	result := grade.CourseResult{
		CourseID:   courseID,
		Items:      []grade.Item{},
		Provenance: site.NewProvenance(site.BackendWS),
	}
	for _, raw := range dto.UserGrades[0].GradeItems {
		item := grade.Item{
			ID:             strconv.FormatInt(raw.ID, 10),
			Kind:           translateGradeKind(raw.ItemType),
			Grade:          raw.GradeRaw,
			Min:            raw.GradeMin,
			Max:            raw.GradeMax,
			Display:        raw.GradeFormatted,
			Feedback:       raw.Feedback,
			FeedbackFormat: raw.FeedbackFormat,
			GradedAt:       numberTime(raw.GradedAt),
			Hidden:         boolPtr(raw.GradeIsHidden),
			Locked:         raw.GradeIsLocked,
			UsesScale:      raw.ScaleID != nil,
		}
		if raw.ItemName != nil {
			item.Name = *raw.ItemName
		}
		if raw.ItemModule != nil {
			item.Module = *raw.ItemModule
		}
		if raw.CMID != nil {
			item.CMID = strconv.FormatInt(*raw.CMID, 10)
		}
		item.Locked = raw.GradeIsLocked
		item.Percentage = grade.Percentage(item.Grade, item.Min, item.Max, item.UsesScale)

		if item.Kind == grade.KindCourse {
			// The course total is the answer to a different question from the
			// rows above it, and its maximum counts only what has been graded
			// so far — so it is never the sum of the items listed.
			total := item
			result.Total = &total
			continue
		}
		result.Items = append(result.Items, item)
	}
	if noGradeRecorded(result) {
		result.NotGradable = b.notAGradedParticipant(ctx, id, user)
	}
	return result, nil
}

// noGradeRecorded reports that not one row carries a grade.
//
// This is the only shape that is ambiguous. A gradebook with any grade in it
// has already said this account is graded here, so the probe below is not
// worth a round trip.
func noGradeRecorded(result grade.CourseResult) bool {
	if result.Total != nil && result.Total.Grade != nil {
		return false
	}
	for _, item := range result.Items {
		if item.Grade != nil {
			return false
		}
	}
	return true
}

// gradableUsersDTO is Moodle's reply to core_grades_get_gradable_users.
type gradableUsersDTO struct {
	Users []struct {
		ID int64 `json:"id"`
	} `json:"users"`
}

// notAGradedParticipant asks the site whether this account is graded here.
//
// gradereport_user_get_grade_items lets a teacher through on
// moodle/grade:viewall and then answers about themselves with the course's
// item list and no grades — indistinguishable from a student whose work is
// not marked yet. Nothing in that reply separates the two: an absent
// grade_grade row is materialised as an empty one either way.
//
// Moodle's own gradebook settles it with get_gradable_users(), which is
// enrolment plus the site's configured gradebook roles rather than a role
// name — and role names are a site's to choose. The web service in front of
// it needs moodle/site:viewuseridentity, which staff hold and students do not;
// measured. That is the right way round, because staff is the ambiguous side.
//
// A refusal is not evidence. A site can grant that capability to anyone, so
// "the call failed" says nothing about who this is, and the answer stays no.
func (b *GradeBackend) notAGradedParticipant(ctx context.Context, courseID, user int64) bool {
	var dto gradableUsersDTO
	if err := b.client.Call(ctx, b.token, FunctionGradableUsers, Params{
		"courseid": courseID,
	}, &dto); err != nil {
		return false
	}
	for _, candidate := range dto.Users {
		if candidate.ID == user {
			return false
		}
	}
	return len(dto.Users) > 0
}

func (b *GradeBackend) Overview(ctx context.Context) (grade.OverviewResult, error) {
	user, err := b.userID()
	if err != nil {
		return grade.OverviewResult{}, err
	}

	var dto courseGradesDTO
	if err := b.client.Call(ctx, b.token, FunctionCourseGrades,
		Params{"userid": user}, &dto); err != nil {
		return grade.OverviewResult{}, err
	}

	result := grade.OverviewResult{
		Courses:    []grade.CourseGrade{},
		Provenance: site.NewProvenance(site.BackendWS),
	}
	for _, raw := range dto.Grades {
		course := grade.CourseGrade{
			CourseID: strconv.FormatInt(raw.CourseID, 10),
			Display:  raw.Grade,
		}
		if raw.RawGrade != nil {
			if value, err := raw.RawGrade.Float64(); err == nil {
				course.Grade = &value
			}
		}
		result.Courses = append(result.Courses, course)
	}
	return result, nil
}

// numberTime converts a timestamp Moodle may send as a number or as a string.
//
// gradedategraded arrives as a number here and null when the work has not been
// marked, but the grade report is one of the places where Moodle's own types
// have shifted between releases, so it is read leniently.
func numberTime(value *json.Number) *time.Time {
	if value == nil {
		return nil
	}
	seconds, err := value.Int64()
	if err != nil {
		return nil
	}
	return unixTime(seconds)
}

// translateGradeKind maps Moodle's word onto ours. An unrecognised type stays
// unknown rather than being presented as an activity that does not exist.
func translateGradeKind(raw string) grade.Kind {
	switch raw {
	case "mod":
		return grade.KindActivity
	case "category":
		return grade.KindCategory
	case "course":
		return grade.KindCourse
	case "manual":
		return grade.KindManual
	default:
		return grade.KindUnknown
	}
}
