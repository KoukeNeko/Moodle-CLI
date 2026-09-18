// Package manual signs in by having the user complete the browser login and
// paste the callback URL back.
//
// It is the fallback of last resort and works everywhere, including where no
// URL scheme can be registered — a headless machine, a locked-down desktop, or
// a macOS install without the .app wrapper.
package manual

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

// Method signs in through a pasted callback URL.
type Method struct{}

// New builds the method.
func New() *Method { return &Method{} }

func (*Method) Name() string { return "manual" }

func (*Method) Describe() string {
	return "open the login page yourself and paste the callback URL back"
}

func (*Method) Probe(_ context.Context, _ site.Site, config *auth.PublicConfig) auth.ProbeResult {
	if config == nil {
		// Worth offering even when the site says nothing: this is the method
		// that works when everything else has failed.
		return auth.ProbeResult{Availability: auth.Unknown, Reason: "the site did not answer"}
	}
	if config.EnableMobileWebService != 1 {
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "the mobile web service is disabled on this site",
		}
	}
	return auth.ProbeResult{Availability: auth.Available}
}

func (*Method) Authenticate(_ context.Context, req auth.Request) (auth.Credential, error) {
	passport := req.Passport
	if passport == "" {
		passport = moodle.NewPassport()
	}
	root := req.WWWRoot
	if root == "" && req.Site.BaseURL != nil {
		root = req.Site.BaseURL.String()
	}

	callback := strings.TrimSpace(req.Callback)
	if callback == "" {
		if req.In == nil {
			return auth.Credential{}, errs.New(errs.CodeUsage,
				"no callback URL given and nothing to read it from").
				WithHint("pass --callback '<scheme>://token=...'")
		}
		launch := moodle.LaunchURL(root, moodle.MobileService, passport, "")
		// Instructions are diagnostics: they go to stderr so that stdout
		// still carries only the result.
		fmt.Fprintf(req.Out, "Open this address in your browser and sign in:\n\n  %s\n\n", launch)
		fmt.Fprint(req.Out,
			"Your browser will try to hand the result to an app and may show an error;\n"+
				"that is expected. Copy the address it tried to open and paste it here.\n\n"+
				"Callback URL: ")

		line, err := bufio.NewReader(req.In).ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return auth.Credential{}, errs.Wrap(errs.CodeUsage, err, "cannot read the callback URL")
		}
		callback = strings.TrimSpace(line)
	}

	parsed, err := moodle.ParseTokenCallback(callback)
	if err != nil {
		return auth.Credential{}, err
	}
	if req.Passport != "" || req.Callback == "" {
		// Only meaningful when this process generated the passport. A
		// callback supplied outright cannot be tied to anything.
		if err := moodle.VerifyPassport(parsed, root, passport); err != nil {
			return auth.Credential{}, err
		}
	}
	return auth.Credential{
		Token:        parsed.Token,
		PrivateToken: parsed.PrivateToken,
		Method:       "manual",
	}, nil
}
