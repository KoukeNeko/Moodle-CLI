package course_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// stub is a backend a test can program.
type stub struct {
	name        site.BackendKind
	requirement site.Requirement
	result      course.ListResult
	err         error
	calls       int
}

func (s *stub) Name() site.BackendKind        { return s.name }
func (s *stub) Requirement() site.Requirement { return s.requirement }
func (s *stub) List(context.Context, course.ListQuery) (course.ListResult, error) {
	s.calls++
	return s.result, s.err
}

func capabilitiesWith(functions ...string) *site.Capabilities {
	c := site.NewCapabilities()
	c.Credential = site.CredentialWSToken
	for _, function := range functions {
		c.Functions[function] = site.FunctionInfo{Name: function}
	}
	return c
}

func TestUsesTheFirstBackendThatCan(t *testing.T) {
	first := &stub{
		name:        site.BackendWS,
		requirement: site.Requirement{AnyFunction: []string{"core_enrol_get_users_courses"}},
		result: course.ListResult{
			Courses:    []course.Summary{{ID: "2", ShortName: "CS204"}},
			Provenance: site.NewProvenance(site.BackendWS),
		},
	}
	second := &stub{name: site.BackendHTML}

	result, err := course.NewService(first, second).
		List(context.Background(), capabilitiesWith("core_enrol_get_users_courses"), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Courses) != 1 {
		t.Fatalf("got %d courses", len(result.Courses))
	}
	if second.calls != 0 {
		t.Error("the second backend was called even though the first succeeded")
	}
}

func TestSkipsABackendWhoseRequirementIsUnmetWithoutCallingIt(t *testing.T) {
	// Checking capabilities first saves a request that would only ever fail,
	// and lets the report name the missing function.
	unusable := &stub{
		name:        site.BackendWS,
		requirement: site.Requirement{AnyFunction: []string{"a_function_this_site_lacks"}},
	}
	usable := &stub{
		name: site.BackendHTML,
		result: course.ListResult{
			Courses:    []course.Summary{{ID: "2"}},
			Provenance: site.NewProvenance(site.BackendHTML),
		},
	}

	result, err := course.NewService(unusable, usable).
		List(context.Background(), capabilitiesWith("something_else"), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if unusable.calls != 0 {
		t.Error("a backend whose requirement was unmet was called anyway")
	}
	if result.Provenance.Source != site.BackendHTML {
		t.Errorf("source = %q, want html", result.Provenance.Source)
	}
}

func TestPermissionDeniedStopsInsteadOfFallingBack(t *testing.T) {
	// This is the rule that matters most: falling back to scraping after a
	// permission error would work around a restriction the site meant to
	// apply, and would hide the real reason from the user.
	denied := &stub{
		name: site.BackendWS,
		err:  errs.New(errs.CodePermissionDenied, "you may not view these courses"),
	}
	fallback := &stub{
		name:   site.BackendHTML,
		result: course.ListResult{Provenance: site.NewProvenance(site.BackendHTML)},
	}

	_, err := course.NewService(denied, fallback).
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if err == nil {
		t.Fatal("the permission error was swallowed")
	}
	if code := errs.From(err).Code; code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", code)
	}
	if fallback.calls != 0 {
		t.Error("fell back to another backend after a permission error")
	}
}

func TestNetworkFailureStopsInsteadOfFallingBack(t *testing.T) {
	// Otherwise a flaky connection would be reported as a missing feature,
	// sending the user to look for a capability problem that is not there.
	down := &stub{name: site.BackendWS, err: errs.New(errs.CodeNetwork, "connection refused")}
	fallback := &stub{name: site.BackendHTML}

	_, err := course.NewService(down, fallback).
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if code := errs.From(err).Code; code != errs.CodeNetwork {
		t.Errorf("code = %q, want network", code)
	}
	if fallback.calls != 0 {
		t.Error("fell back after a network error")
	}
}

func TestAmbiguousOutcomeStopsInsteadOfFallingBack(t *testing.T) {
	// Retrying through another route could repeat an effect that may already
	// have happened.
	ambiguous := &stub{
		name: site.BackendWS,
		err:  errs.New(errs.CodeNetwork, "the response was lost").Ambiguous(),
	}
	fallback := &stub{name: site.BackendHTML}

	_, err := course.NewService(ambiguous, fallback).
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if errs.From(err).EffectiveOutcome() != errs.OutcomeAmbiguous {
		t.Error("the ambiguous outcome was lost")
	}
	if fallback.calls != 0 {
		t.Error("fell back after an ambiguous outcome")
	}
}

func TestFallsBackWhenTheRouteItselfCannotDoIt(t *testing.T) {
	unavailable := &stub{
		name: site.BackendWS,
		err: errs.New(errs.CodeUnavailable, "this site does not expose that function").
			WithReason(errs.ReasonCapability),
	}
	fallback := &stub{
		name: site.BackendHTML,
		result: course.ListResult{
			Courses:    []course.Summary{{ID: "2"}},
			Provenance: site.NewProvenance(site.BackendHTML),
		},
	}

	result, err := course.NewService(unavailable, fallback).
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if fallback.calls != 1 {
		t.Error("did not fall back when the first route could not do it")
	}
	if result.Provenance.Source != site.BackendHTML {
		t.Errorf("source = %q, want html", result.Provenance.Source)
	}
}

func TestProtocolDriftMarksTheResultPartial(t *testing.T) {
	// A later backend answering does not mean everything is fine: something
	// on the site changed shape, and the caller should be able to see that.
	drifted := &stub{
		name: site.BackendWS,
		err: errs.New(errs.CodeUpstream, "cannot read the response").
			WithReason(errs.ReasonProtocolDrift),
	}
	fallback := &stub{
		name: site.BackendHTML,
		result: course.ListResult{
			Courses:    []course.Summary{{ID: "2"}},
			Provenance: site.NewProvenance(site.BackendHTML),
		},
	}

	result, err := course.NewService(drifted, fallback).
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Provenance.Partial {
		t.Error("drift on an earlier backend was not reflected in the result")
	}
}

func TestNoUsableBackendNamesWhatWasTried(t *testing.T) {
	// "unavailable" on its own leaves the user with nowhere to look.
	first := &stub{
		name:        site.BackendWS,
		requirement: site.Requirement{AnyFunction: []string{"missing_function"}},
	}
	_, err := course.NewService(first).
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if err == nil {
		t.Fatal("want an error")
	}
	e := errs.From(err)
	if e.Code != errs.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", e.Code)
	}
	if !strings.Contains(e.Hint, "missing_function") {
		t.Errorf("hint should name the missing function, got %q", e.Hint)
	}
}

func TestEmptyServiceIsNotASilentSuccess(t *testing.T) {
	_, err := course.NewService().
		List(context.Background(), capabilitiesWith(), course.ListQuery{})
	if err == nil {
		t.Fatal("a service with no backends reported success")
	}
	if !errors.As(err, new(*errs.Error)) {
		t.Errorf("got %T, want *errs.Error", err)
	}
}
