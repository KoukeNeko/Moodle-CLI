package moodle_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// pluginfile is a stand-in for Moodle's file endpoint. It records what it was
// asked for, because what travels in the URL is the credential.
type pluginfile struct {
	server *httptest.Server
	asked  []url.Values
	handle func(w http.ResponseWriter, r *http.Request)
}

func newPluginfile(t *testing.T, handle func(http.ResponseWriter, *http.Request)) *pluginfile {
	t.Helper()
	p := &pluginfile{handle: handle}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.asked = append(p.asked, r.URL.Query())
		p.handle(w, r)
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *pluginfile) fetcher(t *testing.T) *moodle.FileFetcher {
	t.Helper()
	base, err := site.ParseBaseURL(p.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := moodle.NewClient(site.Site{Name: "school", BaseURL: base})
	return moodle.NewFileFetcher(client, "good-token", site.NewCapabilities())
}

func servePDF(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
	_, _ = w.Write([]byte("%PDF-1.4 real"))
}

func TestTheTokenTravelsInTheUrlBecauseMoodleWantsItThere(t *testing.T) {
	p := newPluginfile(t, servePDF)
	body, err := p.fetcher(t).Fetch(context.Background(),
		p.server.URL+"/webservice/pluginfile.php/1/a/b/report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Content.Close()

	if len(p.asked) != 1 {
		t.Fatalf("%d requests were made", len(p.asked))
	}
	if got := p.asked[0].Get("token"); got != "good-token" {
		t.Errorf("token = %q", got)
	}
	if body.SuggestedName != "report.pdf" {
		t.Errorf("suggested name = %q", body.SuggestedName)
	}
}

func TestATokenAlreadyInTheLinkIsReplacedNotKept(t *testing.T) {
	// A link can arrive from anywhere, including from someone else's session.
	// Using the token that came with it would act as that someone else.
	p := newPluginfile(t, servePDF)
	body, err := p.fetcher(t).Fetch(context.Background(),
		p.server.URL+"/webservice/pluginfile.php/1/a/b/report.pdf?token=somebody-elses")
	if err != nil {
		t.Fatal(err)
	}
	body.Content.Close()

	tokens := p.asked[0]["token"]
	if len(tokens) != 1 || tokens[0] != "good-token" {
		t.Errorf("tokens sent = %v, want exactly the signed-in one", tokens)
	}
}

func TestNothingIsSentToAnotherHost(t *testing.T) {
	// Moodle wants the credential in the URL, so where the request goes is a
	// credential decision: the wrong host is handed the token.
	p := newPluginfile(t, servePDF)
	_, err := p.fetcher(t).Fetch(context.Background(),
		"https://evil.example.com/webservice/pluginfile.php/1/a/b/report.pdf")
	if err == nil {
		t.Fatal("a download from another host was allowed")
	}
	if code := errs.From(err).Code; code != errs.CodePermissionDenied {
		t.Errorf("code = %q, want permission_denied", code)
	}
	if len(p.asked) != 0 {
		t.Error("a request was made before the host was checked")
	}
}

func TestAFailureWearingASuccessIsNotSavedAsTheFile(t *testing.T) {
	// The trap: a refused download is HTTP 200 with a JSON body. A client that
	// trusts the status code writes the error message to disk under the name
	// of the coursework it wanted.
	p := newPluginfile(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":"Invalid token - token not found","errorcode":"invalidtoken"}`))
	})

	_, err := p.fetcher(t).Fetch(context.Background(),
		p.server.URL+"/webservice/pluginfile.php/1/a/b/report.pdf")
	if err == nil {
		t.Fatal("an error page was handed back as a file")
	}
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		t.Errorf("code = %q, want authentication", e.Code)
	}
	if !strings.Contains(e.Error(), "Invalid token") {
		t.Errorf("Moodle's own wording was lost: %q", e.Error())
	}
}

func TestAGenuineJsonFileStillDownloadsWhole(t *testing.T) {
	// Checking for an error payload must not eat the file when the file really
	// is JSON.
	content := `{"marks": [1, 2, 3], "note": "not an error"}`
	p := newPluginfile(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(content))
	})

	body, err := p.fetcher(t).Fetch(context.Background(),
		p.server.URL+"/webservice/pluginfile.php/1/a/b/marks.json")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Content.Close()

	got, err := io.ReadAll(body.Content)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want the whole file", got)
	}
}

func TestOnlyHttpUrlsAreFetched(t *testing.T) {
	p := newPluginfile(t, servePDF)
	for _, raw := range []string{
		"file:///etc/passwd",
		"/webservice/pluginfile.php/1/a/b/report.pdf",
		"ftp://moodle.example.edu/x",
	} {
		if _, err := p.fetcher(t).Fetch(context.Background(), raw); err == nil {
			t.Errorf("%s was fetched", raw)
		}
	}
	if len(p.asked) != 0 {
		t.Error("a request was made for a URL that should have been refused")
	}
}

func TestAFileTheAccountMayNotReadIsNotBlamedOnTheSiteURL(t *testing.T) {
	// Moodle 對「你不能讀的檔案」與「這個檔案不存在」回同一個 404——那是刻意的，
	// 否則清單就能被拿來探測它藏了什麼。我們分不出來，這沒辦法；但把人指去檢查
	// 站台網址，是在解一個他根本沒有的問題。實測：同學去抓別人的繳交檔就是這樣。
	p := newPluginfile(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Sorry, the requested file could not be found"))
	})

	_, err := p.fetcher(t).Fetch(context.Background(),
		p.server.URL+"/webservice/pluginfile.php/1/assignsubmission_file/x.pdf")
	if err == nil {
		t.Fatal("a 404 was reported as a download")
	}
	failure := errs.From(err)
	if failure.Code != errs.CodeNotFound {
		t.Errorf("code = %q, want not_found", failure.Code)
	}
	if strings.Contains(failure.Hint, "site URL") {
		t.Errorf("the reader was sent to check a URL that is not the problem: %q", failure.Hint)
	}
	if !strings.Contains(failure.Hint, "may not be allowed") {
		t.Errorf("the hint does not name the other possibility: %q", failure.Hint)
	}
}
