//go:build linux

package callback

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Registration on Linux, through the desktop entry specification.
//
// The desktop file says two things: which scheme this handles, and that it
// should be started over D-Bus rather than by running a command. The second
// is the important one. A handler started as a command receives the URL on
// its command line, and /proc/<pid>/cmdline is readable by other users on an
// ordinary system — so the Moodle token would be world-readable for as long
// as the process lived. Through D-Bus the URI is a message parameter and
// never appears there.
//
// A desktop that cannot activate over D-Bus therefore gets no automatic
// callback at all. That is deliberate: this refuses rather than falling back
// to a way of working it would have to describe as insecure.

// busName is what this claims on the session bus. It is derived from the
// scheme so that two builds registering different schemes do not collide.
func busName(scheme string) string {
	return "org.moodlecli." + strings.NewReplacer("-", "", ".", "", "+", "").Replace(scheme)
}

func objectPath(scheme string) string {
	return "/" + strings.ReplaceAll(busName(scheme), ".", "/")
}

func desktopFileName(scheme string) string { return busName(scheme) + ".desktop" }

// Registration says what is installed and where, so a status command can
// show it and an uninstall can undo exactly it.
type Registration struct {
	Scheme      string
	DesktopFile string
	ServiceFile string
	Executable  string
	// MIMEDefault is what the desktop says handles this scheme, which may be
	// something else entirely.
	MIMEDefault string
}

func dataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

// Register installs the per-user handler for a scheme.
//
// Per-user, never system-wide: this needs no elevation and touches nothing
// belonging to anyone else.
func Register(scheme, executable string) (Registration, error) {
	if err := CheckScheme(scheme); err != nil {
		return Registration{}, err
	}
	data := dataHome()
	if data == "" {
		return Registration{}, errs.New(errs.CodeUnavailable,
			"cannot find where to install a desktop entry")
	}
	if !filepath.IsAbs(executable) {
		return Registration{}, errs.New(errs.CodeUsage,
			"the handler needs this program's full path").
			WithHint("a desktop entry is read long after the shell that made it")
	}

	applications := filepath.Join(data, "applications")
	services := filepath.Join(data, "dbus-1", "services")
	for _, dir := range []string{applications, services} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Registration{}, errs.Wrap(errs.CodeUnavailable, err,
				"cannot create "+dir)
		}
	}

	reg := Registration{
		Scheme:      scheme,
		DesktopFile: filepath.Join(applications, desktopFileName(scheme)),
		ServiceFile: filepath.Join(services, busName(scheme)+".service"),
		Executable:  executable,
	}

	if err := os.WriteFile(reg.DesktopFile, []byte(desktopEntry(scheme, executable)), 0o644); err != nil {
		return Registration{}, errs.Wrap(errs.CodeUnavailable, err,
			"cannot write the desktop entry")
	}
	if err := os.WriteFile(reg.ServiceFile, []byte(serviceEntry(scheme, executable)), 0o644); err != nil {
		return Registration{}, errs.Wrap(errs.CodeUnavailable, err,
			"cannot write the D-Bus service file")
	}

	// Best effort, both of them. The cache and the association are the
	// desktop's to keep, and a desktop that does not use them is not broken —
	// so a missing tool is reported in the status rather than as a failure.
	_ = run("update-desktop-database", applications)
	_ = run("xdg-mime", "default", desktopFileName(scheme), "x-scheme-handler/"+scheme)
	reg.MIMEDefault = mimeDefault(scheme)
	return reg, nil
}

// desktopEntry is the file the desktop reads.
//
// NoDisplay keeps it out of application menus: this is a handler, not
// something anyone should launch. DBusActivatable is the load-bearing line —
// see the note at the top of this file.
func desktopEntry(scheme, executable string) string {
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Moodle CLI sign-in
Comment=Receives the sign-in result from your browser
Exec=%s auth callback
NoDisplay=true
Terminal=false
DBusActivatable=true
MimeType=x-scheme-handler/%s;
`, executable, scheme)
}

// serviceEntry lets the bus start the handler on demand, so nothing has to be
// running before the browser sends the callback.
func serviceEntry(scheme, executable string) string {
	return fmt.Sprintf(`[D-BUS Service]
Name=%s
Exec=%s auth callback
`, busName(scheme), executable)
}

// Status reports what is installed for a scheme, and what the desktop
// actually associates with it — which may be another application entirely.
func Status(scheme string) Registration {
	data := dataHome()
	reg := Registration{
		Scheme:      scheme,
		DesktopFile: filepath.Join(data, "applications", desktopFileName(scheme)),
		ServiceFile: filepath.Join(data, "dbus-1", "services", busName(scheme)+".service"),
		MIMEDefault: mimeDefault(scheme),
	}
	if raw, err := os.ReadFile(reg.DesktopFile); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if rest, ok := strings.CutPrefix(line, "Exec="); ok {
				reg.Executable = strings.TrimSuffix(strings.TrimSpace(rest), " auth callback")
			}
		}
	}
	return reg
}

// Installed reports whether both files this writes are present.
func (r Registration) Installed() bool {
	for _, path := range []string{r.DesktopFile, r.ServiceFile} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// Unregister removes only what Register wrote.
func Unregister(scheme string) error {
	reg := Status(scheme)
	for _, path := range []string{reg.DesktopFile, reg.ServiceFile} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return errs.Wrap(errs.CodeUnavailable, err, "cannot remove "+path)
		}
	}
	_ = run("update-desktop-database", filepath.Join(dataHome(), "applications"))
	return nil
}

// mimeDefault asks the desktop what it would open this scheme with. An empty
// answer means it could not be asked, which is not the same as nothing.
func mimeDefault(scheme string) string {
	out, err := exec.Command("xdg-mime", "query", "default",
		"x-scheme-handler/"+scheme).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func run(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	return exec.Command(path, args...).Run()
}
