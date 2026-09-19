//go:build unix

package callback

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// socketName is the file the waiting process listens on. One per user, in the
// directory the desktop specification set aside for exactly this.
const socketName = "auth.sock"

// Listen opens the channel the handler hands a callback back through.
//
// The directory is the one XDG reserves for runtime objects: owned by this
// user and mode 0700 by specification, which is what keeps another user out.
// That is checked rather than assumed — a runtime directory that is not those
// things is not a place to put a credential channel, and the honest response
// is to refuse rather than to continue on a worse guarantee.
func Listen(dir string) (net.Listener, error) {
	if dir == "" {
		return nil, errs.New(errs.CodeUnavailable,
			"there is no per-user runtime directory to listen in").
			WithHint("XDG_RUNTIME_DIR is unset; automatic browser login needs " +
				"a private directory, so use `--method manual` here")
	}
	if err := checkPrivate(dir); err != nil {
		return nil, err
	}

	ours := filepath.Join(dir, "moodle-cli")
	if err := os.MkdirAll(ours, 0o700); err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, err,
			"cannot create the directory for the login channel")
	}
	// MkdirAll leaves an existing directory's mode alone, so a directory
	// someone else created loosely would be used as it stands.
	if err := checkPrivate(ours); err != nil {
		return nil, err
	}

	path := filepath.Join(ours, socketName)
	if err := clearStale(path); err != nil {
		return nil, err
	}
	// Created 0600 rather than chmod'ed afterwards: between bind and chmod is
	// a window, and the window is the whole thing being guarded.
	previous := syscall.Umask(0o177)
	listener, err := net.Listen("unix", path)
	syscall.Umask(previous)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, err,
			"cannot open the login channel")
	}
	return listener, nil
}

// checkPrivate refuses a directory another user could reach into.
func checkPrivate(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot read "+dir)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		// Following it would mean trusting wherever it points, which is not
		// this directory's guarantee.
		return errs.New(errs.CodeUnavailable, dir+" is a symbolic link").
			WithHint("the login channel needs a real private directory")
	}
	if !info.IsDir() {
		return errs.New(errs.CodeUnavailable, dir+" is not a directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errs.New(errs.CodeInternal, "cannot read the owner of "+dir)
	}
	if uint64(stat.Uid) != uint64(os.Getuid()) {
		return errs.New(errs.CodeUnavailable,
			dir+" belongs to another user").
			WithHint("the login channel must live somewhere only you can reach")
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return errs.New(errs.CodeUnavailable,
			fmt.Sprintf("%s is readable by others (mode %04o)", dir, perm)).
			WithHint("the specification says this directory is 0700; " +
				"something has changed it")
	}
	return nil
}

// clearStale removes a socket left by a process that is gone.
//
// Only a socket, and only one of ours: unlinking whatever happens to be in
// the way would be a way to delete a file by choosing its name.
func clearStale(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot read "+path)
	}
	if info.Mode()&fs.ModeSocket == 0 {
		return errs.New(errs.CodeConflict,
			path+" is in the way and is not a socket").
			WithHint("something else is using that name; move it aside")
	}
	// A live listener answers. If one does, this is not stale and another
	// login is already waiting.
	if conn, err := net.Dial("unix", path); err == nil {
		_ = conn.Close()
		return errs.New(errs.CodeConflict,
			"another login is already waiting on this machine").
			WithHint("finish or cancel it first")
	}
	if err := os.Remove(path); err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot clear "+path)
	}
	return nil
}

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
