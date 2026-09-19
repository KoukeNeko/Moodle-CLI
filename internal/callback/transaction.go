// Package callback holds the state of a browser login while it is happening.
//
// Moodle's browser handoff ends with the site redirecting to
// "<scheme>://token=<payload>". Once a URL scheme is registered, that address
// can be produced by anything on the machine, and by any web page the user
// visits. So the arrival of a well-formed callback proves nothing on its own;
// what proves something is that it answers a login this process started and
// has not already finished.
//
// Moodle's own source says passports "are valid only one time", but there is
// no server-side store enforcing it — the site only computes a hash. Making
// that true is the client's job, and it is this file's whole purpose.
package callback

import (
	"crypto/subtle"
	"sync"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// DefaultLifetime is how long a login may take. It is generous because the
// user may have to find a password manager, an SSO prompt and a second
// factor; it is finite because a transaction left open is a callback someone
// else can still answer.
const DefaultLifetime = 5 * time.Minute

// Transaction is one login waiting for its answer.
type Transaction struct {
	// SiteHash is md5(canonical wwwroot + passport), which is what comes back
	// in the payload. It is the only correlation Moodle offers: the callback
	// format is fixed, so nothing else can be sent along and returned.
	SiteHash string
	// WWWRoot is the site's own idea of its address, as its public config
	// reported it — not the address the user typed. The hash is computed from
	// this, so a site reached by an alias still matches.
	WWWRoot  string
	Passport string
	Expires  time.Time
}

// Store holds the logins in flight.
//
// There is usually one. There can be more: a user may be signing in to two
// sites at once, and answering one must not finish the other.
type Store struct {
	mu      sync.Mutex
	pending map[string]Transaction
	// consumed remembers what has already been answered, so a replay is
	// refused as a replay rather than as an unknown transaction. The
	// difference matters to whoever reads the message: one means "too late",
	// the other means "not from here".
	consumed map[string]time.Time
	now      func() time.Time
}

// NewStore builds an empty store.
func NewStore() *Store {
	return &Store{
		pending:  map[string]Transaction{},
		consumed: map[string]time.Time{},
		now:      time.Now,
	}
}

// Begin records a login that is about to start.
func (s *Store) Begin(t Transaction) error {
	if t.SiteHash == "" {
		return errs.New(errs.CodeInternal, "a login transaction needs a site hash")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	if _, exists := s.pending[t.SiteHash]; exists {
		// The same site and the same passport, twice. A passport is 128 bits
		// of randomness, so this is not chance.
		return errs.New(errs.CodeConflict, "that login is already in progress")
	}
	if t.Expires.IsZero() {
		t.Expires = s.now().Add(DefaultLifetime)
	}
	s.pending[t.SiteHash] = t
	return nil
}

// Claim answers a callback, at most once.
//
// The site hash is compared in constant time. It is a public value, so this
// is not strictly required — but a comparison that decides whether to accept
// a credential is not the place to keep a careless habit.
func (s *Store) Claim(siteHash string) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()

	for candidate, transaction := range s.pending {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(siteHash)) != 1 {
			continue
		}
		delete(s.pending, candidate)
		s.consumed[candidate] = s.now()
		return transaction, nil
	}

	if when, ok := s.consumed[siteHash]; ok {
		return Transaction{}, errs.New(errs.CodeConflict,
			"that login was already completed at "+when.Format(time.RFC3339)).
			WithHint("a callback answers one login once; start another with " +
				"`moodle auth login`")
	}
	// Either it was never ours, or it waited too long. Both are refusals, and
	// neither should say more than it knows: the store cannot tell a callback
	// from another machine apart from one this process has forgotten.
	return Transaction{}, errs.New(errs.CodeValidation,
		"no login is waiting for that callback").
		WithHint("it may have expired, or it may answer a login started elsewhere")
}

// Pending reports how many logins are waiting, for a status line.
func (s *Store) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	return len(s.pending)
}

// sweep drops what has expired. The caller holds the lock.
//
// Consumed hashes are kept for a while after expiry so that a replay arriving
// late is still named as a replay.
func (s *Store) sweep() {
	now := s.now()
	for hash, transaction := range s.pending {
		if now.After(transaction.Expires) {
			delete(s.pending, hash)
		}
	}
	for hash, when := range s.consumed {
		if now.Sub(when) > time.Hour {
			delete(s.consumed, hash)
		}
	}
}
