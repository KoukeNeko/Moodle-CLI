// Package tunnel gives a local address a public HTTPS one by running
// cloudflared's account-less quick tunnel.
//
// It is for the person who has not set up a stable address: trying the host
// from a laptop, or showing it working. A quick tunnel gets a new address on
// every start and Cloudflare promises it nothing, so it is a way in, not a
// place to stay.
//
// This package only manages the subprocess. It knows nothing about what is
// being exposed, which is why it takes an address to point at and not a
// service to run.
package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ErrNotInstalled says cloudflared is not on the machine. The caller is the
// one who can say what to do about it, since the answer depends on its flags.
var ErrNotInstalled = errors.New("cloudflared is not installed")

// addressPattern finds the quick tunnel's address in cloudflared's log, where
// it is printed inside a box.
var addressPattern = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// connected is the line cloudflared logs once the tunnel can carry traffic. The
// address is printed several seconds earlier, while nothing can reach it yet.
const connected = "Registered tunnel connection"

// killAfter is how long cloudflared gets to act on an interrupt before it is
// killed. A variable so a test need not wait five seconds.
var killAfter = 5 * time.Second

// Options adjust how the tunnel is started.
type Options struct {
	// Binary is the cloudflared executable. Empty looks it up on PATH.
	Binary string
	// Env is the environment cloudflared runs in. Nil is this process's own.
	// Whatever is given, cloudflared's own TUNNEL_* settings are removed from
	// it: a quick tunnel has to be the one asked for, not one a variable in the
	// person's shell turned into something else.
	Env []string
	// Timeout bounds how long the tunnel gets to come up. Empty is a minute:
	// the first connection attempt can time out and be retried before one
	// succeeds.
	Timeout time.Duration
}

// Tunnel is a running quick tunnel.
type Tunnel struct {
	// URL is the public address: an origin, with no path.
	URL string

	stop context.CancelFunc
	done chan struct{}
	err  error
}

