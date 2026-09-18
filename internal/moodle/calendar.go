package moodle

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// FunctionActionEvents lists what the caller still has to do.
const FunctionActionEvents = "core_calendar_get_action_events_by_timesort"

// actionEventsDTO is Moodle's reply. The reply carries a good deal more than
// this — rendered HTML, edit and delete links — which is deliberately not read.
type actionEventsDTO struct {
	Events []struct {
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
	} `json:"events"`
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
	return b.route.requirement([]string{FunctionActionEvents})
}

func (b *CalendarBackend) Upcoming(ctx context.Context, q calendar.Query) (calendar.Result, error) {
	params := map[string]any{}
	// timesortfrom is deliberately left unset. Moodle then keeps returning an
	// action event after its deadline has passed, until the work is done —
	// which is exactly the list a student needs. Starting the window at "now"
	// would quietly drop the overdue work.
	if q.Until != nil {
		params["timesortto"] = q.Until.Unix()
	}
	if q.Limit > 0 {
		params["limitnum"] = q.Limit
	}

	var dto actionEventsDTO
	if err := b.route.call(ctx, FunctionActionEvents, params, &dto); err != nil {
		return calendar.Result{}, err
	}

	result := calendar.Result{
		Events:     []calendar.Event{},
		Provenance: site.NewProvenance(b.route.kind()),
	}
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
		result.Events = append(result.Events, event)
	}
	return result, nil
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
