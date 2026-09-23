package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func tempConfig(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.yaml")
}

func TestLoadMissingFileIsEmptyNotAnError(t *testing.T) {
	path := tempConfig(t)
	file, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(file.Sites) != 0 {
		t.Errorf("want no sites, got %v", file.SiteNames())
	}
	// Loading must not create the file; nothing is written until it is needed.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Load created %s", path)
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	path := tempConfig(t)
	file, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := file.AddSite("school", "moodle.example.edu")
	if err != nil {
		t.Fatal(err)
	}
	entry.UpsertAccount("student", config.Account{
		UserID:         "4",
		Username:       "student1",
		DisplayName:    "Sam Student",
		AuthMethod:     "token",
		CredentialKind: site.CredentialWSToken,
	})
	entry.Academic = config.Academic{
		CreditsField: "credits", LevelField: "academic_level", TermField: "academic_term",
		Minimum: config.AcademicMinimum{Undergraduate: 21, Graduate: 6},
	}
	if err := file.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.Sites["school"]
	if !ok {
		t.Fatalf("site missing after reload: %v", reloaded.SiteNames())
	}
	if got.ID != entry.ID {
		t.Errorf("site id changed: %q -> %q", entry.ID, got.ID)
	}
	if got.BaseURL != "https://moodle.example.edu" {
		t.Errorf("base_url = %q", got.BaseURL)
	}
	if got.Accounts["student"].Username != "student1" {
		t.Errorf("account not preserved: %+v", got.Accounts["student"])
	}
	if got.Academic.Minimum.Undergraduate != 21 || got.Academic.Minimum.Graduate != 6 {
		t.Errorf("academic settings not preserved: %+v", got.Academic)
	}
	if reloaded.Current.Site != "school" {
		t.Errorf("first site should become current, got %q", reloaded.Current.Site)
	}
}

func TestSavedFileIsNotWorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not apply on Windows")
	}
	path := tempConfig(t)
	file, _ := config.Load(path)
	if _, err := file.AddSite("school", "https://example.edu"); err != nil {
		t.Fatal(err)
	}
	if err := file.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The file carries usernames and site addresses; it is private.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %v, want 0600", perm)
	}
}

func TestSaveRefusesToWriteSomethingThatLooksLikeACredential(t *testing.T) {
	// The keychain is the only place a credential may live. If one ever
	// reaches the config struct, saving must fail loudly rather than put it
	// on disk.
	path := tempConfig(t)
	file, _ := config.Load(path)
	entry, err := file.AddSite("school", "https://example.edu")
	if err != nil {
		t.Fatal(err)
	}
	entry.Extra = map[string]any{"wstoken": "c9e291c29aa29ecb3d932383d09d16c1"}

	err = file.Save()
	if err == nil {
		t.Fatal("Save accepted a credential")
	}
	if code := errs.From(err).Code; code != errs.CodeInternal {
		t.Errorf("code = %q, want internal", code)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("the rejected configuration was written anyway")
	}
}

