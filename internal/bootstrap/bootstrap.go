// Package bootstrap is the composition root: it builds every concrete
// dependency and wires them together.
//
// It may import anything under internal/. Nothing may import it except
// cmd/moodle — that rule is enforced by tests/arch.
package bootstrap

import (
	"context"
	"os"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
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
	streams := cli.Streams{Out: os.Stdout, Err: os.Stderr}

	configPath, err := config.DefaultPath()
	if err != nil {
		// Without a configuration path nothing can be resolved, so report it
		// through the same renderer the commands would have used.
		renderer := cli.Renderer{Streams: streams, Format: cli.FormatTable}
		return renderer.RenderError(err)
	}

	// One HTTP client for the whole process, so connections are reused.
	httpClient := moodle.NewHTTPClient()
	deps := cli.Deps{
		ConfigPath: configPath,
		Auth: auth.NewManager(secret.Keyring{}, func(target site.Site) *moodle.Client {
			return moodle.NewClient(target,
				moodle.WithHTTPClient(httpClient),
				moodle.WithUserAgent(moodle.DefaultUserAgent(version)),
			)
		}),
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

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
