//go:build unix

package callback_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/callback"
)

// runtimeDir builds a directory with the mode the specification requires of
// XDG_RUNTIME_DIR, so the tests exercise the intended case rather than a
// permissive one.
func runtimeDir(t *testing.T) string {
	t.Helper()
	// t.TempDir includes the full test name. On macOS that can make the Unix
	// socket path exceed sockaddr_un.sun_path before the code under test gets
	// a chance to enforce any of its security properties.
	dir, err := os.MkdirTemp("", "mcl-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestWaitingForACallbackHonoursCancellation(t *testing.T) {
	dir := runtimeDir(t)
	listener, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = callback.Receive(ctx, listener, time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Receive error = %v, want context cancellation", err)
	}
}

func TestTheChannelIsPrivateToThisUser(t *testing.T) {
	dir := runtimeDir(t)
	listener, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	path := filepath.Join(dir, "moodle-cli", "auth.sock")
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	// 0600, and created that way rather than chmod'ed afterwards: between
	// bind and chmod is a window, and the window is the whole point.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("socket mode = %04o, want 0600", perm)
	}
	parent, err := os.Lstat(filepath.Join(dir, "moodle-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := parent.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("directory mode = %04o; others can reach in", perm)
	}
}

func TestARuntimeDirectoryOthersCanReadIsRefused(t *testing.T) {
	// The directory's mode is the boundary. If it is not what the
	// specification promises, the promise is not there to rely on, and
	// carrying on would be resting a credential channel on it anyway.
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := callback.Listen(dir)
	if err == nil {
		t.Fatal("a world-readable runtime directory was accepted")
	}
	if !strings.Contains(err.Error(), "readable by others") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

func TestAnExistingLooseDirectoryIsRefused(t *testing.T) {
	// MkdirAll leaves an existing directory's mode alone, so one created
	// loosely by something else would otherwise be used as it stands.
	dir := runtimeDir(t)
	ours := filepath.Join(dir, "moodle-cli")
	if err := os.Mkdir(ours, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ours, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := callback.Listen(dir); err == nil {
		t.Fatal("a directory others can write to was used for the login channel")
	}
}

func TestASymbolicLinkIsNotFollowed(t *testing.T) {
	// Following it would mean trusting wherever it points, which is not this
	// directory's guarantee — and the target is chosen by whoever made it.
	elsewhere := t.TempDir()
	dir := filepath.Join(t.TempDir(), "runtime")
	if err := os.Symlink(elsewhere, dir); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	if _, err := callback.Listen(dir); err == nil {
		t.Fatal("a symbolic link was used as the runtime directory")
	}
}

func TestNoRuntimeDirectoryIsSaidPlainly(t *testing.T) {
	// Not an internal error and not a silent fallback: the machine does not
	// offer what this needs, and the user is told which other method works.
	_, err := callback.Listen("")
	if err == nil {
		t.Fatal("an empty runtime directory was accepted")
	}
	if !strings.Contains(err.Error(), "runtime directory") {
		t.Errorf("message = %v", err)
	}
}

func TestAStaleSocketIsClearedButALiveOneIsNot(t *testing.T) {
	dir := runtimeDir(t)

	first, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A second login while the first is still waiting must be refused, not
	// quietly take over the channel the first is listening on.
	if _, err := callback.Listen(dir); err == nil {
		first.Close()
		t.Fatal("a second listener took over a live channel")
	}

	// Once the first is gone its socket is stale, and the next login may have
	// it. Closing a unix listener unlinks the file, so put one back.
	first.Close()
	path := filepath.Join(dir, "moodle-cli", "auth.sock")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	stale.Close()

	second, err := callback.Listen(dir)
	if err != nil {
		t.Fatalf("a stale socket was not cleared: %v", err)
	}
	second.Close()
}

func TestSomethingElseUsingTheNameIsNotDeleted(t *testing.T) {
	// Unlinking whatever is in the way would be a way to delete a file by
	// choosing its name.
	dir := runtimeDir(t)
	ours := filepath.Join(dir, "moodle-cli")
	if err := os.MkdirAll(ours, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ours, "auth.sock")
	if err := os.WriteFile(path, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := callback.Listen(dir); err == nil {
		t.Fatal("a regular file in the way was accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file was removed: %v", err)
	}
}

func TestTheConnectingProcessIsCheckedAgainstThisUser(t *testing.T) {
	dir := runtimeDir(t)
	listener, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		conn, err := net.Dial("unix", filepath.Join(dir, "moodle-cli", "auth.sock"))
		if err == nil {
			defer conn.Close()
		}
	}()

	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	same, err := callback.PeerIsSelf(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Error("this process was not recognised as itself")
	}
}
