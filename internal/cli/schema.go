package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func newSchemaCommand(r *Renderer, rootOf func() *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [kind | command...]",
		Short: "Print a response kind's JSON Schema, or a command's contract with its safety",
		Long: "With a kind such as assignment.list, prints that response's JSON Schema.\n" +
			"With a command such as `assignment submit`, prints what the command changes\n" +
			"(safety: read, local or write), whether a failed run may be repeated\n" +
			"(idempotency), and its input and output JSON Schema — what an unattended\n" +
			"caller needs to decide whether it may run it.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				kinds := v1.SchemaKinds()
				// The list is data: it goes to stdout in both formats, because
				// `moodle schema` with no argument is a query, not help.
				_, err := fmt.Fprintln(r.Streams.Out, strings.Join(kinds, "\n"))
				return err
			}
			// A published kind keeps meaning what it always meant. The few
			// one-word commands that share a kind's name — version, doctor,
			// resolve, commands — read nothing and change nothing, and
			// `moodle commands --json` gives their safety too.
			if len(args) > 1 || !isKind(args[0]) {
				target, err := findCommand(rootOf(), args)
				if err != nil && len(args) == 1 && errs.From(err).Code == errs.CodeUsage &&
					strings.HasPrefix(errs.From(err).Message, "no command") {
					return errs.New(errs.CodeUsage,
						fmt.Sprintf("%q is neither a response kind nor a command", args[0])).
						WithHint("`moodle schema` lists the kinds; `moodle commands` lists the commands")
				}
				if err != nil {
					return err
				}
				descriptor := describeCommand(target, strings.Join(args, " "))
				return r.Render(Result{
					Envelope: v1.NewEnvelope("command.schema", descriptor, v1.NewMeta(v1.SourceLocal)),
					Human:    func(w io.Writer) error { return writeCommandSchemaHuman(w, descriptor) },
				})
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

func isKind(name string) bool {
	for _, kind := range v1.SchemaKinds() {
		if kind == name {
			return true
		}
	}
	return false
}
