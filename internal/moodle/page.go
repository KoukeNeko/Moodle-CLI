package moodle

import (
	"context"
	"net/http"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// PageReader fetches Moodle's own pages with a browser session.
//
// It is read-only by construction: it issues GET and never follows a form or
// an action. Reading a page is the last route, for what neither the web
// service nor the AJAX endpoint exposes.
type PageReader struct {
	client *Client
	cookie SessionCookie
}

// NewPageReader builds the page transport.
func NewPageReader(client *Client, cookie SessionCookie) *PageReader {
	if strings.TrimSpace(cookie.Name) == "" {
		cookie.Name = DefaultSessionCookieName
	}
	return &PageReader{client: client, cookie: cookie}
}

// Get fetches one page, relative to the site root.
func (p *PageReader) Get(ctx context.Context, path string, query map[string]string) (string, error) {
	endpoint := p.client.Site().Endpoint(path)
	if len(query) > 0 {
		values := make([]string, 0, len(query))
		for name, value := range query {
			values = append(values, name+"="+value)
		}
		endpoint += "?" + strings.Join(values, "&")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternal, err, "cannot build the request")
	}
	request.AddCookie(&http.Cookie{Name: p.cookie.Name, Value: p.cookie.Value})
	// Ask for a page rather than JSON; the client's default Accept is for the
	// web service endpoints.
	request.Header.Set("Accept", "text/html")

	body, err := p.client.send(request, path)
	if err != nil {
		return "", err
	}

	page := string(body)
	if isLoginPage(page) {
		// Moodle answers an unrecognised session by serving the login form,
		// with HTTP 200. Handing that to a parser would report the shape of
		// the login page as the shape of the data.
		return "", errs.New(errs.CodeAuthentication,
			"the site did not accept that browser session").
			WithReason(errs.ReasonTokenExpired).
			WithHint("the session may have expired; sign in again in your browser and copy it")
	}
	return page, nil
}

// isLoginPage reports whether Moodle served the sign-in form.
//
// It anchors on the form's own identifiers rather than on any wording, so it
// holds whatever language the site is in.
func isLoginPage(page string) bool {
	return strings.Contains(page, `id="login"`) &&
		strings.Contains(page, "logintoken")
}
