package mcp

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTP hardening shared by `rebar mcp serve` (HTTP mode) and `rebar ask
// serve`. Both run an LLM per request on behalf of whoever can reach them,
// so:
//
//   - they listen on 127.0.0.1 unless told otherwise, and refuse a
//     non-loopback address without --api-key (unless --insecure-no-auth);
//   - every request (not only /mcp) arriving on a loopback address must name
//     a loopback Host (DNS rebinding), and browsers' cross-origin requests
//     are refused (http.CrossOriginProtection);
//   - POST bodies must be application/json (no CORS-simple text/plain
//     posts) and at most MaxBodyBytes, read within BodyReadTimeout; /v1/ask
//     also needs a Content-Length (411);
//   - API keys are compared in constant time;
//   - at most MaxAsks asks run at once; more get 503 (busy).
var (
	// MaxBodyBytes caps a request body (413 beyond it).
	MaxBodyBytes int64 = 1 << 20
	// BodyReadTimeout bounds reading a request body (60 s, the python
	// servers' request timeout). It is not the server's ReadTimeout: that
	// deadline stays on the connection while the handler runs, and
	// net/http cancels the request when it passes, which would cut every
	// ask longer than it.
	BodyReadTimeout = 60 * time.Second
	// ReadHeaderTimeout and IdleTimeout are the http.Server's.
	ReadHeaderTimeout = 10 * time.Second
	IdleTimeout       = 120 * time.Second
	// DefaultMaxAsks is --max-asks' default.
	DefaultMaxAsks = 8
)

// isLoopbackHost reports whether host (a --host value or a Host header
// without its port) names the loopback interface.
func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkBind is the start-up rule (as in the v3.1.2 python servers): a
// non-loopback address needs an API key, or --insecure-no-auth, which is
// warned about on warn.
func checkBind(prog string, o *options, warn io.Writer) error {
	if isLoopbackHost(o.host) || o.apiKey != "" {
		return nil
	}
	shown := o.host
	if shown == "" {
		shown = "0.0.0.0"
	}
	if o.insecureNoAuth {
		fmt.Fprintf(warn, "%s: WARNING: listening on %s with no API key (--insecure-no-auth): "+
			"anyone who can reach this port can query every agent\n", prog, shown)
		return nil
	}
	return fmt.Errorf("refusing to listen on non-loopback address '%s' without an API key.\n"+
		"  Set --api-key KEY (or ASK_API_KEY), or bind --host 127.0.0.1, or pass\n"+
		"  --insecure-no-auth to serve unauthenticated on a trusted network.", shown)
}

// newHTTPServer is the http.Server both servers run.
func newHTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: ReadHeaderTimeout, IdleTimeout: IdleTimeout}
}

// crossOrigin refuses browsers' cross-origin non-GET requests.
var crossOrigin = http.NewCrossOriginProtection()

// guardHost applies the Host and cross-origin rules (before auth); false
// means it answered.
func guardHost(w http.ResponseWriter, r *http.Request) bool {
	if !hostAllowed(r) {
		jsonError(w, 403, "Host not allowed (server is bound to loopback)")
		return false
	}
	if err := crossOrigin.Check(r); err != nil {
		jsonError(w, 403, "Forbidden: cross-origin request")
		return false
	}
	return true
}

// guardBody applies the body rules (after auth): POSTs are
// application/json (415), bodies at most MaxBodyBytes (413), read within
// BodyReadTimeout. False means it answered.
func guardBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost {
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			jsonError(w, 415, "Content-Type must be application/json")
			return false
		}
	}
	if r.ContentLength > MaxBodyBytes {
		tooLargeError(w)
		return false
	}
	if r.ContentLength != 0 && r.Body != nil && r.Body != http.NoBody {
		r.Body = newBody(w, r.Body)
	}
	return true
}

// tooLargeError is the 413 answer.
func tooLargeError(w http.ResponseWriter) {
	jsonError(w, 413, fmt.Sprintf("Request body too large (max %d bytes)", MaxBodyBytes))
}

// hostAllowed: a request that arrived on a loopback address must carry a
// loopback Host (a browser lured to a rebinding domain sends that domain).
// No Host at all (an HTTP/1.0 client) is allowed: browsers always send one.
func hostAllowed(r *http.Request) bool {
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return true
	}
	tcp, ok := local.(*net.TCPAddr)
	if !ok || !tcp.IP.IsLoopback() || r.Host == "" {
		return true
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return isLoopbackHost(host)
}

// body is a request body capped at MaxBodyBytes and read within
// BodyReadTimeout; the read deadline is lifted once it has been read to
// the end, so the handler (an ask of minutes) is not cut short.
type body struct {
	io.ReadCloser
	rc   *http.ResponseController
	done bool
}

func newBody(w http.ResponseWriter, r io.ReadCloser) *body {
	rc := http.NewResponseController(w)
	rc.SetReadDeadline(time.Now().Add(BodyReadTimeout))
	return &body{ReadCloser: http.MaxBytesReader(w, r, MaxBodyBytes), rc: rc}
}

func (b *body) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF && !b.done {
		// Read whole: lift the deadline. (Not on an error: the server
		// still discards the rest of an unread body, and that must not
		// wait forever.)
		b.done = true
		b.rc.SetReadDeadline(time.Time{})
	}
	return n, err
}

// tooLarge reports whether err is the body cap's.
func tooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

// keyMatches compares a presented API key with the configured one in
// constant time.
func keyMatches(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// limiter caps concurrent asks; a nil limiter (or n <= 0) does not.
type limiter chan struct{}

func newLimiter(n int) limiter {
	if n <= 0 {
		return nil
	}
	return make(limiter, n)
}

// acquire takes a slot without waiting; ok is false when all are taken.
func (l limiter) acquire() (release func(), ok bool) {
	if l == nil {
		return func() {}, true
	}
	select {
	case l <- struct{}{}:
		return func() { <-l }, true
	default:
		return nil, false
	}
}

// busyMessage is the answer when every slot is taken.
func busyMessage(n int) string {
	return fmt.Sprintf("Server busy: %d asks already running; try again later", n)
}

// busy answers 503 with Retry-After.
func busy(w http.ResponseWriter, n int) {
	w.Header().Set("Retry-After", "5")
	jsonError(w, 503, busyMessage(n))
}
