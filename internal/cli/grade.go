package cli

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/grade"
)

func newGradeCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grade",
		Short: "Read your grades",
	}
	cmd.AddCommand(
		newGradeListCommand(r, deps),
		newGradeOverviewCommand(r, deps),
	)
	return cmd
}

// openGrades resolves the session and builds the use case.
func openGrades(cmd *cobra.Command, deps Deps, flags sessionFlags) (
	*grade.Service, *resolvedSession, error) {
	file, err := config.Load(deps.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	resolved, token, err := resolveSession(deps, file, flags.site, flags.account)
	if err != nil {
		return nil, nil, err
	}
	session := openSessionFor(deps, resolved, token)
	capabilities, err := session.Capabilities(cmd.Context())
	if err != nil {
		return nil, nil, err
	}
	return deps.Grades(session, capabilities),
		&resolvedSession{resolved: resolved, capabilities: capabilities}, nil
}

func newGradeListCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags    sessionFlags
		courseID string
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Show one course's gradebook",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "grade.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if courseID == "" {
				return errs.New(errs.CodeUsage, "no course given").
					WithHint("pass --course, or use `moodle grade overview` for every course")
			}
			service, session, err := openGrades(cmd, deps, flags)
			if err != nil {
				return err
			}
			result, err := service.Course(cmd.Context(), session.capabilities, courseID)
			if err != nil {
				return err
			}
			envelope := v1.GradeList(result,
				session.resolved.SiteName, session.resolved.AccountName)
			payload, _ := envelope.Data.(v1.GradeReport)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeGradeTable(w, payload) },
			})
		},
	}
	flags.bind(cmd, "read grades from")
	cmd.Flags().StringVar(&courseID, "course", "", "course to show the gradebook of")
	return cmd
}

func newGradeOverviewCommand(r *Renderer, deps Deps) *cobra.Command {
	var flags sessionFlags
	cmd := &cobra.Command{
		Use:         "overview",
		Short:       "Show your total in every course",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "grade.overview"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openGrades(cmd, deps, flags)
			if err != nil {
				return err
			}
			result, err := service.Overview(cmd.Context(), session.capabilities)
			if err != nil {
				return err
			}
			envelope := v1.GradeOverview(result,
				session.resolved.SiteName, session.resolved.AccountName)
			totals, _ := envelope.Data.([]v1.CourseTotal)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeOverviewTable(w, totals) },
			})
		},
	}
	flags.bind(cmd, "read grades from")
	return cmd
}

func writeGradeTable(w io.Writer, report v1.GradeReport) error {
	if len(report.Items) == 0 && report.Total == nil {
		_, err := fmt.Fprintln(w, "Nothing in this gradebook.")
		return err
	}
	if len(report.Items) == 0 {
		// A header with nothing under it reads as a broken command. It happens
		// for real: an activity-level override drops the item and leaves the
		// course total behind, and the reply carries no warning to say so —
		// measured. The sentence is what the listing can honestly claim.
		fmt.Fprintln(w, "No grade items are visible to this account.")
	} else {
		table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "ITEM\tGRADE\tOUT OF\tMARKED")
		for _, item := range report.Items {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\n",
				item.Name, gradeCell(item), outOf(item), date(item.GradedAt))
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}

	if report.Total != nil {
		// Said separately, and said plainly: this is not the sum of the rows
		// above, because Moodle counts only what has been marked so far.
		fmt.Fprintf(w, "\nCourse total: %s\n", gradeCell(*report.Total))
		if report.Total.Max != nil {
			fmt.Fprintf(w, "  out of %s, counting only what has been marked so far\n",
				strconv.FormatFloat(*report.Total.Max, 'f', -1, 64))
		}
	}
	for _, item := range report.Items {
		if item.Feedback != nil {
			fmt.Fprintf(w, "\n%s — feedback:\n  %s\n", item.Name, plainText(*item.Feedback))
		}
	}
	if report.GradableUnknown {
		// Only on a site that does not offer the call. Every student on an
		// ordinary site is refused it for a different reason and sees nothing
		// here, which is the point: a hedge in front of every student, every
		// course, would be noise guarding against a case they are not in.
		fmt.Fprintln(w, "\nThis site does not say who is graded on a course, so an "+
			"empty gradebook here may not be about you.")
	}
	if report.NotGradable {
		// Without this the table above reads as "nothing of yours has been
		// marked yet" to someone who will never be marked here, because the
		// gradebook Moodle hands staff about themselves is the course's item
		// list with every grade empty.
		fmt.Fprintln(w, "\nYou are not a graded participant on this course, "+
			"so none of these rows is about you.")
	}
	return nil
}

// gradeCell renders one grade for a person.
//
// Moodle's own rendering wins wherever it exists: for an item graded on a
// scale or in letters the raw number is a position in a list, not a mark.
func gradeCell(item v1.Grade) string {
	if item.Hidden != nil && *item.Hidden {
		return "hidden"
	}
	if item.Grade == nil {
		return "-"
	}
	if item.Display != "" && item.Display != "-" {
		return item.Display
	}
	return strconv.FormatFloat(*item.Grade, 'f', -1, 64)
}

func outOf(item v1.Grade) string {
	if item.Max == nil {
		return "-"
	}
	return strconv.FormatFloat(*item.Max, 'f', -1, 64)
}

func writeOverviewTable(w io.Writer, totals []v1.CourseTotal) error {
	if len(totals) == 0 {
		_, err := fmt.Fprintln(w, "No course totals.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "COURSE\tTOTAL")
	for _, total := range totals {
		value := total.Display
		if value == "" || value == "-" {
			value = "not graded yet"
		}
		fmt.Fprintf(table, "%s\t%s\n", total.CourseID, value)
	}
	return table.Flush()
}
