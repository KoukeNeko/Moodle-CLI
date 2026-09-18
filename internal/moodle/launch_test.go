package moodle_test

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
)

// callbackFor builds the payload Moodle would send for a given site and
// passport, so the tests exercise the real format rather than a guess.
func callbackFor(wwwRoot, passport, token, privateToken string) string {
	sum := md5.Sum([]byte(wwwRoot + passport))
	payload := hex.EncodeToString(sum[:]) + ":::" + token
	if privateToken != "" {
		payload += ":::" + privateToken
	}
	return "moodlemobile://token=" + base64.StdEncoding.EncodeToString([]byte(payload))
}

func TestParseTokenCallback(t *testing.T) {
	raw := callbackFor("https://moodle.example.edu", "abc123", "tok-1", "priv-1")
	got, err := moodle.ParseTokenCallback(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "tok-1" {
		t.Errorf("token = %q", got.Token)
	}
	if got.PrivateToken != "priv-1" {
		t.Errorf("private token = %q", got.PrivateToken)
	}
}

func TestParseTokenCallbackAcceptsAnyScheme(t *testing.T) {
	// A site can force its own URL scheme, and that setting is invisible
	// before signing in. Rejecting an unexpected scheme would only block a
	// user who did everything right.
	base := callbackFor("https://moodle.example.edu", "abc123", "tok-1", "")
	payload := strings.TrimPrefix(base, "moodlemobile://token=")
	for _, scheme := range []string{"moodlemobile", "moodlecli", "somebrandedapp"} {
		if _, err := moodle.ParseTokenCallback(scheme + "://token=" + payload); err != nil {
			t.Errorf("scheme %q rejected: %v", scheme, err)
		}
	}
}

func TestParseTokenCallbackAcceptsUnpaddedBase64(t *testing.T) {
	// What a user pastes depends on their browser and on how they copied it.
	sum := md5.Sum([]byte("https://x" + "p"))
	payload := hex.EncodeToString(sum[:]) + ":::tok"
	unpadded := base64.RawStdEncoding.EncodeToString([]byte(payload))
	if _, err := moodle.ParseTokenCallback("moodlemobile://token=" + unpadded); err != nil {
		t.Errorf("unpadded base64 rejected: %v", err)
	}
}

func TestParseTokenCallbackRejectsRubbish(t *testing.T) {
	for _, raw := range []string{
		"",
		"https://moodle.example.edu/login",
		"moodlemobile://token=not-base64!!",
		"moodlemobile://token=" + base64.StdEncoding.EncodeToString([]byte("no-separators")),
	} {
		if _, err := moodle.ParseTokenCallback(raw); err == nil {
			t.Errorf("ParseTokenCallback(%q) should have failed", raw)
		}
	}
}

func TestVerifyPassportAcceptsTheMatchingLogin(t *testing.T) {
	const root, passport = "https://moodle.example.edu", "abc123"
	callback, err := moodle.ParseTokenCallback(callbackFor(root, passport, "tok", ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := moodle.VerifyPassport(callback, root, passport); err != nil {
		t.Errorf("a matching callback was rejected: %v", err)
	}
}

func TestVerifyPassportRejectsAnotherLogin(t *testing.T) {
	// Without this check, any process on the machine could hand the CLI a
	// token — including one for a different site.
	const root = "https://moodle.example.edu"
	callback, err := moodle.ParseTokenCallback(callbackFor(root, "someone-elses-passport", "tok", ""))
	if err != nil {
		t.Fatal(err)
	}
	err = moodle.VerifyPassport(callback, root, "my-passport")
	if err == nil {
		t.Fatal("a callback from another login attempt was accepted")
	}
	if code := errs.From(err).Code; code != errs.CodeValidation {
		t.Errorf("code = %q, want validation", code)
	}
}

func TestVerifyPassportRejectsAnotherSite(t *testing.T) {
	const passport = "abc123"
	callback, err := moodle.ParseTokenCallback(
		callbackFor("https://evil.example.com", passport, "tok", ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := moodle.VerifyPassport(callback, "https://moodle.example.edu", passport); err == nil {
		t.Fatal("a callback minted for another site was accepted")
	}
}

func TestPassportsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		passport := moodle.NewPassport()
		if seen[passport] {
			t.Fatalf("duplicate passport %q", passport)
		}
		seen[passport] = true
	}
}

func TestLaunchURL(t *testing.T) {
	got := moodle.LaunchURL("https://moodle.example.edu/", moodle.MobileService, "abc123", "moodlecli")
	for _, want := range []string{
		"https://moodle.example.edu/admin/tool/mobile/launch.php?",
		"service=moodle_mobile_app",
		"passport=abc123",
		"urlscheme=moodlecli",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("LaunchURL missing %q:\n%s", want, got)
		}
	}
	// The trailing slash on the site root must not produce a double slash.
	if strings.Contains(got, "edu//admin") {
		t.Errorf("double slash in %q", got)
	}
}

func TestParseQRLogin(t *testing.T) {
	got, err := moodle.ParseQRLogin(
		"moodlemobile://https://moodle.example.edu?qrlogin=KEY123&userid=42")
	if err != nil {
		t.Fatal(err)
	}
	if got.SiteURL != "https://moodle.example.edu" {
		t.Errorf("site = %q", got.SiteURL)
	}
	if got.Key != "KEY123" || got.UserID != "42" {
		t.Errorf("got %+v", got)
	}
}

func TestParseQRLoginRejectsASiteOnlyCode(t *testing.T) {
	// Moodle can show a QR that only holds the site address. It looks the
	// same to a user, so the message has to explain the difference.
	_, err := moodle.ParseQRLogin("moodlemobile://https://moodle.example.edu")
	if err == nil {
		t.Fatal("a site-only QR was accepted as a login code")
	}
	if hint := errs.From(err).Hint; !strings.Contains(hint, "QR login") {
		t.Errorf("hint should explain the difference, got %q", hint)
	}
}
