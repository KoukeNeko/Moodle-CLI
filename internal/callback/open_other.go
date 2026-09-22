//go:build !linux

package callback

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// OpenInBrowser asks the system to open an address.
//
// There is no portal here, so the address goes on a command line. On these
// systems that is the ordinary way, and the caller is told which route was
// used either way.
func OpenInBrowser(ctx context.Context, address string) (viaPortal bool, err error) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{address}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", address}
	default:
		return false, errs.New(errs.CodeUnavailable,
			"cannot open a browser on "+runtime.GOOS)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return false, errs.Wrap(errs.CodeUnavailable, err, "cannot find "+name)
	}
	if err := exec.CommandContext(ctx, path, args...).Run(); err != nil {
		return false, errs.Wrap(errs.CodeUnavailable, err, "cannot open a browser")
	}
	return false, nil
}
