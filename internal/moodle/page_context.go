package moodle

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// pageContext is what the page-reading routes lean on besides the pages.
//
// A page names an activity and its course module, and nothing else worth
// trusting: it cannot enumerate enrolments or name a course, and it prints
// dates only as prose in the site's language. The course listing and the
// calendar, both open to a browser session, answer those as data.
type pageContext struct {
	// calendar reads activity dates; nil where there is no session for it.
	calendar *AjaxSession
	// courses lists the account's courses, with their names and dates.
	courses func(ctx context.Context) ([]course.Summary, error)
}

// knownCourses indexes the account's courses by id. Nil when there is no way
// to list them; the error says why.
func (p pageContext) knownCourses(ctx context.Context) (map[string]course.Summary, error) {
	if p.courses == nil {
		return nil, errs.New(errs.CodeUnavailable, "no course listing on this route")
	}
	list, err := p.courses(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[string]course.Summary, len(list))
	for _, item := range list {
		known[item.ID] = item
	}
	return known, nil
}

// events reads one module's calendar events across the months the given
// courses run, with a month either side for a date set just outside them.
// courseID narrows the read when there is only one course.
func (p pageContext) events(ctx context.Context, module string, courses []course.Summary, courseID string) (ActivityEvents, error) {
	if p.calendar == nil || len(courses) == 0 {
		return nil, errs.New(errs.CodeUnavailable, "no calendar to read dates from")
	}
	now := time.Now()
	var from, to time.Time
	for _, item := range courses {
		start := now.AddDate(-1, 0, 0)
		if item.StartDate != nil {
			start = *item.StartDate
		}
		// A course without an end date is still running; half a year ahead
		// covers a term.
		end := now.AddDate(0, 6, 0)
		if item.EndDate != nil {
			end = *item.EndDate
		}
		if from.IsZero() || start.Before(from) {
			from = start
		}
		if to.IsZero() || end.After(to) {
			to = end
		}
	}
	return readActivityEvents(ctx, p.calendar, module, courseID,
		from.AddDate(0, -1, 0), to.AddDate(0, 1, 0))
}

// at returns one event's time, nil when the calendar holds no such event.
func (e ActivityEvents) at(cmid, eventType string) *time.Time {
	when, ok := e[cmid][eventType]
	if !ok {
		return nil
	}
	return &when
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
