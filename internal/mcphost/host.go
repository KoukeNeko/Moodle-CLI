// Package mcphost lets a client that is not on this machine use the MCP server.
//
// The stdio server answers whoever started it, so it only reaches a client
// that runs beside it. A Claude connector runs the other way round: Anthropic's
// side opens a connection to a public HTTPS address. This package is that
// address — the same tools served over Streamable HTTP, behind a sign-in that
// admits only the person who runs the host.
//
// It knows nothing about Moodle. It takes a handler that speaks the protocol
// and decides who may reach it.
package mcphost

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// minPassphrase is the shortest passphrase a person may choose.
const minPassphrase = 12

// Config says what name the host answers to and what it is serving.
type Config struct {
	// PublicURL is the address clients reach the host at, as an origin with no
	// path. Sign-in metadata names it, and a client that is told a different
	// address than the one it called refuses to continue.
	PublicURL string
	// Passphrase is what the owner types to admit a client. Empty means one
	// is generated for this run and printed.
	Passphrase string

	// SiteName, AccountName and AllowWrite are what the consent page tells
	// the owner they are handing over.
	SiteName    string
	AccountName string
	AllowWrite  bool

	// Log receives the startup notes and one line per request.
	Log io.Writer
}

// Host serves the protocol handler to remote clients.
type Host struct {
	config    Config
	authority *authority
	protocol  http.Handler
	generated bool

	// inflight is the context every tool call runs under. It ends when the
	// host stops waiting for calls to finish, and not when a client goes away.
	inflight     context.Context
	stopInflight context.CancelFunc
}

// shutdownGrace is how long a stopping host lets calls in flight finish before
// it tells them to stop. A variable so a test need not wait five seconds.
var shutdownGrace = 5 * time.Second

// New checks the configuration and builds a host around a handler that speaks
// the protocol.
func New(config Config, protocol http.Handler) (*Host, error) {
	origin, err := parsePublicURL(config.PublicURL)
	if err != nil {
		return nil, err
	}
	if config.Log == nil {
		config.Log = io.Discard
	}
	if err := CheckPassphrase(config.Passphrase); err != nil {
		return nil, err
	}
	host := &Host{config: config, protocol: protocol}
	host.inflight, host.stopInflight = context.WithCancel(context.Background())
	if passphrase := strings.TrimSpace(config.Passphrase); passphrase == "" {
		host.config.Passphrase = generatePassphrase()
		host.generated = true
	} else {
		// Compared trimmed, so it has to be kept trimmed: a padded one would
		// otherwise be a passphrase nobody can type.
		host.config.Passphrase = passphrase
	}
	host.authority = newAuthority(origin, host.config.Passphrase, config.SiteName, config.AllowWrite)
	return host, nil
}

// CheckPassphrase says whether a passphrase may be used, so a caller can refuse
// a bad one before it has done anything slow — such as bringing up a tunnel —
// rather than after. Empty is allowed: the host generates one.
func CheckPassphrase(passphrase string) error {
	passphrase = strings.TrimSpace(passphrase)
	if passphrase == "" || utf8.RuneCountInString(passphrase) >= minPassphrase {
		return nil
	}
	// Five tries per ten minutes still lets a patient guesser through a short
	// secret in days. The limit is for the owner's typos, not a substitute for
	// a passphrase worth guessing at.
	return errs.New(errs.CodeUsage,
		fmt.Sprintf("the host passphrase must be at least %d characters", minPassphrase)).
		WithHint("leave it out to have one generated")
}

// parsePublicURL accepts an origin: Claude derives every other address from
// it, and a path would have to be threaded through all of them.
func parsePublicURL(raw string) (string, error) {
	bad := func(why string) error {
		return errs.New(errs.CodeUsage, "--public-url "+why).
			WithHint("use the address Claude will reach, such as https://moodle.example.com")
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return "", bad("is not a web address")
	}
	if parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", bad("must be an origin with no path, query or credentials")
	}
	switch {
	case parsed.Scheme == "https":
	case parsed.Scheme == "http" && isLoopback(parsed.Hostname()):
		// For Claude Code on this machine. The hosted apps connect from
		// Anthropic's servers, which cannot reach a loopback address.
	default:
		return "", bad("must be https, since Claude only connects over HTTPS")
	}
	// Claude lowercases the host of the address it is given before comparing,
	// so an origin kept in mixed case would never equal what it sends back.
	return parsed.Scheme + "://" + strings.ToLower(parsed.Host), nil
}

// Handler returns everything the host serves.
func (h *Host) Handler() http.Handler {
	mux := http.NewServeMux()
	h.authority.routes(mux)
	mux.Handle(endpointPath, h.authority.protect(h.detached(h.protocol)))
	return logRequests(h.config.Log, mux)
}

