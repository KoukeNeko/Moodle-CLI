// Package testmoodle is a programmable stand-in for a Moodle site.
//
// It exists so the failure modes that matter can be reproduced exactly and
// offline: a Moodle exception behind HTTP 200, an expired token, a response
// that is cut off mid-flight, and above all a request the server processed
// whose reply never arrived — the case retry logic must never guess about
//.
package testmoodle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
)

// Handler answers one web service function.
type Handler func(params url.Values) (any, error)

// Server is a fake Moodle site.
type Server struct {
	server *httptest.Server

	mu        sync.Mutex
	functions map[string]Handler
	// requests records every call, so a test can assert that a dry run sent
	// no writes at all rather than merely that it reported none.
	requests []Request
	failures map[string]Failure
	tokens   map[string]bool
}

// Request is one recorded call.
type Request struct {
	Function string
	Token    string
	Params   url.Values
}

// Failure makes a function misbehave in a specific way.
type Failure string

const (
	// FailInvalidToken answers with Moodle's invalidtoken exception.
	FailInvalidToken Failure = "invalid-token"
	// FailServiceUnavailable answers as a site with web services switched off.
	FailServiceUnavailable Failure = "service-unavailable"
	// FailTruncated sends a Content-Length it does not honour, so the client
	// sees the connection drop mid-body.
	FailTruncated Failure = "truncated"
	// FailAppliedThenLost processes the request and then drops the connection:
	// the effect happened, the caller cannot know it. This is the ambiguous
	// outcome.
	FailAppliedThenLost Failure = "applied-then-lost"
	// FailTooManyRequests answers 429 with a Retry-After.
	FailTooManyRequests Failure = "429"
	// FailServerError answers 500.
	FailServerError Failure = "500"
	// FailHTML answers with an HTML page where JSON was expected.
	FailHTML Failure = "html"
)

// New starts a fake Moodle.
func New() *Server {
	s := &Server{
		functions: map[string]Handler{},
		failures:  map[string]Failure{},
		tokens:    map[string]bool{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/webservice/rest/server.php", s.handleREST)
	mux.HandleFunc("/login/token.php", s.handleToken)
	s.server = httptest.NewServer(mux)
	return s
}

// Close shuts the server down.
func (s *Server) Close() { s.server.Close() }

// URL is the site root.
func (s *Server) URL() string { return s.server.URL }

// Handle registers a function.
func (s *Server) Handle(function string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.functions[function] = handler
}

// HandleValue registers a function that always returns the same value.
func (s *Server) HandleValue(function string, value any) {
	s.Handle(function, func(url.Values) (any, error) { return value, nil })
}

// Fail makes a function misbehave. Passing an empty Failure clears it.
func (s *Server) Fail(function string, failure Failure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if failure == "" {
		delete(s.failures, function)
		return
	}
	s.failures[function] = failure
}

// AddToken marks a token as valid.
func (s *Server) AddToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = true
}

// Requests returns every call received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// CallsTo counts the calls made to a function.
func (s *Server) CallsTo(function string) int {
	count := 0
	for _, request := range s.Requests() {
		if request.Function == function {
			count++
		}
	}
	return count
}

func (s *Server) handleREST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	function := r.Form.Get("wsfunction")
	token := r.Form.Get("wstoken")

	s.mu.Lock()
	s.requests = append(s.requests, Request{Function: function, Token: token, Params: r.Form})
	handler, known := s.functions[function]
	failure := s.failures[function]
	tokenKnown := s.tokens[token]
	anyTokens := len(s.tokens) > 0
	s.mu.Unlock()

	if failure != "" {
		s.writeFailure(w, failure)
		return
	}
	if anyTokens && !tokenKnown {
		writeException(w, "moodle_exception", "invalidtoken", "Invalid token - token not found")
		return
	}
	if !known {
		// Moodle's own answer when a function is not exposed by the service.
		writeException(w, "webservice_access_exception", "accessexception", "Access control exception")
		return
	}
	value, err := handler(r.Form)
	if err != nil {
		writeException(w, "moodle_exception", "invalidparameter", err.Error())
		return
	}
	writeJSON(w, value)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username := r.Form.Get("username")
	password := r.Form.Get("password")
	if username == "" || password == "" {
		writeJSON(w, map[string]any{
			"error":     "Invalid login, please try again",
			"errorcode": "invalidlogin",
		})
		return
	}
	token := "token-for-" + username
	s.AddToken(token)
	writeJSON(w, map[string]any{"token": token, "privatetoken": nil})
}

func (s *Server) writeFailure(w http.ResponseWriter, failure Failure) {
	switch failure {
	case FailInvalidToken:
		writeException(w, "moodle_exception", "invalidtoken", "Invalid token - token not found")
	case FailServiceUnavailable:
		writeException(w, "moodle_exception", "enablewsdescription",
			"Web services must be enabled in Advanced features.")
	case FailTooManyRequests:
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	case FailServerError:
		w.WriteHeader(http.StatusInternalServerError)
	case FailHTML:
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!DOCTYPE html><html><body>Login required</body></html>")
	case FailTruncated, FailAppliedThenLost:
		// Promise more than is sent, then hang up: the client sees the body
		// end early. For AppliedThenLost the effect is already recorded above,
		// which is the whole point — the request ran, the answer was lost.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":`))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}
	}
}

func writeException(w http.ResponseWriter, exception, errorCode, message string) {
	// Moodle reports errors with HTTP 200; that is the trap this server has to
	// reproduce faithfully.
	writeJSON(w, map[string]any{
		"exception": exception,
		"errorcode": errorCode,
		"message":   message,
	})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(value)
}
