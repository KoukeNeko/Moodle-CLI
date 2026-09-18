package moodle

import (
	"context"
	"strconv"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// FunctionTimelineCourses lists courses over the AJAX endpoint.
//
// It is not the function the web service route uses: core_enrol_get_users_courses
// is not exposed over AJAX at all. The two answer the same question with
// different shapes, which is why each backend maps its own.
const FunctionTimelineCourses = "core_course_get_enrolled_courses_by_timeline_classification"

// timelineCoursesDTO is the AJAX endpoint's reply.
//
// The reply also carries a base64 course image, which is deliberately not read:
// it is tens of kilobytes per course and nothing here shows pictures.
type timelineCoursesDTO struct {
	Courses []struct {
		ID        int64   `json:"id"`
		FullName  string  `json:"fullname"`
		ShortName string  `json:"shortname"`
		StartDate int64   `json:"startdate"`
		EndDate   int64   `json:"enddate"`
		Visible   bool    `json:"visible"`
		Progress  float64 `json:"progress"`
		// HasProgress separates a course at 0% from one that does not track
		// progress at all.
		HasProgress bool `json:"hasprogress"`
	} `json:"courses"`
	NextOffset int `json:"nextoffset"`
}

// CourseAjaxBackend lists courses over a browser session.
type CourseAjaxBackend struct {
	session *AjaxSession
}

// NewCourseAjaxBackend builds the AJAX backend for courses.
func NewCourseAjaxBackend(session *AjaxSession) *CourseAjaxBackend {
	return &CourseAjaxBackend{session: session}
}

func (b *CourseAjaxBackend) Name() site.BackendKind { return site.BackendAJAX }

// Requirement is empty on purpose.
//
// There is no way to ask an AJAX endpoint what it offers — get_site_info is
// itself not available there — so a capability check would have nothing to
// read. Availability is established by calling and reading the refusal.
func (b *CourseAjaxBackend) Requirement() site.Requirement {
	return site.Requirement{}
}

func (b *CourseAjaxBackend) List(ctx context.Context, q course.ListQuery) (course.ListResult, error) {
	var dto timelineCoursesDTO
	if err := b.session.Call(ctx, FunctionTimelineCourses, map[string]any{
		"classification": "all",
	}, &dto); err != nil {
		return course.ListResult{}, err
	}

	all := make([]course.Summary, 0, len(dto.Courses))
	for _, item := range dto.Courses {
		summary := course.Summary{
			ID:        strconv.FormatInt(item.ID, 10),
			ShortName: item.ShortName,
			FullName:  item.FullName,
			StartDate: unixTime(item.StartDate),
			EndDate:   unixTime(item.EndDate),
			Visible:   item.Visible,
		}
		if item.HasProgress {
			// Without the flag a course that does not track progress would be
			// reported as one the student has done none of.
			progress := item.Progress
			summary.Progress = &progress
		}
		all = append(all, summary)
	}

	page, next := window(all, q)
	return course.ListResult{
		Courses:    page,
		NextCursor: next,
		Provenance: site.NewProvenance(site.BackendAJAX),
	}, nil
}