func TestUnknownFieldsSurviveARewrite(t *testing.T) {
	path := tempConfig(t)
	raw := `schema_version: 1
current:
  site: school
  account: ""
sites:
  school:
    id: 11111111-1111-4111-8111-111111111111
    base_url: https://example.edu
    www_root: ""
    backend: auto
    default_account: ""
    accounts: {}
    future_field: keep me
future_top_level: keep me too
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Save(); err != nil {
		t.Fatal(err)
	}
	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"future_field", "future_top_level"} {
		if !strings.Contains(string(rewritten), needle) {
			t.Errorf("%q was dropped:\n%s", needle, rewritten)
		}
	}
}

func TestNewerSchemaVersionIsRefused(t *testing.T) {
	// Guessing at a format we do not know could destroy settings.
	path := tempConfig(t)
	if err := os.WriteFile(path, []byte("schema_version: 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("Load accepted a newer schema version")
	}
	e := errs.From(err)
	if e.Code != errs.CodeConfiguration {
		t.Errorf("code = %q, want configuration", e.Code)
	}
	if !strings.Contains(e.Hint, "upgrade") {
		t.Errorf("hint should tell the user to upgrade, got %q", e.Hint)
	}
}

func TestOlderSchemaVersionIsMigratedAfterABackup(t *testing.T) {
	path := tempConfig(t)
	raw := "schema_version: 0\nsites: {}\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.SchemaVersion != config.SchemaVersion {
		t.Errorf("schema_version = %d, want %d", file.SchemaVersion, config.SchemaVersion)
	}
	backup := path + ".bak.v0"
	if got, err := os.ReadFile(backup); err != nil {
		t.Errorf("no backup at %s: %v", backup, err)
	} else if string(got) != raw {
		t.Errorf("backup does not match the original:\n%s", got)
	}
}

func TestResolvePrefersFlagThenCurrentThenOnlyCandidate(t *testing.T) {
	file, _ := config.Load(tempConfig(t))
	school, _ := file.AddSite("school", "https://school.example.edu")
	school.UpsertAccount("student", config.Account{Username: "student1"})
	if _, err := file.AddSite("work", "https://work.example.edu"); err != nil {
		t.Fatal(err)
	}

	// Current was set to the first site added.
	resolved, err := file.Resolve("", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.SiteName != "school" || resolved.AccountName != "student" {
		t.Errorf("got %q/%q, want school/student", resolved.SiteName, resolved.AccountName)
	}

	// An explicit site wins over the current one.
	resolved, err = file.Resolve("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.SiteName != "work" {
		t.Errorf("got site %q, want work", resolved.SiteName)
	}
	// ...and must not drag the other site's account along with it.
	if resolved.Account != nil {
		t.Errorf("account %q leaked onto site work", resolved.AccountName)
	}
}

func TestResolveNamesTheChoicesInsteadOfGuessing(t *testing.T) {
	file, _ := config.Load(tempConfig(t))
	if _, err := file.AddSite("a", "https://a.example.edu"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.AddSite("b", "https://b.example.edu"); err != nil {
		t.Fatal(err)
	}
	file.Current = config.Current{}

	_, err := file.Resolve("", "")
	if err == nil {
		t.Fatal("Resolve guessed between two sites")
	}
	hint := errs.From(err).Hint
	if !strings.Contains(hint, "a") || !strings.Contains(hint, "b") {
		t.Errorf("hint should list the candidates, got %q", hint)
	}
}

func TestRequireAccountExplainsHowToSignIn(t *testing.T) {
	file, _ := config.Load(tempConfig(t))
	if _, err := file.AddSite("school", "https://example.edu"); err != nil {
		t.Fatal(err)
	}
	_, err := file.RequireAccount("", "")
	if err == nil {
		t.Fatal("RequireAccount succeeded without an account")
	}
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if e.Reason != errs.ReasonCredentialMissing {
		t.Errorf("reason = %q, want credential_missing", e.Reason)
	}
}

func TestRemoveSiteReportsAccountsSoCredentialsCanBeCleaned(t *testing.T) {
	file, _ := config.Load(tempConfig(t))
	school, _ := file.AddSite("school", "https://example.edu")
	school.UpsertAccount("student", config.Account{Username: "student1"})
	school.UpsertAccount("ta", config.Account{Username: "ta1"})

	accounts, err := file.RemoveSite("school")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Errorf("got %d accounts, want 2", len(accounts))
	}
	if file.Current.Site != "" {
		t.Errorf("current still points at the removed site: %q", file.Current.Site)
	}
}

func TestAccountIDIsStableAcrossUpdates(t *testing.T) {
	// The keychain key is built from this ID; if it changed on update the
	// stored credential would be orphaned.
	file, _ := config.Load(tempConfig(t))
	school, _ := file.AddSite("school", "https://example.edu")
	first := school.UpsertAccount("student", config.Account{Username: "old"})
	id := first.ID
	second := school.UpsertAccount("student", config.Account{Username: "new"})
	if second.ID != id {
		t.Errorf("account id changed on update: %q -> %q", id, second.ID)
	}
	if second.Username != "new" {
		t.Errorf("update did not apply: %q", second.Username)
	}
}
