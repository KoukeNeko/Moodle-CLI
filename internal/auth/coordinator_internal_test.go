package auth

import (
	"context"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/site"
)

type namedMethod string

func (m namedMethod) Name() string   { return string(m) }
func (namedMethod) Describe() string { return "test method" }
func (namedMethod) Probe(context.Context, site.Site, *PublicConfig) ProbeResult {
	return ProbeResult{Availability: Available}
}
func (namedMethod) Authenticate(context.Context, Request) (Credential, error) {
	return Credential{}, nil
}

func TestBrowserSessionAlwaysNeedsAnExplicitCredential(t *testing.T) {
	method := namedMethod("browser-session")
	if !needsInput(method, Request{}) {
		t.Fatal("browser-session could be selected without a supplied session")
	}
	if needsInput(method, Request{SessionCookie: "MoodleSession=value"}) {
		t.Fatal("an explicitly supplied session was treated as missing")
	}
}
