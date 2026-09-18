// Package file covers the files Moodle hands out and getting them onto disk.
//
// Downloading is where several of Moodle's habits meet at once: the credential
// travels in the URL rather than a header, a failure arrives as HTTP 200 with
// a JSON body, and the filename is suggested by the server. Each of those is a
// way to end up with the wrong bytes in the wrong place under a name of
// someone else's choosing, so each is handled explicitly here.
package file

import (
	"context"
	"io"
	"time"
)

// Ref is a file Moodle told us about.
type Ref struct {
	Name string
	// Path is the folder within the file area, "/" at the top. Moodle allows
	// a hierarchy, and a name alone can collide.
	Path       string
	Size       int64
	URL        string
	MIMEType   string
	ModifiedAt *time.Time
	// External marks a file held by another service, such as a linked cloud
	// drive. The site's own token does not necessarily open it.
	External bool
}

// Body is an open download.
type Body struct {
	Content io.ReadCloser
	// SuggestedName is what the server said to call it. It is a suggestion
	// from a remote party and is never used as a path.
	SuggestedName string
	Size          int64
	MIMEType      string
}

// Fetcher opens a file for reading. The transport implements it.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) (*Body, error)
}
