package mcphost

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

const maxState = 512

// authRequest is a sign-in the consent page has been shown for. It rides in the
// page's hidden field, signed, so showing the page stores nothing and the form
// cannot be edited into asking for something else.
type authRequest struct {
	ClientID  string `json:"c"`
	Redirect  string `json:"r"`
	Challenge string `json:"p"`
	State     string `json:"s"`
	Expires   int64  `json:"e"`
}

// showConsent is the authorization endpoint: the client has sent a person
// here, and the person is the owner or nobody.
func (a *authority) showConsent(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	client, known := a.client(query.Get("client_id"))
	redirect := query.Get("redirect_uri")
	if !known || !redirectRegistered(client.Redirects, redirect) {
		// Nothing is sent to an address that was not registered: an error
		// delivered there would tell an attacker's page what happened.
		render(w, http.StatusBadRequest, page{
			Title: "Cannot sign in",
			Message: "This link is not valid. If the host was restarted, remove the " +
				"connector in Claude and add it again.",
		})
		return
	}

	state := query.Get("state")
	reject := func(code string) {
		sendBack(w, r, http.StatusFound, redirect, state, map[string]string{"error": code})
	}
	switch {
	case query.Get("response_type") != "code":
		reject("unsupported_response_type")
	case query.Get("code_challenge_method") != "S256" || !validChallenge(query.Get("code_challenge")):
		// Without PKCE a code taken off the wire could be redeemed by anyone.
		reject("invalid_request")
	case len(state) > maxState:
		reject("invalid_request")
	case query.Get("resource") != "" && !a.ownsResource(query.Get("resource")):
		reject("invalid_target")
	default:
		sealed := a.seal.seal("authorization", authRequest{
			ClientID:  query.Get("client_id"),
			Redirect:  redirect,
			Challenge: query.Get("code_challenge"),
			State:     state,
			Expires:   a.now().Add(requestLifetime).Unix(),
		})
		render(w, http.StatusOK, a.consentPage(client, redirect, sealed, ""))
	}
}

// decide takes the owner's answer from the consent page.
func (a *authority) decide(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		render(w, http.StatusBadRequest, page{Title: "Cannot sign in", Message: "The form could not be read."})
		return
	}

	sealed := r.PostFormValue("request")
	var request authRequest
	client, known := registeredClient{}, false
	if a.seal.open("authorization", sealed, &request) {
		client, known = a.client(request.ClientID)
	}
	if !known || a.now().Unix() > request.Expires {
		render(w, http.StatusBadRequest, page{
			Title:   "Cannot sign in",
			Message: "This request has expired. Start again from Claude.",
		})
		return
	}

	if r.PostFormValue("decision") != "allow" {
		sendBack(w, r, http.StatusSeeOther, request.Redirect, request.State,
			map[string]string{"error": "access_denied"})
		return
	}
	switch a.tryPassphrase(r.PostFormValue("passphrase")) {
	case attemptLocked:
		render(w, http.StatusTooManyRequests, page{
			Title:   "Too many attempts",
			Message: "Wrong passphrases were entered too often. Try again in a few minutes.",
		})
		return
	case attemptWrong:
		render(w, http.StatusForbidden,
			a.consentPage(client, request.Redirect, sealed, "Wrong passphrase."))
		return
	}

	code := randomToken()
	a.mu.Lock()
	a.forgetExpired()
	a.codes[digest(code)] = authCode{
		clientID:  request.ClientID,
		redirect:  request.Redirect,
		challenge: request.Challenge,
		expires:   a.now().Add(codeLifetime),
	}
	a.mu.Unlock()
	sendBack(w, r, http.StatusSeeOther, request.Redirect, request.State,
		map[string]string{"code": code})
}

// consentPage says what is being handed over, to a visitor nobody has
// vouched for. The page is reachable by anyone who knows the address, so it
// names the site and the access but not the account: who the host belongs to
// is for the owner, who already knows, and not for whoever is probing.
func (a *authority) consentPage(client registeredClient, redirect, sealed, message string) page {
	returnsTo := redirect
	if parsed, err := url.Parse(redirect); err == nil {
		returnsTo = displayHost(parsed)
	}
	return page{
		Title:     "Allow this application to use Moodle?",
		Consent:   true,
		Client:    client.Name,
		Site:      a.siteName,
		Write:     a.allowWrite,
		ReturnsTo: returnsTo,
		Request:   sealed,
		Message:   message,
	}
}

// displayHost spells a redirect's host the way a browser resolves it. A
// look-alike letter from another alphabet turns into xn-- text, which is the
// one signal on the page that cannot be made to read as the real name.
func displayHost(address *url.URL) string {
	host := address.Hostname()
	if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		host = ascii
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := address.Port(); port != "" {
		host += ":" + port
	}
	return host
}

// sendBack returns the person to the client with the outcome in the query.
func sendBack(w http.ResponseWriter, r *http.Request, status int, redirect, state string, params map[string]string) {
	target, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, "bad redirect", http.StatusInternalServerError)
		return
	}
	query := target.Query()
	for name, value := range params {
		query.Set(name, value)
	}
	if state != "" {
		query.Set("state", state)
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), status)
}

// validChallenge accepts what S256 produces: a SHA-256 digest, unpadded.
func validChallenge(challenge string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(raw) == sha256.Size
}

// pkceMatches checks the verifier against the challenge sent at the start.
func pkceMatches(challenge, verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}
