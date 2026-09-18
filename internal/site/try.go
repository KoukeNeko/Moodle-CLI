package site

import (
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Attempt is one backend's offer to answer a question.
type Attempt[T any] struct {
	Kind        BackendKind
	Requirement Requirement
	Call        func() (T, error)
}

// Outcome is what trying the attempts produced.
type Outcome[T any] struct {
	Result T
	// Drift reports that an earlier backend could not read the site. The
	// answer is still good; the caller should mark it partial so nobody reads
	// it as a complete picture.
	Drift bool
	// Tried names every backend that did not answer and why, so a failure can
	// say what was attempted rather than a bare "unavailable".
	Tried []string
}

// Try runs the attempts in order and returns the first answer.
//
// The order is the caller's: a feature knows its own alternatives and its own
// tolerance for an incomplete answer, which no shared dispatcher could learn
// for every feature. What is shared here is only the mechanics — when falling
// back is allowed at all.
//
// Falling back happens only when the route itself cannot do the work. A
// permission error stops, because retrying by another route would work around
// a restriction the site meant to apply; a network error stops too, or a flaky
// connection would be reported as a missing feature.
func Try[T any](capabilities *Capabilities, attempts []Attempt[T]) (Outcome[T], error) {
	var (
		outcome   Outcome[T]
		lastError error
	)

	for _, attempt := range attempts {
		if ok, why := attempt.Requirement.SatisfiedBy(capabilities); !ok {
			outcome.Tried = append(outcome.Tried, string(attempt.Kind)+": "+why)
			continue
		}

		result, err := attempt.Call()
		if err == nil {
			outcome.Result = result
			return outcome, nil
		}

		fallback := MayFallBack(err)
		if !fallback.Allowed {
			return Outcome[T]{}, err
		}
		if fallback.Drift {
			outcome.Drift = true
		}
		lastError = err
		outcome.Tried = append(outcome.Tried,
			string(attempt.Kind)+": "+errs.From(err).Error())
	}

	return outcome, Unavailable(outcome.Tried, lastError)
}

// Unavailable explains what every route said, rather than reporting a bare
// "unavailable" that leaves the user with nowhere to look.
func Unavailable(tried []string, lastError error) error {
	err := errs.New(errs.CodeUnavailable, "no route to this data on this site").
		WithReason(errs.ReasonCapability).
		WithHint("run `moodle doctor` to see what this site offers")
	if len(tried) > 0 {
		err = err.WithHint("tried: " + join(tried) +
			"\nrun `moodle doctor` to see what this site offers")
	}
	if lastError != nil {
		if upstream := errs.From(lastError).Upstream; upstream != nil {
			err.Upstream = upstream
		}
	}
	return err
}

// Explain adds why the route that refused could not do the work.
//
// Most refusals carry no hint of their own — a permission error is its own
// explanation — and the reason is appended only when there is one. Writing the
// separator regardless leaves a sentence that trails off after a semicolon.
func Explain(err error, what string) error {
	failure := errs.From(err)
	hint := "cannot " + what + " on this site"
	if failure.Hint != "" {
		hint += "; " + failure.Hint
	}
	return failure.WithHint(hint)
}

func join(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "; "
		}
		out += item
	}
	return out
}
