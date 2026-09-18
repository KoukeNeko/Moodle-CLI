package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/file"
)

func newFileCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Fetch files the site is holding",
	}
	cmd.AddCommand(newFileDownloadCommand(r, deps))
	return cmd
}

func newFileDownloadCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags sessionFlags
		dir   string
		as    string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "download <url>",
		Short: "Download a file from the site you are signed in to",
		Long: "Downloads a file Moodle is holding, such as an assignment attachment or\n" +
			"something you submitted. The URL must belong to the site you signed in to:\n" +
			"Moodle wants the credential in the URL, so fetching from anywhere else\n" +
			"would hand your token to whoever runs that address.\n\n" +
			"The file is written to a temporary name and moved into place only once it\n" +
			"has all arrived, so an interrupted download leaves nothing behind rather\n" +
			"than a truncated file under the right name.",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "file.download"},
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, token, err := resolveSession(deps, configFile, flags.site, flags.account)
			if err != nil {
				return err
			}
			session := openSessionFor(deps, resolved, token)
			capabilities, err := session.Capabilities(cmd.Context())
			if err != nil {
				return err
			}
			if !capabilities.CanDownload {
				return errs.New(errs.CodePermissionDenied,
					"this site does not allow file downloads for your account").
					WithReason(errs.ReasonCapability)
			}

			result, err := deps.Files(session, capabilities).
				Download(cmd.Context(), file.Request{
					URL: args[0], Dir: dir, As: as, Overwrite: force,
				})
			if err != nil {
				return err
			}

			envelope := v1.FileDownload(result, resolved.SiteName, resolved.AccountName)
			payload, _ := envelope.Data.(v1.DownloadedFile)
			return r.Render(Result{
				Envelope: envelope,
				Human: func(w io.Writer) error {
					if payload.Replaced {
						fmt.Fprintf(w, "Replaced %s (%d bytes)\n", payload.Path, payload.Size)
						return nil
					}
					_, err := fmt.Fprintf(w, "Saved %s (%d bytes)\n", payload.Path, payload.Size)
					return err
				},
			})
		},
	}
	flags.bind(cmd, "download from")
	cmd.Flags().StringVar(&dir, "dir", "", "directory to save into (default: the working directory)")
	cmd.Flags().StringVar(&as, "as", "", "save under this name instead of the one the site suggests")
	cmd.Flags().BoolVar(&force, "force", false, "replace a file that is already there")
	return cmd
}
