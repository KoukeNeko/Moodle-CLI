package token_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/authmethod/token"
)

func TestExistingTokenCanBePastedWithoutEcho(t *testing.T) {
	var prompts bytes.Buffer
	method := token.New(func() (string, error) { return "  secret-token  ", nil })
	credential, err := method.Authenticate(context.Background(), auth.Request{
		In: strings.NewReader(""), Out: &prompts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token != "secret-token" || strings.Contains(prompts.String(), "secret-token") {
		t.Fatalf("token = %q; prompt = %q", credential.Token, prompts.String())
	}
}

func TestTokenPromptIsUnavailableWithoutInput(t *testing.T) {
	method := token.New(func() (string, error) {
		t.Fatal("hidden input must not be read without an interactive request")
		return "", nil
	})
	_, err := method.Authenticate(context.Background(), auth.Request{})
	if err == nil || !strings.Contains(err.Error(), "no token given") {
		t.Fatalf("error = %v", err)
	}
}
