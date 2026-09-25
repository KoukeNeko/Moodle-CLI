package moodle

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// FunctionUserCourses lists the courses a user is enrolled in.
const FunctionUserCourses = "core_enrol_get_users_courses"

// courseDTO is Moodle's course shape. Unexported: these names must not reach
// the JSON contract.
type courseDTO struct {
	ID        int64    `json:"id"`
	ShortName string   `json:"shortname"`
	FullName  string   `json:"fullname"`
	StartDate int64    `json:"startdate"`
	EndDate   int64    `json:"enddate"`
	Visible   int      `json:"visible"`
	Progress  *float64 `json:"progress"`
}

// CourseBackend lists courses over the web service API.
type CourseBackend struct {
	client       *Client
	token        string
	capabilities *site.Capabilities
}

// NewCourseBackend builds the web service backend for course listing.
func NewCourseBackend(client *Client, token string, capabilities *site.Capabilities) *CourseBackend {
	return &CourseBackend{client: client, token: token, capabilities: capabilities}
}

func (b *CourseBackend) Name() site.BackendKind { return site.BackendWS }

// Requirement declares the alternatives this backend can use. Naming them
// here lets the caller say which function is missing rather than reporting a
// bare "unavailable".
func (b *CourseBackend) Requirement() site.Requirement {
	return site.Requirement{
		AnyFunction: []string{FunctionUserCourses},
		Credential:  site.CredentialWSToken,
	}
}

func (b *CourseBackend) List(ctx context.Context, q course.ListQuery) (course.ListResult, error) {
	if b.capabilities == nil || b.capabilities.UserID == "" {
		return course.ListResult{}, errs.New(errs.CodeInternal,
			"the account's Moodle user id is unknown")
	}
	userID, err := strconv.ParseInt(b.capabilities.UserID, 10, 64)
	if err != nil {
		return course.ListResult{}, errs.Wrap(errs.CodeInternal, err,
			"the account's Moodle user id is not a number")
	}

	var dtos []courseDTO
	if err := b.client.Call(ctx, b.token, FunctionUserCourses,
		Params{"userid": userID}, &dtos); err != nil {
		return course.ListResult{}, err
	}

	all := make([]course.Summary, 0, len(dtos))
	for _, dto := range dtos {
		all = append(all, course.Summary{
			ID:        strconv.FormatInt(dto.ID, 10),
			ShortName: dto.ShortName,
			FullName:  dto.FullName,
			StartDate: unixTime(dto.StartDate),
			EndDate:   unixTime(dto.EndDate),
			Visible:   dto.Visible != 0,
			Progress:  dto.Progress,
		})
	}

	// This function returns every course at once: Moodle offers no paging for
	// it. The window is therefore applied here, which keeps the cursor in the
	// contract honest even though the request itself is not paged.
	page, next := window(all, q)
	return course.ListResult{
		Courses:    page,
		Provenance: site.NewProvenance(site.BackendWS),
		NextCursor: next,
	}, nil
}

// unixTime converts Moodle's seconds-since-epoch. Moodle uses 0 for "not set",
// which must become null rather than 1 January 1970.
func unixTime(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	t := time.Unix(seconds, 0).UTC()
	return &t
}

// window applies the query's filter, limit and cursor to a full listing. The
// filter comes first, so a cursor counts the same courses on every page.
func window(all []course.Summary, q course.ListQuery) ([]course.Summary, string) {
	if q.Current {
		now := time.Now()
		running := make([]course.Summary, 0, len(all))
		for _, item := range all {
			if item.Running(now) {
				running = append(running, item)
			}
		}
		all = running
	}
	offset := decodeCursor(q.Cursor)
	if offset > len(all) {
		offset = len(all)
	}
	rest := all[offset:]
	if q.Limit <= 0 || q.Limit >= len(rest) {
		return rest, ""
	}
	return rest[:q.Limit], encodeCursor(offset + q.Limit)
}

// The cursor is opaque by contract, so its encoding can change without
// breaking anyone.
const cursorPrefix = "o:"

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(cursorPrefix + strconv.Itoa(offset)))
}

func decodeCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	text, ok := strings.CutPrefix(string(raw), cursorPrefix)
	if !ok {
		return 0
	}
	offset, err := strconv.Atoi(text)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}
