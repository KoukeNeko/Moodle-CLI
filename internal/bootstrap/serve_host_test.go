//go:build !windows

package bootstrap

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/mcphost"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/tests/testmoodle"
)

// These tests run the real composition root against a fake Moodle and a shell
// script standing in for cloudflared, because the wiring is what keeps a tunnel
// pointed at this host and nowhere else, and nothing smaller can show that.

// lockedBuffer stands in for os.Stderr and is read while the host still writes.
type lockedBuffer struct {
	mu   sync.Mutex
	text bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.String()
}

// hostSession is a configured site, an environment credential, a stand-in for
// cloudflared first on PATH, and a way to run `mcp host`.
type hostSession struct {
	dir    string
	stderr *lockedBuffer
}

func newHostSession(t *testing.T, script string) *hostSession {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cloudflared"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	server := testmoodle.New()
	t.Cleanup(server.Close)
	server.AddToken("good-token")
	server.Handle(moodle.FunctionSiteInfo, func(url.Values) (any, error) {
		return map[string]any{
			"sitename": "Test Moodle", "username": "student1",
			"firstname": "Sam", "lastname": "Student", "userid": 7,
			"release": "5.2.3", "downloadfiles": 1, "uploadfiles": 1,
			"functions": []any{},
		}, nil
	})
	t.Setenv("MOODLE_CLI_CONFIG", filepath.Join(dir, "config.yaml"))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MOODLE_HOST_PASSPHRASE", "")
	t.Setenv("MOODLE_WS_TOKEN", "good-token")
	if code := Run(context.Background(), Build{}, []string{"site", "add", "school", server.URL()}); code != 0 {
		t.Fatalf("site add: exit %d", code)
	}

	// There is no tunnel to reach, so the address check is stood in for.
	previous := verifyPublicAddress
	verifyPublicAddress = func(context.Context, *mcphost.Host) error { return nil }
	t.Cleanup(func() { verifyPublicAddress = previous })

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &lockedBuffer{}
	original := os.Stderr
	os.Stderr = writer
	t.Cleanup(func() { os.Stderr = original })
	go io.Copy(stderr, reader)
	return &hostSession{dir: dir, stderr: stderr}
}

// run starts `mcp host` and returns its exit code, once it ends, and a way to
// stop it as a signal would.
func (h *hostSession) run(args ...string) (<-chan int, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	codes := make(chan int, 1)
	go func() { codes <- Run(ctx, Build{}, append([]string{"mcp", "host"}, args...)) }()
	return codes, cancel
}

func (h *hostSession) waitFor(t *testing.T, what string, within time.Duration, condition func() bool) {
	t.Helper()
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if condition() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s:\n%s", what, h.stderr.String())
}

