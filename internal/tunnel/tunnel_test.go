package tunnel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for cloudflared: Start runs a program
// with cloudflared's arguments, and here that program answers as the case in
// FAKE_CLOUDFLARED says. It is the one way to test a subprocess that behaves
// the same on every platform.
func TestMain(m *testing.M) {
	// Start runs cloudflared under a guard, which is this program started with
	// an argument of its own; here that is the test binary.
	if len(os.Args) > 1 && os.Args[1] == GuardArg {
		os.Exit(RunGuard(context.Background(), os.Args[2:]))
	}
	if os.Getenv("FAKE_TUNNEL_PARENT") != "" {
		fakeParent()
		return
	}
	if mode := os.Getenv("FAKE_CLOUDFLARED"); mode != "" {
		fakeCloudflared(mode)
		return
	}
	os.Exit(m.Run())
}

// fakeParent stands in for a host: it starts a tunnel against the stand-in for
// cloudflared and then waits to be killed, which the test does without giving it
// a chance to clean up.
func fakeParent() {
	// The stand-in for cloudflared must not see the variable that made this
	// process a stand-in for a host, or it would become one too.
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "FAKE_TUNNEL_PARENT=") {
			environment = append(environment, entry)
		}
	}
	tunnel, err := Start(context.Background(), "http://127.0.0.1:1", Options{
		Binary: os.Args[0],
		Env:    append(environment, "FAKE_CLOUDFLARED=pid"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "parent:", err)
		os.Exit(1)
	}
	_ = tunnel
	time.Sleep(time.Hour)
}

