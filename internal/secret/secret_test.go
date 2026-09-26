package secret_test

import (
	"errors"
	"os"
	"path/filepath"
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
	// A single run is not a way to stay signed in. The one option that is
	// has to be named where the failure happens, or nobody finds it.
	if !strings.Contains(e.Hint, "--credential-store file") {
		t.Errorf("hint should offer the file store, got %q", e.Hint)
	}
}

func TestFileStoreRoundTripsAndStaysPrivate(t *testing.T) {
	dir := t.TempDir()
	store := secret.NewFile(filepath.Join(dir, "config.yaml"))
	token := secret.Ref{SiteID: "s", AccountID: "a", Kind: secret.KindWSToken}
	session := secret.Ref{SiteID: "s", AccountID: "a", Kind: secret.KindSession}

	if _, err := store.Get(token); !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("an empty store should report not found, got %v", err)
	}
	if err := store.Set(token, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(session, "cookie"); err != nil {
		t.Fatal(err)
	}
	if value, err := store.Get(token); err != nil || value != "abc" {
		t.Fatalf("Get = %q, %v", value, err)
	}

	// Other accounts on the machine must not be able to read it.
	info, err := os.Stat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %04o, want 0600", mode)
	}

	// One credential going away must not take the others with it.
	if err := store.Delete(token); err != nil {
		t.Fatal(err)
	}
	if value, err := store.Get(session); err != nil || value != "cookie" {
		t.Fatalf("the session was lost with the token: %q, %v", value, err)
	}
	// Deleting what is already gone is a success, as it is for the keychain.
	if err := store.Delete(token); err != nil {
		t.Errorf("Delete on a missing entry: %v", err)
	}
	// The last one out removes the file rather than leaving something that
	// looks like it holds credentials.
	if err := store.Delete(session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an emptied store left a file behind: %v", err)
	}
}

func TestAFileStoreRefusesRatherThanDiscardWhatItCannotRead(t *testing.T) {
	// Rewriting an unreadable file would throw away credentials this build
	// cannot parse — including another version's.
	dir := t.TempDir()
	store := secret.NewFile(filepath.Join(dir, "config.yaml"))
	if err := os.WriteFile(store.Path, []byte("not json at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(secret.Ref{SiteID: "s", AccountID: "a", Kind: secret.KindWSToken}, "x"); err == nil {
		t.Fatal("an unreadable store was overwritten")
	}
	data, err := os.ReadFile(store.Path)
	if err != nil || string(data) != "not json at all" {
		t.Errorf("the file was changed: %q, %v", data, err)
	}
}

func TestTheStoreIsChosenWhenACredentialIsUsed(t *testing.T) {
	// The composition root builds the store before the flag is parsed, so the
	// choice has to be read per call rather than captured.
	dir := t.TempDir()
	chosen := new(secret.Backend)
	store := secret.Selected{Backend: chosen, ConfigPath: filepath.Join(dir, "config.yaml")}
	ref := secret.Ref{SiteID: "s", AccountID: "a", Kind: secret.KindWSToken}

	*chosen = secret.BackendFile
	if err := store.Set(ref, "abc"); err != nil {
		t.Fatal(err)
	}
	if value, err := store.Get(ref); err != nil || value != "abc" {
		t.Fatalf("Get = %q, %v", value, err)
	}
	if _, err := os.Stat(filepath.Join(dir, secret.FileName)); err != nil {
		t.Errorf("nothing was written to the file store: %v", err)
	}
}

func TestOnlyKnownCredentialStoresAreAccepted(t *testing.T) {
	if _, err := secret.ParseBackend("keyring"); err != nil {
		t.Errorf("keyring: %v", err)
	}
	if _, err := secret.ParseBackend("FILE"); err != nil {
		t.Errorf("case should not matter: %v", err)
	}
	err := errs.From(mustFail(t, "pass"))
	if err.Code != errs.CodeConfiguration || !strings.Contains(err.Hint, "file") {
		t.Errorf("unknown store: %+v", err)
	}
}

func mustFail(t *testing.T, value string) error {
	t.Helper()
	backend, err := secret.ParseBackend(value)
	if err == nil {
		t.Fatalf("%q was accepted as %q", value, backend)
	}
	return err
}
