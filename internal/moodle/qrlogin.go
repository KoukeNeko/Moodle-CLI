package moodle

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// FunctionQRTokens exchanges a QR login key for a web service token.
const FunctionQRTokens = "tool_mobile_get_tokens_for_qr_login"

// MoodleAppUserAgent is sent for the QR exchange and for nothing else.
//
// Moodle rejects that endpoint unless the caller's User-Agent contains
// "MoodleMobile". The check exists to stop a web page from stealing a QR key
// through XSS; a CLI is not a browser, so it protects nothing here, but
// without it QR login simply cannot be used. Scope is therefore limited to the
// single request, and every other request identifies this tool honestly
//.
const MoodleAppUserAgent = "MoodleMobile 4.5.0 (44500)"

// QRLogin is the content of a Moodle login QR code.
type QRLogin struct {
	SiteURL string
	Key     string
	UserID  string
}

// ParseQRLogin reads a "<scheme>://<wwwroot>?qrlogin=<key>&userid=<id>" URI.
func ParseQRLogin(raw string) (QRLogin, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return QRLogin{}, errs.New(errs.CodeUsage, "the QR content is empty")
	}
	// Strip the custom scheme so the rest parses as an ordinary URL.
	if _, rest, found := strings.Cut(trimmed, "://"); found {
		trimmed = rest
	}
	parsed, err := url.Parse("https://" + strings.TrimPrefix(trimmed, "https://"))
	if err != nil {
		return QRLogin{}, errs.Wrap(errs.CodeUsage, err, "cannot read the QR content")
	}

	key := parsed.Query().Get("qrlogin")
	userID := parsed.Query().Get("userid")
	if key == "" || userID == "" {
		return QRLogin{}, errs.New(errs.CodeUsage,
			"that QR code is not a login code").
			WithHint("the site must have QR login enabled; a QR that only holds the site address cannot sign you in")
	}
	if _, err := strconv.Atoi(userID); err != nil {
		return QRLogin{}, errs.New(errs.CodeUsage, "the QR code carries an invalid user id")
	}

	parsed.RawQuery = ""
	return QRLogin{
		SiteURL: strings.TrimSuffix(parsed.String(), "/"),
		Key:     key,
		UserID:  userID,
	}, nil
}

// ExchangeQRLogin turns a QR login key into a web service token.
//
// The key is single use and short lived — Moodle's default is ten minutes, and
// a site can shorten it — so a stale code fails with invalidkey rather than
// anything more descriptive.
func (c *Client) ExchangeQRLogin(ctx context.Context, login QRLogin) (token, privateToken string, err error) {
	var reply struct {
		Token        string `json:"token"`
		PrivateToken string `json:"privatetoken"`
	}
	// The app User-Agent is applied to this client copy only, so it cannot
	// leak into any other request.
	appClient := *c
	appClient.userAgent = MoodleAppUserAgent

	err = appClient.CallNoLogin(ctx, FunctionQRTokens, map[string]any{
		"qrloginkey": login.Key,
		"userid":     login.UserID,
	}, &reply)
	if err != nil {
		return "", "", explainQRFailure(err)
	}
	if reply.Token == "" {
		return "", "", errs.New(errs.CodeUpstream, "the site returned no token").
			WithReason(errs.ReasonProtocolDrift)
	}
	return reply.Token, reply.PrivateToken, nil
}

// explainQRFailure turns Moodle's terse codes into something a user can act on.
func explainQRFailure(err error) error {
	e := errs.From(err)
	if e.Upstream == nil {
		return err
	}
	switch e.Upstream.ErrorCode {
	case "invalidkey":
		return e.WithHint(
			"the QR code is single use and expires within minutes; refresh the page and scan a fresh one")
	case "apprequired":
		return e.WithHint("this site refuses QR login from anything but the Moodle app")
	case "httpsrequired":
		return e.WithHint("QR login only works on sites served over https")
	case "autologinnotallowedtoadmins":
		return e.WithHint("Moodle refuses QR login for site administrators; use a normal account")
	}
	return e
}