// configArgument is the file the stand-in was told to use as its configuration.
func configArgument() string {
	for i, argument := range os.Args {
		if argument == "--config" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// A stand-in that waits has to do it with a sleep: with nothing else running,
// the Go runtime treats an empty select as a deadlock and aborts.
func fakeCloudflared(mode string) {
	banner := func() {
		// The first line cloudflared prints is long and holds other addresses; a
		// pattern loose enough to match one of them would publish it instead.
		fmt.Fprintln(os.Stderr, "2026-10-12T06:11:41Z INF Thank you for trying Cloudflare Tunnel. Doing so, without a Cloudflare account, is a quick way to experiment and try it out. However, be aware that these account-less Tunnels have no uptime guarantee, are subject to the Cloudflare Online Services Terms of Use (https://www.cloudflare.com/website-terms/), and Cloudflare reserves the right to investigate your use of Tunnels for violations of such terms. If you intend to use Tunnels in production you should use a pre-created named tunnel by following: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps")
		fmt.Fprintln(os.Stderr, "2026-10-12T06:11:42Z INF Requesting new quick Tunnel on trycloudflare.com...")
		fmt.Fprintln(os.Stderr, "2026-10-12T06:11:46Z INF |  https://how-eminem-festival-launched.trycloudflare.com                                    |")
	}
	up := func() {
		fmt.Fprintln(os.Stderr, "2026-10-12T06:11:54Z INF Registered tunnel connection connIndex=0 location=tpe01")
	}
	switch mode {
	case "ok":
		banner()
		time.Sleep(50 * time.Millisecond)
		up()
		interrupted := make(chan os.Signal, 1)
		signal.Notify(interrupted, os.Interrupt)
		<-interrupted
		if marker := os.Getenv("FAKE_MARKER"); marker != "" {
			os.WriteFile(marker, nil, 0o600)
		}
		fmt.Fprintln(os.Stderr, "INF Initiating graceful shutdown")
		os.Exit(0)
	case "args":
		banner()
		up()
		// Record how it was run once it has the address and has connected. The
		// file the test waits for is written last. With nowhere to record to,
		// write nothing: a path built from an empty name lands in the package.
		record := os.Getenv("FAKE_RECORD")
		if record == "" {
			time.Sleep(time.Hour)
		}
		if config := configArgument(); config != "" {
			if content, err := os.ReadFile(config); err == nil {
				os.WriteFile(record+".config", content, 0o600)
			}
		}
		os.WriteFile(record, []byte(strings.Join(os.Args[1:], "\n")+"\n---\n"+
			os.Getenv("TUNNEL_NAME")+"|"+os.Getenv("KEEP_ME")), 0o600)
		time.Sleep(time.Hour)
	case "pid":
		banner()
		up()
		os.WriteFile(os.Getenv("FAKE_PIDFILE"), []byte(fmt.Sprint(os.Getpid())), 0o600)
		time.Sleep(time.Hour)
	case "exits-clean":
		banner()
		up()
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	case "ignores-interrupt":
		banner()
		up()
		signal.Ignore(os.Interrupt)
		os.WriteFile(os.Getenv("FAKE_PIDFILE"), []byte(fmt.Sprint(os.Getpid())), 0o600)
		time.Sleep(time.Hour)
	case "split":
		// A write need not end where a line does.
		fmt.Fprint(os.Stderr, "INF |  https://split-across-writes-fake.try")
		time.Sleep(20 * time.Millisecond)
		fmt.Fprintln(os.Stderr, "cloudflare.com  |")
		up()
		time.Sleep(time.Hour)
	case "no-address":
		up()
		time.Sleep(time.Hour)
	case "no-connection":
		banner()
		time.Sleep(time.Hour)
	case "exit-early":
		fmt.Fprintln(os.Stderr, "ERR config.yml found: a quick tunnel cannot run with one")
		os.Exit(1)
	case "dies-later":
		banner()
		up()
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintln(os.Stderr, "ERR connection lost")
		os.Exit(3)
	}
}

// start runs Start against the stand-in for the given case.
func start(t *testing.T, mode string, timeout time.Duration) (*Tunnel, error) {
	t.Helper()
	t.Setenv("FAKE_CLOUDFLARED", mode)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return Start(ctx, "http://127.0.0.1:1", Options{Binary: os.Args[0], Timeout: timeout})
}

func TestStartReturnsTheAddressOnceTheTunnelCarriesTraffic(t *testing.T) {
	tunnel, err := start(t, "ok", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()

	if tunnel.URL != "https://how-eminem-festival-launched.trycloudflare.com" {
		t.Errorf("URL = %q", tunnel.URL)
	}
}

func TestAnAddressSplitAcrossWritesIsStillFound(t *testing.T) {
	tunnel, err := start(t, "split", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()

	if tunnel.URL != "https://split-across-writes-fake.trycloudflare.com" {
		t.Errorf("URL = %q", tunnel.URL)
	}
}

func TestTheAddressAloneIsNotEnough(t *testing.T) {
	// It is printed seconds before anything can reach it, and handing it out
	// then would send the first request to a tunnel that is not there.
	_, err := start(t, "no-connection", 300*time.Millisecond)

	if err == nil || !strings.Contains(err.Error(), "did not come up") {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestAConnectionWithNoAddressIsNotEnoughEither(t *testing.T) {
	_, err := start(t, "no-address", 300*time.Millisecond)

	if err == nil || !strings.Contains(err.Error(), "did not come up") {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestAFailureSaysWhatCloudflaredSaid(t *testing.T) {
	_, err := start(t, "exit-early", 10*time.Second)

	if err == nil || !strings.Contains(err.Error(), "a quick tunnel cannot run with one") {
		t.Errorf("err = %v, want cloudflared's own words", err)
	}
}

func TestAMissingCloudflaredIsNamed(t *testing.T) {
	_, err := Start(context.Background(), "http://127.0.0.1:1",
		Options{Binary: "cloudflared-that-is-not-installed"})

	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("err = %v, want ErrNotInstalled", err)
	}
}

func TestStoppingTheTunnelStopsCloudflared(t *testing.T) {
	tunnel, err := start(t, "ok", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	closed := make(chan struct{})
	go func() { tunnel.Close(); close(closed) }()

	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("cloudflared was not stopped")
	}
	if tunnel.Err() != nil {
		t.Errorf("a tunnel stopped on purpose reported %v", tunnel.Err())
	}
}

func TestCancellingTheContextStopsCloudflared(t *testing.T) {
	t.Setenv("FAKE_CLOUDFLARED", "ok")
	ctx, cancel := context.WithCancel(context.Background())
	tunnel, err := Start(ctx, "http://127.0.0.1:1", Options{Binary: os.Args[0], Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	cancel()

	select {
	case <-tunnel.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("cloudflared outlived the context")
	}
	if tunnel.Err() != nil {
		t.Errorf("err = %v, want nil: the context ending is not a failure", tunnel.Err())
	}
}

func TestATunnelThatDiesLaterSaysSo(t *testing.T) {
	// The address goes with it, so the host has to be told.
	tunnel, err := start(t, "dies-later", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-tunnel.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the tunnel's end was not noticed")
	}
	if tunnel.Err() == nil || !strings.Contains(tunnel.Err().Error(), "connection lost") {
		t.Errorf("err = %v, want it to carry cloudflared's last words", tunnel.Err())
	}
}

// recordedRun starts the stand-in and waits until it has recorded how it was
// run. Entries in caller are given as the whole environment, on top of this
// process's own; none given leaves the choice to Start.
func recordedRun(t *testing.T, caller ...string) (record string, config string, tunnel *Tunnel) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "record")
	t.Setenv("FAKE_RECORD", file)
	t.Setenv("FAKE_CLOUDFLARED", "args")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	options := Options{Binary: os.Args[0], Timeout: 10 * time.Second}
	if len(caller) > 0 {
		options.Env = append(os.Environ(), caller...)
	}
	tunnel, err := Start(ctx, "http://127.0.0.1:1", options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tunnel.Close)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if raw, err := os.ReadFile(file); err == nil && strings.Contains(string(raw), "---") {
			size, _ := os.ReadFile(file + ".config")
			return string(raw), string(size), tunnel
		}
	}
	t.Fatal("the stand-in never recorded how it was run")
	return "", "", nil
}

func TestCloudflaredIsPointedAtTheTargetAndAtNoConfigFileOfItsOwn(t *testing.T) {
	// A config.yml in one of cloudflared's default places carries ingress
	// rules that replace --url: the tunnel comes up and publishes another
	// service. Giving it a file of ours that routes nothing is what stops that.
	record, config, tunnel := recordedRun(t)

	arguments := strings.Split(strings.SplitN(record, "\n---\n", 2)[0], "\n")
	if len(arguments) != 6 || arguments[0] != "tunnel" || arguments[1] != "--config" ||
		arguments[3] != "--no-autoupdate" || arguments[4] != "--url" || arguments[5] != "http://127.0.0.1:1" {
		t.Errorf("cloudflared was run with %q", arguments)
	}
	if config != "no-autoupdate: true\n" || strings.Contains(config, "ingress") {
		t.Errorf("the configuration file it was given held %q, want one that routes nothing", config)
	}

	path := arguments[2]
	tunnel.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the configuration file was left behind: %v", err)
	}
}

func TestCloudflaredsOwnEnvironmentSettingsAreNotPassedOn(t *testing.T) {
	// TUNNEL_NAME, TUNNEL_URL and the like are flags in the form of variables.
	// One in the person's shell could turn the quick tunnel into something else.
	t.Setenv("TUNNEL_NAME", "somebody-elses-tunnel")
	t.Setenv("KEEP_ME", "kept")

	record, _, _ := recordedRun(t)

	if !strings.HasSuffix(record, "---\n|kept") {
		t.Errorf("record = %q, want TUNNEL_NAME gone and the rest of the environment kept", record)
	}
}

func TestAnEnvironmentOfTheCallersChoosingIsUsedInstead(t *testing.T) {
	t.Setenv("KEEP_ME", "from this process")

	record, _, _ := recordedRun(t, "KEEP_ME=from the caller", "TUNNEL_NAME=x")

	if !strings.HasSuffix(record, "---\n|from the caller") {
		t.Errorf("record = %q", record)
	}
}

func TestStoppingTheTunnelLetsCloudflaredShutDownFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("an interrupt cannot be sent there; the process is killed instead")
	}
	marker := filepath.Join(t.TempDir(), "marker")
	t.Setenv("FAKE_MARKER", marker)
	tunnel, err := start(t, "ok", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // let the stand-in install its handler

	tunnel.Close()

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("cloudflared was not interrupted: %v", err)
	}
	select {
	case <-tunnel.Done():
	default:
		t.Error("Close returned while cloudflared was still running")
	}
}

func TestCloudflaredThatIgnoresTheInterruptIsKilled(t *testing.T) {
	previous := killAfter
	killAfter = 300 * time.Millisecond
	t.Cleanup(func() { killAfter = previous })
	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("FAKE_PIDFILE", pidFile)
	tunnel, err := start(t, "ignores-interrupt", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && pid == 0; time.Sleep(10 * time.Millisecond) {
		if raw, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
		}
	}
	if pid == 0 {
		t.Fatal("the stand-in never recorded its pid")
	}

	closed := make(chan struct{})
	go func() { tunnel.Close(); close(closed) }()

	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close waited for a cloudflared that ignores the interrupt")
	}
	// Not just that Close returned: the host gives up on a guard that does not
	// finish, and what the guard failed to stop would be left publishing.
	if alive(pid) {
		t.Errorf("cloudflared %d ignored the interrupt and was not killed", pid)
	}
}

func TestACleanExitIsStillALostTunnel(t *testing.T) {
	// The address goes with it whatever the exit status.
	tunnel, err := start(t, "exits-clean", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-tunnel.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the tunnel's end was not noticed")
	}
	if tunnel.Err() == nil {
		t.Error("cloudflared exiting with status 0 took the address with it but reported no error")
	}
}

func TestCancellingWhileWaitingReportsTheCancellation(t *testing.T) {
	t.Setenv("FAKE_CLOUDFLARED", "no-connection")
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()

	_, err := Start(ctx, "http://127.0.0.1:1", Options{Binary: os.Args[0], Timeout: time.Minute})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the cancellation", err)
	}
}

func TestOnlyTheTunnelAddressIsTakenFromTheFirstLine(t *testing.T) {
	// The long first line holds cloudflare.com and developers.cloudflare.com
	// addresses. None of them is the tunnel.
	tunnel, err := start(t, "ok", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()

	if !strings.HasSuffix(tunnel.URL, ".trycloudflare.com") || strings.Contains(tunnel.URL, "cloudflare.com/") {
		t.Errorf("URL = %q", tunnel.URL)
	}
}
