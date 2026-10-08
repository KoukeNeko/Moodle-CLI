//go:build linux || darwin

package callback

import (
	"io"
	"net"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Deliver uses the private socket shared by the Linux D-Bus and macOS
// Apple Event handlers. A callback without a waiting sign-in is refused.
func Deliver(runtimeDir, uri string) error {
	path, err := socketPath(runtimeDir)
	if err != nil {
		return err
	}
	if err := checkPrivate(runtimeDir); err != nil {
		return err
	}
	conn, err := net.DialTimeout("unix", path, 5*time.Second)
	if err != nil {
		return errs.New(errs.CodeNotFound,
			"no sign-in is waiting for this callback on this machine").
			WithHint("it may have been cancelled, or it may belong to another user's session")
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot set the callback delivery deadline")
	}
	if _, err := io.WriteString(conn, uri+"\n"); err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot hand over the callback")
	}
	return nil
}
