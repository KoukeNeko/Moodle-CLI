// Package errs holds the error vocabulary shared by every layer: the stable
// error Code, the open-ended Reason, and the execution Outcome.
//
// It is the lowest package in the dependency graph — it imports only the
// standard library, so anything may depend on it without creating a cycle.
// The mapping from these values to JSON and to process exit codes lives in
// internal/contract/v1; this package knows nothing about presentation.
package errs

import (
	"errors"
	"fmt"
)

// Code is the stable error class. The set is closed: adding a value is a
// breaking change to the JSON contract.
type Code string

const (
	CodeUsage            Code = "usage"
	CodeConfiguration    Code = "configuration"
	CodeAuthentication   Code = "authentication"
	CodePermissionDenied Code = "permission_denied"
	CodeNotFound         Code = "not_found"
	CodeValidation       Code = "validation"
	CodeConflict         Code = "conflict"
	CodeUnavailable      Code = "unavailable"
	CodeNetwork          Code = "network"
	CodeUpstream         Code = "upstream"
	CodeInternal         Code = "internal"
)

// Codes returns every valid Code, in the order used by the exit-code table.
func Codes() []Code {
	return []Code{
		CodeUsage, CodeConfiguration, CodeAuthentication, CodePermissionDenied,
		CodeNotFound, CodeValidation, CodeConflict, CodeUnavailable,
		CodeNetwork, CodeUpstream, CodeInternal,
	}
}

// Valid reports whether c is a known Code.
func (c Code) Valid() bool {
	for _, known := range Codes() {
		if c == known {
			return true
		}
	}
	return false
}

// Reason narrows a Code. Unlike Code the set is open: new values may be added
// without breaking the contract, so consumers must tolerate unknown ones.
type Reason string

const (
	ReasonCapability             Reason = "capability"
	ReasonProtocolDrift          Reason = "protocol_drift"
	ReasonResponseLost           Reason = "response_lost"
	ReasonTokenExpired           Reason = "token_expired"
	ReasonMobileServicesDisabled Reason = "mobile_services_disabled"
	ReasonCredentialMissing      Reason = "credential_missing"
	// ReasonRateLimited means the site asked for fewer requests. It is a
	// reason to wait, never a reason to retry a write: the request may have
	// been refused before it ran or after.
	ReasonRateLimited Reason = "rate_limited"
)

// Outcome says whether the effect of a request is known. An ambiguous outcome
// means the request may or may not have been applied, and must never be
// retried automatically.
type Outcome string

const (
	OutcomeKnown     Outcome = "known"
	OutcomeAmbiguous Outcome = "ambiguous"
)

// Upstream carries Moodle's own error information. It is diagnostic only and
// is not part of the stable contract.
type Upstream struct {
	Exception string
	ErrorCode string
	Message   string
}

// Error is the single error type crossing package boundaries.
type Error struct {
	Code      Code
	Reason    Reason
	Outcome   Outcome
	Retryable bool
	// Message is English prose for humans. Programs must branch on Code,
	// Reason and Outcome, never on this text.
	Message  string
	Hint     string
	Upstream *Upstream
	Cause    error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := e.Message
	if msg == "" {
		msg = string(e.Code)
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.Cause)
	}
	return msg
}

// Unwrap exposes the wrapped cause so errors.Is and errors.As keep working
// down the chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// EffectiveOutcome normalises the zero value to OutcomeKnown.
func (e *Error) EffectiveOutcome() Outcome {
	if e == nil || e.Outcome == "" {
		return OutcomeKnown
	}
	return e.Outcome
}

// New builds an Error with a known outcome.
func New(code Code, message string) *Error {
	return &Error{Code: code, Outcome: OutcomeKnown, Message: message}
}

// Wrap builds an Error that keeps cause reachable through errors.Is/As.
func Wrap(code Code, cause error, message string) *Error {
	return &Error{Code: code, Outcome: OutcomeKnown, Message: message, Cause: cause}
}

// WithReason returns a copy carrying reason.
func (e *Error) WithReason(reason Reason) *Error {
	out := *e
	out.Reason = reason
	return &out
}

// WithHint returns a copy carrying a suggested next step for the user.
func (e *Error) WithHint(hint string) *Error {
	out := *e
	out.Hint = hint
	return &out
}

// Ambiguous returns a copy marked as an ambiguous outcome: the request may
// already have been applied upstream.
func (e *Error) Ambiguous() *Error {
	out := *e
	out.Outcome = OutcomeAmbiguous
	out.Retryable = false
	return &out
}

// From converts any error into an *Error. An error that is not already one is
// reported as CodeInternal, so an unclassified failure can never masquerade as
// a benign one.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Wrap(CodeInternal, err, "unexpected internal error")
}
