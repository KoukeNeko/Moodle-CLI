// Package cli is the Cobra presentation layer: it turns arguments into use
// case input, and use case results into output. It never speaks HTTP.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// BuildInfo describes the running binary. It is injected by the composition
// root so this package does not reach for globals.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// App is the assembled command tree.
type App struct {
	root     *cobra.Command
	renderer *Renderer
}

// New builds the command tree.
func New(build BuildInfo, streams Streams, deps Deps) *App {
	if streams.Out == nil {
		streams.Out = os.Stdout
	}
	if streams.Err == nil {
		streams.Err = os.Stderr
	}

	renderer := &Renderer{Streams: streams, Format: FormatTable}
	app := &App{renderer: renderer}

	var asJSON bool
	var pretty bool

	root := &cobra.Command{
		Use:           "moodle",
		Short:         "A Moodle client for students, scripts and agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Cobra would otherwise print its own error text to stdout and exit 1,
		// which would break both the stream split and the exit code table.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				renderer.Format = FormatJSON
			}
			renderer.Pretty = pretty
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No subcommand: show help on stderr and report a usage error, so
			// stdout stays empty and the exit code is 2.
			if err := cmd.Help(); err != nil {
				return err
			}
			return errs.New(errs.CodeUsage, "no command given")
		},
	}

	// Help and usage text are diagnostics, not data: they belong on stderr.
	root.SetOut(streams.Err)
	root.SetErr(streams.Err)
	if streams.In != nil {
		root.SetIn(streams.In)
	}

	root.PersistentFlags().BoolVar(&asJSON, "json", false,
		"emit the versioned JSON contract on stdout")
	root.PersistentFlags().BoolVar(&pretty, "pretty", false,
		"indent JSON output")

	siteCmd := newSiteCommand(renderer, deps)
	siteCmd.AddCommand(newSiteInspectCommand(renderer, deps))

	root.AddCommand(
		newVersionCommand(renderer, build),
		newSchemaCommand(renderer),
		newCommandsCommand(renderer, func() *cobra.Command { return root }),
		siteCmd,
		newAuthCommand(renderer, deps),
		newCourseCommand(renderer, deps),
		newDoctorCommand(renderer, deps),
	)

	app.root = root
	return app
}

// Execute runs the command tree and returns the process exit code. It never
// panics out to the caller and never writes data to stderr.
func (a *App) Execute(ctx context.Context, args []string) int {
	// Decide the output format before Cobra resolves the command. If the
	// command name is wrong, Cobra fails before it ever parses flags, and a
	// caller that passed --json would get a bare stderr line instead of the
	// JSON document the contract promises.
	a.applyOutputFlags(args)
	a.root.SetArgs(args)
	err := a.root.ExecuteContext(ctx)
	if err == nil {
		return v1.ExitOK
	}
	return a.renderer.RenderError(classify(err))
}

// applyOutputFlags pre-parses only the global output flags, tolerating the
// unknown flags of whatever subcommand may follow.
func (a *App) applyOutputFlags(args []string) {
	set := pflag.NewFlagSet("moodle-output", pflag.ContinueOnError)
	set.ParseErrorsAllowlist.UnknownFlags = true
	set.SetOutput(io.Discard)
	set.Usage = func() {}
	asJSON := set.Bool("json", false, "")
	pretty := set.Bool("pretty", false, "")
	// A parse failure here is not fatal: the real parse happens in Cobra and
	// reports the error properly.
	_ = set.Parse(args)
	if *asJSON {
		a.renderer.Format = FormatJSON
	}
	a.renderer.Pretty = *pretty
}

// classify maps Cobra's own failures onto the error vocabulary. Anything
// Cobra rejects before a command runs is a usage problem.
func classify(err error) error {
	var e *errs.Error
	if ok := asError(err, &e); ok {
		return e
	}
	// Keep the cobra error reachable through Unwrap rather than flattening it
	// into a string; the short context avoids repeating its text.
	return errs.Wrap(errs.CodeUsage, err, "invalid command line")
}

func asError(err error, target **errs.Error) bool {
	type unwrapper interface{ Unwrap() error }
	for err != nil {
		if e, ok := err.(*errs.Error); ok {
			*target = e
			return true
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// humanLine is a small helper for commands whose human output is one line.
func humanLine(format string, args ...any) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := fmt.Fprintf(w, format+"\n", args...)
		return err
	}
}
