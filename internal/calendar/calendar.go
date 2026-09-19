// Package calendar is the deadline and to-do use cases.
//
// What a student wants from a calendar is not a list of dates but an answer to
// "what do I still have to do". Moodle calls those action events, and it keeps
// returning one after its deadline has passed until the work is actually done.
// That is the behaviour this package preserves: overdue work is the most
// important thing on the list, so it is never filtered out to make the list
// look tidy.
package calendar

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Event is one thing the caller has to do.
type Event struct {
	ID string
	// Title is Moodle's own sentence, such as "Essay 1 is due".
	Title string
	// Activity is the thing itself, without the sentence around it.
	Activity string
	// Kind is Moodle's event type: "due", "open", "close" and so on.
	Kind string
	// Module is the activity type, such as "assign".
	Module string
	// CMID is the course module id.
	//
	// Moodle sends it in a field called "instance", which elsewhere means the
	// activity's own id. It is not that here: an assignment with id 1 appears
	// as instance 2 when its course module is 2. Treating it as the activity
	// id silently links to a different activity, and since both are small
	// integers nothing looks wrong. Join it against an assignment's CMID.
	CMID            string
	CourseID        string
	CourseShortName string
	// At is when the event sorts by, which for a deadline is the deadline.
	At *time.Time
	// Overdue is Moodle's own verdict, not a comparison against the clock:
	// extensions and cut-offs are its business, not ours.
	Overdue bool
	Action  *Action
	// URL points at the activity. Anything Moodle sends carrying a session key
	// is dropped rather than passed on.
	URL string
	// Source is which answer this came from; see Source.
	Source Source
}

// Source says which of the site's answers an event came from. The two search
// different universes, so an event that only the action list returned has not
// been shown to be on this account's calendar — and a listing that quietly
// mixed them would be claiming exactly that.
type Source string

const (
	// FromCalendar: the site's own calendar returned this event.
	FromCalendar Source = "calendar"
	// FromActions: only the list of outstanding actions returned it.
	FromActions Source = "actions"
)

// Action is what Moodle suggests doing about an event.
type Action struct {
	Name string
	URL  string
	// ItemCount is how many things await, where the activity counts them.
	ItemCount int
	// Actionable reports whether the caller can still act. An event can be
	// listed and not actionable, for instance once a cut-off has passed.
	Actionable bool
}

// Query narrows a listing.
type Query struct {
	// Until caps how far ahead to look. Zero means Moodle's own horizon.
	Until *time.Time
	// Limit caps the number of events. Zero means the backend's default.
	Limit int
}

// Result is a listing plus where it came from.
type Result struct {
	Events     []Event
	Provenance site.Provenance
}

// Backend is one way of reading the calendar.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	Upcoming(ctx context.Context, q Query) (Result, error)
}
