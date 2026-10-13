package browser_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/browser"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// realProfilesINI is the file Firefox 156 wrote on this machine, unedited.
func realProfilesINI(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/profiles.ini")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// firefoxHome lays out a home directory the way Firefox 156 does.
func firefoxHome(t *testing.T, content string) (home, configHome string) {
	t.Helper()
	home = t.TempDir()
	configHome = filepath.Join(home, ".config")
	root := filepath.Join(configHome, "mozilla", "firefox")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "profiles.ini"),
		[]byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return home, configHome
}

func TestProfilesAreFoundWhereFirefoxActuallyPutsThem(t *testing.T) {
	// Firefox 156 uses the XDG configuration directory. Every tutorial, and
	// this project's own plan, said ~/.mozilla/firefox — measured here: a
	// fresh Firefox created ~/.config/mozilla/firefox and never touched the
	// other one at all.
	home, configHome := firefoxHome(t, realProfilesINI(t))
	profiles, err := browser.FirefoxProfiles(home, configHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want the 2 in the recorded file: %+v", len(profiles), profiles)
	}
}

func TestTheInstallsDefaultWinsOverTheOlderMarker(t *testing.T) {
	// The recorded file has both, and they disagree: [Profile1] carries
	// Default=1 while the install section names default-release. Firefox
	// opens the install's choice, so picking the other would read the wrong
	// profile's cookies and report "not signed in" for a site the user is
	// signed in to.
	home, configHome := firefoxHome(t, realProfilesINI(t))
	profiles, err := browser.FirefoxProfiles(home, configHome)
	if err != nil {
		t.Fatal(err)
	}
	picked, err := browser.PickProfile(profiles)
	if err != nil {
		t.Fatal(err)
	}
	if picked.Name != "default-release" {
		t.Errorf("picked %q; the install section names default-release", picked.Name)
	}
	if !strings.HasSuffix(picked.Path, "ekyhii12.default-release") {
		t.Errorf("path = %q", picked.Path)
	}
}

func TestSeveralProfilesWithNoDefaultAreNotChosenBetween(t *testing.T) {
	// They hold different sessions. Choosing silently would mean reporting
	// no cookie for a site the user is signed in to in the other one, which
	// reads as being signed out.
	home, configHome := firefoxHome(t, `[Profile0]
Name=work
IsRelative=1
Path=aaa.work

[Profile1]
Name=personal
IsRelative=1
Path=bbb.personal
`)
	profiles, err := browser.FirefoxProfiles(home, configHome)
	if err != nil {
		t.Fatal(err)
	}
	_, err = browser.PickProfile(profiles)
	if err == nil {
		t.Fatal("one of two profiles was chosen without saying so")
	}
	if code := errs.From(err).Code; code != errs.CodeUsage {
		t.Errorf("code = %q, want usage — the caller has to say which", code)
	}
	hint := errs.From(err).Hint
	for _, name := range []string{"work", "personal"} {
		if !strings.Contains(hint, name) {
			t.Errorf("the hint does not name %q, so the user cannot act on it:\n%s", name, hint)
		}
	}
	// --profile takes a directory, not one of those names, so the hint has
	// to send the user to where the directories are listed.
	if !strings.Contains(hint, "--list-profiles") {
		t.Errorf("the hint names profiles but not where to find the directory --profile needs:\n%s", hint)
	}
}

func TestOneProfileNeedsNoChoosing(t *testing.T) {
	home, configHome := firefoxHome(t, `[Profile0]
Name=default
IsRelative=1
Path=only.default
`)
	profiles, err := browser.FirefoxProfiles(home, configHome)
	if err != nil {
		t.Fatal(err)
	}
	picked, err := browser.PickProfile(profiles)
	if err != nil {
		t.Fatal(err)
	}
	if picked.Name != "default" {
		t.Errorf("picked %q", picked.Name)
	}
}

func TestAnAbsolutePathIsNotJoinedToTheRoot(t *testing.T) {
	// IsRelative=0 means the path stands on its own, and joining it to the
	// profile root would produce a directory that does not exist.
	home, configHome := firefoxHome(t, `[Profile0]
Name=elsewhere
IsRelative=0
Path=/var/lib/firefox/elsewhere
`)
	profiles, err := browser.FirefoxProfiles(home, configHome)
	if err != nil {
		t.Fatal(err)
	}
	if profiles[0].Path != "/var/lib/firefox/elsewhere" {
		t.Errorf("path = %q", profiles[0].Path)
	}
}

func TestNoFirefoxIsAnAnswerNotAFailure(t *testing.T) {
	home := t.TempDir()
	_, err := browser.FirefoxProfiles(home, filepath.Join(home, ".config"))
	if err == nil {
		t.Fatal("a machine with no Firefox reported a profile")
	}
	if code := errs.From(err).Code; code != errs.CodeNotFound {
		t.Errorf("code = %q, want not_found", code)
	}
}

