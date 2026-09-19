package browser_test

import (
	"os"
	"path/filepath"
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
