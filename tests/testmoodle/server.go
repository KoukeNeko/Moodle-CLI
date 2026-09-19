// Package testmoodle is a programmable stand-in for a Moodle site.
//
// It exists so the failure modes that matter can be reproduced exactly and
// offline: a Moodle exception behind HTTP 200, an expired token, a response
// that is cut off mid-flight, and above all a request the server processed
// whose reply never arrived — the case retry logic must never guess about.
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
	// exceptions override failures with a Moodle errorcode a test names
	// itself, for the codes that are worth classifying but have no canned
	// Failure of their own.
	exceptions map[string]exception
	tokens     map[string]bool
	// nextItemID is handed out by the upload endpoint.
	nextItemID int64
}

// Request is one recorded call.
type Request struct {
	Function string
	Token    string
	Params   url.Values
	// UserAgent is recorded so a test can prove that the Moodle app identity
	// is claimed for the QR exchange and for nothing else.
	UserAgent string
}

// UploadFunction is the name the upload endpoint is recorded and failed under.
// It is not a web service function, but it is a write, so tests need to name it.
const UploadFunction = "upload.php"

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

// exception is one of Moodle's error answers, as the site sends it.
type exception struct {
	Exception string
	ErrorCode string
	Message   string
}

// New starts a fake Moodle.
func New() *Server {
	s := &Server{
		functions:  map[string]Handler{},
		failures:   map[string]Failure{},
		exceptions: map[string]exception{},
		tokens:     map[string]bool{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/webservice/rest/server.php", s.handleREST)
	mux.HandleFunc("/login/token.php", s.handleToken)
	mux.HandleFunc("/lib/ajax/service-nologin.php", s.handleNoLogin)
	mux.HandleFunc("/webservice/upload.php", s.handleUpload)
	mux.HandleFunc("/my/", s.handleDashboard)
	s.server = httptest.NewServer(mux)
	return s
}

// Close shuts the server down.
func (s *Server) Close() { s.server.Close() }

// URL is the site root.
func (s *Server) URL() string { return s.server.URL }

// Handle registers a function.
// handleDashboard serves the page a browser session reads.
//
// It is the only place a session can learn two things Moodle does not expose
// over any endpoint it can reach: the session key, and who the session
// belongs to. Both are in the configuration block every signed-in page
// carries — the shape is copied from a real Moodle 5.2.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("MoodleSession"); err != nil {
		// No session: Moodle answers with the login page, which carries
		// neither value. That is how a client tells a rejected session from a
		// changed page.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Log in</body></html>`))
		return
	}
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(
		`<html><head><script>M.cfg = {"wwwroot":"` + s.URL() +
			`","sesskey":"testsesskey","userId":4};</script></head>` +
			`<body>Dashboard</body></html>`))
}

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

// FailException answers the function with a Moodle errorcode of the test's
// choosing. A canned Failure covers the codes that are already understood;
// this is for the ones being classified now, which a test has to be able to
// spell out. Passing an empty errorCode clears it.
func (s *Server) FailException(function, exceptionName, errorCode, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if errorCode == "" {
		delete(s.exceptions, function)
		return
	}
	s.exceptions[function] = exception{Exception: exceptionName, ErrorCode: errorCode, Message: message}
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
	s.requests = append(s.requests, Request{
		Function: function, Token: token, Params: r.Form,
		UserAgent: r.Header.Get("User-Agent"),
	})
	handler, known := s.functions[function]
	failure := s.failures[function]
	named, namedException := s.exceptions[function]
	tokenKnown := s.tokens[token]
	anyTokens := len(s.tokens) > 0
	s.mu.Unlock()

	if namedException {
		writeException(w, named.Exception, named.ErrorCode, named.Message)
		return
	}

	if failure == FailAppliedThenLost && known {
		// The name is the behaviour: the site really does the work, and only
		// the answer is lost. A fake that skipped the handler here would let
		// reconciliation pass by never having anything to reconcile.
		_, _ = handler(r.Form)
	}
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

// handleNoLogin serves the functions Moodle exposes without a token, such as
// the pre-login site configuration. The reply is an array with one entry per
// call, which is a different shape from the REST endpoint.
func (s *Server) handleNoLogin(w http.ResponseWriter, r *http.Request) {
	function := r.URL.Query().Get("info")

	s.mu.Lock()
	s.requests = append(s.requests, Request{
		Function: function, UserAgent: r.Header.Get("User-Agent"),
	})
	handler, known := s.functions[function]
	failure := s.failures[function]
	named, namedException := s.exceptions[function]
	s.mu.Unlock()

	// 命名錯誤碼要在兩個端點都生效。QR 交換走的是這裡（service-nologin），
	// 而測試不該因為客戶端碰巧用了哪個端點就寫不出那個錯誤。
	if namedException {
		writeNoLoginException(w, named.Exception, named.ErrorCode, named.Message)
		return
	}
	if failure != "" {
		s.writeFailure(w, failure)
		return
	}
	if !known {
		writeJSON(w, []map[string]any{{
			"error": "unknown function",
			"exception": map[string]any{
				"exception": "moodle_exception",
				"errorcode": "accessexception",
				"message":   "Access control exception",
			},
		}})
		return
	}
	value, err := handler(r.URL.Query())
	if err != nil {
		writeJSON(w, []map[string]any{{
			"error": err.Error(),
			"exception": map[string]any{
				"exception": "moodle_exception",
				"errorcode": "invalidparameter",
				"message":   err.Error(),
			},
		}})
		return
	}
	writeJSON(w, []map[string]any{{"error": false, "data": value}})
}

// handleUpload stands in for the draft file area.
//
// It is a separate endpoint with its own conventions: the token travels in the
// query string rather than the form, and a failure comes back inside the array
// instead of as an exception. Recording it as a request matters because
// allocating a draft area is a write, and a dry run must not reach here.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	s.mu.Lock()
	s.requests = append(s.requests, Request{
		Function: UploadFunction, Token: token,
		UserAgent: r.Header.Get("User-Agent"),
	})
	failure := s.failures[UploadFunction]
	tokenKnown := s.tokens[token]
	anyTokens := len(s.tokens) > 0
	s.nextItemID++
	itemID := s.nextItemID
	s.mu.Unlock()

	if failure != "" {
		s.writeFailure(w, failure)
		return
	}
	if anyTokens && !tokenKnown {
		writeException(w, "moodle_exception", "invalidtoken", "Invalid token - token not found")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, []map[string]any{{
			"error": "invalid upload", "errorcode": "invalidparameter",
		}})
		return
	}

	var files []map[string]any
	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			files = append(files, map[string]any{
				"filename": header.Filename,
				// Moodle sends this as a number, which is why the client reads
				// it as a json.Number rather than a string.
				"itemid":   itemID,
				"filesize": header.Size,
			})
		}
	}
	if len(files) == 0 {
		writeJSON(w, []map[string]any{{
			"error": "no file given", "errorcode": "nofile",
		}})
		return
	}
	writeJSON(w, files)
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

// writeNoLoginException is the same failure in the shape the no-login endpoint
// uses: an array of per-call results, each one carrying its own exception.
func writeNoLoginException(w http.ResponseWriter, exception, errorCode, message string) {
	writeJSON(w, []map[string]any{{
		"error": message,
		"exception": map[string]any{
			"exception": exception,
			"errorcode": errorCode,
			"message":   message,
		},
	}})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(value)
}
