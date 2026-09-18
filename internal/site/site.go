// Package site holds the core vocabulary: which Moodle we are talking to,
// as whom, and what that combination can actually do.
//
// It is pure: no HTTP, no files, no keychain. Adapters depend on it, not the
// other way round.
package site

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// ID is a locally generated identifier for a site or an account.
//
// It is deliberately not derived from the site URL or the Moodle user id:
// those change (http→https, a renamed account), and a credential keyed on
// them would be orphaned when they do.
type ID string

// NewID returns a random RFC 4122 version 4 identifier.
func NewID() ID {
	var b [16]byte
	// crypto/rand.Read never returns an error on any supported platform, and
	// a failure here would mean the process cannot generate secrets at all.
	if _, err := rand.Read(b[:]); err != nil {
		panic("site: cannot read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return ID(h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32])
}

// Site is one Moodle installation.
type Site struct {
	ID   ID
	Name string
	// BaseURL is what the user typed.
	BaseURL *url.URL
	// WWWRoot is the canonical root Moodle reports for itself. It can differ
	// from BaseURL, and the SSO payload hash is computed over this one
	//.
	WWWRoot *url.URL
}

// Account is one Moodle user on one Site.
type Account struct {
	ID       ID
	SiteID   ID
	Name     string
	UserID   string // Moodle's user id, metadata only; a string, never a number
	Username string
	FullName string
}

// CredentialKind names what kind of credential authenticates a request.
type CredentialKind string

const (
	CredentialWSToken        CredentialKind = "ws_token"
	CredentialBrowserSession CredentialKind = "browser_session"
)

// BackendKind names how a request reaches Moodle.
type BackendKind string

const (
	BackendWS   BackendKind = "ws"
	BackendAJAX BackendKind = "ajax"
	BackendHTML BackendKind = "html"
)

// ParseBaseURL normalises a user-supplied site address.
func ParseBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errs.New(errs.CodeUsage, "site URL is empty")
	}
	if !strings.Contains(trimmed, "://") {
		// A bare host is almost always meant as https; assuming http would
		// silently downgrade a site that only serves TLS.
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUsage, err, fmt.Sprintf("cannot parse site URL %q", raw))
	}
	if parsed.Host == "" {
		return nil, errs.New(errs.CodeUsage, fmt.Sprintf("site URL %q has no host", raw))
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errs.New(errs.CodeUsage,
			fmt.Sprintf("site URL %q must be http or https", raw))
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

// Endpoint builds an absolute URL for a path under the site root.
func (s Site) Endpoint(path string) string {
	root := s.BaseURL
	if root == nil {
		return path
	}
	return strings.TrimSuffix(root.String(), "/") + "/" + strings.TrimPrefix(path, "/")
}
