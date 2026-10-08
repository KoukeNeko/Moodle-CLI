//go:build linux

package callback

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// The handler side, and the waiting side's way of hearing from it.
//
// The desktop activates this process over D-Bus and calls Open with the URI
// the browser was sent to. It forwards that to the process that is waiting
// for it and exits. The URI is a bus message parameter throughout: it never
// reaches a command line, where another user could read it out of /proc.

// applicationInterface is what the desktop entry specification says a
// D-Bus-activatable application exports.
const applicationInterface = "org.freedesktop.Application"

// handler receives the callback and hands it on.
type handler struct {
	delivered chan string
}

// Open is called by the desktop with the addresses to open. There is only
// ever one here, and it is this tool's own scheme.
func (h *handler) Open(uris []string, _ map[string]dbus.Variant) *dbus.Error {
	for _, uri := range uris {
		select {
		case h.delivered <- uri:
		default:
			// Already holding one. A second is not worth queueing: the
			// waiting process answers one login.
		}
	}
	return nil
}

// Activate and ActivateAction are required by the interface and mean nothing
// here: there is no window to raise and no action to take. Refusing to export
// them would make the desktop consider the application broken.
func (h *handler) Activate(_ map[string]dbus.Variant) *dbus.Error { return nil }

func (h *handler) ActivateAction(_ string, _ []dbus.Variant, _ map[string]dbus.Variant) *dbus.Error {
	return nil
}

// ServeCallback runs as the handler the desktop activated.
//
// It waits for one Open, passes the URI to whatever is waiting on the local
// channel, and returns. A handler that lingers would be a process holding a
// credential for no reason.
func ServeCallback(ctx context.Context, scheme, runtimeDir string, wait time.Duration) error {
	// A connection this owns, not the shared one. dbus.SessionBus() hands
	// back a singleton, so closing it here would close it for everything else
	// in the process — which is exactly what happened the first time this was
	// tested in-process.
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, err,
			"there is no desktop session bus to receive the callback on").
			WithHint("automatic sign-in needs a desktop session; " +
				"use `--method manual` instead")
	}
	defer conn.Close()

	receiver := &handler{delivered: make(chan string, 1)}
	if err := conn.Export(receiver, dbus.ObjectPath(objectPath(scheme)),
		applicationInterface); err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot receive callbacks")
	}
	reply, err := conn.RequestName(busName(scheme), dbus.NameFlagDoNotQueue)
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, "cannot claim the callback name")
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return errs.New(errs.CodeConflict,
			"another process is already receiving these callbacks").
			WithHint("only one can; finish or cancel the other sign-in")
	}

	select {
	case uri := <-receiver.delivered:
		return Deliver(runtimeDir, uri)
	case <-time.After(wait):
		return errs.New(errs.CodeUnavailable,
			"no callback arrived").
			WithReason(errs.ReasonTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BusNameFor and ObjectPathFor expose what a handler claims, so a test can
// call it the way the desktop would rather than duplicating the derivation.
func BusNameFor(scheme string) string    { return busName(scheme) }
func ObjectPathFor(scheme string) string { return objectPath(scheme) }
