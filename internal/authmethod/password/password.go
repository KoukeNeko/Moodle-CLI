// Package password signs in with a Moodle username and password.
//
// It only works for accounts Moodle authenticates itself. An account behind
// SSO cannot use it: the identity provider, not Moodle, holds the password
//.
package password

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// ClientFactory builds a transport client for a site.
type ClientFactory func(target site.Site) *moodle.Client

// Method signs in with a username and password.
type Method struct {
	newClient ClientFactory
	// readPassword reads without echoing. It is injected so tests do not need
	// a terminal.
	readPassword func() (string, error)
}

// New builds the method.
func New(newClient ClientFactory, readPassword func() (string, error)) *Method {
	return &Method{newClient: newClient, readPassword: readPassword}
}

func (*Method) Name() string { return "password" }

func (*Method) Describe() string {
	return "sign in with a Moodle username and password (not for SSO accounts)"
}

func (*Method) Probe(_ context.Context, _ site.Site, config *auth.PublicConfig) auth.ProbeResult {
	if config == nil {
		return auth.ProbeResult{Availability: auth.Unknown, Reason: "the site did not answer"}
	}
	if config.EnableMobileWebService != 1 {
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "the mobile web service is disabled on this site",
		}
	}
	if config.ShowLoginForm != 1 {
		// The site hides its own login form, which means it expects an
		// identity provider to handle sign-in.
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "this site does not offer a password login form",
		}
	}
	if config.TypeOfLogin != 1 && config.HasIdentityProviders {
		// Possible, but probably not what the user wants: their account is
		// most likely held by the identity provider.
		return auth.ProbeResult{
			Availability: auth.Unknown,
			Reason:       "this site uses single sign-on; a Moodle password may not exist for your account",
		}
	}
	return auth.ProbeResult{Availability: auth.Available}
}

func (m *Method) Authenticate(ctx context.Context, req auth.Request) (auth.Credential, error) {
	username := strings.TrimSpace(req.Username)
	password := req.Password

	if username == "" {
		var err error
		username, err = prompt(req, "Moodle username: ")
		if err != nil {
			return auth.Credential{}, err
		}
	}
	if password == "" {
		if m.readPassword == nil {
			return auth.Credential{}, errs.New(errs.CodeUsage, "no password given").
				WithHint("pass --password-stdin to read it from a pipe")
		}
		fmt.Fprint(req.Out, "Moodle password: ")
		var err error
		password, err = m.readPassword()
		fmt.Fprintln(req.Out)
		if err != nil {
			return auth.Credential{}, errs.Wrap(errs.CodeUsage, err, "cannot read the password")
		}
	}
	if username == "" || password == "" {
		return auth.Credential{}, errs.New(errs.CodeUsage, "a username and password are required")
	}

	token, privateToken, err := m.newClient(req.Site).
		LoginToken(ctx, username, password, moodle.MobileService)
	if err != nil {
		return auth.Credential{}, err
	}
	return auth.Credential{Token: token, PrivateToken: privateToken, Method: "password"}, nil
}

func prompt(req auth.Request, label string) (string, error) {
	if req.In == nil {
		return "", errs.New(errs.CodeUsage, "no username given").
			WithHint("pass --username <name>")
	}
	fmt.Fprint(req.Out, label)
	reader := bufio.NewReader(req.In)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", errs.Wrap(errs.CodeUsage, err, "cannot read the username")
	}
	return strings.TrimSpace(line), nil
}
