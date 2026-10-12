// Package update tells a person when a newer release exists. It only tells: it
// downloads nothing and changes nothing on this machine.
//
// The question goes to github.com, at most once a day, and is the same one the
// install script asks: where does the repository's "latest release" link
// redirect to. That is a request for one address and a read of one header. It
// carries no account, no site and no identifier beyond the program's name and
// version, and it avoids api.github.com, whose anonymous rate limit is shared
// by everyone behind the same address.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Repository is where releases are published.
const Repository = "KoukeNeko/Moodle-CLI"

// EnvDisable turns the check off. Anything but an explicit off counts as on,
// the way MOODLE_CLI_READ_ONLY does: someone who sets this at all means to stop
// the request, and a typo must not quietly keep sending it.
const EnvDisable = "MOODLE_CLI_NO_UPDATE_CHECK"

// interval is how long an answer is kept before GitHub is asked again.
const interval = 24 * time.Hour

// origin is where the release pages live.
const origin = "https://github.com"

// Release is a published version.
type Release struct {
	// Tag is the version as the repository names it: "v0.4.0".
	Tag string
	// URL is the release's page, built from Tag and nothing GitHub sent.
	URL string
}

// Checker asks whether a newer release exists, and remembers the answer.
type Checker struct {
	// Current is this build's version, with or without a leading "v". A build
	// that is not a release ("dev") is never told it is out of date.
	Current string
	// CachePath is the file that remembers the last answer. It is what keeps the
	// question to once a day, so a Checker without one asks nothing.
	CachePath string
	// Origin is where the release pages live. Empty is github.com; a test
	// points it at its own server.
	Origin string
	// Now is the clock. Nil is the real one.
	Now func() time.Time
}

// DefaultCachePath is where the answer is kept on this machine.
func DefaultCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "moodle-cli", "update-check.json"), nil
}

// Disabled reports whether the environment says not to check: the opt-out
// above, or an automation that has no one to tell.
func Disabled(getenv func(string) string) bool {
	return set(getenv(EnvDisable)) || set(getenv("CI"))
}

func set(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}

// Newer returns the latest release when it is newer than this build.
//
// It is advisory, so nothing here can fail loudly: a network that cannot be
// reached, a page that answers oddly and a cache that cannot be written all
// mean "no news". The attempt is recorded before it is made, so a network that
// never answers costs one wait a day, not one per command.
func (c *Checker) Newer(ctx context.Context) (Release, bool) {
	current, ok := parse(c.Current)
	if !ok || c.CachePath == "" {
		return Release{}, false
	}
	now := c.now()
	cached := c.load()
	if cached.stale(now) {
		cached.CheckedAt = now
		c.save(cached)
		if tag, err := c.latest(ctx); err == nil {
			cached.Latest = tag
			c.save(cached)
		}
	}
	// The cached tag is read from a file anyone on this account can edit, and
	// it ends up on a terminal: it is held to the same shape as a fresh one.
	latest, ok := parseTag(cached.Latest)
	if !ok || !current.before(latest) {
		return Release{}, false
	}
	return Release{Tag: cached.Latest, URL: c.origin() + "/" + Repository + "/releases/tag/" + cached.Latest}, true
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Checker) origin() string {
	if c.Origin != "" {
		return c.Origin
	}
	return origin
}

// latest asks where the "latest release" link leads. That link never leads to
// a pre-release, which is why it is the one asked.
func (c *Checker) latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.origin()+"/"+Repository+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "moodle-cli/"+c.Current)
	client := &http.Client{
		Timeout: 5 * time.Second,
		// The redirect is the answer; following it would fetch a page nobody
		// needs.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return "", errors.New("the latest release link did not redirect")
	}
	location, err := resp.Location()
	if err != nil {
		return "", err
	}
	// Only the tag is taken from the answer, and only from the repository's own
	// page on the host that was asked.
	tag, ok := strings.CutPrefix(location.Path, "/"+Repository+"/releases/tag/")
	if _, valid := parseTag(tag); !ok || !valid || location.Host != req.URL.Host {
		return "", errors.New("the latest release link led somewhere unexpected")
	}
	return tag, nil
}

// state is what is kept between runs.
type state struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

// stale says the answer is old enough to ask again. A clock that went backwards
// counts: otherwise a timestamp from the future would stop the check for good.
func (s state) stale(now time.Time) bool {
	return s.CheckedAt.IsZero() || now.Before(s.CheckedAt) || now.Sub(s.CheckedAt) >= interval
}

// load reads the cache. A missing or unreadable file is an empty one: the
// worst that follows is one more question.
func (c *Checker) load() state {
	var s state
	if raw, err := os.ReadFile(c.CachePath); err == nil {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

// save is best effort, for the reason given on Newer.
func (c *Checker) save(s state) {
	raw, err := json.Marshal(s)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(c.CachePath), 0o700) != nil {
		return
	}
	_ = os.WriteFile(c.CachePath, raw, 0o600)
}

// version is a release number, which is all this package compares.
type version struct {
	major, minor, patch int
	// pre is set for a pre-release such as 1.2.0-rc.1, which comes before the
	// release it leads up to.
	pre bool
}

var versionPattern = regexp.MustCompile(`^(v?)([0-9]{1,9})\.([0-9]{1,9})\.([0-9]{1,9})(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func parse(s string) (version, bool) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return version{}, false
	}
	// The pattern allows nine digits, so these cannot fail.
	major, _ := strconv.Atoi(m[2])
	minor, _ := strconv.Atoi(m[3])
	patch, _ := strconv.Atoi(m[4])
	return version{major: major, minor: minor, patch: patch, pre: m[5] != ""}, true
}

// parseTag accepts only what a release tag looks like: a "v", three numbers and
// nothing else.
func parseTag(tag string) (version, bool) {
	v, ok := parse(tag)
	if !ok || !strings.HasPrefix(tag, "v") || v.pre || strings.Contains(tag, "+") {
		return version{}, false
	}
	return v, true
}

// before reports whether v comes before release. Two pre-releases of the same
// number are not ordered: only a published release is ever compared against.
func (v version) before(release version) bool {
	switch {
	case v.major != release.major:
		return v.major < release.major
	case v.minor != release.minor:
		return v.minor < release.minor
	case v.patch != release.patch:
		return v.patch < release.patch
	default:
		return v.pre
	}
}
