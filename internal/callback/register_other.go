//go:build !linux && !darwin

package callback

import (
	"context"
	"runtime"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func (Registration) Installed() bool { return false }

// Register is not implemented away from Linux and macOS, and says so plainly rather
// than appearing to work.
//
// Windows needs a registry entry and a named pipe with a restrictive
// descriptor. That credential path needs verification on a Windows desktop.
func Register(string, string) (Registration, error) {
	return Registration{}, unsupported()
}

func Status(scheme string) Registration { return Registration{Scheme: scheme} }

func Unregister(string) error { return unsupported() }

func ServeCallback(context.Context, string, string, time.Duration) error {
	return unsupported()
}

func Deliver(string, string) error { return unsupported() }

func unsupported() error {
	return errs.New(errs.CodeUnavailable,
		"automatic browser sign-in is not implemented on "+runtime.GOOS).
		WithHint("use `moodle auth login --method manual`")
}
