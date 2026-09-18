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

		var message request
		if err := json.Unmarshal(line, &message); err != nil {
			// The id is unknown, so the reply carries a null id as the
			// protocol requires. The next line is still read: one bad message
			// has not ended the session.
			if s.write(response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpcError{Code: codeParseError, Message: "invalid JSON"},
			}) != nil {
				return nil
			}
			continue
		}

		if message.JSONRPC != "2.0" {
			s.fail(message, codeInvalidRequest, "jsonrpc must be \"2.0\"", nil)
			continue
		}
		s.handle(ctx, message)
	}

	if err := lines.Err(); err != nil && !errors.Is(err, io.EOF) {
		// A line too long for the buffer lands here. Saying so on stderr is
		// the only thing left: the stream position is lost either way.
		fmt.Fprintf(s.streams.Log, "moodle: cannot read from the client: %v\n", err)
	}
	return nil
}

func (s *Server) handle(ctx context.Context, message request) {
	switch message.Method {
	case "initialize":
		s.handleInitialize(message)
	case "notifications/initialized":
		// Nothing to do, and nothing to answer: it is a notification.
	case "ping":
		s.reply(message, map[string]any{})
	case "tools/list":
		if !s.ready(message) {
			return
		}
		s.reply(message, toolsListResult{Tools: s.tools.describe()})
	case "tools/call":
		if !s.ready(message) {
			return
		}
		s.handleCall(ctx, message)
	default:
		if message.isNotification() {
			// An unknown notification is ignored rather than answered: the
			// protocol forbids a reply, and refusing to run is worse than
			// carrying on.
			return
		}
		s.fail(message, codeMethodNotFound,
			fmt.Sprintf("unknown method %q", message.Method), nil)
	}
}

// ready refuses work before the handshake.
func (s *Server) ready(message request) bool {
	if s.initialized {
		return true
	}
	s.fail(message, codeInvalidRequest,
		"the session has not been initialized", nil)
	return false
}

func (s *Server) handleInitialize(message request) {
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

	s.initialized = true
	s.reply(message, initializeResult{
		ProtocolVersion: version,
		Capabilities:    capabilities{Tools: &toolsCapability{ListChanged: false}},
		ServerInfo: serverInfo{
			Name: "moodle-cli", Version: s.build.Version, Title: "Moodle",
		},
		Instructions: s.tools.instructions(),
	})
}

func (s *Server) handleCall(ctx context.Context, message request) {
	var params callToolParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.fail(message, codeInvalidParams, "params must be an object with a name", nil)
		return
	}

	result := s.tools.call(ctx, params.Name, params.Arguments)
	s.reply(message, result)
}

func (s *Server) reply(message request, result any) {
	if message.isNotification() {
		return
	}
	_ = s.write(response{JSONRPC: "2.0", ID: message.ID, Result: result})
}

func (s *Server) fail(message request, code int, text string, data any) {
	if message.isNotification() {
		return
	}
	id := message.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	_ = s.write(response{
		JSONRPC: "2.0", ID: id,
		Error: &rpcError{Code: code, Message: text, Data: data},
	})
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
