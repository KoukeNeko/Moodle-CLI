package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

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

// EnvSession supplies a browser session for a single run.
//
// It is the only way in on a site that issues no token at all, and like the
// token variable it exists so CI and headless machines are not required to
// have a keychain.
const EnvSession = "MOODLE_SESSION"

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
		newAuthImportBrowserCommand(r, deps),
		newAuthImportSessionCommand(r, deps),
		newAuthRegisterHandlerCommand(r, deps.Handler),
		newAuthUnregisterHandlerCommand(r, deps.Handler),
		newAuthHandlerStatusCommand(r, deps.Handler),
		newAuthCallbackCommand(deps.Handler),
	)
	return cmd
}

func newAuthLoginCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		siteFlag           string
		accountName        string
		methodName         string
		token              string
		tokenStdin         bool
		username           string
		passwordStdin      bool
		callback           string
		passport           string
		qr                 string
		qrStdin            bool
		sessionCookie      string
		sessionCookieStdin bool
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
			// Every credential a method needs can arrive on stdin, so that
			// none has to travel in argv, where `ps` shows it to anyone on the
			// machine and the shell keeps it in history.
			//
			// At most one, because each reads the whole stream: a second read
			// would get nothing and sign in with an empty credential.
			password := ""
			stdinSources := []struct {
				flag   string
				asked  bool
				target *string
				method string
			}{
				{"--token-stdin", tokenStdin, &token, "token"},
				{"--password-stdin", passwordStdin, &password, "password"},
				{"--qr-stdin", qrStdin, &qr, "qr"},
				{"--session-cookie-stdin", sessionCookieStdin, &sessionCookie, "browser-session"},
			}
			chosen := -1
			for i, source := range stdinSources {
				if !source.asked {
					continue
				}
				if chosen >= 0 {
					return errs.New(errs.CodeUsage,
						fmt.Sprintf("%s and %s both read stdin, and it can only be read once",
							stdinSources[chosen].flag, source.flag)).
						WithHint("pass one of them")
				}
				chosen = i
			}
			if chosen >= 0 {
				source := stdinSources[chosen]
				read, err := readAll(cmd.InOrStdin())
				if err != nil {
					return errs.Wrap(errs.CodeUsage, err,
						"cannot read "+source.flag+" from stdin")
				}
				if read == "" {
					// Otherwise an empty pipe signs in with an empty
					// credential and the site's refusal names the site.
					return errs.New(errs.CodeUsage, source.flag+" read nothing from stdin")
				}
				*source.target = read
				if methodName == "" {
					methodName = source.method
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

			// With --no-input there is nothing to read an answer from, and each
			// method already says which flag supplies what it would have asked.
			in := cmd.InOrStdin()
			if r.NoInput {
				in = nil
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
				In:            in,
				// Prompts are diagnostics: stdout carries the result only.
				Out: r.Streams.Err,
			}, methodName)
			if err != nil {
				return err
			}

			return storeLogin(cmd, r, deps, file, resolved, target, credential, accountName)
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to sign in to")
	cmd.Flags().StringVar(&accountName, "account", "", "name for this account (defaults to the Moodle username)")
	cmd.Flags().StringVar(&methodName, "method", "",
		"login method: token, password, qr, mobilelaunch, browser-session or manual")
	cmd.Flags().StringVar(&token, "token", "", "an existing web service token")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin")
	cmd.Flags().StringVar(&username, "username", "", "Moodle username (password method)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from stdin")
	cmd.Flags().StringVar(&callback, "callback", "", "a pasted <scheme>://token=... callback URL (manual method)")
	cmd.Flags().StringVar(&passport, "passport", "", "the passport used to start the login, so the callback can be verified")
	cmd.Flags().StringVar(&qr, "qr", "", "the decoded content of a login QR code (qr method)")
	cmd.Flags().BoolVar(&qrStdin, "qr-stdin", false, "read the QR content from stdin")
	cmd.Flags().StringVar(&sessionCookie, "session-cookie", "",
		"a session your browser already holds, as MoodleSession=… (browser-session method)")
	cmd.Flags().BoolVar(&sessionCookieStdin, "session-cookie-stdin", false,
		"read the browser session from stdin")
	return cmd
}

// methodInfo describes one login method for `auth methods`.
type methodInfo struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Availability string  `json:"availability"`
	Reason       *string `json:"reason"`
}

