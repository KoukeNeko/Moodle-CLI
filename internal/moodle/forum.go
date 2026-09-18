package moodle

import (
	"context"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Forum web service functions. Only reads: mod_forum_view_* records a view and
// can complete an activity, so none of those are called.
const (
	FunctionForums = "mod_forum_get_forums_by_courses"
	// FunctionNavigationOptions is not a forum call. It is the only cheap way
	// to ask whether this account can reach a course at all, which is what an
	// empty forum listing leaves open.
	FunctionNavigationOptions = "core_course_get_user_navigation_options"
	FunctionForumDiscussion   = "mod_forum_get_forum_discussions"
	FunctionForumPosts        = "mod_forum_get_discussion_posts"
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
	// ForumID is the thread's forum, which the caller did not have to know.
	// It is what makes the reply-count check below possible without asking the
	// reader for something they were not holding.
	ForumID int64 `json:"forumid"`
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
	if len(result.Forums) == 0 && len(courseIDs) > 0 {
		// Only now, and only for a question about named courses: a listing
		// that returned something has already answered, and an empty listing
		// with no course named is a different question.
		if err := b.refuseUnreadableCourses(ctx, courseIDs); err != nil {
			return forum.ListResult{}, err
		}
	}
	return result, nil
}

// navigationOptionsDTO is Moodle's reply to
// core_course_get_user_navigation_options. Only the shape that says which
// courses were readable is mapped; the options themselves are not wanted.
type navigationOptionsDTO struct {
	Courses []struct {
		ID int64 `json:"id"`
	} `json:"courses"`
	Warnings []struct {
		Item        string `json:"item"`
		ItemID      int64  `json:"itemid"`
		WarningCode string `json:"warningcode"`
	} `json:"warnings"`
}

// refuseUnreadableCourses reports the courses this account cannot reach.
//
// mod_forum_get_forums_by_courses computes warnings and then drops them: its
// returns declaration has no field for them, and the source says so. An empty
// reply is therefore three different answers at once — the course is
// unreachable, every forum in it is hidden from this account, or there are
// none — and nothing in the reply separates them.
//
// This asks a different function the same question. It is worth a round trip
// only because the listing already came back empty, and it answers the one
// part that can be answered: get_user_navigation_options runs
// validate_context() rather than checking enrolments, so a manager holding no
// enrolment is reported as able to read the course, which is what ruled out
// deciding this from the account's own course list.
//
// A probe that does not answer leaves the question open rather than closing
// it the wrong way: only a warning naming a course is treated as a refusal.
func (b *ForumBackend) refuseUnreadableCourses(ctx context.Context, courseIDs []string) error {
	ids := make([]any, 0, len(courseIDs))
	for _, raw := range courseIDs {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil
		}
		ids = append(ids, id)
	}

	var dto navigationOptionsDTO
	if err := b.route.call(ctx, FunctionNavigationOptions,
		map[string]any{"courseids": ids}, &dto); err != nil {
		// The site may not offer this function, or the route may not reach it.
		// Either way nothing has been learned, and the listing stands.
		return nil
	}

	var unreadable []string
	for _, warning := range dto.Warnings {
		if warning.Item != "course" {
			continue
		}
		unreadable = append(unreadable, strconv.FormatInt(warning.ItemID, 10))
	}
	if len(unreadable) == 0 {
		return nil
	}
	return errs.New(errs.CodePermissionDenied,
		"this account cannot read course "+strings.Join(unreadable, ", ")).
		WithHint("the forum listing came back empty because the course is out of reach, " +
			"not because it holds no forums")
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
	if len(result.Posts) == 1 {
		// Exactly one post is the shape a Q&A forum produces for someone who
		// has not posted yet: it shows the question and withholds every
		// answer. It is also the shape of a thread nobody has replied to, and
		// the two read identically — a student concludes nobody answered.
		//
		// Only this shape is worth a round trip. Once an account can see any
		// reply it can see them all, so a thread that came back with more than
		// one post is not being filtered this way.
		if withheld := b.withheldPosts(ctx, dto.ForumID, id, len(result.Posts)); withheld > 0 {
			result.WithheldPosts = withheld
			result.Provenance.Partial = true
		}
	}
	return result, nil
}

// withheldPosts reports how many posts the site kept back from this thread.
//
// Nothing in the posts reply says any were: the warnings array is empty, the
// key set is identical, and the posts that arrive carry the same capabilities
// as a full reading — measured. The discussion listing does know, because its
// numreplies counts the thread rather than what this account may read.
//
// Zero is both "none were withheld" and "could not tell". Only a count the
// site actually supports is acted on.
func (b *ForumBackend) withheldPosts(ctx context.Context, forumID, discussionID int64, got int) int {
	if forumID == 0 {
		return 0
	}
	var dto discussionsDTO
	if err := b.route.call(ctx, FunctionForumDiscussion,
		map[string]any{"forumid": forumID}, &dto); err != nil {
		return 0
	}
	for _, item := range dto.Discussions {
		if item.Discussion != discussionID {
			continue
		}
		// The opening post is not a reply, so a thread holds numreplies + 1.
		if missing := item.NumReplies + 1 - got; missing > 0 {
			return missing
		}
		return 0
	}
	return 0
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
