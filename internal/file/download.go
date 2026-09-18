package file

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Request is a download the caller asked for.
type Request struct {
	URL string
	// Dir is where to put it. Empty means the working directory.
	Dir string
	// As overrides the name. Empty means the name is worked out from the
	// server's suggestion, then from the URL.
	As string
	// Overwrite allows replacing a file that is already there. Without it an
	// existing file is left alone: coursework is not something to clobber
	// because two files happen to share a name.
	Overwrite bool
}

// Result is what happened.
type Result struct {
	Path string
	Size int64
	// Replaced reports that something was already there and is now gone.
	Replaced bool
}

// Downloader writes a remote file to disk.
type Downloader struct {
	fetcher Fetcher
}

// NewDownloader builds a Downloader.
func NewDownloader(fetcher Fetcher) *Downloader { return &Downloader{fetcher: fetcher} }

// Download fetches a file and puts it on disk.
//
// The bytes go to a temporary file beside the destination and are renamed into
// place only once they have all arrived. A download interrupted halfway then
// leaves nothing behind rather than a truncated file with a plausible name —
// which is the worse outcome, because it looks like it worked.
func (d *Downloader) Download(ctx context.Context, req Request) (Result, error) {
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Result{}, errs.Wrap(errs.CodeUsage, err, fmt.Sprintf("cannot use %s", dir))
	}
	if !info.IsDir() {
		return Result{}, errs.New(errs.CodeUsage, fmt.Sprintf("%s is not a directory", dir))
	}

	body, err := d.fetcher.Fetch(ctx, req.URL)
	if err != nil {
		return Result{}, err
	}
	defer body.Content.Close()

	name, err := resolveName(req, body)
	if err != nil {
		return Result{}, err
	}
	target := filepath.Join(dir, name)

	existed := false
	if _, err := os.Stat(target); err == nil {
		existed = true
		if !req.Overwrite {
			return Result{}, errs.New(errs.CodeConflict,
				fmt.Sprintf("%s already exists", target)).
				WithHint("pass --force to replace it, or --as to save under another name")
		}
	}

	written, err := writeAtomically(dir, target, body.Content)
	if err != nil {
		return Result{}, err
	}
	return Result{Path: target, Size: written, Replaced: existed}, nil
}

// writeAtomically streams into a temporary file in the same directory and
// renames it into place.
//
// Same directory on purpose: a rename across filesystems is a copy, and stops
// being atomic.
func writeAtomically(dir, target string, content io.Reader) (int64, error) {
	temp, err := os.CreateTemp(dir, ".moodle-download-*")
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternal, err, "cannot create a temporary file")
	}
	tempName := temp.Name()
	// Removing the temporary file is safe at any point after this: either the
	// rename has happened and this name no longer exists, or it has not and
	// the partial file is exactly what should go.
	defer os.Remove(tempName)

	written, err := io.Copy(temp, content)
	if err != nil {
		temp.Close()
		return 0, errs.Wrap(errs.CodeNetwork, err, "the download was cut short")
	}
	// Flush to the disk before the rename: without this the rename can be
	// visible while the contents are not.
	if err := temp.Sync(); err != nil {
		temp.Close()
		return 0, errs.Wrap(errs.CodeInternal, err, "cannot flush the download to disk")
	}
	if err := temp.Close(); err != nil {
		return 0, errs.Wrap(errs.CodeInternal, err, "cannot close the download")
	}
	if err := os.Rename(tempName, target); err != nil {
		return 0, errs.Wrap(errs.CodeInternal, err, "cannot put the download in place")
	}
	return written, nil
}

// resolveName decides what to call the file.
//
// The order is the caller's choice, then the server's suggestion, then the
// URL. Everything but the caller's choice comes from the far end and is
// treated as such.
func resolveName(req Request, body *Body) (string, error) {
	if req.As != "" {
		// Still checked: a name the user typed can be a mistake, and
		// --as ../../.bashrc is a mistake worth catching.
		return safeName(req.As, "the name given")
	}
	if body.SuggestedName != "" {
		if name, err := safeName(body.SuggestedName, "the name the site suggested"); err == nil {
			return name, nil
		}
		// A suggestion this project will not use is not a failure; there is
		// still the URL to fall back on.
	}
	if parsed, err := url.Parse(req.URL); err == nil {
		if name, err := safeName(path.Base(parsed.Path), "the name in the URL"); err == nil {
			return name, nil
		}
	}
	return "", errs.New(errs.CodeValidation,
		"cannot work out a safe filename for this download").
		WithHint("pass --as to choose one")
}

// safeName reduces a name to something that can only land in the directory
// asked for.
//
// A filename from a remote server is untrusted input that is about to be used
// as a path. Rejecting is better than silently mangling: a caller who asked
// for one thing should not quietly get another.
func safeName(raw, source string) (string, error) {
	name := strings.TrimSpace(raw)
	// Percent-encoding in a URL path, and occasionally in Content-Disposition,
	// hides separators from a naive check.
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	// Both separators: a Windows-shaped name must not be a path on Linux
	// either, since this file may be read back on another machine.
	if strings.ContainsAny(name, `/\`) {
		return "", errs.New(errs.CodeValidation,
			fmt.Sprintf("%s contains a path separator: %q", source, raw))
	}
	if strings.ContainsRune(name, 0) {
		return "", errs.New(errs.CodeValidation,
			fmt.Sprintf("%s contains a null byte", source))
	}
	if name == "" || name == "." || name == ".." {
		return "", errs.New(errs.CodeValidation,
			fmt.Sprintf("%s is not a usable filename: %q", source, raw))
	}
	return name, nil
}

// NameFromDisposition reads the filename out of a Content-Disposition header.
// It returns an empty string when there is nothing usable, which is not an
// error: the URL is still there to fall back on.
func NameFromDisposition(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	// RFC 5987's filename* wins when both are present: it carries the encoding
	// and so survives non-ASCII names that the plain form mangles.
	if name := params["filename*"]; name != "" {
		return name
	}
	return params["filename"]
}
