package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/forum"
)

func newForumCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forum",
		Short: "Read course discussions",
	}
	cmd.AddCommand(
		newForumListCommand(r, deps),
		newForumDiscussionsCommand(r, deps),
		newForumReadCommand(r, deps),
	)
	return cmd
}

// openForums resolves the session and builds the use case.
func openForums(cmd *cobra.Command, deps Deps, flags sessionFlags) (
	*forum.Service, *resolvedSession, error) {
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
	return deps.Forums(session, capabilities),
		&resolvedSession{resolved: resolved, capabilities: capabilities}, nil
}

func newForumListCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags     sessionFlags
		courseIDs []string
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the forums in your courses",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "forum.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openForums(cmd, deps, flags)
			if err != nil {
				return err
			}
			result, err := service.List(cmd.Context(), session.capabilities, courseIDs)
			if err != nil {
				return err
			}
			envelope := v1.ForumList(result,
				session.resolved.SiteName, session.resolved.AccountName)
			forums, _ := envelope.Data.([]v1.Forum)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeForumTable(w, forums) },
			})
		},
	}
	flags.bind(cmd, "list forums from")
	cmd.Flags().StringSliceVar(&courseIDs, "course", nil,
		"limit to these course ids (repeatable); every course by default")
	return cmd
}

func newForumDiscussionsCommand(r *Renderer, deps Deps) *cobra.Command {
	var flags sessionFlags
	cmd := &cobra.Command{
		Use:         "discussions <forum-id|url>",
		Short:       "List the threads in one forum",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "forum.discussions"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openForums(cmd, deps, flags)
			if err != nil {
				return err
			}
			result, err := service.Discussions(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			envelope := v1.ForumDiscussions(result,
				session.resolved.SiteName, session.resolved.AccountName)
			discussions, _ := envelope.Data.([]v1.Discussion)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeDiscussionTable(w, discussions) },
			})
		},
	}
	flags.bind(cmd, "read the forum from")
	return cmd
}

func newForumReadCommand(r *Renderer, deps Deps) *cobra.Command {
	var flags sessionFlags
	cmd := &cobra.Command{
		Use:   "read <discussion-id|url>",
		Short: "Read one thread, oldest post first",
		Long: "Prints a thread in the order a person reads it: the opening post, then\n" +
			"the replies oldest to newest. Moodle returns them newest first, which\n" +
			"puts every reply before the thing it answers.\n\n" +
			"Reading here never marks anything as read on the site.",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "forum.thread"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, session, err := openForums(cmd, deps, flags)
			if err != nil {
				return err
			}
			result, err := service.Thread(cmd.Context(), session.capabilities, args[0])
			if err != nil {
				return err
			}
			envelope := v1.ForumThread(result,
				session.resolved.SiteName, session.resolved.AccountName)
			posts, _ := envelope.Data.([]v1.Post)
			return r.Render(Result{
				Envelope: envelope,
				Human: func(w io.Writer) error {
					return writeThread(w, posts, result.WithheldPosts)
				},
			})
		},
	}
	flags.bind(cmd, "read the discussion from")
	return cmd
}

func writeForumTable(w io.Writer, forums []v1.Forum) error {
	if len(forums) == 0 {
		// Not "this course has no forums": that is more than the reply
		// supports. mod_forum_get_forums_by_courses filters by activity
		// visibility and by mod/forum:viewdiscussion before it answers, so an
		// empty listing is about what this account can see, never about what
		// the course holds. A course it cannot read at all is refused earlier.
		_, err := fmt.Fprintln(w, "No forums are visible to this account.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// Every course carries a forum called Announcements, so without this
	// column a listing across courses is rows of one repeated word —
	// measured on a teacher of 31 courses: six leading rows reading
	// "Announcements news 0", separable only by id. The course id is what
	// mod_forum_get_forums_by_courses returns; it does not send a name, and
	// it is also what --course takes.
	fmt.Fprintln(table, "ID\tCOURSE\tNAME\tTYPE\tTHREADS")
	for _, item := range forums {
		// Moodle leaves the count out rather than sending zero, and a column
		// reading 0 says the forum is empty. A dash says the site did not say.
		threads := "-"
		if item.Discussions != nil {
			threads = strconv.Itoa(*item.Discussions)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			item.ID, item.CourseID, item.Name, item.Kind, threads)
	}
	return table.Flush()
}

func writeDiscussionTable(w io.Writer, discussions []v1.Discussion) error {
	if len(discussions) == 0 {
		_, err := fmt.Fprintln(w, "No discussions.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tTHREAD\tSTARTED BY\tREPLIES\tLAST ACTIVITY")
	for _, item := range discussions {
		name := item.Name
		if item.Pinned {
			name = "[pinned] " + name
		}
		if item.Locked {
			name = "[locked] " + name
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%d\t%s\n",
			item.ID, name, item.Author, item.Replies, date(item.ModifiedAt))
	}
	return table.Flush()
}

func writeThread(w io.Writer, posts []v1.Post, withheld int) error {
	if len(posts) == 0 {
		_, err := fmt.Fprintln(w, "Nothing in this thread.")
		return err
	}
	for index, post := range posts {
		if index > 0 {
			fmt.Fprintln(w)
		}
		if post.Deleted {
			// The post stays in the thread: taking it out would lose the shape
			// of the conversation around it.
			fmt.Fprintln(w, "[deleted post]")
			continue
		}
		who := post.Author
		if who == "" {
			who = "unknown"
		}
		fmt.Fprintf(w, "%s — %s", who, post.Subject)
		if post.CreatedAt != nil {
			fmt.Fprintf(w, "  (%s)", when(post.CreatedAt))
		}
		fmt.Fprintln(w)
		if text := plainText(post.Message); text != "" {
			for _, line := range strings.Split(text, "\n") {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
		for _, attachment := range post.Attachments {
			fmt.Fprintf(w, "  [attachment] %s (%d bytes)\n", attachment.Name, attachment.Size)
		}
	}
	if withheld > 0 {
		// A question with its answers withheld looks exactly like a question
		// nobody answered, and a Q&A forum shows a student precisely that
		// until they post. Saying how many are missing is the difference
		// between "nobody replied" and "you cannot see the replies yet".
		phrase := "posts in this thread were"
		if withheld == 1 {
			phrase = "post in this thread was"
		}
		fmt.Fprintf(w, "\n%d further %s not sent to this account.\n", withheld, phrase)
	}
	return nil
}
