// Package course is the course use cases and their model.
//
// It owns which backend to try and in what order. There is no central
// dispatcher: a feature knows its own alternatives and its own tolerance for
// incomplete answers, and a shared one would have to learn all of that for
// every feature.
package course

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Summary is one course as this tool understands it.
//
// It is not Moodle's shape: the adapter maps upstream fields onto this, so a
// change on their side stops at the adapter instead of reaching the JSON
// contract.
type Summary struct {
	ID        string
	ShortName string
	FullName  string
	// StartDate and EndDate are nil when the course does not set them. Moodle
	// sends 0 for "unset", which must not become 1970.
	StartDate *time.Time
	EndDate   *time.Time
	Visible   bool
	// Progress is nil when the site does not report it, which is different
	// from a course at 0%.
	Progress *float64
}

// ListQuery limits a listing.
type ListQuery struct {
	// Limit caps the number of courses returned. Zero means the backend's
	// default.
	Limit int
	// Cursor continues a previous listing. It is opaque to callers.
	Cursor string
}

// ListResult is a listing plus how it was obtained.
type ListResult struct {
	Courses []Summary
	// Provenance tells the caller which backend answered and whether anything
	// is missing, so an agent can tell "no value" from "could not fetch".
	Provenance site.Provenance
	NextCursor string
}

// Backend is one way of listing courses. A feature declares the interface it
// needs; adapters in other packages satisfy it.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	List(ctx context.Context, q ListQuery) (ListResult, error)
}

// Service lists courses, trying its backends in order.
type Service struct {
	backends []Backend
}

// NewService builds a Service. The order is the preference order, and the
// composition root decides it.
func NewService(backends ...Backend) *Service {
	return &Service{backends: backends}
}

// List returns the courses this account is enrolled in.
//
// Backends whose requirements are not met are skipped without a request. Once
// a request has been made, only a "this route cannot do it" error allows the
// next backend to be tried; anything else is returned as-is so a permission
// or network problem is never disguised as a missing feature.
func (s *Service) List(ctx context.Context, capabilities *site.Capabilities, q ListQuery) (ListResult, error) {
	var (
		skipped   []string
		sawDrift  bool
		lastError error
	)

	for _, backend := range s.backends {
		if ok, why := backend.Requirement().SatisfiedBy(capabilities); !ok {
			skipped = append(skipped, string(backend.Name())+": "+why)
			continue
		}

		result, err := backend.List(ctx, q)
		if err == nil {
			if sawDrift {
				// An earlier backend could not read the site. The answer is
				// good, but the caller should know something drifted.
				result.Provenance.Partial = true
			}
			return result, nil
		}

		fallback := site.MayFallBack(err)
		if !fallback.Allowed {
			return ListResult{}, err
		}
		if fallback.Drift {
			sawDrift = true
		}
		lastError = err
		skipped = append(skipped, site.Describe(backend.Name(), err))
	}

	return ListResult{}, unavailable(skipped, lastError)
}

// unavailable explains what every backend said, rather than reporting a bare
// "unavailable" that leaves the user with nowhere to look.
func unavailable(skipped []string, lastError error) error {
	message := "cannot list courses on this site"
	err := errs.New(errs.CodeUnavailable, message).
		WithReason(errs.ReasonCapability).
		WithHint("run `moodle doctor` to see what this site offers")
	if len(skipped) > 0 {
		err = err.WithHint("tried: " + joinLines(skipped) +
			"\nrun `moodle doctor` to see what this site offers")
	}
	if lastError != nil {
		if upstream := errs.From(lastError).Upstream; upstream != nil {
			err.Upstream = upstream
		}
	}
	return err
}

func joinLines(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "; "
		}
		out += item
	}
	return out
}
