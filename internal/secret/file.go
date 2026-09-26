package secret

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// Backend names where credentials are kept.
type Backend string

const (
	// BackendKeyring is the operating system's own keychain, and the default.
	BackendKeyring Backend = "keyring"
	// BackendFile is a file on this machine, readable only by its owner.
	//
	// It is never chosen automatically. A machine with no keychain — a
	// headless Linux box, an SSH session, WSL, a container — otherwise has no
	// way to stay signed in between commands, and telling a student to paste
	// a session cookie on every run is how a credential ends up in shell
	// history. Choosing it is the user's decision to record, not this tool's
	// to make quietly: 0600 keeps other accounts out, and nothing more. It
	// does not protect against anything running as this user, which is also
	// true of an unlocked desktop keychain.
	BackendFile Backend = "file"
)

// Backends returns the valid choices, for a flag's usage text and its check.
func Backends() []Backend { return []Backend{BackendKeyring, BackendFile} }

// ParseBackend reads a backend name.
func ParseBackend(value string) (Backend, error) {
	trimmed := Backend(strings.ToLower(strings.TrimSpace(value)))
	for _, known := range Backends() {
		if trimmed == known {
			return known, nil
		}
	}
	names := make([]string, 0, len(Backends()))
	for _, known := range Backends() {
		names = append(names, string(known))
	}
	return "", errs.New(errs.CodeConfiguration,
		"unknown credential store "+value).
		WithHint("use one of: " + strings.Join(names, ", "))
}

// FileName is the file a file-backed store keeps credentials in. It sits
// beside the configuration, which is already per-user and already private.
const FileName = "credentials.json"

// File keeps credentials in one JSON object on disk.
//
// The file is written whole through a temporary file and a rename, so an
// interrupted write cannot leave a truncated store that loses every other
// account's credential.
type File struct {
	// Path is the file to use.
	Path string
	// mu serialises read-modify-write, so two goroutines in one process
	// cannot lose each other's entry.
	mu sync.Mutex
}

// NewFile builds a file-backed store beside the given configuration file.
func NewFile(configPath string) *File {
	return &File{Path: filepath.Join(filepath.Dir(configPath), FileName)}
}

const (
	// dirMode keeps the directory out of other accounts' reach.
	dirMode fs.FileMode = 0o700
	// fileMode is the near-universal bar for a credential on disk, and what
	// gh, cargo and gcloud all use.
	fileMode fs.FileMode = 0o600
)

func (f *File) Get(ref Ref) (string, error) {
	if err := ref.valid(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return "", err
	}
	value, ok := values[ref.Key()]
	if !ok {
		return "", errs.Wrap(errs.CodeAuthentication, ErrNotFound, "no stored credential").
			WithReason(errs.ReasonCredentialMissing).
			WithHint("sign in with `moodle auth login`")
	}
	return value, nil
}

func (f *File) Set(ref Ref, value string) error {
	if err := ref.valid(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	}
	values[ref.Key()] = value
	return f.write(values)
}

func (f *File) Delete(ref Ref) error {
	if err := ref.valid(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	}
	if _, ok := values[ref.Key()]; !ok {
		// Removing something already gone is a success, as it is for the
		// keychain: logout must not fail because the file was cleared by hand.
		return nil
	}
	delete(values, ref.Key())
	if len(values) == 0 {
		// An empty store is a file that says nothing; leaving it behind only
		// leaves a thing that looks like it holds credentials.
		if err := os.Remove(f.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return errs.Wrap(errs.CodeConfiguration, err, "cannot remove the credential file")
		}
		return nil
	}
	return f.write(values)
}

func (f *File) read() (map[string]string, error) {
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeConfiguration, err, "cannot read the credential file")
	}
	values := map[string]string{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(data, &values); err != nil {
		// Refusing is the only safe answer: rewriting the file would throw
		// away whatever is in it, including credentials this tool cannot read.
		return nil, errs.Wrap(errs.CodeConfiguration, err,
			"the credential file is not readable as JSON").
			WithHint("inspect " + f.Path + ", or delete it and sign in again")
	}
	return values, nil
}

func (f *File) write(values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(f.Path), dirMode); err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, "cannot create the configuration directory")
	}
	// Sorted so the file does not churn between writes; Go randomises map
	// order, which would otherwise rewrite every line each time.
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(values))
	for _, key := range keys {
		ordered[key] = values[key]
	}
	encoded, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return errs.Wrap(errs.CodeInternal, err, "cannot encode the credentials")
	}

	// Created with the final mode rather than chmod'ed afterwards: between
	// create and chmod the credentials would be world-readable.
	temp, err := os.OpenFile(f.Path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, fileMode)
	if errors.Is(err, fs.ErrExist) {
		// A leftover from an interrupted write. Its content is incomplete by
		// definition, so replacing it loses nothing.
		if err = os.Remove(f.Path + ".tmp"); err == nil {
			temp, err = os.OpenFile(f.Path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, fileMode)
		}
	}
	if err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, "cannot write the credential file")
	}
	if _, err := temp.Write(append(encoded, '\n')); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return errs.Wrap(errs.CodeConfiguration, err, "cannot write the credential file")
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return errs.Wrap(errs.CodeConfiguration, err, "cannot write the credential file")
	}
	if err := os.Rename(temp.Name(), f.Path); err != nil {
		os.Remove(temp.Name())
		return errs.Wrap(errs.CodeConfiguration, err, "cannot replace the credential file")
	}
	// An existing file keeps its own mode through a rename, so a store
	// created before this rule, or loosened by hand, is tightened here.
	if err := os.Chmod(f.Path, fileMode); err != nil {
		return errs.Wrap(errs.CodeConfiguration, err, "cannot restrict the credential file")
	}
	return nil
}

// Selected defers the choice of store until a credential is actually used.
//
// The composition root assembles the authentication stack before the flag that
// chooses a store has been parsed, which is the same reason --backend and
// --verbose are pointers.
type Selected struct {
	// Backend is read on each call. An empty value means the keyring.
	Backend *Backend
	// ConfigPath locates the file store, beside the configuration.
	ConfigPath string
}

func (s Selected) store() Store {
	if s.Backend != nil && *s.Backend == BackendFile {
		return NewFile(s.ConfigPath)
	}
	return Keyring{}
}

func (s Selected) Get(ref Ref) (string, error)     { return s.store().Get(ref) }
func (s Selected) Set(ref Ref, value string) error { return s.store().Set(ref, value) }
func (s Selected) Delete(ref Ref) error            { return s.store().Delete(ref) }
