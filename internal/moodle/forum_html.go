package moodle

import (
	"context"

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
	return result, nil
}

// Discussions and Thread have no page route here.
//
// They could be read from the forum's own pages, but nothing in this project
// needs them yet on a site without web services, and a parser with no fixture
// behind it is a guess. Refusing says which route was missing; returning an
// empty list would say the forum has no threads.
func (b *ForumHTMLBackend) Discussions(_ context.Context, _ string) (forum.DiscussionsResult, error) {
	return forum.DiscussionsResult{}, errs.New(errs.CodeUnavailable,
		"reading pages cannot list a forum's discussions").
		WithReason(errs.ReasonCapability)
}

func (b *ForumHTMLBackend) Thread(_ context.Context, _ string) (forum.ThreadResult, error) {
	return forum.ThreadResult{}, errs.New(errs.CodeUnavailable,
		"reading pages cannot read a discussion").
		WithReason(errs.ReasonCapability)
}
