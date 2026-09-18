package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/KoukeNeko/moodle-cli/internal/mcp"
)

// session drives a server over a scripted set of messages.
type session struct {
	t       *testing.T
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	replies []map[string]any
}

func run(t *testing.T, tools *mcp.Registry, messages ...string) *session {
	t.Helper()
	s := &session{t: t}
	server := mcp.NewServer(
		mcp.Streams{
			In:  strings.NewReader(strings.Join(messages, "\n") + "\n"),
			Out: &s.stdout,
			Log: &s.stderr,
		},
		mcp.BuildInfo{Version: "1.2.3"},
		tools,
	)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatalf("serve: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(s.stdout.String()), "\n") {
		if line == "" {
			continue
		}
		var reply map[string]any
		if err := json.Unmarshal([]byte(line), &reply); err != nil {
			t.Fatalf("stdout carried something that is not JSON-RPC: %q", line)
		}
		s.replies = append(s.replies, reply)
	}
	return s
}

// reply finds the answer to one request id.
func (s *session) reply(id float64) map[string]any {
	s.t.Helper()
	for _, reply := range s.replies {
		if got, ok := reply["id"].(float64); ok && got == id {
			return reply
		}
	}
	s.t.Fatalf("no reply with id %v in %s", id, s.stdout.String())
	return nil
}

const handshake = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
const initialized = `{"jsonrpc":"2.0","method":"notifications/initialized"}`

func TestStdoutCarriesNothingButTheProtocol(t *testing.T) {
	// One stray line and the client cannot parse the stream. The check is
	// every line, not the ones we meant to write.
	s := run(t, testTools(t, false),
		handshake, initialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
		`not json at all`,
	)
	if len(s.replies) < 3 {
		t.Fatalf("got %d replies", len(s.replies))
	}
	for _, reply := range s.replies {
		if reply["jsonrpc"] != "2.0" {
			t.Errorf("a line on stdout is not a JSON-RPC message: %v", reply)
		}
	}
}

func TestAMalformedLineIsAnsweredAndTheSessionContinues(t *testing.T) {
	// A client that sends one bad line has not ended the session.
	s := run(t, testTools(t, false),
		handshake, initialized,
		`{ this is not json`,
		`{"jsonrpc":"2.0","id":9,"method":"ping"}`,
	)
	if _, ok := s.reply(9)["result"]; !ok {
		t.Error("the session stopped after a malformed line")
	}
}

func TestANotificationIsNeverAnswered(t *testing.T) {
	// Replying to a notification is a protocol violation.
	s := run(t, testTools(t, false),
		handshake,
		initialized,
		`{"jsonrpc":"2.0","method":"notifications/something_unknown"}`,
	)
	if len(s.replies) != 1 {
		t.Errorf("got %d replies, want only the handshake: %s", len(s.replies), s.stdout.String())
	}
}

func TestTheClientsProtocolVersionIsHonoured(t *testing.T) {
	// An older client should not have to give up on a newer server.
	s := run(t, testTools(t, false),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
	)
	result := s.reply(1)["result"].(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want the client's own", result["protocolVersion"])
	}
}

func TestAnUnknownProtocolVersionFallsBackToOurs(t *testing.T) {
	s := run(t, testTools(t, false),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
	)
	result := s.reply(1)["result"].(map[string]any)
	if result["protocolVersion"] == "1999-01-01" {
		t.Error("the server claimed to speak a version it does not know")
	}
}

func TestToolsAreRefusedBeforeTheHandshake(t *testing.T) {
	// A client calling a tool before initializing has a bug, and answering
	// anyway would hide it.
	s := run(t, testTools(t, false),
		`{"jsonrpc":"2.0","id":5,"method":"tools/list"}`,
	)
	if _, ok := s.reply(5)["error"]; !ok {
		t.Error("tools/list was answered before the handshake")
	}
}

func TestAStringIdIsEchoedBackAsAString(t *testing.T) {
	// JSON-RPC allows either, and rewriting 	"3" as 3 makes the client wait
	// forever for a reply it will not recognise.
	s := run(t, testTools(t, false),
		`{"jsonrpc":"2.0","id":"abc","method":"initialize","params":{}}`,
	)
	if got := s.replies[0]["id"]; got != "abc" {
		t.Errorf("id = %#v, want the string \"abc\"", got)
	}
}

func TestAnUnknownMethodIsAProtocolError(t *testing.T) {
	s := run(t, testTools(t, false),
		handshake, initialized,
		`{"jsonrpc":"2.0","id":7,"method":"resources/list"}`,
	)
	failure, ok := s.reply(7)["error"].(map[string]any)
	if !ok {
		t.Fatal("an unknown method was answered as a success")
	}
	if failure["code"].(float64) != -32601 {
		t.Errorf("code = %v, want -32601 (method not found)", failure["code"])
	}
}

func TestABadLineDoesNotSpinTheServer(t *testing.T) {
	// A streaming decoder reports the same error from the same position
	// forever, so one malformed line from a client would emit error frames
	// until something killed the process. Reading a line at a time is what
	// makes a bad message survivable.
	done := make(chan int, 1)
	go func() {
		s := run(t, testTools(t, false),
			handshake, initialized,
			`{{{ not json`,
			`also not json`,
			`{"jsonrpc":"2.0","id":9,"method":"ping"}`,
		)
		done <- len(s.replies)
	}()

	select {
	case replies := <-done:
		// The handshake, two parse errors and the ping. A spin would produce
		// them without end.
		if replies != 4 {
			t.Errorf("got %d replies, want 4", replies)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not finish: a bad line is being retried forever")
	}
}

func TestARealisticallyLargeMessageIsAccepted(t *testing.T) {
	// A line scanner stops at 64KB by default, which a real tool call can
	// exceed — a long list of paths, or an argument carrying a document. This
	// is the message size the raised cap exists for.
	large := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"resolve_url",` +
		`"arguments":{"url":"https://moodle.example.edu/mod/assign/view.php?id=4&x=` +
		strings.Repeat("a", 200<<10) + `"}}}`
	s := run(t, testTools(t, false), handshake, initialized, large)
	if _, ok := s.reply(2)["result"]; !ok {
		t.Errorf("a 200KB message was not answered: %s", s.stderr.String())
	}
}

func TestAMessageBeyondTheCapIsReportedNotSilentlyDropped(t *testing.T) {
	// The cap has to be somewhere. What matters is that hitting it says so,
	// rather than the session going quiet for no visible reason.
	huge := `{"jsonrpc":"2.0","id":2,"method":"ping","params":{"x":"` +
		strings.Repeat("a", 9<<20) + `"}}`
	s := run(t, testTools(t, false), handshake, huge)
	if len(s.replies) == 0 {
		t.Fatal("the handshake was lost")
	}
	if !strings.Contains(s.stderr.String(), "cannot read") {
		t.Errorf("hitting the cap was not reported on stderr: %q", s.stderr.String())
	}
}
