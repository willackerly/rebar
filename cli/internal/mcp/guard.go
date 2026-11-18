package mcp

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"time"
)

// Request hardening, as in the v3.1.2 python servers.

const (
	// MaxBodyBytes caps POST bodies (larger: 413).
	MaxBodyBytes = 1 << 20
	// RequestTimeout is how long a client may take to send its request.
	RequestTimeout = 60 * time.Second
)

// validName is valid_name: name can only be one directory entry — not
// empty, no leading "." or "-", no "..", no "/", "\", ":" (ask's
// project:agent separator) or control characters.
func validName(name string) bool {
	if name == "" || name[0] == '.' || name[0] == '-' || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r == ':' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// isLoopbackHost is is_loopback_host: localhost or a loopback IP literal
// (brackets allowed).
func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// hostHeaderIsLoopback is host_header_is_loopback: a Host header ("name",
// "name:port", "[v6]:port") naming a loopback host. A request without one
// (HTTP/1.0) passes: browsers, the DNS-rebinding vector, always send one.
func hostHeaderIsLoopback(r *http.Request) bool {
	if r.Host == "" {
		return true
	}
	v := strings.TrimSpace(r.Host)
	host := v
	switch {
	case strings.HasPrefix(v, "["):
		if end := strings.Index(v, "]"); end != -1 {
			host = v[:end+1]
		}
	case strings.Count(v, ":") == 1:
		host, _, _ = strings.Cut(v, ":")
	}
	return isLoopbackHost(host)
}

// apiKeyMatches compares an Authorization header with "Bearer <key>" in
// constant time.
func apiKeyMatches(header, key string) bool {
	return subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+key)) == 1
}

// checkHost is _check_host: a server bound to loopback only answers
// loopback Host names, so a web page cannot reach it by DNS rebinding.
func checkHost(w http.ResponseWriter, r *http.Request, loopbackOnly bool) bool {
	if !loopbackOnly || hostHeaderIsLoopback(r) {
		return true
	}
	jsonError(w, 403, "Host not allowed (server is bound to loopback)")
	return false
}

// checkJSONContentType is _check_json_content_type: POST bodies must be
// declared JSON, which browsers cannot send cross-site without a CORS
// preflight (never approved here).
func checkJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	ctype, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";")
	if strings.ToLower(strings.TrimSpace(ctype)) == "application/json" {
		return true
	}
	jsonError(w, 415, "Content-Type must be application/json")
	return false
}
