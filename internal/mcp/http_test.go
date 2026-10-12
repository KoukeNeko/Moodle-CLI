package mcp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/KoukeNeko/moodle-cli/internal/mcp"
)

func httpServer(t *testing.T, allowWrite bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(
		mcp.NewHTTPServer(io.Discard, mcp.BuildInfo{Version: "1.2.3"}, testTools(t, allowWrite)).Handler(),
	)
	t.Cleanup(server.Close)
	return server
}

func post(t *testing.T, url, body string, headers ...string) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		request.Header.Set(headers[i], headers[i+1])
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	text, _ := io.ReadAll(response.Body)
	return response, string(text)
}

func TestARequestIsAnsweredWithoutAHandshake(t *testing.T) {
	// Every HTTP request stands alone: a client that reconnects after a
	// restart has no handshake to replay, and there is nowhere to remember one.
	server := httpServer(t, false)

	response, body := post(t, server.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body %s", response.StatusCode, body)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want plain JSON", got)
	}
	if !strings.Contains(body, `"course_list"`) {
		t.Errorf("the tool list is missing course_list: %s", body)
	}
}

func TestANotificationIsAcceptedWithNoBody(t *testing.T) {
	server := httpServer(t, false)

	response, body := post(t, server.URL, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	if response.StatusCode != http.StatusAccepted || body != "" {
		t.Errorf("got %d %q, want 202 with no body", response.StatusCode, body)
	}
}

func TestAToolCallRunsOverHTTP(t *testing.T) {
	server := httpServer(t, false)

	_, body := post(t, server.URL,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"course_list","arguments":{}}}`)

	if !strings.Contains(body, "Operating Systems") {
		t.Errorf("the course did not come back: %s", body)
	}
}

func TestGetIsRefusedBecauseThereIsNoStreamToOffer(t *testing.T) {
	server := httpServer(t, false)

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", response.StatusCode)
	}
	if allow := response.Header.Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want POST", allow)
	}
}

func TestAnUnknownProtocolVersionHeaderIsRefused(t *testing.T) {
	server := httpServer(t, false)

	response, _ := post(t, server.URL, `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		"MCP-Protocol-Version", "1999-01-01")

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.StatusCode)
	}
}

func TestAKnownProtocolVersionHeaderIsAccepted(t *testing.T) {
	server := httpServer(t, false)

	response, _ := post(t, server.URL, `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		"MCP-Protocol-Version", "2025-06-18")

	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", response.StatusCode)
	}
}

func TestABatchIsRefusedRatherThanPartlyAnswered(t *testing.T) {
	// Answering the first element and dropping the rest would lose requests
	// without a word.
	server := httpServer(t, false)

	response, body := post(t, server.URL,
		`[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","id":2,"method":"ping"}]`)

	if response.StatusCode != http.StatusBadRequest || !strings.Contains(body, "batch") {
		t.Errorf("got %d %s, want 400 naming the batch", response.StatusCode, body)
	}
}

func TestAMalformedBodyIsAParseError(t *testing.T) {
	server := httpServer(t, false)

	_, body := post(t, server.URL, `{ not json`)

	if !strings.Contains(body, "-32700") {
		t.Errorf("want JSON-RPC parse error, got %s", body)
	}
}

func TestAMessageBeyondTheCapIsRefused(t *testing.T) {
	server := httpServer(t, false)
	huge := `{"jsonrpc":"2.0","id":2,"method":"ping","params":{"x":"` +
		strings.Repeat("a", 9<<20) + `"}}`

	response, _ := post(t, server.URL, huge)

	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", response.StatusCode)
	}
}

func TestAToolCallSentAsANotificationIsNeitherRunNorAnswered(t *testing.T) {
	// Over HTTP there is no handshake to stop it, so this is the only guard
	// against a write whose outcome nobody can ever be told.
	registry, _, clock := build(t, false)
	server := httptest.NewServer(
		mcp.NewHTTPServer(io.Discard, mcp.BuildInfo{Version: "1.2.3"}, registry).Handler())
	t.Cleanup(server.Close)

	response, body := post(t, server.URL,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"calendar_upcoming","arguments":{}}}`)

	if clock.calls != 0 {
		t.Error("the tool ran")
	}
	if response.StatusCode != http.StatusAccepted || body != "" {
		t.Errorf("got %d %q, want 202 with nothing said", response.StatusCode, body)
	}
}

func TestOverlappingToolCallsRunOneAtATime(t *testing.T) {
	// The session behind the tools keeps its state in plain fields. Only the
	// race detector can see the lock missing, which is why make verify runs it.
	server := httpServer(t, false)
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar_upcoming","arguments":{}}}`

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			post(t, server.URL, call)
		}()
	}
	wg.Wait()
}

