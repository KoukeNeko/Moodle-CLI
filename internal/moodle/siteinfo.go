package moodle

import (
	"context"
	"strconv"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// FunctionSiteInfo is the function every session starts with.
const FunctionSiteInfo = "core_webservice_get_site_info"

// FunctionPublicConfig can be called before signing in, and tells us which
// login methods a site supports.
const FunctionPublicConfig = "tool_mobile_get_public_config"

// siteInfoDTO is Moodle's reply. It is unexported on purpose: upstream field
// names must not leak into the rest of the program, let alone into the public
// JSON contract.
type siteInfoDTO struct {
	SiteName  string `json:"sitename"`
	Username  string `json:"username"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	FullName  string `json:"fullname"`
	UserID    int64  `json:"userid"`
	Release   string `json:"release"`
	// SiteURL is the root Moodle builds its own links from, which can differ
	// from the address the user typed. File URLs are built from this one.
	SiteURL string `json:"siteurl"`
	Version string `json:"version"`
	// Moodle sends 0 or 1 here, not a JSON boolean.
	DownloadFiles int `json:"downloadfiles"`
	UploadFiles   int `json:"uploadfiles"`
	Functions     []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"functions"`
}

// PublicConfig is the pre-login view of a site.
type PublicConfig struct {
	WWWRoot  string `json:"wwwroot"`
	SiteName string `json:"sitename"`
	// TypeOfLogin is 1 in-app, 2 browser, 3 embedded browser.
	TypeOfLogin int    `json:"typeoflogin"`
	LaunchURL   string `json:"launchurl"`
	// QRCodeType is 0 disabled, 1 site URL only, 2 login.
	QRCodeType             int `json:"tool_mobile_qrcodetype"`
	EnableWebServices      int `json:"enablewebservices"`
	EnableMobileWebService int `json:"enablemobilewebservice"`
	ShowLoginForm          int `json:"showloginform"`
	MaintenanceEnabled     int `json:"maintenanceenabled"`
	IdentityProviders      []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"identityproviders"`
}

// SiteInfo fetches the account's capabilities.
//
// This is the single source of truth for what a command may do: the function
// list comes from the site, not from a version comparison.
func (c *Client) SiteInfo(ctx context.Context, token string, accountID site.ID) (*site.Capabilities, error) {
	var dto siteInfoDTO
	if err := c.Call(ctx, token, FunctionSiteInfo, nil, &dto); err != nil {
		return nil, err
	}

	capabilities := site.NewCapabilities()
	capabilities.AccountID = accountID
	capabilities.Credential = site.CredentialWSToken
	capabilities.SiteName = dto.SiteName
	capabilities.SiteURL = dto.SiteURL
	capabilities.Username = dto.Username
	capabilities.FullName = fullName(dto)
	capabilities.Release = dto.Release
	capabilities.Version = dto.Version
	capabilities.CanDownload = dto.DownloadFiles == 1
	capabilities.CanUpload = dto.UploadFiles == 1
	if dto.UserID != 0 {
		// Ids are strings everywhere above this layer.
		capabilities.UserID = strconv.FormatInt(dto.UserID, 10)
	}
	for _, function := range dto.Functions {
		capabilities.Functions[function.Name] = site.FunctionInfo{
			Name:    function.Name,
			Version: function.Version,
		}
	}
	return capabilities, nil
}

// PublicConfig fetches the pre-login configuration. It needs no token.
func (c *Client) PublicConfig(ctx context.Context) (*PublicConfig, error) {
	var config PublicConfig
	if err := c.CallNoLogin(ctx, FunctionPublicConfig, map[string]any{}, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func fullName(dto siteInfoDTO) string {
	if dto.FullName != "" {
		return dto.FullName
	}
	switch {
	case dto.FirstName != "" && dto.LastName != "":
		return dto.FirstName + " " + dto.LastName
	case dto.FirstName != "":
		return dto.FirstName
	default:
		return dto.LastName
	}
}

// LoginToken exchanges a username and password for a web service token.
//
// This only works for accounts Moodle itself authenticates; an SSO account
// cannot use it.
func (c *Client) LoginToken(ctx context.Context, username, password, service string) (token, privateToken string, err error) {
	if service == "" {
		service = MobileService
	}
	values, err := Encode(Params{
		"username": username,
		"password": password,
		"service":  service,
	})
	if err != nil {
		return "", "", err
	}
	body, err := c.post(ctx, c.site.Endpoint(PathTokenPHP), values, "login/token.php")
	if err != nil {
		return "", "", err
	}

	// decode already turns token.php's failure shape into an error: it reports
	// an errorcode like any other Moodle response.
	var reply struct {
		Token        string `json:"token"`
		PrivateToken string `json:"privatetoken"`
	}
	if err := decode(body, "login/token.php", &reply); err != nil {
		return "", "", err
	}
	if reply.Token == "" {
		return "", "", errs.New(errs.CodeUpstream, "the site returned no token").
			WithReason(errs.ReasonProtocolDrift)
	}
	return reply.Token, reply.PrivateToken, nil
}
