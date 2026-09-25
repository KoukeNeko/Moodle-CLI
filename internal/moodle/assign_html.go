package moodle

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

// PathAssignView is the page showing one assignment.
const PathAssignView = "/mod/assign/view.php"

// PathCourseView is the page listing a course's activities.
const PathCourseView = "/course/view.php"

// formatHTML is Moodle's FORMAT_HTML, the format a page's markup is in.
const formatHTML = 1

// Contract fields no page states in a form this route can read. They are
// reported as missing rather than left to read as answers: an empty plugin
// list or a false statement flag would be a claim about the assignment.
var (
	assignPageBlindFields = []string{
		"cut_off_date", "needs_hand_in", "requires_statement",
		"submission_plugins", "max_files", "max_bytes",
	}
	assignDetailBlindFields = []string{
		"max_grade", "allow_from", "time_limit_seconds", "max_attempts",
		"team_submission", "blind_marking", "identities_revealed",
	}
	assignStatusBlindFields = []string{
		"can_submit", "modified_at", "extension_due_date", "group_submission",
		"members_still_to_submit", "earlier_attempts", "timer_ends_at",
		"online_submission", "files.size", "files.mime_type", "files.modified_at",
	}
)

// AssignHTMLBackend reads assignments from Moodle's own pages.
//
// It is the last route, and the only one left on a site with mobile web
// services switched off: assignments are not exposed over the AJAX endpoint
// either.
//
// Identity works differently here, and it matters. A page carries only the
// course module id — the assignment's own id appears nowhere in the markup —
// so that is what this backend reports as the id. It is the number in the
// address bar, which is also the one a student can see, but it is not the same
// number the web service route uses for the same assignment.
type AssignHTMLBackend struct {
	pages *PageReader
	pageContext
}

// NewAssignHTMLBackend builds the page-reading backend for assignments.
func NewAssignHTMLBackend(pages *PageReader, calendar *AjaxSession,
	courses func(context.Context) ([]course.Summary, error)) *AssignHTMLBackend {
	return &AssignHTMLBackend{pages: pages,
		pageContext: pageContext{calendar: calendar, courses: courses}}
}

func (b *AssignHTMLBackend) Name() site.BackendKind { return site.BackendHTML }

// Requirement is empty: a page needs no function to be exposed. What it needs
// is a browser session, which the caller has already established by building
// this backend at all.
func (b *AssignHTMLBackend) Requirement() site.Requirement {
	return site.Requirement{}
}

func (b *AssignHTMLBackend) List(ctx context.Context, courseIDs []string) (assignment.ListResult, error) {
	known, err := b.knownCourses(ctx)
	if len(courseIDs) == 0 {
		if b.courses == nil {
			return assignment.ListResult{}, errs.New(errs.CodeUsage,
				"reading pages cannot find your courses on its own").
				WithHint("name the course with --course")
		}
		if err != nil {
			return assignment.ListResult{}, err
		}
		for _, item := range known {
			courseIDs = append(courseIDs, item.ID)
		}
	}

	result := assignment.ListResult{
		Assignments: []assignment.Summary{},
		Provenance:  site.NewProvenance(site.BackendHTML),
	}
	missing := append([]string{}, assignPageBlindFields...)
	var selected []course.Summary
	for _, courseID := range courseIDs {
		page, err := b.pages.Get(ctx, PathCourseView, map[string]string{"id": courseID})
		if err != nil {
			return assignment.ListResult{}, err
		}
		activities, err := webread.ParseCourseActivities(page)
		if err != nil {
			return assignment.ListResult{}, err
		}
		summary, named := known[courseID]
		if named {
			selected = append(selected, summary)
		} else if !contains(missing, "course_short_name") {
			missing = append(missing, "course_short_name")
		}
		for _, activity := range activities {
			if activity.Module != "assign" {
				continue
			}
			item := assignment.Summary{
				// Both are the course module id: it is the only identifier the
				// page carries, and pretending otherwise would invent one.
				ID:       activity.CMID,
				CMID:     activity.CMID,
				CourseID: courseID,
				Name:     activity.Name,
			}
			if named {
				short := summary.ShortName
				item.CourseShortName = &short
			}
			result.Assignments = append(result.Assignments, item)
		}
	}

	deadlines, err := b.events(ctx, "assign", selected, "")
	if err != nil || len(selected) < len(courseIDs) {
		// Without the calendar a null deadline would read as "none set".
		missing = append(missing, "due_date")
	}
	for i := range result.Assignments {
		result.Assignments[i].DueDate = deadlines.at(result.Assignments[i].CMID, "due")
	}
	result.Provenance.Partial = true
	result.Provenance.Missing = missing
	return result, nil
}

