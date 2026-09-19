//go:build linux

package callback

import (
	"net"
	"os"
	"syscall"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// PeerIsSelf reports whether the process at the other end runs as this user.
//
// It is defence in depth, not the boundary: the directory's mode is what
// keeps another user out. This catches the case where that has been weakened
// without anyone noticing.
func PeerIsSelf(conn net.Conn) (bool, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return false, errs.New(errs.CodeInternal, "not a unix connection")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return false, errs.Wrap(errs.CodeInternal, err, "cannot inspect the connection")
	}
	var creds *syscall.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		creds, credErr = syscall.GetsockoptUcred(
			int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return false, errs.Wrap(errs.CodeInternal, err, "cannot read the peer")
	}
	if credErr != nil {
		return false, errs.Wrap(errs.CodeInternal, credErr, "cannot read the peer")
	}
	return int(creds.Uid) == os.Getuid(), nil
}
