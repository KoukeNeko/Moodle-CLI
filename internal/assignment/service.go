package assignment

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service is the assignment use case.
//
// There is one backend today. It still owns the choice rather than the command
// layer, because the AJAX and HTML routes arrive later and the decision of
// which to use belongs to the feature, not to the presentation.
type Service struct {
	backend Backend
	writer  Writer
	mode    safety.Mode
}

// NewService builds the use case.
func NewService(backend Backend, writer Writer, mode safety.Mode) *Service {
	return &Service{backend: backend, writer: writer, mode: mode}
}

// List returns the assignments of the given courses, or of every course the
// account is enrolled in when none are named.
func (s *Service) List(ctx context.Context, capabilities *site.Capabilities, courseIDs []string) (ListResult, error) {
	if err := s.check(capabilities, "list assignments"); err != nil {
		return ListResult{}, err
	}
	return s.backend.List(ctx, courseIDs)
}

// Locate turns what the caller typed into an assignment id.
//
// A bare number is already one. Anything else is read as a Moodle address,
// which is what someone has in their clipboard after looking at the assignment
// in a browser. Those addresses carry the course module id rather than the
// assignment's own id, so getting from one to the other takes a lookup — the
// two are both small integers and are not interchangeable.
func (s *Service) Locate(ctx context.Context, capabilities *site.Capabilities, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errs.New(errs.CodeUsage, "no assignment given")
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return trimmed, nil
	}

	resource, err := site.ParseResourceURL(trimmed)
	if err != nil {
		return "", errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is neither an assignment id nor a Moodle address", ref))
	}
	if resource.Kind != site.ResourceActivity || resource.Module != "assign" {
		return "", errs.New(errs.CodeUsage,
			fmt.Sprintf("that address points at %s, not an assignment", describeResource(resource))).
			WithHint("copy the address of the assignment itself")
	}

	list, err := s.List(ctx, capabilities, nil)
	if err != nil {
		return "", err
	}
	for _, item := range list.Assignments {
		if item.CMID == resource.CMID {
			return item.ID, nil
		}
	}
	return "", errs.New(errs.CodeNotFound,
		"that assignment is not one you are enrolled in").
		WithHint("check you are signed in to the right site, with `moodle auth status`")
}

// describeResource names a resource for an error message.
func describeResource(resource site.Resource) string {
	switch resource.Kind {
	case site.ResourceActivity:
		return "a " + resource.Module + " activity"
	case site.ResourceUnknown:
		return "a part of Moodle this build does not recognise"
	default:
		return "a " + string(resource.Kind)
	}
}

// Show returns one assignment's full definition.
func (s *Service) Show(ctx context.Context, capabilities *site.Capabilities, assignmentID string) (Detail, error) {
	if err := s.check(capabilities, "read assignments"); err != nil {
		return Detail{}, err
	}
	return s.backend.Show(ctx, assignmentID)
}

// Status reports where one submission stands.
func (s *Service) Status(ctx context.Context, capabilities *site.Capabilities, assignmentID string) (State, error) {
	if err := s.check(capabilities, "read submission status"); err != nil {
		return State{}, err
	}
	return s.backend.Status(ctx, assignmentID)
}

// Submit hands work in.
func (s *Service) Submit(ctx context.Context, capabilities *site.Capabilities, req SubmitRequest) (SubmitResult, error) {
	if s.mode.ReadOnly {
		// Checked here rather than left to the guard further in, because a dry
		// run returns before reaching it. Read-only is a standing restriction
		// on what this process may do at all, so it is not something an
		// individual request gets to argue with.
		return SubmitResult{}, errs.New(errs.CodePermissionDenied,
			"refusing to submit in read-only mode").
			WithHint("read-only mode is on; remove --read-only to allow writes")
	}
	if err := s.check(capabilities, "submit assignments"); err != nil {
		return SubmitResult{}, err
	}
	if !capabilities.CanUpload && !req.DryRun {
		// Moodle reports this itself, and refusing here saves the student from
		// discovering it after the files have been read.
		return SubmitResult{}, errs.New(errs.CodePermissionDenied,
			"this site does not allow file uploads for your account").
			WithReason(errs.ReasonCapability)
	}
	return NewSubmitter(s.backend, s.writer, s.mode).Submit(ctx, req)
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
