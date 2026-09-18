package auth

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// TestASessionCarriesOneCredentialNotTwo locks down the rule that keeps two
// people's data apart.
//
// A token and a browser session are separate principals: the token belongs to
// whoever it was issued to, the cookie to whoever signed in. The composition
// root builds a web service backend from the token and a page-reading one from
// the cookie, and a feature falls from the first to the second on its own. If a
// session ever held both, a fallback would answer a question about one account
// with the other's data — and say nothing, because both routes succeeded.
//
// Nothing downstream compares the two principals, and nothing needs to while
// this holds: every constructor sets one and leaves the other empty.
func TestASessionCarriesOneCredentialNotTwo(t *testing.T) {
	manager := NewManager(nil, func(site.Site) *moodle.Client { return nil })
	target := site.Site{}

	withToken := manager.OpenWithToken(target, "account", "a-token")
	if !withToken.HasToken() {
		t.Fatal("a token session has no token")
	}
	if withToken.Cookie().Value != "" {
		t.Error("a token session also carried a browser session, so a fallback " +
			"could answer as a different account")
	}

	withCookie := manager.OpenWithSession(target, "account", "MoodleSession=abc")
	if withCookie.Cookie().Value == "" {
		t.Fatal("a browser session has no cookie")
	}
	if withCookie.HasToken() {
		t.Error("a browser session also carried a token, so a fallback " +
			"could answer as a different account")
	}
}
