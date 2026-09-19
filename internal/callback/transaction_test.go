package callback_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/callback"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func begin(t *testing.T, store *callback.Store, hash string) {
	t.Helper()
	if err := store.Begin(callback.Transaction{
		SiteHash: hash, WWWRoot: "https://moodle.example.edu", Passport: "p",
		Expires: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestACallbackAnswersItsOwnLogin(t *testing.T) {
	store := callback.NewStore()
	begin(t, store, "aaaa")

	transaction, err := store.Claim("aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if transaction.WWWRoot != "https://moodle.example.edu" {
		t.Errorf("www root = %q", transaction.WWWRoot)
	}
}

func TestACallbackIsAnsweredOnlyOnce(t *testing.T) {
	// Moodle's source says passports are valid one time, but nothing on the
	// server enforces it — the site only computes a hash. Whoever replays a
	// captured callback gets a token again unless this refuses.
	store := callback.NewStore()
	begin(t, store, "aaaa")

	if _, err := store.Claim("aaaa"); err != nil {
		t.Fatal(err)
	}
	_, err := store.Claim("aaaa")
	if err == nil {
		t.Fatal("a replayed callback was answered a second time")
	}
	if code := errs.From(err).Code; code != errs.CodeConflict {
		t.Errorf("code = %q, want conflict", code)
	}
	// Named as a replay rather than as an unknown transaction: one means too
	// late, the other means not from here, and they lead somewhere different.
	if !strings.Contains(err.Error(), "already completed") {
		t.Errorf("a replay was not described as one: %v", err)
	}
}

func TestAnUnknownCallbackIsRefusedWithoutGuessing(t *testing.T) {
	store := callback.NewStore()
	begin(t, store, "aaaa")

	_, err := store.Claim("bbbb")
	if err == nil {
		t.Fatal("a callback for another login was accepted")
	}
	// The store cannot tell a callback aimed at another machine apart from
	// one it has forgotten, and the message must not pretend otherwise.
	if !strings.Contains(err.Error(), "no login is waiting") {
		t.Errorf("message = %v", err)
	}
	// And it must not have disturbed the login that is still waiting.
	if store.Pending() != 1 {
		t.Errorf("pending = %d; an unrelated callback consumed a live login", store.Pending())
	}
}

func TestAnExpiredLoginIsNoLongerAnswerable(t *testing.T) {
	// A transaction left open is a callback someone else can still answer.
	store := callback.NewStore()
	if err := store.Begin(callback.Transaction{
		SiteHash: "aaaa", WWWRoot: "https://moodle.example.edu",
		Expires: time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Claim("aaaa"); err == nil {
		t.Fatal("an expired login was still answerable")
	}
}

func TestTwoLoginsAtOnceDoNotAnswerEachOther(t *testing.T) {
	// A user may be signing in to two sites at once. Finishing one must not
	// finish the other, and the token from one must not be stored for it.
	store := callback.NewStore()
	begin(t, store, "aaaa")
	begin(t, store, "bbbb")

	first, err := store.Claim("aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if first.SiteHash != "aaaa" {
		t.Errorf("claimed %q for aaaa", first.SiteHash)
	}
	if store.Pending() != 1 {
		t.Errorf("pending = %d, want the other login still waiting", store.Pending())
	}
	if _, err := store.Claim("bbbb"); err != nil {
		t.Errorf("the other login could no longer be answered: %v", err)
	}
}

func TestOnlyOneClaimWinsUnderConcurrency(t *testing.T) {
	// The handler is a separate process and there may be more than one. Two
	// arriving together must not both be handed a token.
	store := callback.NewStore()
	begin(t, store, "aaaa")

	var wins, losses int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Claim("aaaa")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				wins++
			} else {
				losses++
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Errorf("%d claims succeeded, want exactly 1 (and %d refused)", wins, losses)
	}
}

func TestTheSameLoginCannotBeStartedTwice(t *testing.T) {
	// A passport is 128 bits of randomness, so a repeat is not chance.
	store := callback.NewStore()
	begin(t, store, "aaaa")
	err := store.Begin(callback.Transaction{
		SiteHash: "aaaa", Expires: time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatal("the same transaction was started twice")
	}
}
