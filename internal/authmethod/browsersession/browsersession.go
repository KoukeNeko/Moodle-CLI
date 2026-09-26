// Package browsersession signs in with a session a browser already holds.
//
// It exists for the sites a token cannot be got from any other way: single
// sign-on, two-factor, a captcha — anything where the login happens in a
// browser and there is no password this tool could send. The browser has
// already done the hard part; this turns what it holds into an ordinary web
// service token, once, and then no longer needs it.
package browsersession

import (
	"context"
	"fmt"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Method exchanges a browser session for a token.
type Method struct {
	newClient  func(site.Site) *moodle.Client
	readHidden func() (string, error)
}

// New builds the method.
// New builds the method. readHidden reads a line from the terminal without
// echoing it; nil leaves the method unable to prompt, which is what a caller
// with no terminal wants.
func New(newClient func(site.Site) *moodle.Client, readHidden func() (string, error)) *Method {
	return &Method{newClient: newClient, readHidden: readHidden}
}

func (*Method) Name() string { return "browser-session" }

func (*Method) Describe() string {
	return "exchange a session your browser already holds for a token"
}

// Probe reports whether the exchange could work.
//
// It is never chosen automatically: the caller has to hand over a session
// cookie, and a credential that powerful is not something to go looking for on
// someone's behalf.
func (*Method) Probe(_ context.Context, _ site.Site, config *auth.PublicConfig) auth.ProbeResult {
	if config == nil {
		return auth.ProbeResult{
			Availability: auth.Unknown,
			Reason:       "the site did not answer",
		}
	}
	if config.EnableMobileWebService != 1 {
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "mobile web services are switched off, so no token can be issued",
		}
	}
	return auth.ProbeResult{Availability: auth.Available}
}

func (m *Method) Authenticate(ctx context.Context, req auth.Request) (auth.Credential, error) {
	raw := req.SessionCookie
	if strings.TrimSpace(raw) == "" {
		// Asked for rather than refused, so that choosing this method from a
		// menu leads somewhere. Hidden, because a session cookie is a
		// credential that outlives the command: echoing it would leave it in
		// the terminal's scrollback and in any recording of the session.
		if req.In == nil || m.readHidden == nil {
			return auth.Credential{}, errs.New(errs.CodeUsage,
				"no browser session given").
				WithHint("pass --session-cookie 'MoodleSession=…', copied from the browser " +
					"that is already signed in, or --session-cookie-stdin to pipe it")
		}
		fmt.Fprint(req.Out,
			"Open this site in a browser you are already signed in to, then copy the\n"+
				"MoodleSession cookie's value from its developer tools.\n"+
				"Storage or Application → Cookies → MoodleSession.\n\n"+
				"Paste the session (input hidden): ")
		read, err := m.readHidden()
		fmt.Fprintln(req.Out)
		if err != nil {
			return auth.Credential{}, errs.Wrap(errs.CodeUsage, err, "cannot read the session")
		}
		raw = read
	}
	cookie := moodle.ParseSessionCookie(raw)
	if cookie.Value == "" {
		return auth.Credential{}, errs.New(errs.CodeUsage,
			"that is not a session cookie").
			WithHint("paste the MoodleSession value, or MoodleSession=<value>")
	}

	passport := req.Passport
	if passport == "" {
		passport = moodle.NewPassport()
	}
	root := req.WWWRoot
	if root == "" && req.Site.BaseURL != nil {
		root = req.Site.BaseURL.String()
	}

	callback, err := m.newClient(req.Site).ExchangeSession(ctx,
		cookie, passport, moodle.DefaultURLScheme)
	if err != nil {
		return auth.Credential{}, err
	}
	// The passport was generated here, so the reply can be tied to the request
	// that asked for it. Without this any local process could hand over a
	// token, including one minted for another site.
	if err := moodle.VerifyPassport(callback, root, passport); err != nil {
		return auth.Credential{}, err
	}

	return auth.Credential{
		Token:        callback.Token,
		PrivateToken: callback.PrivateToken,
		Method:       "browser-session",
	}, nil
}
