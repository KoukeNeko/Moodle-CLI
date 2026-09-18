package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/mcp"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// The fakes below stand in for a site. They are deliberately thin: what is
// under test is the tool surface and how answers reach the agent, not the use
// cases, which have their own tests.

type fakeCourses struct{}

func (fakeCourses) Name() site.BackendKind        { return site.BackendWS }
func (fakeCourses) Requirement() site.Requirement { return site.Requirement{} }
func (fakeCourses) List(context.Context, course.ListQuery) (course.ListResult, error) {
	return course.ListResult{
		Courses:    []course.Summary{{ID: "2", ShortName: "CS204", FullName: "Operating Systems"}},
		Provenance: site.NewProvenance(site.BackendWS),
	}, nil
}

type fakeAssignments struct {
	// submitted records a real submission reaching the site.
	submitted int
	state     assignment.State
}

func drafts(value bool) *bool { return &value }

func (fakeAssignments) Name() site.BackendKind        { return site.BackendWS }
func (fakeAssignments) Requirement() site.Requirement { return site.Requirement{} }

func (f *fakeAssignments) summary() assignment.Summary {
	return assignment.Summary{
		ID: "7", CMID: "12", CourseID: "2", Name: "Essay 1",
		SubmissionDrafts: drafts(true), Plugins: []string{assignment.PluginFile},
	}
}

func (f *fakeAssignments) List(context.Context, []string) (assignment.ListResult, error) {
	return assignment.ListResult{
		Assignments: []assignment.Summary{f.summary()},
		Provenance:  site.NewProvenance(site.BackendWS),
	}, nil
}

func (f *fakeAssignments) Show(context.Context, string) (assignment.Detail, error) {
	return assignment.Detail{Summary: f.summary()}, nil
}

func (f *fakeAssignments) Status(context.Context, string) (assignment.State, error) {
	return f.state, nil
}

func (f *fakeAssignments) UploadDraft(context.Context, []string) (string, error) {
	f.submitted++
	return "900", nil
}
func (f *fakeAssignments) NewDraftArea(context.Context) (string, error) { return "901", nil }
func (f *fakeAssignments) SaveSubmission(context.Context, string, assignment.Content) error {
	f.submitted++
	return nil
}
func (f *fakeAssignments) SubmitForGrading(context.Context, string, bool) error {
	f.submitted++
	return nil
}

type fakeGrades struct{}

func (fakeGrades) Name() site.BackendKind        { return site.BackendWS }
func (fakeGrades) Requirement() site.Requirement { return site.Requirement{} }
func (fakeGrades) Course(context.Context, string) (grade.CourseResult, error) {
	return grade.CourseResult{CourseID: "2", Provenance: site.NewProvenance(site.BackendWS)}, nil
}
func (fakeGrades) Overview(context.Context) (grade.OverviewResult, error) {
	return grade.OverviewResult{Provenance: site.NewProvenance(site.BackendWS)}, nil
}

type fakeCalendar struct{ query calendar.Query }

func (fakeCalendar) Name() site.BackendKind        { return site.BackendWS }
func (fakeCalendar) Requirement() site.Requirement { return site.Requirement{} }
func (f *fakeCalendar) Upcoming(_ context.Context, q calendar.Query) (calendar.Result, error) {
	f.query = q
	due := time.Unix(1789000000, 0).UTC()
	return calendar.Result{
		Events: []calendar.Event{{
			ID: "3", Title: "Essay 1 is due", Activity: "Essay 1",
			Kind: "due", Module: "assign", CMID: "12", At: &due, Overdue: true,
		}},
		Provenance: site.NewProvenance(site.BackendWS),
	}, nil
}

type fakeForums struct{}

func (fakeForums) Name() site.BackendKind        { return site.BackendWS }
func (fakeForums) Requirement() site.Requirement { return site.Requirement{} }
func (fakeForums) List(context.Context, []string) (forum.ListResult, error) {
	return forum.ListResult{Provenance: site.NewProvenance(site.BackendWS)}, nil
}
func (fakeForums) Discussions(context.Context, string) (forum.DiscussionsResult, error) {
	return forum.DiscussionsResult{Provenance: site.NewProvenance(site.BackendWS)}, nil
}
func (fakeForums) Thread(context.Context, string) (forum.ThreadResult, error) {
	return forum.ThreadResult{Provenance: site.NewProvenance(site.BackendWS)}, nil
}

