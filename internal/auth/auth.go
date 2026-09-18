// Package auth turns a configured account into a usable session.
//
// It is the seam between the command layer and the transport: commands ask
// for a session, this package finds the credential and builds the client.
// Without it the CLI would be calling Moodle directly, which the import rules
// forbid precisely because that is how presentation and protocol get welded
// together.
package auth

import (
	"context"

	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/secret"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// ClientFactory builds a transport client for a site. The composition root
// supplies it, so tests can point a session at a fake Moodle.
type ClientFactory func(target site.Site) *moodle.Client

// Manager opens sessions and stores credentials.
type Manager struct {
	secrets   secret.Store
	newClient ClientFactory
}

// NewManager builds a Manager.
func NewManager(secrets secret.Store, newClient ClientFactory) *Manager {
	return &Manager{secrets: secrets, newClient: newClient}
}

// PublicConfig is a site's pre-login configuration: enough to decide which
// login methods are possible before any credential exists.
type PublicConfig struct {
	SiteName string
	WWWRoot  string
	// TypeOfLogin is 1 in-app, 2 browser, 3 embedded browser.
	TypeOfLogin int
	// QRCodeType is 0 disabled, 1 site URL only, 2 login.
	QRCodeType             int
	EnableWebServices      int
	EnableMobileWebService int
	ShowLoginForm          int
	MaintenanceEnabled     int
	HasIdentityProviders   bool
}

// Prober reads a site's pre-login configuration.
type Prober interface {
	PublicConfig(ctx context.Context) (*PublicConfig, error)
}

// Capabilitier reports what the signed-in account may do.
type Capabilitier interface {
	Capabilities(ctx context.Context) (*site.Capabilities, error)
}

// Probe returns a Prober for a site, usable before signing in.
func (m *Manager) Probe(target site.Site) Prober {
	return &prober{client: m.newClient(target)}
}

type prober struct{ client *moodle.Client }

func (p *prober) PublicConfig(ctx context.Context) (*PublicConfig, error) {
	config, err := p.client.PublicConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &PublicConfig{
		SiteName:               config.SiteName,
		WWWRoot:                config.WWWRoot,
		TypeOfLogin:            config.TypeOfLogin,
		QRCodeType:             config.QRCodeType,
		EnableWebServices:      config.EnableWebServices,
		EnableMobileWebService: config.EnableMobileWebService,
		ShowLoginForm:          config.ShowLoginForm,
		MaintenanceEnabled:     config.MaintenanceEnabled,
		HasIdentityProviders:   len(config.IdentityProviders) > 0,
	}, nil
}

// Session is an authenticated connection to one site as one account.
type Session struct {
	client    *moodle.Client
	token     string
	accountID site.ID
	cached    *site.Capabilities
	// cookie is a browser session, used on sites that issue no token.
	cookie moodle.SessionCookie
}

// Open builds a session from a stored credential.
//
// The token is read at the moment it is needed rather than kept in the
// configuration, so it only ever lives in the keychain and in memory.
func (m *Manager) Open(target site.Site, siteID, accountID site.ID) (*Session, error) {
	token, err := m.Token(siteID, accountID)
	if err != nil {
		return nil, err
	}
	return m.OpenWithToken(target, accountID, token), nil
}

// OpenWithToken builds a session from a token the caller already holds, such
// as one supplied through the environment for a single run.
func (m *Manager) OpenWithToken(target site.Site, accountID site.ID, token string) *Session {
	return &Session{client: m.newClient(target), token: token, accountID: accountID}
}

// OpenWithSession builds a session that authenticates with a browser session
// rather than a token, for sites that issue none.
// The cookie is given as text because the command layer may not name the
// transport's types; parsing it is this seam's job.
func (m *Manager) OpenWithSession(target site.Site, accountID site.ID, cookie string) *Session {
	return &Session{
		client:    m.newClient(target),
		accountID: accountID,
		cookie:    moodle.ParseSessionCookie(cookie),
	}
}

// Token returns the stored web service token for an account.
func (m *Manager) Token(siteID, accountID site.ID) (string, error) {
	return m.secrets.Get(secret.Ref{
		SiteID: siteID, AccountID: accountID, Kind: secret.KindWSToken,
	})
}

// StoreToken saves a verified token.
func (m *Manager) StoreToken(siteID, accountID site.ID, token string) error {
	return m.secrets.Set(secret.Ref{
		SiteID: siteID, AccountID: accountID, Kind: secret.KindWSToken,
	}, token)
}

// Forget deletes every credential held for an account.
//
// It never asks Moodle to revoke the token: the same token is often the one
// the user's phone app holds, and signing out here must not sign them out
// there.
func (m *Manager) Forget(siteID, accountID site.ID) error {
	for _, kind := range []secret.Kind{
		secret.KindWSToken, secret.KindPrivateToken, secret.KindSession,
	} {
		if err := m.secrets.Delete(secret.Ref{
			SiteID: siteID, AccountID: accountID, Kind: kind,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Client and Token expose what the composition root needs to assemble feature
// backends. They are not for command code: the import rules stop the command
// layer from naming the transport at all.
func (s *Session) Client() *moodle.Client { return s.client }

// Token returns the credential this session authenticates with.
func (s *Session) Token() string { return s.token }

// Cookie returns the browser session, empty when there is none.
func (s *Session) Cookie() moodle.SessionCookie { return s.cookie }

// HasToken reports whether a web service token is available.
func (s *Session) HasToken() bool { return s.token != "" }

// Capabilities reports what this account may do, fetching once per process.
//
// Nothing is cached between runs: the CLI is short-lived, and a stale idea of
// what a site allows is worse than the one request it would save.
func (s *Session) Capabilities(ctx context.Context) (*site.Capabilities, error) {
	if s.cached != nil {
		return s.cached, nil
	}
	if s.token == "" && s.cookie.Value != "" {
		// A browser session cannot be asked what the site offers:
		// core_webservice_get_site_info is not exposed over the AJAX endpoint,
		// and nothing there lists what is. An empty set is the honest answer —
		// it lets a backend that needs no capability run, and stops one that
		// needs a token with a reason rather than a failed probe.
		capabilities := site.NewCapabilities()
		capabilities.AccountID = s.accountID
		capabilities.Credential = site.CredentialBrowserSession
		s.cached = capabilities
		return capabilities, nil
	}
	capabilities, err := s.client.SiteInfo(ctx, s.token, s.accountID)
	if err != nil {
		return nil, err
	}
	s.cached = capabilities
	return capabilities, nil
}
