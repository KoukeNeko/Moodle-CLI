// Package moodle speaks Moodle's web service protocol.
//
// It owns transport and wire format only: which function to call, and what to
// do with the answer, is decided above it. It never retries — retry policy
// needs to know whether an operation writes, which lives in internal/safety.
package moodle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Endpoints on a Moodle site.
const (
	PathREST      = "/webservice/rest/server.php"
	PathTokenPHP  = "/login/token.php"
	PathAJAXLogin = "/lib/ajax/service-nologin.php"
)

// MobileService is Moodle's built-in mobile web service.
const MobileService = "moodle_mobile_app"

// maxResponseBytes caps a single web service response. Moodle can be made to
// return very large payloads, and an unbounded read turns that into an
// out-of-memory crash. File downloads do not go through here.
const maxResponseBytes = 64 << 20 // 64 MiB

// Client calls one Moodle site.
type Client struct {
	site       site.Site
	httpClient *http.Client
	userAgent  string
	trace      io.Writer
	// limiter paces requests so this client is not a burden on a site it does
	// not own. It is shared by every call through this client.
	limiter *limiter
	// credentials re-reads the stored token. It exists for one case: a
	// process that outlives the credential it started with. A short command
	// leaves it nil, because nothing can change underneath a run that lasts
	// half a second.
	//
	// It lives here rather than in each backend because every backend already
	// shares this client, and because none of them should have to know where
	// a credential comes from or when it is worth re-reading.
	credentials func() (string, error)
}

// WithCredentialSource lets a long-lived process recover from a credential
// that was replaced while it was running.
//
// Without it an MCP server reads the token once at startup and holds a dead
// one for ever: the agent is told, correctly, that authentication failed, and
// there is nothing it can do about it — it cannot sign in, and the human who
// can has no way to hand the result over short of a restart.
func WithCredentialSource(source func() (string, error)) Option {
	return func(c *Client) { c.credentials = source }
}

// SetCredentialSource attaches the source after construction, for a session
// that learns which account it is only once the caller has resolved it.
func (c *Client) SetCredentialSource(source func() (string, error)) {
	c.credentials = source
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client, for tests and for callers that
// need a proxy or custom roots.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) { c.httpClient = client }
}

// WithUserAgent overrides the User-Agent.
//
// Only the QR login exchange may claim to be the Moodle app, and only for
// that one request.
func WithUserAgent(agent string) Option {
	return func(c *Client) { c.userAgent = agent }
}

// WithTrace writes redacted request and response lines for debugging.
// WithPacing sets how often requests may go out. A zero MinInterval disables
// pacing, which is what a test usually wants.
func WithPacing(pacing Pacing) Option {
	return func(c *Client) { c.limiter = newLimiter(pacing) }
}

func WithTrace(w io.Writer) Option {
	return func(c *Client) { c.trace = w }
}

// DefaultUserAgent identifies this tool honestly.
func DefaultUserAgent(version string) string {
	return "moodle-cli/" + version
}

// NewClient builds a client for a site.
func NewClient(target site.Site, opts ...Option) *Client {
	client := &Client{
		site:       target,
		httpClient: NewHTTPClient(),
		userAgent:  DefaultUserAgent("dev"),
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// NewHTTPClient builds the shared HTTP client.
//
// The timeouts are segmented on purpose. A single http.Client.Timeout covers
// the whole request including the body, which would kill a legitimate slow
// download; these bound the parts that should never be slow and leave the
// body to the caller's context.
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Transport: transport,
		// No Timeout: see above. No CheckRedirect override either — the
		// default already stops after 10 hops.
	}
}

// Site returns the site this client talks to.
func (c *Client) Site() site.Site { return c.site }

// Call invokes a web service function and decodes the result into out.
//
// A Moodle exception arrives with HTTP 200, so the status line is never
// enough: the body is inspected before it is handed to the caller.
func (c *Client) Call(ctx context.Context, token, function string, params Params, out any) error {
	err := c.call(ctx, token, function, params, out)
	fresh, ok := c.reauthenticate(token, err)
	if !ok {
		return err
	}
	// Exactly once, and only for a credential that has actually changed.
	// Retrying is safe here and nowhere else: Moodle rejects an unknown token
	// during authentication, before the function asked for runs at all.
	return c.call(ctx, fresh, function, params, out)
}

