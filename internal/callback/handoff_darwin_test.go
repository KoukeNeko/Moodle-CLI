//go:build darwin

package callback

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAppleEventCallbackTravelsThroughPrivateSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "mcl-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	listener, err := Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	const uri = "moodle-cli-auth://token=synthetic-callback"
	if err := receiveAppleEvent(context.Background(), Scheme, dir, time.Second, strings.NewReader(uri)); err != nil {
		t.Fatal(err)
	}
	got, err := Receive(context.Background(), listener, time.Second)
	if err != nil || strings.TrimSpace(got) != uri {
		t.Fatalf("receive = %q, %v", got, err)
	}
}

func TestAppleEventRejectsWrongSchemeAndOversizedInput(t *testing.T) {
	for _, input := range []string{"moodlemobile://token=synthetic", Scheme + "://" + strings.Repeat("x", maxCallbackURI)} {
		if err := receiveAppleEvent(context.Background(), Scheme, "", time.Second, strings.NewReader(input)); err == nil {
			t.Fatal("invalid callback reached the delivery path")
		}
	}
}

func TestAppleEventReceiveCanBeCancelled(t *testing.T) {
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := receiveAppleEvent(ctx, Scheme, "", time.Minute, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("receive = %v", err)
	}
}
