package webread

import (
	"strconv"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Anchors on a forum page. Each is written by Moodle's discussion list
// template from data, for its own scripts to read.
const (
	// attrRegion names a region; discussion rows carry regionDiscussion.
	attrRegion       = "data-region"
	regionDiscussion = "discussion-list-item"
	regionLocked     = "locked-label"
	// attrDiscussionID is the discussion's own id on its row.
	attrDiscussionID = "data-discussionid"
	// classPinned marks a pinned row.
	classPinned = "pinned"
	// classAuthorInfo wraps a name and a time; the row has one for who
	// started the thread and one for who last posted.
	classAuthorInfo = "author-info"
	// classTruncate holds the name inside the author block.
	classTruncate = "text-truncate"
	// attrTimestamp is the Unix time on a time element.
	attrTimestamp = "data-timestamp"
	// classPagination marks a paging bar: more threads than one page shows.
	classPagination = "pagination"
	// idDiscussionList prefixes the list's wrapper.
	idDiscussionList = "discussion-list-"
)

// DiscussionRow is one thread on a forum page.
//
// The number of replies is on the row too, but only as a bare number in a
// cell with no marker of its own, so it is not read.
type DiscussionRow struct {
	ID         string
	Name       string
	Author     string
	LastAuthor string
	CreatedAt  *time.Time
	ModifiedAt *time.Time
	Pinned     bool
	Locked     bool
}

// ForumPage is what mod/forum/view.php lists.
type ForumPage struct {
	Discussions []DiscussionRow
	// MorePages reports a paging bar: the threads above are the first page.
	MorePages bool
}

// ParseForumPage reads the discussion list off mod/forum/view.php.
func ParseForumPage(markup string) (ForumPage, error) {
	document, err := parse(markup)
	if err != nil {
		return ForumPage{}, errs.Wrap(errs.CodeUpstream, err, "cannot read the forum page").
			WithReason(errs.ReasonProtocolDrift)
	}
	list, ok := document.find(func(n node) bool {
		return strings.HasPrefix(n.attr("id"), idDiscussionList)
	})
	if !ok {
		// A forum with no threads still renders the list's wrapper, so its
		// absence is a page this build cannot read, not an empty forum.
		return ForumPage{}, errs.New(errs.CodeUpstream,
			"this forum page carries no discussion list").
			WithReason(errs.ReasonProtocolDrift).
			WithHint("the site's pages may have changed shape, or this may not be a forum page")
	}

	var page ForumPage
	for _, row := range list.findAll(func(n node) bool { return n.attr(attrRegion) == regionDiscussion }) {
		id := row.attr(attrDiscussionID)
		if !isDigits(id) {
			continue
		}
		item := DiscussionRow{ID: id, Pinned: row.hasClass(classPinned)}
		if link, ok := row.find(func(n node) bool {
			return n.Data == "a" && strings.Contains(n.attr("href"), "discuss.php?d="+id)
		}); ok {
			item.Name = strings.TrimSpace(link.attr("title"))
			if item.Name == "" {
				item.Name = link.text()
			}
		}
		if label, ok := row.find(func(n node) bool { return n.attr(attrRegion) == regionLocked }); ok {
			item.Locked = !hasAttr(label, "hidden")
		}
		authors := row.findAll(byClass(classAuthorInfo))
		if len(authors) > 0 {
			item.Author = authorName(authors[0])
		}
		if len(authors) > 1 {
			item.LastAuthor = authorName(authors[1])
		}
		item.CreatedAt = timestamp(row, "time-created-"+id)
		item.ModifiedAt = timestamp(row, "time-modified-"+id)
		page.Discussions = append(page.Discussions, item)
	}
	_, page.MorePages = document.find(byClass(classPagination))
	return page, nil
}

func authorName(info node) string {
	if name, ok := info.find(byClass(classTruncate)); ok {
		return name.text()
	}
	return ""
}

func timestamp(row node, id string) *time.Time {
	element, ok := row.find(byID(id))
	if !ok {
		return nil
	}
	seconds, err := strconv.ParseInt(element.attr(attrTimestamp), 10, 64)
	if err != nil || seconds <= 0 {
		return nil
	}
	when := time.Unix(seconds, 0).UTC()
	return &when
}

func hasAttr(n node, name string) bool {
	for _, a := range n.Attr {
		if a.Key == name {
			return true
		}
	}
	return false
}
