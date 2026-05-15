// Package repo derives a repo's canonical contract namespace from its
// git remote.
//
// Architecture: CONTRACT:github.com/willackerly/rebar:S2-ASK-CLI.1.0
package repo

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ErrNoRemote indicates the repo has no `origin` remote configured.
// Callers should surface this with guidance to pass --namespace or set
// contract_namespace in .rebarrc.
var ErrNoRemote = errors.New("no git remote origin configured")

// InferNamespace runs `git remote get-url origin` in repoRoot and
// returns the Go-module form (host/org/repo). Returns ErrNoRemote if
// origin is not set.
func InferNamespace(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		// `git remote get-url` exits 2 when the remote is missing.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", ErrNoRemote
		}
		return "", fmt.Errorf("running git remote get-url: %w", err)
	}
	return ParseRemoteURL(strings.TrimSpace(string(out)))
}

var (
	// SSH short form: git@host:org/repo.git or user@host:path/repo
	sshShortRE = regexp.MustCompile(`^[A-Za-z0-9._-]+@([A-Za-z0-9.-]+):(.+)$`)
	// URL form with scheme: ssh://, https://, http://, git://
	urlSchemeRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
)

// ParseRemoteURL normalizes a git remote URL to host/org/repo. The host
// is lowercased; the path portion is preserved case-sensitively because
// path case is host-defined. Trailing `.git` is stripped.
//
// Supported inputs:
//
//	git@github.com:willackerly/rebar.git          → github.com/willackerly/rebar
//	https://github.com/willackerly/rebar.git      → github.com/willackerly/rebar
//	https://github.com/willackerly/rebar          → github.com/willackerly/rebar
//	ssh://git@gitlab.com/team/foo.git             → gitlab.com/team/foo
//	git@gitea.example.com:nested/group/repo.git   → gitea.example.com/nested/group/repo
func ParseRemoteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("remote URL is empty")
	}

	var host, path string

	switch {
	case urlSchemeRE.MatchString(raw):
		// Strip scheme.
		idx := strings.Index(raw, "://")
		rest := raw[idx+3:]
		// Strip optional userinfo (user@ or user:pass@).
		if at := strings.Index(rest, "@"); at >= 0 && at < strings.Index(rest+"/", "/") {
			rest = rest[at+1:]
		}
		// Split host from path.
		slash := strings.Index(rest, "/")
		if slash < 0 {
			return "", fmt.Errorf("remote URL %q has no path", raw)
		}
		host = rest[:slash]
		path = rest[slash+1:]
	case sshShortRE.MatchString(raw):
		m := sshShortRE.FindStringSubmatch(raw)
		host = m[1]
		path = m[2]
	default:
		return "", fmt.Errorf("unrecognized remote URL format: %q", raw)
	}

	// Strip port from host (e.g. localhost:2222).
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	host = strings.ToLower(host)

	// Strip trailing slash and .git suffix from path.
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")

	if host == "" || path == "" {
		return "", fmt.Errorf("remote URL %q missing host or path after normalization", raw)
	}

	return host + "/" + path, nil
}
