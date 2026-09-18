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
type Service struct {
	backend Backend
}

// NewService builds the use case.
func NewService(backend Backend) *Service { return &Service{backend: backend} }

// List returns the forums of the given courses, or of every course when none
// are named.
func (s *Service) List(ctx context.Context, capabilities *site.Capabilities, courseIDs []string) (ListResult, error) {
	if err := s.check(capabilities); err != nil {
		return ListResult{}, err
	}
	return s.backend.List(ctx, courseIDs)
}

// Discussions returns one forum's threads. The reference may be a forum id or
// a Moodle address.
func (s *Service) Discussions(ctx context.Context, capabilities *site.Capabilities, ref string) (DiscussionsResult, error) {
	if err := s.check(capabilities); err != nil {
		return DiscussionsResult{}, err
	}
	id, err := s.locateForum(ctx, capabilities, ref)
	if err != nil {
		return DiscussionsResult{}, err
	}
	return s.backend.Discussions(ctx, id)
}

// Thread returns one discussion's posts, in reading order.
func (s *Service) Thread(ctx context.Context, capabilities *site.Capabilities, ref string) (ThreadResult, error) {
	if err := s.check(capabilities); err != nil {
		return ThreadResult{}, err
	}
	id, err := locateDiscussion(ref)
	if err != nil {
		return ThreadResult{}, err
	}
	return s.backend.Thread(ctx, id)
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

	list, err := s.backend.List(ctx, nil)
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

func (s *Service) check(capabilities *site.Capabilities) error {
	ok, why := s.backend.Requirement().SatisfiedBy(capabilities)
	if ok {
		return nil
	}
	return errs.New(errs.CodeUnavailable, "cannot read forums on this site").
		WithReason(errs.ReasonCapability).
		WithHint(string(s.backend.Name()) + ": " + why +
			"\nrun `moodle doctor` to see what this site offers")
}
