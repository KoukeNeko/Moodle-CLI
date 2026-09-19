//go:build unix

package callback_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/callback"
)

// TestAnotherUserGenuinelyCannotConnect proves the boundary rather than
// asserting the mode that is supposed to create it.
//
// Every other test here checks that the directory is 0700 and the socket
// 0600. That is what the specification says keeps another user out, but it is
// a claim about what those numbers mean, not evidence. This one has a second
// user try, and requires the attempt to fail.
//
// It needs the privilege to become someone else, so it is skipped where that
// is not available — and skipped loudly, because a security test that quietly
// does nothing is worse than none.
func TestAnotherUserGenuinelyCannotConnect(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root to run the connecting half as another user")
	}
	other, err := lookupUID("nobody")
	if err != nil {
		t.Skipf("no second user to test with: %v", err)
	}

	// Not t.TempDir(): its parent is 0700 for this user, so the second user
	// would be stopped by Go's own temporary directory rather than by
	// anything under test.
	//
	// What is under test is the design as a whole — the runtime directory's
	// mode, this tool's own subdirectory, and the socket — not any one of
	// them. Checked by taking them away: loosen the runtime directory and
	// skip the checks, and this test fails, which is the only reason to
	// believe it means anything while it passes.
	dir, err := os.MkdirTemp("/tmp", "moodle-runtime-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	listener, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(dir, "moodle-cli", "auth.sock")

	// First make sure the second user could reach the socket if the
	// permissions allowed it, so a failure below is about permissions rather
	// than about the path being wrong.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the socket is not where the test looks: %v", err)
	}

	out, err := asUser(other, "test -S "+path)
	if err == nil {
		t.Errorf("another user can see the socket: %s", out)
	}

	// And the connection itself.
	out, err = asUser(other, "exec 3<>"+path)
	if err == nil {
		t.Errorf("another user connected to the login channel: %s", out)
	} else if !strings.Contains(strings.ToLower(out), "permission denied") &&
		!strings.Contains(strings.ToLower(out), "no such file") {
		t.Logf("refused, with: %s", out)
	}
}

func lookupUID(name string) (int, error) {
	out, err := exec.Command("id", "-u", name).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// asUser runs a shell command as another user and returns what it said.
func asUser(uid int, script string) (string, error) {
	cmd := exec.Command("setpriv", "--reuid", strconv.Itoa(uid),
		"--regid", strconv.Itoa(uid), "--clear-groups", "/bin/sh", "-c", script)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
