package mcp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
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
	canonRepo := s.Registry.ResolveRepo(repo)
	if canonRepo == "" {
		return uri
	}
	canonRole := s.Registry.ResolveRole(canonRepo, role)
	if canonRole == "" {
		return uri
	}
	return "ask://" + kind + "/" + canonRepo + ":" + canonRole
}

// A request before initialize gets the SDK's error "method "<m>" is invalid
// during session initialization" with code 0 (a plain Go error, raised
// before any middleware runs). Code 0 is not a JSON-RPC error code, so the
// answer is rewritten on its way out to -32600 (invalid request): what the
// SDK itself answers from v1.9.0 on (modelcontextprotocol/go-sdk, server.go,
// "nopreinitinvalidrequest"), so upgrading changes nothing on the wire and
// this rewrite can go then. (The python server used -32002, a code MCP has
// also used for "resource not found".)
const codeNotInitialized = -32600

var notInitialized = regexp.MustCompile(`"error":\{"code":0,("message":"method \\"[^"\\]*\\" is invalid during session initialization"\})`)

// fixNotInitialized is b with the SDK's not-initialized errors given
// codeNotInitialized.
func fixNotInitialized(b []byte) []byte {
	if !bytes.Contains(b, []byte("is invalid during session initialization")) {
		return b
	}
	return notInitialized.ReplaceAll(b, []byte(fmt.Sprintf(`"error":{"code":%d,$1`, codeNotInitialized)))
}

// notInitializedWriter applies fixNotInitialized to the SDK's HTTP answers
// (one JSON-RPC message or event per Write).
type notInitializedWriter struct{ http.ResponseWriter }

func (w notInitializedWriter) Write(b []byte) (int, error) {
	if _, err := w.ResponseWriter.Write(fixNotInitialized(b)); err != nil {
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
