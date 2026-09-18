package site_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func attempt(kind site.BackendKind, req site.Requirement, answer string, err error) site.Attempt[string] {
	return site.Attempt[string]{
		Kind: kind, Requirement: req,
		Call: func() (string, error) { return answer, err },
	}
}

func TestTheFirstRouteThatCanAnswerWins(t *testing.T) {
	capabilities := site.NewCapabilities()
	capabilities.Credential = site.CredentialWSToken
	capabilities.Functions["f"] = site.FunctionInfo{Name: "f"}

	outcome, err := site.Try(capabilities, []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{AnyFunction: []string{"f"}}, "ws", nil),
		attempt(site.BackendAJAX, site.Requirement{}, "ajax", nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result != "ws" {
		t.Errorf("answer = %q, want the first route", outcome.Result)
	}
}

func TestARouteThatCannotRunIsSkippedWithItsReason(t *testing.T) {
	// A session-only account has no token, so the web service route is not
	// tried at all — and the failure has to say that rather than report a
	// missing feature.
	capabilities := site.NewCapabilities()
	capabilities.Credential = site.CredentialBrowserSession

	outcome, err := site.Try(capabilities, []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{Credential: site.CredentialWSToken}, "ws", nil),
		attempt(site.BackendAJAX, site.Requirement{}, "ajax", nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result != "ajax" {
		t.Errorf("answer = %q, want the session route", outcome.Result)
	}
	if len(outcome.Tried) != 1 || !strings.Contains(outcome.Tried[0], "ws_token") {
		t.Errorf("tried = %v, want the web service route's reason", outcome.Tried)
	}
}

func TestAPermissionFailureStopsRatherThanFallingBack(t *testing.T) {
	// Trying another route would work around a restriction the site meant to
	// apply.
	denied := errs.New(errs.CodePermissionDenied, "not yours to read")
	_, err := site.Try(site.NewCapabilities(), []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{}, "", denied),
		attempt(site.BackendAJAX, site.Requirement{}, "ajax", nil),
	})
	if err == nil {
		t.Fatal("a permission failure fell through to another route")
	}
	if code := errs.From(err).Code; code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want the original refusal", code)
	}
}

func TestAnAmbiguousOutcomeNeverFallsBack(t *testing.T) {
	// The request may already have taken effect; another route could repeat it.
	lost := errs.New(errs.CodeNetwork, "the reply was lost").Ambiguous()
	_, err := site.Try(site.NewCapabilities(), []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{}, "", lost),
		attempt(site.BackendAJAX, site.Requirement{}, "ajax", nil),
	})
	if err == nil {
		t.Fatal("an ambiguous outcome fell through to another route")
	}
	if errs.From(err).EffectiveOutcome() != errs.OutcomeAmbiguous {
		t.Error("the ambiguity was lost on the way out")
	}
}

func TestAMissingCapabilityFallsBackAndTheAnswerIsMarkedPartial(t *testing.T) {
	drift := errs.New(errs.CodeUpstream, "cannot read the reply").
		WithReason(errs.ReasonProtocolDrift)
	outcome, err := site.Try(site.NewCapabilities(), []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{}, "", drift),
		attempt(site.BackendAJAX, site.Requirement{}, "ajax", nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result != "ajax" {
		t.Errorf("answer = %q", outcome.Result)
	}
	if !outcome.Drift {
		t.Error("an earlier route could not read the site and the answer is not marked")
	}
}

func TestEveryRouteFailingSaysWhatEachOneSaid(t *testing.T) {
	_, err := site.Try(site.NewCapabilities(), []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{AnyFunction: []string{"missing"}}, "", nil),
		attempt(site.BackendAJAX, site.Requirement{}, "",
			errs.New(errs.CodeUnavailable, "not offered here").WithReason(errs.ReasonCapability)),
	})
	if err == nil {
		t.Fatal("no route answered and no error was reported")
	}
	hint := errs.From(err).Hint
	if !strings.Contains(hint, "ws:") || !strings.Contains(hint, "ajax:") {
		t.Errorf("the failure does not say what each route said: %q", hint)
	}
}

func TestAnUnclassifiedErrorStops(t *testing.T) {
	// A plain error is not evidence that another route would do better.
	_, err := site.Try(site.NewCapabilities(), []site.Attempt[string]{
		attempt(site.BackendWS, site.Requirement{}, "", errors.New("boom")),
		attempt(site.BackendAJAX, site.Requirement{}, "ajax", nil),
	})
	if err == nil {
		t.Fatal("an unclassified error fell through to another route")
	}
}
