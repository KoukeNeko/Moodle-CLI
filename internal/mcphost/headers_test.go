package mcphost

import (
	"strings"
	"testing"
)

func TestTheConsentPageIsNeverCachedFramedOrSniffed(t *testing.T) {
	// X-Frame-Options and frame-ancestors are the only clickjacking defence
	// for the page that asks for the passphrase.
	h := newTestHost(t)

	response, _ := h.get(h.authorizeQuery(h.register(claudeCallback), nil))

	for name, want := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := response.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	policy := response.Header.Get("Content-Security-Policy")
	if !strings.Contains(policy, "frame-ancestors 'none'") {
		t.Errorf("Content-Security-Policy = %q, want it to forbid framing", policy)
	}
	// Browsers apply form-action to the redirect that follows a submission,
	// and that redirect leaves for Claude.
	if strings.Contains(policy, "form-action") {
		t.Errorf("Content-Security-Policy = %q, want no form-action", policy)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %q", response.Header.Get("Content-Type"))
	}
}

func TestTokenAndErrorAnswersAreNeverCached(t *testing.T) {
	// RFC 6749 requires it of every token response, and an error answer can
	// carry a client's own state back to it.
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)

	issued, _ := h.exchange(clientID, code)
	refused, _ := h.exchange(clientID, code)

	for name, response := range map[string]interface{ Get(string) string }{
		"token":       issued.Header,
		"token error": refused.Header,
	} {
		if got := response.Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", name, got)
		}
		if got := response.Get("Content-Type"); got != "application/json" {
			t.Errorf("%s: Content-Type = %q, want application/json", name, got)
		}
	}
	if got := issued.Header.Get("Pragma"); got != "no-cache" {
		t.Errorf("token: Pragma = %q, want no-cache", got)
	}
}
