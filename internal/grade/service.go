package grade

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service reads grades.
type Service struct {
	backend Backend
}

// NewService builds the use case.
func NewService(backend Backend) *Service {
	return &Service{backend: backend}
}

// Course returns one course's gradebook.
func (s *Service) Course(ctx context.Context, capabilities *site.Capabilities, courseID string) (CourseResult, error) {
	if err := s.check(capabilities, "read grades"); err != nil {
		return CourseResult{}, err
	}
	return s.backend.Course(ctx, courseID)
}

// Overview returns every course's total.
func (s *Service) Overview(ctx context.Context, capabilities *site.Capabilities) (OverviewResult, error) {
	if err := s.check(capabilities, "read course totals"); err != nil {
		return OverviewResult{}, err
	}
	return s.backend.Overview(ctx)
}

func (s *Service) check(capabilities *site.Capabilities, what string) error {
	ok, why := s.backend.Requirement().SatisfiedBy(capabilities)
	if ok {
		return nil
	}
	return errs.New(errs.CodeUnavailable, "cannot "+what+" on this site").
		WithReason(errs.ReasonCapability).
		WithHint(string(s.backend.Name()) + ": " + why +
			"\nrun `moodle doctor` to see what this site offers")
}
