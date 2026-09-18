package v1

import (
	"github.com/KoukeNeko/moodle-cli/internal/file"
)

// DownloadedFile is the file.download payload.
type DownloadedFile struct {
	// Path is where the bytes ended up, so a script can act on the file
	// without having to guess the name the site chose.
	Path string `json:"path"`
	Size int64  `json:"size"`
	// Replaced reports that something was already there and is now gone.
	Replaced bool `json:"replaced"`
}

// FileDownload converts a completed download into its envelope.
func FileDownload(result file.Result, siteName, accountName string) Envelope {
	meta := NewMeta(SourceWS)
	meta.Site = optional(siteName)
	meta.Account = optional(accountName)
	return NewEnvelope("file.download", DownloadedFile{
		Path:     result.Path,
		Size:     result.Size,
		Replaced: result.Replaced,
	}, meta)
}
