package mcp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Case-insensitive aliases, kept from the python server: a tools/call of
// ask_tdflite_architect reaches the registered ask_TDFLite_architect, and
// resources/read of ask://memory/tdflite:Architect reaches
// ask://memory/TDFLite:architect. A user hit exactly this (the 2026-04-28
// usability red team, finding C2: "can't see TDFLite"), so the names are
// rewritten to the registered spelling before the SDK looks them up; names
// that match nothing are left alone and get the SDK's errors.

// aliases is the receiving middleware that canonicalizes tool names and
// resource URIs. It also guards the SDK (v1.8.0): "arguments": null
// panicked in its schema defaults (a nil map) and took the server down, so
// null arguments count as absent, and a panic in a handler becomes an
// internal error instead of a crash. (Fixed upstream in v1.9.0:
// modelcontextprotocol/go-sdk#1340, #1343; the null guard can go with the
// upgrade.)
func (s *Server) aliases(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (res sdk.Result, err error) {
		defer func() {
			if r := recover(); r != nil {
				res, err = nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: fmt.Sprintf("internal error: %v", r)}
			}
		}()
		switch p := req.GetParams().(type) {
		case *sdk.CallToolParamsRaw:
			if method == "tools/call" && p != nil {
				p.Name = s.canonicalTool(p.Name)
				if string(bytes.TrimSpace(p.Arguments)) == "null" {
					p.Arguments = nil
				}
			}
		case *sdk.ReadResourceParams:
			if method == "resources/read" && p != nil {
				p.URI = s.canonicalURI(p.URI)
			}
		}
		return next(ctx, method, req)
	}
}

// canonicalTool is the registered tool name matching name, ignoring case
// (name itself when it is registered or matches none).
func (s *Server) canonicalTool(name string) string {
	var match string
	for _, a := range s.Registry.Agents() {
		switch t := ToolName(a); {
		case t == name:
			return name
		case match == "" && strings.EqualFold(t, name):
			match = t
		}
	}
	if match == "" {
		return name
	}
	return match
}

// canonicalURI is ask://<kind>/<repo>:<role> with the registered spelling
// of repo and role (uri itself when they match none).
func (s *Server) canonicalURI(uri string) string {
	rest, ok := strings.CutPrefix(uri, "ask://")
	if !ok {
		return uri
	}
	kind, id, ok := strings.Cut(rest, "/")
	if !ok {
		return uri
	}
	repo, role, ok := strings.Cut(id, ":")
	if !ok {
		return uri
	}
	// Names are path-escaped in the URIs (resourceURI); a client may send
	// them escaped or not.
	if u, err := url.PathUnescape(repo); err == nil {
		repo = u
	}
	if u, err := url.PathUnescape(role); err == nil {
		role = u
	}
	canonRepo := s.Registry.ResolveRepo(repo)
	if canonRepo == "" {
		return uri
	}
	canonRole := s.Registry.ResolveRole(canonRepo, role)
	if canonRole == "" {
		return uri
	}
	return resourceURI(kind, Agent{canonRepo, canonRole})
}

// The SDK (v1.8.0) answers some protocol errors with code 0, a plain Go
// error raised before any middleware runs; code 0 is not a JSON-RPC error
// code. Answers are rewritten on their way out (stdout, /mcp):
//
//   - a request before initialize ("method "<m>" is invalid during session
//     initialization") and a second initialize get -32600 (invalid
//     request): the first is what the SDK itself answers from v1.9.0 on
//     (modelcontextprotocol/go-sdk, server.go, "nopreinitinvalidrequest"),
//     so upgrading changes nothing on the wire. (The python server used
//     -32002, a code MCP has also used for "resource not found".)
//   - a request missing its required params gets -32602 (invalid params);
//   - any other code-0 error gets -32603 (internal error).
const codeNotInitialized = -32600

var codeZero = regexp.MustCompile(`"error":\{"code":0,"message":"((?:[^"\\]|\\.)*)"`)

// errorCode is the JSON-RPC code for an SDK error message sent with 0.
func errorCode(message string) int {
	switch {
	case strings.HasSuffix(message, `is invalid during session initialization`),
		strings.HasPrefix(message, `duplicate \"initialize\"`):
		return codeNotInitialized
	case strings.Contains(message, `missing required \"params\"`):
		return jsonrpc.CodeInvalidParams
	}
	return jsonrpc.CodeInternalError
}

// fixErrorCodes is b with code-0 errors given their JSON-RPC codes.
func fixErrorCodes(b []byte) []byte {
	if !bytes.Contains(b, []byte(`"error":{"code":0,`)) {
		return b
	}
	return codeZero.ReplaceAllFunc(b, func(m []byte) []byte {
		msg := codeZero.FindSubmatch(m)[1]
		return []byte(fmt.Sprintf(`"error":{"code":%d,"message":"%s"`, errorCode(string(msg)), msg))
	})
}

// notInitializedWriter applies fixErrorCodes to the SDK's HTTP answers
// (one JSON-RPC message or event per Write).
type notInitializedWriter struct{ http.ResponseWriter }

func (w notInitializedWriter) Write(b []byte) (int, error) {
	if _, err := w.ResponseWriter.Write(fixErrorCodes(b)); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w notInitializedWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w notInitializedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
