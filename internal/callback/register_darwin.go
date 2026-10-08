//go:build darwin

package callback

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

const launchServicesRegister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

func bundleID(scheme string) string {
	return "org.moodlecli.Handler.s" + hex.EncodeToString([]byte(strings.ToLower(scheme)))
}

func registrationPaths(scheme string) Registration {
	home, err := os.UserHomeDir()
	if err != nil {
		return Registration{Scheme: scheme}
	}
	bundle := filepath.Join(home, "Applications", "Moodle CLI Sign In "+strings.ToLower(scheme)+".app")
	return Registration{
		Scheme: scheme, DesktopFile: bundle,
		ServiceFile: filepath.Join(bundle, "Contents", "Resources", "Scripts", "main.scpt"),
	}
}

func (r Registration) Installed() bool {
	if r.Executable == "" || r.MIMEDefault != filepath.Base(r.DesktopFile) {
		return false
	}
	for _, path := range []string{r.Executable, r.ServiceFile, filepath.Join(r.DesktopFile, "Contents", "MacOS", "applet")} {
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func Status(scheme string) Registration {
	reg := registrationPaths(scheme)
	if reg.DesktopFile == "" {
		return reg
	}
	if raw, err := os.ReadFile(filepath.Join(reg.DesktopFile, "Contents", "Resources", "moodle-cli-executable")); err == nil {
		reg.Executable = string(raw)
	}
	if path, err := defaultApplication(scheme); err == nil && path != "" {
		// The CLI's platform-neutral status contract compares the entry's
		// basename with MIMEDefault, just as it does for a Linux desktop file.
		if filepath.Clean(path) == filepath.Clean(reg.DesktopFile) {
			reg.MIMEDefault = filepath.Base(reg.DesktopFile)
		} else {
			reg.MIMEDefault = path
		}
	}
	return reg
}

// Register builds a small native AppleScript application using macOS's own
// compiler. A downloaded CLI needs neither Xcode nor a separate helper binary.
func Register(scheme, executable string) (registered Registration, retErr error) {
	if err := CheckScheme(scheme); err != nil {
		return Registration{}, err
	}
	if !filepath.IsAbs(executable) || strings.ContainsAny(executable, "\r\n") {
		return Registration{}, errs.New(errs.CodeUsage, "the handler needs this program's full path without line breaks")
	}
	if info, err := os.Stat(executable); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return Registration{}, errs.New(errs.CodeUsage, "the handler program is not executable")
	}
	reg := registrationPaths(scheme)
	if reg.DesktopFile == "" {
		return Registration{}, errs.New(errs.CodeUnavailable, "cannot locate your Applications folder")
	}
	current, err := defaultApplication(scheme)
	if err != nil {
		return Registration{}, err
	}
	if current != "" && filepath.Clean(current) != filepath.Clean(reg.DesktopFile) {
		return Registration{}, errs.New(errs.CodeConflict, "another application already handles "+scheme)
	}
	if existing := Status(scheme); existing.Installed() && existing.Executable == executable {
		return existing, nil
	}
	if _, err := os.Lstat(reg.DesktopFile); err == nil {
		if !ownedBundle(reg) {
			return Registration{}, errs.New(errs.CodeConflict, "the handler application path is already in use")
		}
	} else if !os.IsNotExist(err) {
		return Registration{}, err
	}
	apps := filepath.Dir(reg.DesktopFile)
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return Registration{}, err
	}
	stage, err := os.MkdirTemp(apps, ".moodle-cli-handler-")
	if err != nil {
		return Registration{}, err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(stage)
		}
	}()
	bundle := filepath.Join(stage, filepath.Base(reg.DesktopFile))
	compiler := exec.Command("/usr/bin/osacompile", "-o", bundle)
	compiler.Stdin = strings.NewReader(handlerScript(scheme, executable))
	if output, err := compiler.CombinedOutput(); err != nil {
		return Registration{}, errs.Wrap(errs.CodeUnavailable, err, "cannot build the browser handler: "+strings.TrimSpace(string(output)))
	}
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	types, _ := json.Marshal([]map[string]any{{"CFBundleTypeRole": "Viewer", "CFBundleURLName": bundleID(scheme), "CFBundleURLSchemes": []string{scheme}}})
	for _, args := range [][]string{
		{"-replace", "CFBundleIdentifier", "-string", bundleID(scheme), plist},
		{"-replace", "LSUIElement", "-bool", "YES", plist},
		{"-replace", "CFBundleURLTypes", "-json", string(types), plist},
	} {
		if out, err := exec.Command("/usr/bin/plutil", args...).CombinedOutput(); err != nil {
			return Registration{}, errs.Wrap(errs.CodeUnavailable, err, "cannot configure the browser handler: "+strings.TrimSpace(string(out)))
		}
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Resources", "moodle-cli-executable"), []byte(executable), 0o600); err != nil {
		return Registration{}, err
	}
	// Editing Info.plist invalidates the applet's original signature. This
	// locally compiled helper is signed ad hoc, not a release artifact.
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", bundle).CombinedOutput(); err != nil {
		return Registration{}, errs.Wrap(errs.CodeUnavailable, err, "cannot sign the browser handler: "+strings.TrimSpace(string(out)))
	}
	backup := filepath.Join(stage, "previous.app")
	hadPrevious := false
	if _, err := os.Lstat(reg.DesktopFile); err == nil {
		if err := os.Rename(reg.DesktopFile, backup); err != nil {
			return Registration{}, err
		}
		hadPrevious = true
	}
	if err := os.Rename(bundle, reg.DesktopFile); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backup, reg.DesktopFile); restoreErr != nil {
				keepBackup = true
				return Registration{}, fmt.Errorf("install handler: %v; restore previous handler: %w", err, restoreErr)
			}
		}
		return Registration{}, err
	}
	defer func() {
		if retErr == nil {
			return
		}
		if err := exec.Command(launchServicesRegister, "-u", reg.DesktopFile).Run(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("unregister failed handler: %w", err))
		}
		if err := os.RemoveAll(reg.DesktopFile); err != nil {
			keepBackup = hadPrevious
			retErr = errors.Join(retErr, fmt.Errorf("remove failed handler: %w", err))
			return
		}
		if hadPrevious {
			if err := os.Rename(backup, reg.DesktopFile); err != nil {
				keepBackup = true
				retErr = errors.Join(retErr, fmt.Errorf("restore previous handler from %s: %w", backup, err))
				return
			}
			if err := exec.Command(launchServicesRegister, "-f", reg.DesktopFile).Run(); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("register restored handler: %w", err))
			}
		}
	}()
	if out, err := exec.Command(launchServicesRegister, "-f", reg.DesktopFile).CombinedOutput(); err != nil {
		return Registration{}, errs.Wrap(errs.CodeUnavailable, err, "cannot register the browser handler: "+strings.TrimSpace(string(out)))
	}
	quotedScheme, _ := json.Marshal(scheme)
	quotedID, _ := json.Marshal(bundleID(scheme))
	associate := exec.Command("/usr/bin/osascript", "-l", "JavaScript")
	associate.Stdin = strings.NewReader(`ObjC.import("CoreServices"); var status = $.LSSetDefaultHandlerForURLScheme($(` + string(quotedScheme) + `), $(` + string(quotedID) + `)); if (status !== 0) throw Error("Launch Services status " + status);`)
	if out, err := associate.CombinedOutput(); err != nil {
		return Registration{}, errs.Wrap(errs.CodeUnavailable, err, "cannot associate the browser handler: "+strings.TrimSpace(string(out)))
	}
	reg = Status(scheme)
	if !reg.Installed() {
		return Registration{}, errs.New(errs.CodeUnavailable, fmt.Sprintf("macOS did not associate the browser callback with this handler (default: %q)", reg.MIMEDefault))
	}
	return reg, nil
}

