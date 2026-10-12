package mcphost

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxRedirects  = 5
	maxClientName = 60
)

// registeredClient is what a client id stands for. It is carried inside the id
// itself, signed, so registering stores nothing.
type registeredClient struct {
	Name      string   `json:"n"`
	Redirects []string `json:"r"`
}

// register is RFC 7591 dynamic client registration. Claude uses it to get a
// client id without anyone filling in a form.
//
// Anyone who can reach the host can register. That is harmless on its own: a
// client id grants nothing, and getting a token still needs the owner to type
// the passphrase on a page that names the client and where it will return.
func (a *authority) register(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RedirectURIs            []string `json:"redirect_uris"`
		ClientName              string   `json:"client_name"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the body is not a JSON object")
		return
	}
	if len(request.RedirectURIs) == 0 || len(request.RedirectURIs) > maxRedirects {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri",
			"send between one and five redirect_uris")
		return
	}
	for _, redirect := range request.RedirectURIs {
		if !acceptableRedirect(redirect) {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri",
				"a redirect_uri must be https, or http on localhost, with no fragment")
			return
		}
	}
	if method := request.TokenEndpointAuthMethod; method != "" && method != "none" {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata",
			"only public clients are supported: token_endpoint_auth_method must be none")
		return
	}
	if len(request.GrantTypes) > 0 && !slices.Contains(request.GrantTypes, "authorization_code") {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata",
			"grant_types must include authorization_code")
		return
	}

	name := cleanName(request.ClientName)
	clientID := a.seal.seal("client", registeredClient{Name: name, Redirects: request.RedirectURIs})
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  clientID,
		"client_name":                name,
		"redirect_uris":              request.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

// client reads a client id back, and says whether this process issued it.
func (a *authority) client(clientID string) (registeredClient, bool) {
	var client registeredClient
	return client, a.seal.open("client", clientID, &client)
}

// redirectRegistered says whether a client may send a person back to the
// address it presents. Matching is exact, except that a loopback address may
// use any port: a native client takes a free one each time it starts, so the
// port it registered with is not the one it will listen on later (RFC 8252
// section 7.3, and Claude's own guidance for Claude Code).
func redirectRegistered(registered []string, presented string) bool {
	if slices.Contains(registered, presented) {
		return true
	}
	got, err := url.Parse(presented)
	if err != nil || got.Scheme != "http" || !isLoopback(got.Hostname()) {
		return false
	}
	for _, address := range registered {
		want, err := url.Parse(address)
		if err == nil && want.Scheme == "http" && want.Hostname() == got.Hostname() &&
			want.Path == got.Path && want.RawQuery == got.RawQuery {
			return true
		}
	}
	return false
}

// acceptableRedirect applies the rule the authorization specification gives:
// a redirect is HTTPS, or it is on the machine running the client. Anything
// else would let a code be sent to a plaintext address on the network.
func acceptableRedirect(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Fragment != "" || parsed.User != nil || parsed.Host == "" {
		return false
	}
	switch parsed.Scheme {
	case "https":
		return true
	case "http":
		return isLoopback(parsed.Hostname())
	default:
		return false
	}
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// cleanName makes a client's self-chosen name safe to put on the consent page:
// no control or format characters, and short enough that it cannot push the
// real facts off the screen. Format characters are the ones that reorder or
// hide text — a right-to-left override, a zero-width space — which is how a
// name is made to read as something it is not.
func cleanName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if utf8.RuneCountInString(name) > maxClientName {
		name = string([]rune(name)[:maxClientName])
	}
	if name == "" {
		return "An application"
	}
	return name
}
