// Package browser reads a Moodle session out of a browser's own storage.
//
// It exists because Moodle's session cookie is the credential a signed-in
// browser holds, and exchanging it for a web service token is the only way in
// on a site that issues no token of its own. Until now that meant asking the
// user to open developer tools and copy a cookie by hand.
//
// Two things about Firefox decide the shape of this package, and both were
// measured rather than assumed:
//
// Moodle sets its session cookie with no expiry — lifetime 0 in
// lib/classes/session/manager.php — which makes it a session cookie, and
// Firefox deliberately keeps those out of cookies.sqlite. Measured on Firefox
// 156 against a live Moodle 5.2: cookies.sqlite held nothing at all, while
// sessionstore-backups/recovery.jsonlz4 held the cookie. Reading the SQLite
// database, which is where one would look first, answers the wrong question.
//
// What this package can therefore say is narrow, and the wording elsewhere
// has to respect it: a cookie was or was not found in the snapshot Firefox
// last wrote. Not finding one is not evidence that the user is not signed in.
// The snapshot is written periodically, privacy settings can exclude it, and
// a different profile or a container tab can hold the real session.
package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// SessionCookieName is what Moodle calls its session cookie by default. A
// site can rename it, which is why the caller may say so.
const SessionCookieName = "MoodleSession"

// Found is a cookie read out of a browser's storage, with where it came from.
type Found struct {
	// Host is the cookie's own host as the browser recorded it, which is not
	// necessarily the address the user typed.
	Host  string
	Name  string
	Value string
	// Source names the file it was read from, so the user can see exactly
	// what was opened on their behalf.
	Source string
}

// sessionStore is the part of Firefox's session file this reads.
//
// The file holds a great deal more — every open tab, its history, its scroll
// position. None of that is read: the fields simply are not declared, so the
// decoder skips them.
type sessionStore struct {
	Cookies []struct {
		Host  string `json:"host"`
		Name  string `json:"name"`
		Value string `json:"value"`
		Path  string `json:"path"`
	} `json:"cookies"`
}

// snapshots are the files Firefox keeps the session in, freshest first.
//
// Which of them exists depends on what the browser is doing, and the
// difference is not cosmetic — measured on Firefox 156:
//
//   - while it runs, sessionstore-backups/recovery.jsonlz4 is written every
//     few seconds and sessionstore.jsonlz4 does not exist;
//   - after it closes cleanly the recovery files are gone and
//     sessionstore.jsonlz4 holds the session instead.
//
// A reader that knew only the first would work while the browser was open
// and find nothing the moment the user closed it, which is the more likely
// state when someone turns to a command line.
var snapshots = []string{
	filepath.Join("sessionstore-backups", "recovery.jsonlz4"),
	filepath.Join("sessionstore-backups", "recovery.baklz4"),
	"sessionstore.jsonlz4",
}

// FirefoxSession looks for one site's session cookie in one Firefox profile.
//
// It reads the named cookie for the named host and returns nothing else. The
// file it opens does contain other sites' session cookies — that cannot be
// avoided, since they share one container — but nothing else in it is
// returned, logged or kept.
func FirefoxSession(profile, host, cookieName string) (Found, error) {
	if cookieName == "" {
		cookieName = SessionCookieName
	}

	var read int
	for _, name := range snapshots {
		path := filepath.Join(profile, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Found{}, errs.Wrap(errs.CodeUnavailable, err, "cannot read "+path)
		}
		read++

		found, err := findIn(raw, path, host, cookieName)
		if err != nil {
			return Found{}, err
		}
		if found.Value != "" {
			return found, nil
		}
	}

	if read == 0 {
		return Found{}, errs.New(errs.CodeNotFound,
			"this profile has no session snapshot to read").
			WithHint("Firefox writes one while it runs and another when it " +
				"closes; if it has never been started with this profile there " +
				"is nothing here yet")
	}
	// Deliberately not "you are not signed in to that site". This knows what
	// the snapshots contained, and they are written periodically, can be
	// excluded by a privacy setting, and cover one profile.
	return Found{}, errs.New(errs.CodeNotFound,
		"no "+cookieName+" for "+host+" in this profile's session snapshot").
		WithHint("that is what the snapshot holds, not whether you are signed " +
			"in; a different profile, a container tab, or a snapshot written " +
			"before you signed in would all look like this")
}

// findIn reads one snapshot. A cookie that is not there is not an error: the
// next file may hold it.
func findIn(raw []byte, path, host, cookieName string) (Found, error) {
	decoded, err := decodeMozLz4(raw)
	if err != nil {
		return Found{}, errs.Wrap(errs.CodeUpstream, err,
			"cannot read Firefox's session snapshot").
			WithReason(errs.ReasonProtocolDrift).
			WithHint("this build reads the format Firefox 156 writes; " +
				"yours may have changed it")
	}

	var store sessionStore
	if err := json.Unmarshal(decoded, &store); err != nil {
		return Found{}, errs.Wrap(errs.CodeUpstream, err,
			"Firefox's session snapshot is not in the expected form").
			WithReason(errs.ReasonProtocolDrift)
	}

	for _, cookie := range store.Cookies {
		if cookie.Name != cookieName || !hostMatches(cookie.Host, host) {
			continue
		}
		if cookie.Value == "" {
			continue
		}
		return Found{
			Host: cookie.Host, Name: cookie.Name, Value: cookie.Value,
			Source: path,
		}, nil
	}
	return Found{}, nil
}

// hostMatches compares a cookie's host with the site's.
//
// Firefox stores a leading dot for a cookie a site asked to share with its
// subdomains. Moodle's own session cookie is host-only, but a site behind a
// front end may differ, so both forms are accepted for the host itself.
// Subdomains are not: a cookie for the whole of a university is not something
// to pick up because one of its hosts was asked about.
func hostMatches(cookieHost, want string) bool {
	cookieHost = strings.TrimPrefix(strings.ToLower(cookieHost), ".")
	return cookieHost == strings.ToLower(want)
}
