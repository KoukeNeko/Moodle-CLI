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
	// backends are the read routes in preference order.
	backends []Backend
	// submitting is the route the submission flow uses, and it is deliberately
	// not the list above.
	//
	// Handing work in depends on settings only the web service route can read:
	// whether saving is enough, or whether a separate hand-in is required. A
	// route that cannot see those would report a draft as handed in, which is
	// the mistake this package exists to avoid. It refuses on its own when
	// there is no token, which is the right answer — better to say submission
	// is unavailable than to submit and misreport the result.
	submitting Backend
	writer     Writer
	mode       safety.Mode
}

// NewService builds the use case. The first backend is the one trusted to
// submit; the rest are read-only alternatives.
func NewService(writer Writer, mode safety.Mode, backends ...Backend) *Service {
	service := &Service{backends: backends, writer: writer, mode: mode}
	if len(backends) > 0 {
		service.submitting = backends[0]
	}
	return service
}

// read runs one question across the read routes in order.
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
		return zero, site.Explain(err, what)
	}
	if outcome.Drift {
		partial(&outcome.Result)
	}
	return outcome.Result, nil
}

// List returns the assignments of the given courses, or of every course the
// account is enrolled in when none are named.
func (s *Service) List(ctx context.Context, capabilities *site.Capabilities, courseIDs []string) (ListResult, error) {
	return read(s, capabilities, "list assignments",
		func(b Backend) (ListResult, error) { return b.List(ctx, courseIDs) },
		func(r *ListResult) { r.Provenance.Partial = true })
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
	return read(s, capabilities, "read assignments",
		func(b Backend) (Detail, error) { return b.Show(ctx, assignmentID) },
		func(*Detail) {})
}

// Status reports where one submission stands.
func (s *Service) Status(ctx context.Context, capabilities *site.Capabilities, assignmentID string) (State, error) {
	return read(s, capabilities, "read submission status",
		func(b Backend) (State, error) { return b.Status(ctx, assignmentID) },
		func(r *State) { r.Provenance.Partial = true })
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
	if err := s.checkSubmitting(capabilities); err != nil {
		return SubmitResult{}, err
	}
	if !capabilities.CanUpload && !req.DryRun {
		// Moodle reports this itself, and refusing here saves the student from
		// discovering it after the files have been read.
		return SubmitResult{}, errs.New(errs.CodePermissionDenied,
			"this site does not allow file uploads for your account").
			WithReason(errs.ReasonCapability)
	}
	return NewSubmitter(s.submitting, s.writer, s.mode).Submit(ctx, req)
}

// checkSubmitting refuses when the route that can submit is unavailable.
//
// There is deliberately no fallback here. Another route might read the page
// well enough to look like it worked, but it cannot see whether saving counts
// as handing in — and reporting a draft as submitted is worse than refusing.
func (s *Service) checkSubmitting(capabilities *site.Capabilities) error {
	if s.submitting == nil {
		return errs.New(errs.CodeUnavailable, "cannot submit assignments on this site").
			WithReason(errs.ReasonCapability)
	}
	ok, why := s.submitting.Requirement().SatisfiedBy(capabilities)
	if ok {
		return nil
	}
	return errs.New(errs.CodeUnavailable, "cannot submit assignments on this site").
		WithReason(errs.ReasonCapability).
		WithHint(string(s.submitting.Name()) + ": " + why +
			"\nsubmitting needs a web service token; run `moodle doctor` to see what this site offers")
}
