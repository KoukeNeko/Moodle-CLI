package mcphost

import "net/http"

// resourceMetadata is RFC 9728: it tells a client which authorization server
// guards the endpoint. Claude reads it after the first 401, and requires
// `resource` to equal the address the person typed into the connector.
func (a *authority) resourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 a.endpoint,
		"authorization_servers":    []string{a.origin},
		"bearer_methods_supported": []string{"header"},
	})
}

// serverMetadata is RFC 8414. S256 is the only challenge method offered and
// "none" the only client authentication, because every client here is a
// public one that proves itself with PKCE rather than a secret.
func (a *authority) serverMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                a.origin,
		"authorization_endpoint":                a.origin + "/authorize",
		"token_endpoint":                        a.origin + "/token",
		"registration_endpoint":                 a.origin + "/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	})
}
