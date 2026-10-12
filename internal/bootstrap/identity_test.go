package bootstrap

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/tunnel"
)

func TestNamedPutsTheSitesOwnNameFirst(t *testing.T) {
	for _, c := range []struct{ label, reported, want string }{
		{"school", "Example University", "Example University (school)"},
		{"school", "", "school"},
		{"school", "school", "school"},
	} {
		if got := named(c.label, c.reported); got != c.want {
			t.Errorf("named(%q, %q) = %q, want %q", c.label, c.reported, got, c.want)
		}
	}
}

func TestTheAccountIsNamedAsMoodleKnowsIt(t *testing.T) {
	for name, c := range map[string]struct {
		capabilities site.Capabilities
		want         string
	}{
		"both":     {site.Capabilities{FullName: "Sam Student", Username: "student1"}, "Sam Student, student1"},
		"username": {site.Capabilities{Username: "student1"}, "student1"},
		"fullname": {site.Capabilities{FullName: "Sam Student"}, "Sam Student"},
		"neither":  {site.Capabilities{}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := accountOnSite(&c.capabilities); got != c.want {
				t.Errorf("accountOnSite = %q, want %q", got, c.want)
			}
		})
	}
}

func TestATunnelIsPointedAtTheLoopbackAddressTheHostBoundTo(t *testing.T) {
	for _, c := range []struct {
		address *net.TCPAddr
		want    string
	}{
		{&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5000}, "http://127.0.0.1:5000"},
		{&net.TCPAddr{IP: net.IPv6loopback, Port: 5003}, "http://[::1]:5003"},
	} {
		if got := localURL(c.address); got != c.want {
			t.Errorf("localURL(%v) = %q, want %q", c.address, got, c.want)
		}
	}
}

func TestATunnelIsOnlyEverPointedAtLoopback(t *testing.T) {
	// A wildcard bind can succeed on some systems while another program holds
	// the loopback address; the tunnel would then publish that program.
	for _, c := range []struct {
		address *net.TCPAddr
		ok      bool
	}{
		{&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}, true},
		{&net.TCPAddr{IP: net.IPv6loopback, Port: 1}, true},
		{&net.TCPAddr{IP: net.IPv4zero, Port: 1}, false},
		{&net.TCPAddr{IP: net.IPv6unspecified, Port: 1}, false},
		{&net.TCPAddr{IP: net.IPv4(192, 168, 1, 5), Port: 1}, false},
	} {
		err := requireLoopback(c.address)
		if (err == nil) != c.ok {
			t.Errorf("requireLoopback(%v) = %v, want ok = %v", c.address, err, c.ok)
		}
		if err != nil && errs.From(err).Code != errs.CodeUsage {
			t.Errorf("requireLoopback(%v): code %s, want usage", c.address, errs.From(err).Code)
		}
	}
}

func TestMoodleCredentialsAreNotHandedToCloudflared(t *testing.T) {
	got := withoutPrefix([]string{"PATH=/bin", "MOODLE_WS_TOKEN=secret", "moodle_session=x", "HOME=/h"}, "MOODLE_")

	if strings.Join(got, ",") != "PATH=/bin,HOME=/h" {
		t.Errorf("environment = %v", got)
	}
}

func TestAMissingCloudflaredIsExplainedInTermsOfTheFlags(t *testing.T) {
	err := tunnelUnavailable(errors.Join(tunnel.ErrNotInstalled, errors.New("not on PATH")))

	got := errs.From(err)
	if got.Code != errs.CodeUnavailable || !strings.Contains(got.Hint, "install cloudflared") ||
		!strings.Contains(got.Hint, "--public-url") {
		t.Errorf("err = %v, hint %q: want to be told to install it, or to pass --public-url", err, got.Hint)
	}
}

func TestAnyOtherTunnelFailureOnlySuggestsAnAddressOfYourOwn(t *testing.T) {
	err := tunnelUnavailable(errors.New("quick tunnel provisioning failed with status 429"))

	got := errs.From(err)
	if strings.Contains(got.Hint, "install cloudflared") || !strings.Contains(got.Hint, "--public-url") {
		t.Errorf("hint = %q: installing cloudflared would not help here", got.Hint)
	}
}
