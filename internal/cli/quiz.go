package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/quiz"
)

func newQuizCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quiz",
		Short: "See your quizzes and how your attempts went",
	}
	cmd.AddCommand(newQuizListCommand(r, deps), newQuizShowCommand(r, deps))
	return cmd
}

// openQuizzes resolves the session and builds the use case.
func openQuizzes(cmd *cobra.Command, deps Deps, flags sessionFlags) (*quiz.Service, *resolvedSession, error) {
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
	return deps.Quizzes(session, capabilities),
		&resolvedSession{resolved: resolved, capabilities: capabilities, session: session}, nil
}

func newQuizListCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags     sessionFlags
		courseIDs []string
		current   bool
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the quizzes in your courses, with when they open and close",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "quiz.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openQuizzes(cmd, deps, flags)
			if err != nil {
				return err
			}
			scope, err := scopeCourses(cmd, deps, session, courseIDs, current)
			if err != nil {
				return err
			}
			result, err := service.List(cmd.Context(), session.capabilities, scope)
			if err != nil {
				return err
			}
			envelope := v1.QuizList(result, session.resolved.SiteName, session.resolved.AccountName)
			quizzes, _ := envelope.Data.([]v1.Quiz)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeQuizTable(w, quizzes, len(scope) > 0) },
			})
		},
	}
	flags.bind(cmd, "list quizzes from")
	cmd.Flags().StringSliceVar(&courseIDs, "course", nil,
		"limit to these course ids (repeatable); every course by default")
	cmd.Flags().BoolVar(&current, "current", false, currentFlagUsage)
	return cmd
}

func newQuizShowCommand(r *Renderer, deps Deps) *cobra.Command {
	var flags sessionFlags
	cmd := &cobra.Command{
		Use:         "show <quiz-id|url>",
		Short:       "Show one quiz and your attempts at it",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "quiz.show"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openQuizzes(cmd, deps, flags)
			if err != nil {
				return err
			}
			detail, err := service.Show(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			envelope := v1.QuizShow(detail, session.resolved.SiteName, session.resolved.AccountName)
			payload, _ := envelope.Data.(v1.QuizDetail)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeQuizDetail(w, payload) },
			})
		},
	}
	flags.bind(cmd, "read the quiz from")
	return cmd
}

func writeQuizTable(w io.Writer, quizzes []v1.Quiz, named bool) error {
	if len(quizzes) == 0 {
		// The same care as the other listings: this is what this account can
		// see in the courses asked about, not a statement about the site.
		if named {
			_, err := fmt.Fprintln(w, "No quizzes visible to this account in those courses.")
			return err
		}
		_, err := fmt.Fprintln(w, "No quizzes visible to this account in its courses.")
		return err
	}
	table := newTable(w)
	fmt.Fprintln(table, "ID\tCOURSE\tNAME\tOPENS\tCLOSES")
	for _, item := range quizzes {
		course := item.CourseID
		if item.CourseShortName != nil {
			course = *item.CourseShortName
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			item.ID, course, item.Name, when(item.OpensAt), when(item.ClosesAt))
	}
	return table.Flush()
}

func writeQuizDetail(w io.Writer, detail v1.QuizDetail) error {
	fmt.Fprintf(w, "%s\n\n", detail.Name)
	table := newTable(w)
	row := func(label, value string) {
		if value != "" {
			fmt.Fprintf(table, "%s\t%s\n", label, value)
		}
	}
	if detail.CourseShortName != nil {
		row("Course", *detail.CourseShortName)
	}
	row("Opens", moment(detail.OpensAt))
	row("Closes", moment(detail.ClosesAt))
	if limit := detail.TimeLimitSeconds; limit != nil {
		if *limit == 0 {
			row("Time limit", "none")
		} else {
			row("Time limit", fmt.Sprintf("%d minutes", *limit/60))
		}
	}
	if attempts := detail.MaxAttempts; attempts != nil {
		if *attempts == 0 {
			row("Attempts allowed", "unlimited")
		} else {
			row("Attempts allowed", strconv.Itoa(*attempts))
		}
	}
	if detail.MaxGrade != nil {
		row("Marked out of", formatNumber(*detail.MaxGrade))
	}
	if detail.BestGrade != nil {
		row("Best grade", formatNumber(*detail.BestGrade))
	}
	if err := table.Flush(); err != nil {
		return err
	}

	if detail.Attempts == nil {
		// Null is not "none": the route could not read them, and the note on
		// stderr says which fields it could not see.
		return nil
	}
	if len(detail.Attempts) == 0 {
		_, err := fmt.Fprintln(w, "\nNo attempts yet.")
		return err
	}
	fmt.Fprintln(w)
	attempts := newTable(w)
	fmt.Fprintln(attempts, "ATTEMPT\tSTATE\tSTARTED\tFINISHED\tGRADE")
	for _, attempt := range detail.Attempts {
		grade := "-"
		if attempt.Grade != nil {
			grade = formatNumber(*attempt.Grade)
		}
		fmt.Fprintf(attempts, "%d\t%s\t%s\t%s\t%s\n", attempt.Number,
			strings.ReplaceAll(attempt.State, "inprogress", "in progress"),
			when(attempt.StartedAt), when(attempt.FinishedAt), grade)
	}
	return attempts.Flush()
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}
