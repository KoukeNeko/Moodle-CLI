// Package v1 is the only public machine-readable representation of this tool.
// Everything here is a compatibility promise.
//
// Internals may be refactored freely; this package may not change except by
// the compatibility rules.
package v1

import "github.com/KoukeNeko/moodle-cli/internal/errs"

// SchemaVersion is the major version of the JSON contract.
const SchemaVersion = 1

// KindError is the kind used by every error envelope.
const KindError = "error"

// Source names which backend produced the data.
type Source string

const (
	SourceWS    Source = "ws"
	SourceAJAX  Source = "ajax"
	SourceHTML  Source = "html"
	SourceLocal Source = "local" // produced without talking to Moodle
)

// Meta accompanies every successful response. Every field is always present;
// a field with no value is null.
type Meta struct {
	Site    *string `json:"site"`
	Account *string `json:"account"`
	Source  Source  `json:"source"`
	Partial bool    `json:"partial"`
	// Missing lists contract field names that could not be retrieved. It is
	// never null — an empty list means nothing was missing.
	Missing    []string `json:"missing"`
	NextCursor *string  `json:"next_cursor"`
}

// NewMeta returns a Meta for data produced locally, with no site or account.
func NewMeta(source Source) Meta {
	return Meta{Source: source, Missing: []string{}}
}

// Envelope is the successful response shape.
type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Data          any    `json:"data"`
	Meta          Meta   `json:"meta"`
}

// NewEnvelope builds a successful envelope of the given kind.
func NewEnvelope(kind string, data any, meta Meta) Envelope {
	if meta.Missing == nil {
		meta.Missing = []string{}
	}
	return Envelope{SchemaVersion: SchemaVersion, Kind: kind, Data: data, Meta: meta}
}

// UpstreamBody is Moodle's own error information. Diagnostic only: not part of
// the stable contract.
type UpstreamBody struct {
	Exception string `json:"exception"`
	ErrorCode string `json:"errorcode"`
	Message   string `json:"message"`
}

// ErrorBody is the error detail. Message and Hint are English prose for
// humans; programs branch on Code, Reason and Outcome.
type ErrorBody struct {
	Code      errs.Code    `json:"code"`
	Reason    *errs.Reason `json:"reason"`
	Outcome   errs.Outcome `json:"outcome"`
	Retryable bool         `json:"retryable"`
	Message   string       `json:"message"`
	Hint      *string      `json:"hint"`
	Upstream  *UpstreamBody `json:"upstream"`
}

// ErrorEnvelope is the failure response shape. With --json it goes to stdout,
// exactly like a success.
type ErrorEnvelope struct {
	SchemaVersion int       `json:"schema_version"`
	Kind          string    `json:"kind"`
	Error         ErrorBody `json:"error"`
}

// NewErrorEnvelope converts an error into its wire form.
func NewErrorEnvelope(err error) ErrorEnvelope {
	e := errs.From(err)
	body := ErrorBody{
		Code:      e.Code,
		Outcome:   e.EffectiveOutcome(),
		Retryable: e.Retryable,
		Message:   e.Error(),
	}
	if e.Reason != "" {
		reason := e.Reason
		body.Reason = &reason
	}
	if e.Hint != "" {
		hint := e.Hint
		body.Hint = &hint
	}
	if e.Upstream != nil {
		body.Upstream = &UpstreamBody{
			Exception: e.Upstream.Exception,
			ErrorCode: e.Upstream.ErrorCode,
			Message:   e.Upstream.Message,
		}
	}
	return ErrorEnvelope{SchemaVersion: SchemaVersion, Kind: KindError, Error: body}
}
