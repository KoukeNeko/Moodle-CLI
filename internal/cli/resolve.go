package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func newResolveCommand(r *Renderer) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <url>",
		Short: "Say what a Moodle link points at",
		Long: "Reads a Moodle address and says what it refers to. This is parsing only:\n" +
			"no request is made, so it works offline and looking at a link never tells\n" +
			"the site you have it.\n\n" +
			"An activity address carries the course module id rather than the\n" +
			"activity's own id. The commands that take an assignment also accept the\n" +
			"address itself and do that lookup for you.",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "resolve"},
		RunE: func(cmd *cobra.Command, args []string) error {
			resource, err := site.ParseResourceURL(args[0])
			if err != nil {
				return err
			}
			command := suggestCommand(args[0], resource)
			envelope := v1.Resolve(args[0], resource, command)
			payload, _ := envelope.Data.(v1.ResolvedURL)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeResolved(w, payload) },
			})
		},
	}
}

// suggestCommand names what to run, and stays quiet when nothing here can act
// on the address. A confident wrong suggestion is worse than none.
func suggestCommand(raw string, resource site.Resource) string {
	switch {
	case resource.Kind == site.ResourceActivity && resource.Module == "assign":
		return fmt.Sprintf("moodle assignment show %s", raw)
	case resource.Kind == site.ResourceCourse && resource.CourseID != "":
		return fmt.Sprintf("moodle grade list --course %s", resource.CourseID)
	case resource.Kind == site.ResourceGrades && resource.CourseID != "":
		return fmt.Sprintf("moodle grade list --course %s", resource.CourseID)
	case resource.Kind == site.ResourceFile:
		return fmt.Sprintf("moodle file download %s", raw)
	case resource.Kind == site.ResourceCalendar:
		return "moodle calendar upcoming"
	default:
		return ""
	}
}

func writeResolved(w io.Writer, resolved v1.ResolvedURL) error {
	fmt.Fprintf(w, "%s\n", describeKind(resolved))
	if resolved.CMID != nil {
		// Spelled out because the two are both small integers and are not
		// interchangeable.
		fmt.Fprintf(w, "  course module id: %s (not the activity's own id)\n", *resolved.CMID)
	}
	if resolved.CourseID != nil {
		fmt.Fprintf(w, "  course id: %s\n", *resolved.CourseID)
	}
	if resolved.UserID != nil {
		fmt.Fprintf(w, "  user id: %s\n", *resolved.UserID)
	}
	if resolved.DiscussionID != nil {
		fmt.Fprintf(w, "  discussion id: %s\n", *resolved.DiscussionID)
	}
	if resolved.FileName != nil {
		fmt.Fprintf(w, "  file: %s\n", *resolved.FileName)
	}
	if resolved.Command != nil {
		fmt.Fprintf(w, "\n  %s\n", *resolved.Command)
	}
	return nil
}

func describeKind(resolved v1.ResolvedURL) string {
	switch resolved.Kind {
	case string(site.ResourceActivity):
		if resolved.Module != nil {
			return "Activity: " + *resolved.Module
		}
		return "Activity"
	case string(site.ResourceUnknown):
		return "A Moodle address this build does not recognise."
	case string(site.ResourceGrades):
		return "Grades"
	default:
		return strings.ToUpper(resolved.Kind[:1]) + resolved.Kind[1:]
	}
}
