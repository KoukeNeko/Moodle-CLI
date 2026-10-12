// Package bootstrap is the composition root: it builds every concrete
// dependency and wires them together.
//
// It may import anything under internal/. Nothing may import it except
// cmd/moodle — that rule is enforced by tests/arch.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/KoukeNeko/moodle-cli/internal/api"
	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/browsersession"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/manual"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/mobilelaunch"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/password"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/qrlogin"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/token"
	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/callback"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
	"github.com/KoukeNeko/moodle-cli/internal/mcp"
	"github.com/KoukeNeko/moodle-cli/internal/mcphost"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/quiz"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/tunnel"
	"github.com/KoukeNeko/moodle-cli/internal/workload"
	"github.com/KoukeNeko/moodle-cli/internal/wsregistry"
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
	// The program starts itself as the guard of a tunnel it runs. That has
	// nothing to do with the command line below, and must not touch the
	// configuration or the keychain.
	if len(args) > 0 && args[0] == tunnel.GuardArg {
		return tunnel.RunGuard(ctx, args[1:])
	}

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
	// Read when a request is made, not now: --verbose is parsed after the
	// transport has been assembled.
	verbose := new(bool)
	newClient := func(target site.Site) *moodle.Client {
		options := []moodle.Option{
			moodle.WithHTTPClient(httpClient),
			moodle.WithUserAgent(moodle.DefaultUserAgent(version)),
			// A Moodle is usually a shared university service, and the thing
			// driving this client may be a loop.
			moodle.WithPacing(moodle.DefaultPacing),
		}
		if *verbose {
			// stderr, because stdout carries the result. The trace redacts
			// the token and the session cookie before writing them.
			options = append(options, moodle.WithTrace(streams.Err))
		}
		return moodle.NewClient(target, options...)
	}
	// The store is chosen by a flag parsed after this, and by the
	// configuration's own preference; both are read when a credential is used.
	credentialStore := new(secret.Backend)
	if file, err := config.Load(configPath); err == nil {
		if chosen, err := secret.ParseBackend(file.Preferences.CredentialStore); err == nil {
			*credentialStore = chosen
		}
	}
	manager := auth.NewManager(
		secret.Selected{Backend: credentialStore, ConfigPath: configPath}, newClient)
	registry, err := wsregistry.Load()
	if err != nil {
		renderer := cli.Renderer{Streams: streams, Format: cli.FormatTable}
		return renderer.RenderError(err)
	}

	// mcpTools assembles the tool surface for a session. Both transports
	// serve this one, so what an agent can do does not depend on how it
	// connected.
	mcpTools := func(session cli.MCPSession) *mcp.Registry {
		client, token := session.Session.Client(), session.Session.Token()
		return mcp.Register(mcp.Deps{
			// Asked per tool call rather than captured: this process
			// outlives the answer, and the session expires it on a timer.
			Capabilities: session.Session.Capabilities,
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
			Calendar:   calendar.NewService(moodle.NewCalendarBackend(client, token)),
			Forums:     forum.NewService(moodle.NewForumBackend(client, token)),
			WSRegistry: registry,
			WS: wsregistry.NewService(
				registry,
				moodle.NewRawCaller(client, token),
				safety.Mode{ReadOnly: !session.AllowWrite},
				session.AllowWrite,
			),
		}, session.AllowWrite)
	}

	deps := cli.Deps{
		ConfigPath: configPath,
		Auth:       manager,
		Handler:    desktopHandler{},
		// Preference order: least disruptive first. A method that needs the
		// user to paste something is never chosen automatically.
		Login: auth.NewCoordinator(manager,
			token.New(readPassword),
			password.New(newClient, readPassword),
			// Browser handoff is the SSO path that asks for no credential to
			// be copied. Linux needs prior handler registration; macOS can
			// install its per-user handler when this method runs.
			mobilelaunch.New(newClient, callback.DefaultBroker()),
			qrlogin.New(newClient),
			// Never chosen automatically: it needs a session cookie handed
			// over, and a credential that powerful is not something to go
			// looking for on someone's behalf.
			browsersession.New(newClient, readPassword),
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
					moodle.NewPageReader(session.Client(), cookie), ajax,
					func(ctx context.Context) ([]course.Summary, error) {
						result, err := moodle.NewCourseAjaxBackend(ajax).List(ctx, course.ListQuery{})
						return result.Courses, err
					}))
			}
			return assignment.NewService(
				moodle.NewAssignmentWriter(session.Client(), session.Token()),
				mode, backends...)
		},
		Grades: func(session *auth.Session, capabilities *site.Capabilities) *grade.Service {
			backends := []grade.Backend{
				moodle.NewGradeBackend(session.Client(), session.Token(), capabilities),
			}
			if cookie := session.Cookie(); cookie.Value != "" {
				// Grades are exposed over no endpoint a session can reach, so
				// the report page is the only remaining source.
				backends = append(backends, moodle.NewGradeHTMLBackend(
					moodle.NewPageReader(session.Client(), cookie)))
			}
			return grade.NewService(backends...)
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
		Forums: func(session *auth.Session, capabilities *site.Capabilities) *forum.Service {
			backends := pickBackends(session,
				func() forum.Backend {
					return moodle.NewForumBackend(session.Client(), session.Token())
				},
				func(s *moodle.AjaxSession) forum.Backend {
					return moodle.NewForumAjaxBackend(s)
				})
			if cookie := session.Cookie(); cookie.Value != "" {
				// The AJAX endpoint does not offer the forum function —
				// measured, it answers servicenotavailable on 4.5, 5.1 and
				// 5.2 — so on a site without a token the course page is the
				// only place left that names a course's forums.
				ajax := moodle.NewAjaxSession(session.Client(), cookie)
				backends = append(backends, moodle.NewForumHTMLBackend(
					moodle.NewPageReader(session.Client(), cookie),
					func(ctx context.Context) ([]string, error) {
						return courseIDs(ctx, capabilities, ajax)
					}))
			}
			return forum.NewService(backends...)
		},
		Quizzes: func(session *auth.Session, capabilities *site.Capabilities) *quiz.Service {
			backends := []quiz.Backend{
				moodle.NewQuizBackend(session.Client(), session.Token(), capabilities),
			}
			if cookie := session.Cookie(); cookie.Value != "" {
				// The quiz functions are not offered over AJAX, so without a
				// token the course and quiz pages are what is left, with the
				// calendar for the dates they print only as prose.
				ajax := moodle.NewAjaxSession(session.Client(), cookie)
				backends = append(backends, moodle.NewQuizHTMLBackend(
					moodle.NewPageReader(session.Client(), cookie), ajax,
					func(ctx context.Context) ([]course.Summary, error) {
						result, err := moodle.NewCourseAjaxBackend(ajax).List(ctx, course.ListQuery{})
						return result.Courses, err
					}))
			}
			return quiz.NewService(backends...)
		},
		Files: func(session *auth.Session, capabilities *site.Capabilities) *file.Downloader {
			return file.NewDownloader(
				moodle.NewFileFetcher(session.Client(), session.Token(), capabilities).
					WithSession(session.Cookie()),
			)
		},
		API: func(session *auth.Session, mode safety.Mode, allowWrite bool) *api.Service {
			return api.NewService(
				moodle.NewRawCaller(session.Client(), session.Token()),
				mode, allowWrite,
			)
		},
		WSRegistry: registry,
		WS: func(session *auth.Session, mode safety.Mode, allowWrite bool) *wsregistry.Service {
			return wsregistry.NewService(
				registry,
				moodle.NewRawCaller(session.Client(), session.Token()),
				mode, allowWrite,
			)
		},
		Workload: func(session *auth.Session, _ *site.Capabilities) *workload.Service {
			return workload.NewService(moodle.NewWorkloadBackend(session.Client(), session.Token()))
		},
		ServeMCP: func(ctx context.Context, session cli.MCPSession) error {
			server := mcp.NewServer(
				// stdout is the protocol. Everything this server says to a
				// person goes to stderr, or the client cannot parse the stream.
				mcp.Streams{In: os.Stdin, Out: os.Stdout, Log: os.Stderr},
				mcp.BuildInfo{Version: version},
				mcpTools(session),
			)
			return server.Serve(ctx)
		},
		ServeMCPHost: func(ctx context.Context, session cli.MCPSession, remote cli.MCPHost) error {
			// A passphrase that will be refused is refused now, not after a
			// tunnel has been brought up and a public address created.
			if err := mcphost.CheckPassphrase(remote.Passphrase); err != nil {
				return err
			}
			// Bind first. A tunnel is pointed at this listener, so what the
			// internet is given is known to be this host and not whatever else
			// happens to hold a port.
			listener, err := mcphost.Listen(remote.Addr)
			if err != nil {
				return err
			}
			defer listener.Close()

			serving, stop := context.WithCancel(ctx)
			defer stop()
			publicURL := remote.PublicURL
			var quick *tunnel.Tunnel
			if publicURL == "" {
				if err := requireLoopback(listener.Addr()); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "moodle: no --public-url, so starting a Cloudflare quick tunnel")
				quick, err = tunnel.Start(serving, localURL(listener.Addr()), tunnel.Options{
					// cloudflared is a third party's program and does not need
					// the credentials this process was given.
					Env: withoutPrefix(os.Environ(), "MOODLE_"),
				})
				if err != nil {
					if ctx.Err() != nil {
						// Stopped while it was coming up: that is an interrupt,
						// not a tunnel that failed.
						return ctx.Err()
					}
					return tunnelUnavailable(err)
				}
				defer quick.Close()
				publicURL = quick.URL
				fmt.Fprintln(os.Stderr, "moodle: the tunnel is up; anyone with its address can reach the "+
					"sign-in page until this host stops, and Cloudflare relays all traffic")
				// The address goes with the tunnel, so a tunnel that dies
				// takes the host down with it rather than serving to no one.
				go func() {
					select {
					case <-quick.Done():
						stop()
					case <-serving.Done():
					}
				}()
			}
			host, err := mcphost.New(mcphost.Config{
				PublicURL:   publicURL,
				Passphrase:  remote.Passphrase,
				SiteName:    named(session.SiteName, session.Capabilities.SiteName),
				AccountName: named(session.AccountName, accountOnSite(session.Capabilities)),
				AllowWrite:  session.AllowWrite,
				// stdout stays free for whatever a caller pipes; the host talks
				// to the person who started it on stderr.
				Log: os.Stderr,
			}, mcp.NewHTTPServer(os.Stderr, mcp.BuildInfo{Version: version}, mcpTools(session)).Handler())
			if err != nil {
				return err
			}

			finished := make(chan error, 1)
			go func() { finished <- host.Serve(serving, listener) }()
			if quick != nil {
				// The tunnel printed an address; whether it leads here is a
				// separate question, and the answer decides whether the person
				// is told to use it.
				if err := verifyPublicAddress(serving, host); err != nil {
					stop()
					<-finished
					if ctx.Err() != nil {
						return ctx.Err()
					}
					// A tunnel that died while it was being checked is not one
					// that leads elsewhere, and its last words are the cause.
					if died := quick.Err(); died != nil {
						return tunnelStopped(died)
					}
					return errs.Wrap(errs.CodeUnavailable, err,
						"the Cloudflare quick tunnel does not lead to this host").
						WithHint("check for a cloudflared configuration that sends traffic elsewhere, or pass --public-url with an address of your own")
				}
			}
			host.Announce(listener.Addr())

			served := <-finished
			if quick != nil {
				if err := quick.Err(); err != nil {
					return tunnelStopped(err)
				}
			}
			return served
		},
		Interactive:     func() bool { return term.IsTerminal(int(os.Stdin.Fd())) },
		Verbose:         verbose,
		CredentialStore: credentialStore,
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

