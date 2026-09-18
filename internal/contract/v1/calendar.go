package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
)

// CalendarEvent is one thing the caller has to do, on the wire.
type CalendarEvent struct {
	ID string `json:"id"`
	// Title is Moodle's own sentence; activity is the thing itself.
	Title    string `json:"title"`
	Activity string `json:"activity"`
	// Kind is Moodle's event type: due, open, close and so on. It is passed
	// through rather than mapped, because a site's plugins define their own.
	Kind   string `json:"kind"`
	Module string `json:"module"`
	// CMID is the course module id, which is what links an event back to the
	// activity. It is NOT the activity's own id: an assignment with id 1 has a
	// cmid of 2, and joining on the wrong one points at a different activity.
	// Join it against an assignment's `cmid`.
	CMID            string  `json:"cmid"`
	CourseID        string  `json:"course_id"`
	CourseShortName string  `json:"course_short_name"`
	DueAt           *string `json:"due_at"`
	// Overdue is Moodle's own verdict, which accounts for extensions and
	// cut-offs that a comparison against the clock cannot see.
	Overdue bool `json:"overdue"`
	// ActionName is what Moodle suggests doing, null when it suggests nothing.
	ActionName *string `json:"action_name"`
	// Actionable reports whether the caller can still act: an event can be
	// listed and past acting on.
	Actionable bool `json:"actionable"`
	ItemCount  *int `json:"item_count"`
	// URL is null when Moodle's link carried a session key, which is never
	// passed on.
	URL *string `json:"url"`
}

// CalendarUpcoming converts a listing into its envelope.
func CalendarUpcoming(result calendar.Result, siteName, accountName string) Envelope {
	events := make([]CalendarEvent, 0, len(result.Events))
	for _, item := range result.Events {
		event := CalendarEvent{
			ID:              item.ID,
			Title:           item.Title,
			Activity:        item.Activity,
			Kind:            item.Kind,
			Module:          item.Module,
			CMID:            item.CMID,
			CourseID:        item.CourseID,
			CourseShortName: item.CourseShortName,
			DueAt:           Timestamp(item.At),
			Overdue:         item.Overdue,
			URL:             optional(item.URL),
		}
		if item.Action != nil {
			event.ActionName = optional(item.Action.Name)
			event.Actionable = item.Action.Actionable
			if item.Action.ItemCount > 0 {
				count := item.Action.ItemCount
				event.ItemCount = &count
			}
		}
		events = append(events, event)
	}
	return NewEnvelope("calendar.upcoming", events,
		MetaFrom(result.Provenance, siteName, accountName))
}
