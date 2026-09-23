// Package wsregistry owns the typed contract for Moodle core external
// functions across the supported Moodle releases.
//
// The snapshots are generated from disposable, official-version test sites by
// test/e2e/export-ws-registry.php. They describe the core distribution, not a
// token's service. Runtime capabilities are checked separately before a call.
package wsregistry

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed data/v45.json data/v51.json data/v52.json
var snapshots embed.FS

// Effect is the observable class of an external function.
type Effect string

const (
	EffectRead  Effect = "read"
	EffectWrite Effect = "write"
)

// Transports records which Moodle entry points understand a function.
type Transports struct {
	REST bool `json:"rest"`
	AJAX bool `json:"ajax"`
}

// Variant is the contract as installed in one Moodle version.
type Variant struct {
	Name               string          `json:"name"`
	Component          string          `json:"component"`
	Description        *string         `json:"description"`
	Deprecated         bool            `json:"deprecated"`
	Effect             Effect          `json:"effect"`
	Destructive        bool            `json:"destructive"`
	Credential         bool            `json:"credential"`
	Retry              string          `json:"retry"`
	Capabilities       []string        `json:"capabilities"`
	Services           []string        `json:"services"`
	Transports         Transports      `json:"transports"`
	LoginRequired      bool            `json:"login_required"`
	ExternalDependency string          `json:"external_dependency"`
	Parameters         json.RawMessage `json:"parameters"`
	Returns            json.RawMessage `json:"returns"`
}

// Function is the union entry for one function. Variants are keyed by v45,
// v51 and v52 so schema drift remains visible instead of being overwritten by
// whichever snapshot happened to load last.
type Function struct {
	Name         string             `json:"name"`
	Component    string             `json:"component"`
	Versions     []string           `json:"versions"`
	Effect       Effect             `json:"effect"`
	Destructive  bool               `json:"destructive"`
	Credential   bool               `json:"credential"`
	DeprecatedIn []string           `json:"deprecated_in"`
	Variants     map[string]Variant `json:"variants,omitempty"`
}

// MoodleVersion identifies a generated snapshot.
type MoodleVersion struct {
	Key     string `json:"key"`
	Release string `json:"release"`
	Version string `json:"version"`
}

type snapshot struct {
	SchemaVersion int `json:"schema_version"`
	Moodle        struct {
		Release string `json:"release"`
		Version string `json:"version"`
	} `json:"moodle"`
	Functions []Variant `json:"functions"`
}

// Registry is an immutable union of the generated snapshots.
type Registry struct {
	functions map[string]Function
	versions  []MoodleVersion
	digest    string
}

var snapshotFiles = []struct {
	key  string
	path string
}{
	{key: "v45", path: "data/v45.json"},
	{key: "v51", path: "data/v51.json"},
	{key: "v52", path: "data/v52.json"},
}

// Load validates and merges every embedded snapshot.
func Load() (*Registry, error) {
	registry := &Registry{functions: map[string]Function{}}
	digest := sha256.New()
	for _, source := range snapshotFiles {
		raw, err := snapshots.ReadFile(source.path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", source.path, err)
		}
		digest.Write([]byte(source.key))
		digest.Write(raw)
		var document snapshot
		if err := json.Unmarshal(raw, &document); err != nil {
			return nil, fmt.Errorf("decode %s: %w", source.path, err)
		}
		if document.SchemaVersion != 1 {
			return nil, fmt.Errorf("%s: registry schema_version %d, want 1", source.path, document.SchemaVersion)
		}
		registry.versions = append(registry.versions, MoodleVersion{
			Key: source.key, Release: document.Moodle.Release, Version: document.Moodle.Version,
		})
		seen := map[string]bool{}
		for _, variant := range document.Functions {
			if strings.TrimSpace(variant.Name) == "" {
				return nil, fmt.Errorf("%s: function with no name", source.path)
			}
			if seen[variant.Name] {
				return nil, fmt.Errorf("%s: duplicate function %s", source.path, variant.Name)
			}
			seen[variant.Name] = true
			if variant.Effect != EffectRead && variant.Effect != EffectWrite {
				return nil, fmt.Errorf("%s: %s has unknown effect %q", source.path, variant.Name, variant.Effect)
			}
			if !json.Valid(variant.Parameters) || !json.Valid(variant.Returns) {
				return nil, fmt.Errorf("%s: %s has an invalid JSON schema", source.path, variant.Name)
			}

			function, ok := registry.functions[variant.Name]
			if !ok {
				function = Function{
					Name: variant.Name, Component: variant.Component,
					Effect: variant.Effect, Versions: []string{},
					DeprecatedIn: []string{}, Variants: map[string]Variant{},
				}
			}
			function.Versions = append(function.Versions, source.key)
			function.Variants[source.key] = variant
			if variant.Effect == EffectWrite {
				function.Effect = EffectWrite
			}
			function.Destructive = function.Destructive || variant.Destructive
			function.Credential = function.Credential || variant.Credential
			if variant.Deprecated {
				function.DeprecatedIn = append(function.DeprecatedIn, source.key)
			}
			registry.functions[variant.Name] = function
		}
	}
	registry.digest = hex.EncodeToString(digest.Sum(nil))
	return registry, nil
}

// Digest identifies the exact three input snapshots.
func (r *Registry) Digest() string {
	if r == nil {
		return ""
	}
	return r.digest
}

// Versions returns the snapshots that make up this registry.
func (r *Registry) Versions() []MoodleVersion {
	if r == nil {
		return []MoodleVersion{}
	}
	out := make([]MoodleVersion, len(r.versions))
	copy(out, r.versions)
	return out
}

// All returns every union function in name order.
func (r *Registry) All() []Function {
	if r == nil {
		return []Function{}
	}
	out := make([]Function, 0, len(r.functions))
	for _, function := range r.functions {
		out = append(out, cloneFunction(function))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup returns one union function.
func (r *Registry) Lookup(name string) (Function, bool) {
	if r == nil {
		return Function{}, false
	}
	function, ok := r.functions[strings.TrimSpace(name)]
	return cloneFunction(function), ok
}

// VariantForRelease selects the schema for an actual Moodle release.
func (r *Registry) VariantForRelease(name, release string) (Variant, string, bool) {
	function, ok := r.Lookup(name)
	if !ok {
		return Variant{}, "", false
	}
	key := VersionKey(release)
	variant, ok := function.Variants[key]
	return variant, key, ok
}

// VersionKey maps a Moodle release string to a supported registry key.
func VersionKey(release string) string {
	release = strings.TrimSpace(release)
	switch {
	case strings.HasPrefix(release, "4.5"):
		return "v45"
	case strings.HasPrefix(release, "5.1"):
		return "v51"
	case strings.HasPrefix(release, "5.2"):
		return "v52"
	default:
		return ""
	}
}

func cloneFunction(in Function) Function {
	if in.Name == "" {
		return Function{}
	}
	out := in
	out.Versions = append([]string{}, in.Versions...)
	out.DeprecatedIn = append([]string{}, in.DeprecatedIn...)
	out.Variants = make(map[string]Variant, len(in.Variants))
	for key, variant := range in.Variants {
		out.Variants[key] = variant
	}
	return out
}
