// Package config reads and writes the on-disk configuration.
//
// The file holds metadata only. Credentials live in the OS keychain, and this
// package refuses to write a file that looks like it contains one.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// SchemaVersion is the config format version this build understands.
const SchemaVersion = 1

// EnvPath overrides the configuration location.
const EnvPath = "MOODLE_CLI_CONFIG"

// Current names the site and account commands act on by default.
type Current struct {
	Site    string `yaml:"site"`
	Account string `yaml:"account"`
}

// Account is one Moodle user on one site. Everything here is metadata: the
// credential itself is in the keychain, keyed by the site and account IDs.
type Account struct {
	ID             site.ID             `yaml:"id"`
	UserID         string              `yaml:"user_id"`
	Username       string              `yaml:"username"`
	DisplayName    string              `yaml:"display_name"`
	AuthMethod     string              `yaml:"auth_method"`
	CredentialKind site.CredentialKind `yaml:"credential_kind"`

	// Extra keeps fields this build does not know about, so hand-added keys
	// survive a write instead of being silently dropped.
	Extra map[string]any `yaml:",inline"`
}

// Site is one Moodle installation and the accounts known on it.
type Site struct {
	ID      site.ID `yaml:"id"`
	BaseURL string  `yaml:"base_url"`
	WWWRoot string  `yaml:"www_root"`
	// Backend is "auto" or "ws-only".
	Backend        string              `yaml:"backend"`
	DefaultAccount string              `yaml:"default_account"`
	Accounts       map[string]*Account `yaml:"accounts"`

	Extra map[string]any `yaml:",inline"`
}

// Preferences are user-facing defaults. Command-line flags always win.
type Preferences struct {
	Output string `yaml:"output"`

	Extra map[string]any `yaml:",inline"`
}

// File is the whole configuration document.
type File struct {
	SchemaVersion int              `yaml:"schema_version"`
	Current       Current          `yaml:"current"`
	Sites         map[string]*Site `yaml:"sites"`
	Preferences   Preferences      `yaml:"preferences"`

	Extra map[string]any `yaml:",inline"`

	// path is where this document was loaded from, so Save writes back to the
	// same place without the caller having to remember.
	path string
}

// Backend values.
const (
	BackendAuto   = "auto"
	BackendWSOnly = "ws-only"
)

// DefaultPath returns the configuration location for this platform, honouring
// the environment override.
func DefaultPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(EnvPath)); override != "" {
		return override, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", errs.Wrap(errs.CodeConfiguration, err, "cannot determine the user configuration directory")
	}
	return filepath.Join(dir, "moodle-cli", "config.yaml"), nil
}

// Load reads the configuration. A missing file is not an error: it yields an
// empty configuration, and nothing is written until something needs saving.
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{SchemaVersion: SchemaVersion, Sites: map[string]*Site{}, path: path}, nil
		}
		return nil, errs.Wrap(errs.CodeConfiguration, err, fmt.Sprintf("cannot read %s", path))
	}

	var file File
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, errs.Wrap(errs.CodeConfiguration, err, fmt.Sprintf("cannot parse %s", path)).
			WithHint("fix the YAML, or move the file aside to start over")
	}
	file.path = path
	if file.Sites == nil {
		file.Sites = map[string]*Site{}
	}

	switch {
	case file.SchemaVersion == SchemaVersion:
	case file.SchemaVersion > SchemaVersion:
		// A newer build wrote this. Guessing at a format we do not know could
		// destroy settings, so stop instead.
		return nil, errs.New(errs.CodeConfiguration, fmt.Sprintf(
			"%s was written by a newer moodle-cli (schema_version %d, this build understands %d)",
			path, file.SchemaVersion, SchemaVersion)).
			WithHint("upgrade moodle-cli")
	default:
		if err := migrate(&file, raw, path); err != nil {
			return nil, err
		}
	}
	return &file, nil
}

// migrate upgrades an older document in place, after backing up the original.
func migrate(file *File, original []byte, path string) error {
	from := file.SchemaVersion
	backup := fmt.Sprintf("%s.bak.v%d", path, from)
	if err := os.WriteFile(backup, original, 0o600); err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, fmt.Sprintf("cannot back up %s", path))
	}
	// No migrations exist yet: version 0 means "written before the field
	// existed", which is shape-compatible with version 1.
	file.SchemaVersion = SchemaVersion
	return nil
}

// Path returns where this configuration lives.
func (f *File) Path() string { return f.path }

// forbiddenKeys must never appear in the configuration file. Credentials
// belong in the keychain.
var forbiddenKeys = []string{
	"token", "wstoken", "privatetoken", "private_token",
	"password", "passwd", "secret", "moodlesession", "cookie",
}

// Save writes the configuration atomically: a temporary file in the same
// directory, flushed, then renamed over the target. A crash mid-write leaves
// the previous file intact rather than a truncated one.
func (f *File) Save() error {
	if f.path == "" {
		return errs.New(errs.CodeInternal, "configuration has no path")
	}
	if f.SchemaVersion == 0 {
		f.SchemaVersion = SchemaVersion
	}

	encoded, err := yaml.Marshal(f)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot encode configuration")
	}
	if err := assertNoCredentials(encoded); err != nil {
		return err
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, fmt.Sprintf("cannot create %s", dir))
	}

	temp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, "cannot create a temporary file")
	}
	tempName := temp.Name()
	defer func() {
		// Harmless once the rename has happened; essential if it has not.
		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return errs.Wrap(errs.CodeConfiguration, err, "cannot set permissions on the temporary file")
	}
	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		return errs.Wrap(errs.CodeConfiguration, err, "cannot write the temporary file")
	}
	// Flush before the rename, so the rename cannot publish an empty file.
	if err := temp.Sync(); err != nil {
		temp.Close()
		return errs.Wrap(errs.CodeConfiguration, err, "cannot flush the temporary file")
	}
	if err := temp.Close(); err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, "cannot close the temporary file")
	}
	if err := os.Rename(tempName, f.path); err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, fmt.Sprintf("cannot replace %s", f.path))
	}
	return nil
}

// assertNoCredentials is a last line of defence: a credential reaching the
// config file is a bug, and it should fail loudly rather than land on disk.
func assertNoCredentials(encoded []byte) error {
	var document map[string]any
	if err := yaml.Unmarshal(encoded, &document); err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot re-read the encoded configuration")
	}
	if found := findForbiddenKey(document); found != "" {
		return errs.New(errs.CodeInternal, fmt.Sprintf(
			"refusing to write the configuration: it contains %q, which looks like a credential", found))
	}
	return nil
}

func findForbiddenKey(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			for _, forbidden := range forbiddenKeys {
				if lower == forbidden {
					return key
				}
			}
			if found := findForbiddenKey(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findForbiddenKey(child); found != "" {
				return found
			}
		}
	}
	return ""
}
