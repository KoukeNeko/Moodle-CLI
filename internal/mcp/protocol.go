// Package mcp serves the use cases to an AI agent over the Model Context
// Protocol.
//
// It shares the use cases with the command line rather than shelling out to
// it: an agent and a person asking the same question must get the same answer,
// and a second path to the same data is a second place for it to drift.
//
// The transport is JSON-RPC 2.0 over stdio, which means stdout carries the
// protocol and nothing else. That is the same rule the CLI already follows for
// its JSON contract, for the same reason: one stray line of progress text and
// the consumer cannot parse the stream.
package mcp

import "encoding/json"

// protocolVersion is what this server speaks. A client asking for a version we
// know is answered in its own version; anything else is answered in ours and
// the client decides whether to continue.
const protocolVersion = "2025-06-18"

// supportedVersions are the versions this server can speak, newest first.
var supportedVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// request is an incoming JSON-RPC message.
//
// ID is kept as a raw message because JSON-RPC allows a string or a number and
// the reply has to echo back exactly what arrived. Rewriting 3 as "3" makes a
// client wait forever for a reply it will not recognise.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// isNotification reports whether the message expects no reply. A notification
// has no id, and answering one is a protocol violation.
func (r request) isNotification() bool { return len(r.ID) == 0 }

// response is an outgoing JSON-RPC message.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC error codes. These are the protocol's own, not this project's exit
// codes: a transport failure is a different thing from a Moodle failure, and a
// tool that failed is reported as a successful call carrying an error result.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// serverInfo identifies this server to the client.
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Title   string `json:"title,omitempty"`
}

// initializeResult answers the handshake.
type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	// Instructions tell the agent how to treat this server. The read-only
	// state is said here because it changes what the agent should even
	// attempt.
	Instructions string `json:"instructions,omitempty"`
}

// capabilities says what this server offers. Only tools for now: there are no
// resources or prompts to expose that the tools do not already cover.
type capabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	// ListChanged reports whether the tool list can change during a session.
	// It cannot here: the surface is decided at startup from the site's
	// capabilities and the read-only setting.
	ListChanged bool `json:"listChanged"`
}

// tool is one callable tool.
type tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	// Annotations tell the agent what a tool does before it calls it. They are
	// hints, not enforcement — the enforcement is in the use cases.
	Annotations *toolAnnotations `json:"annotations,omitempty"`
}

// toolAnnotations describe a tool's effects.
type toolAnnotations struct {
	Title string `json:"title,omitempty"`
	// ReadOnlyHint says the tool changes nothing.
	ReadOnlyHint bool `json:"readOnlyHint"`
	// DestructiveHint says the tool can remove or overwrite something. A
	// submission replaces whatever was there before, so it counts.
	DestructiveHint bool `json:"destructiveHint"`
	// IdempotentHint says calling twice is the same as calling once. No Moodle
	// write carries an idempotency key, so this is never true for a write.
	IdempotentHint bool `json:"idempotentHint"`
	// OpenWorldHint says the tool talks to something outside this process.
	OpenWorldHint bool `json:"openWorldHint"`
}

// toolsListResult answers tools/list.
type toolsListResult struct {
	Tools []tool `json:"tools"`
}

// callToolParams is the argument of tools/call.
type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// callToolResult is a tool's outcome.
//
// A tool that failed is still a successful JSON-RPC call: IsError tells the
// agent the tool did not work, which lets it read the message and try
// something else rather than treating the transport as broken.
type callToolResult struct {
	Content []content `json:"content"`
	// StructuredContent carries the same answer as data rather than text, so
	// an agent does not have to parse prose back into fields.
	StructuredContent any  `json:"structuredContent,omitempty"`
	IsError           bool `json:"isError,omitempty"`
}

// content is one piece of a tool's answer.
type content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func textContent(text string) []content {
	return []content{{Type: "text", Text: text}}
}
