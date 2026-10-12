package mcphost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

const (
	claudeCallback = "https://claude.ai/api/mcp/auth_callback"
	passphrase     = "correct horse battery"
	verifier       = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
)

// testHost is a host listening on a real socket, with a stub in place of the
// protocol so what is under test is who gets through, not what they say.
type testHost struct {
	t       *testing.T
	server  *httptest.Server
	host    *Host
	reached int
	log     lockedBuffer
	client  *http.Client
}

func newTestHost(t *testing.T, mutate ...func(*Config)) *testHost {
	t.Helper()
	return newTestHostWith(t, nil, mutate...)
}

// newTestHostWith serves a protocol handler of the test's own, for the cases
// where what the handler does is the point.
func newTestHostWith(t *testing.T, protocol http.Handler, mutate ...func(*Config)) *testHost {
	t.Helper()
	h := &testHost{t: t}
	h.server = httptest.NewUnstartedServer(nil)
	config := Config{
		PublicURL:   "http://" + h.server.Listener.Addr().String(),
		Passphrase:  passphrase,
		SiteName:    "school",
		AccountName: "student1",
		Log:         &h.log,
	}
	for _, change := range mutate {
		change(&config)
	}
	if protocol == nil {
		protocol = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			h.reached++
			_, _ = io.WriteString(w, "reached")
		})
	}
	host, err := New(config, protocol)
	if err != nil {
		t.Fatal(err)
	}
	h.host = host
	h.server.Config.Handler = host.Handler()
	h.server.Start()
	t.Cleanup(h.server.Close)
	h.client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return h
}

func (h *testHost) url(path string) string { return h.server.URL + path }

func (h *testHost) do(request *http.Request) (*http.Response, string) {
	h.t.Helper()
	response, err := h.client.Do(request)
	if err != nil {
		h.t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response, string(body)
}

func (h *testHost) get(path string) (*http.Response, string) {
	h.t.Helper()
	request, _ := http.NewRequest(http.MethodGet, h.url(path), nil)
	return h.do(request)
}

func (h *testHost) postForm(path string, form url.Values) (*http.Response, string) {
	h.t.Helper()
	request, _ := http.NewRequest(http.MethodPost, h.url(path), strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return h.do(request)
}

func (h *testHost) callMCP(token string, extra ...string) (*http.Response, string) {
	h.t.Helper()
	request, _ := http.NewRequest(http.MethodPost, h.url("/mcp"),
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		request.Header.Set(extra[i], extra[i+1])
	}
	return h.do(request)
}

func (h *testHost) register(redirect string) string {
	h.t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"redirect_uris": []string{redirect}, "client_name": "Claude",
		"token_endpoint_auth_method": "none",
	})
	request, _ := http.NewRequest(http.MethodPost, h.url("/register"), bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response, body := h.do(request)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("register: %d %s", response.StatusCode, body)
	}
	var registered struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal([]byte(body), &registered); err != nil {
		h.t.Fatal(err)
	}
	return registered.ClientID
}

func challenge() string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (h *testHost) authorizeQuery(clientID string, override url.Values) string {
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {claudeCallback},
		"code_challenge":        {challenge()},
		"code_challenge_method": {"S256"},
		"state":                 {"abc"},
		"resource":              {h.server.URL + "/mcp"},
	}
	for name, value := range override {
		query[name] = value
	}
	return "/authorize?" + query.Encode()
}

var hiddenRequest = regexp.MustCompile(`name="request" value="([^"]+)"`)

// consent opens the consent page and returns the signed request it carries.
func (h *testHost) consent(clientID string) string {
	h.t.Helper()
	response, body := h.get(h.authorizeQuery(clientID, nil))
	match := hiddenRequest.FindStringSubmatch(body)
	if response.StatusCode != http.StatusOK || match == nil {
		h.t.Fatalf("consent page: %d %s", response.StatusCode, body)
	}
	return match[1]
}

// approve answers the consent page and returns the code from the redirect.
func (h *testHost) approve(request, answer string) string {
	h.t.Helper()
	response, body := h.postForm("/authorize", url.Values{
		"request": {request}, "passphrase": {answer}, "decision": {"allow"},
	})
	if response.StatusCode != http.StatusSeeOther {
		h.t.Fatalf("approve: %d %s", response.StatusCode, body)
	}
	location, _ := url.Parse(response.Header.Get("Location"))
	if location.Query().Get("state") != "abc" {
		h.t.Errorf("state was not returned: %s", location)
	}
	return location.Query().Get("code")
}

func (h *testHost) tokenRequest(form url.Values) (*http.Response, map[string]any) {
	h.t.Helper()
	response, body := h.postForm("/token", form)
	var parsed map[string]any
	_ = json.Unmarshal([]byte(body), &parsed)
	return response, parsed
}

func (h *testHost) exchange(clientID, code string) (*http.Response, map[string]any) {
	return h.tokenRequest(url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {claudeCallback}, "code_verifier": {verifier},
		"resource": {h.server.URL + "/mcp"},
	})
}

// signIn runs the whole flow and returns the client id and the tokens.
func (h *testHost) signIn() (string, map[string]any) {
	h.t.Helper()
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)
	response, tokens := h.exchange(clientID, code)
	if response.StatusCode != http.StatusOK {
		h.t.Fatalf("exchange: %d %v", response.StatusCode, tokens)
	}
	return clientID, tokens
}

