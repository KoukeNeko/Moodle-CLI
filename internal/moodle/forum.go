package moodle

import (
	"context"
	"html"
	"sort"
	"strconv"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Forum web service functions. Only reads: mod_forum_view_* records a view and
// can complete an activity, so none of those are called.
const (
	FunctionForums          = "mod_forum_get_forums_by_courses"
	FunctionForumDiscussion = "mod_forum_get_forum_discussions"
	FunctionForumPosts      = "mod_forum_get_discussion_posts"
)

// forumDTO is Moodle's reply to mod_forum_get_forums_by_courses.
type forumDTO struct {
	ID             int64  `json:"id"`
	CMID           int64  `json:"cmid"`
	Course         int64  `json:"course"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	Intro          string `json:"intro"`
	NumDiscussions int    `json:"numdiscussions"`
}

// discussionsDTO is Moodle's reply to mod_forum_get_forum_discussions.
type discussionsDTO struct {
	Discussions []struct {
		// Discussion is the thread's id. The "id" field beside it is the
		// opening post's id, which is a different number.
		Discussion   int64  `json:"discussion"`
		Name         string `json:"name"`
		UserFullName string `json:"userfullname"`
		// UserModifiedFullName is whoever replied last.
		UserModifiedFullName string `json:"usermodifiedfullname"`
		Created              int64  `json:"created"`
		TimeModified         int64  `json:"timemodified"`
		NumReplies           int    `json:"numreplies"`
		NumUnread            int    `json:"numunread"`
		Pinned               bool   `json:"pinned"`
		Locked               bool   `json:"locked"`
		CanReply             bool   `json:"canreply"`
	} `json:"discussions"`
}

// postsDTO is Moodle's reply to mod_forum_get_discussion_posts.
type postsDTO struct {
	Posts []struct {
		ID            int64  `json:"id"`
		Subject       string `json:"subject"`
		Message       string `json:"message"`
		MessageFormat int    `json:"messageformat"`
		DiscussionID  int64  `json:"discussionid"`
		// ParentID is null for the post that opened the thread.
		ParentID    *int64 `json:"parentid"`
		TimeCreated int64  `json:"timecreated"`
		IsDeleted   bool   `json:"isdeleted"`
		Author      *struct {
			FullName  string `json:"fullname"`
			IsDeleted bool   `json:"isdeleted"`
		} `json:"author"`
		Attachments []fileDTO `json:"attachments"`
		URLs        *struct {
			View string `json:"view"`
		} `json:"urls"`
	} `json:"posts"`
}

// ForumBackend reads forums over the web service API.
type ForumBackend struct {
	route route
}

// NewForumBackend builds the web service backend for forums.
func NewForumBackend(client *Client, token string) *ForumBackend {
	return &ForumBackend{route: wsRoute{client: client, token: token}}
}

// NewForumAjaxBackend builds the browser-session backend.
//
// Only reading a thread works over this route: listing forums and their
// discussions is not exposed there, and each will say so when asked.
func NewForumAjaxBackend(session *AjaxSession) *ForumBackend {
	return &ForumBackend{route: ajaxRoute{session: session}}
}

func (b *ForumBackend) Name() site.BackendKind { return b.route.kind() }

func (b *ForumBackend) Requirement() site.Requirement {
	return b.route.requirement([]string{FunctionForums})
}

func (b *ForumBackend) List(ctx context.Context, courseIDs []string) (forum.ListResult, error) {
	params := map[string]any{}
	if len(courseIDs) > 0 {
		ids := make([]any, 0, len(courseIDs))
		for _, raw := range courseIDs {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return forum.ListResult{}, errs.New(errs.CodeUsage,
					"course id "+raw+" is not a number")
			}
			ids = append(ids, id)
		}
		params["courseids"] = ids
	}

	var dto []forumDTO
	if err := b.route.call(ctx, FunctionForums, params, &dto); err != nil {
		return forum.ListResult{}, err
	}

	result := forum.ListResult{
		Forums:     []forum.Forum{},
		Provenance: site.NewProvenance(b.route.kind()),
	}
	for _, item := range dto {
		result.Forums = append(result.Forums, forum.Forum{
			ID:   strconv.FormatInt(item.ID, 10),
			CMID: strconv.FormatInt(item.CMID, 10),
			Kind: item.Type,
			// This function HTML-escapes the name and its sibling
			// get_forum_discussions does not, so a forum called "Q&A" arrives
			// as "Q&amp;A" here and "Q&A" there. Undoing it is what makes the
			// two agree.
			Name:        html.UnescapeString(item.Name),
			Description: item.Intro,
			CourseID:    strconv.FormatInt(item.Course, 10),
			Discussions: item.NumDiscussions,
		})
	}
	return result, nil
}

func (b *ForumBackend) Discussions(ctx context.Context, forumID string) (forum.DiscussionsResult, error) {
	id, err := strconv.ParseInt(forumID, 10, 64)
	if err != nil {
		return forum.DiscussionsResult{}, errs.New(errs.CodeUsage,
			"forum id "+forumID+" is not a number")
	}

	var dto discussionsDTO
	if err := b.route.call(ctx, FunctionForumDiscussion,
		map[string]any{"forumid": id}, &dto); err != nil {
		return forum.DiscussionsResult{}, err
	}

	result := forum.DiscussionsResult{
		ForumID:     forumID,
		Discussions: []forum.Discussion{},
		Provenance:  site.NewProvenance(b.route.kind()),
	}
	for _, item := range dto.Discussions {
		result.Discussions = append(result.Discussions, forum.Discussion{
			ID:         strconv.FormatInt(item.Discussion, 10),
			Name:       item.Name,
			ForumID:    forumID,
			Author:     item.UserFullName,
			LastAuthor: item.UserModifiedFullName,
			CreatedAt:  unixTime(item.Created),
			ModifiedAt: unixTime(item.TimeModified),
			Replies:    item.NumReplies,
			Unread:     item.NumUnread,
			Pinned:     item.Pinned,
			Locked:     item.Locked,
			CanReply:   item.CanReply,
		})
	}
	return result, nil
}

func (b *ForumBackend) Thread(ctx context.Context, discussionID string) (forum.ThreadResult, error) {
	id, err := strconv.ParseInt(discussionID, 10, 64)
	if err != nil {
		return forum.ThreadResult{}, errs.New(errs.CodeUsage,
			"discussion id "+discussionID+" is not a number")
	}

	var dto postsDTO
	if err := b.route.call(ctx, FunctionForumPosts,
		map[string]any{"discussionid": id}, &dto); err != nil {
		return forum.ThreadResult{}, err
	}
	if len(dto.Posts) == 0 {
		// A thread the account cannot read is answered with a successful call
		// and an empty list, not an error: a separate-groups forum does this to
		// anyone outside the group, and a Q&A forum to anyone who has not
		// posted yet. Passing the empty list on would say "nothing was written
		// here" about a thread that was never read.
		//
		// Nothing is lost by refusing. A discussion is created around its
		// opening post and keeps it, so an empty list cannot be an empty
		// thread, and a discussion that does not exist is an error rather than
		// an empty reply.
		return forum.ThreadResult{}, errs.New(errs.CodePermissionDenied,
			"this account cannot read discussion "+discussionID).
			WithHint("the site returned it without the opening post every discussion has")
	}

	result := forum.ThreadResult{
		DiscussionID: discussionID,
		Posts:        []forum.Post{},
		Provenance:   site.NewProvenance(b.route.kind()),
	}
	for _, item := range dto.Posts {
		post := forum.Post{
			ID:            strconv.FormatInt(item.ID, 10),
			Subject:       item.Subject,
			Message:       item.Message,
			MessageFormat: item.MessageFormat,
			CreatedAt:     unixTime(item.TimeCreated),
			Deleted:       item.IsDeleted,
			Attachments:   fileRefs(item.Attachments),
		}
		if item.ParentID != nil && *item.ParentID != 0 {
			post.ParentID = strconv.FormatInt(*item.ParentID, 10)
		}
		if item.Author != nil {
			post.Author = item.Author.FullName
		}
		if item.URLs != nil {
			post.URL = safeURL(item.URLs.View)
		}
		result.Posts = append(result.Posts, post)
	}

	orderForReading(result.Posts)
	return result, nil
}

// orderForReading puts a thread in the order a person reads it.
//
// Moodle returns posts newest first, so the reply arrives before the question
// it answers. Anyone reading top to bottom then meets the thread backwards.
// Oldest first, with the opening post first whatever its timestamp says.
func orderForReading(posts []forum.Post) {
	sort.SliceStable(posts, func(i, j int) bool {
		if posts[i].Opening() != posts[j].Opening() {
			return posts[i].Opening()
		}
		left, right := posts[i].CreatedAt, posts[j].CreatedAt
		if left != nil && right != nil && !left.Equal(*right) {
			return left.Before(*right)
		}
		// Same second, or no timestamp: ids ascend with time, so they settle
		// it rather than leaving the order to chance.
		return numericLess(posts[i].ID, posts[j].ID)
	})
}

func numericLess(left, right string) bool {
	a, errA := strconv.ParseInt(left, 10, 64)
	b, errB := strconv.ParseInt(right, 10, 64)
	if errA != nil || errB != nil {
		return left < right
	}
	return a < b
}
