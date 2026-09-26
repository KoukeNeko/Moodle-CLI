package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func TestCompletionCandidatesGoToStdout(t *testing.T) {
	// The shell reads candidates from stdout. Cobra's out is pointed at
	// stderr here so that help and usage stay off stdout, which sent every
	// candidate to the wrong stream and made completion silently empty.
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	stdout, _, code := f.run("__complete", "course", "list", "--site", "")
	if code != v1.ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "school") {
		t.Errorf("the site is not offered on stdout:\n%s", stdout)
	}
}

func TestSiteAndAccountCompletionComeFromTheConfiguration(t *testing.T) {
	// Completions run while someone is still typing, so they read the
	// configuration and never the network.
	f := newFixture(t)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	if _, stderr, code := f.run("site", "add", "other", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	if _, stderr, code := f.run("auth", "login", "--token", "good-token",
		"--site", "school", "--account", "sam"); code != 0 {
		t.Fatalf("auth login: %s", stderr)
	}

	// Whatever signing in cost, completions must add nothing to it.
	afterLogin := len(f.server.Requests())

	// The URL is what tells two similar names apart.
	stdout, _, _ := f.run("__complete", "course", "list", "--site", "")
	if !strings.Contains(stdout, "school\t"+f.server.URL()) {
		t.Errorf("a site should be offered with its URL:\n%s", stdout)
	}
	if stdout, _, _ := f.run("__complete", "course", "list", "--site", "sch"); strings.Contains(stdout, "other") {
		t.Errorf("a prefix should narrow the sites:\n%s", stdout)
	}

	// An account name is unique only within its site, so --site decides.
	stdout, _, _ = f.run("__complete", "course", "list", "--site", "school", "--account", "")
	if !strings.Contains(stdout, "sam") {
		t.Errorf("the account is not offered:\n%s", stdout)
	}
	stdout, _, _ = f.run("__complete", "course", "list", "--site", "other", "--account", "")
	if strings.Contains(stdout, "sam") {
		t.Errorf("another site's account was offered:\n%s", stdout)
	}
	// Reaching the network is the thing being avoided here: a shell runs
	// these while the user is still typing.
	if calls := len(f.server.Requests()); calls != afterLogin {
		t.Errorf("completions made %d requests", calls-afterLogin)
	}
}

func TestShellCompletionPrintsAScriptForEachShell(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		stdout, stderr, code := run(t, "shell-completion", shell)
		if code != v1.ExitOK {
			t.Errorf("%s: exit %d: %s", shell, code, stderr)
			continue
		}
		if !strings.Contains(stdout, "moodle") || len(stdout) < 200 {
			t.Errorf("%s: output does not look like a script (%d bytes)", shell, len(stdout))
		}
	}
}

func TestShellCompletionNamesTheShellsItKnows(t *testing.T) {
	_, stderr, code := run(t, "shell-completion", "csh")
	if code != v1.ExitUsage {
		t.Fatalf("exit %d, want usage", code)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		if !strings.Contains(stderr, shell) {
			t.Errorf("the refusal does not offer %s: %s", shell, stderr)
		}
	}
}

func TestTheFileCredentialStoreKeepsSomeoneSignedIn(t *testing.T) {
	// The case this exists for: a machine with no keychain, where every
	// command would otherwise need the credential passed to it again. The
	// real store is wired in rather than the in-memory one, so the file on
	// disk is what the second command reads.
	f := newFixture(t)
	chosen := new(secret.Backend)
	f.deps.CredentialStore = chosen
	store := secret.Selected{Backend: chosen, ConfigPath: f.deps.ConfigPath}
	manager := auth.NewManager(store, func(target site.Site) *moodle.Client {
		return moodle.NewClient(target)
	})
	f.deps.Auth = manager
	f.deps.Login = testCoordinator(manager)
	if _, stderr, code := f.run("site", "add", "school", f.server.URL()); code != 0 {
		t.Fatalf("site add: %s", stderr)
	}
	stdout, stderr, code := f.run("--credential-store", "file", "auth", "login", "--token", "good-token")
	if code != v1.ExitOK {
		t.Fatalf("login exit %d: %s", code, stderr)
	}
	// Where it went is the difference between being able to remove it and not.
	if !strings.Contains(stdout, "readable only by this user") {
		t.Errorf("login did not say where the credential was kept:\n%s", stdout)
	}

	// A later command finds it, which is the whole point.
	if stdout, stderr, code := f.run("--credential-store", "file", "auth", "status"); code != v1.ExitOK {
		t.Fatalf("status exit %d: %s%s", code, stdout, stderr)
	}
	// The file itself, at the mode other accounts cannot read.
	info, err := os.Stat(filepath.Join(filepath.Dir(f.deps.ConfigPath), secret.FileName))
	if err != nil {
		t.Fatalf("nothing was written to the file store: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %04o, want 0600", mode)
	}
}
