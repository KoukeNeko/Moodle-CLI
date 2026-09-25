package quiz

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service reads quizzes. It owns which route to try and in what order.
type Service struct {
	backends []Backend
}

// NewService builds the use case. The order is the preference order.
func NewService(backends ...Backend) *Service { return &Service{backends: backends} }

// try runs one question across the routes in order.
func try[T any](s *Service, capabilities *site.Capabilities, what string,
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
		return zero, site.Explain(err, what)
	}
	if outcome.Drift {
		partial(&outcome.Result)
	}
	return outcome.Result, nil
}

// List returns the quizzes of the given courses, or of every course when none
// are named.
func (s *Service) List(ctx context.Context, capabilities *site.Capabilities, courseIDs []string) (ListResult, error) {
	return try(s, capabilities, "list quizzes",
		func(b Backend) (ListResult, error) { return b.List(ctx, courseIDs) },
		func(r *ListResult) { r.Provenance.Partial = true })
}

// Show returns one quiz and the caller's attempts. The reference may be an id
// as List reports it, or the quiz's address.
func (s *Service) Show(ctx context.Context, capabilities *site.Capabilities, ref string) (Detail, error) {
	id, err := s.locate(ctx, capabilities, ref)
	if err != nil {
		return Detail{}, err
	}
	return try(s, capabilities, "read the quiz",
		func(b Backend) (Detail, error) { return b.Show(ctx, id) },
		func(r *Detail) { r.Provenance.Partial = true })
}

// locate turns what the caller typed into a quiz id.
//
// A quiz's address carries the course module id, not the quiz's own id, so
// getting from one to the other takes a lookup — the same trap as
// assignments and forums, with the same two small integers.
func (s *Service) locate(ctx context.Context, capabilities *site.Capabilities, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errs.New(errs.CodeUsage, "no quiz given").
			WithHint("list them with `moodle quiz list`")
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return trimmed, nil
	}

	resource, err := site.ParseResourceURL(trimmed)
	if err != nil {
		return "", errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is neither a quiz id nor a Moodle address", ref))
	}
	if resource.Kind != site.ResourceActivity || resource.Module != "quiz" {
		return "", errs.New(errs.CodeUsage, "that address does not point at a quiz").
			WithHint("copy the address of the quiz itself, which looks like .../mod/quiz/view.php?id=…")
	}

	list, err := s.List(ctx, capabilities, nil)
	if err != nil {
		return "", err
	}
	for _, item := range list.Quizzes {
		if item.CMID == resource.CMID {
			return item.ID, nil
		}
	}
	return "", errs.New(errs.CodeNotFound, "that quiz is not one you can see").
		WithHint("check you are signed in to the right site, with `moodle auth status`")
}
