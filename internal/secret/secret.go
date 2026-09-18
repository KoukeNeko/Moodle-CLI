// Package secret stores credentials in the operating system's keychain.
//
// Credentials never go in the configuration file. When no keychain is
// available this package says so plainly rather than falling back to a
// plaintext file, which would quietly downgrade the user's security
//.
package secret

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// Service is the keychain service name all entries live under.
const Service = "moodle-cli"

// Kind names what a stored credential is.
type Kind string

const (
	KindWSToken      Kind = "ws-token"
	KindPrivateToken Kind = "private-token"
	KindSession      Kind = "session"
)

// Ref identifies one credential. It is built from locally generated IDs, so
// renaming a site or changing its URL never orphans the stored value.
type Ref struct {
	SiteID    site.ID
	AccountID site.ID
	Kind      Kind
}

// Key is the keychain entry name: moodle-cli/<site-id>/<account-id>/<kind>.
func (r Ref) Key() string {
	return fmt.Sprintf("%s/%s/%s", r.SiteID, r.AccountID, r.Kind)
}

func (r Ref) valid() error {
	if r.SiteID == "" || r.AccountID == "" || r.Kind == "" {
		return errs.New(errs.CodeInternal, "incomplete credential reference")
	}
	return nil
}

// Store reads and writes credentials. The interface exists so commands can be
// tested without touching the developer's real keychain.
type Store interface {
	Get(Ref) (string, error)
	Set(Ref, string) error
	Delete(Ref) error
}

// Keyring is the real OS-backed store.
type Keyring struct{}

// ErrNotFound reports that no credential is stored for a reference.
var ErrNotFound = errors.New("credential not found")

func (Keyring) Get(ref Ref) (string, error) {
	if err := ref.valid(); err != nil {
		return "", err
	}
	value, err := keyring.Get(Service, ref.Key())
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", errs.Wrap(errs.CodeAuthentication, ErrNotFound, "no stored credential").
				WithReason(errs.ReasonCredentialMissing).
				WithHint("sign in with `moodle auth login`")
		}
		return "", unavailable(err, "read from")
	}
	return value, nil
}

func (Keyring) Set(ref Ref, value string) error {
	if err := ref.valid(); err != nil {
		return err
	}
	if err := keyring.Set(Service, ref.Key(), value); err != nil {
		return unavailable(err, "write to")
	}
	return nil
}

func (Keyring) Delete(ref Ref) error {
	if err := ref.valid(); err != nil {
		return err
	}
	if err := keyring.Delete(Service, ref.Key()); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			// Removing something that is already gone is a success: logout
			// should not fail because a credential was cleared by hand.
			return nil
		}
		return unavailable(err, "delete from")
	}
	return nil
}

// unavailable turns a keychain failure into an actionable error. On a headless
// Linux box, over SSH or under WSL there is usually no Secret Service at all,
// and the raw D-Bus message does not tell the user what to do about it.
func unavailable(cause error, verb string) error {
	err := errs.Wrap(errs.CodeConfiguration, cause,
		fmt.Sprintf("cannot %s the OS keychain", verb)).
		WithReason("keychain_unavailable")
	if isMissingSecretService(cause) {
		return err.WithHint(
			"no keychain is available (headless Linux, SSH or WSL often have none). " +
				"Pass the credential for a single run with MOODLE_WS_TOKEN instead.")
	}
	return err
}

func isMissingSecretService(err error) bool {
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"dbus", "secret service", "no such interface",
		"the name org.freedesktop.secrets was not provided",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// Memory is an in-process store for tests.
type Memory struct {
	values map[string]string
}

// NewMemory returns an empty in-process store.
func NewMemory() *Memory { return &Memory{values: map[string]string{}} }

func (m *Memory) Get(ref Ref) (string, error) {
	if err := ref.valid(); err != nil {
		return "", err
	}
	value, ok := m.values[ref.Key()]
	if !ok {
		return "", errs.Wrap(errs.CodeAuthentication, ErrNotFound, "no stored credential").
			WithReason(errs.ReasonCredentialMissing)
	}
	return value, nil
}

func (m *Memory) Set(ref Ref, value string) error {
	if err := ref.valid(); err != nil {
		return err
	}
	m.values[ref.Key()] = value
	return nil
}

func (m *Memory) Delete(ref Ref) error {
	if err := ref.valid(); err != nil {
		return err
	}
	delete(m.values, ref.Key())
	return nil
}