// exit waits for the host to end and returns its code.
func (h *hostSession) exit(t *testing.T, codes <-chan int, within time.Duration) int {
	t.Helper()
	select {
	case code := <-codes:
		time.Sleep(200 * time.Millisecond) // let the pipe deliver the last lines
		return code
	case <-time.After(within):
		t.Fatalf("the host did not end:\n%s", h.stderr.String())
		return -1
	}
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// upScript is a cloudflared that records how it was run, prints an address, and
// reports being connected.
const upScript = `
dir=$(dirname "$0")
echo "$@" > "$dir/args"
env > "$dir/env"
echo $$ > "$dir/pid"
echo "2026-01-01T00:00:00Z INF |  https://fake-tunnel-address.trycloudflare.com   |" >&2
echo "2026-01-01T00:00:01Z INF Registered tunnel connection connIndex=0" >&2
trap 'sleep 0.6; exit 0' INT TERM
while true; do sleep 0.1; done
`

func TestTheTunnelIsPointedAtTheHostsOwnListenerAndStoppedWithIt(t *testing.T) {
	// The tunnel must be given the port this process bound — the one the host
	// says it listens on — and must not outlive the host.
	h := newHostSession(t, upScript)
	codes, stop := h.run("--site", "school")
	defer stop()

	h.waitFor(t, "the connector address", 20*time.Second, func() bool {
		return strings.Contains(h.stderr.String(), "fake-tunnel-address.trycloudflare.com/mcp")
	})
	var listening string
	for _, line := range strings.Split(h.stderr.String(), "\n") {
		if rest, ok := strings.CutPrefix(line, "moodle: listening on "); ok {
			listening = strings.TrimSpace(rest)
		}
	}
	args, _ := os.ReadFile(filepath.Join(h.dir, "args"))
	if listening == "" || !strings.Contains(string(args), "--url http://"+listening) {
		t.Errorf("the host listens on %q but cloudflared was given %q", listening, args)
	}
	if !strings.HasPrefix(listening, "127.0.0.1:") || strings.HasSuffix(listening, ":0") {
		t.Errorf("listening on %q, want a free loopback port", listening)
	}
	rawPID, _ := os.ReadFile(filepath.Join(h.dir, "pid"))
	pid, _ := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if !strings.Contains(h.stderr.String(), "anyone with its address can reach the sign-in page") {
		t.Errorf("the person was not told the address is public:\n%s", h.stderr.String())
	}

	stop()

	// Checked the moment the host has ended, not a while later: the stand-in
	// takes a moment to exit, and the host has to have waited for it.
	var code int
	select {
	case code = <-codes:
	case <-time.After(20 * time.Second):
		t.Fatalf("the host did not end:\n%s", h.stderr.String())
	}
	if alive(pid) {
		t.Errorf("cloudflared %d was still running when the host ended", pid)
	}
	if code != 0 {
		t.Errorf("exit %d\n%s", code, h.stderr.String())
	}
}

func TestCloudflaredIsNotHandedTheMoodleCredentials(t *testing.T) {
	h := newHostSession(t, upScript)
	t.Setenv("MOODLE_SESSION", "a-browser-session")
	codes, stop := h.run("--site", "school")
	defer stop()
	h.waitFor(t, "the connector address", 20*time.Second, func() bool {
		return strings.Contains(h.stderr.String(), "fake-tunnel-address.trycloudflare.com/mcp")
	})

	environment, _ := os.ReadFile(filepath.Join(h.dir, "env"))
	stop()
	h.exit(t, codes, 20*time.Second)

	if strings.Contains(string(environment), "MOODLE_") {
		t.Errorf("cloudflared was given Moodle settings:\n%s", environment)
	}
	if !strings.Contains(string(environment), "PATH=") {
		t.Errorf("cloudflared was given no environment at all:\n%s", environment)
	}
}

func TestAQuickTunnelIsNeverPointedAtAWildcardAddress(t *testing.T) {
	h := newHostSession(t, upScript)
	codes, stop := h.run("--site", "school", "--listen", "0.0.0.0:0")
	defer stop()

	code := h.exit(t, codes, 20*time.Second)

	if _, err := os.Stat(filepath.Join(h.dir, "args")); err == nil {
		t.Error("cloudflared was started against a wildcard listener")
	}
	if out := h.stderr.String(); code == 0 || !strings.Contains(out, "needs --listen on a loopback address") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestTheAddressIsOnlyAnnouncedOnceItIsKnownToWork(t *testing.T) {
	// A tunnel can come up and lead somewhere else. Telling the person to use
	// its address before checking would send them to it.
	h := newHostSession(t, upScript)
	verifyPublicAddress = func(context.Context, *mcphost.Host) error {
		return context.DeadlineExceeded // stands in for "this address does not reach the host"
	}
	codes, stop := h.run("--site", "school")
	defer stop()

	code := h.exit(t, codes, 20*time.Second)

	out := h.stderr.String()
	if code == 0 || !strings.Contains(out, "does not lead to this host") {
		t.Errorf("exit %d, want a refusal naming the tunnel\n%s", code, out)
	}
	if strings.Contains(out, "as a custom connector") {
		t.Errorf("an address that does not work was announced:\n%s", out)
	}
	rawPID, _ := os.ReadFile(filepath.Join(h.dir, "pid"))
	if pid, _ := strconv.Atoi(strings.TrimSpace(string(rawPID))); pid != 0 && alive(pid) {
		t.Errorf("cloudflared %d was left running", pid)
	}
}

func TestTheHostStopsWhenItsTunnelDies(t *testing.T) {
	h := newHostSession(t, `
echo "2026-01-01T00:00:00Z INF |  https://fake-tunnel-address.trycloudflare.com   |" >&2
echo "2026-01-01T00:00:01Z INF Registered tunnel connection connIndex=0" >&2
sleep 1
echo "ERR connection lost" >&2
exit 3
`)
	codes, stop := h.run("--site", "school")
	defer stop()

	code := h.exit(t, codes, 20*time.Second)

	out := h.stderr.String()
	if code == 0 || !strings.Contains(out, "quick tunnel stopped") || !strings.Contains(out, "connection lost") {
		t.Errorf("exit %d, want a failure that carries cloudflared's last words\n%s", code, out)
	}
}

func TestCloudflaredNotBeingInstalledIsSaidPlainly(t *testing.T) {
	h := newHostSession(t, "exit 0\n")
	t.Setenv("PATH", t.TempDir())
	codes, stop := h.run("--site", "school")
	defer stop()

	code := h.exit(t, codes, 20*time.Second)

	if out := h.stderr.String(); code == 0 || !strings.Contains(out, "cloudflared is not installed") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestInterruptingWhileTheTunnelComesUpIsAnInterruptNotAFailure(t *testing.T) {
	// The start-up window is many seconds. Ctrl-C in it must not read as a
	// tunnel that failed, with a hint about --public-url.
	h := newHostSession(t, `
echo "2026-01-01T00:00:00Z INF |  https://fake-tunnel-address.trycloudflare.com   |" >&2
trap 'exit 0' INT TERM
while true; do sleep 0.1; done
`)
	codes, stop := h.run("--site", "school")
	h.waitFor(t, "the tunnel to be started", 10*time.Second, func() bool {
		return strings.Contains(h.stderr.String(), "starting a Cloudflare quick tunnel")
	})
	time.Sleep(300 * time.Millisecond)

	stop()

	code := h.exit(t, codes, 20*time.Second)

	out := h.stderr.String()
	if code != 130 || strings.Contains(out, "does not lead") || strings.Contains(out, "cannot start a Cloudflare quick tunnel") {
		t.Errorf("exit %d, want 130 and no failure message\n%s", code, out)
	}
}

func TestAShortPassphraseIsRefusedBeforeAnyTunnelIsStarted(t *testing.T) {
	// Waiting out a tunnel to be told the passphrase was too short, with a
	// public address already created, is the wrong order.
	h := newHostSession(t, upScript)
	t.Setenv("MOODLE_HOST_PASSPHRASE", "hunter2")
	codes, stop := h.run("--site", "school")
	defer stop()

	code := h.exit(t, codes, 20*time.Second)

	if _, err := os.Stat(filepath.Join(h.dir, "args")); err == nil {
		t.Error("cloudflared was started for a host that could not run")
	}
	if out := h.stderr.String(); code == 0 || !strings.Contains(out, "at least 12 characters") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestACloudflaredConfigFileCannotRedirectTheTunnel(t *testing.T) {
	// The stand-in notes whether it was given a configuration file of its own:
	// without one, cloudflared reads config.yml from its default places and
	// ingress rules there replace --url.
	h := newHostSession(t, upScript)
	codes, stop := h.run("--site", "school")
	defer stop()
	h.waitFor(t, "the connector address", 20*time.Second, func() bool {
		return strings.Contains(h.stderr.String(), "fake-tunnel-address.trycloudflare.com/mcp")
	})

	args, _ := os.ReadFile(filepath.Join(h.dir, "args"))
	stop()
	h.exit(t, codes, 20*time.Second)

	if !strings.Contains(string(args), "--config ") {
		t.Errorf("cloudflared was run with %q, want an explicit --config", args)
	}
}

func TestATunnelThatDiesWhileTheAddressIsBeingCheckedIsReportedAsThat(t *testing.T) {
	// Verify gives up when the tunnel's death stops the host. What the person
	// needs is cloudflared's last words, not a hint about configuration files.
	h := newHostSession(t, `
echo "2026-01-01T00:00:00Z INF |  https://fake-tunnel-address.trycloudflare.com   |" >&2
echo "2026-01-01T00:00:01Z INF Registered tunnel connection connIndex=0" >&2
sleep 1
echo "ERR connection lost" >&2
exit 3
`)
	verifyPublicAddress = func(ctx context.Context, _ *mcphost.Host) error {
		<-ctx.Done() // the address never answers
		return ctx.Err()
	}
	codes, stop := h.run("--site", "school")
	defer stop()

	code := h.exit(t, codes, 20*time.Second)

	out := h.stderr.String()
	if code == 0 || !strings.Contains(out, "quick tunnel stopped") || !strings.Contains(out, "connection lost") {
		t.Errorf("exit %d, want the tunnel's end reported with its last words\n%s", code, out)
	}
	if strings.Contains(out, "does not lead to this host") {
		t.Errorf("a tunnel that died was blamed on a configuration file:\n%s", out)
	}
}
