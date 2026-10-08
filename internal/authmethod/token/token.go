// Package token signs in with a web service token the user already holds.
//
// It is the most reliable method and needs nothing from the site, so it is
// first in the coordinator's order.
package token

import (
	"context"
	"fmt"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Method signs in with an existing token.
type Method struct{ readHidden func() (string, error) }

// New builds the method.
func New(readHidden ...func() (string, error)) *Method {
	m := &Method{}
	if len(readHidden) > 0 {
		m.readHidden = readHidden[0]
	}
	return m
}

func (*Method) Name() string { return "token" }

func (*Method) Describe() string {
	return "use a web service token you already have"
}

// Probe always reports available: a token is verified by using it, and the
// site's configuration says nothing about whether one exists.
func (*Method) Probe(context.Context, site.Site, *auth.PublicConfig) auth.ProbeResult {
	return auth.ProbeResult{Availability: auth.Available}
}

func (m *Method) Authenticate(_ context.Context, req auth.Request) (auth.Credential, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		if req.In == nil || m.readHidden == nil {
			return auth.Credential{}, errs.New(errs.CodeUsage, "no token given").
				WithHint("use --token-stdin to read an existing token from a pipe; if you do not have one, choose password or a browser login method")
		}
		fmt.Fprint(req.Out, "Paste your existing Moodle web service token (input hidden): ")
		read, err := m.readHidden()
		fmt.Fprintln(req.Out)
		if err != nil {
			return auth.Credential{}, errs.Wrap(errs.CodeUsage, err, "cannot read the token")
		}
		token = strings.TrimSpace(read)
		if token == "" {
			return auth.Credential{}, errs.New(errs.CodeUsage, "empty token")
		}
	}
	// The token is not checked here: the caller verifies it against the site
	// before storing it, so an unusable credential never reaches the keychain.
	return auth.Credential{Token: token, Method: "token"}, nil
}
