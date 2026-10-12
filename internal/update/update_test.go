package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// release serves the repository's "latest release" link the way GitHub does:
// a redirect to the tag's page. It counts the requests it gets.
func release(t *testing.T, location string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var asked atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if r.URL.Path != "/"+Repository+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, location, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	return server, &asked
}

func checker(t *testing.T, server *httptest.Server, current string, clock *time.Time) *Checker {
	t.Helper()
	return &Checker{
		Current:   current,
		CachePath: filepath.Join(t.TempDir(), "cache", "update-check.json"),
		Origin:    server.URL,
		Now:       func() time.Time { return *clock },
	}
}

func TestANewerReleaseIsReportedWithItsPage(t *testing.T) {
	server, _ := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	got, ok := checker(t, server, "0.3.0", &now).Newer(context.Background())

	if !ok || got.Tag != "v0.4.0" {
		t.Fatalf("Newer() = %+v, %v; want v0.4.0", got, ok)
	}
	if want := server.URL + "/" + Repository + "/releases/tag/v0.4.0"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
}

func TestNothingIsReportedWhenThisBuildIsCurrentOrAhead(t *testing.T) {
	for _, current := range []string{"0.4.0", "v0.4.0", "0.5.0", "0.4.1-rc.1", "1.0.0"} {
		server, _ := release(t, "/"+Repository+"/releases/tag/v0.4.0")
		now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

		if got, ok := checker(t, server, current, &now).Newer(context.Background()); ok {
			t.Errorf("Newer() with %s = %+v; want no news", current, got)
		}
	}
}

func TestAPreReleaseIsOlderThanTheReleaseItLeadsTo(t *testing.T) {
	server, _ := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	if _, ok := checker(t, server, "0.4.0-rc.1", &now).Newer(context.Background()); !ok {
		t.Error("0.4.0-rc.1 was not told that 0.4.0 is out")
	}
}

func TestABuildThatIsNotAReleaseIsNeverAsked(t *testing.T) {
	for _, current := range []string{"dev", "", "0.3", "latest", "0.3.0.1"} {
		server, asked := release(t, "/"+Repository+"/releases/tag/v9.9.9")
		now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

		if _, ok := checker(t, server, current, &now).Newer(context.Background()); ok {
			t.Errorf("%q was told it is out of date", current)
		}
		if asked.Load() != 0 {
			t.Errorf("%q made %d request(s); want none", current, asked.Load())
		}
	}
}

func TestGitHubIsAskedOnceADay(t *testing.T) {
	server, asked := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)

	c.Newer(context.Background())
	now = now.Add(interval - time.Minute)
	got, ok := c.Newer(context.Background())

	if asked.Load() != 1 {
		t.Errorf("asked %d times within a day; want 1", asked.Load())
	}
	if !ok || got.Tag != "v0.4.0" {
		t.Errorf("the cached answer was not used: %+v, %v", got, ok)
	}

	now = now.Add(2 * time.Minute)
	c.Newer(context.Background())
	if asked.Load() != 2 {
		t.Errorf("asked %d times after a day; want 2", asked.Load())
	}
}

func TestAnAnswerFromTheFutureDoesNotStopTheCheck(t *testing.T) {
	server, asked := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)
	c.Newer(context.Background())

	now = now.Add(-48 * time.Hour)
	c.Newer(context.Background())

	if asked.Load() != 2 {
		t.Errorf("asked %d times after the clock went back; want 2", asked.Load())
	}
}

func TestAFailedAttemptCountsAsTheDaysAttempt(t *testing.T) {
	var asked atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)

	for range 3 {
		if _, ok := c.Newer(context.Background()); ok {
			t.Fatal("a failed check reported a release")
		}
	}

	if asked.Load() != 1 {
		t.Errorf("asked %d times after a failure; want 1", asked.Load())
	}
}

func TestAnAbandonedAttemptCountsAsTheDaysAttempt(t *testing.T) {
	// The caller gives up on a slow network; the next command must not wait
	// for it again.
	release := make(chan struct{})
	var asked atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	c.Newer(ctx)
	c.Newer(context.Background())

	if asked.Load() != 1 {
		t.Errorf("asked %d times after an abandoned attempt; want 1", asked.Load())
	}
}

func TestAFailedCheckKeepsWhatWasLearnedBefore(t *testing.T) {
	server, _ := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)
	c.Newer(context.Background())

	server.Close()
	now = now.Add(2 * interval)
	got, ok := c.Newer(context.Background())

	if !ok || got.Tag != "v0.4.0" {
		t.Errorf("Newer() after the network went away = %+v, %v; want the cached v0.4.0", got, ok)
	}
}

func TestOnlyTheRepositorysOwnTagPageIsBelieved(t *testing.T) {
	for name, location := range map[string]string{
		"another repository": "/someone/else/releases/tag/v9.0.0",
		"another host":       "https://example.com/" + Repository + "/releases/tag/v9.0.0",
		"not a tag":          "/" + Repository + "/releases/tag/nightly",
		"a pre-release":      "/" + Repository + "/releases/tag/v9.0.0-rc.1",
		"extra path":         "/" + Repository + "/releases/tag/v9.0.0/../../x",
		"escape sequence":    "/" + Repository + "/releases/tag/v9.0.0%1b%5b31m",
		"the releases page":  "/" + Repository + "/releases",
	} {
		t.Run(name, func(t *testing.T) {
			server, _ := release(t, location)
			now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

			if got, ok := checker(t, server, "0.3.0", &now).Newer(context.Background()); ok {
				t.Errorf("Newer() = %+v; want the redirect refused", got)
			}
		})
	}
}

