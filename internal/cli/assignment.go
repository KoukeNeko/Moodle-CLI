package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/assignment"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
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
	addWorkflowCommands(cmd, r, deps, mode, assignmentWorkflowSpecs()...)
	return cmd
}

// resolvedSession is what a site-bound command needs after sign-in.
type resolvedSession struct {
	resolved     config.Resolved
	capabilities *site.Capabilities
	// session is the open connection, for a command that needs a second use
	// case besides its own — a course listing to name or filter courses.
	session *auth.Session
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
	session := openSessionFor(deps, resolved, token)
	capabilities, err := session.Capabilities(cmd.Context())
	if err != nil {
		return nil, nil, err
	}
	return deps.Assignments(session, capabilities, mode),
		&resolvedSession{resolved: resolved, capabilities: capabilities, session: session}, nil
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
				Human: func(w io.Writer) error {
					return writeAssignmentTable(w, items, len(courseIDs) > 0)
				},
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
		Use:         "show <assignment-id|url>",
		Short:       "Show one assignment and where you stand in it",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "assignment.show"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openAssignments(cmd, deps, flags, safety.Mode{})
			if err != nil {
				return err
			}
			id, err := service.Locate(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			detail, err := service.Show(cmd.Context(), session.capabilities, id)
			if err != nil {
				return err
			}
			// The definition alone does not answer the question people
			// actually have, which is whether their work is in.
			state, err := service.Status(cmd.Context(), session.capabilities, id)
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
		Use:         "status <assignment-id|url>",
		Short:       "Show whether your work is saved, handed in, or neither",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "assignment.status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openAssignments(cmd, deps, flags, safety.Mode{})
			if err != nil {
				return err
			}
			id, err := service.Locate(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			state, err := service.Status(cmd.Context(), session.capabilities, id)
			if err != nil {
				return err
			}
			envelope := v1.AssignmentStatus(id, state,
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
		Use:   "submit <assignment-id|url> <file>...",
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
			id, err := service.Locate(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			request := assignment.SubmitRequest{
				AssignmentID:    id,
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

func writeAssignmentTable(w io.Writer, items []v1.Assignment, named bool) error {
	if len(items) == 0 {
		// Named without a course, mod_assign_get_assignments searches the
		// courses this account is enrolled on and nothing else. An account can
		// read a course it is not enrolled on — measured: a manager who can
		// open a course holding two assignments was told there were none.
		// The set is in the sentence rather than in a footnote under it.
		if named {
			_, err := fmt.Fprintln(w, "No assignments in those courses.")
			return err
		}
		_, err := fmt.Fprintln(w,
			"No assignments in the courses this account is enrolled on.")
		return err
	}
	table := newTable(w)
	// COURSE earns its width on the accounts that need it most: measured on a
	// teacher of 31 courses, this listing held two rows called "Problem Set 6"
	// five years apart, eight called "Exercise 1" and eight "Final Project".
	// The course's full name does not separate them either — eight years of
	// one course share that too — so the short name is what carries the year.
	fmt.Fprintln(table, "ID\tCOURSE\tNAME\tDUE\tHAND-IN")
	for _, item := range items {
		course := "-"
		if item.CourseShortName != nil {
			course = *item.CourseShortName
		}
		// With the time: a deadline at 00:00 is the evening before in
		// practice, and one at 23:59 is not.
		due := when(item.DueDate)
		// Spelling this out is the point: on these assignments, saving work is
		// not submitting it. When the route could not see the setting, say so
		// rather than pick the reassuring answer.
		handIn := "unknown"
		if item.NeedsHandIn != nil {
			handIn = "not needed"
			if *item.NeedsHandIn {
				handIn = "required"
			}
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			item.ID, course, item.Name, due, handIn)
	}
	return table.Flush()
}

func writeAssignmentDetail(w io.Writer, detail v1.AssignmentDetail) error {
	fmt.Fprintf(w, "%s\n\n", detail.Name)
	if text := plainText(detail.Description); text != "" {
		fmt.Fprintf(w, "%s\n\n", text)
	}

	table := newTable(w)
	row := func(label, value string) {
		if value != "" {
			fmt.Fprintf(table, "%s\t%s\n", label, value)
		}
	}
	row("Opens", moment(detail.AllowFrom))
	row("Due", moment(detail.DueDate))
	// The cut-off is the one that actually stops you: after the due date work
	// is merely late, after the cut-off Moodle refuses it.
	row("Cut-off", moment(detail.CutOff))
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
	handIn := "unknown — this route cannot see the setting"
	if detail.NeedsHandIn != nil {
		handIn = "not needed; saving submits it"
		if *detail.NeedsHandIn {
			handIn = "required after saving"
		}
	}
	row("Hand-in", handIn)
	if detail.RequiresStatement {
		row("Statement", "you must accept it with --accept-statement")
	}
	if detail.TeamSubmission {
		row("Team", "this is a group submission")
	}
	if detail.BlindMarking {
		// Deliberately not "no grader can identify you": some roles can, and
		// saying otherwise would be a promise this tool cannot keep. What a
		// student does need is that an empty grade may be one Moodle is
		// holding back rather than one nobody has awarded.
		marking := "anonymous; a finished grade is withheld until identities are revealed"
		if detail.IdentitiesRevealed {
			marking = "was anonymous; identities have been revealed"
		}
		row("Marking", marking)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if len(detail.Attachments) > 0 {
		// The teacher's handouts are often the assignment itself; the link
		// is what `moodle file download` takes.
		fmt.Fprintln(w, "\nAttachments:")
		files := newTable(w)
		for _, attachment := range detail.Attachments {
			fmt.Fprintf(files, "  %s\t%s\n", attachment.Name, attachment.URL)
		}
		if err := files.Flush(); err != nil {
			return err
		}
	}

	fmt.Fprintln(w)
	return writeStatus(w, detail.Submission)
}

// moment is when() for a row that is left out when there is no value.
func moment(value *string) string {
	if value == nil {
		return ""
	}
	return when(value)
}

// date renders a timestamp as the reader's local date. The contract carries
// UTC, and slicing its date off named the wrong day for anything between local
// midnight and the UTC one: a Taiwanese term starting on 1 August read 31 July.
func date(value *string) string {
	if value == nil {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return *value
	}
	return parsed.Local().Format("2006-01-02")
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
	// What Moodle holds, by name and link, so the reader can check it is
	// what they meant to hand in.
	for _, held := range state.Files {
		if held.URL == "" {
			fmt.Fprintf(w, "           %s\n", held.Name)
			continue
		}
		fmt.Fprintf(w, "           %s  %s\n", held.Name, held.URL)
	}
	if state.GradingStatus != nil {
		fmt.Fprintf(w, "Grading:   %s\n", *state.GradingStatus)
	}
	if state.OnlineSubmission != nil && !*state.OnlineSubmission {
		// "Handed in: no" is true and reads as a failure to submit. There is
		// nothing to submit: the work for this one is done somewhere else and
		// the grade arrives without anything passing through Moodle.
		fmt.Fprintln(w, "Offline:   this assignment takes no online submission; "+
			"the work is marked from elsewhere")
	}
	if state.TimerEndsAt != nil {
		// Deliberately not called a deadline. Moodle accepts work after the
		// timer expires and marks it as over time; the cut-off is what closes
		// submission. But a student reading only the due date has no idea a
		// clock is running at all, and it can run out weeks earlier.
		fmt.Fprintf(w, "Timer:     started; the time limit runs out at %s\n",
			when(state.TimerEndsAt))
	}
	for _, earlier := range state.EarlierAttempts {
		// Without this a reopened assignment reads as a first attempt that was
		// never handed in: the current one really is empty, and the work the
		// student did submit is in a record this listing never mentioned.
		// Moodle counts from zero, so attempt 0 is the first one.
		line := fmt.Sprintf("Earlier:   attempt %d was %s", earlier.Number+1, earlier.Status)
		if earlier.FileCount > 0 {
			file := "files"
			if earlier.FileCount == 1 {
				file = "file"
			}
			line += fmt.Sprintf(" with %d %s", earlier.FileCount, file)
		}
		if earlier.SavedAt != nil {
			line += " on " + date(earlier.SavedAt)
		}
		fmt.Fprintln(w, line)
	}
	if state.GroupSubmission {
		// Without this, "Handed in: yes" reads as a statement about the person
		// asking. On a group assignment it is about the group, and the
		// assignment's own settings are in a different call the status command
		// never makes — so the reader had nothing to tell them.
		fmt.Fprintln(w, "Team:      this is a group submission, so the state above "+
			"is the group's")
	}
	if waiting := state.MembersStillToSubmit; waiting != nil && *waiting > 0 {
		phrase := "members still have"
		if *waiting == 1 {
			phrase = "member still has"
		}
		fmt.Fprintf(w, "Waiting:   %d group %s to submit\n", *waiting, phrase)
	}
	if state.ExtensionDueDate != nil {
		// The assignment's own cut-off is printed from the assignment, and it
		// can already have passed while this account may still submit: an
		// extension is granted per person and Moodle folds it into whether
		// the submission is open, not into the dates it publishes. Without
		// this line the reader sees only the date they appear to have missed.
		fmt.Fprintf(w, "Extension: %s — this account may submit until then\n",
			when(state.ExtensionDueDate))
	}
	if !state.CanEdit {
		// Moodle's canedit is one flag over several causes: a submission window
		// that has not opened, a cut-off that has passed, a lock, a missing
		// group. "No longer" picked one of them and was wrong whenever the
		// assignment had simply not opened yet. This call does not carry the
		// dates that would settle it, so it reports the refusal and not a cause.
		fmt.Fprintln(w, "\nMoodle is not accepting changes to this submission.")
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
	case "reopened":
		return "reopened (a grader opened another attempt; the earlier one was handed in)"
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
	table := newTable(w)
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
