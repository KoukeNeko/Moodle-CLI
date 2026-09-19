package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/forum"
)

// Forum is one discussion area on the wire.
type Forum struct {
	ID   string `json:"id"`
	CMID string `json:"cmid"`
	// Kind is Moodle's forum type: general, news, qanda and so on. It decides
	// who may post, so it is passed through rather than flattened away.
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CourseID    string `json:"course_id"`
	// Discussions is the count the site reports, which is not always the
	// number a listing returns: a site can hide some from this account.
	// Discussions is null when the site did not report a count.
	Discussions *int `json:"discussions"`
}

// ForumList converts a listing into its envelope.
func ForumList(result forum.ListResult, siteName, accountName string) Envelope {
	forums := make([]Forum, 0, len(result.Forums))
	for _, item := range result.Forums {
		forums = append(forums, Forum{
			ID:          item.ID,
			CMID:        item.CMID,
			Kind:        item.Kind,
			Name:        item.Name,
			Description: item.Description,
			CourseID:    item.CourseID,
			Discussions: item.Discussions,
		})
	}
	return NewEnvelope("forum.list", forums,
		MetaFrom(result.Provenance, siteName, accountName))
}

// Discussion is one thread on the wire.
type Discussion struct {
	// ID is the discussion's own id, which is what reading a thread needs.
	// Moodle sends it beside the opening post's id, and the two are different
	// numbers.
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ForumID    string  `json:"forum_id"`
	Author     string  `json:"author"`
	LastAuthor string  `json:"last_author"`
	CreatedAt  *string `json:"created_at"`
	ModifiedAt *string `json:"modified_at"`
	Replies    int     `json:"replies"`
	Unread     int     `json:"unread"`
	Pinned     bool    `json:"pinned"`
	Locked     bool    `json:"locked"`
	// CanReply is Moodle's own verdict, which folds the forum type, the lock
	// and any cut-off into one answer.
	CanReply bool `json:"can_reply"`
}

// ForumDiscussions converts a thread listing into its envelope.
func ForumDiscussions(result forum.DiscussionsResult, siteName, accountName string) Envelope {
	discussions := make([]Discussion, 0, len(result.Discussions))
	for _, item := range result.Discussions {
		discussions = append(discussions, Discussion{
			ID:         item.ID,
			Name:       item.Name,
			ForumID:    item.ForumID,
			Author:     item.Author,
			LastAuthor: item.LastAuthor,
			CreatedAt:  Timestamp(item.CreatedAt),
			ModifiedAt: Timestamp(item.ModifiedAt),
			Replies:    item.Replies,
			Unread:     item.Unread,
			Pinned:     item.Pinned,
			Locked:     item.Locked,
			CanReply:   item.CanReply,
		})
	}
	return NewEnvelope("forum.discussions", discussions,
		MetaFrom(result.Provenance, siteName, accountName))
}

// Post is one message on the wire.
type Post struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	// Message is Moodle's own markup, unchanged; message_format says how to
	// read it.
	Message       string `json:"message"`
	MessageFormat int    `json:"message_format"`
	Author        string `json:"author"`
	// ParentID is null for the post that opened the thread.
	ParentID  *string `json:"parent_id"`
	CreatedAt *string `json:"created_at"`
	// Deleted marks a post Moodle is withholding. Its content is gone but the
	// post stays, because removing it would lose the shape of the thread.
	Deleted     bool    `json:"deleted"`
	Attachments []File  `json:"attachments"`
	URL         *string `json:"url"`
}

// ForumThread converts one thread into its envelope.
//
// The posts are in reading order — opening post first, then oldest to newest.
// Moodle returns them newest first, which puts every reply before the thing it
// replies to.
func ForumThread(result forum.ThreadResult, siteName, accountName string) Envelope {
	posts := make([]Post, 0, len(result.Posts))
	for _, item := range result.Posts {
		post := Post{
			ID:            item.ID,
			Subject:       item.Subject,
			Message:       item.Message,
			MessageFormat: item.MessageFormat,
			Author:        item.Author,
			ParentID:      optional(item.ParentID),
			CreatedAt:     Timestamp(item.CreatedAt),
			Deleted:       item.Deleted,
			Attachments:   newFiles(item.Attachments),
			URL:           optional(item.URL),
		}
		posts = append(posts, post)
	}
	return NewEnvelope("forum.thread", posts,
		MetaFrom(result.Provenance, siteName, accountName))
}
