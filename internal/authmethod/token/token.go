// Package token signs in with a web service token the user already holds.
//
// It is the most reliable method and needs nothing from the site, so it is
// first in the coordinator's order.
package token

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Method signs in with an existing token.
type Method struct{}

// New builds the method.
func New() *Method { return &Method{} }

func (*Method) Name() string { return "token" }

func (*Method) Describe() string {
	return "use a web service token you already have"
}

// Probe always reports available: a token is verified by using it, and the
// site's configuration says nothing about whether one exists.
func (*Method) Probe(context.Context, site.Site, *auth.PublicConfig) auth.ProbeResult {
	return auth.ProbeResult{Availability: auth.Available}
}

func (*Method) Authenticate(_ context.Context, req auth.Request) (auth.Credential, error) {
	if req.Token == "" {
		return auth.Credential{}, errs.New(errs.CodeUsage, "no token given").
			WithHint("pass --token <token>, or --token-stdin to read it from a pipe")
	}
	// The token is not checked here: the caller verifies it against the site
	// before storing it, so an unusable credential never reaches the keychain.
	return auth.Credential{Token: req.Token, Method: "token"}, nil
}
