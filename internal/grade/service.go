package grade

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service reads grades.
//
// It owns which route to try and in what order. The routes differ in what they
// can answer, not only in speed: reading a page gives one course at a time and
// carries Moodle's own rendering rather than numbers.
type Service struct {
	backends []Backend
}

// NewService builds the use case. The order is the preference order.
func NewService(backends ...Backend) *Service { return &Service{backends: backends} }

// read runs one question across the routes in order.
func read[T any](s *Service, capabilities *site.Capabilities, what string,
	call func(Backend) (T, error), partial func(*T)) (T, error) {
	attempts := make([]site.Attempt[T], 0, len(s.backends))
	for _, backend := range s.backends {
		attempts = append(attempts, site.Attempt[T]{
			Kind:        backend.Name(),
			Requirement: backend.Requirement(),
			Call:        func() (T, error) { return call(backend) },
		})
	}

	outcome, err := site.Try(capabilities, attempts)
	if err != nil {
		var zero T
		return zero, errs.From(err).WithHint("cannot " + what + " on this site; " +
			errs.From(err).Hint)
	}
	if outcome.Drift {
		partial(&outcome.Result)
	}
	return outcome.Result, nil
}

// Course returns one course's gradebook.
func (s *Service) Course(ctx context.Context, capabilities *site.Capabilities, courseID string) (CourseResult, error) {
	return read(s, capabilities, "read grades",
		func(b Backend) (CourseResult, error) { return b.Course(ctx, courseID) },
		func(r *CourseResult) { r.Provenance.Partial = true })
}

// Overview returns every course's total.
func (s *Service) Overview(ctx context.Context, capabilities *site.Capabilities) (OverviewResult, error) {
	return read(s, capabilities, "read course totals",
		func(b Backend) (OverviewResult, error) { return b.Overview(ctx) },
		func(r *OverviewResult) { r.Provenance.Partial = true })
}
