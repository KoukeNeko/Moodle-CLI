package file_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
)

// fakeFetcher stands in for the site.
type fakeFetcher struct {
	content   string
	suggested string
	failAfter int // bytes to send before the connection drops; 0 sends it all
	err       error
	requested string
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string) (*file.Body, error) {
	f.requested = rawURL
	if f.err != nil {
		return nil, f.err
	}
	var content io.Reader = strings.NewReader(f.content)
	if f.failAfter > 0 {
		content = io.MultiReader(
			strings.NewReader(f.content[:f.failAfter]),
			brokenReader{},
		)
	}
	return &file.Body{
		Content:       io.NopCloser(content),
		SuggestedName: f.suggested,
	}, nil
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func download(t *testing.T, fetcher file.Fetcher, req file.Request) (file.Result, error) {
	t.Helper()
	if req.Dir == "" {
		req.Dir = t.TempDir()
	}
	if req.URL == "" {
		req.URL = "https://moodle.example.edu/webservice/pluginfile.php/1/a/b/notes.pdf"
	}
	return file.NewDownloader(fetcher).Download(context.Background(), req)
}

func TestAnInterruptedDownloadLeavesNothingBehind(t *testing.T) {
	// The worse outcome is not an error: it is a truncated file sitting there
	// under the right name, looking like it worked.
	dir := t.TempDir()
	fetcher := &fakeFetcher{content: "a whole report", suggested: "report.pdf", failAfter: 4}

	if _, err := download(t, fetcher, file.Request{Dir: dir}); err == nil {
		t.Fatal("a cut-off download was reported as a success")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Errorf("%s was left behind", entry.Name())
	}
}

func TestAnExistingFileIsNotReplacedByAccident(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(existing, []byte("my own work"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{content: "something else", suggested: "report.pdf"}

	_, err := download(t, fetcher, file.Request{Dir: dir})
	if code := errs.From(err).Code; code != errs.CodeConflict {
		t.Fatalf("code = %q, want conflict", code)
	}
	kept, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "my own work" {
		t.Errorf("the existing file was changed to %q", kept)
	}
}

func TestForceReplacesAndSaysSo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "report.pdf"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{content: "new bytes", suggested: "report.pdf"}

	result, err := download(t, fetcher, file.Request{Dir: dir, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replaced {
		t.Error("something was replaced and the result does not say so")
	}
	content, _ := os.ReadFile(result.Path)
	if string(content) != "new bytes" {
		t.Errorf("content = %q", content)
	}
}

func TestANameFromTheSiteCannotEscapeTheDirectory(t *testing.T) {
	// The filename is chosen by the far end. Treating it as a path is how a
	// download overwrites something it was never pointed at.
	for _, suggested := range []string{
		"../escaped.txt",
		"../../etc/passwd",
		"/etc/passwd",
		`..\windows.txt`,
		"%2e%2e%2fescaped.txt",
		"",
		".",
		"..",
	} {
		t.Run(suggested, func(t *testing.T) {
			dir := t.TempDir()
			fetcher := &fakeFetcher{content: "x", suggested: suggested}
			result, err := download(t, fetcher, file.Request{
				Dir: dir,
				URL: "https://moodle.example.edu/webservice/pluginfile.php/1/a/b/safe.pdf",
			})
			if err != nil {
				// Falling back to the URL's own name is fine; escaping is not.
				return
			}
			within, relErr := filepath.Rel(dir, result.Path)
			if relErr != nil || strings.HasPrefix(within, "..") {
				t.Fatalf("the download landed at %s, outside %s", result.Path, dir)
			}
			if within != "safe.pdf" {
				t.Errorf("saved as %q; the suggestion should have been refused", within)
			}
		})
	}
}

func TestANameTheUserTypedIsCheckedToo(t *testing.T) {
	// A mistake is still a mistake, and --as ../../.bashrc is worth catching.
	dir := t.TempDir()
	fetcher := &fakeFetcher{content: "x", suggested: "report.pdf"}

	_, err := download(t, fetcher, file.Request{Dir: dir, As: "../escaped.txt"})
	if code := errs.From(err).Code; code != errs.CodeValidation {
		t.Fatalf("code = %q, want validation", code)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped.txt")); err == nil {
		t.Fatal("a file was written outside the directory")
	}
}

func TestTheUrlIsTheLastResortForANameAndIsAlsoChecked(t *testing.T) {
	dir := t.TempDir()
	fetcher := &fakeFetcher{content: "x"} // the site suggests nothing

	result, err := download(t, fetcher, file.Request{
		Dir: dir,
		URL: "https://moodle.example.edu/webservice/pluginfile.php/1/a/b/notes%20and%20more.pdf",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Path) != "notes and more.pdf" {
		t.Errorf("saved as %q", filepath.Base(result.Path))
	}
}

func TestADownloadWithNoUsableNameAsksForOne(t *testing.T) {
	dir := t.TempDir()
	fetcher := &fakeFetcher{content: "x"}

	_, err := download(t, fetcher, file.Request{
		Dir: dir,
		URL: "https://moodle.example.edu/",
	})
	if err == nil {
		t.Fatal("a nameless download was saved anyway")
	}
	if !strings.Contains(errs.From(err).Hint, "--as") {
		t.Errorf("the error does not say how to proceed: %q", errs.From(err).Hint)
	}
}

func TestDispositionParsing(t *testing.T) {
	cases := map[string]string{
		`attachment; filename="report.pdf"`: "report.pdf",
		`attachment; filename=report.pdf`:   "report.pdf",
		`inline`:                            "",
		``:                                  "",
		`attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.pdf`:           "报告.pdf",
		`attachment; filename="plain.pdf"; filename*=UTF-8''%C3%A9.pdf`: "é.pdf",
	}
	for header, want := range cases {
		if got := file.NameFromDisposition(header); got != want {
			t.Errorf("%q: got %q, want %q", header, got, want)
		}
	}
}

func TestAMissingDirectoryIsReportedNotCreated(t *testing.T) {
	// Creating it silently would scatter files wherever a typo pointed.
	fetcher := &fakeFetcher{content: "x", suggested: "report.pdf"}
	_, err := download(t, fetcher, file.Request{Dir: filepath.Join(t.TempDir(), "nope")})
	if code := errs.From(err).Code; code != errs.CodeUsage {
		t.Errorf("code = %q, want usage", code)
	}
}
