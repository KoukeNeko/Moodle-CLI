package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/callback"
	"github.com/KoukeNeko/moodle-cli/internal/cli"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// desktopHandler adapts the platform callback package to the CLI-owned port.
// Translation belongs in the composition root: neither side needs to import
// the other just to render registration status.
type desktopHandler struct{}

func (desktopHandler) Scheme() string { return callback.Scheme }

func (desktopHandler) Register() (cli.HandlerRegistration, error) {
	executable, err := os.Executable()
	if err != nil {
		return cli.HandlerRegistration{}, errs.Wrap(errs.CodeUnavailable, err,
			"cannot find where this program is installed")
	}

	existing := callback.Status(callback.Scheme)
	ours := filepath.Base(existing.DesktopFile)
	if existing.MIMEDefault != "" && existing.MIMEDefault != ours {
		return cli.HandlerRegistration{}, errs.New(errs.CodeConflict,
			fmt.Sprintf("%s is already handled by %s",
				callback.Scheme, existing.MIMEDefault)).
			WithHint("leave that association in place and use `moodle auth login --method manual`")
	}

	registration, err := callback.Register(callback.Scheme, executable)
	if err != nil {
		return cli.HandlerRegistration{}, err
	}
	return presentHandler(registration), nil
}

func (desktopHandler) Unregister() error { return callback.Unregister(callback.Scheme) }

func (desktopHandler) Status() cli.HandlerRegistration {
	return presentHandler(callback.Status(callback.Scheme))
}

func (desktopHandler) Serve(ctx context.Context, scheme string) error {
	if err := callback.CheckScheme(scheme); err != nil {
		return err
	}
	return callback.ServeCallback(ctx, scheme,
		os.Getenv("XDG_RUNTIME_DIR"), 2*time.Minute)
}

func presentHandler(reg callback.Registration) cli.HandlerRegistration {
	return cli.HandlerRegistration{
		Scheme:      reg.Scheme,
		DesktopFile: reg.DesktopFile,
		ServiceFile: reg.ServiceFile,
		Executable:  reg.Executable,
		MIMEDefault: reg.MIMEDefault,
		Installed:   reg.Installed(),
	}
}