// build assembles a registry over the fakes.
func build(t *testing.T, allowWrite bool) (*mcp.Registry, *fakeAssignments, *fakeCalendar) {
	t.Helper()
	assignments := &fakeAssignments{
		state: assignment.State{Status: assignment.StatusNew, CanEdit: true, CanSubmit: true},
	}
	clock := &fakeCalendar{}
	deps := mcp.Deps{
		Capabilities: site.NewCapabilities(),
		SiteName:     "school",
		AccountName:  "student1",
		Courses:      course.NewService(fakeCourses{}),
		Assignments: assignment.NewService(assignments,
			safety.Mode{ReadOnly: !allowWrite}, assignments),
		Grades:   grade.NewService(fakeGrades{}),
		Calendar: calendar.NewService(clock),
		Forums:   forum.NewService(fakeForums{}),
	}
	return mcp.Register(deps, allowWrite), assignments, clock
}

func testTools(t *testing.T, allowWrite bool) *mcp.Registry {
	t.Helper()
	registry, _, _ := build(t, allowWrite)
	return registry
}

// callTool sends one tool call and returns the result object.
func callTool(t *testing.T, tools *mcp.Registry, name string, args map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	s := run(t, tools, handshake, initialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+string(encoded)+`}`)
	result, ok := s.reply(2)["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %s", s.stdout.String())
	}
	return result
}

// data digs the contract payload out of a tool result.
func data(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("no structuredContent in %v", result)
	}
	return structured
}

