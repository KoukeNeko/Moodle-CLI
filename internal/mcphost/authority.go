package mcphost

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	endpointPath = "/mcp"

	accessLifetime  = time.Hour
	refreshLifetime = 30 * 24 * time.Hour
	codeLifetime    = 5 * time.Minute
	// requestLifetime is how long the consent page stays valid once shown.
	requestLifetime = 10 * time.Minute

	// A wrong passphrase is the one guess an outsider can make, so guesses are
	// counted across the whole process. Behind a tunnel every caller arrives
	// from the same address, so counting per address would count nothing.
	maxFailures   = 5
	failureWindow = 10 * time.Minute
)

// authority is the sign-in side of the host: it decides who may call the MCP
// endpoint, and hands out the tokens that say so.
//
// It is the authorization server and the resource server in one process. That
// is what lets a person point a Claude connector at one URL with nothing else
// to set up — and it is why there is exactly one human it ever admits, the
// owner who knows the passphrase.
type authority struct {
	origin     string
	endpoint   string
	seal       sealer
	passphrase [sha256.Size]byte

	siteName   string
	allowWrite bool

	now func() time.Time

	// mu guards everything below. Each of these is created only by an owner
	// who has just proved the passphrase, so none of them grows on an
	// outsider's say-so.
	mu       sync.Mutex
	codes    map[string]authCode
	access   map[string]grant
	refresh  map[string]grant
	failures []time.Time
}

type authCode struct {
	clientID  string
	redirect  string
	challenge string
	expires   time.Time
}

type grant struct {
	clientID string
	expires  time.Time
}

func newAuthority(origin, passphrase, site string, allowWrite bool) *authority {
	return &authority{
		origin:     origin,
		endpoint:   origin + endpointPath,
		seal:       newSealer(),
		passphrase: sha256.Sum256([]byte(passphrase)),
		siteName:   site,
		allowWrite: allowWrite,
		now:        time.Now,
		codes:      map[string]authCode{},
		access:     map[string]grant{},
		refresh:    map[string]grant{},
	}
}

func (a *authority) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", a.resourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource"+endpointPath, a.resourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", a.serverMetadata)
	mux.HandleFunc("POST /register", a.register)
	mux.HandleFunc("GET /authorize", a.showConsent)
	mux.HandleFunc("POST /authorize", a.decide)
	mux.HandleFunc("POST /token", a.token)
}

// protect admits a request to the MCP endpoint only with a live access token.
func (a *authority) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A browser sends Origin on a cross-site request; the clients this
		// host serves are other servers and never do. Refusing any other
		// origin is what stops a web page from reaching a host listening on
		// this machine's loopback.
		if origin := r.Header.Get("Origin"); origin != "" && origin != a.origin {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		token, presented := bearerToken(r)
		if !presented || !a.validAccess(token) {
			// Claude starts sign-in from this 401 and the pointer in it; a
			// 200 carrying the same header would be ignored.
			challenge := `Bearer resource_metadata="` + a.origin +
				`/.well-known/oauth-protected-resource` + endpointPath + `"`
			if presented {
				challenge += `, error="invalid_token"`
			}
			w.Header().Set("WWW-Authenticate", challenge)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func (a *authority) validAccess(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	held, ok := a.access[digest(token)]
	return ok && a.now().Before(held.expires)
}

// ownsResource reports whether a client's `resource` names this server. The
// bare origin is accepted too: a client that was given only the host has
// still named this server, and refusing it would fail a sign-in over a
// trailing path.
func (a *authority) ownsResource(resource string) bool {
	trimmed := strings.TrimSuffix(resource, "/")
	return trimmed == a.origin || trimmed == a.endpoint
}

// passphraseMatches compares digests rather than the text, so the comparison
// takes the same time wherever the guess first differs.
func (a *authority) passphraseMatches(guess string) bool {
	got := sha256.Sum256([]byte(strings.TrimSpace(guess)))
	return subtle.ConstantTimeCompare(got[:], a.passphrase[:]) == 1
}

// lockedOut reports whether guessing has gone on too long to allow another try.
func (a *authority) lockedOut() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.forgetOldFailures()
	return len(a.failures) >= maxFailures
}

func (a *authority) recordFailure() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failures = append(a.failures, a.now())
}

// forgetOldFailures expects mu to be held.
func (a *authority) forgetOldFailures() {
	cutoff := a.now().Add(-failureWindow)
	kept := a.failures[:0]
	for _, at := range a.failures {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	a.failures = kept
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// oauthError answers in the shape RFC 6749 gives, which is what a client
// parses to decide whether to retry, refresh or start over.
func oauthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}
