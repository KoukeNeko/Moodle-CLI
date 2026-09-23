package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Handler runs one tool.
//
// It returns the contract envelope the equivalent command would produce. An
// agent and a person asking the same question get the same answer, in the same
// shape, with the same guarantees.
type Handler func(ctx context.Context, args Arguments) (v1.Envelope, error)

// Arguments are a tool call's arguments, with the reading this server does of
// them kept in one place: an agent sends whatever JSON it likes, and a number
// may arrive as 4, "4" or 4.0.
type Arguments map[string]any

// String reads a string argument. A number is accepted and rendered, because
// ids are strings in this contract but an agent will often send them as
// numbers.
func (a Arguments) String(name string) string {
	switch value := a[name].(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(value)
	case float64:
		// JSON has one number type; an id is always integral.
		if value == float64(int64(value)) {
			return fmt.Sprintf("%d", int64(value))
		}
		return fmt.Sprintf("%v", value)
	case bool:
		return fmt.Sprintf("%t", value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// Required reads an argument that must be there.
func (a Arguments) Required(name string) (string, error) {
	value := a.String(name)
	if value == "" {
		return "", errs.New(errs.CodeUsage, fmt.Sprintf("%s is required", name))
	}
	return value, nil
}

// Strings reads a list argument, tolerating a single value where a list is
// expected.
func (a Arguments) Strings(name string) []string {
	switch value := a[name].(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, Arguments{"x": item}.String("x"))
		}
		return out
	default:
		if single := a.String(name); single != "" {
			return []string{single}
		}
		return nil
	}
}

// Int reads a whole-number argument, tolerating the string form.
func (a Arguments) Int(name string) int {
	switch value := a[name].(type) {
	case float64:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// Bool reads a flag, accepting the string forms an agent may send.
func (a Arguments) Bool(name string) bool {
	switch value := a[name].(type) {
	case bool:
		return value
	case string:
		lowered := strings.ToLower(strings.TrimSpace(value))
		return lowered == "true" || lowered == "yes" || lowered == "1"
	default:
		return false
	}
}

// Object reads a JSON object argument.
func (a Arguments) Object(name string) map[string]any {
	value, _ := a[name].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

// definition is one tool in the Registry.
type definition struct {
	Name        string
	Title       string
	Description string
	// Schema is the tool's input schema. It is written here rather than
	// generated from the contract because the contract describes answers, and
	// a tool's arguments are a different thing.
	Schema  json.RawMessage
	Mutates bool
	// Destructive marks a tool that replaces something that was there. A
	// submission overwrites whatever was saved before.
	Destructive bool
	Handler     Handler
}

// Registry is the tool surface of one session.
//
// It is built once at startup: which tools exist depends on whether this
// session may write, and that does not change while it runs.
type Registry struct {
	tools []definition
	// allowWrite records whether this session may change anything. When it is
	// false the writing tools are not registered at all: a tool an agent
	// cannot see is one it cannot decide to try.
	allowWrite bool
	// siteName names the site in the one message an agent cannot act on.
	siteName string
}

func newRegistry(allowWrite bool, siteName string) *Registry {
	return &Registry{allowWrite: allowWrite, siteName: siteName}
}

// add registers a tool, dropping a writing one when this session is read-only.
func (r *Registry) add(tool definition) {
	if tool.Mutates && !r.allowWrite {
		return
	}
	r.tools = append(r.tools, tool)
}

func (r *Registry) describe() []tool {
	out := make([]tool, 0, len(r.tools))
	for _, item := range r.tools {
		out = append(out, tool{
			Name:        item.Name,
			Title:       item.Title,
			Description: item.Description,
			InputSchema: item.Schema,
			Annotations: &toolAnnotations{
				Title:        item.Title,
				ReadOnlyHint: !item.Mutates,
				// No Moodle write carries an idempotency key. Destructive is
				// supplied by the use case or generated function registry.
				DestructiveHint: item.Destructive,
				IdempotentHint:  !item.Mutates,
				OpenWorldHint:   true,
			},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// instructions tell the agent what kind of server this is.
func (r *Registry) instructions() string {
	var b strings.Builder
	b.WriteString("Read and act on a Moodle site within the signed-in account's capabilities. ")
	b.WriteString("The account may be a learner, educator, course creator, manager, administrator, or a custom role.\n\n")
	b.WriteString("Ids are strings. A timestamp is RFC 3339 in UTC, or null when the site ")
	b.WriteString("sets none — Moodle's 0 means \"unset\", not 1970.\n\n")
	b.WriteString("For assignments: saving work and handing it in are separate steps on ")
	b.WriteString("some assignments and one step on others, and the assignment decides ")
	b.WriteString("which. Check needs_hand_in, and trust handed_in over any step that ")
	b.WriteString("appeared to succeed.\n\n")
	if r.allowWrite {
		b.WriteString("This session MAY change things on the site. Confirm the intended ")
		b.WriteString("scope and target before calling a writing tool; inspect its annotations ")
		b.WriteString("for destructive and idempotency properties. Some writes cannot be undone.")
	} else {
		b.WriteString("This session is read-only: no tool here can change anything on the ")
		b.WriteString("site. If a write is needed, a person must restart it explicitly with ")
		b.WriteString("`moodle mcp serve --allow-write`.")
	}
	return b.String()
}

// call runs a tool and turns its outcome into a result the agent can read.
func (r *Registry) call(ctx context.Context, name string, args Arguments) callToolResult {
	for _, item := range r.tools {
		if item.Name != name {
			continue
		}
		envelope, err := item.Handler(ctx, args)
		if err != nil {
			return errorResult(r.humanReadable(err))
		}
		return successResult(envelope)
	}

	// An unknown tool is a tool error rather than a protocol error: the client
	// asked a valid question and the answer is "no such tool", which it can
	// act on. Being told it can act on the read-only case matters.
	message := fmt.Sprintf("no tool named %q", name)
	if !r.allowWrite {
		message += "; this session is read-only, so tools that change anything are not offered"
	}
	return callToolResult{Content: textContent(message), IsError: true}
}

// successResult carries the envelope both ways: as structured data for the
// agent to read, and as text for a model that only sees the transcript.
func successResult(envelope v1.Envelope) callToolResult {
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return callToolResult{
			Content: textContent("cannot encode the answer: " + err.Error()),
			IsError: true,
		}
	}
	return callToolResult{
		Content:           textContent(string(encoded)),
		StructuredContent: envelope,
	}
}

// humanReadable rewrites the one hint an agent cannot act on.
//
// "Sign in again with `moodle auth login`" is written for a person at a
// terminal. An agent has no terminal and no way to authenticate, so as advice
// it is worse than nothing: it reads as something the agent could try.
//
// The replacement says three things separately, because they are three
// different facts: this cannot be fixed from here, a person can fix it, and
// afterwards this process picks the new credential up on its own. The last one
// is only true because the session re-reads its credential when Moodle rejects
// the one it has — without that, a restart really would be required.
func (r *Registry) humanReadable(err error) error {
	e := errs.From(err)
	if e.Code != errs.CodeAuthentication {
		return err
	}
	where := "in a terminal"
	if r.siteName != "" {
		where = "in a terminal: `moodle auth login --site " + r.siteName + "`"
	}
	return e.WithHint("this cannot be fixed from here — the credential has to be " +
		"renewed by a person " + where + ". Once it is, retry: this server reads " +
		"the new credential itself and does not need restarting.")
}

// errorResult reports a failed tool.
//
// The error envelope is the same one the CLI emits, so an agent sees the same
// code, reason and hint a person would — including an ambiguous outcome, which
// it must not treat as a plain failure to retry.
func errorResult(err error) callToolResult {
	envelope := v1.NewErrorEnvelope(err)
	encoded, marshalErr := json.MarshalIndent(envelope, "", "  ")
	if marshalErr != nil {
		return callToolResult{Content: textContent(err.Error()), IsError: true}
	}
	return callToolResult{
		Content:           textContent(string(encoded)),
		StructuredContent: envelope,
		IsError:           true,
	}
}
