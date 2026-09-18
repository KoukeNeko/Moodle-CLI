package moodle_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// paced starts a site and a client that paces requests to it.
func paced(t *testing.T, pacing moodle.Pacing, handle http.HandlerFunc) (*moodle.Client, *int64) {
	t.Helper()
	var calls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		handle(w, r)
	}))
	t.Cleanup(server.Close)

	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{Name: "school", BaseURL: base},
		moodle.WithPacing(pacing))
	return client, &calls
}

func okJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func TestRequestsArePaced(t *testing.T) {
	// A Moodle is usually a shared service and the thing driving this client
	// may be a loop. Restraint belongs here, not in whatever is calling.
	client, calls := paced(t, moodle.Pacing{MinInterval: 20 * time.Millisecond, Burst: 2}, okJSON)

	started := time.Now()
	for i := 0; i < 6; i++ {
		var out map[string]any
		if err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(started)

	if *calls != 6 {
		t.Fatalf("%d requests reached the site, want 6", *calls)
	}
	// Two go out immediately; the other four wait one interval each.
	if want := 4 * 20 * time.Millisecond; elapsed < want {
		t.Errorf("six requests took %v, which is faster than the pacing allows (%v)", elapsed, want)
	}
}

func TestABurstOfInteractiveCallsIsNotSlowedDown(t *testing.T) {
	// A limit people route around protects nobody. One command making a few
	// calls has to stay instant, or someone will turn the pacing off.
	client, _ := paced(t, moodle.DefaultPacing, okJSON)

	started := time.Now()
	for i := 0; i < moodle.DefaultPacing.Burst; i++ {
		var out map[string]any
		if err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Errorf("a burst of %d calls took %v", moodle.DefaultPacing.Burst, elapsed)
	}
}

func TestBeingAskedToWaitIsHonouredForLaterRequests(t *testing.T) {
	// Being told to wait is not a suggestion, and the ask outlives the request
	// that received it: the next caller waits too rather than rediscovering
	// the same 429.
	var served int64
	client, _ := paced(t, moodle.Pacing{MinInterval: time.Millisecond, Burst: 10},
		func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&served, 1) == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			okJSON(w, r)
		})

	var out map[string]any
	err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out)
	if err == nil {
		t.Fatal("a 429 was reported as a success")
	}
	e := errs.From(err)
	if e.Reason != errs.ReasonRateLimited {
		t.Errorf("reason = %q, want rate_limited", e.Reason)
	}
	// The wait has to reach the user, or "try again" is advice they cannot act on.
	if !strings.Contains(e.Hint, "1s") {
		t.Errorf("the hint does not say how long to wait: %q", e.Hint)
	}

	started := time.Now()
	if err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 900*time.Millisecond {
		t.Errorf("the next request went out after %v, ignoring Retry-After", elapsed)
	}
}

func TestCancellingWhileWaitingDoesNotSendTheRequest(t *testing.T) {
	// Someone pressing Ctrl-C while the limiter sleeps should not then watch
	// the request they cancelled go out anyway.
	client, calls := paced(t, moodle.Pacing{MinInterval: 2 * time.Second, Burst: 1}, okJSON)

	var out map[string]any
	// Spend the single burst slot.
	if err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	started := time.Now()
	err := client.Call(ctx, "t", "core_webservice_get_site_info", nil, &out)
	if err == nil {
		t.Fatal("a cancelled call was reported as a success")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("cancelling took %v; it waited out the pacing first", elapsed)
	}
	if *calls != 1 {
		t.Errorf("%d requests reached the site; the cancelled one should not have", *calls)
	}
}

func TestAnHttpErrorKeepsMoodlesOwnExplanation(t *testing.T) {
	// A bare status code leaves the user with a number and nothing to act on.
	client, _ := paced(t, moodle.Pacing{}, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"exception":"dml_write_exception","errorcode":"dmlwriteexception","message":"Error writing to database"}`))
	})

	var out map[string]any
	err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out)
	if err == nil {
		t.Fatal("HTTP 500 was reported as a success")
	}
	if !strings.Contains(errs.From(err).Error(), "Error writing to database") {
		t.Errorf("Moodle's own message was lost: %q", errs.From(err).Error())
	}
}

func TestAnHtmlErrorPageIsNotQuotedBackAtTheUser(t *testing.T) {
	// Pasting a page of markup into an error message helps nobody.
	client, _ := paced(t, moodle.Pacing{}, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>502 Bad Gateway</body></html>"))
	})

	var out map[string]any
	err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out)
	if err == nil {
		t.Fatal("HTTP 502 was reported as a success")
	}
	if strings.Contains(errs.From(err).Error(), "<") {
		t.Errorf("markup reached the error message: %q", errs.From(err).Error())
	}
}

func TestRetryAfterAsADateIsUnderstood(t *testing.T) {
	// HTTP allows both forms and Moodle's front ends differ. The date form has
	// second granularity, so the offset here is comfortably larger than the
	// rounding rather than exactly at it.
	var served int64
	client, _ := paced(t, moodle.Pacing{MinInterval: time.Millisecond, Burst: 10},
		func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&served, 1) == 1 {
				w.Header().Set("Retry-After", time.Now().Add(3*time.Second).UTC().Format(http.TimeFormat))
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			okJSON(w, r)
		})

	var out map[string]any
	if err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out); err == nil {
		t.Fatal("a 429 was reported as a success")
	}
	started := time.Now()
	if err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 1500*time.Millisecond {
		t.Errorf("a dated Retry-After was ignored (waited %v)", elapsed)
	}
}

func TestAnAbsurdRetryAfterDoesNotBlockForever(t *testing.T) {
	// A site can name any delay. Past a point the caller is told the number
	// rather than left sitting on it.
	client, _ := paced(t, moodle.Pacing{MinInterval: time.Millisecond, Burst: 10},
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "86400")
			w.WriteHeader(http.StatusTooManyRequests)
		})

	var out map[string]any
	err := client.Call(context.Background(), "t", "core_webservice_get_site_info", nil, &out)
	if err == nil {
		t.Fatal("a 429 was reported as a success")
	}
	if !strings.Contains(errs.From(err).Hint, "24h") {
		t.Errorf("the site's own figure was not passed on: %q", errs.From(err).Hint)
	}
}