func TestWithoutATokenTheEndpointAnswers401AndSaysWhereToSignIn(t *testing.T) {
	h := newTestHost(t)

	response, _ := h.callMCP("")

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
	challenge := response.Header.Get("WWW-Authenticate")
	want := `resource_metadata="` + h.server.URL + `/.well-known/oauth-protected-resource/mcp"`
	if !strings.Contains(challenge, want) {
		t.Errorf("WWW-Authenticate = %q, want it to contain %s", challenge, want)
	}
	if strings.Contains(challenge, "invalid_token") {
		t.Error("a request with no token was told its token was invalid")
	}
	if h.reached != 0 {
		t.Error("the protocol handler ran for a caller with no token")
	}
}

func TestADiscoveryDocumentNamesTheResourceExactly(t *testing.T) {
	h := newTestHost(t)

	for _, path := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
	} {
		_, body := h.get(path)
		var metadata map[string]any
		if err := json.Unmarshal([]byte(body), &metadata); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		// Claude requires this to equal the address typed into the connector.
		if metadata["resource"] != h.server.URL+"/mcp" {
			t.Errorf("%s: resource = %v", path, metadata["resource"])
		}
		if servers, _ := metadata["authorization_servers"].([]any); len(servers) != 1 || servers[0] != h.server.URL {
			t.Errorf("%s: authorization_servers = %v", path, metadata["authorization_servers"])
		}
	}

	_, body := h.get("/.well-known/oauth-authorization-server")
	var server map[string]any
	_ = json.Unmarshal([]byte(body), &server)
	if server["issuer"] != h.server.URL || server["registration_endpoint"] != h.server.URL+"/register" {
		t.Errorf("authorization server metadata = %v", server)
	}
	if methods, _ := server["code_challenge_methods_supported"].([]any); len(methods) != 1 || methods[0] != "S256" {
		t.Errorf("code_challenge_methods_supported = %v, want only S256", methods)
	}
}

func TestTheWholeSignInLeadsToAWorkingToken(t *testing.T) {
	h := newTestHost(t)

	_, tokens := h.signIn()
	access, _ := tokens["access_token"].(string)
	response, body := h.callMCP(access)

	if response.StatusCode != http.StatusOK || body != "reached" {
		t.Fatalf("got %d %q, want the protocol handler", response.StatusCode, body)
	}
	if tokens["token_type"] != "Bearer" || tokens["refresh_token"] == "" {
		t.Errorf("token response = %v", tokens)
	}
}

func TestTheConsentPageSaysWhatIsBeingHandedOver(t *testing.T) {
	h := newTestHost(t, func(c *Config) { c.AllowWrite = true })
	clientID := h.register(claudeCallback)

	_, body := h.get(h.authorizeQuery(clientID, nil))

	for _, want := range []string{
		"Allow this application to use Moodle?", "Claude", "school",
		"Read and write", "claude.ai", "some changes cannot be undone",
		"chose its own name",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the consent page is missing %q", want)
		}
	}
	// The page is open to anyone who knows the address. The account is the
	// owner's to know, so it is not on it.
	if strings.Contains(body, "student1") {
		t.Error("the consent page names the account to a visitor nobody has vouched for")
	}
}

func TestAReadOnlyHostSaysSoOnTheConsentPage(t *testing.T) {
	h := newTestHost(t)

	_, body := h.get(h.authorizeQuery(h.register(claudeCallback), nil))

	if !strings.Contains(body, "Read only") || strings.Contains(body, "Read and write") {
		t.Error("the page does not say the host is read-only")
	}
}

func TestAWrongPassphraseIsRefusedAndNoCodeIsIssued(t *testing.T) {
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))

	response, body := h.postForm("/authorize", url.Values{
		"request": {request}, "passphrase": {"guess"}, "decision": {"allow"},
	})

	if response.StatusCode != http.StatusForbidden || !strings.Contains(body, "Wrong passphrase.") {
		t.Errorf("got %d %s", response.StatusCode, body)
	}
	if response.Header.Get("Location") != "" {
		t.Error("a wrong passphrase was redirected")
	}
}

func TestGuessingIsStoppedEvenForTheRightPassphrase(t *testing.T) {
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))
	for range maxFailures {
		h.postForm("/authorize", url.Values{"request": {request}, "passphrase": {"guess"}, "decision": {"allow"}})
	}

	response, _ := h.postForm("/authorize", url.Values{
		"request": {request}, "passphrase": {passphrase}, "decision": {"allow"},
	})

	if response.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429: the limit has to hold against the right answer too", response.StatusCode)
	}
}

func TestTheLockoutEndsAfterTheWindow(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	request := h.consent(clientID)
	for range maxFailures {
		h.postForm("/authorize", url.Values{"request": {request}, "passphrase": {"guess"}, "decision": {"allow"}})
	}
	h.host.authority.now = func() time.Time { return time.Now().Add(failureWindow + time.Minute) }
	// The page the owner was looking at has expired by then too.
	request = h.consent(clientID)

	response, _ := h.postForm("/authorize", url.Values{
		"request": {request}, "passphrase": {passphrase}, "decision": {"allow"},
	})

	if response.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want the owner let back in", response.StatusCode)
	}
}

func TestDenyingSendsAnErrorBackToTheClient(t *testing.T) {
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))

	response, _ := h.postForm("/authorize", url.Values{"request": {request}, "decision": {"deny"}})

	location, _ := url.Parse(response.Header.Get("Location"))
	if location.Query().Get("error") != "access_denied" || location.Query().Get("code") != "" {
		t.Errorf("Location = %s", location)
	}
}

func TestACodeWorksOnce(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)

	first, _ := h.exchange(clientID, code)
	second, body := h.exchange(clientID, code)

	if first.StatusCode != http.StatusOK {
		t.Fatalf("first exchange: %d", first.StatusCode)
	}
	if second.StatusCode != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("second exchange: %d %v", second.StatusCode, body)
	}
}