func (c *Client) call(ctx context.Context, token, function string, params Params, out any) error {
	values, err := Encode(params)
	if err != nil {
		return err
	}
	values.Set("wstoken", token)
	values.Set("wsfunction", function)
	values.Set("moodlewsrestformat", "json")

	body, err := c.post(ctx, c.site.Endpoint(PathREST), values, function)
	if err != nil {
		return err
	}
	return decode(body, function, out)
}

// reauthenticate reports whether the call is worth one more attempt, and with
// what.
//
// Only a credential Moodle does not recognise qualifies. An access failure is
// deliberately left alone: Moodle answers accessexception for an IP the site
// will not accept, for a service that is switched off and for a function the
// token's service does not carry, and "sign in again" is the wrong advice for
// every one of them.
func (c *Client) reauthenticate(used string, err error) (string, bool) {
	if err == nil || c.credentials == nil {
		return "", false
	}
	if errs.From(err).Reason != errs.ReasonTokenExpired {
		return "", false
	}
	fresh, readErr := c.credentials()
	if readErr != nil || fresh == "" || fresh == used {
		// Nothing newer is stored, so the answer stands. Saying so is the
		// point: a caller that cannot sign in needs to know that waiting will
		// not help and a person has to act.
		return "", false
	}
	return fresh, true
}

