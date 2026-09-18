package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

// commandFlag describes one flag of a command.
type commandFlag struct {
	Name      string  `json:"name"`
	Shorthand *string `json:"shorthand"`
	Usage     string  `json:"usage"`
}

// commandDescriptor lets an agent discover the command surface without
// scraping help text.
type commandDescriptor struct {
	Path  string  `json:"path"`
	Short string  `json:"short"`
	Kind  *string `json:"kind"`
	// Mutates says whether the command can write to Moodle. It is derived
	// from the command tree, not guessed by the caller.
	Mutates bool          `json:"mutates"`
	Flags   []commandFlag `json:"flags"`
}

// annotation keys carried on each cobra.Command.
const (
	annotationKind    = "moodle.kind"
	annotationMutates = "moodle.mutates"
)

func newCommandsCommand(r *Renderer, rootOf func() *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "Describe every command, for scripts and agents",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			descriptors := describe(rootOf())
			return r.Render(Result{
				Envelope: v1.NewEnvelope("commands", descriptors, v1.NewMeta(v1.SourceLocal)),
				Human: func(w io.Writer) error {
					return writeCommandsHuman(w, descriptors)
				},
			})
		},
	}
}

func describe(root *cobra.Command) []commandDescriptor {
	var out []commandDescriptor
	var walk func(cmd *cobra.Command, prefix string)
	walk = func(cmd *cobra.Command, prefix string) {
		for _, child := range cmd.Commands() {
			if !child.IsAvailableCommand() {
				continue
			}
			path := strings.TrimSpace(prefix + " " + child.Name())
			out = append(out, commandDescriptor{
				Path:    path,
				Short:   child.Short,
				Kind:    annotationValue(child, annotationKind),
				Mutates: child.Annotations[annotationMutates] == "true",
				Flags:   describeFlags(child),
			})
			walk(child, path)
		}
	}
	walk(root, "")
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func annotationValue(cmd *cobra.Command, key string) *string {
	if value, ok := cmd.Annotations[key]; ok && value != "" {
		return &value
	}
	return nil
}

func describeFlags(cmd *cobra.Command) []commandFlag {
	flags := []commandFlag{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flag := commandFlag{Name: f.Name, Usage: f.Usage}
		if f.Shorthand != "" {
			shorthand := f.Shorthand
			flag.Shorthand = &shorthand
		}
		flags = append(flags, flag)
	})
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func writeCommandsHuman(w io.Writer, descriptors []commandDescriptor) error {
	for _, d := range descriptors {
		marker := " "
		if d.Mutates {
			marker = "!"
		}
		if _, err := fmt.Fprintf(w, "%s %-24s %s\n", marker, d.Path, d.Short); err != nil {
			return err
		}
	}
	return nil
}
