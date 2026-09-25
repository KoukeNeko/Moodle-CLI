package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/workload"
)

func newWorkloadCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Calculate term credits from configured course custom fields",
	}
	cmd.AddCommand(
		newWorkloadShowCommand(r, deps),
		newWorkloadValidateCommand(r, deps),
	)
	return cmd
}

func newWorkloadShowCommand(r *Renderer, deps Deps) *cobra.Command {
	return newWorkloadResultCommand(r, deps, "show", false)
}

func newWorkloadValidateCommand(r *Renderer, deps Deps) *cobra.Command {
	return newWorkloadResultCommand(r, deps, "validate", true)
}

func newWorkloadResultCommand(r *Renderer, deps Deps, name string, validation bool) *cobra.Command {
	var flags sessionFlags
	var term string
	var requireMinimum bool
	kind := "workload." + name
	short := "Show courses, credits and the applicable minimum by term"
	if validation {
		short = "Validate each term against the configured credit minimum"
	}
	cmd := &cobra.Command{
		Use:         name,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: kind},
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, session, err := openSession(cmd, deps, flags)
			if err != nil {
				return err
			}
			if resolved.resolved.Site == nil {
				return errs.New(errs.CodeConfiguration, "no site is selected")
			}
			if deps.Workload == nil {
				return errs.New(errs.CodeInternal, "the workload service is not configured")
			}
			result, err := deps.Workload(session, resolved.capabilities).Show(
				cmd.Context(), resolved.capabilities, resolved.resolved.Site.Academic, term)
			if err != nil {
				return err
			}
			envelope := v1.Workload(kind, result,
				resolved.resolved.SiteName, resolved.resolved.AccountName)
			if err := r.Render(Result{Envelope: envelope, Human: func(w io.Writer) error {
				return writeWorkload(w, result)
			}}); err != nil {
				return err
			}
			if validation && requireMinimum {
				if err := workload.ValidationError(result); err != nil {
					if r.Format != FormatJSON {
						fmt.Fprintf(r.Streams.Err, "Validation failed: %s\n", err)
					}
					return Reported(v1.ExitValidation)
				}
			}
			return nil
		},
	}
	flags.bind(cmd, name+" workload for")
	cmd.Flags().StringVar(&term, "term", "", "only this academic term")
	if validation {
		cmd.Flags().BoolVar(&requireMinimum, "require-minimum", false,
			"exit with validation code 7 when any term is incomplete or below its minimum")
	}
	return cmd
}

func writeWorkload(w io.Writer, result workload.Result) error {
	if len(result.Groups) == 0 {
		_, err := fmt.Fprintln(w, "No enrolled courses with academic workload metadata were returned.")
		return err
	}
	table := newTable(w)
	fmt.Fprintln(table, "TERM\tLEVEL\tCREDITS\tMINIMUM\tRESULT\tCOURSES")
	for _, group := range result.Groups {
		status := "meets"
		if !group.Complete {
			status = "incomplete metadata"
		} else if !group.MeetsMinimum {
			status = "below"
		}
		fmt.Fprintf(table, "%s\t%s\t%.2f\t%.2f\t%s\t%d\n",
			group.Term, group.Level, group.Credits, group.Minimum, status, len(group.Courses))
	}
	return table.Flush()
}
