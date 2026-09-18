package cli

import (
	"io"
	"runtime"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

// versionData is the `version` payload. Field names are contract.
type versionData struct {
	Version   string  `json:"version"`
	Commit    string  `json:"commit"`
	BuildDate *string `json:"build_date"`
	GoVersion string  `json:"go_version"`
	Platform  string  `json:"platform"`
}

func newVersionCommand(r *Renderer, build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data := versionData{
				Version:   build.Version,
				Commit:    build.Commit,
				GoVersion: runtime.Version(),
				Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			}
			// build_date is null rather than "" when unset, so a consumer can
			// tell "not recorded" from a real timestamp.
			if build.BuildDate != "" {
				date := build.BuildDate
				data.BuildDate = &date
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("version", data, v1.NewMeta(v1.SourceLocal)),
				Human: func(w io.Writer) error {
					return writeVersionHuman(w, data)
				},
			})
		},
	}
}

func writeVersionHuman(w io.Writer, d versionData) error {
	date := "unknown"
	if d.BuildDate != nil {
		date = *d.BuildDate
	}
	return humanLine("moodle %s (commit %s, built %s, %s, %s)",
		d.Version, d.Commit, date, d.GoVersion, d.Platform)(w)
}
