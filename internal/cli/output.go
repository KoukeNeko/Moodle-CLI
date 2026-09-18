package cli

import (
	"encoding/json"
	"fmt"
	"io"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Format selects how a result is rendered.
type Format string

const (
	// FormatTable is for humans. It is NOT part of the contract and may change
	// at any time.
	FormatTable Format = "table"
	// FormatJSON is the contract. stdout carries exactly one JSON document.
	FormatJSON Format = "json"
)

// Streams are the process streams, injected so tests can capture them.
//
// The split is a contract rule, not a convenience: stdout carries only the
// requested data, stderr only progress and diagnostics.
type Streams struct {
	Out io.Writer
	Err io.Writer
}

// Renderer writes results and errors in the selected format.
type Renderer struct {
	Streams Streams
	Format  Format
	Pretty  bool
}

// Result is what a command produces: an envelope plus the human rendering of
// the same data. Human is only called for FormatTable.
type Result struct {
	Envelope v1.Envelope
	Human    func(w io.Writer) error
}

// Render writes a successful result.
func (r Renderer) Render(res Result) error {
	if r.Format == FormatJSON {
		return r.writeJSON(res.Envelope)
	}
	if res.Human == nil {
		return nil
	}
	return res.Human(r.Streams.Out)
}

// RenderError writes a failed result and returns the process exit code.
//
// With --json the error envelope goes to stdout, so a consumer parses one
// stream and one document whatever the outcome. Without it, the message goes
// to stderr where humans expect it.
func (r Renderer) RenderError(err error) int {
	if err == nil {
		return v1.ExitOK
	}
	if r.Format == FormatJSON {
		if writeErr := r.writeJSON(v1.NewErrorEnvelope(err)); writeErr != nil {
			fmt.Fprintf(r.Streams.Err, "moodle: cannot write error output: %v\n", writeErr)
			return v1.ExitInternal
		}
		return v1.ExitCode(err)
	}

	e := errs.From(err)
	fmt.Fprintf(r.Streams.Err, "Error: %s\n", e.Error())
	if e.Hint != "" {
		fmt.Fprintf(r.Streams.Err, "Hint: %s\n", e.Hint)
	}
	if e.EffectiveOutcome() == errs.OutcomeAmbiguous {
		fmt.Fprintln(r.Streams.Err,
			"The request may already have been applied. Check the current state before retrying.")
	}
	return v1.ExitCode(err)
}

func (r Renderer) writeJSON(doc any) error {
	enc := json.NewEncoder(r.Streams.Out)
	if r.Pretty {
		enc.SetIndent("", "  ")
	}
	// Moodle content may contain characters Go would otherwise escape as
	// & etc.; keep the output readable and byte-faithful.
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}
