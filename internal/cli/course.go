package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/course"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

func newCourseCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "course",
		Short: "Read and manage courses allowed by your Moodle role",
	}
	cmd.AddCommand(newCourseListCommand(r, deps))
	addWorkflowCommands(cmd, r, deps, mode, courseWorkflowSpecs()...)
	return cmd
}

func newCourseListCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		siteFlag    string
		accountFlag string
		limit       int
		cursor      string
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your courses",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "course.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, token, err := resolveSession(deps, file, siteFlag, accountFlag)
			if err != nil {
				return err
			}
			session := openSessionFor(deps, resolved, token)
			capabilities, err := session.Capabilities(cmd.Context())
			if err != nil {
				return err
			}

			result, err := deps.Courses(session, capabilities).
				List(cmd.Context(), capabilities, course.ListQuery{Limit: limit, Cursor: cursor})
			if err != nil {
				return err
			}

			envelope := v1.CourseList(result, resolved.SiteName, resolved.AccountName)
			courses, _ := envelope.Data.([]v1.Course)

			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeCourseTable(w, courses, result.NextCursor) },
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to list courses from")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account to list courses for")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of courses to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue a previous listing")
	return cmd
}

func writeCourseTable(w io.Writer, courses []v1.Course, nextCursor string) error {
	if len(courses) == 0 {
		// "No courses" claimed more than the call answers. This listing is
		// enrolments, and an account can reach a course without holding one: a
		// manager with no enrolment at all reads courses perfectly well, and
		// was being told they had none. Say which question was answered.
		// Neither half of this may claim more than the call answered.
		// core_enrol_get_users_courses leaves out a course whose enrolment is
		// suspended, whose dates have passed, and one the site has hidden —
		// measured: a student of an archived cohort has a live enrolment and
		// an empty list. "You are not enrolled on any course" was false for
		// every one of them.
		// The list of exclusions is not decoration: naming three of them and
		// leaving out the fourth reads as if those three were all of them.
		// core_enrol_get_users_courses calls enrol_get_users_courses with
		// onlyactive hardcoded true — read in 5.2's enrol/externallib.php —
		// and suspending a student is what an institution does instead of
		// unenrolling them. Measured: suspend one enrolment and the list is
		// empty.
		_, err := fmt.Fprintln(w, "No courses came back for this account.\n"+
			"This lists enrolments the site counts as active, so a course that is "+
			"hidden, finished, suspended, or reachable without an enrolment is not here.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSHORT NAME\tFULL NAME\tSTARTS")
	for _, item := range courses {
		starts := "-"
		if item.StartDate != nil {
			starts = (*item.StartDate)[:10]
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", item.ID, item.ShortName, item.FullName, starts)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if nextCursor != "" {
		// Without this a user who passed --limit has no way of knowing there
		// is more, or how to ask for it.
		fmt.Fprintf(w, "\nMore courses available: --cursor %s\n", nextCursor)
	}
	return nil
}
