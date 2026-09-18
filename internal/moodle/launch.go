package moodle

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// PathLaunch is the endpoint that turns a browser login into a token.
const PathLaunch = "/admin/tool/mobile/launch.php"

// DefaultURLScheme is the scheme Moodle redirects to when the client asks for
// nothing else. A site administrator can force a different one, and that
// setting is not visible before signing in
//.
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

// ParseTokenCallback reads a "<scheme>://token=<base64>" callback.
//
// Any scheme is accepted: the site may force its own, and rejecting an
// unexpected one would only stop a user who did everything right.
func ParseTokenCallback(raw string) (TokenCallback, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TokenCallback{}, errs.New(errs.CodeUsage, "the callback URL is empty")
	}

	// The payload is everything after "token=", whatever the scheme.
	_, encoded, found := strings.Cut(trimmed, "token=")
	if !found {
		return TokenCallback{}, errs.New(errs.CodeUsage,
			"that does not look like a Moodle login callback").
			WithHint(`it should look like "moodlemobile://token=..."`)
	}
	encoded = strings.TrimSpace(encoded)

	decoded, err := decodeBase64(encoded)
	if err != nil {
		return TokenCallback{}, errs.Wrap(errs.CodeUsage, err,
			"the callback payload is not valid base64").
			WithHint("copy the whole URL, including everything after token=")
	}

	// siteid:::token[:::privatetoken]
	parts := strings.Split(string(decoded), ":::")
	if len(parts) < 2 {
		return TokenCallback{}, errs.New(errs.CodeUsage,
			"the callback payload is not in the expected form").
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
