package cli

import (
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Deps are the collaborators the composition root injects. Commands reach for
// these instead of constructing their own, so tests can swap any of them.
//
// There is deliberately no Moodle client here: the command layer must not be
// able to speak the protocol itself.
type Deps struct {
	ConfigPath string
	Auth       *auth.Manager
	// Login decides which sign-in method to use. The order lives in the
	// coordinator, not in the methods.
	Login *auth.Coordinator
	// Courses assembles the course use case for a session. The composition
	// root supplies it because deciding which backends exist, and in which
	// order, is a wiring decision — and because building them means naming
	// the transport, which this layer may not do.
	Courses func(*auth.Session, *site.Capabilities) *course.Service
	// Assignments assembles the assignment use case. The safety mode is passed
	// in per invocation because --dry-run is a property of the command line,
	// and the guard that enforces it has to be built with it.
	Assignments func(*auth.Session, *site.Capabilities, safety.Mode) *assignment.Service
	// Interactive reports whether there is a person at the other end to
	// answer a confirmation prompt. It is injected because deciding that means
	// inspecting the real process streams, which this layer does not own.
	Interactive func() bool
}

// targetSite converts a configuration entry into the domain type.
func targetSite(name string, entry *config.Site) (site.Site, error) {
	base, err := site.ParseBaseURL(entry.BaseURL)
	if err != nil {
		return site.Site{}, err
	}
	target := site.Site{ID: entry.ID, Name: name, BaseURL: base}
	if entry.WWWRoot != "" {
		if root, err := site.ParseBaseURL(entry.WWWRoot); err == nil {
			target.WWWRoot = root
		}
	}
	return target, nil
}
