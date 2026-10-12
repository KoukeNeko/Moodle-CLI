package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"
)

// Streams are the process streams the server speaks over.
//
// Out carries the protocol and nothing else. Anything this server wants to say
// to a human goes to Log, because a single stray line on stdout makes the
// stream unparseable for the client.
type Streams struct {
	In  io.Reader
	Out io.Writer
	Log io.Writer
}

// Server answers Model Context Protocol requests.
type Server struct {
	streams Streams
	build   BuildInfo
	tools   *Registry

	// mu guards writes to Out. Requests are handled one at a time today, but
	// the writer is shared and interleaved JSON would be unrecoverable.
	mu sync.Mutex
	// initialized records the handshake. A client that calls a tool before
	// initializing is a client with a bug, and answering anyway would hide it.
	initialized bool
	// stateless drops the handshake check. Over HTTP every request arrives on
	// its own, so there is no session to remember a handshake in.
	stateless bool
	// callMu serialises tool calls. The session behind the tools keeps its
	// capability answer and credential in plain fields, and over HTTP two
	// requests can be in flight at once.
	callMu sync.Mutex
}

// BuildInfo identifies this binary to the client.
type BuildInfo struct {
	Version string
}

// NewServer builds a server over the given streams.
func NewServer(streams Streams, build BuildInfo, tools *Registry) *Server {
	if streams.Log == nil {
		streams.Log = io.Discard
	}
	return &Server{streams: streams, build: build, tools: tools}
}

// NewHTTPServer builds a server for the Streamable HTTP transport.
//
// It keeps no session between requests: every POST stands alone, so a client
// can reconnect after a restart without a handshake to replay.
func NewHTTPServer(log io.Writer, build BuildInfo, tools *Registry) *Server {
	server := NewServer(Streams{Out: io.Discard, Log: log}, build, tools)
	server.stateless = true
	return server
}

// maxMessageBytes caps one message.
//
// A line scanner stops at 64KB by default, which a real tool call can exceed —
// a list of file paths, or an argument carrying a document. The cap is raised
// rather than removed: it still has to be bounded, because the size is chosen
// by whoever is on the other end of the pipe.
const maxMessageBytes = 8 << 20

// Serve reads requests until the input ends or the context is cancelled.
//
// Messages are read a line at a time, which is what the stdio transport
// specifies. Reading with a streaming decoder instead would be a trap: on a
// malformed byte it reports the same error from the same position forever, so
// one bad line from a client would spin here emitting error frames until
// something killed the process.
//
// The loop ends quietly on EOF: a client closing the pipe is how a session
// normally finishes, not a failure to report.
func (s *Server) Serve(ctx context.Context) error {
	lines := bufio.NewScanner(s.streams.In)
	lines.Buffer(make([]byte, 0, 64<<10), maxMessageBytes)

	for lines.Scan() {
		if err := ctx.Err(); err != nil {
			return nil
		}
		line := bytes.TrimSpace(lines.Bytes())
		if len(line) == 0 {
			continue
		}
		// A malformed line is answered and the next one is still read: one
		// bad message has not ended the session.
		if reply := s.exchange(ctx, line); reply != nil && s.write(*reply) != nil {
			return nil
		}
	}

	if err := lines.Err(); err != nil && !errors.Is(err, io.EOF) {
		// A line too long for the buffer lands here. Saying so on stderr is
		// the only thing left: the stream position is lost either way.
		fmt.Fprintf(s.streams.Log, "moodle: cannot read from the client: %v\n", err)
	}
	return nil
}

// exchange answers one message, whatever carried it. It returns nil when the
// message needs no reply.
//
// The transports only differ in how a message arrives and where the reply
// goes; what the server says back is decided here, once.
func (s *Server) exchange(ctx context.Context, raw []byte) *response {
	var message request
	if err := json.Unmarshal(raw, &message); err != nil {
		// The id is unknown, so the reply carries a null id as the protocol
		// requires.
		return &response{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: codeParseError, Message: "invalid JSON"},
		}
	}
	if message.JSONRPC != "2.0" {
		return failure(message, codeInvalidRequest, "jsonrpc must be \"2.0\"", nil)
	}
	return s.answer(ctx, message)
}

