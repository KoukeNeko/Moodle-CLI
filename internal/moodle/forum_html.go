package moodle

import (
	"context"
	"strconv"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
	"github.com/KoukeNeko/moodle-cli/internal/site"
	"github.com/KoukeNeko/moodle-cli/internal/webread"
)

// ForumHTMLBackend lists forums from Moodle's own course pages.
//
// It is the last route and the only one left on a site with mobile web
// services switched off: mod_forum_get_forums_by_courses answers
// servicenotavailable over the AJAX endpoint — measured on 4.5, 5.1 and 5.2.
//
// Identity works as it does for assignments read this way: a course page
// carries the course module id and nothing else, so that is the id reported.
// It is the number in the address bar, not the forum's own id, and the two
// routes therefore name the same forum differently. Saying so is better than
// inventing an id the page never carried.
type ForumHTMLBackend struct {
	pages *PageReader
	// courses lists the courses to look in. A page cannot enumerate
	// enrolments, so the caller supplies that.
	courses func(ctx context.Context) ([]string, error)
}

// NewForumHTMLBackend builds the page-reading backend for forums.
func NewForumHTMLBackend(pages *PageReader, courses func(context.Context) ([]string, error)) *ForumHTMLBackend {
	return &ForumHTMLBackend{pages: pages, courses: courses}
}

func (b *ForumHTMLBackend) Name() site.BackendKind { return site.BackendHTML }

// Requirement is empty: a page needs no function to be exposed.
func (b *ForumHTMLBackend) Requirement() site.Requirement {
	return site.Requirement{}
}

func (b *ForumHTMLBackend) List(ctx context.Context, courseIDs []string) (forum.ListResult, error) {
	if len(courseIDs) == 0 {
		if b.courses == nil {
			return forum.ListResult{}, errs.New(errs.CodeUsage,
				"reading pages cannot find your courses on its own").
				WithHint("name the course with --course")
		}
		found, err := b.courses(ctx)
		if err != nil {
			return forum.ListResult{}, err
		}
		courseIDs = found
	}

	result := forum.ListResult{
		Forums:     []forum.Forum{},
		Provenance: site.NewProvenance(site.BackendHTML),
	}
	for _, courseID := range courseIDs {
		page, err := b.pages.Get(ctx, PathCourseView, map[string]string{"id": courseID})
		if err != nil {
			return forum.ListResult{}, err
		}
		activities, err := webread.ParseCourseActivities(page)
		if err != nil {
			return forum.ListResult{}, err
		}
		for _, activity := range activities {
			if activity.Module != "forum" {
				continue
			}
			result.Forums = append(result.Forums, forum.Forum{
				// Both are the course module id: it is the only identifier the
				// page carries, and pretending otherwise would invent one.
				ID:       activity.CMID,
				CMID:     activity.CMID,
				Name:     activity.Name,
				CourseID: courseID,
				// Kind and Discussions are left unset on purpose. A course page
				// says neither which sort of forum this is nor how many threads
				// it holds, and a zero count reads as an empty forum.
			})
		}
	}
	result.Provenance.Partial = true
	result.Provenance.Missing = []string{"kind", "discussions", "description"}
	return result, nil
}

// PathForumView is the page listing a forum's threads.
const PathForumView = "/mod/forum/view.php"

// Discussions reads the thread list off the forum's page.
//
// The forum id on this route is the course module id, as List reports it,
// which is also what the page's address takes. The reply counts sit in a cell
// with no marker of their own, and the rest are this account's standing in
// the thread, which the page does not state in a form worth reading; they are
// reported as missing rather than as zero.
func (b *ForumHTMLBackend) Discussions(ctx context.Context, forumID string) (forum.DiscussionsResult, error) {
	result := forum.DiscussionsResult{
		ForumID:     forumID,
		Discussions: []forum.Discussion{},
		Provenance:  site.NewProvenance(site.BackendHTML),
	}
	seen := map[string]bool{}
	// A forum shows a hundred threads a page by default. Past that the page
	// carries a paging bar, and view.php takes the page number as p; reading
	// stops at the first page that adds nothing new.
	for pageNumber := 0; pageNumber < maxForumPages; pageNumber++ {
		markup, err := b.pages.Get(ctx, PathForumView,
			map[string]string{"id": forumID, "p": strconv.Itoa(pageNumber)})
		if err != nil {
			return forum.DiscussionsResult{}, err
		}
		page, err := webread.ParseForumPage(markup)
		if err != nil {
			return forum.DiscussionsResult{}, err
		}
		added := 0
		for _, row := range page.Discussions {
			if seen[row.ID] {
				continue
			}
			seen[row.ID] = true
			added++
			result.Discussions = append(result.Discussions, forum.Discussion{
				ID: row.ID, Name: row.Name, ForumID: forumID,
				Author: row.Author, LastAuthor: row.LastAuthor,
				CreatedAt: row.CreatedAt, ModifiedAt: row.ModifiedAt,
				Pinned: row.Pinned, Locked: row.Locked,
			})
		}
		if !page.MorePages || added == 0 {
			break
		}
	}
	result.Provenance.Partial = true
	result.Provenance.Missing = []string{"replies", "unread", "can_reply"}
	return result, nil
}

// maxForumPages bounds one listing, at a hundred threads a page.
const maxForumPages = 20

// Thread has no page route: the AJAX endpoint offers
// mod_forum_get_discussion_posts to a browser session, and that route comes
// first. Refusing says which route was missing; returning nothing would say
// the thread is empty.
func (b *ForumHTMLBackend) Thread(_ context.Context, _ string) (forum.ThreadResult, error) {
	return forum.ThreadResult{}, errs.New(errs.CodeUnavailable,
		"reading pages cannot read a discussion").
		WithReason(errs.ReasonCapability)
}
