package moodle

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

// PathAssignView is the page showing one assignment.
const PathAssignView = "/mod/assign/view.php"

// PathCourseView is the page listing a course's activities.
const PathCourseView = "/course/view.php"

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
	// courses lists the courses to look in. Reading a page cannot enumerate
	// enrolments, so the caller supplies that.
	courses func(ctx context.Context) ([]string, error)
}

// NewAssignHTMLBackend builds the page-reading backend for assignments.
func NewAssignHTMLBackend(pages *PageReader, courses func(context.Context) ([]string, error)) *AssignHTMLBackend {
	return &AssignHTMLBackend{pages: pages, courses: courses}
}

func (b *AssignHTMLBackend) Name() site.BackendKind { return site.BackendHTML }

// Requirement is empty: a page needs no function to be exposed. What it needs
// is a browser session, which the caller has already established by building
// this backend at all.
func (b *AssignHTMLBackend) Requirement() site.Requirement {
	return site.Requirement{}
}

func (b *AssignHTMLBackend) List(ctx context.Context, courseIDs []string) (assignment.ListResult, error) {
	if len(courseIDs) == 0 {
		if b.courses == nil {
			return assignment.ListResult{}, errs.New(errs.CodeUsage,
				"reading pages cannot find your courses on its own").
				WithHint("name the course with --course")
		}
		found, err := b.courses(ctx)
		if err != nil {
			return assignment.ListResult{}, err
		}
		courseIDs = found
	}

	result := assignment.ListResult{
		Assignments: []assignment.Summary{},
		Provenance:  site.NewProvenance(site.BackendHTML),
	}
	for _, courseID := range courseIDs {
		page, err := b.pages.Get(ctx, PathCourseView, map[string]string{"id": courseID})
		if err != nil {
			return assignment.ListResult{}, err
		}
		activities, err := webread.ParseCourseActivities(page)
		if err != nil {
			return assignment.ListResult{}, err
		}
		for _, activity := range activities {
			if activity.Module != "assign" {
				continue
			}
			result.Assignments = append(result.Assignments, assignment.Summary{
				// Both are the course module id: it is the only identifier the
				// page carries, and pretending otherwise would invent one.
				ID:       activity.CMID,
				CMID:     activity.CMID,
				CourseID: courseID,
				Name:     activity.Name,
			})
		}
	}
	return result, nil
}

// Show returns what the page says about one assignment.
//
// The page shows the description and the dates in prose meant for a reader,
// translated and formatted by the site. Rather than guess at those, this
// returns the identity and the caller's standing, which are the parts the page
// states unambiguously.
func (b *AssignHTMLBackend) Show(ctx context.Context, assignmentID string) (assignment.Detail, error) {
	state, err := b.Status(ctx, assignmentID)
	if err != nil {
		return assignment.Detail{}, err
	}
	// The settings that decide the submission flow — whether saving is enough,
	// whether a statement must be accepted — are not on the page in any form
	// this build can read. They are left unset rather than guessed: reporting
	// submissiondrafts wrongly would report a draft as handed in.
	_ = state
	return assignment.Detail{
		Summary: assignment.Summary{ID: assignmentID, CMID: assignmentID},
	}, nil
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
	for _, name := range parsed.Files {
		// Only the name is knowable here. The page links to the file, but
		// downloading it needs a token this route does not have.
		state.Files = append(state.Files, file.Ref{Name: name})
	}
	return state, nil
}