// CallNoLogin invokes a function that does not require a token, through the
// AJAX entry point Moodle exposes for exactly those (QR login, public config).
func (c *Client) CallNoLogin(ctx context.Context, function string, args map[string]any, out any) error {
	payload := []map[string]any{{
		"index":      0,
		"methodname": function,
		"args":       args,
	}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot encode the request")
	}

	endpoint := c.site.Endpoint(PathAJAXLogin) + "?info=" + url.QueryEscape(function)
	body, err := c.postJSON(ctx, endpoint, encoded, function)
	if err != nil {
		return err
	}

	// The AJAX endpoint answers with an array, one entry per call.
	var responses []struct {
		Error     any             `json:"error"`
		Exception *exception      `json:"exception"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &responses); err != nil {
		return protocolDrift(function, body, err)
	}
	if len(responses) == 0 {
		return protocolDrift(function, body, errors.New("empty response array"))
	}
	first := responses[0]
	if first.Exception != nil && !first.Exception.empty() {
		return first.Exception.asError(function)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(first.Data, out); err != nil {
		return protocolDrift(function, first.Data, err)
	}
	return nil
}

// post sends form values and returns the raw body.
func (c *Client) post(ctx context.Context, endpoint string, values url.Values, function string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(values.Encode()))
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err, "cannot build the request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.traceRequest(request, values)
	return c.send(request, function)
}

func (c *Client) postJSON(ctx context.Context, endpoint string, body []byte, function string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err, "cannot build the request")
	}
	request.Header.Set("Content-Type", "application/json")
	c.traceRequest(request, nil)
	return c.send(request, function)
}

// redirectResponse sends a request and hands back the redirect rather than
// following it.
//
// The launch endpoint answers with a custom scheme that Go cannot fetch, and
// the answer is the Location header itself. Following it would at best waste a
// request and at worst send the session cookie somewhere else.
func (c *Client) redirectResponse(request *http.Request, function string) (*http.Response, error) {
	if err := c.limiter.wait(request.Context()); err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	request.Header.Set("User-Agent", c.userAgent)

	noRedirect := *c.httpClient
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := noRedirect.Do(request)
	if err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	return response, nil
}

// stream sends a request and hands back the open response.
//
// Unlike send it does not read the body, because a download can be larger than
// anything worth holding in memory. The caller owns the body and must close
// it.
func (c *Client) stream(request *http.Request, function string) (*http.Response, error) {
	if err := c.limiter.wait(request.Context()); err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	request.Header.Set("User-Agent", c.userAgent)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	if response.StatusCode != http.StatusOK {
		// The body is the error message here, and it is small; reading it is
		// what makes the failure explainable.
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
		response.Body.Close()
		c.noteBackoff(response)
		return nil, httpStatusError(response, body, function)
	}
	return response, nil
}

func (c *Client) send(request *http.Request, function string) ([]byte, error) {
	if err := c.limiter.wait(request.Context()); err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	defer response.Body.Close()

	// The Go client follows redirects, so a hop is only visible afterwards.
	// A web service endpoint never redirects: Moodle answers it in place. One
	// that did means something else took the request — most often an SSO
	// gateway — and the page that came back is that gateway's, not Moodle's.
	redirectedTo := ""
	if final := response.Request.URL; final != nil && final.String() != request.URL.String() {
		redirectedTo = RedactURL(final)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		// The request was sent and may have been executed; the caller has to
		// treat this as unknown rather than as a clean failure.
		return nil, errs.Wrap(errs.CodeNetwork, err,
			fmt.Sprintf("the connection dropped while reading the response to %s", function)).
			WithReason(errs.ReasonResponseLost).
			Ambiguous()
	}
	if len(body) > maxResponseBytes {
		return nil, errs.New(errs.CodeUpstream,
			fmt.Sprintf("the response to %s exceeds %d bytes", function, maxResponseBytes))
	}
	c.traceResponse(response, body)

	if redirectedTo != "" && isHTML(body) {
		// Said before the status check on purpose: such a gateway usually
		// answers 200 with its own sign-in page, so the status line looks
		// perfectly healthy and only the hop gives it away.
		// The reason is not knowable from here. A gateway in front of the site,
		// a session the site no longer accepts, a host it does not consider
		// its own — each ends at a page instead of an answer. Naming one of
		// them is a guess, and the first wording named single sign-on, which
		// was wrong for the session case this is most often.
		return nil, errs.New(errs.CodeAuthentication,
			fmt.Sprintf("the request for %s was answered by a page, not the web service", function)).
			WithReason(errs.ReasonTokenExpired).
			WithHint("it was redirected to " + redirectedTo + "; the credential may no " +
				"longer be accepted, or something in front of the site is handling sign-in")
	}

	if response.StatusCode != http.StatusOK {
		c.noteBackoff(response)
		return nil, httpStatusError(response, body, function)
	}
	return body, nil
}

// noteBackoff lets a site slow this client down.
//
// Being asked to wait is not a suggestion, and the ask outlives the request
// that received it: the next call waits too, rather than every caller
// discovering the same 429 for itself.
func (c *Client) noteBackoff(response *http.Response) {
	if c.limiter == nil {
		return
	}
	switch response.StatusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
	default:
		return
	}
	now := time.Now()
	delay, named := retryAfter(response.Header, now)
	if !named {
		// Asked for quiet without saying how long. A short pause is the
		// cooperative reading; guessing a long one would be worse than the
		// site asking again.
		delay = 5 * time.Second
	}
	if delay > maxRetryAfter {
		delay = maxRetryAfter
	}
	c.limiter.pause(now.Add(delay))
}

// decode turns a REST body into out, failing on a Moodle exception first.
func decode(body []byte, function string, out any) error {
	trimmed := strings.TrimSpace(string(body))
	// A Moodle exception is always a JSON object; a successful call may return
	// an array or a bare value, so only objects are worth inspecting.
	if strings.HasPrefix(trimmed, "{") {
		var ex exception
		if err := json.Unmarshal(body, &ex); err == nil && !ex.empty() {
			return ex.asError(function)
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return protocolDrift(function, body, err)
	}
	return nil
}

// protocolDrift reports a response we could not read at all. It is deliberately
// not "not found": the data may well be there, we just stopped understanding
// the format.
func protocolDrift(function string, body []byte, cause error) error {
	return errs.Wrap(errs.CodeUpstream, cause,
		fmt.Sprintf("cannot read Moodle's response to %s", function)).
		WithReason(errs.ReasonProtocolDrift).
		WithHint(fmt.Sprintf("the site answered with %s", describeBody(body)))
}

// isHTML reports that a body is a page rather than the JSON a web service
// call answers with.
func isHTML(body []byte) bool {
	trimmed := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func describeBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "an empty body"
	}
	if isHTML(body) {
		// Very common: a login page or an error page where JSON was expected.
		return "an HTML page rather than JSON, which usually means the URL is not a Moodle web service endpoint"
	}
	if notice := phpNotice(trimmed); notice != "" {
		// A site with display_errors on prepends PHP's own warning to the
		// body, so valid JSON arrives unparseable. Quoting the whole thing
		// gives the reader a wall of markup; naming it tells an administrator
		// exactly what to turn off.
		return "JSON preceded by a PHP " + notice + ", which a site should not " +
			"display; an administrator has debugging output switched on"
	}
	const limit = 120
	if len(trimmed) > limit {
		trimmed = trimmed[:limit] + "…"
	}
	return fmt.Sprintf("%q", trimmed)
}

// phpNotice names the kind of PHP diagnostic a body starts with, empty when
// it does not start with one.
func phpNotice(trimmed string) string {
	// PHP writes these as "<br />\n<b>Notice</b>:  ..." when html_errors is on
	// and as "Notice: ..." when it is not.
	plain := strings.TrimSpace(strings.TrimPrefix(trimmed, "<br />"))
	plain = strings.TrimSpace(strings.TrimPrefix(plain, "<b>"))
	for _, kind := range []string{"Notice", "Warning", "Deprecated", "Fatal error", "Parse error"} {
		if strings.HasPrefix(plain, kind+"</b>:") || strings.HasPrefix(plain, kind+":") {
			return strings.ToLower(kind)
		}
	}
	return ""
}

func httpStatusError(response *http.Response, body []byte, function string) error {
	code := errs.CodeUpstream
	reason := errs.Reason("")
	hint := ""
	retryable := false
	switch {
	case response.StatusCode == http.StatusUnauthorized, response.StatusCode == http.StatusForbidden:
		code = errs.CodeAuthentication
		hint = "sign in again with `moodle auth login`"
		if isHTML(body) {
			// Moodle refuses a web service call with HTTP 200 and a JSON
			// exception, never with a bare 401/403 page. Something in front of
			// it did this — a WAF, a proxy, an SSO gateway — and telling the
			// reader to sign in again sends them to fix a credential that was
			// never looked at. Measured against a WAF-shaped block page.
			code = errs.CodeUpstream
			hint = "Moodle refuses a web service call with JSON, not a page, so " +
				"something in front of the site blocked this request"
		}
	case response.StatusCode == http.StatusNotFound:
		code = errs.CodeNotFound
		hint = "check the site URL points at a Moodle installation"
		if errorCode, _ := moodleErrorPage(body); errorCode != "" {
			// Moodle itself answered, with its own error page. It serves a
			// refusal as a 404 too — measured: a course that does not show
			// students their grade report — and the site URL is plainly fine.
			hint = "Moodle refused this page to this account; the course may not offer it to you"
			if errorCode == "moodle/nopermissions" {
				code = errs.CodePermissionDenied
			}
		}
		if function == PathPluginFile {
			// Moodle answers a file this account may not read with the same
			// 404 as one that does not exist — deliberately, so that a listing
			// cannot be probed for what it is hiding. Measured: a classmate
			// asking for another student's submission gets exactly this.
			// Sending them to check the site URL is advice for a problem they
			// do not have.
			hint = "the file may not exist, or this account may not be allowed " +
				"to read it — Moodle answers both the same way"
		}
	case response.StatusCode == http.StatusTooManyRequests:
		code = errs.CodeUnavailable
		reason = errs.ReasonRateLimited
		hint = "the site is asking for fewer requests; " + waitAdvice(response)
		retryable = true
	case response.StatusCode == http.StatusServiceUnavailable:
		code = errs.CodeUnavailable
		hint = "the site is temporarily unavailable; " + waitAdvice(response)
		retryable = true
	case response.StatusCode >= 500:
		code = errs.CodeUpstream
	}

	message := fmt.Sprintf("Moodle answered %s with HTTP %d", function, response.StatusCode)
	// Moodle often explains itself in the body of an HTTP error. Throwing that
	// away leaves the user with a number and nothing to act on.
	if detail := describeErrorBody(body); detail != "" {
		message += ": " + detail
	}
	err := errs.New(code, message).WithHint(hint)
	if retryable {
		err = err.AsRetryable()
	}
	if reason != "" {
		err = err.WithReason(reason)
	}
	return err
}

// Moodle's error page links the error's documentation as
// .../error/<component>/<code>, and prints the reason in p.errormessage. Both
// come from print_error, whatever the theme or language.
var (
	moodleErrorLink    = regexp.MustCompile(`/error/([a-z0-9_]+)/([a-z0-9_]+)"`)
	moodleErrorMessage = regexp.MustCompile(`(?s)<p class="errormessage">(.*?)</p>`)
	markupTag          = regexp.MustCompile(`<[^>]*>`)
)

// moodleErrorPage reads the error code, as component/code, and the reason off
// one of Moodle's own error pages. The code is empty for any other page.
func moodleErrorPage(body []byte) (errorCode, message string) {
	link := moodleErrorLink.FindSubmatch(body)
	if link == nil {
		return "", ""
	}
	errorCode = string(link[1]) + "/" + string(link[2])
	if found := moodleErrorMessage.FindSubmatch(body); found != nil {
		text := markupTag.ReplaceAllString(string(found[1]), "")
		message = strings.Join(strings.Fields(html.UnescapeString(text)), " ")
	}
	return errorCode, message
}

// waitAdvice turns Retry-After into something a person can act on.
func waitAdvice(response *http.Response) string {
	delay, named := retryAfter(response.Header, time.Now())
	if !named {
		return "try again in a moment"
	}
	return "try again in " + delay.Round(time.Second).String()
}

// describeErrorBody pulls a usable sentence out of an HTTP error body.
func describeErrorBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	// An exception has a message worth quoting; an HTML error page does not.
	if strings.HasPrefix(trimmed, "{") {
		var ex exception
		if err := json.Unmarshal(body, &ex); err == nil && !ex.empty() {
			return firstNonEmpty(ex.Message, ex.Error, ex.ErrorCode)
		}
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "<") {
		// Moodle's own error page is the exception: it states the reason
		// and names the error, which is worth more than the status code.
		if errorCode, message := moodleErrorPage(body); errorCode != "" {
			if message == "" {
				return errorCode
			}
			return message + " (" + errorCode + ")"
		}
		return ""
	}
	const limit = 160
	if len(trimmed) > limit {
		return trimmed[:limit] + "…"
	}
	return trimmed
}

