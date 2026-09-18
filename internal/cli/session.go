package cli

import (
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/config"
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
