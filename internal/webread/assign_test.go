package webread_test

import (
	"os"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

// fixture loads a page captured from a real Moodle.
func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/assign-" + name + ".html")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestStatusIsReadFromRealPages(t *testing.T) {
	// Each page came from a Moodle with mobile web services switched off,
	// where no API can answer these questions at all.
	cases := map[string]struct {
		status string
		files  []string
	}{
		"submitted": {"submitted", []string{"report.pdf"}},
		"draft":     {"draft", nil},
		"none":      {"new", nil},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := webread.ParseAssignStatus(fixture(t, name))
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != want.status {
				t.Errorf("status = %q, want %q", got.Status, want.status)
			}
			if len(got.Files) != len(want.files) {
				t.Fatalf("files = %v, want %v", got.Files, want.files)
			}
			for i, file := range want.files {
				if got.Files[i] != file {
					t.Errorf("file %d = %q, want %q", i, got.Files[i], file)
				}
			}
		})
	}
}

func TestNothingSubmittedIsNewNotAGuess(t *testing.T) {
	// Moodle writes no status class when there is nothing to describe, and it
	// does so in exactly that one case. The absence is the answer, so a page
	// that anchors correctly must never be reported as unreadable.
	got, err := webread.ParseAssignStatus(fixture(t, "none"))
	if err != nil {
		t.Fatalf("a page with nothing submitted was reported as unreadable: %v", err)
	}
	if got.Status != "new" {
		t.Errorf("status = %q, want new", got.Status)
	}
}

func TestAPageWithoutTheAnchorSaysTheShapeChanged(t *testing.T) {
	// Reading whatever else is on the page is how a scraper reports a
	// confident wrong answer.
	page := strings.Replace(fixture(t, "submitted"), "submissionstatustable", "somethingelse", 1)

	_, err := webread.ParseAssignStatus(page)
	if err == nil {
		t.Fatal("a page missing its anchor was parsed anyway")
	}
	e := errs.From(err)
	if e.Reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q, want protocol_drift", e.Reason)
	}
	if !strings.Contains(e.Error(), "submissionstatustable") {
		t.Errorf("the error does not name what was missing: %q", e.Error())
	}
}

func TestSomethingThatIsNotAnAssignmentPageIsRefused(t *testing.T) {
	for _, page := range []string{
		"",
		"<html><body>not moodle</body></html>",
		"<html><body><table class=\"generaltable\"><tr><td>Submitted</td></tr></table></body></html>",
	} {
		if _, err := webread.ParseAssignStatus(page); err == nil {
			t.Errorf("parsed a page that is not an assignment: %.40q", page)
		}
	}
}

func TestTheStatusComesFromAClassNotFromWords(t *testing.T) {
	// The words are translated and the class is not: Moodle builds the class
	// from the database value. A site in another language must read the same.
	page := fixture(t, "draft")
	// Replace every visible word with something else, leaving the markup.
	translated := strings.NewReplacer(
		"Draft (not submitted)", "草稿（尚未提交）",
		"Submission status", "繳交狀態",
		"Grading status", "評分狀態",
		"Not graded", "尚未評分",
	).Replace(page)

	got, err := webread.ParseAssignStatus(translated)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "draft" {
		t.Errorf("status = %q on a translated page, want draft", got.Status)
	}
}

func TestAPrefixDoesNotMatchAClassThatMerelyStartsWithIt(t *testing.T) {
	// Moodle's classes share prefixes: "submissionstatustable" begins with
	// "submissionstatus". A cell carrying such a class must not be read as a
	// status of "table".
	page := `<html><body><div class="submissionstatustable"><table>` +
		`<tr><td class="submissionstatustable">x</td></tr>` +
		`</table></div></body></html>`

	got, err := webread.ParseAssignStatus(page)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "new" {
		t.Errorf("status = %q; a container class was read as a status", got.Status)
	}
}

func TestGradedAndLockedAreReadWhenPresent(t *testing.T) {
	// Both are class tokens Moodle sets from state, so a synthetic page is
	// enough to prove they are read; the real pages above prove the anchors.
	page := `<html><body><div class="submissionstatustable"><table>` +
		`<tr><td class="submissionstatussubmitted">x</td></tr>` +
		`<tr><td class="submissiongraded">x</td></tr>` +
		`<tr><td class="submissionlocked">x</td></tr>` +
		`</table></div></body></html>`

	got, err := webread.ParseAssignStatus(page)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "submitted" || !got.Graded || !got.Locked {
		t.Errorf("got %+v", got)
	}
}
