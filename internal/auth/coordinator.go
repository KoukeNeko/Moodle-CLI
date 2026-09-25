package auth

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Coordinator picks a login method and runs it.
//
// The order lives here rather than in the methods themselves. A method can
// only see its own situation; deciding that a token beats a password, and that
// a password beats pasting a callback by hand, needs the whole picture.
type Coordinator struct {
	manager *Manager
	methods []Method
}

// NewCoordinator builds a Coordinator. The slice order is the preference
// order: most reliable and least disruptive first.
func NewCoordinator(manager *Manager, methods ...Method) *Coordinator {
	return &Coordinator{manager: manager, methods: methods}
}

// Methods returns the registered methods in preference order.
func (c *Coordinator) Methods() []Method { return c.methods }

// Find returns a method by name.
func (c *Coordinator) Find(name string) (Method, error) {
	for _, method := range c.methods {
		if method.Name() == name {
			return method, nil
		}
	}
	return nil, errs.New(errs.CodeUsage, fmt.Sprintf("no login method named %q", name)).
		WithHint("available methods: " + strings.Join(c.names(), ", "))
}

func (c *Coordinator) names() []string {
	out := make([]string, 0, len(c.methods))
	for _, method := range c.methods {
		out = append(out, method.Name())
	}
	return out
}

// Candidate is a method paired with what a probe said about it.
type Candidate struct {
	Method Method
	Probe  ProbeResult
}

// Candidates probes every method against a site.
//
// A nil config means the site could not be asked. Every method then reports
// Unknown rather than Unavailable: an unreachable site must not look like one
// that forbids signing in.
func (c *Coordinator) Candidates(ctx context.Context, target site.Site, config *PublicConfig) []Candidate {
	out := make([]Candidate, 0, len(c.methods))
	for _, method := range c.methods {
		out = append(out, Candidate{Method: method, Probe: method.Probe(ctx, target, config)})
	}
	return out
}

// Authenticate runs one named method, or picks the best available one.
//
// Choosing automatically never includes a method that needs the user to paste
// something: silently demanding manual work is worse than saying which methods
// exist and letting them choose.
func (c *Coordinator) Authenticate(ctx context.Context, req Request, methodName string) (Credential, error) {
	config, probeErr := c.probeConfig(ctx, req.Site)
	if config != nil && config.WWWRoot != "" && req.WWWRoot == "" {
		// Callback hashes are computed over the root Moodle reports for
		// itself, which is not always the URL the user typed.
		req.WWWRoot = config.WWWRoot
	}
	if req.WWWRoot == "" && req.Site.BaseURL != nil {
		req.WWWRoot = req.Site.BaseURL.String()
	}

	if methodName != "" {
		method, err := c.Find(methodName)
		if err != nil {
			return Credential{}, err
		}
		// An explicit choice is honoured even when the probe is doubtful: the
		// user may know something the probe cannot see, and a site that
		// answers nothing would otherwise block every method.
		return method.Authenticate(ctx, req)
	}

	candidates := c.Candidates(ctx, req.Site, config)
	for _, candidate := range candidates {
		if candidate.Probe.Availability != Available {
			continue
		}
		if needsInput(candidate.Method, req) {
			continue
		}
		return candidate.Method.Authenticate(ctx, req)
	}
	return Credential{}, c.nothingToTry(candidates, probeErr)
}

// needsInput reports whether a method would have to ask the user for something
// this request does not carry.
func needsInput(method Method, req Request) bool {
	switch method.Name() {
	case "token":
		return req.Token == ""
	case "password":
		return req.Username == "" || req.Password == ""
	case "qr":
		return req.QR == ""
	case "browser-session":
		// Reading or pasting a browser credential is always explicit. The
		// coordinator must never turn profile discovery into background work.
		return req.SessionCookie == ""
	case "manual":
		// Always interactive: it is the fallback of last resort, never the
		// automatic choice.
		return true
	}
	return false
}

func (c *Coordinator) probeConfig(ctx context.Context, target site.Site) (*PublicConfig, error) {
	config, err := c.manager.Probe(target).PublicConfig(ctx)
	if err != nil {
		// Not fatal: the methods that need no configuration can still run,
		// and the failure is reported if nothing works.
		return nil, err
	}
	return config, nil
}

// nothingToTry explains why no method ran, listing what each one said.
func (c *Coordinator) nothingToTry(candidates []Candidate, probeErr error) error {
	var lines []string
	for _, candidate := range candidates {
		reason := candidate.Probe.Reason
		if reason == "" {
			reason = string(candidate.Probe.Availability)
		}
		lines = append(lines, fmt.Sprintf("%s: %s", candidate.Method.Name(), reason))
	}
	message := "no login method can run without more information"
	if probeErr != nil {
		// A site that cannot be reached is the more useful answer.
		return errs.From(probeErr)
	}
	hint := "tried — " + strings.Join(lines, "; ") +
		"\nchoose one explicitly with --method, for example `moodle auth login --method manual`"
	if runtime.GOOS == "darwin" {
		hint += "\nAlready signed in with Safari? Try `moodle auth import-browser --browser safari --store`; if its cookie is absent from disk, use `moodle auth import-session`."
	}
	return errs.New(errs.CodeUsage, message).WithHint(hint)
}
