//go:build !linux

package callback

import (
	"context"
	"runtime"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Registration is what would be installed, per platform. The fields exist on
// every platform so the command layer needs no build tags of its own.
type Registration struct {
	Scheme      string
	DesktopFile string
	ServiceFile string
	Executable  string
	MIMEDefault string
}

func (Registration) Installed() bool { return false }

// Register is not implemented away from Linux, and says so plainly rather
// than appearing to work.
//
// macOS needs an application bundle and an Apple Event handler — the URL does
// not arrive on a command line there, so a bare binary in a bundle would be
// registered and then never hear anything. Windows needs a registry entry and
// a named pipe with a restrictive descriptor. Both are real work, and neither
// can be verified on the machine this was written on; a version written from
// specifications and called done is how a credential path ends up untested.
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
	hint := "use `moodle auth login --method manual`"
	if runtime.GOOS == "darwin" {
		hint = "sign in with Safari, then run `moodle auth import-browser --browser safari --store`; or use `moodle auth login --method manual`"
	}
	return errs.New(errs.CodeUnavailable,
		"automatic browser sign-in is not implemented on "+runtime.GOOS).
		WithHint(hint)
}
