package bootstrap

import (
	"context"
	"os"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/tunnel"
)

// TestMain lets the test binary act as the guard a tunnel runs under, which is
// the program started again with an argument of its own.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == tunnel.GuardArg {
		os.Exit(tunnel.RunGuard(context.Background(), os.Args[2:]))
	}
	os.Exit(m.Run())
}
