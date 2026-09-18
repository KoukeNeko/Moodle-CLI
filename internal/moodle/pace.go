package moodle

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Pacing keeps this client from being a burden on a site it does not own.
//
// A Moodle is usually a shared university service, and the thing driving this
// client may be a loop: a script, or an agent that decided to check every
// assignment. Neither has any instinct for how often is too often, so the
// restraint belongs here rather than in whatever is calling.
//
// This is not a retry mechanism. Waiting before sending is a different thing
// from sending again after a failure, and the difference matters: a write that
// failed is never repeated, but any request may politely wait its turn.
type Pacing struct {
	// MinInterval is the smallest gap between two requests. Zero disables
	// pacing entirely, which is what a test usually wants.
	MinInterval time.Duration
	// Burst is how many requests may go out back to back before the interval
	// starts applying. Interactive use is bursty — a command makes three or
	// four calls and then waits for a person — so a small burst keeps the
	// common case instant.
	Burst int
}

// DefaultPacing is deliberately unhurried.
//
// Ten requests a second sustained is far more than a person at a terminal
// generates and far less than a loop can. Nothing here is fast enough to
// matter to a user and nothing is slow enough to be worth working around,
// which is the point: a limit people route around protects nobody.
var DefaultPacing = Pacing{MinInterval: 100 * time.Millisecond, Burst: 8}

// limiter paces requests and honours a site asking for quiet.
type limiter struct {
	pacing Pacing

	mu sync.Mutex
	// allowance is how many requests may go out immediately, as a fraction so
	// it refills smoothly rather than in steps.
	allowance float64
	last      time.Time
	// until is a pause the site itself asked for. It overrides the ordinary
	// pacing: being told to wait is not a suggestion.
	until time.Time
}

func newLimiter(pacing Pacing) *limiter {
	if pacing.Burst < 1 {
		pacing.Burst = 1
	}
	return &limiter{
		pacing:    pacing,
		allowance: float64(pacing.Burst),
		last:      time.Now(),
	}
}

// wait blocks until a request may go out, or the context ends.
//
// A cancelled context returns its own error rather than proceeding: someone
// pressing Ctrl-C while a limiter is sleeping should not then watch a request
// they cancelled go out anyway.
func (l *limiter) wait(ctx context.Context) error {
	if l == nil || l.pacing.MinInterval <= 0 {
		return nil
	}
	for {
		delay := l.reserve()
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

// reserve takes a slot, or reports how long to wait for one.
func (l *limiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if wait := l.until.Sub(now); wait > 0 {
		return wait
	}

	// Refill by however much time has passed, capped at the burst.
	elapsed := now.Sub(l.last)
	l.last = now
	l.allowance += float64(elapsed) / float64(l.pacing.MinInterval)
	if l.allowance > float64(l.pacing.Burst) {
		l.allowance = float64(l.pacing.Burst)
	}

	if l.allowance >= 1 {
		l.allowance--
		return 0
	}
	// Wait for the fraction still missing.
	return time.Duration((1 - l.allowance) * float64(l.pacing.MinInterval))
}

// pause records that the site asked for quiet until a point in time.
func (l *limiter) pause(until time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if until.After(l.until) {
		l.until = until
	}
}

// maxRetryAfter caps how long a Retry-After is honoured for.
//
// A site can name any delay, including one longer than anybody is prepared to
// sit through. Past this the caller is told the number rather than left
// blocked on it, and can decide for itself.
const maxRetryAfter = 5 * time.Minute

// retryAfter reads the header in both of the forms HTTP allows.
//
// It returns the delay and whether one was given at all: a 429 with no
// Retry-After is a site asking for quiet without saying for how long, which is
// a different situation from one that named a delay.
func retryAfter(header http.Header, now time.Time) (time.Duration, bool) {
	raw := header.Get("Retry-After")
	if raw == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	// The other form is an HTTP date.
	if at, err := http.ParseTime(raw); err == nil {
		delay := at.Sub(now)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}
