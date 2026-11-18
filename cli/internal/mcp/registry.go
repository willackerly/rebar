package mcp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/willackerly/rebar/cli/internal/ask"
)

// Registry is the repos a server hosts (RepoRegistry of the python
// servers): name → path of each directory with an agents/ subdirectory,
// and its agents, the subdirectories of agents/ holding an AGENT.md.
type Registry struct {
	mu    sync.Mutex
	repos map[string]string
	// Prog prefixes warnings ("ask-mcp-server", "ask-server").
	Prog string
	// Log gets the "registered:" lines (stderr for the MCP server, stdout
	// for ask-server); Warn gets warnings.
	Log, Warn io.Writer
	// SkipMarker honours .rebar-skip-mcp files: a repo with one is not
	// served (both servers; ask-server ignored it before rebar 4.0).
	SkipMarker bool
}

// NewRegistry returns an empty registry.
func NewRegistry(prog string, log, warn io.Writer, skipMarker bool) *Registry {
	return &Registry{repos: map[string]string{}, Prog: prog, Log: log, Warn: warn, SkipMarker: skipMarker}
}

// scanSkip are directory names discovery never descends into.
var scanSkip = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true,
	"out": true, ".next": true, ".nuxt": true, "coverage": true, "__pycache__": true,
}

// resolve is Path(p).resolve(): absolute, symlinks resolved as far as
// they exist.
func resolve(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// ScanDir registers the repos below dir: its subdirectories with agents/,
// and (depth 2) those one level further down inside subdirectories
// without.
func (r *Registry) ScanDir(dir string) { r.scan(dir, 2) }

func (r *Registry) scan(dir string, depth int) {
	dir = resolve(dir)
	if !isDir(dir) {
		fmt.Fprintf(r.Warn, "%s: warning: repos-dir not found: %s\n", r.Prog, dir)
		return
	}
	if depth <= 0 {
		return
	}
	entries, _ := os.ReadDir(dir)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		entry := filepath.Join(dir, e.Name())
		if !isDir(entry) || strings.HasPrefix(e.Name(), ".") || scanSkip[e.Name()] {
			continue
		}
		if r.skipped(entry, e.Name()) {
			continue
		}
		if isDir(filepath.Join(entry, "agents")) {
			r.AddRepo(e.Name(), entry)
		} else if depth > 1 {
			r.scan(entry, depth-1)
		}
	}
}

// skipped reports (and logs) a repo opted out of MCP with .rebar-skip-mcp,
// when the registry honours the marker.
func (r *Registry) skipped(dir, shown string) bool {
	if !r.SkipMarker {
		return false
	}
	if _, err := os.Lstat(filepath.Join(dir, ".rebar-skip-mcp")); err != nil {
		return false
	}
	fmt.Fprintf(r.Warn, "  skipped: %s (.rebar-skip-mcp present)\n", shown)
	return true
}

// AddRepos registers explicit repo paths (a repo with .rebar-skip-mcp is
// skipped here too).
func (r *Registry) AddRepos(paths []string) {
	for _, p := range paths {
		p = resolve(p)
		if r.skipped(p, p) {
			continue
		}
		if isDir(p) && isDir(filepath.Join(p, "agents")) {
			r.AddRepo(filepath.Base(p), p)
		} else {
			fmt.Fprintf(r.Warn, "%s: warning: skipping %s (no agents/ dir)\n", r.Prog, p)
		}
	}
}

// AddRepo registers one repo.
func (r *Registry) AddRepo(name, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repos[name] = path
	agents := agentsFor(path)
	fmt.Fprintf(r.Log, "  registered: %s → %s (%d agents: %s)\n", name, path, len(agents), strings.Join(agents, ", "))
}

// Len is the number of repos.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.repos)
}

func (r *Registry) names() []string {
	names := make([]string, 0, len(r.repos))
	for n := range r.repos {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Agent is one (repo, role).
type Agent struct{ Repo, Role string }

// ID is "repo:role".
func (a Agent) ID() string { return a.Repo + ":" + a.Role }

// Agents lists every agent, by repo then role.
func (r *Registry) Agents() []Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Agent
	for _, name := range r.names() {
		for _, role := range agentsFor(r.repos[name]) {
			out = append(out, Agent{name, role})
		}
	}
	return out
}

// agentsJSON is list_all_agents as JSON.
func (r *Registry) agentsJSON() []any {
	out := []any{}
	for _, a := range r.Agents() {
		out = append(out, object{{"repo", a.Repo}, {"role", a.Role}, {"id", a.ID()}})
	}
	return out
}

// reposJSON is list_repos as JSON.
func (r *Registry) reposJSON() []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []any{}
	for _, name := range r.names() {
		out = append(out, object{{"repo", name}, {"path", r.repos[name]}, {"agents", agentsFor(r.repos[name])}})
	}
	return out
}

// Path is the path of repo ("" when unknown, or not a plain name).
func (r *Registry) Path(repo string) string {
	if !ask.ValidAgentName(repo) {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.repos[repo]
}

// ResolveRepo is the registered name matching repo, ignoring case.
func (r *Registry) ResolveRepo(repo string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.repos[repo]; ok {
		return repo
	}
	lc := strings.ToLower(repo)
	for _, name := range r.insertionOrder() {
		if strings.ToLower(name) == lc {
			return name
		}
	}
	return ""
}

// insertionOrder approximates the python dict's iteration order for the
// case-insensitive lookups; with distinct lower-case names it does not
// matter.
func (r *Registry) insertionOrder() []string { return r.names() }

// ResolveRole is the agent of repo matching role, ignoring case.
func (r *Registry) ResolveRole(repo, role string) string {
	path := r.Path(repo)
	if path == "" {
		return ""
	}
	roles := agentsFor(path)
	for _, x := range roles {
		if x == role {
			return x
		}
	}
	lc := strings.ToLower(role)
	for _, x := range roles {
		if strings.ToLower(x) == lc {
			return x
		}
	}
	return ""
}

// HasAgent reports whether role is one of repo's agents: exactly a name
// agentsFor lists, so a role is never a path ("../../other/agents/x").
func (r *Registry) HasAgent(repo, role string) bool {
	path := r.Path(repo)
	if path == "" || !ask.ValidAgentName(role) {
		return false
	}
	for _, x := range agentsFor(path) {
		if x == role {
			return true
		}
	}
	return false
}

// agentsFor lists the agents of the repo at path.
func agentsFor(path string) []string {
	dir := filepath.Join(path, "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	agents := []string{}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if ask.ValidAgentName(e.Name()) && isDir(p) && isFile(filepath.Join(p, "AGENT.md")) {
			agents = append(agents, e.Name())
		}
	}
	return agents
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
