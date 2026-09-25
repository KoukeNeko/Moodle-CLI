package moodle

import (
	"context"
	"strconv"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// FunctionCalendarMonth is the calendar's month view over the AJAX endpoint.
//
// It is how a browser session learns an activity's dates as numbers. The
// activity's own page prints them too, but as prose formatted in the site's
// language, and the action-event calls leave out work that is already handed
// in. The month view lists every event, done or not.
const FunctionCalendarMonth = "core_calendar_get_calendar_monthly_view"

// siteCourseID asks the month view for every course the account can see.
const siteCourseID = 1

// maxCalendarMonths bounds how many months one read may ask for, so a course
// with implausible dates cannot turn a listing into hundreds of requests.
const maxCalendarMonths = 36

// monthViewDTO is the part of the month view read here. The reply also
// carries each event's course with a base64 image, which is not read.
type monthViewDTO struct {
	Weeks []struct {
		Days []struct {
			Events []struct {
				ModuleName string `json:"modulename"`
				// Instance is the course module id, as in the action events.
				Instance  int64  `json:"instance"`
				EventType string `json:"eventtype"`
				TimeStart int64  `json:"timestart"`
			} `json:"events"`
		} `json:"days"`
	} `json:"weeks"`
}

// ActivityEvents maps a course module id to its events' times by event type,
// such as "due" for an assignment's deadline.
type ActivityEvents map[string]map[string]time.Time

// readActivityEvents reads one module's events from every month between from
// and to. courseID narrows the read to one course; empty reads them all.
func readActivityEvents(ctx context.Context, ajax *AjaxSession, module, courseID string, from, to time.Time) (ActivityEvents, error) {
	course := siteCourseID
	if courseID != "" {
		id, err := strconv.Atoi(courseID)
		if err != nil {
			return nil, errs.New(errs.CodeUsage, "course ids are numbers")
		}
		course = id
	}

	events := ActivityEvents{}
	month := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	for count := 0; !month.After(to); count++ {
		if count == maxCalendarMonths {
			// The months already read hold real dates, but a deadline in one
			// that was not read would come out as none at all.
			return nil, errs.New(errs.CodeUnavailable,
				"the courses span more months than one calendar read covers")
		}
		var dto monthViewDTO
		if err := ajax.Call(ctx, FunctionCalendarMonth, map[string]any{
			"year": month.Year(), "month": int(month.Month()), "day": 1,
			"courseid": course, "mini": true, "includenavigation": false,
		}, &dto); err != nil {
			return nil, err
		}
		for _, week := range dto.Weeks {
			for _, day := range week.Days {
				for _, event := range day.Events {
					if event.ModuleName != module || event.TimeStart == 0 {
						continue
					}
					cmid := strconv.FormatInt(event.Instance, 10)
					if events[cmid] == nil {
						events[cmid] = map[string]time.Time{}
					}
					events[cmid][event.EventType] = time.Unix(event.TimeStart, 0).UTC()
				}
			}
		}
		month = month.AddDate(0, 1, 0)
	}
	return events, nil
}