func networkError(ctx context.Context, cause error, function string) error {
	// A cancelled context is the user's own Ctrl-C, not a site problem.
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return errs.Wrap(errs.CodeNetwork, cause,
				fmt.Sprintf("%s timed out", function)).AsRetryable()
		}
		return errs.Wrap(errs.CodeNetwork, cause, "cancelled")
	}
	message := fmt.Sprintf("cannot reach Moodle for %s", function)
	hint := ""
	var dnsErr *net.DNSError
	if errors.As(cause, &dnsErr) {
		hint = "check the site URL and your network connection"
	}
	// The request never reached the site, so nothing there changed and the
	// same call is safe to make again. That is the one thing a caller most
	// wants to know here, and the contract had been answering "no" to it.
	return errs.Wrap(errs.CodeNetwork, cause, message).WithHint(hint).AsRetryable()
}

func (c *Client) traceRequest(request *http.Request, values url.Values) {
	if c.trace == nil {
		return
	}
	fmt.Fprintf(c.trace, "> %s %s\n", request.Method, RedactURL(request.URL))
	for name, items := range RedactHeader(request.Header) {
		fmt.Fprintf(c.trace, ">   %s: %s\n", name, strings.Join(items, ", "))
	}
	if values != nil {
		fmt.Fprintf(c.trace, ">   body: %s\n", RedactValues(values).Encode())
	}
}

func (c *Client) traceResponse(response *http.Response, body []byte) {
	if c.trace == nil {
		return
	}
	fmt.Fprintf(c.trace, "< %s\n", response.Status)
	for name, items := range RedactHeader(response.Header) {
		fmt.Fprintf(c.trace, "<   %s: %s\n", name, strings.Join(items, ", "))
	}
	fmt.Fprintf(c.trace, "<   body: %s\n", describeBody(body))
}
