//go:build darwin

package callback

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This exercises the real macOS URL event, applet, NSTask and stdin pipe.
// It is opt-in because it launches a native application in the desktop session.
func TestNativeMacOSURLHandler(t *testing.T) {
	if os.Getenv("MOODLE_CLI_TEST_APPLE_EVENT") != "1" {
		t.Skip("set MOODLE_CLI_TEST_APPLE_EVENT=1 to exercise native Launch Services")
	}
	dir := t.TempDir()
	scheme := fmt.Sprintf("moodle-cli-test-%d", time.Now().UnixNano())
	executable := filepath.Join(dir, "receiver with spaces")
	output := filepath.Join(dir, "callback")
	args := filepath.Join(dir, "arguments")
	// Only synthetic test data is written. The production helper pipes the
	// callback into the CLI, which verifies it and stores it in the keychain.
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + args + "'\ncat > '" + output + "'\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	reg, err := Register(scheme, executable)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Unregister(scheme); err != nil {
			t.Error(err)
		}
	})
	if !reg.Installed() || reg.Executable != executable {
		t.Fatalf("registration = %+v", reg)
	}
	if _, err := Register(scheme, executable); err != nil {
		t.Fatalf("idempotent registration: %v", err)
	}
	uri := scheme + "://token=synthetic-callback"
	if out, err := exec.Command("/usr/bin/open", "-g", uri).CombinedOutput(); err != nil {
		t.Fatalf("native URL open: %v: %s", err, out)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got, err := os.ReadFile(output); err == nil && string(got) == uri {
			gotArgs, err := os.ReadFile(args)
			if err != nil {
				t.Fatal(err)
			}
			if want := "auth\ncallback\n--scheme\n" + scheme + "\n"; string(gotArgs) != want {
				t.Fatalf("handler arguments = %q", gotArgs)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the native handler did not deliver the callback through stdin")
}

func TestMacOSRegistrationPreservesUnrelatedApplications(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := registrationPaths("moodle-cli-test")
	if err := os.MkdirAll(reg.DesktopFile, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(reg.DesktopFile, "unrelated")
	if err := os.WriteFile(file, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Unregister(reg.Scheme); err == nil {
		t.Fatal("unrelated application accepted for removal")
	}
	if got, err := os.ReadFile(file); err != nil || strings.TrimSpace(string(got)) != "keep" {
		t.Fatalf("unrelated file = %q, %v", got, err)
	}
}
