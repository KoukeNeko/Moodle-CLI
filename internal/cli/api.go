package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/api"
	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

func newAPICommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Work with the site's web service functions directly",
	}
	cmd.AddCommand(
		newAPIFunctionsCommand(r, deps),
		newAPICallCommand(r, deps, mode),
	)
	return cmd
}

// openSession is the part every api command needs.
func openSession(cmd *cobra.Command, deps Deps, flags sessionFlags) (*resolvedSession, *auth.Session, error) {
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
	return &resolvedSession{resolved: resolved, capabilities: capabilities}, session, nil
}

func newAPIFunctionsCommand(r *Renderer, deps Deps) *cobra.Command {
	var (
		flags      sessionFlags
		writesOnly bool
		unreviewed bool
		nameFilter string
	)
	cmd := &cobra.Command{
		Use:   "functions",
		Short: "List the functions this site exposes, and what is known about them",
		Long: "Lists what the site actually offers, with what this project knows about\n" +
			"each one: whether it has been reviewed, whether it can change anything,\n" +
			"and whether a failed call may be repeated.\n\n" +
			"A function that has not been reviewed is assumed to write and is never\n" +
			"retried. That is not a gap in the list, it is the answer.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "api.functions"},
		RunE: func(cmd *cobra.Command, args []string) error {
			session, _, err := openSession(cmd, deps, flags)
			if err != nil {
				return err
			}
			functions := api.Functions(session.capabilities)

			filtered := functions[:0]
			for _, item := range functions {
				if writesOnly && !item.Mutates {
					continue
				}
				if unreviewed && item.Reviewed {
					continue
				}
				if nameFilter != "" && !strings.Contains(item.Name, nameFilter) {
					continue
				}
				filtered = append(filtered, item)
			}

			envelope := v1.APIFunctions(filtered,
				session.resolved.SiteName, session.resolved.AccountName)
			listed, _ := envelope.Data.([]v1.APIFunction)
			return r.Render(Result{
				Envelope: envelope,
				Human: func(w io.Writer) error {
					return writeFunctionTable(w, listed, len(functions))
				},
			})
		},
	}
	flags.bind(cmd, "list functions from")
	cmd.Flags().BoolVar(&writesOnly, "writes", false, "only functions that can change something")
	cmd.Flags().BoolVar(&unreviewed, "unreviewed", false, "only functions this project has not reviewed")
	cmd.Flags().StringVar(&nameFilter, "match", "", "only functions whose name contains this")
	return cmd
}

func newAPICallCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	var (
		flags      sessionFlags
		params     []string
		paramsJSON string
		allowWrite bool
		dryRun     bool
	)
	cmd := &cobra.Command{
		Use:   "call <function>",
		Short: "Call one web service function directly",
		Long: "Calls a function and prints exactly what the site returned. This is the\n" +
			"way out of the typed commands, for anything they do not cover — a site's\n" +
			"plugins can expose functions this project has never heard of.\n\n" +
			"There is no typed contract for an arbitrary function, so the response is\n" +
			"passed through unread. Anything that can change something needs\n" +
			"--allow-write, and a function that has not been reviewed counts as one\n" +
			"that can: a plugin may do anything, and the alternative is guessing with\n" +
			"your own coursework.",
		Example: "  moodle api call core_enrol_get_users_courses --param userid=4\n" +
			"  moodle api call core_course_get_contents --params-json '{\"courseid\": 2}'",
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			annotationKind: "api.call",
			// It can write, so read-only mode withholds it. The typed commands
			// remain available there.
			annotationMutates: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := collectParams(params, paramsJSON)
			if err != nil {
				return err
			}
			session, authed, err := openSession(cmd, deps, flags)
			if err != nil {
				return err
			}

			effective := safety.Mode{DryRun: dryRun, ReadOnly: mode.ReadOnly}
			result, err := deps.API(authed, effective, allowWrite).
				Call(cmd.Context(), session.capabilities, args[0], values, dryRun)
			if err != nil {
				return err
			}

			envelope := v1.APICall(result,
				session.resolved.SiteName, session.resolved.AccountName)
			payload, _ := envelope.Data.(v1.APICallResult)
			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeCallResult(w, payload) },
			})
		},
	}
	flags.bind(cmd, "call")
	cmd.Flags().StringArrayVar(&params, "param", nil,
		"a parameter as name=value, repeatable; Moodle's bracket notation works, such as courseids[0]=2")
	cmd.Flags().StringVar(&paramsJSON, "params-json", "",
		"all parameters as a JSON object, for anything with nested structure")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false,
		"permit a function that can change something")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"show what would be sent without sending it")
	return cmd
}

// collectParams turns the two ways of giving parameters into one map.
//
// The JSON form goes in first so a --param can override a single field of it,
// which is how someone iterates on a call.
func collectParams(pairs []string, raw string) (map[string]any, error) {
	values := map[string]any{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, errs.Wrap(errs.CodeUsage, err, "--params-json is not a JSON object")
		}
	}
	for _, pair := range pairs {
		name, value, found := strings.Cut(pair, "=")
		if !found || strings.TrimSpace(name) == "" {
			return nil, errs.New(errs.CodeUsage,
				fmt.Sprintf("--param %q is not name=value", pair))
		}
		values[strings.TrimSpace(name)] = value
	}
	return values, nil
}

func writeFunctionTable(w io.Writer, functions []v1.APIFunction, total int) error {
	if len(functions) == 0 {
		_, err := fmt.Fprintf(w, "No matching functions (the site offers %d).\n", total)
		return err
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "FUNCTION\tEFFECT\tRETRY\tREVIEWED")
	for _, item := range functions {
		effect := "read"
		if item.Mutates {
			effect = "WRITES"
		}
		reviewed := "yes"
		if !item.Reviewed {
			// Said plainly: this is an assumption, not a finding.
			reviewed = "no — assumed to write"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", item.Name, effect, item.Retry, reviewed)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if len(functions) != total {
		fmt.Fprintf(w, "\n%d of %d functions shown.\n", len(functions), total)
	}
	return nil
}

func writeCallResult(w io.Writer, result v1.APICallResult) error {
	if result.DryRun {
		effect := "reads"
		if result.Mutates {
			effect = "CAN WRITE"
		}
		fmt.Fprintf(w, "Would call %s (%s). Nothing was sent.\n", result.Function, effect)
		if len(result.Params) > 0 {
			encoded, err := json.MarshalIndent(result.Params, "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "  %s\n", encoded)
		}
		if result.NeedsAllowWrite {
			fmt.Fprintln(w, "\nA real run would stop here: pass --allow-write to go ahead.")
		}
		return nil
	}
	// The site's own shape, printed as it arrived: there is nothing to
	// tabulate when the shape is not known in advance.
	var pretty any
	if err := json.Unmarshal(result.Response, &pretty); err != nil {
		_, err := w.Write(result.Response)
		return err
	}
	encoded, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", encoded)
	return err
}