// tunnelStopped reports a quick tunnel that ended while the host ran.
func tunnelStopped(err error) error {
	return errs.Wrap(errs.CodeUnavailable, err, "the Cloudflare quick tunnel stopped").
		WithHint("start the host again for a new address, and add the connector again")
}

// verifyPublicAddress is how the host checks a tunnel's address. It is a
// variable so the wiring can be tested without a tunnel that really leads
// anywhere.
var verifyPublicAddress = func(ctx context.Context, host *mcphost.Host) error {
	return host.Verify(ctx)
}

// requireLoopback refuses a listener a tunnel should not be pointed at.
//
// A wildcard address is reached through loopback, but on some systems a
// wildcard bind succeeds while another program holds the loopback address, and
// the tunnel would then publish that program. A specific loopback address has
// no such gap, and it keeps the plain-HTTP sign-in page off the local network.
func requireLoopback(address net.Addr) error {
	if tcp, ok := address.(*net.TCPAddr); ok && tcp.IP.IsLoopback() {
		return nil
	}
	return errs.New(errs.CodeUsage, "a Cloudflare quick tunnel needs --listen on a loopback address").
		WithHint("use 127.0.0.1, or pass --public-url with a tunnel or proxy of your own")
}

// localURL is how a process on this machine reaches a loopback listener.
func localURL(address net.Addr) string {
	tcp := address.(*net.TCPAddr)
	return "http://" + net.JoinHostPort(tcp.IP.String(), strconv.Itoa(tcp.Port))
}

