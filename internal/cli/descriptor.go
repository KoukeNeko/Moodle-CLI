package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Safety says what running a command can change. An unattended caller runs
// read commands freely, weighs local ones, and needs a decision for writes.
const (
	// safetyRead changes nothing, on Moodle or on this machine.
	safetyRead = "read"
	// safetyLocal changes only this machine: configuration, the keychain, a
	// downloaded file.
	safetyLocal = "local"
	// safetyWrite can change something on Moodle.
	safetyWrite = "write"
)

// Idempotency says whether a failed or unconfirmed run may simply be repeated.
const (
	idempotent    = "idempotent"
	nonIdempotent = "non_idempotent"
)

// annotationSafety overrides the safety derived from annotationMutates. It is
// set only on commands whose effect stays on this machine: "mutates" there
// decides what read-only mode withholds, which is a different question.
const annotationSafety = "moodle.safety"

// safetyOf derives a command's safety from its annotations.
func safetyOf(cmd *cobra.Command) string {
	if value := cmd.Annotations[annotationSafety]; value != "" {
		return value
	}
	if cmd.Annotations[annotationMutates] == "true" {
		return safetyWrite
	}
	return safetyRead
}

// idempotencyOf is non-idempotent for every write. Moodle functions carry no
// idempotency key, and the reviewed registry marks all 381 core writes as
// never to be retried; a command built on them cannot promise more.
func idempotencyOf(safety string) string {
	if safety == safetyWrite {
		return nonIdempotent
	}
	return idempotent
}

// commandSchema is what `moodle schema <command>` returns: enough for an agent
// to decide whether it may run a command unattended, and to build and check
// the call, without reading help text.
type commandSchema struct {
	Command     string  `json:"command"`
	Description string  `json:"description"`
	Kind        *string `json:"kind"`
	Safety      string  `json:"safety"`
	Idempotency string  `json:"idempotency"`
	// Input describes the command's own flags by name, and its positional
	// arguments as "args". The global flags are the same for every command
	// and are described in the JSON contract instead.
	Input map[string]any `json:"input_schema"`
	// Output is the published schema of the command's kind, null for a
	// command that has none.
	Output json.RawMessage `json:"output_schema"`
}

const jsonSchemaDraft = "https://json-schema.org/draft/2020-12/schema"

// findCommand resolves a command path such as ["assignment", "submit"].
func findCommand(root *cobra.Command, words []string) (*cobra.Command, error) {
	current := root
	for _, word := range words {
		var next *cobra.Command
		// Hidden commands count: read-only mode hides every write, and an
		// agent running under it still needs to learn that a command is one.
		for _, child := range current.Commands() {
			if child.Name() == word {
				next = child
			}
		}
		if next == nil {
			return nil, errs.New(errs.CodeUsage,
				fmt.Sprintf("no command %q", strings.Join(words, " "))).
				WithHint("list them with `moodle commands`")
		}
		current = next
	}
	if !current.Runnable() {
		var names []string
		for _, child := range current.Commands() {
			if child.IsAvailableCommand() {
				names = append(names, child.Name())
			}
		}
		return nil, errs.New(errs.CodeUsage,
			fmt.Sprintf("%q is a group of commands", strings.Join(words, " "))).
			WithHint("name one of: " + strings.Join(names, ", "))
	}
	return current, nil
}

// describeCommand builds the descriptor for one runnable command.
func describeCommand(cmd *cobra.Command, path string) commandSchema {
	safety := safetyOf(cmd)
	out := commandSchema{
		Command:     path,
		Description: cmd.Short,
		Kind:        annotationValue(cmd, annotationKind),
		Safety:      safety,
		Idempotency: idempotencyOf(safety),
		Input:       inputSchema(cmd),
		Output:      json.RawMessage("null"),
	}
	if out.Kind != nil {
		if doc, err := v1.Schema(*out.Kind); err == nil {
			out.Output = doc
		}
	}
	return out
}

// inputSchema describes a command's flags and positional arguments.
func inputSchema(cmd *cobra.Command) map[string]any {
	properties := map[string]any{}
	var required []string
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		properties[f.Name] = flagSchema(f)
		if values, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok && len(values) > 0 && values[0] == "true" {
			required = append(required, f.Name)
		}
	})
	if args := positionalSchema(cmd.Use); args != nil {
		properties["args"] = args
		if minimum, _ := args["minItems"].(int); minimum > 0 {
			required = append(required, "args")
		}
	}
	schema := map[string]any{
		"$schema":              jsonSchemaDraft,
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}
	return schema
}

// flagSchema maps a flag's value type onto JSON Schema.
func flagSchema(f *pflag.Flag) map[string]any {
	schema := map[string]any{"description": f.Usage}
	switch f.Value.Type() {
	case "bool":
		schema["type"] = "boolean"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "count":
		schema["type"] = "integer"
	case "float32", "float64":
		schema["type"] = "number"
	case "stringSlice", "stringArray", "intSlice":
		schema["type"] = "array"
		schema["items"] = map[string]any{"type": "string"}
	default:
		schema["type"] = "string"
	}
	switch f.DefValue {
	case "", "[]", "false", "0":
	default:
		schema["default"] = f.DefValue
	}
	return schema
}

// positionalSchema reads the positional arguments out of a Use line such as
// "submit <assignment-id|url> <file>...": <x> is required, [x] optional, and
// a trailing ... repeats. Nil when the command takes none.
func positionalSchema(use string) map[string]any {
	words := strings.Fields(use)
	if len(words) < 2 {
		return nil
	}
	var names []string
	minimum, variadic := 0, false
	for _, word := range words[1:] {
		repeats := strings.HasSuffix(word, "...")
		word = strings.TrimSuffix(word, "...")
		switch {
		case strings.HasPrefix(word, "<") && strings.HasSuffix(word, ">"):
			minimum++
		case strings.HasPrefix(word, "[") && strings.HasSuffix(word, "]"):
		default:
			continue
		}
		if repeats {
			names = append(names, word+"...")
		} else {
			names = append(names, word)
		}
		variadic = variadic || repeats
	}
	if len(names) == 0 {
		return nil
	}
	schema := map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "positional arguments, in order: " + strings.Join(names, " "),
		"minItems":    minimum,
	}
	if !variadic {
		schema["maxItems"] = len(names)
	}
	return schema
}

func writeCommandSchemaHuman(w io.Writer, schema commandSchema) error {
	kind := "-"
	if schema.Kind != nil {
		kind = *schema.Kind
	}
	fmt.Fprintf(w, "%s — %s\n\n", schema.Command, schema.Description)
	table := newTable(w)
	fmt.Fprintf(table, "Safety\t%s\n", schema.Safety)
	fmt.Fprintf(table, "Idempotency\t%s\n", schema.Idempotency)
	fmt.Fprintf(table, "Output kind\t%s\n", kind)
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "\nAdd --json for the input and output JSON Schema.")
	return err
}
