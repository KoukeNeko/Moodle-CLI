package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/wsregistry"
)

func newWSCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ws",
		Short: "Inspect and call typed Moodle core web services",
		Long: "The typed registry is generated from Moodle 4.5, 5.1 and 5.2 test " +
			"sites. It validates parameters and identifies effects before a request " +
			"is sent. Third-party plugin functions remain available through `api call`.",
	}
	cmd.AddCommand(
		newWSListCommand(r, deps),
		newWSDescribeCommand(r, deps),
		newWSCallCommand(r, deps, mode),
	)
	return cmd
}

func requireWSRegistry(deps Deps) (*wsregistry.Registry, error) {
	if deps.WSRegistry == nil {
		return nil, errs.New(errs.CodeInternal, "the typed web-service registry is not configured")
	}
	return deps.WSRegistry, nil
}

func newWSListCommand(r *Renderer, deps Deps) *cobra.Command {
	var version, component, effect, match string
	var destructive, credential, deprecated bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the core function union for Moodle 4.5, 5.1 and 5.2",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "ws.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := requireWSRegistry(deps)
			if err != nil {
				return err
			}
			if version != "" && version != "v45" && version != "v51" && version != "v52" {
				return errs.New(errs.CodeUsage, "--version must be v45, v51 or v52")
			}
			if effect != "" && effect != "read" && effect != "write" {
				return errs.New(errs.CodeUsage, "--effect must be read or write")
			}
			functions := make([]wsregistry.Function, 0)
			for _, function := range registry.All() {
				if version != "" && !contains(function.Versions, version) {
					continue
				}
				if component != "" && function.Component != component {
					continue
				}
				if effect != "" && string(function.Effect) != effect {
					continue
				}
				if match != "" && !strings.Contains(function.Name, match) {
					continue
				}
				if destructive && !function.Destructive {
					continue
				}
				if credential && !function.Credential {
					continue
				}
				if deprecated && len(function.DeprecatedIn) == 0 {
					continue
				}
				functions = append(functions, function)
			}
			envelope := v1.WSList(registry, functions)
			return r.Render(Result{Envelope: envelope, Human: func(w io.Writer) error {
				return writeWSList(w, functions, len(registry.All()), registry.Digest())
			}})
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "only functions installed by v45, v51 or v52")
	cmd.Flags().StringVar(&component, "component", "", "only this Moodle component")
	cmd.Flags().StringVar(&effect, "effect", "", "only read or write functions")
	cmd.Flags().StringVar(&match, "match", "", "only names containing this text")
	cmd.Flags().BoolVar(&destructive, "destructive", false, "only destructive writes")
	cmd.Flags().BoolVar(&credential, "credential", false, "only functions that issue or change credentials")
	cmd.Flags().BoolVar(&deprecated, "deprecated", false, "only functions deprecated in at least one version")
	return cmd
}

func newWSDescribeCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "describe <function>",
		Short:       "Show versioned parameters, returns, effects and requirements",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "ws.describe"},
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := requireWSRegistry(deps)
			if err != nil {
				return err
			}
			function, ok := registry.Lookup(args[0])
			if !ok {
				return errs.New(errs.CodeNotFound, args[0]+" is not in the core registry").
					WithHint("use `moodle api functions` to discover third-party plugin functions")
			}
			envelope := v1.WSDescribe(function)
			return r.Render(Result{Envelope: envelope, Human: func(w io.Writer) error {
				return writeWSDescription(w, function)
			}})
		},
	}
	return cmd
}

func newWSCallCommand(r *Renderer, deps Deps, mode *safety.Mode) *cobra.Command {
	var flags sessionFlags
	var params []string
	var paramsJSON string
	var allowWrite, dryRun bool
	cmd := &cobra.Command{
		Use:   "call <function>",
		Short: "Validate and call one registered core function",
		Long: "Validates parameters against the selected Moodle version's generated " +
			"schema before sending anything. Reads run directly. Writes require " +
			"--allow-write and are never retried; --dry-run validates and shows the plan.",
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			annotationKind: "ws.call",
			// Dynamic: registered reads remain usable under global --read-only,
			// while the service rejects registered writes.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := collectTypedParams(params, paramsJSON)
			if err != nil {
				return err
			}
			if _, err := requireWSRegistry(deps); err != nil {
				return err
			}
			if deps.WS == nil {
				return errs.New(errs.CodeInternal, "the typed web-service caller is not configured")
			}
			resolved, authed, err := openSession(cmd, deps, flags)
			if err != nil {
				return err
			}
			effective := safety.Mode{DryRun: dryRun, ReadOnly: mode.ReadOnly}
			result, err := deps.WS(authed, effective, allowWrite).Call(
				cmd.Context(), resolved.capabilities, resolved.capabilities.Release,
				args[0], values, dryRun)
			if err != nil {
				return err
			}
			envelope := v1.WSCall(result, resolved.resolved.SiteName, resolved.resolved.AccountName)
			return r.Render(Result{Envelope: envelope, Human: func(w io.Writer) error {
				return writeWSCall(w, result)
			}})
		},
	}
	flags.bind(cmd, "call")
	cmd.Flags().StringArrayVar(&params, "param", nil, "a parameter as name=value, repeatable")
	cmd.Flags().StringVar(&paramsJSON, "params-json", "", "parameters as a JSON object, including nested structures")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and show the request without sending it")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "permit a registered function whose effect is write")
	return cmd
}

