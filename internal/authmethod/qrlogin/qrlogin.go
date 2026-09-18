// Package qrlogin signs in with the QR code Moodle shows in a user's profile.
//
// It is the friendliest option for an SSO account: the browser has already
// authenticated the user, no cookie has to be read from disk, and no URL
// scheme has to be registered.
package qrlogin

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

// Method signs in with a QR login code.
type Method struct {
	newClient ClientFactory
}

// New builds the method.
func New(newClient ClientFactory) *Method { return &Method{newClient: newClient} }

func (*Method) Name() string { return "qr" }

func (*Method) Describe() string {
	return "paste the login QR code from your Moodle profile (works with SSO)"
}

func (*Method) Probe(_ context.Context, target site.Site, config *auth.PublicConfig) auth.ProbeResult {
	if config == nil {
		return auth.ProbeResult{Availability: auth.Unknown, Reason: "the site did not answer"}
	}
	if config.QRCodeType != 2 {
		// Type 1 is a QR that only holds the site address; it cannot sign
		// anyone in.
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "QR login is not enabled on this site",
		}
	}
	if target.BaseURL != nil && target.BaseURL.Scheme != "https" {
		// Moodle refuses the exchange outright over plain http.
		return auth.ProbeResult{
			Availability: auth.Unavailable,
			Reason:       "QR login requires https",
		}
	}
	return auth.ProbeResult{Availability: auth.Available}
}

func (m *Method) Authenticate(ctx context.Context, req auth.Request) (auth.Credential, error) {
	raw := strings.TrimSpace(req.QR)
	if raw == "" {
		if req.In == nil {
			return auth.Credential{}, errs.New(errs.CodeUsage, "no QR content given").
				WithHint("pass --qr 'moodlemobile://...?qrlogin=...&userid=...'")
		}
		fmt.Fprint(req.Out,
			"In Moodle, open your profile and choose \"Mobile app\" to show the login QR code.\n"+
				"Decode it with any QR reader and paste the result here.\n"+
				"The code is single use and expires within minutes.\n\n"+
				"QR content: ")
		line, err := bufio.NewReader(req.In).ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return auth.Credential{}, errs.Wrap(errs.CodeUsage, err, "cannot read the QR content")
		}
		raw = strings.TrimSpace(line)
	}

	login, err := moodle.ParseQRLogin(raw)
	if err != nil {
		return auth.Credential{}, err
	}
	token, privateToken, err := m.newClient(req.Site).ExchangeQRLogin(ctx, login)
	if err != nil {
		return auth.Credential{}, err
	}
	return auth.Credential{Token: token, PrivateToken: privateToken, Method: "qr"}, nil
}
