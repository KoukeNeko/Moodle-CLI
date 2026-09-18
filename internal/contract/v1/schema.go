package v1

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed schema/*.json
var schemaFS embed.FS

const schemaSuffix = ".schema.json"

// SchemaKinds returns every kind that has a published schema, sorted.
func SchemaKinds() []string {
	entries, err := fs.ReadDir(schemaFS, "schema")
	if err != nil {
		// The files are embedded at build time; a failure here is a bug.
		panic(fmt.Sprintf("contract: embedded schemas unreadable: %v", err))
	}
	kinds := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, schemaSuffix) {
			kinds = append(kinds, strings.TrimSuffix(name, schemaSuffix))
		}
	}
	sort.Strings(kinds)
	return kinds
}

// Schema returns the JSON Schema document for a kind.
func Schema(kind string) ([]byte, error) {
	// Kinds legitimately contain dots ("auth.login"), so only path separators
	// and parent references are rejected — a kind must never be able to
	// escape the embedded directory.
	if kind == "" || strings.ContainsAny(kind, "/\\") || strings.Contains(kind, "..") {
		return nil, fmt.Errorf("unknown schema kind %q", kind)
	}
	data, err := schemaFS.ReadFile("schema/" + kind + schemaSuffix)
	if err != nil {
		return nil, fmt.Errorf("unknown schema kind %q (known: %s)", kind, strings.Join(SchemaKinds(), ", "))
	}
	return data, nil
}

// SchemaFS exposes the embedded schemas so tests can load them as a set,
// resolving $ref between them without hitting the network.
func SchemaFS() fs.FS {
	sub, err := fs.Sub(schemaFS, "schema")
	if err != nil {
		panic(fmt.Sprintf("contract: embedded schemas unreadable: %v", err))
	}
	return sub
}