func TestAReadOnlySessionDoesNotOfferTheWritingTool(t *testing.T) {
	// An agent cannot decide to try a tool it cannot see. That is the point of
	// withholding it rather than refusing it at the door.
	s := run(t, testTools(t, false), handshake, initialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listed := s.reply(2)["result"].(map[string]any)["tools"].([]any)
	if len(listed) == 0 {
		t.Fatal("no tools were offered at all")
	}
	for _, item := range listed {
		tool := item.(map[string]any)
		if tool["name"] == "assignment_submit" {
			t.Error("a read-only session offered the submission tool")
		}
		annotations := tool["annotations"].(map[string]any)
		if annotations["readOnlyHint"] != true {
			t.Errorf("%v is offered in a read-only session but is not marked read-only", tool["name"])
		}
	}
}

func TestCallingAWithheldToolSaysWhyItIsMissing(t *testing.T) {
	// "No such tool" alone would send an agent looking for a typo.
	result := callTool(t, testTools(t, false), "assignment_submit", map[string]any{
		"assignment": "7", "files": []any{"/tmp/x"},
	})
	if result["isError"] != true {
		t.Fatal("calling a withheld tool was reported as a success")
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "read-only") {
		t.Errorf("the refusal does not explain itself: %q", text)
	}
}

func TestTheWritingToolIsRefusedByTheUseCaseToo(t *testing.T) {
	// Withholding it from the list is the first layer. If a tool somehow ran
	// anyway, the use case still refuses: the surface is a convenience, the
	// safety is underneath.
	registry, assignments, _ := build(t, false)
	_ = registry
	deps := mcp.Deps{
		Capabilities: site.NewCapabilities(),
		Assignments: assignment.NewService(assignments,
			safety.Mode{ReadOnly: true}, assignments),
	}
	// Registering with writing allowed while the use case stays read-only is
	// the mismatch being tested.
	tools := mcp.Register(deps, true)
	result := callTool(t, tools, "assignment_submit", map[string]any{
		"assignment": "7", "files": []any{"/tmp/x"},
	})
	if result["isError"] != true {
		t.Fatal("the use case allowed a write in a read-only session")
	}
	if assignments.submitted != 0 {
		t.Errorf("%d write(s) reached the site", assignments.submitted)
	}
}

func TestAWritingSessionOffersTheToolAndMarksItPlainly(t *testing.T) {
	s := run(t, testTools(t, true), handshake, initialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listed := s.reply(2)["result"].(map[string]any)["tools"].([]any)

	var found bool
	for _, item := range listed {
		tool := item.(map[string]any)
		if tool["name"] != "assignment_submit" {
			continue
		}
		found = true
		annotations := tool["annotations"].(map[string]any)
		if annotations["readOnlyHint"] != false {
			t.Error("the submission tool is marked read-only")
		}
		if annotations["destructiveHint"] != true {
			t.Error("submitting replaces what was saved before and is not marked destructive")
		}
		// No Moodle write carries an idempotency key, so calling twice is
		// never the same as calling once.
		if annotations["idempotentHint"] != false {
			t.Error("a Moodle write was marked idempotent")
		}
	}
	if !found {
		t.Fatal("a writing session did not offer the submission tool")
	}
}

func TestAnAgentIsToldWhetherItMayWrite(t *testing.T) {
	// The instructions change what an agent should even attempt.
	readOnly := run(t, testTools(t, false), handshake)
	instructions := readOnly.reply(1)["result"].(map[string]any)["instructions"].(string)
	if !strings.Contains(instructions, "read-only") {
		t.Errorf("a read-only session does not say so:\n%s", instructions)
	}

	writing := run(t, testTools(t, true), handshake)
	instructions = writing.reply(1)["result"].(map[string]any)["instructions"].(string)
	if !strings.Contains(instructions, "cannot be undone") {
		t.Errorf("a writing session does not warn about it:\n%s", instructions)
	}
}

func TestAnIdSentAsANumberIsAccepted(t *testing.T) {
	// Ids are strings in the contract, but an agent will send 7 as often as
	// "7", and refusing that is a pointless round trip.
	result := callTool(t, testTools(t, false), "assignment_status", map[string]any{
		"assignment": float64(7),
	})
	if result["isError"] == true {
		t.Fatalf("a numeric id was refused: %v", result)
	}
	payload := data(t, result)["data"].(map[string]any)
	if payload["assignment_id"] != "7" {
		t.Errorf("assignment_id = %v, want the string \"7\"", payload["assignment_id"])
	}
}

func TestAMissingRequiredArgumentIsAToolErrorNotACrash(t *testing.T) {
	result := callTool(t, testTools(t, false), "assignment_status", map[string]any{})
	if result["isError"] != true {
		t.Fatal("a missing argument was reported as a success")
	}
}

func TestAToolFailureCarriesTheSameErrorShapeAsTheCli(t *testing.T) {
	// An agent should see the same code, reason and hint a person would —
	// including an ambiguous outcome, which it must not retry blindly.
	result := callTool(t, testTools(t, false), "resolve_url", map[string]any{
		"url": "file:///etc/passwd",
	})
	if result["isError"] != true {
		t.Fatal("a refused URL was reported as a success")
	}
	envelope := data(t, result)
	if envelope["kind"] != "error" {
		t.Errorf("kind = %v, want the error envelope", envelope["kind"])
	}
	failure := envelope["error"].(map[string]any)
	if failure["code"] != string(errs.CodeUsage) {
		t.Errorf("code = %v, want usage", failure["code"])
	}
}

func TestAnAnswerArrivesAsBothDataAndText(t *testing.T) {
	// Structured for an agent to read, text for a model that only sees the
	// transcript. They have to be the same answer.
	result := callTool(t, testTools(t, false), "course_list", map[string]any{})
	envelope := data(t, result)
	if envelope["kind"] != "course.list" {
		t.Errorf("kind = %v", envelope["kind"])
	}

	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	var fromText map[string]any
	if err := json.Unmarshal([]byte(text), &fromText); err != nil {
		t.Fatalf("the text content is not the envelope: %v", err)
	}
	if fromText["kind"] != envelope["kind"] {
		t.Error("the text and the structured content disagree")
	}
}

func TestDaysReachesTheUseCaseAsADeadline(t *testing.T) {
	registry, _, clock := build(t, false)
	if result := callTool(t, registry, "calendar_upcoming", map[string]any{
		"days": float64(14),
	}); result["isError"] == true {
		t.Fatalf("%v", result)
	}
	if clock.query.Until == nil {
		t.Fatal("--days did not reach the use case")
	}
	// Never a start: Moodle keeps returning overdue work, and setting one
	// would quietly drop it.
	if got := time.Until(*clock.query.Until); got < 13*24*time.Hour {
		t.Errorf("the horizon is %v, want about 14 days", got)
	}
}