// detached runs a request under the host's own lifetime instead of the
// connection's.
//
// Go cancels a request's context when its connection closes, and a tunnel
// closing a connection is routine. The transport specification says a dropped
// connection is not the client cancelling, and a write already sent to Moodle
// is no less sent: aborting it would only make its outcome unknown. What does
// end a call is the host being stopped, once calls in flight have had their
// grace period — the same signal a stdio server's tools get.
func (h *Host) detached(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
		defer cancel()
		defer context.AfterFunc(h.inflight, cancel)()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Listen binds the address the host will serve on. Loopback is the right place
// for it: a tunnel or reverse proxy on the same machine carries the traffic
// from there.
//
// Binding is separate from serving so that whatever has to point at the address
// can be started against a listener known to be this host's own. Pointing a
// tunnel at a port and hoping nothing else holds it would publish whatever
// does.
func Listen(addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, errs.Wrap(errs.CodeConfiguration, err, "cannot listen on "+addr).
			WithHint("choose another address with --listen")
	}
	return listener, nil
}

// Serve answers on listener until ctx ends, then lets requests in flight
// finish. It closes the listener.
func (h *Host) Serve(ctx context.Context, listener net.Listener) error {
	server := &http.Server{
		Handler: h.Handler(),
		// A slow client is holding a connection open for nothing. The
		// write side has no deadline: a tool call can take as long as Moodle
		// does, and the caller's own timeout is the limit that matters.
		ReadHeaderTimeout: 10 * time.Second,
		// The whole request, body included. A message is capped at a few
		// megabytes, so a minute is generous; a client slower than that is
		// holding a connection for nothing.
		ReadTimeout: time.Minute,
		IdleTimeout: 2 * time.Minute,
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if server.Shutdown(shutdown) != nil {
			// The grace period is over. Connections are closed so nothing
			// more is sent; the calls still running are told to stop when
			// Serve returns.
			_ = server.Close()
		}
	}()
	// Ends every call that is still running, with the same signal a stdio
	// server's tools get, so a write stops rather than carrying on into a
	// process that is exiting.
	defer h.stopInflight()

	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-stopped
		return nil
	}
	return err
}

// Announce tells the person running the host what it is serving and where to
// point Claude. It is separate from Serve so it can wait for Verify: the
// connector's address is not worth printing before it is known to work.
func (h *Host) Announce(listening net.Addr) {
	access := "read-only"
	if h.config.AllowWrite {
		access = "read and write"
	}
	fmt.Fprintf(h.config.Log, "moodle: hosting %s as %s, %s\n",
		h.config.SiteName, h.config.AccountName, access)
	fmt.Fprintf(h.config.Log, "moodle: listening on %s\n", listening)
	fmt.Fprintf(h.config.Log, "moodle: add %s as a custom connector in Claude\n", h.authority.endpoint)
	if h.generated {
		fmt.Fprintf(h.config.Log, "moodle: host passphrase for this run: %s\n", h.config.Passphrase)
		fmt.Fprintf(h.config.Log, "moodle: use --passphrase-file to keep one across runs\n")
	}
	fmt.Fprintf(h.config.Log, "moodle: sign-ins end when this process stops\n")
}

// How long Verify keeps trying, and how often. Variables so a test need not
// wait half a minute.
var (
	verifyWithin = 30 * time.Second
	verifyEvery  = time.Second
)

// Verify checks that the public address reaches this host and nothing else.
//
// It matters most for a tunnel the host started itself. Such a tunnel can come
// up, print an address, and still point somewhere other than the listener this
// host bound — a configuration file or an ingress rule of the person's own is
// enough — and then the address publishes a different service while this host
// sits unused. Asking the address for the host's own sign-in metadata, and
// requiring the issuer in it to be that address, answers whether the traffic
// arrives here. It retries for a while, because a new address can take a
// moment to be reachable.
//
// The host has to be serving when it is called.
func (h *Host) Verify(ctx context.Context) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
		// An answer from somewhere else is not this host's, however it got there.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	address := h.authority.origin + "/.well-known/oauth-authorization-server"
	deadline := time.Now().Add(verifyWithin)
	for {
		last := h.probe(ctx, client, address)
		if last == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s does not reach this host: %w", h.authority.origin, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(verifyEvery):
		}
	}
}

func (h *Host) probe(ctx context.Context, client *http.Client, address string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("it answered %d", response.StatusCode)
	}
	var metadata struct {
		Issuer string `json:"issuer"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&metadata); err != nil {
		return errors.New("it answered with something that is not this host's metadata")
	}
	if metadata.Issuer != h.authority.origin {
		return fmt.Errorf("it answered as %q", metadata.Issuer)
	}
	return nil
}

// generatePassphrase makes a secret a person can read off a terminal and type:
// 120 bits, in groups.
func generatePassphrase() string {
	raw := make([]byte, 15)
	rand.Read(raw) // never fails: the runtime aborts if the system has no randomness
	text := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	groups := make([]string, 0, len(text)/4)
	for len(text) > 0 {
		groups = append(groups, text[:4])
		text = text[4:]
	}
	return strings.Join(groups, "-")
}

// logRequests notes each request's method, path and outcome. The query is left
// out on purpose: the sign-in endpoints carry codes and states in it.
func logRequests(log io.Writer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)
		// The escaped form, because the path is whatever a stranger sent: a
		// decoded %0A would start a line of its own, and a decoded %1B would
		// reach the owner's terminal as a control sequence.
		fmt.Fprintf(log, "moodle: %s %s %d %s\n",
			r.Method, r.URL.EscapedPath(), recorder.status, time.Since(started).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}