// syncLog is a log the server writes while the test reads it.
type syncLog struct {
	mu   sync.Mutex
	text strings.Builder
}

func (l *syncLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(p)
}

func (l *syncLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.String()
}

func TestAHostNotesEachToolCallWithoutItsArguments(t *testing.T) {
	// Whoever runs a host is not watching the conversation. With writing on,
	// "what did Claude do to my account" needs an answer — but what a person
	// asked about is not the log's to keep.
	log := &syncLog{}
	server := httptest.NewServer(
		mcp.NewHTTPServer(log, mcp.BuildInfo{Version: "1.2.3"}, testTools(t, false)).Handler())
	t.Cleanup(server.Close)

	post(t, server.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"course_list","arguments":{}}}`)
	post(t, server.URL, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"assignment_show","arguments":{"assignment":"SECRET-ID-123"}}}`)
	post(t, server.URL, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"no_such\ntool","arguments":{}}}`)

	text := log.String()
	if !strings.Contains(text, `moodle: tool "course_list" ok `) {
		t.Errorf("a successful call is not on the log:\n%s", text)
	}
	if !strings.Contains(text, `moodle: tool "no_such\ntool" error `) {
		t.Errorf("a failed call is not on the log, or its name is not escaped:\n%s", text)
	}
	if strings.Contains(text, "SECRET-ID-123") {
		t.Errorf("an argument reached the log:\n%s", text)
	}
}

func TestAStdioServerKeepsItsLogToTransportTrouble(t *testing.T) {
	// The client of a stdio server shows each call to the person using it.
	s := run(t, testTools(t, false), handshake, initialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"course_list","arguments":{}}}`)

	if strings.Contains(s.stderr.String(), "tool ") {
		t.Errorf("a stdio server logged a tool call:\n%s", s.stderr.String())
	}
}

func TestANewerRevisionsProbeIsAnsweredSoItFallsBackToInitialize(t *testing.T) {
	// Claude opens with server/discover under revision 2026-07-28 and falls
	// back to initialize when the answer is a 400 that is not a recognised
	// modern JSON-RPC error. An answer in the new revision's own error shape
	// would make it treat this as a modern server and not fall back.
	server := httpServer(t, false)

	probe, body := post(t, server.URL,
		`{"jsonrpc":"2.0","id":674165525,"method":"server/discover","params":{}}`,
		"MCP-Protocol-Version", "2026-07-28", "Mcp-Method", "server/discover")
	fallback, reply := post(t, server.URL,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)

	if probe.StatusCode != http.StatusBadRequest {
		t.Errorf("probe status = %d, want 400", probe.StatusCode)
	}
	if strings.Contains(body, "jsonrpc") || strings.Contains(body, "error") && strings.Contains(body, "code") {
		t.Errorf("the refusal looks like a JSON-RPC error, which tells a client this server is modern: %s", body)
	}
	if fallback.StatusCode != http.StatusOK || !strings.Contains(reply, `"protocolVersion":"2025-06-18"`) {
		t.Errorf("the fallback was not served in 2025-06-18: %d %s", fallback.StatusCode, reply)
	}
}
