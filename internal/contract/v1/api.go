package v1

import (
	"encoding/json"

	"github.com/KoukeNeko/moodle-cli/internal/api"
)

// APIFunction is one function a site exposes, on the wire.
type APIFunction struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// Reviewed reports whether this project has looked at the function.
	// Anything unreviewed is assumed to write, which is why it matters.
	Reviewed bool `json:"reviewed"`
	Mutates  bool `json:"mutates"`
	// Retry is safe, auth-only or never. A write with no idempotency key —
	// which is every Moodle write — is never retried.
	Retry string `json:"retry"`
	// Why records the reasoning behind a reviewed entry, so a caller can judge
	// it rather than trust it. Null for anything unreviewed.
	Why *string `json:"why"`
}

// APIFunctions converts a function listing into its envelope.
func APIFunctions(functions []api.Function, siteName, accountName string) Envelope {
	out := make([]APIFunction, 0, len(functions))
	for _, item := range functions {
		out = append(out, APIFunction{
			Name:     item.Name,
			Version:  item.Version,
			Reviewed: item.Reviewed,
			Mutates:  item.Mutates,
			Retry:    item.Retry,
			Why:      optional(item.Why),
		})
	}
	meta := NewMeta(SourceWS)
	meta.Site = optional(siteName)
	meta.Account = optional(accountName)
	return NewEnvelope("api.functions", out, meta)
}

// APICallResult is the api.call payload.
type APICallResult struct {
	Function string `json:"function"`
	// Params is what was sent, so a dry run has something to show and a real
	// call can be reproduced.
	Params map[string]any `json:"params"`
	// Response is exactly what the site returned, unread. There is no typed
	// contract for an arbitrary function: this field is the one place in the
	// contract where the shape is the site's and not this project's.
	Response json.RawMessage `json:"response"`
	// DryRun reports that nothing was sent.
	DryRun bool `json:"dry_run"`
	// NeedsAllowWrite reports that a real run would be refused without
	// --allow-write. Always false outside a plan.
	NeedsAllowWrite bool `json:"needs_allow_write"`
	// Mutates and Reviewed say what was known about the function beforehand.
	Mutates  bool `json:"mutates"`
	Reviewed bool `json:"reviewed"`
}

// APICall converts a call's outcome into its envelope.
func APICall(result api.Result, siteName, accountName string) Envelope {
	payload := APICallResult{
		Function:        result.Function,
		Params:          result.Params,
		Response:        result.Response,
		DryRun:          result.Planned,
		NeedsAllowWrite: result.NeedsAllowWrite,
		Mutates:         result.Policy.Mutates,
		Reviewed:        result.Policy.Reviewed,
	}
	if payload.Params == nil {
		payload.Params = map[string]any{}
	}
	if payload.Response == nil {
		payload.Response = json.RawMessage("null")
	}
	meta := NewMeta(SourceWS)
	meta.Site = optional(siteName)
	meta.Account = optional(accountName)
	return NewEnvelope("api.call", payload, meta)
}
