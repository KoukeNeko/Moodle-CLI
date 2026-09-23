package v1

import (
	"encoding/json"

	"github.com/KoukeNeko/moodle-cli/internal/wsregistry"
)

// WSFunctionSummary is one core function in the three-version union.
type WSFunctionSummary struct {
	Name         string   `json:"name"`
	Component    string   `json:"component"`
	Versions     []string `json:"versions"`
	Effect       string   `json:"effect"`
	Destructive  bool     `json:"destructive"`
	Credential   bool     `json:"credential"`
	DeprecatedIn []string `json:"deprecated_in"`
}

// WSListResult is the ws.list payload.
type WSListResult struct {
	RegistryDigest string                     `json:"registry_digest"`
	Snapshots      []wsregistry.MoodleVersion `json:"snapshots"`
	Functions      []WSFunctionSummary        `json:"functions"`
}

// WSList converts a filtered registry listing into its envelope.
func WSList(registry *wsregistry.Registry, functions []wsregistry.Function) Envelope {
	items := make([]WSFunctionSummary, 0, len(functions))
	for _, function := range functions {
		items = append(items, WSFunctionSummary{
			Name: function.Name, Component: function.Component,
			Versions: function.Versions, Effect: string(function.Effect),
			Destructive: function.Destructive, Credential: function.Credential,
			DeprecatedIn: function.DeprecatedIn,
		})
	}
	return NewEnvelope("ws.list", WSListResult{
		RegistryDigest: registry.Digest(), Snapshots: registry.Versions(), Functions: items,
	}, NewMeta(SourceLocal))
}

// WSVariant is one version-specific external-function contract.
type WSVariant struct {
	Description        *string         `json:"description"`
	Deprecated         bool            `json:"deprecated"`
	Effect             string          `json:"effect"`
	Destructive        bool            `json:"destructive"`
	Credential         bool            `json:"credential"`
	Retry              string          `json:"retry"`
	Capabilities       []string        `json:"capabilities"`
	Services           []string        `json:"services"`
	Transports         any             `json:"transports"`
	LoginRequired      bool            `json:"login_required"`
	ExternalDependency string          `json:"external_dependency"`
	Parameters         json.RawMessage `json:"parameters"`
	Returns            json.RawMessage `json:"returns"`
}

// WSDescription is the ws.describe payload.
type WSDescription struct {
	Name         string               `json:"name"`
	Component    string               `json:"component"`
	Versions     []string             `json:"versions"`
	Effect       string               `json:"effect"`
	Destructive  bool                 `json:"destructive"`
	Credential   bool                 `json:"credential"`
	DeprecatedIn []string             `json:"deprecated_in"`
	Variants     map[string]WSVariant `json:"variants"`
}

// WSDescribe converts a union function into its public contract.
func WSDescribe(function wsregistry.Function) Envelope {
	variants := make(map[string]WSVariant, len(function.Variants))
	for key, variant := range function.Variants {
		variants[key] = WSVariant{
			Description: variant.Description, Deprecated: variant.Deprecated,
			Effect: string(variant.Effect), Destructive: variant.Destructive,
			Credential: variant.Credential, Retry: variant.Retry,
			Capabilities: variant.Capabilities, Services: variant.Services,
			Transports: variant.Transports, LoginRequired: variant.LoginRequired,
			ExternalDependency: variant.ExternalDependency,
			Parameters:         variant.Parameters, Returns: variant.Returns,
		}
	}
	return NewEnvelope("ws.describe", WSDescription{
		Name: function.Name, Component: function.Component,
		Versions: function.Versions, Effect: string(function.Effect),
		Destructive: function.Destructive, Credential: function.Credential,
		DeprecatedIn: function.DeprecatedIn, Variants: variants,
	}, NewMeta(SourceLocal))
}

// WSCallResult is the ws.call payload.
type WSCallResult struct {
	Function        string          `json:"function"`
	Version         string          `json:"version"`
	Params          map[string]any  `json:"params"`
	Response        json.RawMessage `json:"response"`
	DryRun          bool            `json:"dry_run"`
	NeedsAllowWrite bool            `json:"needs_allow_write"`
	Effect          string          `json:"effect"`
	Destructive     bool            `json:"destructive"`
}

// WSCall converts a typed call result into its envelope.
func WSCall(result wsregistry.CallResult, siteName, accountName string) Envelope {
	payload := WSCallResult{
		Function: result.Function, Version: result.Version, Params: result.Params,
		Response: result.Response, DryRun: result.DryRun,
		NeedsAllowWrite: result.NeedsAllowWrite, Effect: string(result.Effect),
		Destructive: result.Destructive,
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
	return NewEnvelope("ws.call", payload, meta)
}
