package site

import "github.com/KoukeNeko/moodle-cli/internal/errs"

// Fallback says whether a feature may try its next backend after an error.
//
// The rule is narrow on purpose: only "this route cannot do it"
// allows another attempt. A permission error must never be retried through
// HTML scraping — that would quietly work around a restriction the site meant
// to apply — and a network failure must not be reported as "unsupported",
// which would send the user hunting for a capability problem that is not there.
type Fallback struct {
	// Allowed reports whether the next backend may be tried.
	Allowed bool
	// Drift reports that the backend understood neither the response nor the
	// page, so the final result should say so even if a later backend works.
	Drift bool
}

// MayFallBack classifies an error from a backend attempt.
func MayFallBack(err error) Fallback {
	if err == nil {
		return Fallback{}
	}
	e := errs.From(err)

	// An ambiguous outcome means the request may already have taken effect.
	// Trying another route could repeat it.
	if e.EffectiveOutcome() == errs.OutcomeAmbiguous {
		return Fallback{}
	}

	switch e.Reason {
	case errs.ReasonProtocolDrift:
		return Fallback{Allowed: true, Drift: true}
	case errs.ReasonCapability, errs.ReasonMobileServicesDisabled:
		return Fallback{Allowed: true}
	}

	// A function the service does not expose looks like this when it is not
	// in the capability list but is called anyway.
	if e.Code == errs.CodeUnavailable {
		return Fallback{Allowed: true}
	}
	return Fallback{}
}
