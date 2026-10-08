//go:build darwin

package callback

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

func (b *Broker) prepare() error {
	if b.runtimeDir == "" {
		return errs.New(errs.CodeUnavailable, "cannot locate the private sign-in directory")
	}
	if err := os.MkdirAll(b.runtimeDir, 0o700); err != nil {
		return err
	}
	return checkPrivate(b.runtimeDir)
}

// The broker acquires the private listener first, so concurrent sign-ins
// cannot both rebuild the native handler before noticing each other.
func (b *Broker) prepareHandler() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	_, err = Register(b.scheme, executable)
	return err
}

func ServeCallback(ctx context.Context, scheme, runtimeDir string, wait time.Duration) error {
	return receiveAppleEvent(ctx, scheme, runtimeDir, wait, os.Stdin)
}

func receiveAppleEvent(ctx context.Context, scheme, runtimeDir string, wait time.Duration, input io.Reader) error {
	type result struct {
		uri []byte
		err error
	}
	ready := make(chan result, 1)
	go func() {
		uri, err := io.ReadAll(io.LimitReader(input, maxCallbackURI+1))
		ready <- result{uri, err}
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case answer := <-ready:
		if answer.err != nil {
			return errs.Wrap(errs.CodeUnavailable, answer.err, "cannot receive the browser callback")
		}
		if len(answer.uri) > maxCallbackURI {
			return errs.New(errs.CodeValidation, "the browser callback is too large")
		}
		uri := strings.TrimSpace(string(answer.uri))
		if !strings.HasPrefix(strings.ToLower(uri), strings.ToLower(scheme)+"://") {
			return errs.New(errs.CodeValidation, "unexpected browser callback scheme")
		}
		return Deliver(runtimeDir, uri)
	case <-timer.C:
		return errs.New(errs.CodeUnavailable, "no browser callback arrived").WithReason(errs.ReasonTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}