// storeLogin verifies a fresh credential, records the account and reports it.
//
// Shared by `auth login` and `setup`: the verification is the load-bearing
// part — a credential that does not work must never be written to the store as
// though it did — and two copies of it would be two chances to lose it.
func storeLogin(cmd *cobra.Command, r *Renderer, deps Deps, file *config.File,
	resolved config.Resolved, target site.Site, credential auth.Credential,
	accountName string) error {
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
	if err := deps.Auth.StoreCredential(resolved.Site.ID, account.ID, credential); err != nil {
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
		Human: humanLine("Signed in to %s as %s (%s) using %s.%s",
			capabilities.SiteName, capabilities.FullName, capabilities.Username,
			credential.Method, storedWhere(deps)),
	})
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
	table := newTable(w)
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
			if _, err := targetSite(resolved.SiteName, resolved.Site); err != nil {
				return err
			}

			status := v1.AuthStatus{Site: resolved.SiteName}
			setString(&status.Account, resolved.AccountName)

			// The same session every other command opens. Building a token
			// one unconditionally reported "invalid token" for an account
			// holding a browser session — while the commands beside it
			// worked, which is the worst way to be wrong.
			capabilities, err := openSessionFor(deps, resolved, token).
				Capabilities(cmd.Context())
			if err != nil {
				// Report the stored identity alongside the failure: knowing
				// which account stopped working is half the diagnosis.
				return err
			}
			// 身分以站台當下的回答為準，不是設定檔裡那筆記錄。憑證可能是
			// 環境變數帶來的，那不屬於任何一個存下來的帳號——用存下來的那筆
			// 就會報出「以 grad1 的身分登入」，而 token 其實是另一個人。
			status.Valid = true
			setString(&status.SiteName, capabilities.SiteName)
			setString(&status.Release, capabilities.Release)
			setString(&status.Username, capabilities.Username)
			setString(&status.UserID, capabilities.UserID)
			setString(&status.FullName, capabilities.FullName)

			return r.Render(Result{
				Envelope: v1.NewEnvelope("auth.status", status, v1.NewMeta(v1.SourceWS)),
				Human: humanLine("%s on %s as %s — credential valid.",
					resolved.AccountName, resolved.SiteName, describeWho(capabilities)),
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
			annotationKind: "auth.logout", annotationSafety: safetyLocal,
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

// envSession reads a browser session from the environment.
func envSession() string { return strings.TrimSpace(os.Getenv(EnvSession)) }

// resolveSession finds the site, account and token a command should use.
//
// A token in the environment bypasses both the configuration and the keychain.
// That is what makes the tool usable in CI and on headless machines, where
// there is no keychain to store anything in — so it must not require an
// account to have been stored first.
// resolveSession finds the site, account and credential a command should use.
//
// A browser session is only looked at when there is no token: a token is the
// better credential wherever one exists, because the AJAX endpoint a session
// has to use exposes far fewer functions.
func resolveSession(deps Deps, file *config.File, siteFlag, accountFlag string) (config.Resolved, string, error) {
	if token := envToken(); token != "" {
		resolved, err := file.Resolve(siteFlag, accountFlag)
		if err != nil {
			return config.Resolved{}, "", err
		}
		return envCredential(resolved, accountFlag), token, nil
	}
	if session := envSession(); session != "" {
		resolved, err := file.Resolve(siteFlag, accountFlag)
		if err != nil {
			return config.Resolved{}, "", err
		}
		return envCredential(resolved, accountFlag), "", nil
	}
	resolved, err := file.RequireAccount(siteFlag, accountFlag)
	if err != nil {
		return config.Resolved{}, "", err
	}
	if resolved.Account.CredentialKind == site.CredentialBrowserSession {
		// The account holds a browser session rather than a token, which is
		// the only credential some sites will give. It is returned empty here
		// and fetched again when the session is opened, so that nothing
		// treats it as a token by accident.
		return resolved, "", nil
	}
	token, err := deps.Auth.Token(resolved.Site.ID, resolved.Account.ID)
	if err != nil {
		return config.Resolved{}, "", err
	}
	return resolved, token, nil
}

// envCredential names the account an environment credential belongs to.
//
// The credential came from the environment, so it is not the stored account's:
// borrowing that account's name would attribute one person's data to another.
// The name is provenance, and a wrong one is worse than a generic one. An
// explicit --account is the user saying which account it is, so that is kept.
func envCredential(resolved config.Resolved, accountFlag string) config.Resolved {
	if accountFlag != "" && resolved.Account != nil {
		return resolved
	}
	resolved.AccountName = "env"
	resolved.Account = &config.Account{}
	return resolved
}

// openSessionFor builds the session a command runs with, choosing between a
// token and a browser session.
func openSessionFor(deps Deps, resolved config.Resolved, token string) *auth.Session {
	target, err := targetSite(resolved.SiteName, resolved.Site)
	if err != nil {
		// The caller already validated the site; reaching here would be a bug,
		// and a nil session would panic further away from the cause.
		return deps.Auth.OpenWithToken(site.Site{}, resolved.Account.ID, token)
	}
	var session *auth.Session
	if token == "" {
		cookie := envSession()
		if cookie == "" && resolved.Account != nil &&
			resolved.Account.CredentialKind == site.CredentialBrowserSession {
			// Stored by `auth import-browser --store`. Read at the moment it
			// is needed, like a token, so it lives in the keychain and in
			// memory and nowhere else.
			stored, err := deps.Auth.Session(resolved.Site.ID, resolved.Account.ID)
			if err == nil {
				cookie = stored
			}
		}
		if cookie != "" {
			session = deps.Auth.OpenWithSession(target, resolved.Account.ID, cookie)
		}
	}
	if session == nil {
		session = deps.Auth.OpenWithToken(target, resolved.Account.ID, token)
	}
	// The flag speaks for this run, the site's setting for every run. Either
	// way the restriction is applied here rather than in each feature, because
	// every feature would otherwise have to remember to ask.
	if wantsWebServiceOnly(deps, resolved) {
		session.RestrictToWebService()
	}
	return session
}

// wantsWebServiceOnly reports whether the fallbacks are ruled out.
func wantsWebServiceOnly(deps Deps, resolved config.Resolved) bool {
	if deps.Backend != nil && *deps.Backend != "" {
		return *deps.Backend == config.BackendWSOnly
	}
	return resolved.Site != nil && resolved.Site.Backend == config.BackendWSOnly
}

// describeWho names the signed-in user with whatever the site said.
//
// A browser session is answered by a page rather than by the web service, and
// a page carries the user's id and not their name. "as  ()" is what printing
// the missing fields anyway produced — which reads as something broken rather
// than as a credential that works.
func describeWho(c *site.Capabilities) string {
	switch {
	case c.FullName != "" && c.Username != "":
		return c.FullName + " (" + c.Username + ")"
	case c.FullName != "":
		return c.FullName
	case c.Username != "":
		return c.Username
	case c.UserID != "":
		return "user " + c.UserID
	default:
		return "an account the site did not name"
	}
}

func setString(target **string, value string) {
	if value == "" {
		return
	}
	copied := value
	*target = &copied
}
