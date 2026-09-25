package webread

import (
	"net/url"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Anchors on a course page.
const (
	// attrBlock marks a side block.
	attrBlock = "data-block"
	// classActivityName wraps the link to an activity.
	classActivityName = "activityname"
	// prefixModType carries the activity's type, as in "modtype_assign".
	prefixModType = "modtype_"
	// classInstanceName wraps the activity's own name.
	classInstanceName = "instancename"
	// classAccessHide marks text meant only for screen readers. Moodle puts
	// the activity type inside the name element, so it has to be skipped or
	// "Announcements" reads as "Announcements Forum".
	classAccessHide = "accesshide"
)

// Activity is one thing on a course page.
type Activity struct {
	// CMID is the course module id, which is the only identifier a page
	// carries. The activity's own id appears nowhere in the markup.
	CMID string
	// Module is the activity type, such as "assign" or "forum".
	Module string
	Name   string
	URL    string
}

// ParseCourseActivities reads course/view.php.
//
// It anchors on the link to each activity rather than on the course layout,
// which differs by course format — weekly, topics, single activity — while the
// links are the same in all of them.
func ParseCourseActivities(markup string) ([]Activity, error) {
	document, err := parse(markup)
	if err != nil {
		return nil, errs.Wrap(errs.CodeUpstream, err, "cannot read the course page").
			WithReason(errs.ReasonProtocolDrift)
	}

	// Side blocks link activities that are not the course's — measured: a
	// site-wide FAQ forum in an HTML block showed up in every course,
	// attributed to each in turn — so what sits inside a block is skipped.
	// Moodle marks every block with data-block, whatever the theme.
	var links []node
	for _, link := range document.findAll(byTag("a")) {
		if !link.inside(func(n node) bool { return n.attr(attrBlock) != "" }) {
			links = append(links, link)
		}
	}

	var activities []Activity
	seen := map[string]bool{}
	for _, link := range links {
		module, cmid := moduleLink(link.attr("href"))
		if module == "" || seen[cmid] {
			continue
		}
		name := activityName(link)
		if name == "" {
			continue
		}
		seen[cmid] = true
		activities = append(activities, Activity{
			CMID: cmid, Module: module, Name: name, URL: link.attr("href"),
		})
	}

	if len(activities) == 0 {
		// A course with nothing in it is possible, but so is a page this build
		// can no longer read, and the two must not look the same. The anchor
		// settles it: no activity link at all on a page that has the wrapper
		// means the shape changed.
		if _, ok := document.find(byClass(classActivityName)); ok {
			return nil, errs.New(errs.CodeUpstream,
				"the course page lists activities this build cannot read").
				WithReason(errs.ReasonProtocolDrift)
		}
	}
	return activities, nil
}

// moduleLink reads the activity type and course module id out of a link.
func moduleLink(href string) (module, cmid string) {
	if href == "" || !strings.Contains(href, "/mod/") {
		return "", ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return "", ""
	}
	// /mod/<module>/view.php
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if part != "mod" || i+2 >= len(parts) {
			continue
		}
		if parts[i+2] != "view.php" {
			continue
		}
		id := parsed.Query().Get("id")
		if id == "" {
			return "", ""
		}
		return parts[i+1], id
	}
	return "", ""
}

// activityName prefers the element Moodle wraps the name in.
//
// The link's full text also carries the screen-reader label — "Assignment" and
// the completion state — which is not part of the name and is translated.
func activityName(link node) string {
	if inner, ok := link.find(byClass(classInstanceName)); ok {
		return inner.textExcept(classAccessHide)
	}
	return link.textExcept(classAccessHide)
}
