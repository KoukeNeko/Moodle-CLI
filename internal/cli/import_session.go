package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// import-session is the fallback when an active browser cookie is not present
// in its on-disk profile. In particular, Safari's cookie file is not an API
// for the running browser and cannot be relied on for session-only cookies.
func newAuthImportSessionCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteName, accountName, cookieName string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "import-session",
		Short: "Verify and store a Moodle session pasted privately from your browser",
		Long: "Read one Moodle session cookie without placing its value in the command line. " +
			"When run in a terminal, the input is hidden; use --stdin for a pipe. " +
			"The session is verified with the selected Moodle site before storage in the OS keychain. " +
			"Never paste a session into chat, a shell command, or an issue report.",
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "auth.login",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			input, err := readPrivateSession(cmd, r, fromStdin)
			if err != nil {
				return err
			}
			cookie, err := normalizeSessionCookie(input, cookieName)
			if err != nil {
				return err
			}
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(siteName, "")
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}
			name, userID, err := storeImportedSession(cmd.Context(), deps, file, resolved, target, accountName, cookie, "import-session")
			if err != nil {
				return err
			}
			status := v1.AuthStatus{Site: resolved.SiteName, Valid: true}
			setString(&status.Account, name)
			setString(&status.UserID, userID)
			return r.Render(Result{
				Envelope: v1.NewEnvelope("auth.login", status, v1.NewMeta(v1.SourceAJAX)),
				Human: humanLine(
					"Stored the browser session for %s as account %q (user %s).\n"+
						"Signing out in the browser may invalidate this session too.",
					resolved.SiteName, name, userID),
			})
		},
	}
	cmd.Flags().StringVar(&siteName, "site", "", "site to import the session for")
	cmd.Flags().StringVar(&accountName, "account", "", "name for the imported account (default: browser-<user id>)")
	cmd.Flags().StringVar(&cookieName, "cookie-name", "MoodleSession", "session cookie name, if the site renamed it")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read the cookie value or name=value from stdin instead of a hidden terminal prompt")
	return cmd
}

func readPrivateSession(cmd *cobra.Command, r *Renderer, fromStdin bool) (string, error) {
	if fromStdin {
		data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 4097))
		if err != nil {
			return "", errs.Wrap(errs.CodeUsage, err, "cannot read the session from stdin")
		}
		if len(data) > 4096 {
			return "", errs.New(errs.CodeUsage, "session input is too long")
		}
		return strings.TrimSpace(string(data)), nil
	}
	input, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) {
		return "", errs.New(errs.CodeUsage, "no interactive terminal for a hidden session prompt").
			WithHint("pipe the cookie value to `moodle auth import-session --stdin`; never put it in command arguments")
	}
	fmt.Fprint(r.Streams.Err, "Paste Moodle session cookie (input hidden): ")
	data, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(r.Streams.Err)
	if err != nil {
		return "", errs.Wrap(errs.CodeUsage, err, "cannot read the hidden session")
	}
	if len(data) > 4096 {
		return "", errs.New(errs.CodeUsage, "session input is too long")
	}
	return strings.TrimSpace(string(data)), nil
}

func normalizeSessionCookie(raw, name string) (string, error) {
	if name == "" {
		return "", errs.New(errs.CodeUsage, "session cookie name is empty")
	}
	if strings.ContainsAny(raw, ";\r\n") {
		return "", errs.New(errs.CodeUsage, "paste only one cookie value or name=value pair")
	}
	value := raw
	if suppliedName, suppliedValue, hasName := strings.Cut(raw, "="); hasName {
		if suppliedName != name {
			return "", errs.New(errs.CodeUsage, "pasted cookie name does not match --cookie-name")
		}
		value = suppliedValue
	}
	if value == "" || !safeCookieName(name) || !safeCookieValue(value) {
		return "", errs.New(errs.CodeUsage, "invalid or empty session cookie")
	}
	return name + "=" + value, nil
}

func safeCookieName(name string) bool {
	if name == "" {
		return false
	}
	for _, ch := range name {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' ||
			ch >= '0' && ch <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", ch)) {
			return false
		}
	}
	return true
}

func safeCookieValue(value string) bool {
	for _, ch := range value {
		if ch < '!' || ch > '~' || ch == '"' || ch == ';' || ch == '\\' {
			return false
		}
	}
	return true
}

// Store only a session the actual Moodle site accepted, and never include its
// value in an error or human result. Both import paths share this boundary.
func storeImportedSession(ctx context.Context, deps Deps, file *config.File, resolved config.Resolved, target site.Site, requestedName, cookie, method string) (string, string, error) {
	opened := deps.Auth.OpenWithSession(target, "", cookie)
	capabilities, err := opened.Capabilities(ctx)
	if err != nil {
		return "", "", err
	}
	if capabilities.UserID == "" {
		return "", "", errs.New(errs.CodeAuthentication,
			"that session was not accepted by "+resolved.SiteName).
			WithHint("copy the active session cookie from the selected browser and try again")
	}
	name := requestedName
	if name == "" {
		name = "browser-" + capabilities.UserID
	}
	account := resolved.Site.UpsertAccount(name, config.Account{
		UserID:         capabilities.UserID,
		AuthMethod:     method,
		CredentialKind: site.CredentialBrowserSession,
	})
	if err := deps.Auth.StoreSession(resolved.Site.ID, account.ID, cookie); err != nil {
		return "", "", err
	}
	file.Current.Site = resolved.SiteName
	file.Current.Account = name
	if err := file.Save(); err != nil {
		return "", "", err
	}
	return name, capabilities.UserID, nil
}
