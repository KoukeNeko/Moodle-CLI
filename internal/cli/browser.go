package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/browser"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// newAuthImportBrowserCommand reads a session out of the user's own browser.
//
// It is its own command rather than a step inside `auth login` on purpose.
// Opening someone's browser storage is not something to do as a side effect
// of signing in: it is a distinct act, it touches a file this tool does not
// own, and the person running it should be the one who decided to.
//
// What it can promise is narrower than it looks, and the output says so. The
// file it reads holds every site's session cookies in one container, so the
// honest claim is that only the one asked for is taken, kept or shown — not
// that nothing else was ever parsed.
func newAuthImportBrowserCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags       sessionFlags
		profileDir  string
		browserName string
		cookieName  string
		listOnly    bool
		store       bool
	)
	cmd := &cobra.Command{
		Use:   "import-browser",
		Short: "Take this site's session from a browser you are already signed in to",
		Long: "Reads the Moodle session cookie for one site out of a Safari,\n" +
			"Firefox or Chromium profile, so a site that issues no web service\n" +
			"token can be used without copying it out of developer tools.\n\n" +
			"Only the cookie for the site named is returned. A browser keeps\n" +
			"every site's cookies together, so others are necessarily parsed on\n" +
			"the way past; none of them is returned, kept or logged.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "auth.import_browser", annotationSafety: safetyLocal},
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedKind := browser.Kind(strings.ToLower(browserName))
			switch selectedKind {
			case browser.Unknown, browser.Firefox, browser.Chromium, browser.Safari:
			default:
				return errs.New(errs.CodeUsage, "unknown browser "+browserName).
					WithHint("choose safari, firefox or chromium")
			}
			if selectedKind == browser.Safari && runtime.GOOS != "darwin" && profileDir == "" {
				return errs.New(errs.CodeUnavailable,
					"automatic Safari cookie discovery requires macOS").
					WithHint("name an existing Cookies.binarycookies file with --profile")
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return errs.Wrap(errs.CodeUnavailable, err, "cannot find your home directory")
			}
			configHome := os.Getenv("XDG_CONFIG_HOME")

			// Discover only the selected browser when one was named. In
			// particular, asking for Safari should not open another browser's
			// profile index, even though no cookies are read during discovery.
			var profiles []browser.Profile
			if selectedKind == browser.Unknown || selectedKind == browser.Firefox {
				if firefox, err := browser.FirefoxProfiles(home, configHome); err == nil {
					for _, profile := range firefox {
						profile.Kind = browser.Firefox
						profiles = append(profiles, profile)
					}
				}
			}
			if selectedKind == browser.Unknown || selectedKind == browser.Chromium {
				for _, profile := range browser.ChromiumProfiles(home, configHome) {
					profile.Kind = browser.Chromium
					profiles = append(profiles, profile)
				}
			}
			if selectedKind == browser.Unknown || selectedKind == browser.Safari {
				profiles = append(profiles, browser.SafariProfiles(home)...)
			}
			if len(profiles) == 0 && selectedKind == browser.Safari && !listOnly && profileDir == "" {
				// Let the reader distinguish "not present" from "permission denied"
				// and show the Safari-specific recovery hint.
				profiles = []browser.Profile{{
					Name: "Safari", Path: browser.SafariCookiePath(home), Kind: browser.Safari,
				}}
			}
			if len(profiles) == 0 && (profileDir == "" || listOnly) {
				return errs.New(errs.CodeNotFound,
					"no matching browser profile found on this machine").
					WithHint("choose a browser with --browser safari|firefox|chromium, or name its cookie store/profile with --profile")
			}
			if listOnly {
				return r.Render(Result{
					Human: func(w io.Writer) error { return writeProfiles(w, profiles) },
				})
			}

			profile := browser.Profile{Path: profileDir, Kind: selectedKind}
			if profileDir == "" {
				picked, err := browser.PickProfile(profiles)
				if err != nil {
					return err
				}
				profile = picked
			} else if selectedKind == browser.Unknown && filepath.Base(profileDir) == "Cookies.binarycookies" {
				profile.Kind = browser.Safari
			}

			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(flags.site, "")
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}
			host := target.BaseURL.Hostname()

			found, err := browser.ReadSession(profile, host, cookieName)
			if err != nil {
				return err
			}
			if !store {
				return r.Render(Result{
					Human: func(w io.Writer) error {
						return writeImported(w, profile, found, host)
					},
				})
			}

			cookie := found.Name + "=" + found.Value
			name, userID, err := storeImportedSession(cmd.Context(), deps, file, resolved, target, flags.account, cookie, "import-browser")
			if err != nil {
				return err
			}
			return r.Render(Result{
				Human: humanLine(
					"Stored the %s session for %s as account %q (user %s).\n"+
						"It is the browser's own session, so signing out there ends it here too.",
					describeBrowser(profile.Kind), resolved.SiteName, name, userID),
			})
		},
	}
	flags.bind(cmd, "import a session for")
	cmd.Flags().StringVar(&profileDir, "profile", "",
		"read this browser profile directory or Safari cookie file")
	cmd.Flags().StringVar(&browserName, "browser", "",
		"select safari, firefox or chromium (default: discover profiles)")
	cmd.Flags().StringVar(&cookieName, "cookie-name", "",
		"the session cookie's name, if the site has renamed it (default "+
			browser.SessionCookieName+")")
	cmd.Flags().BoolVar(&listOnly, "list-profiles", false,
		"list the browser profiles on this machine and stop")
	cmd.Flags().BoolVar(&store, "store", false,
		"keep the session in the OS keychain so later commands need no flag")
	return cmd
}

