// Package mobilelaunch signs in through the browser the user already uses.
//
// This is the flow Moodle built for its own app: open a page, let the site
// authenticate however it likes — a password, an institutional login, a
// hardware key — and have it hand a token back through a URL scheme the
// client registered.
//
// Everything difficult about it is on the way back. A registered scheme is a
// name any local program can send to, so the arrival of a well-formed
// callback proves nothing; what proves something is that it answers a login
// this process started and has not already finished. That is the transaction
// store's job; the injected broker owns the platform rule that listening must
// begin before the browser is opened.
package mobilelaunch

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/callback"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// ClientFactory builds a transport client for a site.
type ClientFactory func(target site.Site) *moodle.Client

// Broker is the desktop side of a browser handoff. The interface lives here,
// where it is consumed; callback.Broker is the platform adapter.
type Broker interface {
	Scheme() string
	Installed() bool
	OpenAndReceive(context.Context, string, time.Duration, func(bool)) (string, error)
}

// Method signs in through the browser and a registered URL scheme.
type Method struct {
	newClient ClientFactory
	broker    Broker
	store     *callback.Store
	// wait is how long the user has. Generous, because an institutional
	// login can mean a password manager, a second factor and a redirect or
	// three; finite, because a transaction left open is a callback somebody
	// else can still answer.
	wait time.Duration
}

// New builds the method.
func New(newClient ClientFactory, broker Broker) *Method {
	return &Method{
		newClient: newClient,
		broker:    broker,
		store:     callback.NewStore(),
		wait:      callback.DefaultLifetime,
	}
}

func (*Method) Name() string { return "mobilelaunch" }

func (*Method) Describe() string {
	return "sign in in your browser and have the result come back on its own"
}

// Probe reports whether this can work at all.
//
// The handler is checked here rather than at the moment of failure, because
// "nothing came back" is indistinguishable from a dozen other things and
// "you have not installed the handler" is actionable.
func (m *Method) Probe(_ context.Context, _ site.Site, config *auth.PublicConfig) auth.ProbeResult {
	if config == nil {
		return auth.ProbeResult{
			Availability: auth.Unknown,
			Reason:       "the site did not answer",
		}
	}
	if config.EnableMobileWebService != 1 {
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "the mobile web service is disabled on this site",
		}
	}
	if m.broker == nil || !m.broker.Installed() {
		if runtime.GOOS == "darwin" {
			return auth.ProbeResult{
				Availability: auth.Unavailable,
				Reason: "automatic browser callback is not available on macOS; " +
					"try `moodle auth import-browser --browser safari --store` after signing in with Safari, or `moodle auth import-session` if the cookie is absent from disk",
			}
		}
		if runtime.GOOS != "linux" {
			return auth.ProbeResult{
				Availability: auth.Unavailable,
				Reason:       "automatic browser callback is not available on " + runtime.GOOS + "; use `--method manual`",
			}
		}
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "no browser handler is installed; run `moodle auth register-handler`",
		}
	}
	return auth.ProbeResult{Availability: auth.Available}
}

func (m *Method) Authenticate(ctx context.Context, req auth.Request) (auth.Credential, error) {
	if m.broker == nil || !m.broker.Installed() {
		if runtime.GOOS == "darwin" {
			return auth.Credential{}, errs.New(errs.CodeUnavailable,
				"automatic browser callback is not available on macOS").
				WithHint("sign in with Safari, then try `moodle auth import-browser --browser safari --store`; if the cookie is absent from disk, use `moodle auth import-session`")
		}
		if runtime.GOOS != "linux" {
			return auth.Credential{}, errs.New(errs.CodeUnavailable,
				"automatic browser callback is not available on "+runtime.GOOS).
				WithHint("use `--method manual`")
		}
		return auth.Credential{}, errs.New(errs.CodeUnavailable,
			"no browser handler is installed").
			WithHint("run `moodle auth register-handler` once, then try again; " +
				"or use `--method manual`")
	}

	// The site's own idea of its address, not the one the user typed. Moodle
	// computes the callback's hash from this, so an alias or a trailing slash
	// would produce a hash that never matches.
	root := strings.TrimSuffix(req.WWWRoot, "/")
	if root == "" && req.Site.BaseURL != nil {
		root = strings.TrimSuffix(req.Site.BaseURL.String(), "/")
	}
	if root == "" {
		return auth.Credential{}, errs.New(errs.CodeConfiguration,
			"the site's own address is not known yet").
			WithHint("run `moodle doctor` first, which asks the site for it")
	}

	passport := req.Passport
	if passport == "" {
		passport = moodle.NewPassport()
	}
	digest := md5.Sum([]byte(root + passport))
	expected := hex.EncodeToString(digest[:])

	if err := m.store.Begin(callback.Transaction{
		SiteHash: expected, WWWRoot: root, Passport: passport,
		Expires: time.Now().Add(m.wait),
	}); err != nil {
		return auth.Credential{}, err
	}

	address := moodle.LaunchURL(root, moodle.MobileService, passport, m.broker.Scheme())
	uri, err := m.broker.OpenAndReceive(ctx, address, m.wait, func(viaPortal bool) {
		m.announce(req, address, viaPortal)
	})
	if err != nil {
		return auth.Credential{}, err
	}
	return m.accept(ctx, req, strings.TrimSpace(uri), expected, root)
}

// accept checks a callback before anything is kept.
func (m *Method) accept(ctx context.Context, req auth.Request, uri, expected, root string) (auth.Credential, error) {
	parsed, err := moodle.ParseTokenCallback(uri)
	if err != nil {
		return auth.Credential{}, err
	}
	// Against a live transaction, once. Moodle's own source says a passport
	// is valid one time, but nothing on the server enforces it — so a
	// captured callback would work twice unless this refuses.
	transaction, err := m.store.Claim(parsed.SiteHash)
	if err != nil {
		return auth.Credential{}, err
	}
	if transaction.SiteHash != expected {
		return auth.Credential{}, errs.New(errs.CodeValidation,
			"that callback answers a different sign-in")
	}
	if err := moodle.VerifyPassport(parsed, root, transaction.Passport); err != nil {
		return auth.Credential{}, err
	}

	// And the token itself, against the site it claims to be for. A hash that
	// matches says the callback belongs to this attempt; it does not say the
	// token works, and storing one that does not would leave the user signed
	// in to nothing.
	if _, err := m.newClient(req.Site).SiteInfo(ctx, parsed.Token, ""); err != nil {
		return auth.Credential{}, errs.Wrap(errs.CodeAuthentication, err,
			"the site did not accept the token it just issued")
	}
	return auth.Credential{
		Token:        parsed.Token,
		PrivateToken: parsed.PrivateToken,
		Method:       "mobilelaunch",
	}, nil
}

// announce tells the user what is happening and what their browser is doing.
//
// A command that opens a browser and then sits silently looks like a command
// that has hung.
func (m *Method) announce(req auth.Request, address string, viaPortal bool) {
	if req.Out == nil {
		return
	}
	fmt.Fprintln(req.Out, "Opening your browser to sign in.")
	fmt.Fprintln(req.Out, "When you are done there, this will continue on its own.")
	if !viaPortal {
		// The address carries the passport, which ties the callback to this
		// attempt. The portal takes it as a message; a command takes it as an
		// argument, and /proc shows arguments to other users on this machine.
		fmt.Fprintln(req.Out,
			"\nNote: this desktop has no portal, so the address was passed to a")
		fmt.Fprintln(req.Out,
			"command. Another user on this machine could have seen it while it ran.")
	}
	fmt.Fprintf(req.Out, "\nIf nothing opens, go to:\n  %s\n\n", address)
}
