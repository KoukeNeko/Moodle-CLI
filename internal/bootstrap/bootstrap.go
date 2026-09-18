// Package bootstrap is the composition root: it builds every concrete
// dependency and wires them together.
//
// It may import anything under internal/. Nothing may import it except
// cmd/moodle — that rule is enforced by tests/arch.
package bootstrap

import (
	"context"
	"os"

	"github.com/KoukeNeko/moodle-cli/internal/cli"
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
	app := cli.New(
		cli.BuildInfo{
			Version:   valueOr(build.Version, "dev"),
			Commit:    valueOr(build.Commit, "unknown"),
			BuildDate: build.BuildDate,
		},
		cli.Streams{Out: os.Stdout, Err: os.Stderr},
	)
	return app.Execute(ctx, args)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
