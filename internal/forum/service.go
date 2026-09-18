package forum

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service reads forums.
//
// It owns which route to try and in what order. Only reading a thread works
// over a browser session — listing forums and their discussions is not exposed
// there — so the routes differ in what they can answer, not only in speed.
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
		return zero, errs.From(err).WithHint("cannot " + what + " on this site; " +
			errs.From(err).Hint)
	}
	if outcome.Drift {
		partial(&outcome.Result)
	}
	return outcome.Result, nil
}

// List returns the forums of the given courses, or of every course when none
// are named.
func (s *Service) List(ctx context.Context, capabilities *site.Capabilities, courseIDs []string) (ListResult, error) {
	return try(s, capabilities, "list forums",
		func(b Backend) (ListResult, error) { return b.List(ctx, courseIDs) },
		func(r *ListResult) { r.Provenance.Partial = true })
}

// Discussions returns one forum's threads. The reference may be a forum id or
// a Moodle address.
func (s *Service) Discussions(ctx context.Context, capabilities *site.Capabilities, ref string) (DiscussionsResult, error) {
	id, err := s.locateForum(ctx, capabilities, ref)
	if err != nil {
		return DiscussionsResult{}, err
	}
	return try(s, capabilities, "list discussions",
		func(b Backend) (DiscussionsResult, error) { return b.Discussions(ctx, id) },
		func(r *DiscussionsResult) { r.Provenance.Partial = true })
}

// Thread returns one discussion's posts, in reading order.
func (s *Service) Thread(ctx context.Context, capabilities *site.Capabilities, ref string) (ThreadResult, error) {
	id, err := locateDiscussion(ref)
	if err != nil {
		return ThreadResult{}, err
	}
	return try(s, capabilities, "read the discussion",
		func(b Backend) (ThreadResult, error) { return b.Thread(ctx, id) },
		func(r *ThreadResult) { r.Provenance.Partial = true })
}

// locateForum turns what the caller typed into a forum id.
//
// A forum's address carries the course module id, not the forum's own id, so
// getting from one to the other takes a lookup — the same trap as assignments.
func (s *Service) locateForum(ctx context.Context, capabilities *site.Capabilities, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errs.New(errs.CodeUsage, "no forum given").
			WithHint("list them with `moodle forum list`")
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return trimmed, nil
	}

	resource, err := site.ParseResourceURL(trimmed)
	if err != nil {
		return "", errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is neither a forum id nor a Moodle address", ref))
	}
	if resource.Kind != site.ResourceActivity || resource.Module != "forum" {
		return "", errs.New(errs.CodeUsage,
			"that address does not point at a forum").
			WithHint("for a single thread use `moodle forum read` with its address")
	}

	list, err := s.List(ctx, capabilities, nil)
	if err != nil {
		return "", err
	}
	for _, item := range list.Forums {
		if item.CMID == resource.CMID {
			return item.ID, nil
		}
	}
	return "", errs.New(errs.CodeNotFound, "that forum is not one you can see").
		WithHint("check you are signed in to the right site, with `moodle auth status`")
}

// locateDiscussion turns what the caller typed into a discussion id. A
// discussion's address carries the id directly, so no lookup is needed.
func locateDiscussion(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errs.New(errs.CodeUsage, "no discussion given")
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return trimmed, nil
	}

	resource, err := site.ParseResourceURL(trimmed)
	if err != nil {
		return "", errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is neither a discussion id nor a Moodle address", ref))
	}
	if resource.Kind != site.ResourceDiscussion || resource.DiscussionID == "" {
		return "", errs.New(errs.CodeUsage,
			"that address does not point at a discussion").
			WithHint("copy the address of the thread itself, which looks like .../mod/forum/discuss.php?d=…")
	}
	return resource.DiscussionID, nil
}
