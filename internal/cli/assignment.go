package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func newAssignmentCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "assignment",
		Aliases: []string{"assign"},
		Short:   "List assignments and hand work in",
	}
	cmd.AddCommand(
		newAssignmentListCommand(r, deps),
		newAssignmentShowCommand(r, deps),
		newAssignmentStatusCommand(r, deps),
		newAssignmentSubmitCommand(r, deps, mode),
	)
	return cmd
}

// resolvedSession is what a site-bound command needs after sign-in.
type resolvedSession struct {
	resolved     config.Resolved
	capabilities *site.Capabilities
}

// sessionFlags are the two flags every site-bound command carries.
type sessionFlags struct {
	site    string
	account string
}

func (f *sessionFlags) bind(cmd *cobra.Command, what string) {
	cmd.Flags().StringVar(&f.site, "site", "", "site to "+what)
	cmd.Flags().StringVar(&f.account, "account", "", "account to act as")
}

// openAssignments resolves the session and builds the use case.
func openAssignments(cmd *cobra.Command, deps Deps, flags sessionFlags, mode safety.Mode) (
	*assignment.Service, *resolvedSession, error) {
	file, err := config.Load(deps.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	resolved, token, err := resolveSession(deps, file, flags.site, flags.account)
	if err != nil {
		return nil, nil, err
	}
	target, err := targetSite(resolved.SiteName, resolved.Site)
	if err != nil {
		return nil, nil, err
	}
	session := deps.Auth.OpenWithToken(target, resolved.Account.ID, token)
	capabilities, err := session.Capabilities(cmd.Context())
	if err != nil {
		return nil, nil, err
	}
	return deps.Assignments(session, capabilities, mode),
		&resolvedSession{resolved: resolved, capabilities: capabilities}, nil
}

func newAssignmentListCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags     sessionFlags
		courseIDs []string
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your assignments",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "assignment.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openAssignments(cmd, deps, flags, safety.Mode{})
			if err != nil {
				return err
			}
			result, err := service.List(cmd.Context(), session.capabilities, courseIDs)
			if err != nil {
				return err
			}
			envelope := v1.AssignmentList(result,
				session.resolved.SiteName, session.resolved.AccountName)
			items, _ := envelope.Data.([]v1.Assignment)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeAssignmentTable(w, items) },
			})
		},
	}
	flags.bind(cmd, "list assignments from")
	cmd.Flags().StringSliceVar(&courseIDs, "course", nil,
		"limit to these course ids (repeatable); every course by default")
	return cmd
}

func newAssignmentShowCommand(r *Renderer, deps Deps) *cobra.Command {
	var flags sessionFlags
	cmd := &cobra.Command{
		Use:         "show <assignment-id>",
		Short:       "Show one assignment and where you stand in it",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "assignment.show"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openAssignments(cmd, deps, flags, safety.Mode{})
			if err != nil {
				return err
			}
			detail, err := service.Show(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			// The definition alone does not answer the question people
			// actually have, which is whether their work is in.
			state, err := service.Status(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			envelope := v1.AssignmentShow(detail, state,
				session.resolved.SiteName, session.resolved.AccountName)
			payload, _ := envelope.Data.(v1.AssignmentDetail)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeAssignmentDetail(w, payload) },
			})
		},
	}
	flags.bind(cmd, "read the assignment from")
	return cmd
}

func newAssignmentStatusCommand(r *Renderer, deps Deps) *cobra.Command {
	var flags sessionFlags
	cmd := &cobra.Command{
		Use:         "status <assignment-id>",
		Short:       "Show whether your work is saved, handed in, or neither",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "assignment.status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openAssignments(cmd, deps, flags, safety.Mode{})
			if err != nil {
				return err
			}
			state, err := service.Status(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			envelope := v1.AssignmentStatus(args[0], state,
				session.resolved.SiteName, session.resolved.AccountName)
			payload, _ := envelope.Data.(v1.SubmissionState)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeStatus(w, payload) },
			})
		},
	}
	flags.bind(cmd, "read the assignment from")
	return cmd
}

func newAssignmentSubmitCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	var (
		flags           sessionFlags
		dryRun          bool
		draftOnly       bool
		acceptStatement bool
		assumeYes       bool
	)
	cmd := &cobra.Command{
		Use:   "submit <assignment-id> <file>...",
		Short: "Hand work in, and report what Moodle says afterwards",
		Long: "Uploads the given files, attaches them to the submission and, when the\n" +
			"assignment keeps drafts, hands the work in. The final state is read back\n" +
			"from Moodle: a draft is never reported as handed in.",
		Args: cobra.MinimumNArgs(2),
		Annotations: map[string]string{
			annotationKind:    "assignment.submit",
			annotationMutates: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// The global restriction and this invocation's choice are both
			// real; neither overrides the other.
			effective := safety.Mode{DryRun: dryRun, ReadOnly: mode.ReadOnly}
			service, session, err := openAssignments(cmd, deps, flags, effective)
			if err != nil {
				return err
			}
			request := assignment.SubmitRequest{
				AssignmentID:    args[0],
				Files:           args[1:],
				AcceptStatement: acceptStatement,
				DraftOnly:       draftOnly,
				DryRun:          dryRun,
			}
			if !dryRun {
				if err := confirmSubmit(r.Streams, deps.Interactive, request, assumeYes); err != nil {
					return err
				}
			}

			result, err := service.Submit(cmd.Context(), session.capabilities, request)
			if err != nil {
				return err
			}
			envelope := v1.AssignmentSubmit(result,
				session.resolved.SiteName, session.resolved.AccountName)
			payload, _ := envelope.Data.(v1.SubmitReport)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeSubmitReport(w, payload) },
			})
		},
	}
	flags.bind(cmd, "submit to")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"show what would be sent without sending anything")
	cmd.Flags().BoolVar(&draftOnly, "draft", false,
		"save the work without handing it in for grading")
	cmd.Flags().BoolVar(&acceptStatement, "accept-statement", false,
		"record that you accept this assignment's submission statement")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "do not ask for confirmation")
	return cmd
}

// confirmSubmit asks before writing to someone's coursework.
//
// Without a terminal there is nobody to ask, so the caller must have said --yes
// in advance. Assuming consent because stdin happens to be a pipe would make
// the safe default depend on how the command was invoked.
func confirmSubmit(streams Streams, interactive func() bool, req assignment.SubmitRequest, assumeYes bool) error {
	if assumeYes {
		return nil
	}
	if streams.In == nil || interactive == nil || !interactive() {
		return errs.New(errs.CodeUsage, "refusing to submit without confirmation").
			WithHint("pass --yes to confirm, or --dry-run to see what would be sent")
	}

	what := "hand in"
	if req.DraftOnly {
		what = "save as a draft"
	}
	fmt.Fprintf(streams.Err, "About to %s %d file(s) for assignment %s.\n",
		what, len(req.Files), req.AssignmentID)
	for _, path := range req.Files {
		fmt.Fprintf(streams.Err, "  %s\n", path)
	}
	fmt.Fprint(streams.Err, "Continue? [y/N] ")

	line, err := bufio.NewReader(streams.In).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return errs.New(errs.CodeUsage, "no answer given; nothing was submitted")
	}
	if answer := strings.ToLower(strings.TrimSpace(line)); answer != "y" && answer != "yes" {
		return errs.New(errs.CodeUsage, "cancelled; nothing was submitted")
	}
	return nil
}

func writeAssignmentTable(w io.Writer, items []v1.Assignment) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No assignments.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tNAME\tDUE\tHAND-IN")
	for _, item := range items {
		due := "-"
		if item.DueDate != nil {
			due = (*item.DueDate)[:10]
		}
		// Spelling this out is the point: on these assignments, saving work is
		// not submitting it.
		handIn := "not needed"
		if item.NeedsHandIn {
			handIn = "required"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", item.ID, item.Name, due, handIn)
	}
	return table.Flush()
}