func ownedBundle(reg Registration) bool {
	info, err := os.Lstat(reg.DesktopFile)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	out, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", filepath.Join(reg.DesktopFile, "Contents", "Info.plist")).Output()
	return err == nil && strings.TrimSpace(string(out)) == bundleID(reg.Scheme)
}

func Unregister(scheme string) error {
	if err := CheckScheme(scheme); err != nil {
		return err
	}
	reg := registrationPaths(scheme)
	if reg.DesktopFile == "" {
		return errs.New(errs.CodeUnavailable, "cannot locate your Applications folder")
	}
	if _, err := os.Lstat(reg.DesktopFile); os.IsNotExist(err) {
		return nil
	}
	if !ownedBundle(reg) {
		return errs.New(errs.CodeConflict, "refusing to remove an application this tool did not install")
	}
	if out, err := exec.Command(launchServicesRegister, "-u", reg.DesktopFile).CombinedOutput(); err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot unregister the browser handler: "+strings.TrimSpace(string(out)))
	}
	return os.RemoveAll(reg.DesktopFile)
}

func defaultApplication(scheme string) (string, error) {
	quoted, _ := json.Marshal(scheme + "://probe")
	script := `ObjC.import("AppKit"); var u = $.NSWorkspace.sharedWorkspace.URLForApplicationToOpenURL($.NSURL.URLWithString(` + string(quoted) + `)); if (u && !u.isNil()) ObjC.unwrap(u.path);`
	cmd := exec.Command("/usr/bin/osascript", "-l", "JavaScript")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		return "", errs.Wrap(errs.CodeUnavailable, err, "cannot ask macOS which application handles the callback")
	}
	return strings.TrimSpace(string(out)), nil
}

func appleScriptString(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func handlerScript(scheme, executable string) string {
	// NSTask gets the URI over an anonymous pipe. Neither the token nor a
	// shell command containing it appears in argv, logs, or a temporary file.
	return fmt.Sprintf(`use framework "Foundation"
use scripting additions

on open location callbackURL
    if callbackURL does not start with %s then error "Unexpected callback scheme"
    if (count callbackURL) > %d then error "Callback is too large"
    set task to current application's NSTask's alloc()'s init()
    task's setLaunchPath:%s
    task's setArguments:{"auth", "callback", "--scheme", %s}
    set inputPipe to current application's NSPipe's pipe()
    task's setStandardInput:inputPipe
    set callbackData to (current application's NSString's stringWithString:callbackURL)'s dataUsingEncoding:(current application's NSUTF8StringEncoding)
    task's |launch|()
    (inputPipe's fileHandleForWriting())'s writeData:callbackData
    (inputPipe's fileHandleForWriting())'s closeFile()
    task's waitUntilExit()
    if (task's terminationStatus()) is not 0 then error "No waiting sign-in accepted the browser callback"
end open location
`, appleScriptString(scheme+"://"), maxCallbackURI, appleScriptString(executable), appleScriptString(scheme))
}
