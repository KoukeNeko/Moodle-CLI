package cli

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/api"
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
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
	// Handler owns the platform-specific registration and callback process for
	// browser login. It is injected so the presentation layer never imports a
	// desktop adapter directly.
	Handler CallbackHandler
	// Backend overrides the site's own setting for this run: "" leaves the
	// site's choice alone, "ws-only" rules out the page and AJAX routes.
	//
	// It is a pointer because the flag is parsed after every command has been
	// built with its copy of these dependencies — the same reason the safety
	// mode is shared rather than copied.
	Backend *string
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
	// Grades assembles the grade use case.
	Grades func(*auth.Session, *site.Capabilities) *grade.Service
	// Calendar assembles the deadline use case.
	Calendar func(*auth.Session, *site.Capabilities) *calendar.Service
	// Forums assembles the discussion use case.
	Forums func(*auth.Session, *site.Capabilities) *forum.Service
	// Files assembles the download use case.
	Files func(*auth.Session, *site.Capabilities) *file.Downloader
	// API assembles the direct-call escape hatch. The safety mode and the
	// caller's acceptance of a write are both per invocation.
	API func(session *auth.Session, mode safety.Mode, allowWrite bool) *api.Service
	// ServeMCP runs the agent server. It is injected rather than built here
	// because the server speaks a protocol, and this layer does not.
	ServeMCP func(context.Context, MCPSession) error
	// Interactive reports whether there is a person at the other end to
	// answer a confirmation prompt. It is injected because deciding that means
	// inspecting the real process streams, which this layer does not own.
	Interactive func() bool
}

// HandlerRegistration is the presentation layer's view of an installed
// browser callback handler.
type HandlerRegistration struct {
	Scheme      string
	DesktopFile string
	ServiceFile string
	Executable  string
	MIMEDefault string
	Installed   bool
}

// CallbackHandler is the CLI-facing port for desktop callback management.
// The concrete Linux implementation is wired by bootstrap.
type CallbackHandler interface {
	Scheme() string
	Register() (HandlerRegistration, error)
	Unregister() error
	Status() HandlerRegistration
	Serve(context.Context, string) error
}

// MCPSession is everything the agent server needs for one session. It is bound
// to one site and one account: letting a tool call choose a different site
// would make every answer's provenance a question.
type MCPSession struct {
	Session      *auth.Session
	Capabilities *site.Capabilities
	SiteName     string
	AccountName  string
	// AllowWrite decides whether the writing tools exist at all.
	AllowWrite bool
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
