package moodle_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// pageSession serves pages and records the URL of every request, so a test can
// say what the site was actually asked.
func pageSession(t *testing.T) (*moodle.PageReader, func() []*url.URL) {
	t.Helper()
	var mu sync.Mutex
	var seen []*url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL)
		mu.Unlock()
		fmt.Fprint(w, "<html><body>ok</body></html>")
	}))
	t.Cleanup(server.Close)
	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	pages := moodle.NewPageReader(moodle.NewClient(site.Site{BaseURL: base}),
		moodle.SessionCookie{Value: "s"})
	return pages, func() []*url.URL {
		mu.Lock()
		defer mu.Unlock()
		return append([]*url.URL(nil), seen...)
	}
}

func TestAQueryValueCannotAddParametersOrLeaveThePage(t *testing.T) {
	// A course or quiz reference can come from an agent, and the request
	// carries the browser session. A value that could end its own parameter
	// would let the caller set others, such as `edit` or `sesskey`, or move the
	// request to another page.
	for _, value := range []string{
		"2&edit=on&sesskey=x",
		"../../admin/index.php?x=1#",
		"2#fragment",
		"2 3",
		"100%",
		"é",
	} {
		pages, requests := pageSession(t)

		if _, err := pages.Get(context.Background(), "/course/view.php", map[string]string{"id": value}); err != nil {
			t.Fatalf("Get(%q): %v", value, err)
		}

		got := requests()
		if len(got) != 1 {
			t.Fatalf("Get(%q) sent %d requests, want 1", value, len(got))
		}
		query := got[0].Query()
		if got[0].Path != "/course/view.php" {
			t.Errorf("Get(%q) left the page: path %q", value, got[0].Path)
		}
		if len(query) != 1 || query.Get("id") != value {
			t.Errorf("Get(%q) sent the query %q; want the one parameter id with that value", value, got[0].RawQuery)
		}
	}
}

func TestSeveralQueryParametersAllArrive(t *testing.T) {
	pages, requests := pageSession(t)

	if _, err := pages.Get(context.Background(), "/mod/forum/view.php",
		map[string]string{"id": "5", "p": "2"}); err != nil {
		t.Fatal(err)
	}

	query := requests()[0].Query()
	if len(query) != 2 || query.Get("id") != "5" || query.Get("p") != "2" {
		t.Errorf("query = %q, want id=5 and p=2", requests()[0].RawQuery)
	}
}

func TestAPageWithoutAQueryIsAskedForWithoutOne(t *testing.T) {
	pages, requests := pageSession(t)

	if _, err := pages.Get(context.Background(), "/grade/report/overview/index.php", nil); err != nil {
		t.Fatal(err)
	}

	if got := requests()[0]; got.RawQuery != "" || got.Path != "/grade/report/overview/index.php" {
		t.Errorf("requested %q, want the bare page", got.String())
	}
}