// collectTypedParams keeps --param convenient without weakening registry
// validation. Values that are valid JSON literals receive their natural type
// (courseid=42, enabled=false, ids=[1,2]); ordinary text remains text. The
// untyped api call deliberately continues to use collectParams because plugin
// functions commonly rely on Moodle's bracket-notation string fields.
func collectTypedParams(pairs []string, raw string) (map[string]any, error) {
	values, err := collectParams(nil, raw)
	if err != nil {
		return nil, err
	}
	for _, pair := range pairs {
		name, value, found := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, errs.New(errs.CodeUsage,
				fmt.Sprintf("--param %q is not name=value", pair))
		}
		var typed any
		if json.Unmarshal([]byte(value), &typed) == nil {
			values[name] = typed
		} else {
			values[name] = value
		}
	}
	return values, nil
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func writeWSList(w io.Writer, functions []wsregistry.Function, total int, digest string) error {
	table := newTable(w)
	fmt.Fprintln(table, "FUNCTION\tCOMPONENT\tVERSIONS\tEFFECT\tFLAGS")
	for _, function := range functions {
		flags := []string{}
		if function.Destructive {
			flags = append(flags, "destructive")
		}
		if function.Credential {
			flags = append(flags, "credential")
		}
		if len(function.DeprecatedIn) > 0 {
			flags = append(flags, "deprecated:"+strings.Join(function.DeprecatedIn, ","))
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", function.Name, function.Component,
			strings.Join(function.Versions, ","), function.Effect, strings.Join(flags, ","))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "\n%d of %d union functions shown. Registry %s.\n", len(functions), total, digest[:12])
	return nil
}

func writeWSDescription(w io.Writer, function wsregistry.Function) error {
	fmt.Fprintf(w, "%s (%s)\n", function.Name, function.Component)
	fmt.Fprintf(w, "Versions: %s\nEffect: %s", strings.Join(function.Versions, ", "), function.Effect)
	if function.Destructive {
		fmt.Fprint(w, ", destructive")
	}
	if function.Credential {
		fmt.Fprint(w, ", credential")
	}
	fmt.Fprintln(w)
	versions := append([]string{}, function.Versions...)
	sort.Strings(versions)
	for _, version := range versions {
		variant := function.Variants[version]
		fmt.Fprintf(w, "\n[%s]\n", version)
		if variant.Description != nil && *variant.Description != "" {
			fmt.Fprintln(w, *variant.Description)
		}
		fmt.Fprintf(w, "Capabilities: %s\nServices: %s\nREST: %t  AJAX: %t  Login: %t\n",
			strings.Join(variant.Capabilities, ", "), strings.Join(variant.Services, ", "),
			variant.Transports.REST, variant.Transports.AJAX, variant.LoginRequired)
		parameters, _ := json.MarshalIndent(json.RawMessage(variant.Parameters), "", "  ")
		returns, _ := json.MarshalIndent(json.RawMessage(variant.Returns), "", "  ")
		fmt.Fprintf(w, "Parameters:\n%s\nReturns:\n%s\n", parameters, returns)
	}
	return nil
}

func writeWSCall(w io.Writer, result wsregistry.CallResult) error {
	if result.DryRun {
		fmt.Fprintf(w, "Would call %s using the %s schema (%s). Nothing was sent.\n",
			result.Function, result.Version, result.Effect)
		encoded, err := json.MarshalIndent(result.Params, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\n", encoded)
		if result.NeedsAllowWrite {
			fmt.Fprintln(w, "A real run requires --allow-write.")
		}
		return nil
	}
	var value any
	if err := json.Unmarshal(result.Response, &value); err != nil {
		_, err := w.Write(result.Response)
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", encoded)
	return err
}
