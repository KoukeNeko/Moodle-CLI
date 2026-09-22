package auth_test

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func TestLoginStoresTheWholeCredential(t *testing.T) {
	store := secret.NewMemory()
	manager := auth.NewManager(store, nil)
	siteID, accountID := site.ID("site"), site.ID("account")
	if err := manager.StoreCredential(siteID, accountID, auth.Credential{
		Token: "ws-token", PrivateToken: "private-token",
	}); err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[secret.Kind]string{
		secret.KindWSToken: "ws-token", secret.KindPrivateToken: "private-token",
	} {
		got, err := store.Get(secret.Ref{SiteID: siteID, AccountID: accountID, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", kind, got, want)
		}
	}
}

func TestLoginWithoutAPrivateTokenClearsTheOldOne(t *testing.T) {
	store := secret.NewMemory()
	manager := auth.NewManager(store, nil)
	siteID, accountID := site.ID("site"), site.ID("account")
	ref := secret.Ref{SiteID: siteID, AccountID: accountID, Kind: secret.KindPrivateToken}
	if err := store.Set(ref, "stale-private-token"); err != nil {
		t.Fatal(err)
	}
	if err := manager.StoreCredential(siteID, accountID, auth.Credential{Token: "new-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ref); err == nil {
		t.Fatal("a private token from the previous login survived")
	}
}