func TestACodeIsUselessWithoutTheVerifierThatStartedIt(t *testing.T) {
	// A code taken off the wire is what PKCE exists to make worthless.
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)

	response, body := h.tokenRequest(url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {claudeCallback}, "code_verifier": {strings.Repeat("x", 43)},
	})

	if response.StatusCode != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("got %d %v", response.StatusCode, body)
	}
}

func TestACodeCannotBeRedeemedForAnotherRedirect(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)

	response, _ := h.tokenRequest(url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {"https://claude.ai/elsewhere"}, "code_verifier": {verifier},
	})

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestACodeCannotBeRedeemedByAnotherClient(t *testing.T) {
	h := newTestHost(t)
	code := h.approve(h.consent(h.register(claudeCallback)), passphrase)
	// A client id is signed from what was registered, so registering the same
	// thing again gives the same client. Another client has to differ.
	other := h.register("http://localhost:3118/callback")

	response, _ := h.exchange(other, code)

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestARefreshTokenRotates(t *testing.T) {
	h := newTestHost(t)
	clientID, tokens := h.signIn()
	refresh, _ := tokens["refresh_token"].(string)

	response, renewed := h.tokenRequest(url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID},
	})
	reused, body := h.tokenRequest(url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID},
	})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("refresh: %d %v", response.StatusCode, renewed)
	}
	if renewed["refresh_token"] == refresh {
		t.Error("the refresh token was not replaced")
	}
	if reused.StatusCode != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("the spent refresh token was accepted again: %d %v", reused.StatusCode, body)
	}
	access, _ := renewed["access_token"].(string)
	if got, _ := h.callMCP(access); got.StatusCode != http.StatusOK {
		t.Errorf("the renewed access token was refused: %d", got.StatusCode)
	}
}

func TestARefreshTokenBelongsToItsClient(t *testing.T) {
	h := newTestHost(t)
	_, tokens := h.signIn()
	refresh, _ := tokens["refresh_token"].(string)

	response, _ := h.tokenRequest(url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refresh},
		"client_id": {h.register("http://localhost:3118/callback")},
	})

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestAnExpiredAccessTokenIsTold(t *testing.T) {
	h := newTestHost(t)
	_, tokens := h.signIn()
	access, _ := tokens["access_token"].(string)
	h.host.authority.now = func() time.Time { return time.Now().Add(accessLifetime + time.Minute) }

	response, _ := h.callMCP(access)

	if response.StatusCode != http.StatusUnauthorized ||
		!strings.Contains(response.Header.Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Errorf("got %d %q", response.StatusCode, response.Header.Get("WWW-Authenticate"))
	}
}

func TestAMadeUpTokenIsRefused(t *testing.T) {
	h := newTestHost(t)

	response, _ := h.callMCP("not-a-token")

	if response.StatusCode != http.StatusUnauthorized || h.reached != 0 {
		t.Errorf("status = %d, reached = %d", response.StatusCode, h.reached)
	}
}

func TestATokenInTheQueryStringIsNotAToken(t *testing.T) {
	h := newTestHost(t)
	_, tokens := h.signIn()
	access, _ := tokens["access_token"].(string)

	request, _ := http.NewRequest(http.MethodPost, h.url("/mcp?access_token="+access), strings.NewReader("{}"))
	response, _ := h.do(request)

	if response.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d: the specification forbids tokens in the URL", response.StatusCode)
	}
}

func TestAForeignOriginIsRefusedEvenWithAToken(t *testing.T) {
	// A web page reaching a host on loopback is the case this exists for.
	h := newTestHost(t)
	_, tokens := h.signIn()
	access, _ := tokens["access_token"].(string)

	response, _ := h.callMCP(access, "Origin", "https://evil.example")

	if response.StatusCode != http.StatusForbidden || h.reached != 0 {
		t.Errorf("status = %d, reached = %d", response.StatusCode, h.reached)
	}
}

func TestARegisteredRedirectIsRequiredExactly(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)

	response, body := h.get(h.authorizeQuery(clientID, url.Values{"redirect_uri": {"https://evil.example/cb"}}))

	if response.StatusCode != http.StatusBadRequest || response.Header.Get("Location") != "" {
		t.Errorf("got %d, Location %q: an unregistered address must get a page, not a redirect",
			response.StatusCode, response.Header.Get("Location"))
	}
	if !strings.Contains(body, "not valid") {
		t.Errorf("body = %s", body)
	}
}

func TestAClientIdThisProcessDidNotIssueIsRefused(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	// The first character always changes the payload. The last ones cannot be
	// relied on: a base64 signature ends in bits the decoder ignores, so
	// altering them can leave the id meaning exactly what it did.
	forged := "x" + clientID[1:]

	response, _ := h.get(h.authorizeQuery(forged, nil))

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestAClientIdFromAnotherRunIsRefused(t *testing.T) {
	// A restart changes the signing key, and with it everything issued before.
	first := newTestHost(t)
	clientID := first.register(claudeCallback)
	second := newTestHost(t)

	response, _ := second.get(second.authorizeQuery(clientID, nil))

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestATamperedConsentRequestIsRefused(t *testing.T) {
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))
	body, signature, _ := strings.Cut(request, ".")
	tampered := body + "A." + signature

	response, _ := h.postForm("/authorize", url.Values{
		"request": {tampered}, "passphrase": {passphrase}, "decision": {"allow"},
	})

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestAnExpiredConsentRequestIsRefused(t *testing.T) {
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))
	h.host.authority.now = func() time.Time { return time.Now().Add(requestLifetime + time.Minute) }

	response, _ := h.postForm("/authorize", url.Values{
		"request": {request}, "passphrase": {passphrase}, "decision": {"allow"},
	})

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestAuthorizationWithoutS256IsSentBackAsAnError(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)

	for name, override := range map[string]url.Values{
		"plain method": {"code_challenge_method": {"plain"}},
		"no challenge": {"code_challenge": {""}},
		"no method":    {"code_challenge_method": {""}},
	} {
		t.Run(name, func(t *testing.T) {
			response, _ := h.get(h.authorizeQuery(clientID, override))
			location, _ := url.Parse(response.Header.Get("Location"))
			if location.Query().Get("error") != "invalid_request" {
				t.Errorf("Location = %s", location)
			}
		})
	}
}

