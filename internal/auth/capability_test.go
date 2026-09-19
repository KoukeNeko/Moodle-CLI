package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// countingSite answers site info and counts how often it was asked, so a test
// can tell a cached answer from a fresh one.
func countingSite(t *testing.T, asked *atomic.Int64, functions func() []string) *auth.Manager {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked.Add(1)
		var offered []any
		for _, name := range functions() {
			offered = append(offered, map[string]any{"name": name, "version": "5.2"})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sitename": "Test", "username": "student1", "userid": 4,
			"release": "5.2.3", "functions": offered,
		})
	}))
	t.Cleanup(server.Close)
	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return auth.NewManager(secret.NewMemory(), func(site.Site) *moodle.Client {
		return moodle.NewClient(site.Site{BaseURL: base},
			moodle.WithHTTPClient(server.Client()))
	})
}

func TestACommandAsksTheSiteOnce(t *testing.T) {
	// Half a second is not long enough for a site to change its mind, and a
	// second round trip on every feature would be a burden on a service the
	// user does not own.
	var asked atomic.Int64
	manager := countingSite(t, &asked, func() []string { return []string{"core_course_get_courses"} })
	session := manager.OpenWithToken(site.Site{}, "acc", "tok")

	for i := 0; i < 3; i++ {
		if _, err := session.Capabilities(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := asked.Load(); got != 1 {
		t.Errorf("asked the site %d times, want 1", got)
	}
}

func TestALongLivedSessionAsksAgain(t *testing.T) {
	// Measured on a real site: an MCP server whose administrator switched a
	// function on went on refusing the tool for ever, out of its own cache,
	// without asking again. Only a restart cleared it.
	var asked atomic.Int64
	var offered atomic.Value
	offered.Store([]string{})
	manager := countingSite(t, &asked, func() []string { return offered.Load().([]string) })

	session := manager.OpenWithToken(site.Site{}, "acc", "tok")
	session.ExpireCapabilitiesAfter(10 * time.Millisecond)

	first, err := session.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Has("mod_forum_get_forums_by_courses") {
		t.Fatal("a function the site does not offer was reported as available")
	}

	offered.Store([]string{"mod_forum_get_forums_by_courses"})
	time.Sleep(20 * time.Millisecond)

	second, err := session.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !second.Has("mod_forum_get_forums_by_courses") {
		t.Error("a function switched on while the session ran stayed invisible")
	}
}

func TestAnUnreachableSiteDoesNotDiscardWhatWeKnew(t *testing.T) {
	// The site answered once and cannot be reached now. The old answer is
	// what this process has been using all along, and failing the call here
	// would turn a passing network blip into a broken tool — while the
	// request underneath still reports the site's own trouble.
	var asked atomic.Int64
	var down atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked.Add(1)
		if down.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sitename": "Test", "username": "student1", "userid": 4, "release": "5.2.3",
			"functions": []any{map[string]any{"name": "core_course_get_courses", "version": "5.2"}},
		})
	}))
	defer server.Close()
	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	manager := auth.NewManager(secret.NewMemory(), func(site.Site) *moodle.Client {
		return moodle.NewClient(site.Site{BaseURL: base}, moodle.WithHTTPClient(server.Client()))
	})

	session := manager.OpenWithToken(site.Site{}, "acc", "tok")
	session.ExpireCapabilitiesAfter(10 * time.Millisecond)
	if _, err := session.Capabilities(context.Background()); err != nil {
		t.Fatal(err)
	}

	down.Store(true)
	time.Sleep(20 * time.Millisecond)

	kept, err := session.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("a site that went away discarded what it had already said: %v", err)
	}
	if !kept.Has("core_course_get_courses") {
		t.Error("the earlier answer was lost")
	}
}
