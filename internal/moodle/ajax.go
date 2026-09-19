package moodle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// PathAjax is the endpoint a signed-in browser uses.
const PathAjax = "/lib/ajax/service.php"

// AjaxSession is a browser session used directly, without a token.
//
// It exists for sites that issue no web service token at all — mobile web
// services switched off — where the only way in is the session a browser
// already holds.
//
// The endpoint exposes a different and much smaller set of functions than the
// web service one: a function is available here only if its component declared
// it with 'ajax' => true. There is no call that lists them, and
// core_webservice_get_site_info is itself not among them, so what is available
// is learned by asking and remembering the refusals.
type AjaxSession struct {
	client  *Client
	cookie  SessionCookie
	sesskey string
	// userID is read from the same page as the sesskey. Without it a browser
	// session has no identity at all: `auth status` could not say whose it
	// was, and a stored one could not be named after its owner.
	userID string

	// mu guards unavailable, which is shared by every call on this session.
	mu sync.Mutex
	// unavailable records functions this site does not offer over AJAX, so a
	// feature that falls back is not asked to discover the same refusal again.
	unavailable map[string]bool
}

// NewAjaxSession builds a session. The sesskey is fetched on first use.
func NewAjaxSession(client *Client, cookie SessionCookie) *AjaxSession {
	if strings.TrimSpace(cookie.Name) == "" {
		cookie.Name = DefaultSessionCookieName
	}
	return &AjaxSession{
		client:      client,
		cookie:      cookie,
		unavailable: map[string]bool{},
	}
}

// sesskeyPattern finds the token Moodle embeds in every signed-in page.
//
// There is no API that hands it over: it is a CSRF token, and the only place a
// client can read it is a page rendered for this session.
var sesskeyPattern = regexp.MustCompile(`"sesskey"\s*:\s*"([^"]+)"`)

// userIDPattern finds who the page was rendered for. Moodle puts it in the
// same configuration block as the sesskey, so it costs no extra request.
var userIDPattern = regexp.MustCompile(`"userId"\s*:\s*(\d+)`)

// Sesskey fetches and remembers the session key.
func (s *AjaxSession) Sesskey(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.sesskey != "" {
		key := s.sesskey
		s.mu.Unlock()
		return key, nil
	}
	s.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.client.Site().Endpoint("/my/"), nil)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternal, err, "cannot build the request")
	}
	request.AddCookie(&http.Cookie{Name: s.cookie.Name, Value: s.cookie.Value})

	body, err := s.client.send(request, "my/")
	if err != nil {
		return "", err
	}

	match := sesskeyPattern.FindSubmatch(body)
	if match == nil {
		// Either the session is not accepted — Moodle serves the login page,
		// which carries no sesskey — or the page has changed shape.
		return "", errs.New(errs.CodeAuthentication,
			"the site did not accept that browser session").
			WithReason(errs.ReasonTokenExpired).
			WithHint("the session may have expired; sign in again in your browser and copy it")
	}

	s.mu.Lock()
	s.sesskey = string(match[1])
	if who := userIDPattern.FindSubmatch(body); who != nil {
		s.userID = string(who[1])
	}
	s.mu.Unlock()
	return string(match[1]), nil
}

// UserID reports who this session belongs to, once a page has been read.
//
// Empty before that, and empty if the page did not carry it — which is not
// the same as nobody, and callers must not read it that way.
func (s *AjaxSession) UserID(ctx context.Context) string {
	if _, err := s.Sesskey(ctx); err != nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.userID
}

// ajaxReply is one entry of the endpoint's answer. The endpoint answers a
// batch, so the reply is always an array even for a single call.
type ajaxReply struct {
	Error     json.RawMessage `json:"error"`
	Exception *exception      `json:"exception"`
	Data      json.RawMessage `json:"data"`
}

// Call invokes one function over the AJAX endpoint.
func (s *AjaxSession) Call(ctx context.Context, function string, args map[string]any, out any) error {
	if s.Unavailable(function) {
		return errs.New(errs.CodeUnavailable,
			fmt.Sprintf("this site does not offer %s over a browser session", function)).
			WithReason(errs.ReasonCapability)
	}

	sesskey, err := s.Sesskey(ctx)
	if err != nil {
		return err
	}
	if args == nil {
		args = map[string]any{}
	}

	payload, err := json.Marshal([]map[string]any{
		{"index": 0, "methodname": function, "args": args},
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot encode the request")
	}

	endpoint := s.client.Site().Endpoint(PathAjax) +
		"?sesskey=" + sesskey + "&info=" + function
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(string(payload)))
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot build the request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: s.cookie.Name, Value: s.cookie.Value})

	body, err := s.client.send(request, function)
	if err != nil {
		return err
	}

	var replies []ajaxReply
	if err := json.Unmarshal(body, &replies); err != nil {
		return protocolDrift(function, body, err)
	}
	if len(replies) == 0 {
		return errs.New(errs.CodeUpstream,
			fmt.Sprintf("the site returned nothing for %s", function)).
			WithReason(errs.ReasonProtocolDrift)
	}

	reply := replies[0]
	if reply.Exception != nil && !reply.Exception.empty() {
		failure := reply.Exception.asError(function)
		if reply.Exception.ErrorCode == "servicenotavailable" {
			// The function exists on this site but is not exposed over AJAX.
			// Remembering it keeps a feature from rediscovering the same
			// refusal on every call.
			s.markUnavailable(function)
			return errs.New(errs.CodeUnavailable,
				fmt.Sprintf("this site does not offer %s over a browser session", function)).
				WithReason(errs.ReasonCapability)
		}
		return failure
	}
	if out == nil {
		return nil
	}
	if len(reply.Data) == 0 {
		return errs.New(errs.CodeUpstream,
			fmt.Sprintf("the site returned no data for %s", function)).
			WithReason(errs.ReasonProtocolDrift)
	}
	if err := json.Unmarshal(reply.Data, out); err != nil {
		return protocolDrift(function, reply.Data, err)
	}
	return nil
}

// Unavailable reports whether this session already learned that a function is
// not offered here.
func (s *AjaxSession) Unavailable(function string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unavailable[function]
}

func (s *AjaxSession) markUnavailable(function string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unavailable[function] = true
}
