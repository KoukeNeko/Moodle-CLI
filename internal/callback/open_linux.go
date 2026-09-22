//go:build linux

package callback

import (
	"context"
	"os/exec"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// OpenInBrowser asks the desktop to open an address.
//
// The portal is tried first, and not for politeness: it takes the address as
// a bus message parameter, whereas running a command puts it on that
// command's argument list, where /proc exposes it to other users. The
// address carries the passport — the secret that ties a callback to this
// login — so that is worth avoiding where it can be.
//
// Where it cannot, the command is used and the caller is told, because the
// alternative is refusing to sign in on a desktop that works perfectly well.
func OpenInBrowser(ctx context.Context, address string) (viaPortal bool, err error) {
	if err := openThroughPortal(ctx, address); err == nil {
		return true, nil
	}
	for _, opener := range []string{"gio", "xdg-open"} {
		path, lookErr := exec.LookPath(opener)
		if lookErr != nil {
			continue
		}
		args := []string{address}
		if opener == "gio" {
			args = []string{"open", address}
		}
		if runErr := exec.CommandContext(ctx, path, args...).Run(); runErr == nil {
			return false, nil
		}
	}
	return false, errs.New(errs.CodeUnavailable,
		"cannot ask this desktop to open a browser").
		WithHint("open the address yourself, or use `--method manual`")
}

// openThroughPortal uses the desktop portal, where there is one.
func openThroughPortal(ctx context.Context, address string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	done := make(chan error, 1)
	go func() {
		object := conn.Object("org.freedesktop.portal.Desktop",
			dbus.ObjectPath("/org/freedesktop/portal/desktop"))
		call := object.Call("org.freedesktop.portal.OpenURI.OpenURI", 0,
			"", address, map[string]dbus.Variant{})
		done <- call.Err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		return errs.New(errs.CodeUnavailable, "the portal did not answer")
	case <-ctx.Done():
		return ctx.Err()
	}
}
