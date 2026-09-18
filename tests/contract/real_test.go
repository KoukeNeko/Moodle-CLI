// Package contract_test validates output captured from a real Moodle against
// the published schemas.
//
// The unit tests use a fake Moodle, which can only ever answer the way this
// project imagines Moodle answers. These files are recorded from the docker
// test sites, so a field Moodle actually sends differently is caught here
// rather than by a user.
package contract_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

func TestRecordedOutputSatisfiesSchemas(t *testing.T) {
	matches, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Skip("no recorded output; run tests/contract/record.sh against a test site")
	}
	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// The file name states which schema the document must satisfy:
			// <moodle-version>.<kind>.json
			name := strings.TrimSuffix(filepath.Base(path), ".json")
			parts := strings.SplitN(name, ".", 2)
			if len(parts) != 2 {
				t.Fatalf("cannot tell the kind from %q; expected <version>.<kind>.json", name)
			}
			kind := parts[1]

			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("not valid JSON: %v", err)
			}
			if err := compile(t, kind).Validate(doc); err != nil {
				t.Fatalf("does not satisfy the %s schema: %v", kind, err)
			}
		})
	}
}

func compile(t *testing.T, kind string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	root := v1.SchemaFS()
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	var target string
	for _, entry := range entries {
		raw, err := fs.ReadFile(root, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		id := doc.(map[string]any)["$id"].(string)
		if err := compiler.AddResource(id, doc); err != nil {
			t.Fatal(err)
		}
		if entry.Name() == kind+".schema.json" {
			target = id
		}
	}
	if target == "" {
		t.Fatalf("no schema for kind %q", kind)
	}
	schema, err := compiler.Compile(target)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
