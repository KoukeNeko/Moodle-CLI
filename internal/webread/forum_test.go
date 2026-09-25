package webread_test

import (
	"os"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

func TestDiscussionsAreReadFromTheForumPage(t *testing.T) {
	// The shape of a Moodle 4.5 forum page in Traditional Chinese.
	raw, err := os.ReadFile("testdata/forum.html")
	if err != nil {
		t.Fatal(err)
	}
	page, err := webread.ParseForumPage(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Discussions) != 2 {
		t.Fatalf("got %d discussions, want 2", len(page.Discussions))
	}
	first := page.Discussions[0]
	if first.ID != "260280" || first.Name != "[公告] 期末考成績公告" {
		t.Errorf("first = %+v", first)
	}
	if first.Author != "助教 甲" || first.LastAuthor != "教授 乙" {
		t.Errorf("authors = %q / %q", first.Author, first.LastAuthor)
	}
	if first.CreatedAt == nil || first.CreatedAt.Unix() != 1782371289 {
		t.Errorf("created = %v", first.CreatedAt)
	}
	if !first.Pinned || first.Locked {
		t.Errorf("pinned=%v locked=%v; the lock label is there but hidden", first.Pinned, first.Locked)
	}
	if second := page.Discussions[1]; second.Pinned || !second.Locked {
		t.Errorf("second pinned=%v locked=%v", second.Pinned, second.Locked)
	}
	if page.MorePages {
		t.Error("a page with no paging bar reported more pages")
	}
}

func TestAPageWithoutTheListIsNotAnEmptyForum(t *testing.T) {
	_, err := webread.ParseForumPage(`<html><body><p>something else</p></body></html>`)
	if err == nil {
		t.Fatal("a page with no discussion list read as an empty forum")
	}
	if errs.From(err).Reason != errs.ReasonProtocolDrift {
		t.Errorf("reason = %q", errs.From(err).Reason)
	}
}
