package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
)

type fakeCallbackHandler struct {
	registration HandlerRegistration
	servedScheme string
}

func (f *fakeCallbackHandler) Scheme() string { return f.registration.Scheme }

func (f *fakeCallbackHandler) Register() (HandlerRegistration, error) {
	return f.registration, nil
}
func (f *fakeCallbackHandler) Unregister() error {
	f.registration.Installed = false
	return nil
}
func (f *fakeCallbackHandler) Status() HandlerRegistration { return f.registration }
func (f *fakeCallbackHandler) Serve(_ context.Context, scheme string) error {
	f.servedScheme = scheme
	return nil
}

func TestHandlerCommandsUseTheInjectedPort(t *testing.T) {
	var out bytes.Buffer
	handler := &fakeCallbackHandler{registration: HandlerRegistration{
		Scheme: "moodle-cli-auth", Installed: true,
		DesktopFile: "/tmp/moodle.desktop", ServiceFile: "/tmp/moodle.service",
	}}
	cmd := newAuthRegisterHandlerCommand(&Renderer{
		Streams: Streams{Out: &out}, Format: FormatTable,
	}, handler)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("register-handler rendered no result")
	}
	if cmd.Flags().Lookup("scheme") != nil {
		t.Fatal("register-handler exposes a scheme the login flow cannot select")
	}
}

func TestHandlerStatusHasAVersionedJSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	handler := &fakeCallbackHandler{registration: HandlerRegistration{
		Scheme: "moodle-cli-auth", Installed: true,
	}}
	cmd := newAuthHandlerStatusCommand(&Renderer{
		Streams: Streams{Out: &out}, Format: FormatJSON,
	}, handler)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var envelope v1.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != v1.SchemaVersion || envelope.Kind != "auth.handler" {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestTheHiddenCallbackPassesItsSchemeToTheAdapter(t *testing.T) {
	handler := &fakeCallbackHandler{registration: HandlerRegistration{Scheme: "moodle-cli-auth"}}
	cmd := newAuthCallbackCommand(handler)
	cmd.SetArgs([]string{"--scheme", "moodle-cli-test"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if handler.servedScheme != "moodle-cli-test" {
		t.Fatalf("served scheme = %q", handler.servedScheme)
	}
}
