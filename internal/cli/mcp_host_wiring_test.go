package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/cli"
)

// hostRun runs `mcp host` against the fake site with a server that only
// records what it was given.
func hostRun(t *testing.T, args ...string) (cli.MCPSession, cli.MCPHost, int) {
	t.Helper()
	t.Setenv(cli.EnvHostPassphrase, "")
	f := newFixture(t)
	f.addSiteAndLogin()

	var (
		session cli.MCPSession
		host    cli.MCPHost
	)
	f.deps.ServeMCPHost = func(_ context.Context, s cli.MCPSession, h cli.MCPHost) error {
		session, host = s, h
		return nil
	}
	_, stderr, code := f.run(append([]string{"mcp", "host"}, args...)...)
	if code != 0 && stderr == "" {
		t.Fatal("a failure said nothing")
	}
	return session, host, code
}

func TestHostListensOnLoopbackAndReadsOnlyByDefault(t *testing.T) {
	// Both are decisions about exposure, and the help text promises them.
	session, host, code := hostRun(t, "--public-url", "https://moodle-mcp.example.com")

	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if host.Addr != "127.0.0.1:8787" {
		t.Errorf("Addr = %q, want loopback", host.Addr)
	}
	if session.AllowWrite {
		t.Error("the host offered the writing tools without being asked")
	}
	if host.PublicURL != "https://moodle-mcp.example.com" || host.Passphrase != "" {
		t.Errorf("host = %+v", host)
	}
	if session.SiteName != "school" {
		t.Errorf("SiteName = %q: the host is not bound to the signed-in site", session.SiteName)
	}
}

func TestWithoutAPublicURLTheHostAsksForAQuickTunnelOnAFreePort(t *testing.T) {
	// An empty PublicURL is how the composition root is told to start the
	// tunnel, and a free port is what keeps that tunnel from reaching a service
	// that already holds the default one.
	session, host, code := hostRun(t)

	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if host.PublicURL != "" || host.Addr != "127.0.0.1:0" {
		t.Errorf("host = %+v, want no public URL and a free loopback port", host)
	}
	if session.AllowWrite {
		t.Error("the host offered the writing tools without being asked")
	}
}

func TestAnExplicitListenAddressWinsOverTheDefault(t *testing.T) {
	_, host, code := hostRun(t, "--listen", "127.0.0.1:9100")

	if code != 0 || host.Addr != "127.0.0.1:9100" {
		t.Errorf("exit %d, host = %+v", code, host)
	}
}

func TestHostCarriesItsFlagsThrough(t *testing.T) {
	file := filepath.Join(t.TempDir(), "passphrase")
	if err := os.WriteFile(file, []byte("correct horse battery\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	session, host, code := hostRun(t,
		"--public-url", "https://moodle-mcp.example.com",
		"--listen", "127.0.0.1:9000", "--allow-write", "--passphrase-file", file)

	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if host.Addr != "127.0.0.1:9000" || host.Passphrase != "correct horse battery" || !session.AllowWrite {
		t.Errorf("host = %+v, allow write = %v", host, session.AllowWrite)
	}
}
