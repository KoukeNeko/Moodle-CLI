package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func newSchemaCommand(r *Renderer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [kind]",
		Short: "Print the JSON Schema for a response kind, or list the kinds",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				kinds := v1.SchemaKinds()
				// The list is data: it goes to stdout in both formats, because
				// `moodle schema` with no argument is a query, not help.
				_, err := fmt.Fprintln(r.Streams.Out, strings.Join(kinds, "\n"))
				return err
			}
			// A schema document is itself JSON and is already the published
			// artefact, so it is written verbatim rather than wrapped in an
			// envelope — wrapping it would mean callers unwrap to feed a
			// validator.
			doc, err := v1.Schema(args[0])
			if err != nil {
				return errs.New(errs.CodeUsage, err.Error()).
					WithHint("run `moodle schema` to list the known kinds")
			}
			_, writeErr := r.Streams.Out.Write(doc)
			return writeErr
		},
	}
	return cmd
}
