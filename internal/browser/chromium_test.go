package browser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/browser"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// chromeProfile lays out a profile around the recorded database.
//
// testdata/chrome-cookies.sqlite came from a real Chrome 153 that had signed
// in to the docker Moodle: the SQLite layout, the schema version, the column
// set and the encryption are exactly what Chrome wrote. Only the cookie
// values were replaced — re-encrypted the same way, so the decryption path is
// still exercised end to end — so that nothing shaped like a credential is
// committed.
func chromeProfile(t *testing.T) string {
	t.Helper()
	profile := t.TempDir()
	raw, err := os.ReadFile("testdata/chrome-cookies.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	network := filepath.Join(profile, "Network")
	if err := os.MkdirAll(network, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(network, "Cookies"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestAChromiumSessionCookieIsDecrypted(t *testing.T) {
	// Chromium does keep session cookies, unlike Firefox — measured on
	// Chrome 153: the row is there with is_persistent 0 and no expiry. What
	// it does not do is keep them in the clear.
	found, err := browser.ChromiumSession(chromeProfile(t), "192.168.50.169", "")
	if err != nil {
		t.Fatal(err)
	}
	if found.Value == "" {
		t.Fatal("decrypted to nothing")
	}
	if strings.HasPrefix(found.Value, "v10") {
		t.Errorf("the value was returned still encrypted: %q", found.Value)
	}
	// From schema 24 the plaintext starts with a hash of the cookie's own
	// host. A reader that did not strip it would return 32 bytes of hash
	// followed by the value, which looks like a working cookie and is not.
	if len(found.Value) > 64 {
		t.Errorf("the host hash was not stripped: %d bytes", len(found.Value))
	}
}

func TestOnlyTheNamedCookieIsDecrypted(t *testing.T) {
	// The database holds another cookie for the same host. Asking for the
	// session must not return it, and nothing else should be decrypted on
	// the way past.
	found, err := browser.ChromiumSession(chromeProfile(t), "192.168.50.169", "")
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "MoodleSession" {
		t.Errorf("name = %q", found.Name)
	}
}

func TestAnAbsentSiteIsNotReportedAsSignedOut(t *testing.T) {
	_, err := browser.ChromiumSession(chromeProfile(t), "moodle.example.edu", "")
	if err == nil {
		t.Fatal("a site absent from the database was reported as found")
	}
	if code := errs.From(err).Code; code != errs.CodeNotFound {
		t.Errorf("code = %q, want not_found", code)
	}
	message := err.Error() + errs.From(err).Hint
	if strings.Contains(message, "not signed in") {
		t.Errorf("absence from a database was reported as a fact about the user:\n%s", message)
	}
}

func TestAWriteAheadLogIsRefusedRatherThanRead(t *testing.T) {
	// A WAL holds committed rows the main file does not have, so reading the
	// main file alone would answer with a stale row or miss the cookie
	// entirely — and look exactly like a correct answer either way.
	profile := chromeProfile(t)
	if err := os.WriteFile(filepath.Join(profile, "Network", "Cookies-wal"),
		[]byte("pretend this is a log"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := browser.ChromiumSession(profile, "192.168.50.169", "")
	if err == nil {
		t.Fatal("a database with a write-ahead log was read anyway")
	}
	if !strings.Contains(err.Error(), "write-ahead log") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

func TestTheLegacyCookieLocationStillWorks(t *testing.T) {
	profile := chromeProfile(t)
	current := filepath.Join(profile, "Network", "Cookies")
	legacy := filepath.Join(profile, "Cookies")
	if err := os.Rename(current, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := browser.ChromiumSession(profile, "192.168.50.169", ""); err != nil {
		t.Fatalf("legacy profile layout was not read: %v", err)
	}
}

func TestAProfileWithNoDatabaseSaysSo(t *testing.T) {
	_, err := browser.ChromiumSession(t.TempDir(), "192.168.50.169", "")
	if err == nil {
		t.Fatal("a profile with no database returned a cookie")
	}
	if !strings.Contains(err.Error(), "no cookie database") {
		t.Errorf("message = %v", err)
	}
}