// withoutPrefix drops the environment entries whose names start with prefix.
func withoutPrefix(environment []string, prefix string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(name), prefix) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// tunnelUnavailable says what to do about a quick tunnel that could not start,
// in the terms of the flags the person has.
func tunnelUnavailable(err error) error {
	if errors.Is(err, tunnel.ErrNotInstalled) {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot start a Cloudflare quick tunnel").
			WithHint("install cloudflared (https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/), or pass --public-url with an address of your own")
	}
	return errs.Wrap(errs.CodeUnavailable, err, "cannot start a Cloudflare quick tunnel").
		WithHint("read cloudflared's lines above; if the network blocks Cloudflare, or Cloudflare is limiting quick tunnels, pass --public-url with a tunnel or proxy of your own")
}

// named says who or what something is the way the person will recognise it:
// what the site calls it, with the local label beside it when they differ.
func named(label, reported string) string {
	if reported == "" || reported == label {
		return label
	}
	return reported + " (" + label + ")"
}

// accountOnSite is the account as Moodle knows it. The host's startup banner
// names the account it serves, and a local label such as "env" does not tell
// the owner which one that is.
func accountOnSite(capabilities *site.Capabilities) string {
	switch {
	case capabilities.FullName != "" && capabilities.Username != "":
		return capabilities.FullName + ", " + capabilities.Username
	case capabilities.Username != "":
		return capabilities.Username
	default:
		return capabilities.FullName
	}
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
