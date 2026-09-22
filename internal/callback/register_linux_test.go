//go:build linux

package callback_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/callback"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// isolatedHome points the XDG data directory somewhere a test owns, so
// registering does not write into the developer's own desktop.
func isolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("HOME", dir)
	return dir
}

func TestRegisteringWritesADBusActivatableEntry(t *testing.T) {
	// The whole security argument rests on this line. Without
	// DBusActivatable, the desktop starts the handler as a command and the
	// URL arrives as an argument — where /proc/<pid>/cmdline exposes the
	// Moodle token to every other user on the machine.
	data := isolatedHome(t)
	reg, err := callback.Register("moodle-cli-test", "/opt/moodle/bin/moodle")
	if err != nil {
		t.Fatal(err)
	}

	entry, err := os.ReadFile(reg.DesktopFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(entry)
	if !strings.Contains(text, "DBusActivatable=true") {
		t.Errorf("the desktop entry does not ask for D-Bus activation:\n%s", text)
	}
	if !strings.Contains(text, "MimeType=x-scheme-handler/moodle-cli-test;") {
		t.Errorf("the entry does not claim the scheme:\n%s", text)
	}
	if !strings.Contains(text, "NoDisplay=true") {
		t.Errorf("a handler should not appear in application menus:\n%s", text)
	}
	if !strings.HasPrefix(reg.DesktopFile, data) {
		t.Errorf("wrote outside the data directory: %s", reg.DesktopFile)
	}

	service, err := os.ReadFile(reg.ServiceFile)
	if err != nil {
		t.Fatalf("no D-Bus service file, so the bus cannot start the handler: %v", err)
	}
	if !strings.Contains(string(service), `"/opt/moodle/bin/moodle" auth callback --scheme moodle-cli-test`) {
		t.Errorf("the service file does not start this program:\n%s", service)
	}
}

func TestTheMobileAppsSchemeIsNeverTaken(t *testing.T) {
	// A site can send its students to the official Moodle app by forcing that
	// scheme. Registering it here would take those logins away from the app.
	isolatedHome(t)
	_, err := callback.Register("moodlemobile", "/opt/moodle/bin/moodle")
	if err == nil {
		t.Fatal("the Moodle app's own scheme was claimed")
	}
	if !strings.Contains(errs.From(err).Hint, "app") {
		t.Errorf("the refusal does not explain the harm: %v", err)
	}
}

func TestTheMobileAppsSchemeIsCaseInsensitive(t *testing.T) {
	isolatedHome(t)
	if _, err := callback.Register("MoodleMobile", "/opt/moodle/bin/moodle"); err == nil {
		t.Fatal("a differently cased spelling claimed the Moodle app's scheme")
	}
}

func TestASchemeMoodleWouldRefuseIsNotRegistered(t *testing.T) {
	// Moodle validates the scheme before it redirects, so registering one it
	// will not use leaves a handler that can never fire.
	isolatedHome(t)
	for _, scheme := range []string{"", "1nvalid", "has space", "under_score"} {
		if _, err := callback.Register(scheme, "/opt/moodle/bin/moodle"); err == nil {
			t.Errorf("registered %q, which Moodle will not redirect to", scheme)
		}
	}
}

func TestARelativeExecutableIsRefused(t *testing.T) {
	// A desktop entry is read long after the shell that wrote it, from a
	// different working directory.
	isolatedHome(t)
	if _, err := callback.Register("moodle-cli-test", "./moodle"); err == nil {
		t.Fatal("a relative path was written into a desktop entry")
	}
}

func TestRegistrationNamesDoNotCollapseDistinctSchemes(t *testing.T) {
	isolatedHome(t)
	first, err := callback.Register("moodle-cli-test", "/opt/moodle/bin/moodle")
	if err != nil {
		t.Fatal(err)
	}
	second, err := callback.Register("moodleclitest", "/opt/moodle/bin/moodle")
	if err != nil {
		t.Fatal(err)
	}
	if first.DesktopFile == second.DesktopFile || first.ServiceFile == second.ServiceFile {
		t.Fatalf("distinct schemes share registration files: %+v / %+v", first, second)
	}
}

func TestAnExecutablePathWithSpacesIsQuotedAndReportedExactly(t *testing.T) {
	isolatedHome(t)
	want := "/opt/Moodle CLI/bin/moodle"
	reg, err := callback.Register("moodle-cli-test", want)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(reg.ServiceFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `Exec="/opt/Moodle CLI/bin/moodle" auth callback`) {
		t.Errorf("service command does not quote the executable:\n%s", raw)
	}
	if got := callback.Status("moodle-cli-test").Executable; got != want {
		t.Errorf("status executable = %q, want %q", got, want)
	}
}

func TestStatusReportsWhatIsInstalled(t *testing.T) {
	isolatedHome(t)
	if callback.Status("moodle-cli-test").Installed() {
		t.Fatal("reported installed before anything was written")
	}
	if _, err := callback.Register("moodle-cli-test", "/opt/moodle/bin/moodle"); err != nil {
		t.Fatal(err)
	}
	status := callback.Status("moodle-cli-test")
	if !status.Installed() {
		t.Error("reported not installed after registering")
	}
	if status.Executable != "/opt/moodle/bin/moodle" {
		t.Errorf("executable = %q; status should show which build is registered",
			status.Executable)
	}
}

func TestUnregisteringRemovesOnlyWhatWasWritten(t *testing.T) {
	data := isolatedHome(t)
	other := filepath.Join(data, "applications", "someone-elses.desktop")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := callback.Register("moodle-cli-test", "/opt/moodle/bin/moodle"); err != nil {
		t.Fatal(err)
	}
	if err := callback.Unregister("moodle-cli-test"); err != nil {
		t.Fatal(err)
	}
	if callback.Status("moodle-cli-test").Installed() {
		t.Error("registration survived unregistering")
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("another application's entry was removed: %v", err)
	}
}

func TestUnregisteringWhatWasNeverThereIsNotAnError(t *testing.T) {
	isolatedHome(t)
	if err := callback.Unregister("moodle-cli-test"); err != nil {
		t.Errorf("removing nothing reported a failure: %v", err)
	}
}
