package moodle

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// PathLaunch is the endpoint that turns a browser login into a token.
const PathLaunch = "/admin/tool/mobile/launch.php"

// DefaultURLScheme is the scheme Moodle redirects to when the client asks for
// nothing else. A site administrator can force a different one, and that
// setting is not visible before signing in.
const DefaultURLScheme = "moodlemobile"

// NewPassport returns the one-time value that ties a callback to this login.
//
// Moodle echoes it back hashed with the site root, which is how a reply can be
// shown to belong to the request that started it.
func NewPassport() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("moodle: cannot read random bytes: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// LaunchURL builds the URL a user opens to sign in through their browser.
func LaunchURL(siteRoot, service, passport, urlScheme string) string {
	if service == "" {
		service = MobileService
	}
	query := url.Values{}
	query.Set("service", service)
	query.Set("passport", passport)
	if urlScheme != "" {
		query.Set("urlscheme", urlScheme)
	}
	return strings.TrimSuffix(siteRoot, "/") + PathLaunch + "?" + query.Encode()
}

// TokenCallback is what Moodle sends back to the client.
type TokenCallback struct {
	// SiteHash is md5(wwwroot + passport) as Moodle computed it.
	SiteHash     string
	Token        string
	PrivateToken string
}

// maxCallbackBytes bounds the callback. Moodle's payload is a hash, a token
// and sometimes a second token — a few hundred bytes. Anything far larger did
// not come from Moodle, and the point of a bound is to stop reading before
// finding out what it is instead.
const maxCallbackBytes = 8 << 10

// callbackScheme is Moodle's own rule for what it will redirect to, copied
// from launch.php. A scheme outside it cannot have come from Moodle.
var callbackScheme = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-+.]*$`)

// siteHashPattern is what md5 produces. Moodle computes the hash itself, so a
// value of another shape did not come from the exchange this is part of.
var siteHashPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ParseTokenCallback reads a "<scheme>://token=<base64>" callback.
//
// It is deliberately strict about shape while staying quiet about which
// scheme: a site may force its own, so rejecting an unexpected name would
// stop a user who did everything right. Everything else is checked, because
// this is the door a registered URL handler opens — and a registered handler
// is a door any local process, or any web page, can knock on. What passes
// here still has to match a live transaction; this only refuses to carry
// something shaped wrong that far.
func ParseTokenCallback(raw string) (TokenCallback, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TokenCallback{}, errs.New(errs.CodeUsage, "the callback URL is empty")
	}
	if len(trimmed) > maxCallbackBytes {
		return TokenCallback{}, errs.New(errs.CodeValidation,
			"the callback URL is far longer than Moodle ever sends")
	}

	// Not url.Parse: for "scheme://token=x" Go puts "token=x" in Host and
	// leaves Query empty, so reading it as a query parameter finds nothing.
	// The wire format is fixed, so it is matched as written.
	scheme, rest, found := strings.Cut(trimmed, "://")
	if !found || !callbackScheme.MatchString(scheme) {
		return TokenCallback{}, errs.New(errs.CodeUsage,
			"that does not look like a Moodle login callback").
			WithHint(`it should look like "moodlemobile://token=..."`)
	}
	encoded, ok := strings.CutPrefix(rest, "token=")
	if !ok {
		return TokenCallback{}, errs.New(errs.CodeUsage,
			"that does not look like a Moodle login callback").
			WithHint(`it should look like "moodlemobile://token=..."`)
	}
	encoded = strings.TrimSpace(encoded)
	// Moodle appends nothing after the payload. A path, query or fragment
	// means something rewrote this on the way.
	if strings.ContainsAny(encoded, "/?#") {
		return TokenCallback{}, errs.New(errs.CodeValidation,
			"the callback carries more than Moodle sends").
			WithReason(errs.ReasonProtocolDrift)
	}

	decoded, err := decodeBase64(encoded)
	if err != nil {
		return TokenCallback{}, errs.Wrap(errs.CodeUsage, err,
			"the callback payload is not valid base64").
			WithHint("copy the whole URL, including everything after token=")
	}

	// siteid:::token[:::privatetoken], and never anything else: Moodle builds
	// this string itself, so a fourth field is not a newer Moodle, it is
	// something that is not Moodle.
	parts := strings.Split(string(decoded), ":::")
	if len(parts) < 2 || len(parts) > 3 {
		return TokenCallback{}, errs.New(errs.CodeUsage,
			"the callback payload is not in the expected form").
			WithReason(errs.ReasonProtocolDrift)
	}
	if !siteHashPattern.MatchString(parts[0]) {
		return TokenCallback{}, errs.New(errs.CodeValidation,
			"the callback does not identify a site the way Moodle does").
			WithReason(errs.ReasonProtocolDrift)
	}
	callback := TokenCallback{SiteHash: parts[0], Token: parts[1]}
	if len(parts) > 2 {
		callback.PrivateToken = parts[2]
	}
	if callback.Token == "" {
		return TokenCallback{}, errs.New(errs.CodeUsage, "the callback carries no token")
	}
	return callback, nil
}

// decodeBase64 accepts both the padded and unpadded forms, because what a user
// pastes depends on their browser and on how they copied it.
func decodeBase64(encoded string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(encoded)
}

// VerifyPassport checks that a callback belongs to the login that started it.
//
// Without this, any "moodlemobile://token=..." a local process could produce
// would be accepted, including one aimed at a different site.
func VerifyPassport(callback TokenCallback, wwwRoot, passport string) error {
	expected := md5.Sum([]byte(strings.TrimSuffix(wwwRoot, "/") + passport))
	want := hex.EncodeToString(expected[:])
	// Constant time is not strictly required for a public hash, but the
	// comparison is a security decision and should not invite a timing habit.
	if subtle.ConstantTimeCompare([]byte(want), []byte(callback.SiteHash)) != 1 {
		return errs.New(errs.CodeValidation,
			"this callback does not belong to this login attempt").
			WithHint(fmt.Sprintf(
				"the site identified itself as %q; check the site URL is exactly %s",
				callback.SiteHash, wwwRoot))
	}
	return nil
}

// SessionCookie is a browser session this tool has been given.
//
// It is the credential a logged-in browser holds. Handing it to the wrong host
// hands over the account, so it is only ever sent to the site it came from.
type SessionCookie struct {
	// Name is usually MoodleSession, but a site can rename it.
	Name  string
	Value string
}

// DefaultSessionCookieName is what Moodle calls its session cookie unless a
// site has changed $CFG->sessioncookie.
const DefaultSessionCookieName = "MoodleSession"

// ParseSessionCookie reads the forms people actually copy.
//
// Developer tools hand over the whole Cookie header as often as the pair alone
// or the bare value, and refusing any of them would be a pointless round trip.
func ParseSessionCookie(raw string) SessionCookie {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return SessionCookie{}
	}
	for _, part := range strings.Split(trimmed, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if found && strings.EqualFold(strings.TrimSpace(key), DefaultSessionCookieName) {
			return SessionCookie{
				Name:  strings.TrimSpace(key),
				Value: strings.TrimSpace(value),
			}
		}
	}
	if key, value, found := strings.Cut(trimmed, "="); found {
		return SessionCookie{Name: strings.TrimSpace(key), Value: strings.TrimSpace(value)}
	}
	return SessionCookie{Name: DefaultSessionCookieName, Value: trimmed}
}

// ExchangeSession turns a browser session into a web service token.
//
// Moodle's launch endpoint answers a signed-in browser with a redirect to a
// custom scheme carrying the token. There is no need to follow it — and no way
// to, since Go cannot fetch a "moodlemobile://" URL — so the redirect is read
// rather than obeyed.
//
// This is the whole of what a browser session buys: one exchange, after which
// the session is not needed again. The token that comes back is an ordinary
// web service token with no shorter life than any other.
func (c *Client) ExchangeSession(ctx context.Context, cookie SessionCookie, passport, urlScheme string) (TokenCallback, error) {
	if strings.TrimSpace(cookie.Value) == "" {
		return TokenCallback{}, errs.New(errs.CodeUsage, "no session cookie given")
	}
	name := strings.TrimSpace(cookie.Name)
	if name == "" {
		name = DefaultSessionCookieName
	}

	target := LaunchURL(c.site.Endpoint(""), MobileService, passport, urlScheme)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return TokenCallback{}, errs.Wrap(errs.CodeInternal, err, "cannot build the launch request")
	}
	request.AddCookie(&http.Cookie{Name: name, Value: cookie.Value})

	response, err := c.redirectResponse(request, "launch.php")
	if err != nil {
		return TokenCallback{}, err
	}
	defer response.Body.Close()

	location := response.Header.Get("Location")
	if response.StatusCode < 300 || response.StatusCode > 399 || location == "" {
		// A refusal here is not evidence that the session is bad, and saying
		// so was wrong: launch.php also refuses a perfectly good session.
		// Moodle only performs this exchange while $SESSION->justloggedin is
		// still set, and the first page the session loads clears it — measured
		// on 4.5, 5.1 and 5.2: sign in, read /my/, and the exchange returns
		// pluginnotenabledorconfigured for a session that /my/ just answered
		// with 200. A cookie copied out of a browser someone has been using is
		// in that state by definition, so "expired, or from another site" is
		// the one reading the evidence does not support.
		return TokenCallback{}, errs.New(errs.CodeAuthentication,
			"the site would not exchange that browser session for a token").
			WithReason(errs.ReasonTokenExpired).
			WithHint("Moodle only offers this exchange to a session that has just " +
				"signed in, unless the site allows mobile login through a browser; " +
				"the session may also have expired or belong to a different site")
	}
	return ParseTokenCallback(location)
}