func TestChromiumProfilesAreNamedFromItsOwnIndex(t *testing.T) {
	// The directories are called "Default", "Profile 1" and so on; the names
	// a person recognises live in Local State. Listing the directories would
	// show every profile as "Default".
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	root := filepath.Join(configHome, "google-chrome")
	switch runtime.GOOS {
	case "darwin":
		root = filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	case "windows":
		root = filepath.Join(configHome, "Google", "Chrome", "User Data")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/chrome-local-state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	// Local State alone no longer makes a profile: it is listed only while
	// there is a cookie database to read.
	writeChromeCookies(t, filepath.Join(root, "Default"))

	profiles := browser.ChromiumProfiles(home, configHome)
	if len(profiles) != 1 {
		t.Fatalf("got %d profiles: %+v", len(profiles), profiles)
	}
	if !strings.HasPrefix(profiles[0].Name, "Chrome") {
		t.Errorf("name = %q; it should say which browser", profiles[0].Name)
	}
	if !strings.HasSuffix(profiles[0].Path, "Default") {
		t.Errorf("path = %q", profiles[0].Path)
	}
	// Chrome's Local State names no last-used profile on a fresh install, so
	// Default is the one it would open.
	if !profiles[0].Default {
		t.Error("the only profile was not marked as the default")
	}
}

// chromiumUserData is where a Chromium-family browser keeps its user data on
// the OS running the test: the Google Chrome channels as Chromium's
// docs/user_data_dir.md lists them (Canary is "Chrome SxS" on Windows and
// "Chrome Canary" on macOS, Dev is "unstable" on Linux), plus Chromium itself.
// It is a table of its own rather than a call into the code under test, so a
// wrong directory there cannot also be wrong here.
func chromiumUserData(home, configHome, browserName string) string {
	base := configHome
	var dirs map[string]string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
		dirs = map[string]string{
			"Chrome": "Google/Chrome", "Chrome Beta": "Google/Chrome Beta",
			"Chrome Dev": "Google/Chrome Dev", "Chrome Canary": "Google/Chrome Canary",
			"Chromium": "Chromium",
		}
	case "windows":
		dirs = map[string]string{
			"Chrome": "Google/Chrome/User Data", "Chrome Beta": "Google/Chrome Beta/User Data",
			"Chrome Dev": "Google/Chrome Dev/User Data", "Chrome Canary": "Google/Chrome SxS/User Data",
			"Chromium": "Chromium/User Data",
		}
	default:
		dirs = map[string]string{
			"Chrome": "google-chrome", "Chrome Beta": "google-chrome-beta",
			"Chrome Dev": "google-chrome-unstable", "Chrome Canary": "google-chrome-canary",
			"Chromium": "chromium",
		}
	}
	return filepath.Join(base, filepath.FromSlash(dirs[browserName]))
}