func writeAssignmentDetail(w io.Writer, detail v1.AssignmentDetail) error {
	fmt.Fprintf(w, "%s\n\n", detail.Name)
	if text := plainText(detail.Description); text != "" {
		fmt.Fprintf(w, "%s\n\n", text)
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	row := func(label, value string) {
		if value != "" {
			fmt.Fprintf(table, "%s\t%s\n", label, value)
		}
	}
	row("Opens", date(detail.AllowFrom))
	row("Due", date(detail.DueDate))
	// The cut-off is the one that actually stops you: after the due date work
	// is merely late, after the cut-off Moodle refuses it.
	row("Cut-off", date(detail.CutOff))
	if detail.MaxGrade != nil {
		row("Marked out of", strconv.FormatFloat(*detail.MaxGrade, 'f', -1, 64))
	}
	if detail.MaxAttempts != nil {
		row("Attempts", strconv.Itoa(*detail.MaxAttempts))
	}
	if detail.TimeLimitSeconds != nil {
		row("Time limit", (time.Duration(*detail.TimeLimitSeconds) * time.Second).String())
	}
	if len(detail.SubmissionPlugins) > 0 {
		row("Accepts", strings.Join(detail.SubmissionPlugins, ", "))
	}
	handIn := "not needed; saving submits it"
	if detail.NeedsHandIn {
		handIn = "required after saving"
	}
	row("Hand-in", handIn)
	if detail.RequiresStatement {
		row("Statement", "you must accept it with --accept-statement")
	}
	if detail.TeamSubmission {
		row("Team", "this is a group submission")
	}
	if err := table.Flush(); err != nil {
		return err
	}

	fmt.Fprintln(w)
	return writeStatus(w, detail.Submission)
}

func date(value *string) string {
	if value == nil {
		return ""
	}
	return (*value)[:10]
}

// plainText makes Moodle's HTML readable in a terminal. It is deliberately
// crude: the JSON contract carries the original, so nothing is lost by this
// being approximate.
func plainText(html string) string {
	text := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n",
	).Replace(html)
	var out strings.Builder
	depth := 0
	for _, r := range text {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			out.WriteRune(r)
		}
	}
	unescaped := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'",
		"&nbsp;", " ",
	).Replace(out.String())
	return strings.TrimSpace(unescaped)
}

func writeStatus(w io.Writer, state v1.SubmissionState) error {
	fmt.Fprintf(w, "Status:    %s\n", describeStatus(state.Status))
	fmt.Fprintf(w, "Handed in: %s\n", yesNo(state.HandedIn))
	if state.ModifiedAt != nil {
		fmt.Fprintf(w, "Modified:  %s\n", *state.ModifiedAt)
	}
	fmt.Fprintf(w, "Files:     %d\n", state.FileCount)
	if state.GradingStatus != nil {
		fmt.Fprintf(w, "Grading:   %s\n", *state.GradingStatus)
	}
	if !state.CanEdit {
		fmt.Fprintln(w, "\nThis assignment can no longer be edited.")
	}
	return nil
}

// describeStatus spells out what Moodle's word means, because "draft" reads
// like progress and actually means the work is not being marked.
func describeStatus(status string) string {
	switch status {
	case "new":
		return "new (nothing submitted yet)"
	case "draft":
		return "draft (saved but NOT handed in)"
	case "submitted":
		return "submitted (handed in for grading)"
	default:
		return status + " (this site reported a state this build does not recognise)"
	}
}

func writeSubmitReport(w io.Writer, report v1.SubmitReport) error {
	if report.DryRun {
		fmt.Fprintf(w, "Dry run for %s. Nothing was sent.\n\n", report.Assignment.Name)
	} else {
		fmt.Fprintf(w, "%s\n\n", report.Assignment.Name)
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, step := range report.Steps {
		detail := ""
		if step.Detail != nil {
			detail = *step.Detail
		}
		fmt.Fprintf(table, "  %s\t%s\t%s\n", step.Status, step.Name, detail)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if report.DryRun {
		return nil
	}
	fmt.Fprintf(w, "\nMoodle now reports: %s\n", describeStatus(report.FinalStatus))
	if !report.HandedIn {
		// The whole reason this tool exists: never let this go unsaid.
		fmt.Fprintln(w, "This work is NOT handed in for grading.")
	}
	return nil
}
