package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// EnvWSToken supplies a token for a single run, bypassing both the
// configuration and the keychain.
const EnvWSToken = "MOODLE_WS_TOKEN"

func newAuthCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Sign in to Moodle and inspect the current session",
	}
	cmd.AddCommand(
		newAuthLoginCommand(r, deps),
		newAuthMethodsCommand(r, deps),
		newAuthStatusCommand(r, deps),
		newAuthLogoutCommand(r, deps),
	)
	return cmd
}

func newAuthLoginCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		siteFlag      string
		accountName   string
		methodName    string
		token         string
		tokenStdin    bool
		username      string
		passwordStdin bool
		callback      string
		passport      string
		qr            string
		sessionCookie string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in and store the credential in the OS keychain",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "auth.login",
			// Moodle issues or reuses a token, which is a server-side effect.
			annotationMutates: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if tokenStdin {
				read, err := readAll(cmd.InOrStdin())
				if err != nil {
					return errs.Wrap(errs.CodeUsage, err, "cannot read the token from stdin")
				}
				token = read
				if methodName == "" {
					methodName = "token"
				}
			}
			password := ""
			if passwordStdin {
				read, err := readAll(cmd.InOrStdin())
				if err != nil {
					return errs.Wrap(errs.CodeUsage, err, "cannot read the password from stdin")
				}
				password = read
				if methodName == "" {
					methodName = "password"
				}
			}

			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(siteFlag, "")
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}

			credential, err := deps.Login.Authenticate(cmd.Context(), auth.Request{
				Site:          target,
				Username:      username,
				Password:      password,
				Callback:      callback,
				Passport:      passport,
				QR:            qr,
				SessionCookie: sessionCookie,
				Token:         token,
				In:            cmd.InOrStdin(),
				// Prompts are diagnostics: stdout carries the result only.
				Out: r.Streams.Err,
			}, methodName)
			if err != nil {
				return err
			}

			// Verify before storing: a credential that does not work must
			// never be written to the keychain as though it did.
			session := deps.Auth.OpenWithToken(target, "", credential.Token)
			capabilities, err := session.Capabilities(cmd.Context())
			if err != nil {
				return err
			}

			name := accountName
			if name == "" {
				name = capabilities.Username
			}
			if name == "" {
				name = "default"
			}
			account := resolved.Site.UpsertAccount(name, config.Account{
				UserID:         capabilities.UserID,
				Username:       capabilities.Username,
				DisplayName:    capabilities.FullName,
				AuthMethod:     credential.Method,
				CredentialKind: site.CredentialWSToken,
			})
			if err := deps.Auth.StoreToken(resolved.Site.ID, account.ID, credential.Token); err != nil {
				return err
			}
			if resolved.Site.WWWRoot == "" {
				resolved.Site.WWWRoot = target.BaseURL.String()
			}
			file.Current.Site = resolved.SiteName
			file.Current.Account = name
			if err := file.Save(); err != nil {
				return err
			}

			status := v1.AuthStatus{Site: resolved.SiteName, Valid: true}
			setString(&status.Account, name)
			setString(&status.Username, capabilities.Username)
			setString(&status.UserID, capabilities.UserID)
			setString(&status.FullName, capabilities.FullName)
			setString(&status.SiteName, capabilities.SiteName)
			setString(&status.Release, capabilities.Release)

			return r.Render(Result{
				Envelope: v1.NewEnvelope("auth.login", status, v1.NewMeta(v1.SourceWS)),
				Human: humanLine("Signed in to %s as %s (%s) using %s.",
					capabilities.SiteName, capabilities.FullName, capabilities.Username, credential.Method),
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to sign in to")
	cmd.Flags().StringVar(&accountName, "account", "", "name for this account (defaults to the Moodle username)")
	cmd.Flags().StringVar(&methodName, "method", "",
		"login method: token, password, qr, browser-session or manual")
	cmd.Flags().StringVar(&token, "token", "", "an existing web service token")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin")
	cmd.Flags().StringVar(&username, "username", "", "Moodle username (password method)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from stdin")
	cmd.Flags().StringVar(&callback, "callback", "", "a pasted <scheme>://token=... callback URL (manual method)")
	cmd.Flags().StringVar(&passport, "passport", "", "the passport used to start the login, so the callback can be verified")
	cmd.Flags().StringVar(&qr, "qr", "", "the decoded content of a login QR code (qr method)")
	cmd.Flags().StringVar(&sessionCookie, "session-cookie", "",
		"a session your browser already holds, as MoodleSession=… (browser-session method)")
	return cmd
}

// methodInfo describes one login method for `auth methods`.
type methodInfo struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Availability string  `json:"availability"`
	Reason       *string `json:"reason"`
}

func newAuthMethodsCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteFlag string
	cmd := &cobra.Command{
		Use:         "methods",
		Short:       "Show which login methods this site supports",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "auth.methods"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(siteFlag, "")
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}
			// A site that cannot be reached is not an error here: every method
			// then reports "unknown", which is the honest answer.
			publicConfig, _ := deps.Auth.Probe(target).PublicConfig(cmd.Context())

			infos := []methodInfo{}
			for _, candidate := range deps.Login.Candidates(cmd.Context(), target, publicConfig) {
				info := methodInfo{
					Name:         candidate.Method.Name(),
					Description:  candidate.Method.Describe(),
					Availability: string(candidate.Probe.Availability),
				}
				setString(&info.Reason, candidate.Probe.Reason)
				infos = append(infos, info)
			}

			return r.Render(Result{
				Envelope: v1.NewEnvelope("auth.methods", infos, v1.NewMeta(v1.SourceWS)),
				Human:    func(w io.Writer) error { return writeMethods(w, infos) },
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to check")
	return cmd
}

func writeMethods(w io.Writer, infos []methodInfo) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "METHOD\tSTATUS\tDESCRIPTION")
	for _, info := range infos {
		status := info.Availability
		if info.Reason != nil {
			status += " (" + *info.Reason + ")"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", info.Name, status, info.Description)
	}
	return table.Flush()
}

func readAll(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func newAuthStatusCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteFlag, accountFlag string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show who is signed in, and check the credential still works",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "auth.status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, token, err := resolveSession(deps, file, siteFlag, accountFlag)
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}

			status := v1.AuthStatus{Site: resolved.SiteName}
			setString(&status.Account, resolved.AccountName)
			setString(&status.Username, resolved.Account.Username)
			setString(&status.UserID, resolved.Account.UserID)
			setString(&status.FullName, resolved.Account.DisplayName)

			capabilities, err := deps.Auth.
				OpenWithToken(target, resolved.Account.ID, token).
				Capabilities(cmd.Context())
			if err != nil {
				// Report the stored identity alongside the failure: knowing
				// which account stopped working is half the diagnosis.
				return err
			}
			status.Valid = true
			setString(&status.SiteName, capabilities.SiteName)
			setString(&status.Release, capabilities.Release)

			return r.Render(Result{
				Envelope: v1.NewEnvelope("auth.status", status, v1.NewMeta(v1.SourceWS)),
				Human: humanLine("%s on %s as %s (%s) — credential valid.",
					resolved.AccountName, resolved.SiteName, capabilities.FullName, capabilities.Username),
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to check")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account to check")
	return cmd
}

func newAuthLogoutCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteFlag, accountFlag string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored credential from this machine",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "auth.logout",
			// Local only. The Moodle token is deliberately not revoked: it is
			// often the same one the user's phone app holds.
			annotationMutates: "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.RequireAccount(siteFlag, accountFlag)
			if err != nil {
				return err
			}
			siteID := resolved.Site.ID
			account, err := resolved.Site.RemoveAccount(resolved.AccountName)
			if err != nil {
				return err
			}
			if file.Current.Account == resolved.AccountName {
				file.Current.Account = resolved.Site.DefaultAccount
			}
			if err := file.Save(); err != nil {
				return err
			}
			if err := deps.Auth.Forget(siteID, account.ID); err != nil {
				return err
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("auth.logout",
					map[string]string{"site": resolved.SiteName, "account": resolved.AccountName},
					v1.NewMeta(v1.SourceLocal)),
				Human: humanLine(
					"Removed %q from this machine.\n"+
						"The Moodle token itself was not revoked: it may be shared with the Moodle app.\n"+
						"To revoke it, use Security keys in your Moodle profile.",
					resolved.AccountName),
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to sign out of")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account to sign out")
	return cmd
}

// lookupToken finds the token for an account: the environment first, so CI can
// run without a keychain at all.
func lookupToken(deps Deps, resolved config.Resolved) (string, error) {
	if fromEnv := envToken(); fromEnv != "" {
		return fromEnv, nil
	}
	return deps.Auth.Token(resolved.Site.ID, resolved.Account.ID)
}

func envToken() string { return strings.TrimSpace(os.Getenv(EnvWSToken)) }

// resolveSession finds the site, account and token a command should use.
//
// A token in the environment bypasses both the configuration and the keychain.
// That is what makes the tool usable in CI and on headless machines, where
// there is no keychain to store anything in — so it must not require an
// account to have been stored first.
func resolveSession(deps Deps, file *config.File, siteFlag, accountFlag string) (config.Resolved, string, error) {
	if token := envToken(); token != "" {
		resolved, err := file.Resolve(siteFlag, accountFlag)
		if err != nil {
			return config.Resolved{}, "", err
		}
		if resolved.Account == nil {
			// Nothing is stored, and nothing needs to be: the credential came
			// from the environment and lives only for this run.
			resolved.AccountName = "env"
			resolved.Account = &config.Account{}
		}
		return resolved, token, nil
	}
	resolved, err := file.RequireAccount(siteFlag, accountFlag)
	if err != nil {
		return config.Resolved{}, "", err
	}
	token, err := deps.Auth.Token(resolved.Site.ID, resolved.Account.ID)
	if err != nil {
		return config.Resolved{}, "", err
	}
	return resolved, token, nil
}

func setString(target **string, value string) {
	if value == "" {
		return
	}
	copied := value
	*target = &copied
}
