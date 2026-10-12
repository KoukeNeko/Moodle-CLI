package bootstrap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// serveOverStdio runs `mcp serve` through the real composition root, feeds it
// the given JSON-RPC lines, and returns the reply to the request with id 3.
func serveOverStdio(t *testing.T, lines ...string) map[string]any {
	t.Helper()
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inRead, outWrite
	t.Cleanup(func() { os.Stdin, os.Stdout = stdin, stdout })

	go func() {
		for _, line := range lines {
			fmt.Fprintln(inWrite, line)
		}
		inWrite.Close()
	}()
	code := Run(context.Background(), Build{}, []string{"mcp", "serve", "--site", "school"})
	outWrite.Close()
	if code != 0 {
		t.Fatalf("mcp serve exited %d", code)
	}

	replies := bufio.NewScanner(outRead)
	replies.Buffer(make([]byte, 1<<20), 1<<20)
	for replies.Scan() {
		var reply map[string]any
		if err := json.Unmarshal(replies.Bytes(), &reply); err == nil && reply["id"] == float64(3) {
			io.Copy(io.Discard, outRead)
			return reply
		}
	}
	t.Fatal("no reply to the tool call")
	return nil
}

// TestABrowserSessionOnTheAgentServerIsSentSafelyAndRenewedByRestart runs the
// wiring the way an agent reaches it: a browser session the site no longer
// accepts, and a course reference an agent made up.
func TestABrowserSessionOnTheAgentServerIsSentSafelyAndRenewedByRestart(t *testing.T) {
	var mu sync.Mutex
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.URL.RequestURI())
		mu.Unlock()
		// What Moodle serves for a session it does not recognise.
		fmt.Fprint(w, `<html><body><form id="login"><input name="logintoken" value="t"></form></body></html>`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("MOODLE_CLI_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("MOODLE_SESSION", "expired-cookie")
	t.Setenv("MOODLE_WS_TOKEN", "")
	if code := Run(context.Background(), Build{}, []string{"site", "add", "school", server.URL}); code != 0 {
		t.Fatalf("site add: exit %d", code)
	}

	reply := serveOverStdio(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"quiz_list","arguments":{"course_ids":["2&edit=on&sesskey=x"]}}}`,
	)

	result, _ := reply["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("a rejected browser session was reported as a success: %v", reply)
	}
	content, _ := result["content"].([]any)
	text, _ := content[0].(map[string]any)["text"].(string)
	var envelope struct {
		Error struct {
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("the failure is not an envelope: %v\n%s", err, text)
	}
	hint := envelope.Error.Hint
	if !strings.Contains(hint, "restarted") || !strings.Contains(hint, "import-browser --site school") {
		t.Errorf("the hint does not say a restart is needed after renewing the session:\n%s", hint)
	}
	if strings.Contains(hint, "does not need restarting") {
		t.Errorf("the hint promises a renewed browser session is picked up on its own:\n%s", hint)
	}

	mu.Lock()
	defer mu.Unlock()
	var courseViews int
	for _, requestURI := range requested {
		parsed, err := url.ParseRequestURI(requestURI)
		if err != nil {
			t.Fatalf("the site was sent a malformed request %q: %v", requestURI, err)
		}
		if parsed.Path != "/course/view.php" {
			continue
		}
		courseViews++
		if query := parsed.Query(); len(query) != 1 || query.Get("id") != "2&edit=on&sesskey=x" {
			t.Errorf("the course reference changed the request: %q", requestURI)
		}
	}
	if courseViews == 0 {
		t.Errorf("the course page was never asked for, so nothing was checked: %q", requested)
	}
}