// describeBrowser names the family for a person, or declines to guess.
func describeBrowser(kind browser.Kind) string {
	switch kind {
	case browser.Firefox:
		return "Firefox"
	case browser.Chromium:
		return "Chromium-family"
	case browser.Safari:
		return "Safari"
	default:
		// A directory named on the command line: both readers were tried and
		// one of them worked, and saying which would be a guess.
		return "browser"
	}
}

func writeProfiles(w io.Writer, profiles []browser.Profile) error {
	table := newTable(w)
	fmt.Fprintln(table, "NAME\tDEFAULT\tPATH")
	for _, profile := range profiles {
		mark := ""
		if profile.Default {
			mark = "yes"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", profile.Name, mark, profile.Path)
	}
	return table.Flush()
}

func writeImported(w io.Writer, profile browser.Profile, found browser.Found, host string) error {
	name := profile.Name
	if name == "" {
		name = profile.Path
	}
	// Named from the profile rather than hard-coded: a Chrome cookie
	// announced as coming from Firefox reads as the wrong file being opened,
	// which is exactly the worry this command should not create.
	fmt.Fprintf(w, "Found a %s for %s in the %s profile %q.\n",
		found.Name, host, describeBrowser(profile.Kind), name)
	fmt.Fprintf(w, "Read from %s\n\n", found.Source)
	// The value is printed because the next command needs it, and because a
	// credential this tool hands over should be visible to the person it
	// belongs to. It is not stored here: storing it would be the login.
	fmt.Fprintf(w, "  %s=%s\n\n", found.Name, found.Value)
	// Not `auth login --method browser-session`. That exchanges the session
	// for a token, and Moodle only offers the exchange to a session that has
	// just signed in — a session taken out of a browser somebody has been
	// using is past that by definition. Measured: a cookie imported from a
	// real Firefox profile is refused by the exchange and works perfectly for
	// everything below.
	fmt.Fprintln(w, strings.TrimSpace(`
Keep it, so later commands need no flag:
  moodle auth import-browser --store

Or use it for one command:
  MOODLE_SESSION='`+found.Name+`=`+found.Value+`' moodle course list

This is the browser's own session, so signing out there ends it here too.`))
	return nil
}
