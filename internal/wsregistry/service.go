package wsregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Caller sends one external-function request.
type Caller interface {
	Call(context.Context, string, map[string]any) (json.RawMessage, error)
}

// Service validates and executes calls described by a Registry.
type Service struct {
	registry   *Registry
	caller     Caller
	mode       safety.Mode
	allowWrite bool
}

// NewService builds the typed web-service use case.
func NewService(registry *Registry, caller Caller, mode safety.Mode, allowWrite bool) *Service {
	return &Service{registry: registry, caller: caller, mode: mode, allowWrite: allowWrite}
}

// CallResult records both plans and completed typed calls.
type CallResult struct {
	Function        string          `json:"function"`
	Version         string          `json:"version"`
	Params          map[string]any  `json:"params"`
	Response        json.RawMessage `json:"response"`
	DryRun          bool            `json:"dry_run"`
	NeedsAllowWrite bool            `json:"needs_allow_write"`
	Effect          Effect          `json:"effect"`
	Destructive     bool            `json:"destructive"`
}

// Call validates a function and its arguments before it can reach Moodle.
func (s *Service) Call(ctx context.Context, capabilities *site.Capabilities, release, name string, params map[string]any, dryRun bool) (CallResult, error) {
	name = strings.TrimSpace(name)
	_, known := s.registry.Lookup(name)
	if !known {
		return CallResult{}, errs.New(errs.CodeNotFound,
			fmt.Sprintf("%s is not a core function in the Moodle 4.5, 5.1 or 5.2 registry", name)).
			WithHint("use `moodle api call` for third-party plugin functions")
	}
	if capabilities != nil && capabilities.Credential == site.CredentialBrowserSession {
		// Typed calls go to the REST endpoint, which only a token opens. A
		// browser session also learns no release, and the missing snapshot
		// for Moodle "" was reported instead of the reason that mattered.
		return CallResult{}, errs.New(errs.CodeUnavailable,
			fmt.Sprintf("%s needs a web service token; this account signed in with a browser session", name)).
			WithReason(errs.ReasonCapability).
			WithHint("sign in with a token, for example `moodle auth login --method qr`")
	}
	variant, version, supported := s.registry.VariantForRelease(name, release)
	if !supported {
		if version == "" {
			return CallResult{}, errs.New(errs.CodeUnavailable,
				fmt.Sprintf("typed calls do not have a registry snapshot for Moodle %q", release)).
				WithHint("supported snapshots are Moodle 4.5, 5.1 and 5.2; use `moodle api call` as an untyped fallback")
		}
		return CallResult{}, errs.New(errs.CodeUnavailable,
			fmt.Sprintf("%s is not installed by Moodle %s", name, version)).
			WithReason(errs.ReasonCapability)
	}
	if capabilities != nil && !capabilities.Has(name) {
		return CallResult{}, errs.New(errs.CodeUnavailable,
			fmt.Sprintf("the web service this account signs in through does not offer %s", name)).
			WithReason(errs.ReasonCapability).
			WithHint("an administrator can expose the function to this token's service")
	}
	if params == nil {
		params = map[string]any{}
	}
	if err := validateParams(name, variant.Parameters, params); err != nil {
		return CallResult{}, err
	}

	result := CallResult{
		Function: name, Version: version, Params: params,
		Effect: variant.Effect, Destructive: variant.Destructive,
		Response: json.RawMessage("null"),
	}
	if dryRun {
		result.DryRun = true
		result.NeedsAllowWrite = variant.Effect == EffectWrite && !s.allowWrite
		return result, nil
	}
	if variant.Effect == EffectWrite {
		if s.mode.ReadOnly {
			return CallResult{}, errs.New(errs.CodePermissionDenied,
				"refusing to call "+name+" in read-only mode").
				WithHint("read-only mode is on; remove --read-only to allow writes")
		}
		if !s.allowWrite {
			return CallResult{}, errs.New(errs.CodeUsage,
				fmt.Sprintf("%s can change things on the site", name)).
				WithHint("pass --allow-write if you mean to, or --dry-run to inspect the request")
		}
	}

	response, err := s.caller.Call(ctx, name, params)
	if err != nil {
		// A write whose response was lost is already marked ambiguous by the
		// transport. Do not retry it here: Moodle functions have no idempotency
		// key and a generic registry call has no operation-specific reconciler.
		return CallResult{}, err
	}
	result.Response = response
	return result, nil
}

func validateParams(name string, raw json.RawMessage, params map[string]any) error {
	compiler := jsonschema.NewCompiler()
	resource := "urn:moodle-cli:ws:" + name
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot read the embedded parameter schema for "+name)
	}
	if err := compiler.AddResource(resource, document); err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot load the embedded parameter schema for "+name)
	}
	schema, err := compiler.Compile(resource)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot compile the embedded parameter schema for "+name)
	}
	if err := schema.Validate(params); err != nil {
		return errs.Wrap(errs.CodeValidation, err, "parameters do not match "+name).
			WithHint("inspect the accepted shape with `moodle ws describe " + name + "`")
	}
	return nil
}
