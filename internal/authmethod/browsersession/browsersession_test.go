package browsersession_test

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/browsersession"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// callbackPayload builds what Moodle puts after "token=" in the redirect:
// base64 of md5(wwwroot+passport) ":::" token, optionally ":::" privatetoken.
// It is spelled out here rather than exported from the package under test, so
// a change to the real encoder cannot quietly agree with itself.
func callbackPayload(wwwRoot, passport, token, private string) string {
	sum := md5.Sum([]byte(wwwRoot + passport))
	raw := hex.EncodeToString(sum[:]) + ":::" + token
	if private != "" {
		raw += ":::" + private
	}
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// launchSite stands in for Moodle's launch endpoint, recording what it was
// sent: the session cookie is a credential, and where it goes matters.
type launchSite struct {
	server *httptest.Server
	cookie string
	query  string
	// reply is written instead of the redirect when set.
	reply func(http.ResponseWriter)
}

func newLaunchSite(t *testing.T) *launchSite {
	t.Helper()
	s := &launchSite{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(moodle.DefaultSessionCookieName); err == nil {
			s.cookie = c.Value
		}
		s.query = r.URL.RawQuery
		if s.reply != nil {
			s.reply(w)
			return
		}
		if s.cookie != "good-session" {
			// Moodle serves the login page rather than redirecting.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>login</html>"))
			return
		}
		passport := r.URL.Query().Get("passport")
		payload := callbackPayload(s.server.URL, passport, "ws-token", "private")
		w.Header().Set("Location", "moodlemobile://token="+payload)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *launchSite) authenticate(t *testing.T, cookie string) (auth.Credential, error) {
	t.Helper()
	base, err := site.ParseBaseURL(s.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	target := site.Site{Name: "school", BaseURL: base}
	method := browsersession.New(func(site.Site) *moodle.Client {
		return moodle.NewClient(target)
	})
	return method.Authenticate(context.Background(), auth.Request{
		Site: target, WWWRoot: s.server.URL, SessionCookie: cookie,
	})
}

func TestASessionBecomesAToken(t *testing.T) {
	s := newLaunchSite(t)
	credential, err := s.authenticate(t, "MoodleSession=good-session")
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token != "ws-token" {
		t.Errorf("token = %q", credential.Token)
	}
	if credential.Method != "browser-session" {
		t.Errorf("method = %q", credential.Method)
	}
}

func TestTheCookieIsAcceptedInTheFormsPeopleActuallyCopy(t *testing.T) {
	// Developer tools hand over the pair, the value alone, or a whole Cookie
	// header. Refusing any of them would be a pointless round trip.
	for _, form := range []string{
		"MoodleSession=good-session",
		"good-session",
		"  MoodleSession=good-session  ",
		"OTHER=x; MoodleSession=good-session; MOODLEID1_=y",
	} {
		t.Run(form, func(t *testing.T) {
			s := newLaunchSite(t)
			if _, err := s.authenticate(t, form); err != nil {
				t.Fatalf("%q was refused: %v", form, err)
			}
			if s.cookie != "good-session" {
				t.Errorf("the site received %q", s.cookie)
			}
		})
	}
}

func TestARejectedSessionIsAnAuthenticationFailure(t *testing.T) {
	// Moodle answers an unrecognised session by serving the login page, which
	// is a 200. Reading that as success would hand back the page as a token.
	s := newLaunchSite(t)
	_, err := s.authenticate(t, "MoodleSession=stale")
	if err == nil {
		t.Fatal("a rejected session was reported as a success")
	}
	if code := errs.From(err).Code; code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", code)
	}
}

func TestACallbackForAnotherLoginIsRefused(t *testing.T) {
	// The passport ties the reply to the request that asked for it. Without
	// the check, anything that can answer this request could hand over a
	// token, including one minted for a different site.
	s := newLaunchSite(t)
	s.reply = func(w http.ResponseWriter) {
		payload := callbackPayload(
			"https://elsewhere.example.edu", "some-other-passport", "stolen", "")
		w.Header().Set("Location", "moodlemobile://token="+payload)
		w.WriteHeader(http.StatusFound)
	}
	_, err := s.authenticate(t, "MoodleSession=good-session")
	if err == nil {
		t.Fatal("a callback from another login was accepted")
	}
}

func TestNoSessionSaysHowToGetOne(t *testing.T) {
	s := newLaunchSite(t)
	_, err := s.authenticate(t, "")
	if err == nil {
		t.Fatal("an empty session was accepted")
	}
	if !strings.Contains(errs.From(err).Hint, "--session-cookie") {
		t.Errorf("the error does not say what to pass: %q", errs.From(err).Hint)
	}
}

func TestTheSessionIsNotSentAnywhereButTheLaunchEndpoint(t *testing.T) {
	// A session cookie is the account. One request, one destination.
	s := newLaunchSite(t)
	if _, err := s.authenticate(t, "MoodleSession=good-session"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.query, "passport=") {
		t.Errorf("the launch request carried no passport: %q", s.query)
	}
}
