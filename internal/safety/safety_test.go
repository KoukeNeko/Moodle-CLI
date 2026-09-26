package safety_test

import (
	"context"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

func TestUnreviewedFunctionIsAssumedToWrite(t *testing.T) {
	// A plugin can expose anything. Guessing "probably a read" would be
	// guessing with the user's data.
	policy, reviewed := safety.Lookup("local_something_nobody_has_seen")
	if reviewed {
		t.Fatal("an invented function was reported as reviewed")
	}
	if !policy.Mutates {
		t.Error("an unreviewed function must be assumed to write")
	}
	if policy.Retry != safety.RetryNever {
		t.Errorf("retry = %q, want never", policy.Retry)
	}
	if policy.Session {
		t.Error("an unreviewed function must not be allowed over a browser session")
	}
}

func TestReadsThatAreNotReads(t *testing.T) {
	// The whole reason the registry is a list and not a naming convention.
	for _, function := range []string{
		"core_files_get_unused_draft_itemid",
		"mod_wiki_get_page_for_editing",
		"tool_mobile_get_autologin_key",
		"tool_mobile_get_tokens_for_qr_login",
		"core_calendar_get_calendar_export_token",
	} {
		if !safety.Mutates(function) {
			t.Errorf("%s reads like a query but has an effect; it must be a write", function)
		}
	}
}

func TestViewFunctionsAreWrites(t *testing.T) {
	// Recording a view can complete an activity, which changes a student's
	// progress.
	for _, function := range []string{
		"mod_assign_view_assign",
		"core_course_view_course",
		"mod_forum_view_forum",
	} {
		if !safety.Mutates(function) {
			t.Errorf("%s records a view and must count as a write", function)
		}
	}
}

func TestPlainReadsAreRetryable(t *testing.T) {
	policy, reviewed := safety.Lookup("core_enrol_get_users_courses")
	if !reviewed {
		t.Fatal("a listed function was not found")
	}
	if policy.Mutates {
		t.Error("listing courses must not count as a write")
	}
	if policy.Retry != safety.RetrySafe {
		t.Errorf("retry = %q, want safe", policy.Retry)
	}
}

func TestEveryRegisteredWriteIsNeverRetried(t *testing.T) {
	// A write with no idempotency key cannot be repeated safely, and none of
	// Moodle's have one.
	for _, function := range []string{
		"mod_assign_save_submission",
		"mod_assign_submit_for_grading",
		"mod_forum_add_discussion",
	} {
		policy, _ := safety.Lookup(function)
		if policy.Retry != safety.RetryNever {
			t.Errorf("%s: retry = %q, want never", function, policy.Retry)
		}
		if policy.Session {
			t.Errorf("%s: writes over a browser session are not enabled yet", function)
		}
	}
}

func TestEveryEntryExplainsItself(t *testing.T) {
	// An entry without a reason cannot be reviewed later; it can only be
	// trusted, which is what this registry exists to avoid.
	for _, function := range []string{
		"core_webservice_get_site_info",
		"mod_assign_save_submission",
		"core_files_get_unused_draft_itemid",
	} {
		policy, _ := safety.Lookup(function)
		if strings.TrimSpace(policy.Why) == "" {
			t.Errorf("%s has no recorded reasoning", function)
		}
	}
}

func TestDryRunRefusesEveryWrite(t *testing.T) {
	// Enforced centrally so that "--dry-run wrote nothing" is a property of
	// the program, not of each feature's diligence.
	mode := safety.Mode{DryRun: true}
	if err := mode.Allow("mod_assign_save_submission"); err == nil {
		t.Fatal("a dry run allowed a write")
	}
	// Including the ones that do not look like writes.
	if err := mode.Allow("core_files_get_unused_draft_itemid"); err == nil {
		t.Error("a dry run allowed a draft allocation")
	}
	if err := mode.Allow("mod_assign_view_assign"); err == nil {
		t.Error("a dry run allowed a view, which can complete an activity")
	}
	// Reads stay allowed: a dry run still has to inspect the current state.
	if err := mode.Allow("mod_assign_get_submission_status"); err != nil {
		t.Errorf("a dry run blocked a read: %v", err)
	}
}

func TestReadOnlyModeIsPermissionDenied(t *testing.T) {
	// The distinction matters to a caller: a dry run is the user's choice for
	// this invocation, read-only is a standing restriction.
	mode := safety.Mode{ReadOnly: true}
	err := mode.Allow("mod_assign_save_submission")
	if err == nil {
		t.Fatal("read-only mode allowed a write")
	}
	if code := errs.From(err).Code; code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", code)
	}
}

func TestGuardBlocksTheCallItself(t *testing.T) {
	// Not just the decision: the function must never run.
	called := false
	guard := safety.Guard{Mode: safety.Mode{DryRun: true}}
	err := guard.Do(context.Background(), "mod_assign_save_submission",
		func(context.Context) error {
			called = true
			return nil
		})
	if err == nil {
		t.Fatal("the guard allowed a write during a dry run")
	}
	if called {
		t.Fatal("the guarded call ran anyway")
	}
}

func TestGuardAllowsReads(t *testing.T) {
	called := false
	guard := safety.Guard{Mode: safety.Mode{DryRun: true}}
	err := guard.Do(context.Background(), "core_enrol_get_users_courses",
		func(context.Context) error {
			called = true
			return nil
		})
	if err != nil {
		t.Fatalf("the guard blocked a read: %v", err)
	}
	if !called {
		t.Fatal("the read did not run")
	}
}

func TestAnInterruptedWriteBecomesAmbiguous(t *testing.T) {
	// Moodle does not undo a write because the client stopped listening, so
	// an interrupt after the request was sent leaves the outcome unknown.
	// Reporting it as a clean failure would let a caller submit twice.
	interrupted := errs.New(errs.CodeNetwork, "interrupted before it finished").
		WithReason(errs.ReasonInterrupted)

	guard := safety.Guard{}
	err := guard.Do(context.Background(), "mod_assign_save_submission",
		func(context.Context) error { return interrupted })
	if got := errs.From(err); got.EffectiveOutcome() != errs.OutcomeAmbiguous {
		t.Errorf("outcome = %v, want ambiguous", got.EffectiveOutcome())
	} else if got.Hint == "" {
		t.Error("an ambiguous interrupt should say to read the state back")
	}

	// A read changed nothing, so the interrupt stays what it was.
	err = guard.Do(context.Background(), "core_enrol_get_users_courses",
		func(context.Context) error { return interrupted })
	if got := errs.From(err); got.EffectiveOutcome() == errs.OutcomeAmbiguous {
		t.Error("an interrupted read was reported as possibly applied")
	}

	// An unreviewed function is assumed to write, here as everywhere else.
	err = guard.Do(context.Background(), "local_unknown_thing",
		func(context.Context) error { return interrupted })
	if got := errs.From(err); got.EffectiveOutcome() != errs.OutcomeAmbiguous {
		t.Errorf("unreviewed: outcome = %v, want ambiguous", got.EffectiveOutcome())
	}
}