func TestACachedTagIsHeldToTheShapeOfARealOne(t *testing.T) {
	for name, tag := range map[string]string{
		"an escape sequence": `v9.0.0\u001b[31m`,
		"a pre-release":      "v9.0.0-rc.1",
		"no v":               "9.0.0",
	} {
		t.Run(name, func(t *testing.T) {
			server, asked := release(t, "/"+Repository+"/releases/tag/v0.3.0")
			now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
			c := checker(t, server, "0.3.0", &now)
			if err := os.MkdirAll(filepath.Dir(c.CachePath), 0o700); err != nil {
				t.Fatal(err)
			}
			forged := `{"checked_at":"` + now.Format(time.RFC3339) + `","latest":"` + tag + `"}`
			if err := os.WriteFile(c.CachePath, []byte(forged), 0o600); err != nil {
				t.Fatal(err)
			}

			if got, ok := c.Newer(context.Background()); ok || asked.Load() != 0 {
				t.Errorf("Newer() = %+v, %v after %d request(s); want the cached tag refused", got, ok, asked.Load())
			}
		})
	}
}

func TestACorruptCacheIsJustAnEmptyOne(t *testing.T) {
	server, asked := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)
	if err := os.MkdirAll(filepath.Dir(c.CachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.CachePath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := c.Newer(context.Background())

	if !ok || got.Tag != "v0.4.0" || asked.Load() != 1 {
		t.Errorf("Newer() = %+v, %v after %d request(s); want v0.4.0 after 1", got, ok, asked.Load())
	}
}

func TestWithoutACachePathNothingIsAsked(t *testing.T) {
	server, asked := release(t, "/"+Repository+"/releases/tag/v0.4.0")
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := checker(t, server, "0.3.0", &now)
	c.CachePath = ""

	if _, ok := c.Newer(context.Background()); ok || asked.Load() != 0 {
		t.Errorf("a checker with nowhere to remember asked %d time(s)", asked.Load())
	}
}

func TestTheRequestCarriesNothingButTheProgramAndItsVersion(t *testing.T) {
	var got *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		http.Redirect(w, r, "/"+Repository+"/releases/tag/v0.4.0", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	checker(t, server, "0.3.0", &now).Newer(context.Background())

	if got == nil {
		t.Fatal("no request was made")
	}
	if got.Method != http.MethodHead {
		t.Errorf("method = %s, want HEAD", got.Method)
	}
	if got.URL.RawQuery != "" {
		t.Errorf("query = %q, want none", got.URL.RawQuery)
	}
	if ua := got.Header.Get("User-Agent"); ua != "moodle-cli/0.3.0" {
		t.Errorf("User-Agent = %q, want moodle-cli/0.3.0", ua)
	}
	for _, name := range []string{"Cookie", "Authorization", "Referer"} {
		if got.Header.Get(name) != "" {
			t.Errorf("%s header was sent", name)
		}
	}
}

func TestDisabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"nothing set", nil, false},
		{"opt-out", map[string]string{EnvDisable: "1"}, true},
		{"opt-out spelled out", map[string]string{EnvDisable: "true"}, true},
		{"a typo still stops the request", map[string]string{EnvDisable: "ture"}, true},
		{"explicitly off", map[string]string{EnvDisable: "0"}, false},
		{"explicitly off, mixed case", map[string]string{EnvDisable: " False "}, false},
		{"automation", map[string]string{"CI": "true"}, true},
		{"automation switched off", map[string]string{"CI": "false"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Disabled(func(name string) string { return tc.env[name] })
			if got != tc.want {
				t.Errorf("Disabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVersionOrder(t *testing.T) {
	for _, tc := range []struct{ current, latest string }{
		{"0.3.0", "v0.3.1"}, {"0.3.9", "v0.4.0"}, {"0.9.9", "v1.0.0"},
		{"v0.3.0", "v0.10.0"}, {"0.3.0-rc.1", "v0.3.0"}, {"0.3.0+build", "v0.3.1"},
	} {
		current, _ := parse(tc.current)
		latest, ok := parseTag(tc.latest)
		if !ok || !current.before(latest) {
			t.Errorf("%s should be before %s", tc.current, tc.latest)
		}
		if latest.before(current) {
			t.Errorf("%s should not be before %s", tc.latest, tc.current)
		}
	}
	if same, _ := parse("0.3.0"); same.before(version{major: 0, minor: 3, patch: 0}) {
		t.Error("a version is not before itself")
	}
}

func TestOnlyReleaseNumbersParse(t *testing.T) {
	for _, bad := range []string{"", "dev", "1", "1.2", "1.2.3.4", "v", "a.b.c", "1.2.3-", "1.2.3 ", "1.2.3\n", "9999999999.0.0"} {
		if _, ok := parse(bad); ok {
			t.Errorf("parse(%q) accepted it", bad)
		}
	}
	for _, bad := range []string{"0.3.0", "v0.3.0-rc.1", "v0.3.0+x", strings.Repeat("v1.2.3", 2)} {
		if _, ok := parseTag(bad); ok {
			t.Errorf("parseTag(%q) accepted it", bad)
		}
	}
}
