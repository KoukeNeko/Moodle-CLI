package moodle

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// FunctionActionEvents lists what the caller still has to do.
const FunctionActionEvents = "core_calendar_get_action_events_by_timesort"

// FunctionMonthlyView is the site's own calendar, a month at a time — the
// surface its web interface renders.
//
// It is the base collection rather than the action list above, because this
// command promises deadlines and the action list only ever promised actions.
// Moodle's own upcoming view was the other candidate and was rejected: it is
// a window of calendar_lookahead days capped at calendar_maxevents, neither
// of which the caller may set, so it cannot answer --days honestly.
const FunctionMonthlyView = "core_calendar_get_calendar_monthly_view"

// actionEventsDTO is Moodle's reply. The reply carries a good deal more than
// this — rendered HTML, edit and delete links — which is deliberately not read.
type actionEventsDTO struct {
	Events []calendarEventDTO `json:"events"`
}

// monthlyViewDTO is the site's own calendar for one month. Only the grid is
// read: the reply also carries navigation links and rendered HTML.
type monthlyViewDTO struct {
	Weeks []struct {
		Days []struct {
			Events []calendarEventDTO `json:"events"`
		} `json:"days"`
	} `json:"weeks"`
}

// calendarEventDTO is one event. Both calendar calls answer with this shape —
// checked field by field against 4.5, 5.1 and 5.2 — so they share it.
type calendarEventDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// ActivityName is the activity without the sentence Moodle wraps
	// around it in Name.
	ActivityName string `json:"activityname"`
	ModuleName   string `json:"modulename"`
	EventType    string `json:"eventtype"`
	// Instance is the course module id despite its name; see the note on
	// calendar.Event.CMID.
	Instance int64  `json:"instance"`
	TimeSort int64  `json:"timesort"`
	Overdue  bool   `json:"overdue"`
	URL      string `json:"url"`
	Course   *struct {
		ID        int64  `json:"id"`
		ShortName string `json:"shortname"`
	} `json:"course"`
	Action *struct {
		Name       string `json:"name"`
		URL        string `json:"url"`
		ItemCount  int    `json:"itemcount"`
		Actionable bool   `json:"actionable"`
	} `json:"action"`
}

// CalendarBackend reads the calendar over the web service API.
type CalendarBackend struct {
	route route
}

// NewCalendarBackend builds the web service backend for the calendar.
func NewCalendarBackend(client *Client, token string) *CalendarBackend {
	return &CalendarBackend{route: wsRoute{client: client, token: token}}
}

// NewCalendarAjaxBackend builds the browser-session backend. The endpoint
// answers with the same shape, so only the route differs.
func NewCalendarAjaxBackend(session *AjaxSession) *CalendarBackend {
	return &CalendarBackend{route: ajaxRoute{session: session}}
}

func (b *CalendarBackend) Name() site.BackendKind { return b.route.kind() }

func (b *CalendarBackend) Requirement() site.Requirement {
	// Either will do. The calendar is what this command is about, but a site
	// that only offers the action list can still answer a narrower version of
	// the question, and refusing outright would help nobody.
	return b.route.requirement([]string{FunctionMonthlyView, FunctionActionEvents})
}

// months reads the site's own calendar across every month the window touches.
//
// A month at a time is what Moodle offers, and it is asked for from the start
// of the current month so that work whose deadline has just passed is still in
// the answer. Each month's events carry their own action, where the activity
// supplies one, so telling a student's view from a teacher's costs no extra
// call.
func (b *CalendarBackend) months(ctx context.Context, from, until time.Time) ([]calendar.Event, error) {
	var events []calendar.Event
	cursor := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
	for !cursor.After(until) {
		var dto monthlyViewDTO
		if err := b.route.call(ctx, FunctionMonthlyView, map[string]any{
			"year": cursor.Year(), "month": int(cursor.Month()),
		}, &dto); err != nil {
			return nil, err
		}
		for _, week := range dto.Weeks {
			for _, day := range week.Days {
				events = append(events, translateEvents(actionEventsDTO{Events: day.Events})...)
			}
		}
		cursor = cursor.AddDate(0, 1, 0)
	}
	return events, nil
}

