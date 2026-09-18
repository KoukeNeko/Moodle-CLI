package calendar

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service reads the calendar.
type Service struct {
	backend Backend
}

// NewService builds the use case.
func NewService(backend Backend) *Service { return &Service{backend: backend} }

// Upcoming returns what the caller still has to do.
func (s *Service) Upcoming(ctx context.Context, capabilities *site.Capabilities, q Query) (Result, error) {
	ok, why := s.backend.Requirement().SatisfiedBy(capabilities)
	if !ok {
		return Result{}, errs.New(errs.CodeUnavailable,
			"cannot read the calendar on this site").
			WithReason(errs.ReasonCapability).
			WithHint(string(s.backend.Name()) + ": " + why +
				"\nrun `moodle doctor` to see what this site offers")
	}
	return s.backend.Upcoming(ctx, q)
}