// Show returns what the page says about one assignment.
//
// The page shows the dates in prose meant for a reader, translated and
// formatted by the site, so those come from the calendar instead. The settings
// that decide the submission flow — whether saving is enough, whether a
// statement must be accepted — are not on the page in any form this build can
// read, and are reported as missing rather than guessed: reporting
// submissiondrafts wrongly would report a draft as handed in.
func (b *AssignHTMLBackend) Show(ctx context.Context, assignmentID string) (assignment.Detail, error) {
	markup, err := b.pages.Get(ctx, PathAssignView, map[string]string{"id": assignmentID})
	if err != nil {
		return assignment.Detail{}, err
	}
	page, err := webread.ParseAssignPage(markup)
	if err != nil {
		return assignment.Detail{}, err
	}

	detail := assignment.Detail{
		Summary: assignment.Summary{
			ID: assignmentID, CMID: assignmentID,
			CourseID: page.CourseID, Name: page.Name,
		},
		Description:       page.Description,
		DescriptionFormat: formatHTML,
		Provenance:        site.NewProvenance(site.BackendHTML),
	}
	for _, link := range page.Attachments {
		detail.Attachments = append(detail.Attachments, file.Ref{Name: link.Name, URL: link.URL})
	}

	missing := append(append([]string{}, assignPageBlindFields...), assignDetailBlindFields...)
	if page.Name == "" {
		missing = append(missing, "name")
	}
	known, _ := b.knownCourses(ctx)
	summary, named := known[page.CourseID]
	if named {
		short := summary.ShortName
		detail.CourseShortName = &short
	} else {
		missing = append(missing, "course_short_name")
	}
	if page.CourseID == "" {
		missing = append(missing, "course_id")
	}
	var selected []course.Summary
	if named {
		selected = append(selected, summary)
	}
	deadlines, err := b.events(ctx, "assign", selected, page.CourseID)
	if err != nil || !named {
		missing = append(missing, "due_date")
	}
	detail.DueDate = deadlines.at(assignmentID, "due")
	detail.Provenance.Partial = true
	detail.Provenance.Missing = missing
	return detail, nil
}

func (b *AssignHTMLBackend) Status(ctx context.Context, assignmentID string) (assignment.State, error) {
	page, err := b.pages.Get(ctx, PathAssignView, map[string]string{"id": assignmentID})
	if err != nil {
		return assignment.State{}, err
	}

	parsed, err := webread.ParseAssignStatus(page)
	if err != nil {
		return assignment.State{}, err
	}

	state := assignment.State{
		Status:     translateStatus(parsed.Status),
		FileCount:  len(parsed.Files),
		Provenance: site.NewProvenance(site.BackendHTML),
	}
	// The page says nothing this build can read about whether a new submission
	// would be accepted, and a lock is the one refusal it does state.
	state.CanEdit = !parsed.Locked
	state.CanSubmit = !parsed.Locked
	if parsed.Graded {
		state.GradingStatus = "graded"
	} else {
		state.GradingStatus = "notgraded"
	}
	for _, link := range parsed.Files {
		// The page's own link, which the same browser session can download.
		// Size, type and time are printed only as prose, if at all.
		state.Files = append(state.Files, file.Ref{Name: link.Name, URL: link.URL})
	}
	state.Provenance.Partial = true
	state.Provenance.Missing = append([]string{}, assignStatusBlindFields...)
	return state, nil
}
