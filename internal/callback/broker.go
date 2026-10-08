package callback

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Broker owns the operating-system part of one browser handoff. It opens the
// private channel before launching the browser, so a fast redirect cannot
// arrive before anything is listening.
type Broker struct {
	scheme     string
	runtimeDir string
}

// NewBroker builds a broker for one registered scheme and desktop session.
func NewBroker(scheme, runtimeDir string) *Broker {
	return &Broker{scheme: scheme, runtimeDir: runtimeDir}
}

// DefaultBroker uses this tool's scheme and the current desktop session.
func DefaultBroker() *Broker {
	return NewBroker(Scheme, RuntimeDir())
}

// RuntimeDir is shared by the waiting CLI and the desktop-activated handler.
// Launch Services does not inherit a terminal's XDG_RUNTIME_DIR on macOS.
func RuntimeDir() string {
	if runtime.GOOS == "darwin" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		return filepath.Join(cache, "moodle-cli-login")
	}
	return os.Getenv("XDG_RUNTIME_DIR")
}

func (b *Broker) Scheme() string { return b.scheme }

// Installed reports whether this broker has a registered desktop handler.
func (b *Broker) Installed() bool { return Status(b.scheme).Installed() }

// OpenAndReceive performs the platform handoff and waits for its reply.
func (b *Broker) OpenAndReceive(ctx context.Context, address string, wait time.Duration, opened func(bool)) (string, error) {
	if err := b.prepare(); err != nil {
		return "", err
	}
	listener, err := Listen(b.runtimeDir)
	if err != nil {
		return "", err
	}
	defer listener.Close()
	if err := b.prepareHandler(); err != nil {
		return "", err
	}

	viaPortal, err := OpenInBrowser(ctx, address)
	if err != nil {
		return "", err
	}
	if opened != nil {
		opened(viaPortal)
	}
	return Receive(ctx, listener, wait)
}
