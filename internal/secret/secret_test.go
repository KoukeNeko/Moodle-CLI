package secret_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func TestKeyShape(t *testing.T) {
	// The key format is fixed; credentials stored by an
	// earlier build must stay reachable, so this shape is not free to drift.
	ref := secret.Ref{SiteID: "site-1", AccountID: "acct-2", Kind: secret.KindWSToken}
	if got, want := ref.Key(), "site-1/acct-2/ws-token"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

func TestIncompleteReferenceIsRejected(t *testing.T) {
	store := secret.NewMemory()
	for _, ref := range []secret.Ref{
		{AccountID: "a", Kind: secret.KindWSToken},
		{SiteID: "s", Kind: secret.KindWSToken},
		{SiteID: "s", AccountID: "a"},
	} {
		if err := store.Set(ref, "x"); err == nil {
			t.Errorf("Set(%+v) should have failed", ref)
		}
	}
}

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := secret.NewMemory()
	ref := secret.Ref{SiteID: site.NewID(), AccountID: site.NewID(), Kind: secret.KindWSToken}

	if _, err := store.Get(ref); err == nil {
		t.Fatal("Get on an empty store should fail")
	} else if !errors.Is(err, secret.ErrNotFound) {
		t.Errorf("want ErrNotFound in the chain, got %v", err)
	}

	if err := store.Set(ref, "c9e291c2"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ref)
	if err != nil {
		t.Fatal(err)
	}
	if got != "c9e291c2" {
		t.Errorf("Get = %q", got)
	}

	if err := store.Delete(ref); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ref); err == nil {
		t.Error("Get after Delete should fail")
	}
}

func TestMissingCredentialIsAuthenticationNotInternal(t *testing.T) {
	// A missing credential is a normal state on a fresh machine: it must send
	// the user to `auth login`, not look like a crash.
	store := secret.NewMemory()
	_, err := store.Get(secret.Ref{SiteID: "s", AccountID: "a", Kind: secret.KindWSToken})
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if e.Reason != errs.ReasonCredentialMissing {
		t.Errorf("reason = %q, want credential_missing", e.Reason)
	}
}

func TestDeletingSomethingAlreadyGoneSucceeds(t *testing.T) {
	// `auth logout` must not fail because the credential was cleared by hand.
	store := secret.NewMemory()
	ref := secret.Ref{SiteID: "s", AccountID: "a", Kind: secret.KindWSToken}
	if err := store.Delete(ref); err != nil {
		t.Errorf("Delete on a missing entry: %v", err)
	}
}

func TestKeyringReportsUnavailableKeychainActionably(t *testing.T) {
	// This box has no Secret Service, which is exactly the case the hint is
	// for. Where a keychain does exist the round trip is covered by Memory
	// and by the manual check in test/e2e.
	ref := secret.Ref{SiteID: site.NewID(), AccountID: site.NewID(), Kind: secret.KindWSToken}
	err := secret.Keyring{}.Set(ref, "value")
	if err == nil {
		t.Skip("a working keychain is available; nothing to assert here")
	}
	e := errs.From(err)
	if e.Code != errs.CodeConfiguration {
		t.Errorf("code = %q, want configuration", e.Code)
	}
	if !strings.Contains(e.Hint, "MOODLE_WS_TOKEN") {
		t.Errorf("hint should offer a way forward, got %q", e.Hint)
	}
	// A browser session cannot travel in MOODLE_WS_TOKEN; offering only that
	// left someone importing a session with no way forward.
	if !strings.Contains(e.Hint, "MOODLE_SESSION") {
		t.Errorf("hint should name the session variable too, got %q", e.Hint)
	}
}
