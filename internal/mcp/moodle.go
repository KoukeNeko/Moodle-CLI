package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/wsregistry"
)

// Deps are the use cases this server serves.
//
// They arrive already built and already bound to one site and one account: an
// agent session is a session, and letting a tool call choose a different site
// would make every answer's provenance a question.
type Deps struct {
	// Capabilities asks what the site offers, rather than holding the answer.
	//
	// A tool call in this server can happen hours after the last one, and a
	// site's administrator can switch a function on in between. Captured once,
	// the answer went on refusing a tool that had become available — measured,
	// and only a restart cleared it.
	Capabilities func(context.Context) (*site.Capabilities, error)
	SiteName     string
	AccountName  string

	Courses     *course.Service
	Assignments *assignment.Service
	Grades      *grade.Service
	Calendar    *calendar.Service
	Forums      *forum.Service
	WSRegistry  *wsregistry.Registry
	WS          *wsregistry.Service
}

// now is the clock, replaceable in tests.
var now = time.Now

// schema is a small helper for writing an input schema inline.
func schema(raw string) json.RawMessage { return json.RawMessage(raw) }

const noArguments = `{"type":"object","properties":{},"additionalProperties":false}`

// Register builds the tool surface for a session.
//
// Read tools are always present. A tool that can change something is only
// registered when the session was started with writing allowed, so an agent
// never sees a tool it would be refused.
func Register(deps Deps, allowWrite bool) *Registry {
	r := newRegistry(allowWrite, deps.SiteName)
	registerWSTools(r, deps)

	r.add(definition{
		Name:        "course_list",
		Title:       "List courses",
		Description: "The courses visible to the signed-in account through its current service and capabilities.",
		Schema:      schema(noArguments),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Courses.List(ctx, capabilities, course.ListQuery{})
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.CourseList(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "assignment_list",
		Title: "List assignments",
		Description: "Assignments, with needs_hand_in saying whether saving work is " +
			"enough or a separate hand-in is required.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"course_ids": {
					"type": "array", "items": {"type": "string"},
					"description": "Limit to these courses; every course when omitted"
				}
			},
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Assignments.List(ctx, capabilities, args.Strings("course_ids"))
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.AssignmentList(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "assignment_show",
		Title: "Show one assignment",
		Description: "One assignment's description, dates and limits, together with " +
			"the signed-in account's own submission state when Moodle exposes one.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"assignment": {
					"type": "string",
					"description": "An assignment id, or a Moodle address copied from a browser"
				}
			},
			"required": ["assignment"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			ref, err := args.Required("assignment")
			if err != nil {
				return v1.Envelope{}, err
			}
			id, err := deps.Assignments.Locate(ctx, capabilities, ref)
			if err != nil {
				return v1.Envelope{}, err
			}
			detail, err := deps.Assignments.Show(ctx, capabilities, id)
			if err != nil {
				return v1.Envelope{}, err
			}
			state, err := deps.Assignments.Status(ctx, capabilities, id)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.AssignmentShow(detail, state, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "assignment_status",
		Title: "Check a submission",
		Description: "Whether the signed-in account's work is unsubmitted, saved as a draft, or " +
			"handed in for grading. handed_in is the answer; a draft is not submitted.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"assignment": {
					"type": "string",
					"description": "An assignment id, or a Moodle address copied from a browser"
				}
			},
			"required": ["assignment"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			ref, err := args.Required("assignment")
			if err != nil {
				return v1.Envelope{}, err
			}
			id, err := deps.Assignments.Locate(ctx, capabilities, ref)
			if err != nil {
				return v1.Envelope{}, err
			}
			state, err := deps.Assignments.Status(ctx, capabilities, id)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.AssignmentStatus(id, state, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "grade_overview",
		Title: "Grades across courses",
		Description: "The signed-in account's visible total in every course. This call carries no maximum, " +
			"so there is no percentage to compute from it.",
		Schema: schema(noArguments),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Grades.Overview(ctx, capabilities)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.GradeOverview(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "grade_list",
		Title: "One course's gradebook",
		Description: "Grades and feedback for one course. An unmarked item has a null " +
			"grade, which is not a zero; the course total counts only what has been " +
			"marked, so it is not the sum of the items.",
		Schema: schema(`{
			"type": "object",
			"properties": {"course_id": {"type": "string"}},
			"required": ["course_id"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			id, err := args.Required("course_id")
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Grades.Course(ctx, capabilities, id)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.GradeList(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "calendar_upcoming",
		Title: "What is still to do",
		Description: "Deadlines and to-dos, earliest first. Work whose deadline has " +
			"passed is included and marked overdue rather than left out.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"days": {"type": "integer", "description": "How many days ahead to look"},
				"limit": {"type": "integer"}
			},
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			query := calendar.Query{}
			if days := args.Int("days"); days > 0 {
				until := now().AddDate(0, 0, days)
				query.Until = &until
			}
			query.Limit = args.Int("limit")
			result, err := deps.Calendar.Upcoming(ctx, capabilities, query)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.CalendarUpcoming(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:        "forum_list",
		Title:       "List forums",
		Description: "The discussion areas visible to the signed-in account.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"course_ids": {"type": "array", "items": {"type": "string"}}
			},
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Forums.List(ctx, capabilities, args.Strings("course_ids"))
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.ForumList(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:        "forum_discussions",
		Title:       "List discussions",
		Description: "The threads in one forum.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"forum": {"type": "string", "description": "A forum id, or a Moodle address"}
			},
			"required": ["forum"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			ref, err := args.Required("forum")
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Forums.Discussions(ctx, capabilities, ref)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.ForumDiscussions(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "forum_read",
		Title: "Read a thread",
		Description: "One discussion's posts, in reading order: the opening post first, " +
			"then oldest to newest. Reading marks nothing as read on the site.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"discussion": {"type": "string", "description": "A discussion id, or a Moodle address"}
			},
			"required": ["discussion"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			ref, err := args.Required("discussion")
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Forums.Thread(ctx, capabilities, ref)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.ForumThread(result, deps.SiteName, deps.AccountName), nil
		},
	})

	r.add(definition{
		Name:  "resolve_url",
		Title: "Read a Moodle link",
		Description: "Say what a Moodle address points at. Parsing only: no request is " +
			"made. An activity address carries the course module id, not the " +
			"activity's own id.",
		Schema: schema(`{
			"type": "object",
			"properties": {"url": {"type": "string"}},
			"required": ["url"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			raw, err := args.Required("url")
			if err != nil {
				return v1.Envelope{}, err
			}
			resource, err := site.ParseResourceURL(raw)
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.Resolve(raw, resource, ""), nil
		},
	})

	// The one tool that can change anything. It is registered only when the
	// session was started with writing allowed.
	r.add(definition{
		Name:        "assignment_submit",
		Title:       "Hand work in",
		Mutates:     true,
		Destructive: true,
		Description: "Upload files to an assignment and, when the assignment keeps " +
			"drafts, hand the work in. This cannot be undone and replaces whatever " +
			"was saved before. The final state is read back from Moodle: check " +
			"handed_in rather than assuming the steps succeeded. An assignment that " +
			"requires a submission statement needs accept_statement, which is the " +
			"account holder's decision and not yours to make for them.",
		Schema: schema(`{
			"type": "object",
			"properties": {
				"assignment": {"type": "string", "description": "An assignment id, or a Moodle address"},
				"files": {
					"type": "array", "items": {"type": "string"},
					"description": "Paths to the files to submit"
				},
				"draft_only": {
					"type": "boolean",
					"description": "Save the work without handing it in for grading"
				},
				"accept_statement": {
					"type": "boolean",
					"description": "The account holder accepts this assignment's submission statement. Only ever set this when they have said so."
				},
				"dry_run": {
					"type": "boolean",
					"description": "Describe what would be sent without sending anything"
				}
			},
			"required": ["assignment", "files"],
			"additionalProperties": false
		}`),
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			capabilities, err := deps.Capabilities(ctx)
			if err != nil {
				return v1.Envelope{}, err
			}
			ref, err := args.Required("assignment")
			if err != nil {
				return v1.Envelope{}, err
			}
			id, err := deps.Assignments.Locate(ctx, capabilities, ref)
			if err != nil {
				return v1.Envelope{}, err
			}
			result, err := deps.Assignments.Submit(ctx, capabilities, assignment.SubmitRequest{
				AssignmentID:    id,
				Files:           args.Strings("files"),
				DraftOnly:       args.Bool("draft_only"),
				AcceptStatement: args.Bool("accept_statement"),
				DryRun:          args.Bool("dry_run"),
			})
			if err != nil {
				return v1.Envelope{}, err
			}
			return v1.AssignmentSubmit(result, deps.SiteName, deps.AccountName), nil
		},
	})

	return r
}

func registerWSTools(r *Registry, deps Deps) {
	if deps.WSRegistry == nil || deps.WS == nil {
		return
	}
	r.add(definition{
		Name:        "ws_list",
		Title:       "List typed core web services",
		Description: "List the generated union of Moodle 4.5, 5.1 and 5.2 core external functions, including effects and supported versions.",
		Schema: schema(`{
			"type":"object",
			"properties":{
				"version":{"enum":["v45","v51","v52"]},
				"component":{"type":"string"},
				"effect":{"enum":["read","write"]},
				"match":{"type":"string"}
			},
			"additionalProperties":false
		}`),
		Handler: func(_ context.Context, args Arguments) (v1.Envelope, error) {
			functions := make([]wsregistry.Function, 0)
			for _, function := range deps.WSRegistry.All() {
				if version := args.String("version"); version != "" && !stringIn(function.Versions, version) {
					continue
				}
				if component := args.String("component"); component != "" && function.Component != component {
					continue
				}
				if effect := args.String("effect"); effect != "" && string(function.Effect) != effect {
					continue
				}
				if match := args.String("match"); match != "" && !strings.Contains(function.Name, match) {
					continue
				}
				functions = append(functions, function)
			}
			return v1.WSList(deps.WSRegistry, functions), nil
		},
	})

	r.add(definition{
		Name:        "ws_describe",
		Title:       "Describe a typed core web service",
		Description: "Return version-specific parameters, results, effects, capabilities, transport, service exposure, and external dependencies.",
		Schema: schema(`{
			"type":"object",
			"properties":{"function":{"type":"string"}},
			"required":["function"],
			"additionalProperties":false
		}`),
		Handler: func(_ context.Context, args Arguments) (v1.Envelope, error) {
			name, err := args.Required("function")
			if err != nil {
				return v1.Envelope{}, err
			}
			function, ok := deps.WSRegistry.Lookup(name)
			if !ok {
				return v1.Envelope{}, errs.New(errs.CodeNotFound, name+" is not in the core registry")
			}
			return v1.WSDescribe(function), nil
		},
	})

	callSchema := schema(`{
		"type":"object",
		"properties":{
			"function":{"type":"string"},
			"params":{"type":"object"},
			"dry_run":{"type":"boolean"}
		},
		"required":["function"],
		"additionalProperties":false
	}`)
	r.add(definition{
		Name:        "ws_read",
		Title:       "Call a typed read-only core web service",
		Description: "Validate and call a registry function only when the selected Moodle version classifies it as a read.",
		Schema:      callSchema,
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			return callWSTool(ctx, deps, args, wsregistry.EffectRead)
		},
	})
	r.add(definition{
		Name:        "ws_write",
		Title:       "Call a typed writing core web service",
		Description: "Validate and call a registry function only when the selected Moodle version classifies it as a write. Calls are never retried and a lost response is ambiguous.",
		Schema:      callSchema,
		Mutates:     true,
		// This generic tool includes delete, revoke, reset and other
		// destructive functions. The per-function description is authoritative;
		// the generic annotation is intentionally conservative.
		Destructive: true,
		Handler: func(ctx context.Context, args Arguments) (v1.Envelope, error) {
			return callWSTool(ctx, deps, args, wsregistry.EffectWrite)
		},
	})
}

func callWSTool(ctx context.Context, deps Deps, args Arguments, expected wsregistry.Effect) (v1.Envelope, error) {
	name, err := args.Required("function")
	if err != nil {
		return v1.Envelope{}, err
	}
	capabilities, err := deps.Capabilities(ctx)
	if err != nil {
		return v1.Envelope{}, err
	}
	variant, _, ok := deps.WSRegistry.VariantForRelease(name, capabilities.Release)
	if !ok {
		return v1.Envelope{}, errs.New(errs.CodeUnavailable,
			name+" is unavailable in the selected Moodle registry version").
			WithReason(errs.ReasonCapability)
	}
	if variant.Effect != expected {
		tool := "ws_read"
		if expected == wsregistry.EffectWrite {
			tool = "ws_write"
		}
		return v1.Envelope{}, errs.New(errs.CodeUsage,
			fmt.Sprintf("%s is classified as %s and cannot be called through %s", name, variant.Effect, tool))
	}
	result, err := deps.WS.Call(ctx, capabilities, capabilities.Release, name,
		args.Object("params"), args.Bool("dry_run"))
	if err != nil {
		return v1.Envelope{}, err
	}
	return v1.WSCall(result, deps.SiteName, deps.AccountName), nil
}

func stringIn(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}
