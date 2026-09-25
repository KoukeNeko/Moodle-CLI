package moodle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// FileFetcher downloads a file Moodle serves.
type FileFetcher struct {
	client *Client
	token  string
	// siteURL is the root the site builds its own links from, which is not
	// necessarily the address the user typed.
	siteURL string
	// cookie is a browser session, used when there is no token. The files a
	// page links to are served to that session the way they are to the
	// browser it came from.
	cookie SessionCookie
}

// WithSession lets the fetcher download with a browser session when it has no
// token.
func (f *FileFetcher) WithSession(cookie SessionCookie) *FileFetcher {
	if strings.TrimSpace(cookie.Name) == "" {
		cookie.Name = DefaultSessionCookieName
	}
	f.cookie = cookie
	return f
}

// pathWebServiceFile is the token route to a file. A session is not accepted
// there; the same file sits at the plain route without the prefix.
const pathWebServiceFile = "/webservice/pluginfile.php/"

// NewFileFetcher builds the downloader's transport.
// PathPluginFile names the file endpoint in diagnostics. It is not a web
// service function, and its refusals do not read like one: a file this
// account may not see comes back as a 404, the same as one that is gone.
const PathPluginFile = "pluginfile.php"

func NewFileFetcher(client *Client, token string, capabilities *site.Capabilities) *FileFetcher {
	fetcher := &FileFetcher{client: client, token: token}
	if capabilities != nil {
		fetcher.siteURL = capabilities.SiteURL
	}
	return fetcher
}

// Fetch opens a file for reading.
//
// Moodle wants the token in the query string here rather than a header, which
// makes where the request is going a credential decision: sending it to the
// wrong host hands the token to whoever runs that host. So the URL is checked
// against the site before anything is sent, and nothing is ever followed off
// it.
func (f *FileFetcher) Fetch(ctx context.Context, rawURL string) (*file.Body, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, errs.Wrap(errs.CodeUsage, err, "that is not a URL")
	}
	if err := f.belongsToSite(parsed); err != nil {
		return nil, err
	}

	// Any token already in the link is replaced rather than added to: a link
	// that arrived with someone else's token must not be used with it.
	query := parsed.Query()
	bySession := f.token == "" && f.cookie.Value != ""
	if bySession {
		query.Del("token")
		parsed.Path = strings.Replace(parsed.Path, pathWebServiceFile, "/pluginfile.php/", 1)
	} else {
		query.Set("token", f.token)
	}
	parsed.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternal, err, "cannot build the download request")
	}
	if bySession {
		request.AddCookie(&http.Cookie{Name: f.cookie.Name, Value: f.cookie.Value})
	}

	response, err := f.client.stream(request, PathPluginFile)
	if err != nil {
		return nil, err
	}

	if err := rejectErrorPayload(response); err != nil {
		response.Body.Close()
		return nil, err
	}
	if bySession {
		if err := rejectLoginPage(response); err != nil {
			response.Body.Close()
			return nil, err
		}
	}

	return &file.Body{
		Content:       response.Body,
		SuggestedName: file.NameFromDisposition(response.Header.Get("Content-Disposition")),
		Size:          response.ContentLength,
		MIMEType:      contentType(response.Header.Get("Content-Type")),
	}, nil
}

// rejectErrorPayload catches a failure wearing a success's clothes.
//
// A refused download comes back as HTTP 200 with a JSON body, so a client that
// trusts the status code saves the error message to disk under the name of the
// file it wanted. The result is a file that exists, has a plausible name, and
// is not the coursework.
func rejectErrorPayload(response *http.Response) error {
	if contentType(response.Header.Get("Content-Type")) != "application/json" {
		return nil
	}
	// Bounded: an error payload is small, and a site legitimately serving a
	// large JSON file must not be read into memory whole.
	const maxErrorPayload = 64 << 10
	head, err := io.ReadAll(io.LimitReader(response.Body, maxErrorPayload))
	if err != nil {
		return errs.Wrap(errs.CodeUpstream, err, "the site sent an unreadable reply")
	}

	// decode already knows every shape a Moodle failure arrives in, including
	// this endpoint's habit of reporting in "error" rather than "message".
	if failure := decode(head, PathPluginFile, nil); failure != nil {
		return failure
	}

	// A genuine JSON file. Put back what was read so the caller still gets all
	// of it.
	response.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), response.Body), response.Body}
	return nil
}

func contentType(header string) string {
	if header == "" {
		return ""
	}
	parsed, _, err := mime.ParseMediaType(header)
	if err != nil {
		return strings.TrimSpace(strings.SplitN(header, ";", 2)[0])
	}
	return parsed
}

// rejectLoginPage refuses the sign-in form served in place of a file.
//
// Moodle answers a session it no longer accepts with the login page and HTTP
// 200, which would otherwise be saved under the file's name and look like a
// download that worked.
func rejectLoginPage(response *http.Response) error {
	if contentType(response.Header.Get("Content-Type")) != "text/html" {
		return nil
	}
	const maxPage = 512 << 10
	head, err := io.ReadAll(io.LimitReader(response.Body, maxPage))
	if err != nil {
		return errs.Wrap(errs.CodeUpstream, err, "the site sent an unreadable reply")
	}
	if isLoginPage(string(head)) {
		return errs.New(errs.CodeAuthentication,
			"the site did not accept that browser session").
			WithReason(errs.ReasonTokenExpired).
			WithHint("the session may have expired; sign in again in your browser and copy it")
	}
	response.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), response.Body), response.Body}
	return nil
}

// belongsToSite refuses a URL that is not this Moodle's.
func (f *FileFetcher) belongsToSite(target *url.URL) error {
	if !target.IsAbs() || target.Host == "" {
		return errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is not an absolute URL", target.String()))
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return errs.New(errs.CodeUsage,
			fmt.Sprintf("refusing to fetch a %s URL", target.Scheme))
	}

	for _, candidate := range f.roots() {
		if candidate == nil {
			continue
		}
		if strings.EqualFold(candidate.Host, target.Host) && candidate.Scheme == target.Scheme {
			return nil
		}
	}
	return errs.New(errs.CodePermissionDenied,
		fmt.Sprintf("refusing to send this site's credential to %s", target.Host)).
		WithHint("a file link must be on the site you signed in to")
}

// roots are the addresses that count as this site: the one configured, and the
// one the site reports for itself. They differ more often than you would like
// — a site reached at moodle.example.edu can build its own links as
// www.moodle.example.edu — and refusing the site's own links would be wrong.
func (f *FileFetcher) roots() []*url.URL {
	roots := []*url.URL{f.client.Site().BaseURL}
	if wwwroot := f.client.Site().WWWRoot; wwwroot != nil {
		roots = append(roots, wwwroot)
	}
	if f.siteURL != "" {
		if parsed, err := url.Parse(f.siteURL); err == nil {
			roots = append(roots, parsed)
		}
	}
	return roots
}
