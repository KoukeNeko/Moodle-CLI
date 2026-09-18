// Package api is the escape hatch: calling a web service function directly.
//
// It exists because no client will ever cover every function a Moodle site
// exposes — a site with plugins exposes functions this project has never heard
// of. Everything else here is a typed command with a checked contract; this is
// the door out of that, and it is deliberately narrower to walk through.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Function is one function a site exposes, with what is known about it.
type Function struct {
	Name    string
	Version string
	// Reviewed reports whether this project has looked at the function. An
	// unreviewed function is not an unknown quantity to be tried hopefully: it
	// is assumed to write and is never retried.
	Reviewed bool
	Mutates  bool
	Retry    string
	// Why records the reasoning behind a reviewed entry, so a caller can judge
	// it rather than trust it. Empty for anything unreviewed.
	Why string
}

// Caller sends one function call and hands back what the site said, unread.
type Caller interface {
	Call(ctx context.Context, function string, params map[string]any) (json.RawMessage, error)
}

// Service lists and calls functions.
type Service struct {
	caller Caller
	guard  safety.Guard
	// allowWrite records that the caller accepted responsibility for a call
	// that can change something.
	allowWrite bool
}

// NewService builds the use case.
func NewService(caller Caller, mode safety.Mode, allowWrite bool) *Service {
	return &Service{caller: caller, guard: safety.Guard{Mode: mode}, allowWrite: allowWrite}
}

// Functions lists what the site exposes, in name order.
func Functions(capabilities *site.Capabilities) []Function {
	if capabilities == nil {
		return nil
	}
	out := make([]Function, 0, len(capabilities.Functions))
	for name, info := range capabilities.Functions {
		policy, reviewed := safety.Lookup(name)
		out = append(out, Function{
			Name:     name,
			Version:  info.Version,
			Reviewed: reviewed,
			Mutates:  policy.Mutates,
			Retry:    string(policy.Retry),
			Why:      policy.Why,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Result is one call's outcome.
type Result struct {
	Function string
	// Params is what was sent, so a dry run has something to show and a real
	// call can be reproduced.
	Params map[string]any
	// Response is exactly what the site returned, unread. There is no typed
	// contract for an arbitrary function, and inventing one would be a promise
	// this project cannot keep.
	Response json.RawMessage
	// Planned reports that nothing was sent.
	Planned bool
	// NeedsAllowWrite reports that a real run of this call would be refused
	// without --allow-write. Only meaningful for a plan.
	NeedsAllowWrite bool
	Policy          Function
}

// Call sends one function call.
//
// A function this project has not reviewed is treated as a write. That is the
// whole point: a site's plugins can expose anything, and the alternative is
// guessing with someone else's coursework. So the caller has to say
// --allow-write for anything not known to be a read, which turns "I did not
// realise that wrote" into a decision someone made on purpose.
func (s *Service) Call(ctx context.Context, capabilities *site.Capabilities, function string, params map[string]any, dryRun bool) (Result, error) {
	name := strings.TrimSpace(function)
	if name == "" {
		return Result{}, errs.New(errs.CodeUsage, "no function given").
			WithHint("list them with `moodle api functions`")
	}
	if params == nil {
		params = map[string]any{}
	}

	policy, reviewed := safety.Lookup(name)
	described := Function{
		Name: name, Reviewed: reviewed, Mutates: policy.Mutates,
		Retry: string(policy.Retry), Why: policy.Why,
	}
	if capabilities != nil {
		if info, ok := capabilities.Functions[name]; ok {
			described.Version = info.Version
		} else {
			// Refusing here rather than at the site turns a generic
			// "accessexception" into something the caller can act on.
			return Result{}, errs.New(errs.CodeUnavailable,
				fmt.Sprintf("this site does not offer %s", name)).
				WithReason(errs.ReasonCapability).
				WithHint("list what it does offer with `moodle api functions`")
		}
	}

	result := Result{Function: name, Params: params, Policy: described}
	if dryRun {
		// A dry run is exempt from the write check on purpose. It sends
		// nothing, and refusing to describe the call would hide exactly what
		// someone ran it to find out — the refusal even points at --dry-run.
		result.Planned = true
		result.NeedsAllowWrite = policy.Mutates && !s.allowWrite
		return result, nil
	}
	if policy.Mutates && !s.allowWrite {
		return Result{}, errs.New(errs.CodeUsage, refusalMessage(name, reviewed)).
			WithHint("pass --allow-write if you mean to, or --dry-run to see what would be sent")
	}

	err := s.guard.Do(ctx, name, func(ctx context.Context) error {
		response, err := s.caller.Call(ctx, name, params)
		result.Response = response
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func refusalMessage(name string, reviewed bool) string {
	if reviewed {
		return fmt.Sprintf("%s can change things on the site", name)
	}
	return fmt.Sprintf("%s has not been reviewed, so it is treated as one that writes", name)
}
