// Package arch_test enforces the project's import rules.
//
// The rules live here and nowhere else: a violation fails `go test`, so the
// boundaries cannot rot the way a written-down convention would.
//
// Always run this with -count=1. It reads the other packages at run time, so
// Go's test cache cannot see those inputs and will happily replay a stale
// pass after a boundary has been broken. The Makefile and CI both pass it.
package arch_test

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const modulePath = "github.com/KoukeNeko/moodle-cli"

// forbidden lists, per package, the internal packages it may not import.
// A "..." suffix forbids the package and everything under it.
var forbidden = map[string][]string{
	// Feature packages own their own backend choice; they must not know how
	// data is fetched, presented or serialised.
	"internal/course":       {"internal/moodle...", "internal/webread", "internal/cli", "internal/mcp", "internal/contract..."},
	"internal/assignment":   {"internal/moodle...", "internal/webread", "internal/cli", "internal/mcp", "internal/contract..."},
	"internal/grade":        {"internal/moodle...", "internal/webread", "internal/cli", "internal/mcp", "internal/contract..."},
	"internal/calendar":     {"internal/moodle...", "internal/webread", "internal/cli", "internal/mcp", "internal/contract..."},
	"internal/forum":        {"internal/moodle...", "internal/webread", "internal/cli", "internal/mcp", "internal/contract..."},
	"internal/filetransfer": {"internal/cli", "internal/mcp", "internal/contract..."},

	// Adapters sit below the application; they must not reach up.
	"internal/moodle":  {"internal/cli", "internal/mcp", "internal/contract..."},
	"internal/webread": {"internal/cli", "internal/mcp", "internal/contract..."},

	// MCP must call the same use cases as the CLI, never the CLI itself.
	"internal/mcp": {"internal/cli", "github.com/spf13/cobra"},

	// The presentation layer must not build HTTP requests itself.
	"internal/cli": {"internal/moodle...", "internal/webread", "net/http"},

	// Core vocabulary stays free of transport, presentation and platform.
	"internal/site":   {"net/http", "github.com/spf13/cobra", "internal/cli", "internal/moodle..."},
	"internal/safety": {"net/http", "github.com/spf13/cobra", "internal/cli", "internal/moodle..."},
	"internal/errs":   {"net/http", "github.com/spf13/cobra", "internal/contract..."},
}

// bannedNames are package names that hide a missing boundary.
var bannedNames = []string{"utils", "util", "helpers", "common", "misc", "types", "platform"}

func TestImportBoundaries(t *testing.T) {
	loaded := load(t)

	for pkgPath, imports := range loaded {
		rel, ok := strings.CutPrefix(pkgPath, modulePath+"/")
		if !ok {
			continue
		}
		rules, watched := forbidden[rel]
		if !watched {
			continue
		}
		for _, imp := range imports {
			impRel := strings.TrimPrefix(imp, modulePath+"/")
			for _, rule := range rules {
				if matches(impRel, rule) {
					t.Errorf("%s imports %s, which the import rules forbid", rel, impRel)
				}
			}
		}
	}
}

func TestNobodyImportsBootstrap(t *testing.T) {
	// bootstrap is the composition root: it may import everything, and only
	// cmd/moodle may import it.
	const bootstrapPkg = modulePath + "/internal/bootstrap"
	for pkgPath, imports := range load(t) {
		if pkgPath == modulePath+"/cmd/moodle" || pkgPath == bootstrapPkg {
			continue
		}
		for _, imp := range imports {
			if imp == bootstrapPkg {
				t.Errorf("%s imports the composition root; only cmd/moodle may", pkgPath)
			}
		}
	}
}

func TestNoDumpingGroundPackageNames(t *testing.T) {
	for pkgPath := range load(t) {
		rel, ok := strings.CutPrefix(pkgPath, modulePath+"/")
		if !ok {
			continue
		}
		name := rel[strings.LastIndex(rel, "/")+1:]
		for _, banned := range bannedNames {
			if name == banned {
				t.Errorf("package %q has no boundary of its own; give it a name that says what it owns", rel)
			}
		}
	}
}

// load returns every package in the module with its import paths.
func load(t *testing.T) map[string][]string {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports, Dir: repoRoot(t)}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("loading packages: %v", err)
	}
	out := map[string][]string{}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			t.Fatalf("package %s failed to load: %v", pkg.PkgPath, pkg.Errors[0])
		}
		imports := make([]string, 0, len(pkg.Imports))
		for imp := range pkg.Imports {
			imports = append(imports, imp)
		}
		out[pkg.PkgPath] = imports
	}
	if len(out) == 0 {
		t.Fatal("no packages loaded; the architecture rules would pass vacuously")
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return "../.."
}

// matches reports whether an import path is covered by a rule, where a "..."
// suffix covers the package and everything beneath it.
func matches(importPath, rule string) bool {
	if prefix, ok := strings.CutSuffix(rule, "..."); ok {
		return importPath == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(importPath, prefix)
	}
	return importPath == rule
}
