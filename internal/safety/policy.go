// Package safety decides what a call may do and whether it may be repeated.
//
// It exists because neither the HTTP method nor the function name tells you
// whether a Moodle call writes. Almost every web service call is a POST, and
// several functions that read like queries have effects: *_view_* records a
// view and can complete an activity, allocating a draft area is a write, and
// opening a wiki page for editing takes a lock.
package safety

import (
	"context"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Retry says whether a failed call may be sent again.
type Retry string

const (
	// RetrySafe may be repeated freely: the call has no effect to duplicate.
	RetrySafe Retry = "safe"
	// RetryAuthOnly may be repeated only after re-authenticating, because the
	// server rejected the request before running it.
	RetryAuthOnly Retry = "auth-only"
	// RetryNever must not be repeated automatically.
	RetryNever Retry = "never"
)

// Policy is what is known about one upstream function.
type Policy struct {
	// Mutates reports whether the call can change anything on the server.
	Mutates bool
	Retry   Retry
	// Session reports whether the call may go over a browser session rather
	// than a web service token.
	Session bool
	// Why records the review that produced this entry, so a later reader can
	// judge it rather than trust it.
	Why string
}

// unknownPolicy is applied to anything not in the registry.
//
// Pessimistic on purpose: an unrecognised function is assumed to write and is
// never retried. A plugin can expose anything, and guessing "probably a read"
// would be guessing with the user's data.
var unknownPolicy = Policy{
	Mutates: true,
	Retry:   RetryNever,
	Session: false,
	Why:     "not reviewed; assumed to write",
}

// registry is the reviewed list. Every entry carries its reasoning.
var registry = map[string]Policy{
	// Reads.
	"core_webservice_get_site_info": {Retry: RetrySafe, Session: true,
		Why: "reads the account's own capabilities"},
	"tool_mobile_get_public_config": {Retry: RetrySafe, Session: true,
		Why: "pre-login site configuration"},
	"core_enrol_get_users_courses": {Retry: RetrySafe, Session: true,
		Why: "lists the caller's enrolments"},
	"core_course_get_contents": {Retry: RetrySafe, Session: true,
		Why: "reads course contents"},
	"mod_assign_get_assignments": {Retry: RetrySafe, Session: true,
		Why: "reads assignment definitions"},
	"mod_assign_get_submission_status": {Retry: RetrySafe, Session: true,
		Why: "reads the caller's own submission state"},
	"gradereport_user_get_grade_items": {Retry: RetrySafe, Session: true,
		Why: "reads grades"},
	"gradereport_overview_get_course_grades": {Retry: RetrySafe, Session: true,
		Why: "reads grades"},
	"core_calendar_get_action_events_by_timesort": {Retry: RetrySafe, Session: true,
		Why: "reads calendar events"},
	"mod_forum_get_forums_by_courses": {Retry: RetrySafe, Session: true,
		Why: "reads forum definitions"},
	"core_calendar_get_calendar_monthly_view": {Retry: RetrySafe, Session: true,
		Why: "renders the calendar's month; reads events, records nothing"},
	"mod_quiz_get_quizzes_by_courses": {Retry: RetrySafe,
		Why: "reads quiz settings"},
	"mod_quiz_get_user_quiz_attempts": {Retry: RetrySafe,
		Why: "reads the caller's own attempts; starting one is a different call"},
	"mod_quiz_get_user_attempts": {Retry: RetrySafe,
		Why: "the pre-5.1 name of mod_quiz_get_user_quiz_attempts"},
	"mod_quiz_get_user_best_grade": {Retry: RetrySafe,
		Why: "reads the caller's own grade"},

	// Reads that are not reads. These are the reason this registry is a list
	// and not a naming convention.
	"core_files_get_unused_draft_itemid": {Mutates: true, Retry: RetryNever,
		Why: "allocates a draft file area, which is a write"},
	"mod_wiki_get_page_for_editing": {Mutates: true, Retry: RetryNever,
		Why: "takes an edit lock on the page"},
	"tool_mobile_get_autologin_key": {Mutates: true, Retry: RetryNever,
		Why: "issues a credential"},
	"tool_mobile_get_tokens_for_qr_login": {Mutates: true, Retry: RetryNever,
		Why: "issues a credential and consumes a single-use key"},
	"core_calendar_get_calendar_export_token": {Mutates: true, Retry: RetryNever,
		Why: "issues a credential"},

	// Writes.
	//
	// upload.php is not a web service function but an endpoint of its own. It
	// is listed because it stores files, and because a dry run has to be
	// stopped here rather than at the save that follows.
	"upload.php": {Mutates: true, Retry: RetryNever,
		Why: "stores files in the caller's draft area"},
	"mod_assign_save_submission": {Mutates: true, Retry: RetryNever,
		Why: "replaces the submission's content"},
	"mod_assign_submit_for_grading": {Mutates: true, Retry: RetryNever,
		Why: "hands the work in; the state change is what the user came for"},
	"mod_forum_add_discussion": {Mutates: true, Retry: RetryNever,
		Why: "creates a post with no idempotency key"},
	"mod_forum_add_discussion_post": {Mutates: true, Retry: RetryNever,
		Why: "creates a post with no idempotency key"},
}

// Lookup returns the policy for a function, and whether it was reviewed.
func Lookup(function string) (Policy, bool) {
	if policy, ok := registry[function]; ok {
		return policy, true
	}
	// A *_view_* function logs a view and can trip an activity completion
	// rule, so it is a write even though it reads like a query.
	if strings.Contains(function, "_view_") || strings.HasSuffix(function, "_view") {
		return Policy{
			Mutates: true, Retry: RetryNever,
			Why: "records a view, which can complete an activity",
		}, true
	}
	return unknownPolicy, false
}

// Mutates reports whether a function may change anything upstream.
func Mutates(function string) bool {
	policy, _ := Lookup(function)
	return policy.Mutates
}

// Mode restricts what a run is allowed to do.
type Mode struct {
	// DryRun forbids every call that can write. It is enforced here rather
	// than left to each feature, so "--dry-run wrote nothing" is a property
	// of the program and not of anyone's diligence.
	DryRun bool
	// ReadOnly forbids writes for the whole process, including through MCP.
	ReadOnly bool
}

// Allow reports whether a call may proceed.
func (m Mode) Allow(function string) error {
	policy, _ := Lookup(function)
	if !policy.Mutates {
		return nil
	}
	switch {
	case m.DryRun:
		return errs.New(errs.CodeUsage,
			"refusing to call "+function+" because this is a dry run").
			WithHint("re-run without --dry-run to apply the change")
	case m.ReadOnly:
		return errs.New(errs.CodePermissionDenied,
			"refusing to call "+function+" in read-only mode").
			WithHint("read-only mode is on; remove --read-only to allow writes")
	}
	return nil
}

// Guard wraps a call with the mode's restrictions.
type Guard struct {
	Mode Mode
}

// Do runs fn unless the mode forbids the call.
//
// An interrupted write is escalated to an ambiguous outcome here, because this
// is the lowest layer that knows the call was a write: the transport reports
// the interrupt, and Moodle does not undo a request because the client stopped
// listening. Marking it known-failed would let a caller repeat a submission.
func (g Guard) Do(ctx context.Context, function string, fn func(context.Context) error) error {
	if err := g.Mode.Allow(function); err != nil {
		return err
	}
	return AmbiguousIfInterrupted(fn(ctx), Mutates(function))
}

// AmbiguousIfInterrupted marks an interrupted write as an ambiguous outcome.
// It is exported for the callers that decide an effect themselves rather than
// going through a Guard, such as the typed web-service registry.
func AmbiguousIfInterrupted(err error, mutates bool) error {
	if err == nil || !mutates {
		return err
	}
	failure := errs.From(err)
	if failure.Reason != errs.ReasonInterrupted {
		return err
	}
	return failure.Ambiguous().WithHint(
		"the command was interrupted after the request was sent, so it may " +
			"already have been applied; read the current state before trying again")
}