// Start runs cloudflared pointed at target and returns once the tunnel carries
// traffic. target must already be listening and must be the caller's own:
// whatever answers there is what the internet is given.
func Start(ctx context.Context, target string, options Options) (*Tunnel, error) {
	binary := options.Binary
	if binary == "" {
		binary = "cloudflared"
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, ErrNotInstalled
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = time.Minute
	}

	// A configuration file of our own, given explicitly. cloudflared reads
	// config.yml from several default places, and the ingress rules in one
	// replace --url: the tunnel comes up and prints its address, and publishes
	// whatever the file names instead of what this host bound. Anyone who has a
	// named tunnel set up has such a file.
	config, err := inertConfig()
	if err != nil {
		return nil, fmt.Errorf("prepare cloudflared: %w", err)
	}

	guard, err := selfPath()
	if err != nil {
		os.Remove(config)
		return nil, fmt.Errorf("prepare cloudflared's guard: %w", err)
	}

	// cloudflared runs under a guard, a second copy of this program that stops
	// it when this process is gone, however it went. See RunGuard.
	runCtx, stop := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, guard, GuardArg, killAfter.String(), path,
		"tunnel", "--config", config, "--no-autoupdate", "--url", target)
	environment := options.Env
	if environment == nil {
		environment = os.Environ()
	}
	cmd.Env = withoutCloudflaredSettings(environment)
	log := &watcher{ready: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = log, log
	// The write end of the guard's input stays open for as long as this
	// process lives. Closing it is how the guard is told to stop cloudflared,
	// and the operating system closing it is how it finds out this process is
	// gone.
	hold, err := cmd.StdinPipe()
	if err != nil {
		stop()
		os.Remove(config)
		return nil, fmt.Errorf("prepare cloudflared's guard: %w", err)
	}
	cmd.Cancel = hold.Close
	// The guard needs the grace period it was given, and a moment more to act
	// on it, before it is taken to be stuck.
	cmd.WaitDelay = killAfter + 3*time.Second

	t := &Tunnel{stop: stop, done: make(chan struct{})}
	started := make(chan error, 1)
	go func() {
		if err := cmd.Start(); err != nil {
			started <- err
			return
		}
		started <- nil
		waited := cmd.Wait()
		os.Remove(config)
		// Stopping on purpose is not a failure. Anything else is: a tunnel
		// that went away takes the address with it, whatever the exit status.
		switch {
		case runCtx.Err() != nil:
		case waited != nil:
			t.err = fmt.Errorf("cloudflared stopped: %w\n%s", waited, log.tail())
		default:
			t.err = fmt.Errorf("cloudflared stopped\n%s", log.tail())
		}
		close(t.done)
	}()

	if err := <-started; err != nil {
		stop()
		os.Remove(config)
		return nil, fmt.Errorf("start cloudflared: %w", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-log.ready:
		t.URL = log.address
		return t, nil
	case <-t.done:
		return nil, fmt.Errorf("cloudflared stopped before the tunnel came up\n%s", log.tail())
	case <-timer.C:
		t.Close()
		return nil, fmt.Errorf("the tunnel did not come up within %d seconds\n%s", int(timeout.Seconds()), log.tail())
	case <-ctx.Done():
		t.Close()
		return nil, ctx.Err()
	}
}

// inertConfig makes the configuration file cloudflared is given in place of the
// ones it would go looking for. It says only what the command line already
// says: an empty file would do as well, but cloudflared logs an error for one
// and a person reading a failure would take it for the cause.
func inertConfig() (string, error) {
	file, err := os.CreateTemp("", "moodle-cloudflared-*.yml")
	if err != nil {
		return "", err
	}
	_, err = file.WriteString("no-autoupdate: true\n")
	return file.Name(), errors.Join(err, file.Close())
}

// withoutCloudflaredSettings drops cloudflared's environment bindings. Each
// TUNNEL_* variable is a flag of the same name, and NO_AUTOUPDATE would only
// repeat one the command line already sets.
func withoutCloudflaredSettings(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "TUNNEL_") || upper == "NO_AUTOUPDATE" {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// Done is closed when cloudflared has exited.
func (t *Tunnel) Done() <-chan struct{} { return t.done }

// Err says why the tunnel ended. It is nil while the tunnel is running, and
// when it was stopped on purpose.
func (t *Tunnel) Err() error {
	select {
	case <-t.done:
		return t.err
	default:
		return nil
	}
}

// Close stops the tunnel and waits for cloudflared to exit.
func (t *Tunnel) Close() {
	t.stop()
	<-t.done
}

// watcher reads cloudflared's log as it is written. It keeps the address and
// whether the tunnel is up, and the last few lines so a failure can say what
// cloudflared said.
type watcher struct {
	mu      sync.Mutex
	partial []byte
	lines   []string
	address string
	up      bool
	ready   chan struct{}
	closed  bool
}

// keptLines is how much of cloudflared's output a failure reports.
const keptLines = 15

func (w *watcher) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.partial = append(w.partial, p...)
	for {
		end := bytes.IndexByte(w.partial, '\n')
		if end < 0 {
			break
		}
		w.see(strings.TrimRight(string(w.partial[:end]), "\r"))
		w.partial = w.partial[end+1:]
	}
	// A line with no end is not a log line; do not let it grow without bound.
	if len(w.partial) > 64<<10 {
		w.partial = nil
	}
	return len(p), nil
}

// see expects mu to be held.
func (w *watcher) see(line string) {
	if len(line) > 300 {
		line = line[:300]
	}
	w.lines = append(w.lines, line)
	if len(w.lines) > keptLines {
		w.lines = w.lines[1:]
	}
	if w.address == "" {
		w.address = addressPattern.FindString(line)
	}
	if strings.Contains(line, connected) {
		w.up = true
	}
	if w.address != "" && w.up && !w.closed {
		w.closed = true
		close(w.ready)
	}
}

func (w *watcher) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.lines, "\n")
}
