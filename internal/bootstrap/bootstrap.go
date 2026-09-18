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

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
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
	"github.com/KoukeNeko/moodle-cli/internal/grade"
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
			manual.New(),
		),
		Courses: func(session *auth.Session, capabilities *site.Capabilities) *course.Service {
			// Preference order, most reliable first. The AJAX and HTML
			// backends arrive in Phase 8; a feature with one backend is
			// still a feature.
			return course.NewService(
				moodle.NewCourseBackend(session.Client(), session.Token(), capabilities),
			)
		},
		Assignments: func(session *auth.Session, _ *site.Capabilities, mode safety.Mode) *assignment.Service {
			return assignment.NewService(
				moodle.NewAssignmentBackend(session.Client(), session.Token()),
				moodle.NewAssignmentWriter(session.Client(), session.Token()),
				mode,
			)
		},
		Grades: func(session *auth.Session, capabilities *site.Capabilities) *grade.Service {
			return grade.NewService(
				moodle.NewGradeBackend(session.Client(), session.Token(), capabilities),
			)
		},
		Calendar: func(session *auth.Session, _ *site.Capabilities) *calendar.Service {
			return calendar.NewService(
				moodle.NewCalendarBackend(session.Client(), session.Token()),
			)
		},
		Files: func(session *auth.Session, capabilities *site.Capabilities) *file.Downloader {
			return file.NewDownloader(
				moodle.NewFileFetcher(session.Client(), session.Token(), capabilities),
			)
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
