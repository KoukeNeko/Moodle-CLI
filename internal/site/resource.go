package site

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// ResourceKind is what a Moodle address points at.
type ResourceKind string

const (
	ResourceCourse     ResourceKind = "course"
	ResourceActivity   ResourceKind = "activity"
	ResourceDiscussion ResourceKind = "discussion"
	ResourceFile       ResourceKind = "file"
	ResourceUser       ResourceKind = "user"
	ResourceGrades     ResourceKind = "grades"
	ResourceCalendar   ResourceKind = "calendar"
	// ResourceUnknown is a Moodle address this build does not recognise. It is
	// kept distinct from an error: the address may be perfectly valid and
	// simply be for something not covered yet.
	ResourceUnknown ResourceKind = "unknown"
)

// Resource is what a Moodle address refers to.
//
// Working this out is pure parsing: no request is made, and nothing here is
// checked against a site. A link can be read while offline, and reading one
// should not announce to the site that you have it.
type Resource struct {
	Kind ResourceKind
	// Module is the activity type, such as "assign" or "forum".
	Module string
	// CMID is the course module id, which is what an activity's address
	// carries. It is NOT the activity's own id: `moodle assignment show` takes
	// an assignment id, and getting from one to the other needs a lookup.
	CMID         string
	CourseID     string
	UserID       string
	DiscussionID string
	// FileName is the last segment of a file address.
	FileName string
}

// ParseResourceURL works out what a Moodle address points at.
//
// It accepts any Moodle address, not only one belonging to a configured site:
// deciding what a link is and deciding whether to fetch it are separate
// questions, and only the second one needs to care whose site it is.
func ParseResourceURL(raw string) (Resource, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Resource{}, errs.New(errs.CodeUsage, "no URL given")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return Resource{}, errs.Wrap(errs.CodeUsage, err, "that is not a URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Resource{}, errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is not an http or https URL", raw))
	}

	query := parsed.Query()
	segments := splitPath(parsed.Path)
	resource := Resource{Kind: ResourceUnknown}

	switch {
	// /mod/<module>/view.php?id=<cmid> — the shape every activity uses.
	case len(segments) >= 3 && segments[0] == "mod" && segments[2] == "view.php":
		resource.Kind = ResourceActivity
		resource.Module = segments[1]
		resource.CMID = digits(query.Get("id"))
		resource.CourseID = digits(query.Get("course"))

	// /mod/forum/discuss.php?d=<discussion>
	case len(segments) >= 3 && segments[0] == "mod" && segments[2] == "discuss.php":
		resource.Kind = ResourceDiscussion
		resource.Module = segments[1]
		resource.DiscussionID = digits(query.Get("d"))

	case len(segments) >= 2 && segments[0] == "course" && segments[1] == "view.php":
		resource.Kind = ResourceCourse
		resource.CourseID = digits(query.Get("id"))

	case len(segments) >= 2 && segments[0] == "user" && segments[1] == "view.php":
		resource.Kind = ResourceUser
		resource.UserID = digits(query.Get("id"))
		resource.CourseID = digits(query.Get("course"))

	case len(segments) >= 1 && segments[0] == "grade":
		resource.Kind = ResourceGrades
		resource.CourseID = digits(query.Get("id"))

	case len(segments) >= 1 && segments[0] == "calendar":
		resource.Kind = ResourceCalendar
		resource.CourseID = digits(query.Get("course"))

	// /webservice/pluginfile.php/<context>/<component>/<area>/<item>/<name>
	// and the /pluginfile.php and /tokenpluginfile.php variants.
	case isFilePath(segments):
		resource.Kind = ResourceFile
		if len(segments) > 0 {
			// The name can contain anything a filename can, including slashes
			// once decoded, so only the final segment is taken and it is not
			// treated as a path.
			resource.FileName = segments[len(segments)-1]
		}
	}

	if resource.Kind == ResourceActivity && resource.CMID == "" {
		return Resource{}, errs.New(errs.CodeUsage,
			fmt.Sprintf("%q looks like an activity address but carries no id", raw))
	}
	return resource, nil
}

// isFilePath reports whether the address is one of Moodle's file endpoints.
func isFilePath(segments []string) bool {
	for i, segment := range segments {
		switch segment {
		case "pluginfile.php", "tokenpluginfile.php", "webservice":
			// webservice only counts when pluginfile.php follows it.
			if segment == "webservice" {
				return i+1 < len(segments) &&
					strings.HasSuffix(segments[i+1], "pluginfile.php")
			}
			return true
		}
	}
	return false
}

func splitPath(raw string) []string {
	cleaned := path.Clean("/" + strings.TrimSpace(raw))
	parts := strings.Split(strings.Trim(cleaned, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		// A path segment can be percent-encoded; compare the real characters.
		if decoded, err := url.PathUnescape(part); err == nil {
			part = decoded
		}
		out = append(out, part)
	}
	return out
}

// digits keeps a value only when it is a plain non-negative integer.
//
// Moodle ids are integers. Anything else in that position is a malformed or
// hand-edited address, and carrying it forward would let it reach a request.
func digits(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value < 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
