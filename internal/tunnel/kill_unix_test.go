//go:build !windows

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

func TestCloudflaredDoesNotOutliveAHostThatIsKilled(t *testing.T) {
	// A host killed without cleanup cannot stop its tunnel, and a tunnel left
	// running keeps publishing the address to whatever is later given the port.
	// The guard is what notices: the host's end of its input closes with it.
	pidFile := filepath.Join(t.TempDir(), "pid")
	parent := exec.Command(os.Args[0])
	parent.Env = append(os.Environ(), "FAKE_TUNNEL_PARENT=1", "FAKE_PIDFILE="+pidFile)
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	defer parent.Process.Kill()

	var child int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if raw, err := os.ReadFile(pidFile); err == nil {
			child, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			break
		}
	}
	if child == 0 {
		t.Fatal("the stand-in for cloudflared never started")
	}
	defer syscall.Kill(child, syscall.SIGKILL)

	if err := parent.Process.Kill(); err != nil { // SIGKILL: no deferred cleanup runs
		t.Fatal(err)
	}
	_, _ = parent.Process.Wait()

	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if syscall.Kill(child, 0) != nil {
			return
		}
	}
	t.Errorf("cloudflared %d was still running after its host was killed", child)
}
