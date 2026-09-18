// Package forum reads course discussions.
//
// Reading only. Moodle's own "mark as viewed" calls change things — they can
// complete an activity — so none of them are made here: reading a discussion
// from a terminal should not tell the site you have read it.
package forum

import (
	"context"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/file"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Forum is one discussion area in a course.
type Forum struct {
	ID   string
	CMID string
	// Kind is Moodle's forum type: "general", "news", "qanda" and so on. It
	// decides who may post, so it is passed through rather than flattened.
	Kind        string
	Name        string
	Description string
	CourseID    string
	// Discussions is how many threads the site reports, which is not always
	// the number a listing returns: a site can hide some from this account.
	Discussions int
}

// Discussion is one thread.
type Discussion struct {
	// ID is the discussion's own id, which is what reading a thread needs.
	//
	// Moodle sends it in a field called "discussion"; the "id" alongside it is
	// the opening post's id. They are both small integers, so using the wrong
	// one reads a different thread, or none.
	ID      string
	Name    string
	ForumID string
	// Author is who started the thread, and LastAuthor who last replied.
	Author     string
	LastAuthor string
	CreatedAt  *time.Time
	ModifiedAt *time.Time
	Replies    int
	Unread     int
	Pinned     bool
	Locked     bool
	// CanReply is Moodle's own verdict, which accounts for the forum type, the
	// lock and the cut-off in one answer.
	CanReply bool
}

// Post is one message in a thread.
type Post struct {
	ID      string
	Subject string
	// Message is Moodle's own markup, unchanged. MessageFormat says how to
	// read it.
	Message       string
	MessageFormat int
	Author        string
	// ParentID is empty for the post that opened the thread.
	ParentID  string
	CreatedAt *time.Time
	// Deleted marks a post Moodle is withholding. Its content is gone but the
	// post remains, and hiding it entirely would lose the shape of the thread.
	Deleted     bool
	Attachments []file.Ref
	// URL points at the post, with anything carrying a secret dropped.
	URL string
}

// Opening reports whether this post started the thread.
func (p Post) Opening() bool { return p.ParentID == "" }

// ListResult is a set of forums plus where it came from.
type ListResult struct {
	Forums     []Forum
	Provenance site.Provenance
}

// DiscussionsResult is a set of threads.
type DiscussionsResult struct {
	ForumID     string
	Discussions []Discussion
	Provenance  site.Provenance
}

// ThreadResult is one thread's posts, in reading order.
type ThreadResult struct {
	DiscussionID string
	Posts        []Post
	Provenance   site.Provenance
}

// Backend is one way of reading forums.
type Backend interface {
	Name() site.BackendKind
	Requirement() site.Requirement
	List(ctx context.Context, courseIDs []string) (ListResult, error)
	Discussions(ctx context.Context, forumID string) (DiscussionsResult, error)
	Thread(ctx context.Context, discussionID string) (ThreadResult, error)
}