func TestAnotherResourceIsRefused(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)

	response, _ := h.get(h.authorizeQuery(clientID, url.Values{"resource": {"https://other.example/mcp"}}))

	location, _ := url.Parse(response.Header.Get("Location"))
	if location.Query().Get("error") != "invalid_target" {
		t.Errorf("Location = %s", location)
	}
}

func TestRegistrationRefusesWhereACodeCouldLeak(t *testing.T) {
	h := newTestHost(t)

	for redirect, accepted := range map[string]bool{
		"https://claude.ai/api/mcp/auth_callback": true,
		"http://localhost:3118/callback":          true,
		"http://127.0.0.1:3118/callback":          true,
		"http://evil.example/callback":            false,
		"https://claude.ai/cb#fragment":           false,
		"claude://callback":                       false,
		"not a url":                               false,
	} {
		payload, _ := json.Marshal(map[string]any{"redirect_uris": []string{redirect}})
		request, _ := http.NewRequest(http.MethodPost, h.url("/register"), bytes.NewReader(payload))
		response, _ := h.do(request)
		if got := response.StatusCode == http.StatusCreated; got != accepted {
			t.Errorf("%s: accepted = %v, want %v", redirect, got, accepted)
		}
	}
}

func TestRegistrationOnlyAdmitsPublicClients(t *testing.T) {
	h := newTestHost(t)
	payload, _ := json.Marshal(map[string]any{
		"redirect_uris": []string{claudeCallback}, "token_endpoint_auth_method": "client_secret_basic",
	})

	request, _ := http.NewRequest(http.MethodPost, h.url("/register"), bytes.NewReader(payload))
	response, _ := h.do(request)

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestAClientNameCannotBreakTheConsentPage(t *testing.T) {
	h := newTestHost(t)
	payload, _ := json.Marshal(map[string]any{
		"redirect_uris": []string{claudeCallback},
		"client_name":   "<script>alert(1)</script>\nClaude",
	})
	request, _ := http.NewRequest(http.MethodPost, h.url("/register"), bytes.NewReader(payload))
	_, body := h.do(request)
	var registered struct {
		ClientID string `json:"client_id"`
	}
	_ = json.Unmarshal([]byte(body), &registered)

	_, page := h.get(h.authorizeQuery(registered.ClientID, nil))

	if strings.Contains(page, "<script>") {
		t.Error("a client's name reached the page unescaped")
	}
}

func TestOnlyTheOwnersOriginIsAccepted(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://moodle.example.com":      true,
		"https://moodle.example.com/":     true,
		"https://moodle.example.com:8443": true,
		"http://localhost:8787":           true,
		"http://127.0.0.1:8787":           true,
		"http://moodle.example.com":       false,
		"https://moodle.example.com/mcp":  false,
		"https://moodle.example.com/?x=1": false,
		"moodle.example.com":              false,
		"":                                false,
	} {
		_, err := New(Config{PublicURL: raw}, nil)
		if (err == nil) != ok {
			t.Errorf("%q: accepted = %v, want %v (%v)", raw, err == nil, ok, err)
		}
	}
}

func TestTheOriginIsLowercasedToMatchWhatClaudeSends(t *testing.T) {
	host, err := New(Config{PublicURL: "https://Moodle.Example.COM", Passphrase: passphrase}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if host.authority.endpoint != "https://moodle.example.com/mcp" {
		t.Errorf("endpoint = %s", host.authority.endpoint)
	}
}

// lockedBuffer is a log the server writes while the test reads it.
type lockedBuffer struct {
	mu   sync.Mutex
	text bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.String()
}

func TestServeAnswersUntilTheContextEnds(t *testing.T) {
	host, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: passphrase, Log: &lockedBuffer{}},
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if err != nil {
		t.Fatal(err)
	}
	address, stop, done := serving(t, host)

	response, err := http.Get("http://" + address + "/.well-known/oauth-authorization-server")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("not serving: %v %v", err, response)
	}
	response.Body.Close()

	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v after the context ended", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after the context ended")
	}
}

func TestListenSaysWhenTheAddressIsTaken(t *testing.T) {
	first := httptest.NewServer(http.NotFoundHandler())
	defer first.Close()

	_, err := Listen(first.Listener.Addr().String())

	if err == nil || !strings.Contains(err.Error(), "cannot listen on") {
		t.Errorf("err = %v, want it to say the address could not be used", err)
	}
}

func TestWithNoPassphraseOneIsGeneratedAndPrinted(t *testing.T) {
	var log bytes.Buffer
	host, err := New(Config{PublicURL: "http://localhost:8787", Log: &log}, nil)
	if err != nil {
		t.Fatal(err)
	}

	host.Announce(&fakeAddr{})

	generated := host.config.Passphrase
	if len(strings.ReplaceAll(generated, "-", "")) != 24 {
		t.Errorf("passphrase %q is not 24 characters in groups", generated)
	}
	if !strings.Contains(log.String(), generated) {
		t.Error("a generated passphrase was not printed, so nobody can use it")
	}
	if !host.authority.passphraseMatches(generated) {
		t.Error("the generated passphrase does not unlock the host")
	}
}

