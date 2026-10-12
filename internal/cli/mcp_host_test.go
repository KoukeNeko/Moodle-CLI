package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

// runHost executes `mcp host` with no dependencies. Every case here has to be
// refused before anything is signed in or listened on, which is what zero
// dependencies proves: reaching them would panic.
func runHost(t *testing.T, mode safety.Mode, args ...string) error {
	t.Helper()
	cmd := newMCPHostCommand(Deps{}, &mode)
	cmd.SetArgs(args)
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))
	return cmd.Execute()
}

func TestHostRefusesWriteAndReadOnlyTogether(t *testing.T) {
	err := runHost(t, safety.Mode{ReadOnly: true},
		"--public-url", "https://moodle.example.com", "--allow-write")

	if got := errs.From(err); got.Code != errs.CodeUsage || !strings.Contains(got.Message, "opposite") {
		t.Errorf("err = %v, want the contradiction reported", err)
	}
}

func TestHostReportsAPassphraseFileItCannotRead(t *testing.T) {
	err := runHost(t, safety.Mode{},
		"--public-url", "https://moodle.example.com",
		"--passphrase-file", filepath.Join(t.TempDir(), "missing"))

	if got := errs.From(err); got.Code != errs.CodeConfiguration {
		t.Errorf("err = %v, want a configuration error", err)
	}
}

func TestHostReportsAnEmptyPassphraseFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "passphrase")
	if err := os.WriteFile(file, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runHost(t, safety.Mode{},
		"--public-url", "https://moodle.example.com", "--passphrase-file", file)

	if got := errs.From(err); got.Code != errs.CodeConfiguration || !strings.Contains(got.Message, "empty") {
		t.Errorf("err = %v, want the empty file named", err)
	}
}

func TestThePassphraseComesFromTheFileBeforeTheEnvironment(t *testing.T) {
	t.Setenv(EnvHostPassphrase, "from the environment")
	file := filepath.Join(t.TempDir(), "passphrase")
	if err := os.WriteFile(file, []byte("from the file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := hostPassphrase(file)

	if err != nil || got != "from the file" {
		t.Errorf("got %q, %v; want the file's text without its newline", got, err)
	}
}

func TestThePassphraseFallsBackToTheEnvironment(t *testing.T) {
	t.Setenv(EnvHostPassphrase, "  from the environment \n")

	got, err := hostPassphrase("")

	if err != nil || got != "from the environment" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestNoPassphraseAnywhereLeavesItToTheHostToGenerate(t *testing.T) {
	t.Setenv(EnvHostPassphrase, "")

	got, err := hostPassphrase("")

	if err != nil || got != "" {
		t.Errorf("got %q, %v; want empty so the host generates one", got, err)
	}
}

func TestAnEmptyPublicURLIsAnErrorNotAQuickTunnel(t *testing.T) {
	// Leaving the flag out asks for a tunnel. Passing it empty is a script
	// whose variable was unset, and must not publish the host.
	for _, args := range [][]string{
		{"--public-url="},
		{"--public-url", "  "},
	} {
		err := runHost(t, safety.Mode{}, args...)

		if got := errs.From(err); got.Code != errs.CodeUsage || !strings.Contains(got.Message, "--public-url is empty") {
			t.Errorf("%q: err = %v, want a usage error", args, err)
		}
	}
}
