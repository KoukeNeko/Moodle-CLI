package site_test

import (
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func TestParseBaseURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare host defaults to https", "moodle.example.edu", "https://moodle.example.edu"},
		{"explicit https kept", "https://moodle.example.edu", "https://moodle.example.edu"},
		{"explicit http kept", "http://localhost:8521", "http://localhost:8521"},
		{"trailing slash removed", "https://moodle.example.edu/", "https://moodle.example.edu"},
		{"subpath kept", "https://example.edu/moodle", "https://example.edu/moodle"},
		{"query and fragment dropped", "https://example.edu/moodle?a=1#x", "https://example.edu/moodle"},
		{"surrounding space ignored", "  https://example.edu  ", "https://example.edu"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := site.ParseBaseURL(c.in)
			if err != nil {
				t.Fatalf("ParseBaseURL(%q): %v", c.in, err)
			}
			if got.String() != c.want {
				t.Errorf("ParseBaseURL(%q) = %q, want %q", c.in, got.String(), c.want)
			}
		})
	}
}

func TestParseBaseURLRejects(t *testing.T) {
	// A bad address must be a usage error, not something that surfaces later
	// as a confusing network failure.
	for _, in := range []string{"", "   ", "ftp://example.edu", "https://"} {
		got, err := site.ParseBaseURL(in)
		if err == nil {
			t.Errorf("ParseBaseURL(%q) = %v, want an error", in, got)
			continue
		}
		if code := errs.From(err).Code; code != errs.CodeUsage {
			t.Errorf("ParseBaseURL(%q): code %q, want usage", in, code)
		}
	}
}

func TestNewIDIsUniqueAndWellFormed(t *testing.T) {
	seen := map[site.ID]bool{}
	for range 100 {
		id := site.NewID()
		if len(id) != 36 {
			t.Fatalf("id %q has length %d, want 36", id, len(id))
		}
		if id[14] != '4' {
			t.Fatalf("id %q is not version 4", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestEndpoint(t *testing.T) {
	base, err := site.ParseBaseURL("https://example.edu/moodle")
	if err != nil {
		t.Fatal(err)
	}
	s := site.Site{BaseURL: base}
	// Joining must not double or drop the separator whichever way the caller
	// writes the path.
	for _, path := range []string{"login/token.php", "/login/token.php"} {
		if got, want := s.Endpoint(path), "https://example.edu/moodle/login/token.php"; got != want {
			t.Errorf("Endpoint(%q) = %q, want %q", path, got, want)
		}
	}
}