func TestAShortPassphraseIsRefused(t *testing.T) {
	if _, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: "short"}, nil); err == nil {
		t.Error("a five-character passphrase was accepted")
	}
	if _, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: strings.Repeat("a", minPassphrase)}, nil); err != nil {
		t.Errorf("a passphrase at the minimum was refused: %v", err)
	}
}

func TestAPassphraseYouChoseIsNeverPrinted(t *testing.T) {
	var log bytes.Buffer
	host, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: passphrase, Log: &log}, nil)
	if err != nil {
		t.Fatal(err)
	}

	host.Announce(&fakeAddr{})

	if strings.Contains(log.String(), passphrase) {
		t.Error("the passphrase was written to the log")
	}
}

func TestTheRequestLogLeavesOutSecrets(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)
	_, tokens := h.exchange(clientID, code)
	access, _ := tokens["access_token"].(string)
	h.callMCP(access)

	// The line is written after the response is sent, so wait for it.
	logged := h.logContaining("POST /mcp 200")
	for _, secret := range []string{code, access, tokens["refresh_token"].(string), passphrase, challenge()} {
		if strings.Contains(logged, secret) {
			t.Errorf("the log holds a secret: %s", secret)
		}
	}
	if !strings.Contains(logged, "POST /mcp 200") {
		t.Errorf("the log does not record the request: %s", logged)
	}
}

type fakeAddr struct{}

func (*fakeAddr) Network() string { return "tcp" }
func (*fakeAddr) String() string  { return "127.0.0.1:8787" }

// logContaining waits for the log to hold a line, because a request is logged
// after its response has gone out.
func (h *testHost) logContaining(want string) string {
	h.t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if text := h.log.String(); strings.Contains(text, want) {
			return text
		}
	}
	return h.log.String()
}

func TestAConsentRequestEditedUnderItsSignatureIsRefused(t *testing.T) {
	// The page carries what is being approved in a hidden field. Editing it to
	// valid JSON and keeping the old signature has to fail on the signature
	// itself, not on being malformed.
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))
	body, signature, _ := strings.Cut(request, ".")
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["r"] = "https://evil.example/cb"
	edited, _ := json.Marshal(fields)

	response, _ := h.postForm("/authorize", url.Values{
		"request":    {base64.RawURLEncoding.EncodeToString(edited) + "." + signature},
		"passphrase": {passphrase}, "decision": {"allow"},
	})

	if response.StatusCode != http.StatusBadRequest || response.Header.Get("Location") != "" {
		t.Errorf("got %d, Location %q; want a refusal that sends nobody anywhere",
			response.StatusCode, response.Header.Get("Location"))
	}
}

func TestAnExpiredCodeIsRefused(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)
	h.host.authority.now = func() time.Time { return time.Now().Add(codeLifetime + time.Minute) }

	response, body := h.exchange(clientID, code)

	if response.StatusCode != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("got %d %v, want invalid_grant", response.StatusCode, body)
	}
}

func TestAnExpiredRefreshTokenIsRefused(t *testing.T) {
	h := newTestHost(t)
	clientID, tokens := h.signIn()
	refresh, _ := tokens["refresh_token"].(string)
	h.host.authority.now = func() time.Time { return time.Now().Add(refreshLifetime + time.Hour) }

	response, body := h.tokenRequest(url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID},
	})

	if response.StatusCode != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("got %d %v, want invalid_grant", response.StatusCode, body)
	}
}

func TestALoopbackClientMayComeBackOnAnotherPort(t *testing.T) {
	// A native client takes a free port every time it starts. Matching the
	// registered one byte for byte would refuse it on its second sign-in.
	h := newTestHost(t)
	clientID := h.register("http://localhost:3118/callback")

	for _, c := range []struct {
		redirect string
		allowed  bool
	}{
		{"http://localhost:3118/callback", true},
		{"http://localhost:4555/callback", true},
		{"http://localhost:4555/elsewhere", false},
		{"http://127.0.0.1:3118/callback", false},
		{"https://localhost:3118/callback", false},
		{"http://localhost:3118/callback?x=1", false},
	} {
		response, _ := h.get(h.authorizeQuery(clientID, url.Values{"redirect_uri": {c.redirect}}))
		if got := response.StatusCode == http.StatusOK; got != c.allowed {
			t.Errorf("%s: shown the consent page = %v, want %v", c.redirect, got, c.allowed)
		}
	}
}

func TestACodeGoesBackToThePortTheClientAskedFrom(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register("http://localhost:3118/callback")
	asked := "http://localhost:4555/callback"
	_, page := h.get(h.authorizeQuery(clientID, url.Values{"redirect_uri": {asked}}))
	request := hiddenRequest.FindStringSubmatch(page)[1]

	response, _ := h.postForm("/authorize", url.Values{
		"request": {request}, "passphrase": {passphrase}, "decision": {"allow"},
	})
	location, _ := url.Parse(response.Header.Get("Location"))
	exchanged, tokens := h.tokenRequest(url.Values{
		"grant_type": {"authorization_code"}, "code": {location.Query().Get("code")},
		"client_id": {clientID}, "redirect_uri": {asked}, "code_verifier": {verifier},
	})

	if location.Host != "localhost:4555" {
		t.Errorf("the code went to %s, not where the client is listening", location.Host)
	}
	if exchanged.StatusCode != http.StatusOK || tokens["access_token"] == nil {
		t.Errorf("exchange: %d %v", exchanged.StatusCode, tokens)
	}
}

