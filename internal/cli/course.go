package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/course"
)

func newCourseCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "course",
		Short: "Work with the courses you are enrolled in",
	}
	cmd.AddCommand(newCourseListCommand(r, deps))
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
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}

			session := deps.Auth.OpenWithToken(target, resolved.Account.ID, token)
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
		_, err := fmt.Fprintln(w, "No courses.")
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
