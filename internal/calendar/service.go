package calendar

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service reads the calendar.
//
// It owns which route to try and in what order. There is no central
// dispatcher: a feature knows its own alternatives, and a shared one would
// have to learn them for every feature.
type Service struct {
	backends []Backend
}

// NewService builds the use case. The order is the preference order.
func NewService(backends ...Backend) *Service { return &Service{backends: backends} }

// Upcoming returns what the caller still has to do.
func (s *Service) Upcoming(ctx context.Context, capabilities *site.Capabilities, q Query) (Result, error) {
	attempts := make([]site.Attempt[Result], 0, len(s.backends))
	for _, backend := range s.backends {
		attempts = append(attempts, site.Attempt[Result]{
			Kind:        backend.Name(),
			Requirement: backend.Requirement(),
			Call:        func() (Result, error) { return backend.Upcoming(ctx, q) },
		})
	}

	outcome, err := site.Try(capabilities, attempts)
	if err != nil {
		return Result{}, site.Explain(err, "read the calendar")
	}
	if outcome.Drift {
		// An earlier route could not read the site. The answer is good, but
		// the caller should know something drifted.
		outcome.Result.Provenance.Partial = true
	}
	return outcome.Result, nil
}