func TestALookAlikeHostIsShownAsPunycode(t *testing.T) {
	// The "Returns to" row is the one thing on the page the client does not
	// get to word. A Cyrillic "а" in claude.ai must not read as the real one.
	h := newTestHost(t)
	clientID := h.register("https://cl\u0430ude.ai/api/mcp/auth_callback")

	_, page := h.get(h.authorizeQuery(clientID, url.Values{
		"redirect_uri": {"https://cl\u0430ude.ai/api/mcp/auth_callback"},
	}))

	if !strings.Contains(page, "xn--") || strings.Contains(page, "cl\u0430ude.ai") {
		t.Errorf("the look-alike host was not spelled out:\n%s", page)
	}
}

func TestAClientNameCannotHideOrReorderText(t *testing.T) {
	h := newTestHost(t)
	payload, _ := json.Marshal(map[string]any{
		"redirect_uris": []string{claudeCallback},
		"client_name":   "Cl\u202eaude\u200b",
	})
	request, _ := http.NewRequest(http.MethodPost, h.url("/register"), bytes.NewReader(payload))
	_, body := h.do(request)

	var registered struct {
		Name string `json:"client_name"`
	}
	_ = json.Unmarshal([]byte(body), &registered)

	if registered.Name != "Claude" {
		t.Errorf("client_name = %q, want the format characters removed", registered.Name)
	}
}

func TestAHostileRequestPathCannotForgeALogLine(t *testing.T) {
	// The path is whatever a stranger sent. Decoded, %0A starts a line of its
	// own and %1B reaches the owner's terminal as a control sequence.
	h := newTestHost(t)
	connection, err := net.Dial("tcp", h.server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = io.WriteString(connection,
		"GET /%0Amoodle:%20host%20passphrase:%20hunter2%1b[2K HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	_, _ = io.Copy(io.Discard, connection)

	logged := h.logContaining(" 404 ")

	if strings.Contains(logged, "\x1b") {
		t.Errorf("an escape byte reached the log: %q", logged)
	}
	for _, line := range strings.Split(strings.TrimSpace(logged), "\n") {
		if strings.HasPrefix(line, "moodle: host passphrase") {
			t.Errorf("a forged line reached the log: %q", logged)
		}
	}
}

func TestAPaddedPassphraseStillUnlocksTheHost(t *testing.T) {
	host, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: "  correct horse battery \n"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !host.authority.passphraseMatches("correct horse battery") {
		t.Error("the passphrase as typed does not unlock a host configured with it padded")
	}
}

func TestACallSurvivesItsClientDisconnecting(t *testing.T) {
	// A tunnel closing a connection is routine, and a write already sent to
	// Moodle is no less sent. The call must run to its end.
	started, release := make(chan struct{}), make(chan struct{})
	outcome := make(chan error, 1)
	h := newTestHostWith(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		outcome <- r.Context().Err()
	}))
	_, tokens := h.signIn()
	access, _ := tokens["access_token"].(string)

	ctx, hangUp := context.WithCancel(context.Background())
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.url("/mcp"), strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer "+access)
	go func() { _, _ = h.client.Do(request) }()

	<-started
	hangUp()
	time.Sleep(100 * time.Millisecond) // long enough for the server to notice
	close(release)

	select {
	case err := <-outcome:
		if err != nil {
			t.Errorf("the call's context ended with %v when only the client left", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call never finished")
	}
}

// serving starts Serve on a loopback port and returns its address, a way to
// stop it, and what it returned.
func serving(t *testing.T, host *Host) (address string, stop func(), done <-chan error) {
	t.Helper()
	listener, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- host.Serve(ctx, listener) }()
	return listener.Addr().String(), cancel, result
}

// withToken gives a host a live access token without a sign-in, for tests
// that are about what happens after one.
func withToken(host *Host) string {
	token := randomToken()
	host.authority.access[digest(token)] = grant{expires: time.Now().Add(time.Hour)}
	return token
}

func TestStoppingTheHostTellsACallThatOutlivesTheGraceToStop(t *testing.T) {
	// Without this a write still running when the process exits is cut off
	// with nobody left to record that it may have been applied — which is what
	// the signal context is for on stdio.
	previous := shutdownGrace
	shutdownGrace = 100 * time.Millisecond
	t.Cleanup(func() { shutdownGrace = previous })

	started, stopped := make(chan struct{}), make(chan error, 1)
	host, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: passphrase},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
			stopped <- r.Context().Err()
		}))
	if err != nil {
		t.Fatal(err)
	}
	token := withToken(host)
	address, stop, done := serving(t, host)

	go func() {
		request, _ := http.NewRequest(http.MethodPost, "http://"+address+"/mcp", strings.NewReader("{}"))
		request.Header.Set("Authorization", "Bearer "+token)
		_, _ = http.DefaultClient.Do(request)
	}()
	<-started
	stop()

	select {
	case err := <-stopped:
		if err == nil {
			t.Error("the call was released without being cancelled")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a call that outlived the grace period was never told to stop")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestStoppingTheHostLetsACallFinishWithinTheGrace(t *testing.T) {
	previous := shutdownGrace
	shutdownGrace = 5 * time.Second
	t.Cleanup(func() { shutdownGrace = previous })

	started := make(chan struct{})
	host, err := New(Config{PublicURL: "http://localhost:8787", Passphrase: passphrase},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			time.Sleep(200 * time.Millisecond)
			if r.Context().Err() != nil {
				http.Error(w, "cancelled", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, "finished")
		}))
	if err != nil {
		t.Fatal(err)
	}
	token := withToken(host)
	address, stop, done := serving(t, host)

	reply := make(chan string, 1)
	go func() {
		request, _ := http.NewRequest(http.MethodPost, "http://"+address+"/mcp", strings.NewReader("{}"))
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			reply <- err.Error()
			return
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		reply <- string(body)
	}()
	<-started
	stop()

	if got := <-reply; got != "finished" {
		t.Errorf("the call was %q, want it left to finish within the grace period", got)
	}
	if err := <-done; err != nil {
		t.Errorf("Serve returned %v", err)
	}
}

