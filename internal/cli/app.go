// Package cli is the Cobra presentation layer: it turns arguments into use
// case input, and use case results into output. It never speaks HTTP.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

// EnvReadOnly turns on read-only mode for the whole process.
//
// It is an environment variable as well as a flag because the restriction is
// usually imposed by whoever starts the process — a CI job, or a harness
// running an agent — and they do not control every argv the tool is given.
const EnvReadOnly = "MOODLE_CLI_READ_ONLY"

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
	// mode is shared with the commands, which read it when they run rather
	// than when they are built: the global flags are parsed after the tree
	// already exists.
	mode *safety.Mode
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
	mode := &safety.Mode{}
	app := &App{renderer: renderer, mode: mode}

	var asJSON bool
	var pretty bool
	var readOnly bool
	// Shared rather than copied: the flag is parsed after the commands below
	// have been built with their own copy of deps.
	backend := new(string)
	deps.Backend = backend

	root := &cobra.Command{
		Use:           "moodle",
		Short:         "A Moodle client for learners, educators, administrators, scripts and agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Cobra would otherwise print its own error text to stdout and exit 1,
		// which would break both the stream split and the exit code table.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				renderer.Format = FormatJSON
			}
			renderer.Pretty = pretty
			if readOnly {
				mode.ReadOnly = true
			}
			switch *backend {
			case "", config.BackendAuto, config.BackendWSOnly:
			default:
				return errs.New(errs.CodeUsage,
					fmt.Sprintf("unknown backend %q", *backend)).
					WithHint("use " + config.BackendAuto + " or " + config.BackendWSOnly)
			}
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
	root.PersistentFlags().BoolVar(&readOnly, "read-only", false,
		"refuse every call that can change anything on the site")
	// The fallbacks read pages meant for a person. A site may have every
	// reason to treat that differently from an API call, so saying "web
	// service or nothing" has to be possible for one run as well as for good.
	root.PersistentFlags().StringVar(backend, "backend", "",
		"which routes may answer: "+config.BackendAuto+" or "+config.BackendWSOnly+
			" (default: the site's own setting)")

	siteCmd := newSiteCommand(renderer, deps)
	siteCmd.AddCommand(newSiteInspectCommand(renderer, deps))

	root.AddCommand(
		newVersionCommand(renderer, build),
		newSchemaCommand(renderer),
		newCommandsCommand(renderer, func() *cobra.Command { return root }),
		siteCmd,
		newAuthCommand(renderer, deps),
		newCourseCommand(renderer, deps, mode),
		newAssignmentCommand(renderer, deps, mode),
		newGradeCommand(renderer, deps, mode),
		newCalendarCommand(renderer, deps, mode),
		newForumCommand(renderer, deps, mode),
		newQuizCommand(renderer, deps),
		newParticipantCommand(renderer, deps, mode),
		newEnrolmentCommand(renderer, deps, mode),
		newGroupCommand(renderer, deps, mode),
		newCompletionCommand(renderer, deps, mode),
		newFileCommand(renderer, deps),
		newResolveCommand(renderer),
		newWSCommand(renderer, deps, mode),
		newWorkloadCommand(renderer, deps),
		newAPICommand(renderer, deps, mode),
		newMCPCommand(deps, mode),
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
	a.applyGlobalFlags(args)
	if a.mode.ReadOnly {
		// The command surface shrinks rather than each write failing somewhere
		// inside itself: an agent reading `moodle commands` cannot see a
		// command it is not allowed to run.
		withholdMutatingCommands(a.root)
	}
	a.root.SetArgs(args)
	err := a.root.ExecuteContext(ctx)
	if err == nil {
		return v1.ExitOK
	}
	return a.renderer.RenderError(classify(err))
}

// withholdMutatingCommands hides every command that can write and makes it
// refuse.
//
// Hiding alone would leave Cobra reporting "unknown command", which tells
// someone who set the restriction in their environment nothing about why. The
// command stays registered so its own flags still parse and the refusal is the
// thing they see.
func withholdMutatingCommands(parent *cobra.Command) {
	for _, child := range parent.Commands() {
		if child.Annotations[annotationMutates] == "true" {
			name := child.CommandPath()
			child.Hidden = true
			child.RunE = func(*cobra.Command, []string) error {
				return errs.New(errs.CodePermissionDenied,
					"refusing to run "+name+" in read-only mode").
					WithHint("read-only mode is on; unset --read-only or " +
						EnvReadOnly + " to allow writes")
			}
			continue
		}
		withholdMutatingCommands(child)
	}
}

// applyGlobalFlags pre-parses only the global flags, tolerating the unknown
// flags of whatever subcommand may follow.
//
// They have to be known before Cobra resolves the command: a wrong command
// name fails before flags are ever parsed, and read-only mode decides which
// commands exist at all.
func (a *App) applyGlobalFlags(args []string) {
	set := pflag.NewFlagSet("moodle-output", pflag.ContinueOnError)
	set.ParseErrorsAllowlist.UnknownFlags = true
	set.SetOutput(io.Discard)
	set.Usage = func() {}
	asJSON := set.Bool("json", false, "")
	pretty := set.Bool("pretty", false, "")
	readOnly := set.Bool("read-only", false, "")
	// A parse failure here is not fatal: the real parse happens in Cobra and
	// reports the error properly.
	_ = set.Parse(args)
	if *asJSON {
		a.renderer.Format = FormatJSON
	}
	a.renderer.Pretty = *pretty
	a.mode.ReadOnly = *readOnly || envReadOnly()
}

// envReadOnly reads the environment switch. Anything but an explicit off
// counts as on: someone who sets this variable at all means to restrict the
// run, and a typo must not quietly grant write access.
func envReadOnly() bool {
	value := strings.TrimSpace(os.Getenv(EnvReadOnly))
	switch strings.ToLower(value) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
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
