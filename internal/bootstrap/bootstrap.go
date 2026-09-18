// Package bootstrap is the composition root: it builds every concrete
// dependency and wires them together.
//
// It may import anything under internal/. Nothing may import it except
// cmd/moodle — that rule is enforced by tests/arch.
package bootstrap

import (
	"context"
	"os"

	"golang.org/x/term"

	"github.com/KoukeNeko/moodle-cli/internal/api"
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/browsersession"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/manual"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/password"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/qrlogin"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/token"
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/mcp"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Build carries the version stamps injected at link time.
type Build struct {
	Version   string
	Commit    string
	BuildDate string
}

// Run assembles the application and executes it, returning the process exit
// code. It is the only entry point cmd/moodle needs.
func Run(ctx context.Context, build Build, args []string) int {
	version := valueOr(build.Version, "dev")
	streams := cli.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	configPath, err := config.DefaultPath()
	if err != nil {
		// Without a configuration path nothing can be resolved, so report it
		// through the same renderer the commands would have used.
		renderer := cli.Renderer{Streams: streams, Format: cli.FormatTable}
		return renderer.RenderError(err)
	}

	// One HTTP client for the whole process, so connections are reused.
	httpClient := moodle.NewHTTPClient()
	newClient := func(target site.Site) *moodle.Client {
		return moodle.NewClient(target,
			moodle.WithHTTPClient(httpClient),
			moodle.WithUserAgent(moodle.DefaultUserAgent(version)),
			// A Moodle is usually a shared university service, and the thing
			// driving this client may be a loop.
			moodle.WithPacing(moodle.DefaultPacing),
		)
	}
	manager := auth.NewManager(secret.Keyring{}, newClient)

	deps := cli.Deps{
		ConfigPath: configPath,
		Auth:       manager,
		// Preference order: least disruptive first. A method that needs the
		// user to paste something is never chosen automatically.
		Login: auth.NewCoordinator(manager,
			token.New(),
			password.New(newClient, readPassword),
			qrlogin.New(newClient),
			// Never chosen automatically: it needs a session cookie handed
			// over, and a credential that powerful is not something to go
			// looking for on someone's behalf.
			browsersession.New(newClient),
			manual.New(),
		),
		Courses: func(session *auth.Session, capabilities *site.Capabilities) *course.Service {
			// Most reliable first. The web service route answers with more,
			// and skips itself by its own requirement when there is no token;
			// the browser-session route then takes over, which is the only way
			// in on a site that issues no token at all.
			backends := []course.Backend{
				moodle.NewCourseBackend(session.Client(), session.Token(), capabilities),
			}
			if cookie := session.Cookie(); cookie.Value != "" {
				backends = append(backends, moodle.NewCourseAjaxBackend(
					moodle.NewAjaxSession(session.Client(), cookie)))
			}
			return course.NewService(backends...)
		},
		Assignments: func(session *auth.Session, capabilities *site.Capabilities, mode safety.Mode) *assignment.Service {
			// The web service route comes first and is the only one trusted to
			// submit. On a site with no token it skips itself, and reading
			// falls to the pages — the only remaining source, since
			// assignments are not exposed over the AJAX endpoint either.
			backends := []assignment.Backend{
				moodle.NewAssignmentBackend(session.Client(), session.Token()),
			}
			if cookie := session.Cookie(); cookie.Value != "" {
				ajax := moodle.NewAjaxSession(session.Client(), cookie)
				backends = append(backends, moodle.NewAssignHTMLBackend(
					moodle.NewPageReader(session.Client(), cookie),
					func(ctx context.Context) ([]string, error) {
						return courseIDs(ctx, capabilities, ajax)
					}))
			}
			return assignment.NewService(
				moodle.NewAssignmentWriter(session.Client(), session.Token()),
				mode, backends...)
		},
		Grades: func(session *auth.Session, capabilities *site.Capabilities) *grade.Service {
			return grade.NewService(
				moodle.NewGradeBackend(session.Client(), session.Token(), capabilities),
			)
		},
		Calendar: func(session *auth.Session, _ *site.Capabilities) *calendar.Service {
			return calendar.NewService(pickBackends(session,
				func() calendar.Backend {
					return moodle.NewCalendarBackend(session.Client(), session.Token())
				},
				func(s *moodle.AjaxSession) calendar.Backend {
					return moodle.NewCalendarAjaxBackend(s)
				})...)
		},
		Forums: func(session *auth.Session, _ *site.Capabilities) *forum.Service {
			return forum.NewService(pickBackends(session,
				func() forum.Backend {
					return moodle.NewForumBackend(session.Client(), session.Token())
				},
				func(s *moodle.AjaxSession) forum.Backend {
					return moodle.NewForumAjaxBackend(s)
				})...)
		},
		Files: func(session *auth.Session, capabilities *site.Capabilities) *file.Downloader {
			return file.NewDownloader(
				moodle.NewFileFetcher(session.Client(), session.Token(), capabilities),
			)
		},
		API: func(session *auth.Session, mode safety.Mode, allowWrite bool) *api.Service {
			return api.NewService(
				moodle.NewRawCaller(session.Client(), session.Token()),
				mode, allowWrite,
			)
		},
		ServeMCP: func(ctx context.Context, session cli.MCPSession) error {
			client, token := session.Session.Client(), session.Session.Token()
			deps := mcp.Deps{
				Capabilities: session.Capabilities,
				SiteName:     session.SiteName,
				AccountName:  session.AccountName,
				Courses: course.NewService(
					moodle.NewCourseBackend(client, token, session.Capabilities)),
				Assignments: assignment.NewService(
					moodle.NewAssignmentWriter(client, token),
					// The server's own read-only state is separate from the
					// tool surface: a writing tool that somehow ran without
					// being offered would still be stopped here.
					safety.Mode{ReadOnly: !session.AllowWrite},
					moodle.NewAssignmentBackend(client, token),
				),
				Grades: grade.NewService(
					moodle.NewGradeBackend(client, token, session.Capabilities)),
				Calendar: calendar.NewService(moodle.NewCalendarBackend(client, token)),
				Forums:   forum.NewService(moodle.NewForumBackend(client, token)),
			}
			server := mcp.NewServer(
				// stdout is the protocol. Everything this server says to a
				// person goes to stderr, or the client cannot parse the stream.
				mcp.Streams{In: os.Stdin, Out: os.Stdout, Log: os.Stderr},
				mcp.BuildInfo{Version: version},
				mcp.Register(deps, session.AllowWrite),
			)
			return server.Serve(ctx)
		},
		Interactive: func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
	}

	app := cli.New(
		cli.BuildInfo{
			Version:   version,
			Commit:    valueOr(build.Commit, "unknown"),
			BuildDate: build.BuildDate,
		},
		streams,
		deps,
	)
	return app.Execute(ctx, args)
}

// courseIDs lists the caller's courses for a route that cannot enumerate them.
//
// Reading a page finds the activities in a course but not which courses there
// are; that answer comes from the endpoint a browser session can use.
func courseIDs(ctx context.Context, capabilities *site.Capabilities, ajax *moodle.AjaxSession) ([]string, error) {
	result, err := moodle.NewCourseAjaxBackend(ajax).List(ctx, course.ListQuery{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Courses))
	for _, item := range result.Courses {
		ids = append(ids, item.ID)
	}
	return ids, nil
}

// pickBackends puts the web service route first and adds the browser-session
// route when there is a session to use.
//
// The web service route skips itself for want of a token, so on a site that
// issues none the session route is what answers.
func pickBackends[T any](session *auth.Session, ws func() T, ajax func(*moodle.AjaxSession) T) []T {
	backends := []T{ws()}
	if cookie := session.Cookie(); cookie.Value != "" {
		backends = append(backends,
			ajax(moodle.NewAjaxSession(session.Client(), cookie)))
	}
	return backends
}

// readPassword reads from the terminal without echoing. When stdin is not a
// terminal there is nothing to hide the typing from, so the caller is told to
// pipe the password instead of having it appear on screen.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errs.New(errs.CodeUsage, "stdin is not a terminal").
			WithHint("use --password-stdin to pipe the password in")
	}
	raw, err := term.ReadPassword(fd)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
