package browser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/browser"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// profileWithSnapshot lays out a profile directory around the recorded file.
//
// testdata/recovery.jsonlz4 came out of a real Firefox 156 that had signed in
// to the docker Moodle: the structure, the field names and the compression
// are exactly what Firefox wrote. Only the two cookie values were replaced,
// with a string of the same shape, so that nothing shaped like a credential
// is committed.
func profileWithSnapshot(t *testing.T) string {
	t.Helper()
	profile := t.TempDir()
	if err := os.MkdirAll(filepath.Join(profile, "sessionstore-backups"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/recovery.jsonlz4")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(profile, "sessionstore-backups", "recovery.jsonlz4"),
		raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestTheSessionCookieIsReadFromARealSnapshot(t *testing.T) {
	// This is the whole reason the package exists: Moodle's session cookie
	// has no expiry, so Firefox keeps it out of cookies.sqlite and writes it
	// here instead. Measured on Firefox 156 — cookies.sqlite held nothing at
	// all while this file held the cookie.
	found, err := browser.FirefoxSession(profileWithSnapshot(t), "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if found.Value == "" {
		t.Fatal("a cookie was found with no value")
	}
	if found.Name != "MoodleSession" {
		t.Errorf("name = %q", found.Name)
	}
	if !strings.Contains(found.Source, "recovery.jsonlz4") {
		t.Errorf("source = %q; the user should be able to see what was opened", found.Source)
	}
}

func TestOnlyTheHostThatWasAskedAboutIsReturned(t *testing.T) {
	// The snapshot holds two sites' cookies, which is the ordinary case: it
	// is one container for the whole browser. Asking about one must not hand
	// back the other.
	profile := profileWithSnapshot(t)

	first, err := browser.FirefoxSession(profile, "127.0.0.1", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := browser.FirefoxSession(profile, "192.168.50.169", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Host != "127.0.0.1" || second.Host != "192.168.50.169" {
		t.Errorf("hosts came back as %q and %q", first.Host, second.Host)
	}
}

func TestASiteWithNoCookieIsNotReportedAsSignedOut(t *testing.T) {
	// The snapshot says what it contained. It does not say whether the user
	// is signed in: it is written periodically, a privacy setting can exclude
	// it, and it covers one profile out of however many exist.
	_, err := browser.FirefoxSession(profileWithSnapshot(t), "moodle.example.edu", "")
	if err == nil {
		t.Fatal("a site absent from the snapshot was reported as found")
	}
	if code := errs.From(err).Code; code != errs.CodeNotFound {
		t.Errorf("code = %q, want not_found", code)
	}
	message := err.Error() + errs.From(err).Hint
	if strings.Contains(message, "not signed in") || strings.Contains(message, "not logged in") {
		t.Errorf("absence from a snapshot was reported as a fact about the user:\n%s", message)
	}
	if !strings.Contains(message, "snapshot") {
		t.Errorf("the answer does not say what it actually read:\n%s", message)
	}
}

func TestASubdomainCookieIsNotPickedUp(t *testing.T) {
	// A cookie for a whole university is not something to take because one of
	// its hosts was asked about.
	_, err := browser.FirefoxSession(profileWithSnapshot(t), "evil.127.0.0.1", "")
	if err == nil {
		t.Fatal("a cookie for a different host was returned")
	}
}

func TestAProfileThatHasNeverRunSaysSo(t *testing.T) {
	_, err := browser.FirefoxSession(t.TempDir(), "127.0.0.1", "")
	if err == nil {
		t.Fatal("a profile with no snapshot returned a cookie")
	}
	if !strings.Contains(err.Error(), "no session snapshot") {
		t.Errorf("message = %v", err)
	}
}

func TestARubbishSnapshotIsDriftNotAbsence(t *testing.T) {
	// A file this build cannot read and a site that is simply not in it must
	// not look the same: one means the format moved, the other means the user
	// has not signed in to that site in this profile.
	profile := t.TempDir()
	dir := filepath.Join(profile, "sessionstore-backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recovery.jsonlz4"),
		[]byte("not a mozLz4 file at all"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := browser.FirefoxSession(profile, "127.0.0.1", "")
	if err == nil {
		t.Fatal("an unreadable snapshot was reported as no cookie")
	}
	if code := errs.From(err).Code; code == errs.CodeNotFound {
		t.Errorf("an unreadable file was reported as an absent cookie")
	}
	if reason := errs.From(err).Reason; reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q, want protocol drift", reason)
	}
}

func TestTheSnapshotFirefoxWritesOnClosingIsReadToo(t *testing.T) {
	// Measured on Firefox 156: while it runs, the session lives in
	// sessionstore-backups/recovery.jsonlz4 and sessionstore.jsonlz4 does not
	// exist; once it closes cleanly the recovery files are gone and the
	// session is in sessionstore.jsonlz4 instead.
	//
	// A closed browser is the more likely state when someone turns to a
	// command line, so reading only the first would have found nothing
	// exactly when it mattered.
	profile := t.TempDir()
	raw, err := os.ReadFile("testdata/recovery.jsonlz4")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(profile, "sessionstore.jsonlz4"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	found, err := browser.FirefoxSession(profile, "127.0.0.1", "")
	if err != nil {
		t.Fatalf("a closed browser's session was not read: %v", err)
	}
	if !strings.HasSuffix(found.Source, "sessionstore.jsonlz4") {
		t.Errorf("source = %q", found.Source)
	}
}

func TestWhatFirefoxItselfCompressedIsDecoded(t *testing.T) {
	// The other fixture was repacked by this project's own tooling, which
	// emits literals only — so it never exercises the part of LZ4 that copies
	// from what it has already produced. These are bytes Firefox wrote, with
	// real matches: 1073 compressed from 1326.
	//
	// It carries no cookies at all, by construction: the browsing that
	// produced it visited a local page that sets none.
	profile := t.TempDir()
	if err := os.MkdirAll(filepath.Join(profile, "sessionstore-backups"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/firefox-compressed.jsonlz4")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "sessionstore-backups",
		"recovery.jsonlz4"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// Reaching "no cookie for that host" means the file decompressed and
	// parsed as JSON: a failure in either would have said something else.
	_, err = browser.FirefoxSession(profile, "moodle.example.edu", "")
	if err == nil {
		t.Fatal("a snapshot with no cookies returned one")
	}
	if code := errs.From(err).Code; code != errs.CodeNotFound {
		t.Fatalf("Firefox's own compression did not decode: %v", err)
	}
}
