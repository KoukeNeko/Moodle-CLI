package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/browser"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
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
		flags      sessionFlags
		profileDir string
		cookieName string
		listOnly   bool
		store      bool
	)
	cmd := &cobra.Command{
		Use:   "import-browser",
		Short: "Take this site's session from a browser you are already signed in to",
		Long: "Reads the Moodle session cookie for one site out of a Firefox\n" +
			"profile, so a site that issues no web service token can be used\n" +
			"without copying the cookie out of developer tools by hand.\n\n" +
			"Only the cookie for the site named is read, and it is not stored:\n" +
			"it is printed for `auth login --method browser-session` to take, or\n" +
			"exchanged by that command. Firefox keeps every site's session\n" +
			"cookies in one file, so others are necessarily parsed on the way\n" +
			"past; none of them is returned, kept or logged.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "auth.import_browser"},
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return errs.Wrap(errs.CodeUnavailable, err, "cannot find your home directory")
			}
			profiles, err := browser.FirefoxProfiles(home, os.Getenv("XDG_CONFIG_HOME"))
			if err != nil {
				return err
			}
			if listOnly {
				return r.Render(Result{
					Human: func(w io.Writer) error { return writeProfiles(w, profiles) },
				})
			}

			profile := browser.Profile{Path: profileDir}
			if profileDir == "" {
				picked, err := browser.PickProfile(profiles)
				if err != nil {
					return err
				}
				profile = picked
			}

			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(flags.site, flags.account)
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}
			host := target.BaseURL.Hostname()

			found, err := browser.FirefoxSession(profile.Path, host, cookieName)
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

			// Verified before it is kept: a credential that does not work
			// must never be written to the keychain as though it did. This
			// also settles whose session it is, which the cookie does not say.
			cookie := found.Name + "=" + found.Value
			opened := deps.Auth.OpenWithSession(target, "", cookie)
			capabilities, err := opened.Capabilities(cmd.Context())
			if err != nil {
				return err
			}
			if capabilities.UserID == "" {
				// The site answered but did not say who for. Storing it would
				// mean keeping a credential nothing has shown to work.
				return errs.New(errs.CodeAuthentication,
					"that session was not accepted by "+resolved.SiteName).
					WithHint("sign in again in Firefox, then import it")
			}

			// A session carries no username, only a numeric id — the page it
			// was read from says who it is for, not what they are called.
			name := flags.account
			if name == "" {
				name = "browser-" + capabilities.UserID
			}
			account := resolved.Site.UpsertAccount(name, config.Account{
				UserID:         capabilities.UserID,
				AuthMethod:     "import-browser",
				CredentialKind: site.CredentialBrowserSession,
			})
			if err := deps.Auth.StoreSession(resolved.Site.ID, account.ID, cookie); err != nil {
				return err
			}
			file.Current.Site = resolved.SiteName
			file.Current.Account = name
			if err := file.Save(); err != nil {
				return err
			}
			return r.Render(Result{
				Human: humanLine(
					"Stored the Firefox session for %s as account %q (user %s).\n"+
						"It is the browser's own session, so signing out there ends it here too.",
					resolved.SiteName, name, capabilities.UserID),
			})
		},
	}
	flags.bind(cmd, "import a session for")
	cmd.Flags().StringVar(&profileDir, "profile", "",
		"read this Firefox profile directory instead of the default one")
	cmd.Flags().StringVar(&cookieName, "cookie-name", "",
		"the session cookie's name, if the site has renamed it (default "+
			browser.SessionCookieName+")")
	cmd.Flags().BoolVar(&listOnly, "list-profiles", false,
		"list the Firefox profiles on this machine and stop")
	cmd.Flags().BoolVar(&store, "store", false,
		"keep the session in the OS keychain so later commands need no flag")
	return cmd
}

func writeProfiles(w io.Writer, profiles []browser.Profile) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
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
	fmt.Fprintf(w, "Found a %s for %s in the Firefox profile %q.\n",
		found.Name, host, name)
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
