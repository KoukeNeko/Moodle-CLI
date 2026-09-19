package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/calendar"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

func newCalendarCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "See what you still have to do",
	}
	cmd.AddCommand(newCalendarUpcomingCommand(r, deps))
	return cmd
}

func newCalendarUpcomingCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags sessionFlags
		days  int
		limit int
	)
	cmd := &cobra.Command{
		Use:     "upcoming",
		Aliases: []string{"todo"},
		Short:   "List deadlines and to-dos, overdue work included",
		Long: "Lists what is still to be done, earliest first. Work whose deadline has\n" +
			"passed is included and marked OVERDUE: it is the most important thing on\n" +
			"the list, and leaving it out would make the list look tidier than it is.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "calendar.upcoming"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, token, err := resolveSession(deps, file, flags.site, flags.account)
			if err != nil {
				return err
			}
			session := openSessionFor(deps, resolved, token)
			capabilities, err := session.Capabilities(cmd.Context())
			if err != nil {
				return err
			}

			query := calendar.Query{Limit: limit}
			if days > 0 {
				until := time.Now().AddDate(0, 0, days)
				query.Until = &until
			}
			result, err := deps.Calendar(session, capabilities).
				Upcoming(cmd.Context(), capabilities, query)
			if err != nil {
				return err
			}

			envelope := v1.CalendarUpcoming(result, resolved.SiteName, resolved.AccountName)
			events, _ := envelope.Data.([]v1.CalendarEvent)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeCalendarTable(w, events) },
			})
		},
	}
	flags.bind(cmd, "read the calendar from")
	cmd.Flags().IntVar(&days, "days", 30, "how many days ahead to look; 0 for everything Moodle offers")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of events to return")
	return cmd
}

func writeCalendarTable(w io.Writer, events []v1.CalendarEvent) error {
	if len(events) == 0 {
		// Not "nothing to do": the calendar answers with what this account can
		// see. A deadline on a hidden course, or one a group restriction keeps
		// away, is filtered out of the same empty list — and telling a student
		// they have nothing due is the one wrong answer that costs them marks.
		_, err := fmt.Fprintln(w, "Nothing due that this account can see.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "WHEN\tCOURSE\tWHAT\tSTATUS")
	overdue := 0
	for _, event := range events {
		status := ""
		if event.Overdue {
			// Never softened into "late" or left blank: this is the line the
			// reader most needs to see.
			status = "OVERDUE"
			overdue++
		}
		if !event.Actionable && status == "" {
			status = "closed"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n",
			when(event.DueAt), event.CourseShortName, event.Title, status)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if overdue > 0 {
		fmt.Fprintf(w, "\n%d item(s) are past their deadline.\n", overdue)
	}
	return nil
}

// when renders a timestamp for a person, keeping the time of day: a deadline
// at 09:00 and one at 23:59 on the same date are not the same deadline.
func when(value *string) string {
	if value == nil {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return *value
	}
	return parsed.Local().Format("2006-01-02 15:04")
}
