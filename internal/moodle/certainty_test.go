package moodle_test

import (
	"context"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

// The tests here sit on the boundary between the transport and this project's
// own model, which is where certainty gets invented: a JSON key that never
// arrived becomes a Go zero value, and a zero value that happens to mean
// something becomes a claim the site never made.
//
// They assert on the model rather than on printed text. A command that stops
// printing a sentence still has to carry the same distinction, and a command
// that starts printing a new one inherits the guarantee for free.

func statusFrom(t *testing.T, reply map[string]any) assignmentState {
	t.Helper()
	server := testmoodle.New()
	t.Cleanup(server.Close)
	server.HandleValue(moodle.FunctionSubmissionStatus, reply)

	backend := moodle.NewAssignmentBackend(newClient(t, server), "tok")
	state, err := backend.Status(context.Background(), "7")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	return assignmentState{
		OnlineSubmission:     state.OnlineSubmission,
		MembersStillToSubmit: state.MembersStillToSubmit,
		ExtensionDue:         state.ExtensionDue != nil,
		TimerEndsAt:          state.TimerEndsAt != nil,
		Earlier:              len(state.Earlier),
	}
}

type assignmentState struct {
	OnlineSubmission     *bool
	MembersStillToSubmit *int
	ExtensionDue         bool
	TimerEndsAt          bool
	Earlier              int
}

// minimalAttempt is what mod/assign always sends. Every optional key is left
// out, which is how a real site answers an account it tells less to.
func minimalAttempt() map[string]any {
	return map[string]any{
		"lastattempt": map[string]any{
			"canedit": true, "cansubmit": false,
			"locked": false, "graded": false, "gradingstatus": "notgraded",
		},
		"warnings": []any{},
	}
}

func TestAKeyThatNeverArrivedStaysUnknown(t *testing.T) {
	state := statusFrom(t, minimalAttempt())

	if state.OnlineSubmission != nil {
		t.Errorf("submissionsenabled was absent, so whether this assignment "+
			"takes online submissions is unknown; got %v", *state.OnlineSubmission)
	}
	if state.MembersStillToSubmit != nil {
		t.Errorf("the member list was absent, so its length is unknown; got %d",
			*state.MembersStillToSubmit)
	}
	if state.ExtensionDue {
		t.Error("an extension was invented from an absent extensionduedate")
	}
	if state.TimerEndsAt {
		t.Error("a running timer was invented from an absent timelimit")
	}
	if state.Earlier != 0 {
		t.Errorf("earlier attempts were invented from an absent previousattempts: %d",
			state.Earlier)
	}
}

func TestAnEmptyListIsKnownEmptyNotUnknown(t *testing.T) {
	// The other half of the same rule. An absent key is unknown, but a key
	// that arrived carrying an empty list is the site saying "none" — and
	// flattening both to nil would throw away an answer it did give.
	reply := minimalAttempt()
	attempt, _ := reply["lastattempt"].(map[string]any)
	attempt["submissiongroupmemberswhoneedtosubmit"] = []any{}
	attempt["submissionsenabled"] = false

	state := statusFrom(t, reply)
	if state.MembersStillToSubmit == nil {
		t.Error("an empty list that did arrive was reported as unknown")
	} else if *state.MembersStillToSubmit != 0 {
		t.Errorf("count = %d, want 0", *state.MembersStillToSubmit)
	}
	if state.OnlineSubmission == nil {
		t.Fatal("a false that did arrive was reported as unknown")
	}
	if *state.OnlineSubmission {
		t.Error("the site said false and the model says true")
	}
}
