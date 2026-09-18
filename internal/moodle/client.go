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
	"io"
	"net"
	"net/http"
	"net/url"
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

func (c *Client) send(request *http.Request, function string) ([]byte, error) {
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, networkError(request.Context(), err, function)
	}
	defer response.Body.Close()

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

	if response.StatusCode != http.StatusOK {
		return nil, httpStatusError(response, body, function)
	}
	return body, nil
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

func describeBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "an empty body"
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "<!doctype html") ||
		strings.HasPrefix(strings.ToLower(trimmed), "<html") {
		// Very common: a login page or an error page where JSON was expected.
		return "an HTML page rather than JSON, which usually means the URL is not a Moodle web service endpoint"
	}
	const limit = 120
	if len(trimmed) > limit {
		trimmed = trimmed[:limit] + "…"
	}
	return fmt.Sprintf("%q", trimmed)
}

func httpStatusError(response *http.Response, body []byte, function string) error {
	code := errs.CodeUpstream
	reason := errs.Reason("")
	hint := ""
	switch {
	case response.StatusCode == http.StatusUnauthorized, response.StatusCode == http.StatusForbidden:
		code = errs.CodeAuthentication
		hint = "sign in again with `moodle auth login`"
	case response.StatusCode == http.StatusNotFound:
		code = errs.CodeNotFound
		hint = "check the site URL points at a Moodle installation"
	case response.StatusCode == http.StatusTooManyRequests:
		code = errs.CodeNetwork
	case response.StatusCode >= 500:
		code = errs.CodeUpstream
	}
	err := errs.New(code, fmt.Sprintf("Moodle answered %s with HTTP %d",
		function, response.StatusCode)).WithHint(hint)
	if reason != "" {
		err = err.WithReason(reason)
	}
	_ = body
	return err
}

func networkError(ctx context.Context, cause error, function string) error {
	// A cancelled context is the user's own Ctrl-C, not a site problem.
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return errs.Wrap(errs.CodeNetwork, cause,
				fmt.Sprintf("%s timed out", function))
		}
		return errs.Wrap(errs.CodeNetwork, cause, "cancelled")
	}
	message := fmt.Sprintf("cannot reach Moodle for %s", function)
	hint := ""
	var dnsErr *net.DNSError
	if errors.As(cause, &dnsErr) {
		hint = "check the site URL and your network connection"
	}
	return errs.Wrap(errs.CodeNetwork, cause, message).WithHint(hint)
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
