package mcphost

import "net/http"

// token is the token endpoint: a code or a refresh token goes in, a fresh pair
// comes out.
func (a *authority) token(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the body must be form-encoded")
		return
	}
	if resource := r.PostFormValue("resource"); resource != "" && !a.ownsResource(resource) {
		oauthError(w, http.StatusBadRequest, "invalid_target", "this host serves a different resource")
		return
	}
	clientID := r.PostFormValue("client_id")
	if _, known := a.client(clientID); !known {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "unknown client")
		return
	}

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		a.exchangeCode(w, r, clientID)
	case "refresh_token":
		a.exchangeRefresh(w, r, clientID)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type",
			"use authorization_code or refresh_token")
	}
}

func (a *authority) exchangeCode(w http.ResponseWriter, r *http.Request, clientID string) {
	key := digest(r.PostFormValue("code"))
	a.mu.Lock()
	held, found := a.codes[key]
	// Spent whether or not the rest checks out: a code that was presented once
	// by the wrong party must not be there for a second try.
	delete(a.codes, key)
	a.mu.Unlock()

	if !found || !a.now().Before(held.expires) ||
		held.clientID != clientID ||
		held.redirect != r.PostFormValue("redirect_uri") ||
		!pkceMatches(held.challenge, r.PostFormValue("code_verifier")) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code is wrong, used or expired")
		return
	}
	a.issue(w, clientID)
}

// exchangeRefresh rotates: the token presented stops working the moment it is
// used, and the reply carries its replacement. A client that is still holding
// the old one has been copied, and its next attempt fails.
func (a *authority) exchangeRefresh(w http.ResponseWriter, r *http.Request, clientID string) {
	key := digest(r.PostFormValue("refresh_token"))
	a.mu.Lock()
	held, found := a.refresh[key]
	delete(a.refresh, key)
	a.mu.Unlock()

	if !found || !a.now().Before(held.expires) || held.clientID != clientID {
		// RFC 6749's own code, because it is the one that tells a client to
		// stop refreshing and send the person through sign-in again.
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token is wrong, used or expired")
		return
	}
	a.issue(w, clientID)
}

func (a *authority) issue(w http.ResponseWriter, clientID string) {
	access, refresh := randomToken(), randomToken()
	now := a.now()

	a.mu.Lock()
	a.forgetExpired()
	a.access[digest(access)] = grant{clientID: clientID, expires: now.Add(accessLifetime)}
	a.refresh[digest(refresh)] = grant{clientID: clientID, expires: now.Add(refreshLifetime)}
	a.mu.Unlock()

	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessLifetime.Seconds()),
		"refresh_token": refresh,
	})
}

// forgetExpired drops what can no longer be used, so the maps hold only the
// live sign-ins. It expects mu to be held.
func (a *authority) forgetExpired() {
	now := a.now()
	for key, held := range a.codes {
		if !now.Before(held.expires) {
			delete(a.codes, key)
		}
	}
	for key, held := range a.access {
		if !now.Before(held.expires) {
			delete(a.access, key)
		}
	}
	for key, held := range a.refresh {
		if !now.Before(held.expires) {
			delete(a.refresh, key)
		}
	}
}