func quickVerify(t *testing.T) {
	t.Helper()
	within, every := verifyWithin, verifyEvery
	verifyWithin, verifyEvery = 300*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { verifyWithin, verifyEvery = within, every })
}

func TestVerifyAcceptsAnAddressThatReachesTheHost(t *testing.T) {
	h := newTestHost(t)

	if err := h.host.Verify(context.Background()); err != nil {
		t.Errorf("Verify = %v, want the host's own address accepted", err)
	}
}

func TestVerifyRefusesAnAddressThatLeadsSomewhereElse(t *testing.T) {
	// A tunnel can come up and still point at another service. What it answers
	// with is not this host's metadata, so the address must not be announced.
	quickVerify(t)
	for name, handler := range map[string]http.HandlerFunc{
		"another service": func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "DECOY") },
		"a catch-all 404": func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) },
		"another issuer": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"issuer":"https://somewhere-else.trycloudflare.com"}`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			other := httptest.NewServer(handler)
			defer other.Close()
			host, err := New(Config{PublicURL: other.URL, Passphrase: passphrase}, nil)
			if err != nil {
				t.Fatal(err)
			}

			err = host.Verify(context.Background())

			if err == nil || !strings.Contains(err.Error(), "does not reach this host") {
				t.Errorf("Verify = %v, want the address refused", err)
			}
		})
	}
}

func TestVerifyDoesNotFollowARedirectToTheHost(t *testing.T) {
	// An address that merely forwards to the host is not the host's address:
	// what the person would publish is the forwarder.
	quickVerify(t)
	real := httptest.NewUnstartedServer(nil)
	forwarder := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+real.Listener.Addr().String()+r.URL.Path, http.StatusFound)
	}))
	defer forwarder.Close()
	host, err := New(Config{PublicURL: forwarder.URL, Passphrase: passphrase}, nil)
	if err != nil {
		t.Fatal(err)
	}
	real.Config.Handler = host.Handler()
	real.Start()
	defer real.Close()

	err = host.Verify(context.Background())

	if err == nil || !strings.Contains(err.Error(), "does not reach this host") {
		t.Errorf("Verify = %v, want the forwarding address refused", err)
	}
}

func TestVerifyWaitsForAnAddressThatIsNotReachableYet(t *testing.T) {
	// A new address can take a moment to propagate.
	quickVerify(t)
	var calls int
	var host *Host
	server := httptest.NewUnstartedServer(nil)
	host, err := New(Config{PublicURL: "http://" + server.Listener.Addr().String(), Passphrase: passphrase}, nil)
	if err != nil {
		t.Fatal(err)
	}
	real := host.Handler()
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
			return
		}
		real.ServeHTTP(w, r)
	})
	server.Start()
	defer server.Close()

	if err := host.Verify(context.Background()); err != nil || calls < 3 {
		t.Errorf("Verify = %v after %d calls, want it to keep trying until the address answers", err, calls)
	}
}

func TestVerifyStopsWhenTheContextEnds(t *testing.T) {
	// Pressing Ctrl-C while the address is being checked must not wait it out.
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }))
	defer other.Close()
	host, _ := New(Config{PublicURL: other.URL, Passphrase: passphrase}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()

	started := time.Now()
	err := host.Verify(ctx)

	if !errors.Is(err, context.Canceled) || time.Since(started) > 5*time.Second {
		t.Errorf("Verify = %v after %s, want the cancellation promptly", err, time.Since(started))
	}
}

func TestACheckedPassphraseIsRefusedBeforeAnythingElseHappens(t *testing.T) {
	for passphrase, ok := range map[string]bool{
		"":                           true,
		"   ":                        true,
		"short":                      false,
		"correct horse battery":      true,
		"  padded but long enough  ": true,
	} {
		if err := CheckPassphrase(passphrase); (err == nil) != ok {
			t.Errorf("CheckPassphrase(%q) = %v, want ok = %v", passphrase, err, ok)
		}
	}
}

func TestTheAnnouncementNamesTheConnectorAddress(t *testing.T) {
	// With a tunnel of its own this is the only place the person learns the
	// address to add in Claude.
	var log bytes.Buffer
	host, err := New(Config{PublicURL: "https://example-words.trycloudflare.com", Passphrase: passphrase, Log: &log}, nil)
	if err != nil {
		t.Fatal(err)
	}

	host.Announce(&fakeAddr{})

	for _, want := range []string{
		"add https://example-words.trycloudflare.com/mcp as a custom connector",
		"listening on 127.0.0.1:8787",
	} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("the announcement is missing %q:\n%s", want, log.String())
		}
	}
}

func TestListenReportsAConfigurationError(t *testing.T) {
	first := httptest.NewServer(http.NotFoundHandler())
	defer first.Close()

	_, err := Listen(first.Listener.Addr().String())

	if got := errs.From(err); got.Code != errs.CodeConfiguration {
		t.Errorf("code = %s, want configuration", got.Code)
	}
}

func TestAConcurrentBurstOfGuessesStillStopsAtTheLimit(t *testing.T) {
	// The limit has to hold against guesses sent at the same moment, not only
	// one after another. Anyone who has a quick tunnel's address can send a burst.
	h := newTestHost(t)
	request := h.consent(h.register(claudeCallback))
	const burst = 40
	gate := make(chan struct{})
	statuses := make(chan int, burst)
	var wg sync.WaitGroup
	for range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			form := url.Values{"request": {request}, "passphrase": {"a wrong guess"}, "decision": {"allow"}}
			req, _ := http.NewRequest(http.MethodPost, h.url("/authorize"), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			<-gate
			response, err := h.client.Do(req)
			if err != nil {
				statuses <- -1
				return
			}
			response.Body.Close()
			statuses <- response.StatusCode
		}()
	}
	close(gate)
	wg.Wait()
	close(statuses)

	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[http.StatusForbidden] != maxFailures || counts[http.StatusTooManyRequests] != burst-maxFailures {
		t.Errorf("of %d simultaneous guesses, %d were compared and %d were refused; want %d and %d",
			burst, counts[http.StatusForbidden], counts[http.StatusTooManyRequests], maxFailures, burst-maxFailures)
	}
}

// refreshWith presents a refresh token.
func (h *testHost) refreshWith(clientID, refresh string) (*http.Response, map[string]any) {
	return h.tokenRequest(url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID},
	})
}

func (h *testHost) accessWorks(access string) bool {
	response, _ := h.callMCP(access)
	return response.StatusCode == http.StatusOK
}

func TestAReusedRefreshTokenEndsTheWholeSignIn(t *testing.T) {
	// If a thief refreshes first, the owner's next refresh fails, and the owner
	// signs in again. Without this the thief's chain would carry on beside it.
	h := newTestHost(t)
	clientID, first := h.signIn()
	spent, _ := first["refresh_token"].(string)
	_, second := h.refreshWith(clientID, spent)
	laterAccess, _ := second["access_token"].(string)
	laterRefresh, _ := second["refresh_token"].(string)
	h.host.authority.now = func() time.Time { return time.Now().Add(reuseAllowance + time.Second) }

	reuse, _ := h.refreshWith(clientID, spent)
	descendant, _ := h.refreshWith(clientID, laterRefresh)

	if reuse.StatusCode != http.StatusBadRequest {
		t.Errorf("reuse: status %d, want 400", reuse.StatusCode)
	}
	if descendant.StatusCode != http.StatusBadRequest {
		t.Errorf("the refresh token that replaced the spent one still works (%d): the thief's chain would survive", descendant.StatusCode)
	}
	if h.accessWorks(laterAccess) {
		t.Error("an access token issued from the compromised sign-in still works")
	}
}

func TestTwoRequestsRefreshingAtOnceDoNotEndTheSignIn(t *testing.T) {
	// A client that refreshes from two places sends the same token twice. The
	// second loses, and the first carries on.
	h := newTestHost(t)
	clientID, first := h.signIn()
	spent, _ := first["refresh_token"].(string)
	_, winner := h.refreshWith(clientID, spent)
	winnersRefresh, _ := winner["refresh_token"].(string)
	winnersAccess, _ := winner["access_token"].(string)

	loser, _ := h.refreshWith(clientID, spent)

	if loser.StatusCode != http.StatusBadRequest {
		t.Errorf("the second use of one token: status %d, want 400", loser.StatusCode)
	}
	if !h.accessWorks(winnersAccess) {
		t.Error("the sign-in that won the race was ended by the one that lost it")
	}
	if again, _ := h.refreshWith(clientID, winnersRefresh); again.StatusCode != http.StatusOK {
		t.Errorf("the winner could not refresh afterwards: status %d", again.StatusCode)
	}
}

func TestAReplayedCodeEndsTheSignInItStarted(t *testing.T) {
	h := newTestHost(t)
	clientID := h.register(claudeCallback)
	code := h.approve(h.consent(clientID), passphrase)
	_, tokens := h.exchange(clientID, code)
	access, _ := tokens["access_token"].(string)

	replay, _ := h.exchange(clientID, code)

	if replay.StatusCode != http.StatusBadRequest {
		t.Errorf("replay: status %d, want 400", replay.StatusCode)
	}
	if h.accessWorks(access) {
		t.Error("an access token from a code that was later replayed still works")
	}
}

func TestEndingOneSignInLeavesAnotherAlone(t *testing.T) {
	h := newTestHost(t)
	clientID, mine := h.signIn()
	_, theirs := h.signIn()
	theirAccess, _ := theirs["access_token"].(string)
	spent, _ := mine["refresh_token"].(string)
	h.refreshWith(clientID, spent)
	h.host.authority.now = func() time.Time { return time.Now().Add(reuseAllowance + time.Second) }

	h.refreshWith(clientID, spent) // reused: ends only the first sign-in

	if !h.accessWorks(theirAccess) {
		t.Error("ending one sign-in ended another that had nothing to do with it")
	}
}

func TestAThousandGuessesAtOnceStillGetOnlyTheLimit(t *testing.T) {
	// The lockout check and the recording of a miss are one step. Run apart,
	// every guess in a burst passes the check before any has been recorded, and
	// far more than the limit is compared. This is the check at its sharpest:
	// no network between the guesses, nothing to spread them out.
	a := newAuthority("https://moodle.example.com", passphrase, "school", false)
	const burst = 3000
	gate := make(chan struct{})
	var wrong, locked, right atomic.Int64
	var wg sync.WaitGroup
	for range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			switch a.tryPassphrase("a wrong guess") {
			case attemptWrong:
				wrong.Add(1)
			case attemptLocked:
				locked.Add(1)
			case attemptRight:
				right.Add(1)
			}
		}()
	}
	close(gate)
	wg.Wait()

	if wrong.Load() != maxFailures || locked.Load() != burst-maxFailures || right.Load() != 0 {
		t.Errorf("of %d guesses at once, %d were compared and %d refused; want %d and %d",
			burst, wrong.Load(), locked.Load(), maxFailures, burst-maxFailures)
	}
	if a.tryPassphrase(passphrase) != attemptLocked {
		t.Error("the right passphrase was accepted while locked out")
	}
}
