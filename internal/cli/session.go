package cli

import (
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/course"
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
