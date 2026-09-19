//go:build linux

package callback_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/KoukeNeko/moodle-cli/internal/callback"
)

// needsBus skips when there is no session bus. A security test that quietly
// does nothing is worse than none, so it says why.
func needsBus(t *testing.T) {
	t.Helper()
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus; run under dbus-run-session")
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		t.Skipf("no session bus: %v", err)
	}
	_ = conn
}

func TestACallbackArrivesOverTheBusAndNeverOnACommandLine(t *testing.T) {
	// This is the whole reason for D-Bus activation. A handler started as a
	// command receives the URL as an argument, and /proc/<pid>/cmdline is
	// readable by other users on an ordinary system — so the Moodle token
	// would be world-readable for as long as that process lived. As a bus
	// message parameter it never goes near a command line.
	needsBus(t)
	dir := runtimeDir(t)

	listener, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	scheme := "moodle-cli-test"
	served := make(chan error, 1)
	go func() {
		served <- callback.ServeCallback(context.Background(), scheme, dir, 10*time.Second)
	}()
	// Let the handler claim its name before the desktop would call it.
	time.Sleep(300 * time.Millisecond)

	want := "moodle-cli-test://token=" + strings.Repeat("A", 40)
	conn, err := dbus.SessionBus()
	if err != nil {
		t.Fatal(err)
	}
	object := conn.Object(callback.BusNameFor(scheme),
		dbus.ObjectPath(callback.ObjectPathFor(scheme)))
	call := object.Call("org.freedesktop.Application.Open", 0,
		[]string{want}, map[string]dbus.Variant{})
	if call.Err != nil {
		t.Fatalf("the desktop's own call failed: %v", call.Err)
	}

	got, err := callback.Receive(listener, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != want {
		t.Errorf("received %q, want %q", strings.TrimSpace(got), want)
	}
	if err := <-served; err != nil {
		t.Errorf("the handler reported: %v", err)
	}

	// And the proof for the claim above: this process's own command line,
	// which is what /proc exposes, never held it.
	cmdline, err := os.ReadFile("/proc/self/cmdline")
	if err == nil && strings.Contains(string(cmdline), "token=") {
		t.Error("the callback reached a command line after all")
	}
}

func TestTwoHandlersCannotBothReceive(t *testing.T) {
	// Only one process may own the name. A second silently accepting
	// callbacks would be a second process holding a credential.
	needsBus(t)
	dir := runtimeDir(t)
	scheme := "moodle-cli-test-two"

	first := make(chan error, 1)
	go func() {
		first <- callback.ServeCallback(context.Background(), scheme, dir, 3*time.Second)
	}()
	time.Sleep(300 * time.Millisecond)

	err := callback.ServeCallback(context.Background(), scheme, dir, time.Second)
	if err == nil {
		t.Fatal("a second handler claimed the same callbacks")
	}
	if !strings.Contains(err.Error(), "already receiving") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	<-first
}

func TestACallbackWithNobodyWaitingIsNotSwallowed(t *testing.T) {
	// If the waiting process is gone, saying so is the whole answer. A
	// handler that accepted it quietly would leave a user watching a browser
	// that has already finished.
	needsBus(t)
	err := callback.Deliver(runtimeDir(t), "moodle-cli-test://token=AAAA")
	if err == nil {
		t.Fatal("a callback with no listener was reported as delivered")
	}
	if !strings.Contains(err.Error(), "no sign-in is waiting") {
		t.Errorf("message = %v", err)
	}
}

func TestTheChannelIsNotReadIntoUnboundedMemory(t *testing.T) {
	// The other end of this socket is a process, and a process can send
	// anything. Reading until it stops would be reading whatever it likes.
	needsBus(t)
	dir := runtimeDir(t)
	listener, err := callback.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		path := filepath.Join(dir, "moodle-cli", "auth.sock")
		conn, err := dialUnix(path)
		if err != nil {
			return
		}
		defer conn.Close()
		// A megabyte, from something claiming to be the handler.
		_, _ = conn.Write([]byte(strings.Repeat("x", 1<<20)))
	}()

	got, err := callback.Receive(listener, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 16<<10 {
		t.Errorf("read %d bytes from the channel", len(got))
	}
}

func dialUnix(path string) (interface {
	Write([]byte) (int, error)
	Close() error
}, error) {
	return net.Dial("unix", path)
}
