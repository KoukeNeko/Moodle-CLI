package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
)

// Handler serves the protocol over the Streamable HTTP transport: one endpoint,
// JSON-RPC messages in POST bodies, the reply in the response.
//
// It answers with plain JSON and never opens an event stream, which the
// transport allows. This server has nothing to push — a tool call has one
// answer — so a stream would be machinery with no message to carry. For the
// same reason GET is refused, which is how a server says it offers no stream.
//
// Who may call is not decided here. Whatever mounts this handler decides that.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "this endpoint only accepts POST", http.StatusMethodNotAllowed)
		return
	}
	// A client that names a version this server does not speak is told so,
	// rather than being answered in a dialect it did not ask for. An absent
	// header is allowed: the specification tells a server to assume the
	// oldest revision then.
	//
	// This refusal is also how a newer client finds out what it is talking
	// to. Claude opens with a server/discover request under revision
	// 2026-07-28; on a 400 whose body is not a recognised modern JSON-RPC
	// error it falls back to initialize, and this server then speaks
	// 2025-06-18 with it. The body is therefore plain text on purpose: an
	// answer in the new revision's own error shape would tell the client this
	// is a modern server, and it would not fall back.
	if version := r.Header.Get("MCP-Protocol-Version"); version != "" &&
		!slices.Contains(supportedVersions, version) {
		http.Error(w, "unsupported MCP-Protocol-Version "+version, http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMessageBytes))
	if err != nil {
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			writeRPCError(w, http.StatusRequestEntityTooLarge, codeInvalidRequest, "message too large")
			return
		}
		writeRPCError(w, http.StatusBadRequest, codeParseError, "cannot read the request body")
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) > 0 && body[0] == '[' {
		// The 2025-06-18 revision dropped batching. Answering the first
		// element and ignoring the rest would be a silent loss, so it is
		// refused instead.
		writeRPCError(w, http.StatusBadRequest, codeInvalidRequest, "batched requests are not supported")
		return
	}

	reply := s.exchange(r.Context(), body)
	if reply == nil {
		// A notification: accepted, and nothing to say.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, http.StatusOK, reply)
}

func writeRPCError(w http.ResponseWriter, status, code int, message string) {
	writeJSON(w, status, response{
		JSONRPC: "2.0",
		ID:      json.RawMessage("null"),
		Error:   &rpcError{Code: code, Message: message},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	// Moodle content carries characters Go would otherwise escape; the client
	// reads UTF-8 either way, and readable output is worth more.
	encoder.SetEscapeHTML(false)
	// The status line is already sent; a write that fails now means the client
	// went away, and there is nobody left to tell.
	_ = encoder.Encode(body)
}
