package cli

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/browsersession"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/manual"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/password"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/token"
)

func TestSetupRecommendsAnIssuerInsteadOfAnExistingCredential(t *testing.T) {
	candidates := []auth.Candidate{
		{Method: token.New(), Probe: auth.ProbeResult{Availability: auth.Available}},
		{Method: password.New(nil, nil), Probe: auth.ProbeResult{Availability: auth.Available}},
		{Method: browsersession.New(nil, nil), Probe: auth.ProbeResult{Availability: auth.Available}},
		{Method: manual.New(), Probe: auth.ProbeResult{Availability: auth.Available}},
	}
	if got, _ := recommendation(candidates, &auth.PublicConfig{}); got != "password" {
		t.Errorf("Moodle password site: recommended %q, want password", got)
	}
	if got, _ := recommendation(candidates, &auth.PublicConfig{HasIdentityProviders: true}); got != "manual" {
		t.Errorf("SSO site: recommended %q, want browser login via manual callback", got)
	}

	candidates[1].Probe.Availability = auth.Unavailable
	candidates[2].Probe.Availability = auth.Unavailable
	candidates[3].Probe.Availability = auth.Unavailable
	if got, _ := recommendation(candidates, &auth.PublicConfig{}); got != "" {
		t.Errorf("only an existing token can be used: recommended %q, want no presumption", got)
	}
}