func writeLocalState(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// listedDirs is the directory name of every profile listed, sorted.
func listedDirs(profiles []browser.Profile) []string {
	var dirs []string
	for _, profile := range profiles {
		dirs = append(dirs, filepath.Base(profile.Path))
	}
	slices.Sort(dirs)
	return dirs
}

func TestChromeBetaDevAndCanaryAreFoundBesideStable(t *testing.T) {
	// Each channel is a separate install with its own user-data directory and
	// its own sign-ins. Looking only at stable Chrome left someone signed in
	// to Chrome Beta with no profile listed at all.
	for _, channel := range []string{"Chrome Beta", "Chrome Dev", "Chrome Canary"} {
		t.Run(channel, func(t *testing.T) {
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			root := chromiumUserData(home, configHome, channel)
			writeLocalState(t, root, `{"profile":{"info_cache":{"Profile 1":{"name":"Me"}},"last_used":"Profile 1"}}`)
			writeChromeCookies(t, filepath.Join(root, "Profile 1"))

			profiles := browser.ChromiumProfiles(home, configHome)
			if len(profiles) != 1 {
				t.Fatalf("got %d profiles, want the one in %s: %+v", len(profiles), channel, profiles)
			}
			if want := channel + " — Me"; profiles[0].Name != want {
				t.Errorf("name = %q, want %q; it should say which channel", profiles[0].Name, want)
			}
			if profiles[0].Path != filepath.Join(root, "Profile 1") {
				t.Errorf("path = %q", profiles[0].Path)
			}
			if !profiles[0].Default {
				t.Error("the profile Local State names as last used was not marked as the default")
			}
		})
	}
}

func TestAProfileWithNoCookieDatabaseIsNotListed(t *testing.T) {
	// Local State outlives the data it indexes. Listing such a profile, or
	// worse choosing it as the default, can only end in "no cookie database".
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	root := chromiumUserData(home, configHome, "Chrome")
	writeLocalState(t, root, `{"profile":{"info_cache":{"Default":{"name":"Empty"}},"last_used":"Default"}}`)
	if err := os.MkdirAll(filepath.Join(root, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := browser.ChromiumProfiles(home, configHome); len(got) != 0 {
		t.Errorf("listed %d profile(s) that have no cookie database: %+v", len(got), got)
	}
}

func TestAnEmptyDefaultDoesNotHideAProfileThatHasCookies(t *testing.T) {
	// The profile Chrome would open has nothing to read, and the one the
	// person signs in with is called "Profile 1". Choosing Default ends in an
	// error while a usable profile sits beside it.
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	root := chromiumUserData(home, configHome, "Chrome")
	writeLocalState(t, root, `{"profile":{"info_cache":{"Default":{"name":"Empty"},"Profile 1":{"name":"Work"}},"last_used":"Default"}}`)
	if err := os.MkdirAll(filepath.Join(root, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeChromeCookies(t, filepath.Join(root, "Profile 1"))

	profiles := browser.ChromiumProfiles(home, configHome)
	if got, want := listedDirs(profiles), []string{"Profile 1"}; !slices.Equal(got, want) {
		t.Fatalf("listed %v, want %v", got, want)
	}
	picked, err := browser.PickProfile(profiles)
	if err != nil {
		t.Fatalf("the only profile with cookies was not chosen: %v", err)
	}
	if filepath.Base(picked.Path) != "Profile 1" {
		t.Errorf("picked %q", picked.Path)
	}
}

func TestAnEmptyChromiumDoesNotHideChromeBeta(t *testing.T) {
	// The reported machine: a Chromium whose Default has no cookie database,
	// and the browser actually in use, Chrome Beta, signed in under
	// "Profile 1". Both are discovered, but only Chrome Beta has a database,
	// so there is one obvious choice and no "several profiles" refusal.
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	chromium := chromiumUserData(home, configHome, "Chromium")
	writeLocalState(t, chromium, `{"profile":{"info_cache":{"Default":{"name":"Person 1"}}}}`)
	if err := os.MkdirAll(filepath.Join(chromium, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	beta := chromiumUserData(home, configHome, "Chrome Beta")
	writeLocalState(t, beta, `{"profile":{"info_cache":{"Profile 1":{"name":"Me"}},"last_used":"Profile 1"}}`)
	writeChromeCookies(t, filepath.Join(beta, "Profile 1"))

	profiles := browser.ChromiumProfiles(home, configHome)
	picked, err := browser.PickProfile(profiles)
	if err != nil {
		t.Fatalf("the only profile with cookies was not chosen: %v (%+v)", err, profiles)
	}
	if picked.Path != filepath.Join(beta, "Profile 1") {
		t.Errorf("picked %q", picked.Path)
	}
}

func TestTheLegacyCookieLocationCountsAsADatabase(t *testing.T) {
	// Releases before the Network directory kept the file in the profile
	// root, and ChromiumSession still reads it there, so discovery has to
	// agree with what the reader can open. "Profile 1" has neither layout.
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	root := chromiumUserData(home, configHome, "Chrome")
	writeLocalState(t, root, `{"profile":{"info_cache":{"Default":{"name":"Old"},"Profile 1":{"name":"Bare"}}}}`)
	for _, dir := range []string{"Default", "Profile 1"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "Default", "Cookies"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := listedDirs(browser.ChromiumProfiles(home, configHome)), []string{"Default"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
}

func TestAProfileThatCannotBeCheckedIsStillListed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file modes do not stop this user from looking")
	}
	// Only "no such file" means there is nothing to read. A profile whose
	// database cannot be looked up for another reason is still the one the
	// person meant, and an explicit import should surface the real error, the
	// way Safari's unreadable store is listed rather than hidden.
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	root := chromiumUserData(home, configHome, "Chrome")
	writeLocalState(t, root, `{"profile":{"info_cache":{"Default":{"name":"Locked"},"Profile 1":{"name":"Bare"}}}}`)
	writeChromeCookies(t, filepath.Join(root, "Default"))
	if err := os.MkdirAll(filepath.Join(root, "Profile 1"), 0o700); err != nil {
		t.Fatal(err)
	}
	network := filepath.Join(root, "Default", "Network")
	if err := os.Chmod(network, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(network, 0o700) })

	if got, want := listedDirs(browser.ChromiumProfiles(home, configHome)), []string{"Default"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
}

func TestNoChromiumIsAnEmptyListNotAFailure(t *testing.T) {
	home := t.TempDir()
	if got := browser.ChromiumProfiles(home, filepath.Join(home, ".config")); len(got) != 0 {
		t.Errorf("got %d profiles on a machine with no Chromium", len(got))
	}
}

func TestAProfileNamedByHandIsTriedBothWays(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the recorded v10 fixture uses Chromium's Linux fallback key")
	}
	// Someone who points at a directory knows what they pointed at. Being
	// told "that is not a Firefox profile" when it is a Chrome one helps
	// nobody.
	profile := browser.Profile{Path: chromeProfile(t)} // no Kind set
	found, err := browser.ReadSession(profile, "192.168.50.169", "")
	if err != nil {
		t.Fatalf("a Chrome profile named by hand was not read: %v", err)
	}
	if found.Value == "" {
		t.Error("read nothing")
	}
}
