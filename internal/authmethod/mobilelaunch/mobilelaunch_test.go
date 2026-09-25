package mobilelaunch

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/moodle"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

type fakeBroker struct {
	installed bool
	address   string
	reply     string
	err       error
}

func TestAValidCallbackBecomesACredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sitename":"Test","username":"student","userid":7}`))
	}))
	defer server.Close()

	const passport = "known-passport"
	sum := md5.Sum([]byte(server.URL + passport))
	payload := hex.EncodeToString(sum[:]) + ":::ws-token:::private-token"
	broker := &fakeBroker{
		installed: true,
		reply:     "moodle-cli-test://token=" + base64.StdEncoding.EncodeToString([]byte(payload)),
	}
	base, err := site.ParseBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	target := site.Site{BaseURL: base}
	method := New(func(site.Site) *moodle.Client { return moodle.NewClient(target) }, broker)
	credential, err := method.Authenticate(context.Background(), auth.Request{
		Site: target, WWWRoot: server.URL, Passport: passport,
	})
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token != "ws-token" || credential.PrivateToken != "private-token" {
		t.Fatalf("credential = %+v", credential)
	}
	if credential.Method != "mobilelaunch" {
		t.Fatalf("method = %q", credential.Method)
	}
}

func (*fakeBroker) Scheme() string    { return "moodle-cli-test" }
func (b *fakeBroker) Installed() bool { return b.installed }

func (b *fakeBroker) OpenAndReceive(_ context.Context, address string, _ time.Duration, opened func(bool)) (string, error) {
	b.address = address
	if opened != nil {
		opened(true)
	}
	return b.reply, b.err
}

func testClient(target site.Site) *moodle.Client { return moodle.NewClient(target) }

func TestAnUnreachableSiteLeavesAvailabilityUnknown(t *testing.T) {
	method := New(testClient, &fakeBroker{installed: true})
	got := method.Probe(context.Background(), site.Site{}, nil)
	if got.Availability != auth.Unknown {
		t.Fatalf("availability = %q, want unknown", got.Availability)
	}
}

func TestMissingHandlerHintMatchesThePlatform(t *testing.T) {
	method := New(testClient, &fakeBroker{})
	config := &auth.PublicConfig{EnableMobileWebService: 1}
	got := method.Probe(context.Background(), site.Site{}, config)
	if got.Availability != auth.Unavailable {
		t.Fatalf("availability = %q, want unavailable", got.Availability)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(got.Reason, "--browser safari") || strings.Contains(got.Reason, "register-handler") {
			t.Fatalf("macOS hint should offer Safari, not an unsupported handler: %q", got.Reason)
		}
	case "linux":
		if !strings.Contains(got.Reason, "register-handler") {
			t.Fatalf("Linux hint should offer the handler: %q", got.Reason)
		}
	default:
		if strings.Contains(got.Reason, "register-handler") {
			t.Fatalf("unsupported platform was told to install a handler: %q", got.Reason)
		}
	}
}

func TestTheMethodUsesItsArchitecturalName(t *testing.T) {
	if got := New(testClient, &fakeBroker{}).Name(); got != "mobilelaunch" {
		t.Fatalf("method name = %q", got)
	}
}

func TestTheBrokerOwnsTheDesktopHandoff(t *testing.T) {
	broker := &fakeBroker{installed: true, reply: "not a callback"}
	method := New(testClient, broker)
	base, err := site.ParseBaseURL("https://moodle.example.edu")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, err = method.Authenticate(context.Background(), auth.Request{
		Site:     site.Site{BaseURL: base},
		WWWRoot:  "https://moodle.example.edu",
		Passport: "known-passport",
		Out:      &out,
	})
	if err == nil {
		t.Fatal("a malformed callback was accepted")
	}
	for _, want := range []string{"urlscheme=moodle-cli-test", "passport=known-passport"} {
		if !strings.Contains(broker.address, want) {
			t.Errorf("launch address %q does not contain %q", broker.address, want)
		}
	}
	if !strings.Contains(out.String(), "Opening your browser") {
		t.Errorf("the user was not told the browser opened: %q", out.String())
	}
}