// actions reads what this account still has to do.
//
// It is kept alongside the calendar because the two answer different
// questions and neither contains the other: this one reaches back past the
// window to work that was never handed in, and Moodle does not restrict it to
// non-suspended enrolments unless asked. Its rows are joined onto the
// calendar's by event id, and one that appears only here says so — it is not
// evidence that the site's calendar showed it.
func (b *CalendarBackend) actions(ctx context.Context, q calendar.Query) []calendar.Event {
	params := map[string]any{}
	// timesortfrom is deliberately left unset. Moodle then keeps returning an
	// action event after its deadline has passed, until the work is done —
	// which is exactly the list a student needs. Starting the window at "now"
	// would quietly drop the overdue work.
	if q.Until != nil {
		params["timesortto"] = q.Until.Unix()
	}
	var dto actionEventsDTO
	if err := b.route.call(ctx, FunctionActionEvents, params, &dto); err != nil {
		return nil
	}
	return translateEvents(dto)
}

func (b *CalendarBackend) Upcoming(ctx context.Context, q calendar.Query) (calendar.Result, error) {
	now := time.Now()
	until := now.AddDate(1, 0, 0)
	if q.Until != nil {
		until = *q.Until
	}

	provenance := site.NewProvenance(b.route.kind())
	events, err := b.months(ctx, now, until)
	if err != nil {
		// The calendar is the wider of the two answers, so losing it narrows
		// what the listing can be read as. The actions below still answer, but
		// the caller is told the set shrank rather than left to read a short
		// list as a short calendar.
		provenance.Partial = true
		provenance.Missing = append(provenance.Missing, FunctionMonthlyView)
		events = nil
	}
	for i := range events {
		events[i].Source = calendar.FromCalendar
	}

	result := calendar.Result{
		Events:     join(events, b.actions(ctx, q)),
		Provenance: provenance,
	}
	// A month is a coarser slice than the caller asked for, so the edges have
	// to be trimmed back to the window they actually named.
	kept := result.Events[:0]
	for _, event := range result.Events {
		switch {
		case event.At == nil:
		case event.At.After(until):
			continue
		// Work that is late is still work, so the floor only applies to a
		// deadline the account has nothing left to do about.
		case event.At.Before(now) && event.Action == nil:
			continue
		}
		kept = append(kept, event)
	}
	result.Events = kept
	if q.Limit > 0 && len(result.Events) > q.Limit {
		result.Events = result.Events[:q.Limit]
	}
	return result, nil
}

// translateEvents turns Moodle's reply into ours. Both calendar calls answer
// with the same event shape, so they share this.
func translateEvents(dto actionEventsDTO) []calendar.Event {
	events := make([]calendar.Event, 0, len(dto.Events))
	for _, raw := range dto.Events {
		event := calendar.Event{
			ID:       strconv.FormatInt(raw.ID, 10),
			Title:    raw.Name,
			Activity: raw.ActivityName,
			Kind:     raw.EventType,
			Module:   raw.ModuleName,
			CMID:     strconv.FormatInt(raw.Instance, 10),
			At:       unixTime(raw.TimeSort),
			Overdue:  raw.Overdue,
			URL:      safeURL(raw.URL),
		}
		if raw.Course != nil {
			event.CourseID = strconv.FormatInt(raw.Course.ID, 10)
			event.CourseShortName = raw.Course.ShortName
		}
		if raw.Action != nil {
			event.Action = &calendar.Action{
				Name:       raw.Action.Name,
				URL:        safeURL(raw.Action.URL),
				ItemCount:  raw.Action.ItemCount,
				Actionable: raw.Action.Actionable,
			}
		}
		events = append(events, event)
	}
	return events
}

// join puts the action list onto the calendar's by event id.
//
// It is a join and not a union: the two calls searched different universes,
// so an event only the action list returned has not been shown to be on this
// account's calendar. Those rows are kept — the command promises work that is
// late — but they keep saying where they came from.
func join(events, actions []calendar.Event) []calendar.Event {
	seen := make(map[string]bool, len(events))
	for _, event := range events {
		seen[event.ID] = true
	}
	out := events
	for _, action := range actions {
		if seen[action.ID] {
			continue
		}
		action.Source = calendar.FromActions
		out = append(out, action)
	}
	sort.SliceStable(out, func(i, j int) bool {
		switch {
		case out[i].At == nil:
			return false
		case out[j].At == nil:
			return true
		default:
			return out[i].At.Before(*out[j].At)
		}
	})
	return out
}

// safeURL drops a link that carries a secret.
//
// Moodle's calendar hands out edit and delete links with the user's sesskey in
// them. None of those are read here, but a link is not something to pass on
// trustingly: anything that reaches the JSON contract can be logged, piped and
// kept, and a session key in a log is a session key given away.
func safeURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	for name := range parsed.Query() {
		if secretParams[strings.ToLower(name)] {
			return ""
		}
	}
	return raw
}
