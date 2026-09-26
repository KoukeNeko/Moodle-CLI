package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

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
	// In is where a command reads piped input, such as a token or password.
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Renderer writes results and errors in the selected format.
type Renderer struct {
	Streams Streams
	Format  Format
	Pretty  bool
	// Fields narrows the JSON data to these fields; empty keeps them all.
	Fields []string
	// NoInput means nobody may be asked anything: a command that would wait
	// for an answer fails instead, so an unattended caller never hangs.
	NoInput bool
}

// Result is what a command produces: an envelope plus the human rendering of
// the same data. Human is only called for FormatTable.
type Result struct {
	Envelope v1.Envelope
	Human    func(w io.Writer) error
}

// Render writes a successful result.
//
// A partial answer is announced on stderr after the data. Moodle filters a
// successful reply rather than refusing it — a course the account cannot read,
// an activity a restriction withholds — and the contract has carried that as
// meta.partial all along, where only a --json reader ever saw it. A person
// reading a short list had no way to tell it was short. Stderr keeps the notice
// out of the data, so a pipe still receives exactly the rows.
func (r Renderer) Render(res Result) error {
	if r.Format == FormatJSON {
		envelope, err := selectFields(res.Envelope, r.Fields)
		if err != nil {
			return err
		}
		return r.writeJSON(envelope)
	}
	if res.Human == nil {
		return nil
	}
	if err := res.Human(r.Streams.Out); err != nil {
		return err
	}
	if meta := res.Envelope.Meta; meta.Partial {
		if len(meta.Missing) > 0 {
			// Naming the fields is what makes the note useful: a blank due
			// date means something else once the reader knows the route could
			// not see due dates at all.
			fmt.Fprintf(r.Streams.Err,
				"Note: this answer is incomplete; the route that answered (%s) cannot see: %s.\n",
				meta.Source, strings.Join(meta.Missing, ", "))
		} else {
			fmt.Fprintln(r.Streams.Err,
				"Note: the site left something out of this answer, so it is incomplete.")
		}
	}
	return nil
}

// reported marks a failure whose output has already been written. Doctor
// prints its report and then has to exit non-zero; without this it would emit
// a second JSON document and break the one-document rule.
type reported struct{ code int }

func (r reported) Error() string { return "already reported" }

// Reported wraps an exit code for a failure that has already been rendered.
func Reported(code int) error { return reported{code: code} }

// RenderError writes a failed result and returns the process exit code.
//
// With --json the error envelope goes to stdout, so a consumer parses one
// stream and one document whatever the outcome. Without it, the message goes
// to stderr where humans expect it.
func (r Renderer) RenderError(err error) int {
	if err == nil {
		return v1.ExitOK
	}
	var already reported
	if errors.As(err, &already) {
		return already.code
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