func (s *Server) answer(ctx context.Context, message request) *response {
	switch message.Method {
	case "initialize":
		return s.handleInitialize(message)
	case "notifications/initialized":
		// Nothing to do, and nothing to answer: it is a notification.
		return nil
	case "ping":
		return reply(message, map[string]any{})
	case "tools/list":
		if refused := s.refuseBeforeHandshake(message); refused != nil {
			return refused
		}
		return reply(message, toolsListResult{Tools: s.tools.describe()})
	case "tools/call":
		if message.isNotification() {
			// A call is a request. Without an id its result could never be
			// reported, so running it would be a write nobody can see the
			// outcome of — and the handshake check below cannot refuse it,
			// because refusing a notification is silent.
			return nil
		}
		if refused := s.refuseBeforeHandshake(message); refused != nil {
			return refused
		}
		return s.handleCall(ctx, message)
	default:
		if message.isNotification() {
			// An unknown notification is ignored rather than answered: the
			// protocol forbids a reply, and refusing to run is worse than
			// carrying on.
			return nil
		}
		return failure(message, codeMethodNotFound,
			fmt.Sprintf("unknown method %q", message.Method), nil)
	}
}

// refuseBeforeHandshake answers work that arrived before the handshake, and
// returns nil when the work may go ahead.
func (s *Server) refuseBeforeHandshake(message request) *response {
	if s.stateless || s.initialized {
		return nil
	}
	return failure(message, codeInvalidRequest,
		"the session has not been initialized", nil)
}

func (s *Server) handleInitialize(message request) *response {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(message.Params) > 0 {
		// A malformed params object is not fatal here: the handshake can still
		// proceed on this server's own version.
		_ = json.Unmarshal(message.Params, &params)
	}

	// Answer in the client's version when it is one we speak, so an older
	// client is not forced to give up on a newer server.
	version := protocolVersion
	if slices.Contains(supportedVersions, params.ProtocolVersion) {
		version = params.ProtocolVersion
	}

	if !s.stateless {
		s.initialized = true
	}
	return reply(message, initializeResult{
		ProtocolVersion: version,
		Capabilities:    capabilities{Tools: &toolsCapability{ListChanged: false}},
		ServerInfo: serverInfo{
			Name: "moodle-cli", Version: s.build.Version, Title: "Moodle",
		},
		Instructions: s.tools.instructions(),
	})
}

func (s *Server) handleCall(ctx context.Context, message request) *response {
	var params callToolParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		return failure(message, codeInvalidParams, "params must be an object with a name", nil)
	}

	s.callMu.Lock()
	defer s.callMu.Unlock()
	started := time.Now()
	result := s.tools.call(ctx, params.Name, params.Arguments)
	if s.stateless {
		s.audit(params.Name, result.IsError, time.Since(started))
	}
	return reply(message, result)
}

// audit notes one tool call on the log: which tool, whether it worked, and how
// long it took. Arguments are left out because they carry what a person asked
// about.
//
// Only a host needs it. Whoever runs one is not watching the conversation, and
// with writing enabled "what did Claude do to my account" has to have an
// answer; a stdio server's client shows each call to the person using it.
func (s *Server) audit(name string, failed bool, took time.Duration) {
	outcome := "ok"
	if failed {
		outcome = "error"
	}
	fmt.Fprintf(s.streams.Log, "moodle: tool %q %s %s\n", name, outcome, took.Round(time.Millisecond))
}

// reply wraps a result, or returns nil for a notification.
func reply(message request, result any) *response {
	if message.isNotification() {
		return nil
	}
	return &response{JSONRPC: "2.0", ID: message.ID, Result: result}
}

// failure wraps an error, or returns nil for a notification.
func failure(message request, code int, text string, data any) *response {
	if message.isNotification() {
		return nil
	}
	id := message.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return &response{
		JSONRPC: "2.0", ID: id,
		Error: &rpcError{Code: code, Message: text, Data: data},
	}
}

func (s *Server) write(message response) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encoder := json.NewEncoder(s.streams.Out)
	// Moodle content carries characters Go would otherwise escape; the client
	// reads UTF-8 either way, and readable output is worth more.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(message); err != nil {
		fmt.Fprintf(s.streams.Log, "moodle: cannot write to the client: %v\n", err)
		return err
	}
	return nil
}
