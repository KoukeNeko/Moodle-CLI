//go:build linux

package tunnel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCloudflaredDoesNotOutliveAGuardThatIsKilled(t *testing.T) {
	// The guard is what stops cloudflared when the host is killed. If the guard
	// is killed as well, the kernel has to do it on the systems that can.
	pidFile := filepath.Join(t.TempDir(), "pid")
	guard := exec.Command(os.Args[0], GuardArg, "5s", os.Args[0], "tunnel")
	guard.Env = append(os.Environ(), "FAKE_CLOUDFLARED=pid", "FAKE_PIDFILE="+pidFile)
	hold, err := guard.StdinPipe() // kept open: the host is alive
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()
	if err := guard.Start(); err != nil {
		t.Fatal(err)
	}
	defer guard.Process.Kill()

	var child int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && child == 0; time.Sleep(20 * time.Millisecond) {
		if raw, err := os.ReadFile(pidFile); err == nil {
			child, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
		}
	}
	if child == 0 {
		t.Fatal("the stand-in for cloudflared never started")
	}
	// If the protection under test is gone, the stand-in would sleep for an
	// hour; a failing test must not leave it behind.
	defer syscall.Kill(child, syscall.SIGKILL)

	_ = guard.Process.Kill()
	_, _ = guard.Process.Wait()

	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if !alive(child) {
			return
		}
	}
	t.Errorf("cloudflared %d was still running after its guard was killed", child)
}
