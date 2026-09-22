package browser

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Profile is one Firefox profile on this machine.
type Profile struct {
	Name string
	Path string
	// Default marks the one the browser itself would open. For Firefox it
	// comes from the install's own section rather than from the older
	// per-profile marker: measured on 156, a machine can have both and they
	// can disagree.
	Default bool
	// Kind decides which reader runs. The two families store sessions in
	// entirely different places.
	Kind Kind
}

// firefoxRoots are where Firefox keeps profiles.ini, newest convention first.
//
// Firefox 156 uses the XDG configuration directory. Older versions used
// ~/.mozilla/firefox, and plenty of machines still have one from before an
// upgrade — measured here: a fresh Firefox 156 created
// ~/.config/mozilla/firefox and never touched ~/.mozilla at all, which is
// where documentation and every tutorial still say to look.
func firefoxRoots(home, configHome string) []string {
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return []string{
		filepath.Join(configHome, "mozilla", "firefox"),
		filepath.Join(home, ".mozilla", "firefox"),
		// Packaged builds put the whole home elsewhere.
		filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
		filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),
		// macOS.
		filepath.Join(home, "Library", "Application Support", "Firefox"),
	}
}

// FirefoxProfiles lists the profiles on this machine.
//
// Nothing is opened here beyond profiles.ini: this says what exists, and the
// caller decides what to read. A machine with no Firefox is not an error —
// it is an answer.
func FirefoxProfiles(home, configHome string) ([]Profile, error) {
	for _, root := range firefoxRoots(home, configHome) {
		raw, err := os.ReadFile(filepath.Join(root, "profiles.ini"))
		if err != nil {
			continue
		}
		profiles, err := parseProfilesINI(string(raw), root)
		if err != nil {
			return nil, err
		}
		if len(profiles) > 0 {
			return profiles, nil
		}
	}
	return nil, errs.New(errs.CodeNotFound, "no Firefox profile found on this machine").
		WithHint("if Firefox is installed somewhere unusual, name the profile " +
			"directory with --profile")
}

// parseProfilesINI reads Firefox's own index of its profiles.
//
// The format is INI, and the part that matters is which profile is the
// default. Two things can say so and they can disagree: every [InstallXXXX]
// section names the default for one installed copy of Firefox, while a
// [ProfileN] section may carry an older Default=1 marker. The install wins,
// because that is the one Firefox opens — checked on a machine where they
// pointed at different profiles.
func parseProfilesINI(content, root string) ([]Profile, error) {
	var profiles []Profile
	installDefaults := map[string]bool{}

	var section string
	var current *Profile
	var relative bool

	flush := func() {
		if current != nil && current.Path != "" {
			if relative {
				current.Path = filepath.Join(root, current.Path)
			}
			profiles = append(profiles, *current)
		}
		current, relative = nil, false
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = strings.Trim(line, "[]")
			if strings.HasPrefix(section, "Profile") {
				current = &Profile{}
			}
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		if strings.HasPrefix(section, "Install") {
			if key == "Default" {
				installDefaults[value] = true
			}
			continue
		}
		if current == nil {
			continue
		}
		switch key {
		case "Name":
			current.Name = value
		case "Path":
			current.Path = value
		case "IsRelative":
			relative = value == "1"
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeUpstream, err, "cannot read profiles.ini")
	}

	// The install sections name a path as written in the file, so compare on
	// the last element rather than on what it has been joined to.
	for i := range profiles {
		if installDefaults[filepath.Base(profiles[i].Path)] {
			profiles[i].Default = true
		}
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].Default && !profiles[j].Default
	})
	return profiles, nil
}

// PickProfile chooses which profile to read, or refuses to choose.
//
// A machine can hold several, and they hold different sessions. Picking one
// silently would mean reporting "no cookie" for a site the user is signed in
// to in the other, which reads as being signed out.
func PickProfile(profiles []Profile) (Profile, error) {
	switch len(profiles) {
	case 0:
		return Profile{}, errs.New(errs.CodeNotFound, "no browser profile found")
	case 1:
		return profiles[0], nil
	}
	var defaults []Profile
	for _, profile := range profiles {
		if profile.Default {
			defaults = append(defaults, profile)
		}
	}
	if len(defaults) == 1 {
		return defaults[0], nil
	}

	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	// Not the ambiguous-outcome code: that one means a write may already have
	// landed upstream, and this is the caller needing to say which profile.
	return Profile{}, errs.New(errs.CodeUsage,
		"this machine has several browser profiles and no single default").
		WithHint("they hold different sessions, so say which with --profile: " +
			strings.Join(names, ", "))
}

// chromiumRoots are where the Chromium family keeps its user data, by browser.
//
// Each is a separate browser with its own cookies and its own encryption
// settings. They are listed rather than generalised from Chrome, because
// assuming one from another is how a reader ends up looking in the wrong
// place and reporting that the user is not signed in.
func chromiumRoots(home, configHome string) map[string]string {
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	roots := map[string]string{
		"Chrome":   filepath.Join(configHome, "google-chrome"),
		"Chromium": filepath.Join(configHome, "chromium"),
		"Edge":     filepath.Join(configHome, "microsoft-edge"),
		"Brave":    filepath.Join(configHome, "BraveSoftware", "Brave-Browser"),
	}
	if runtime.GOOS == "darwin" {
		support := filepath.Join(home, "Library", "Application Support")
		roots = map[string]string{
			"Chrome":   filepath.Join(support, "Google", "Chrome"),
			"Chromium": filepath.Join(support, "Chromium"),
			"Edge":     filepath.Join(support, "Microsoft Edge"),
			"Brave":    filepath.Join(support, "BraveSoftware", "Brave-Browser"),
		}
	}
	return roots
}

// localState is the part of Chromium's own index this reads.
type localState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name string `json:"name"`
		} `json:"info_cache"`
		LastUsed string `json:"last_used"`
	} `json:"profile"`
}

// ChromiumProfiles lists the Chromium-family profiles on this machine.
//
// A browser's profiles are named in its Local State file rather than in the
// directory listing: the directories are called "Default", "Profile 1" and so
// on, while the names people recognise live in that index.
func ChromiumProfiles(home, configHome string) []Profile {
	var found []Profile
	for browserName, root := range chromiumRoots(home, configHome) {
		raw, err := os.ReadFile(filepath.Join(root, "Local State"))
		if err != nil {
			continue
		}
		var state localState
		if err := json.Unmarshal(raw, &state); err != nil {
			continue
		}
		for dir, info := range state.Profile.InfoCache {
			label := browserName
			if info.Name != "" && info.Name != browserName {
				label = browserName + " — " + info.Name
			}
			found = append(found, Profile{
				Name:    label,
				Path:    filepath.Join(root, dir),
				Default: state.Profile.LastUsed == dir || (state.Profile.LastUsed == "" && dir == "Default"),
			})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Default != found[j].Default {
			return found[i].Default
		}
		return found[i].Name < found[j].Name
	})
	return found
}
