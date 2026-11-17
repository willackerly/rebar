package cmd

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The utilities (internal/utils): refresh-context, agent-health,
// scan-consumers, rebar-doc, flush-notifications and the non-blocking
// paths of inbox-watch (runGolden), with goldens in
// testdata/<name> (regenerate with
// go test ./cmd -run 'RefreshContext|AgentHealth|ScanConsumers|RebarDoc|FlushNotifications|InboxWatch' -update-golden).
// The watcher itself is tested by TestInboxWatchSession and, in
// internal/utils, poll by poll.

// isoTimestamp is a UTC timestamp the scripts write, which differs between
// runs.
var isoTimestamp = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`)

// --- refresh-context ---------------------------------------------------

var (
	daysOld      = regexp.MustCompile(`QUICKCONTEXT is [0-9]+ days`)
	worktreeLine = regexp.MustCompile(`(?m)^/\S+( +[0-9a-f]{7,} \[[^]]*\])$`)
)

var commitLine = regexp.MustCompile(`(?m)^(<worktree> +)?[0-9a-f]{7,12}( )`)

var refreshContextCheck = goldenCheck{
	name:   "refresh-context",
	golden: "refresh-context",
	normalize: func(out string) string {
		// Days since a fixed date grow daily; git prints the work tree's
		// physical path.
		out = daysOld.ReplaceAllString(out, "QUICKCONTEXT is N days")
		out = worktreeLine.ReplaceAllString(out, "<worktree>$1")
		// The fixtures hold dates relative to today, so commit ids change
		// daily.
		out = commitLine.ReplaceAllString(out, "$1<sha>$2")
		return out
	},
}

// fakeTool writes an executable bin/<name> printing n numbered lines and
// exiting with code.
func fakeTool(name string, n, code int) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		testutil.WriteExecutable(t, root, "bin/"+name, "#!/bin/sh\ni=1\nwhile [ $i -le "+strconv.Itoa(n)+" ]; do echo \"line $i\"; i=$((i+1)); done\nprintf 'no newline'\nexit "+strconv.Itoa(code)+"\n")
	}
}

func both(fs ...func(t *testing.T, root string)) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		for _, f := range fs {
			f(t, root)
		}
	}
}

// extraWorktree adds a linked work tree outside root.
func extraWorktree(t *testing.T, root string) {
	testutil.Git(t, root, "worktree", "add", "-q", filepath.Join(t.TempDir(), "wt"), "-b", "side")
}

var refreshTree = map[string]string{
	"QUICKCONTEXT.md":            "# Q\n\nlast-synced: {{ago:2}}\n",
	"TODO.md":                    "last-synced: {{ago:9}}\n- [ ] one\n- [ ] two\n- [x] done\n",
	"src/a.go":                   "// @todo first\n// @tracked tracked\n",
	"src/b/c.ts":                 "// @todo nested\n",
	"lib/d.py":                   "# @todo python\n",
	"cmd/e.txt":                  "@todo wrong extension\n",
	"internal/node_modules/f.js": "// @todo node_modules\n",
	"packages/g.rs":              "// @todo rust\n",
	"other/h.go":                 "// @todo not scanned\n",
}

var refreshContextCases = []goldenCase{
	{name: "no QUICKCONTEXT", files: map[string]string{"README.md": "x\n"}},
	{name: "fresh, not a repository", files: refreshTree},
	{name: "--quick", files: refreshTree, args: []string{"--quick"}},
	{name: "old", files: withFiles(refreshTree, map[string]string{"QUICKCONTEXT.md": "last-synced: {{ago:5}}\n"})},
	{name: "stale", files: withFiles(refreshTree, map[string]string{"QUICKCONTEXT.md": "last-synced: {{ago:30}}\n"})},
	{name: "no last-synced date", files: withFiles(refreshTree, map[string]string{"QUICKCONTEXT.md": "# Q\nlast-synced: soon\n"})},
	{
		name: "no open items: grep -c || echo 0",
		files: map[string]string{
			"QUICKCONTEXT.md": "last-synced: {{ago:1}}\n",
			"TODO.md":         "- [x] a\n- [x] b\n",
		},
	},
	{
		name: "more done than open, more than ten TODOs",
		files: map[string]string{
			"QUICKCONTEXT.md": "last-synced: {{ago:0}}\n",
			"TODO.md":         "- [ ] a\n- [x] b\n- [x] c\n",
			"src/many.go":     strings.Repeat("// @todo again\n", 12),
		},
	},
	{
		name:  "commits since the last sync",
		files: withFiles(refreshTree, map[string]string{"QUICKCONTEXT.md": "last-synced: 2026-01-01\n"}),
		setup: commits("one", "two", "three"),
	},
	{
		name:  "many commits and a second work tree",
		files: withFiles(refreshTree, map[string]string{"QUICKCONTEXT.md": "last-synced: 2026-01-01\n"}),
		setup: both(commits(strings.Split(strings.Repeat("c,", 25), ",")[:25]...), extraWorktree),
	},
	{
		name:  "commits before the last sync",
		files: refreshTree,
		setup: commits("old"),
	},
	{
		name:       "a date that does not exist",
		files:      map[string]string{"QUICKCONTEXT.md": "last-synced: 2026-02-30\n"},
		setup:      commits("before"),
		args:       []string{"--quick"},
		divergesOn: "bsd", // BSD date rolls it over to March 2
		want:       "=== QUICKCONTEXT Freshness Check ===\n\nLast synced: 2026-02-30\n\nCommits since last sync:\nOK: No commits since last sync\n",
	},
	{
		name:  "test baseline: failing suite",
		files: withFiles(refreshTree, map[string]string{"package.json": "{}\n", "pnpm-lock.yaml": "x\n"}),
		setup: fakeTool("pnpm", 25, 3),
		env:   []string{"PATH=$ROOT/bin:" + os.Getenv("PATH")},
		args:  []string{"--test-baseline"},
	},
	{
		name:  "test baseline: passing suite, second argument",
		files: withFiles(refreshTree, map[string]string{"package.json": "{}\n", "pnpm-lock.yaml": "x\n"}),
		setup: fakeTool("pnpm", 2, 0),
		env:   []string{"PATH=$ROOT/bin:" + os.Getenv("PATH")},
		args:  []string{"--full", "--test-baseline"},
	},
	{name: "test baseline: no runner", files: refreshTree, args: []string{"--test-baseline"}},
}

func TestRefreshContextGolden(t *testing.T) {
	runGolden(t, refreshContextCheck, refreshContextCases)
}

// --- agent-health ------------------------------------------------------

// agentID names the /tmp files the agent-health cases write.
const agentID = "rebar-parity-agent"

func cleanAgentFiles(t *testing.T) {
	t.Cleanup(func() {
		for _, suffix := range []string{".heartbeat", ".metrics.jsonl"} {
			os.Remove("/tmp/agent-" + agentID + suffix)
		}
	})
}

var agentHealthCheck = goldenCheck{
	name:      "agent-health",
	golden:    "agent-health",
	writes:    true,
	normalize: func(out string) string { return isoTimestamp.ReplaceAllString(out, "<ts>") },
}

var agentEnv = []string{"AGENT_ID=" + agentID, "AGENT_PROGRESS_FILE=progress.jsonl"}

var agentHealthCases = []goldenCase{
	{name: "loaded", files: map[string]string{"x": "x\n"}, env: agentEnv},
	{name: "agent ID from the work tree", files: map[string]string{"x": "x\n"}, git: true, args: []string{"what"}},
	{name: "metric", files: map[string]string{"x": "x\n"}, env: agentEnv, args: []string{"metric", "rmse_delta", "-0.045"}},
	{name: "rmse", files: map[string]string{"x": "x\n"}, env: agentEnv, args: []string{"rmse", "doc.pdf", "0.5", "0.25"}},
	{name: "heartbeat", files: map[string]string{"x": "x\n"}, env: agentEnv, args: []string{"heartbeat"}},
	{name: "default progress file", files: map[string]string{"x": "x\n"}, env: []string{"AGENT_ID=" + agentID}, args: []string{"metric", "k", "1"}},
	{name: "checkpoint with nothing to commit", files: map[string]string{"x": "x\n"}, setup: commits("init"), env: agentEnv, args: []string{"checkpoint", "msg"}},
	{
		name: "checkpoint without a message", files: map[string]string{"x": "x\n"}, env: agentEnv, args: []string{"checkpoint"},
		divergesOn: "all", want: "agent-health: 1: Usage: agent_checkpoint \"commit message\"\n", wantExit: 1,
	},
	{
		name: "metric without a value", files: map[string]string{"x": "x\n"}, env: agentEnv, args: []string{"metric", "k"},
		divergesOn: "all", want: "agent-health: 2: Usage: agent_metric \"key\" \"value\"\n", wantExit: 1,
	},
	{
		name: "rmse without after", files: map[string]string{"x": "x\n"}, env: agentEnv, args: []string{"rmse", "d", "1"},
		divergesOn: "all", want: "agent-health: 3: parameter null or not set\n", wantExit: 1,
	},
}

func TestAgentHealthGolden(t *testing.T) {
	cleanAgentFiles(t)
	runGolden(t, agentHealthCheck, agentHealthCases)
}

// TestAgentHealthCheckpoint commits a change through the CLI in a fresh
// repository and compares output, progress file and status with the golden
// (captured from the script); and appends metrics to /tmp as the script
// did.
func TestAgentHealthCheckpoint(t *testing.T) {
	cleanAgentFiles(t)
	repo := func() string {
		root := writeTree(t, map[string]string{"a.txt": "one\n", "new.txt": "untracked\n"}, nil)
		os.Remove(filepath.Join(root, "new.txt"))
		commits("init")(t, root)
		testutil.WriteFile(t, root, "a.txt", "two\n")
		testutil.WriteFile(t, root, "untracked.txt", "never committed\n")
		return root
	}
	root := repo()
	env := append(testutil.IsolatedEnv(agentEnv...), "REBAR_PROJECT_ROOT="+root, "REBAR_TEST_RUN_CLI=1")
	out, _ := testutil.RunEnv(t, root, env, testBinary(t), "native", "agent-health", "checkpoint", "fix: the \"thing\"")
	progress, _ := os.ReadFile(filepath.Join(root, "progress.jsonl"))
	status := testutil.Git(t, root, "status", "--short")
	got := isoTimestamp.ReplaceAllString(out+"--- progress\n"+string(progress)+"--- status\n"+status, "<ts>")
	if !strings.Contains(got, "[agent-health] checkpoint: ") || !strings.Contains(got, "?? untracked.txt") {
		t.Errorf("no checkpoint:\n%s", got)
	}
	checkGoldenText(t, "agent-health", "checkpoint session", got)
	metrics, err := os.ReadFile("/tmp/agent-" + agentID + ".metrics.jsonl")
	if err == nil && strings.Count(string(metrics), "\n") > 0 && !strings.Contains(string(metrics), `"agent":"`+agentID+`"`) {
		t.Errorf("metrics file: %s", metrics)
	}
}

// --- scan-consumers and flush-notifications ----------------------------

// consumerRepos writes consumer repos below root/consumers whose
// CONSUMES.md name this repo (root's base name) as the owner.
func consumerRepos(t *testing.T, root string) {
	owner := filepath.Base(root)
	section := func(id, body string) string { return "## " + owner + "/" + id + ".1.0\n" + body }
	testutil.WriteFile(t, root, "consumers/alpha/CONSUMES.md", "# Consumes\n\n"+
		section("C1-API", "- **version_pinned:** 1.0 \n- **pin_date:** 2026-01-01\n- **notify_on_change:** true  # always\n- **rationale:** we call it\n")+
		"## other/C1-API.1.0\n- **version_pinned:** 9.9\n")
	testutil.WriteFile(t, root, "consumers/beta/CONSUMES.md", section("C1-API", "- **version_pinned:**   1 . 0\n- **notify_on_change:** false\n")+
		section("C2-LIB", "- **version_pinned:** 2.0\n"))
	testutil.WriteFile(t, root, "consumers/gamma/CONSUMES.md", section("C1-API", "- **pin_date:** never\n")+"## next\n- **version_pinned:** 0.1\n")
	testutil.WriteFile(t, root, "consumers/delta/README.md", "no CONSUMES.md\n")
	testutil.WriteFile(t, root, "consumers/broken/CONSUMES.md", section("C2-LIB", "- **notify_on_change:** true\n"))
	testutil.WriteFile(t, root, "registry", "me = /nowhere\n"+owner+" = /self\nalpha = "+root+"/consumers/alpha\n beta\t=\t"+root+"/consumers/beta\n\n")
}

var scanConsumersCheck = goldenCheck{name: "scan-consumers", golden: "scan-consumers"}

var consumersEnv = []string{"REBAR_REPOS=$ROOT/consumers/alpha,$ROOT/consumers/beta,$ROOT/consumers/gamma,$ROOT/consumers/delta,$ROOT/missing"}

var scanConsumersCases = []goldenCase{
	{name: "no arguments", files: map[string]string{"x": "x\n"}},
	{name: "two IDs", files: map[string]string{"x": "x\n"}, args: []string{"C1", "C2"}},
	{name: "--help", files: map[string]string{"x": "x\n"}, args: []string{"--json", "-h"}},
	{name: "nothing to scan", files: map[string]string{"x": "x\n"}, env: []string{"ASK_REGISTRY=$ROOT/none"}, args: []string{"C1-API"}},
	{name: "nothing to scan, --json", files: map[string]string{"x": "x\n"}, env: []string{"ASK_REGISTRY=$ROOT/none"}, args: []string{"--json", "C1-API"}},
	{name: "REBAR_REPOS", files: map[string]string{"x": "x\n"}, setup: consumerRepos, env: consumersEnv, args: []string{"C1-API"}},
	{name: "REBAR_REPOS, --json", files: map[string]string{"x": "x\n"}, setup: consumerRepos, env: consumersEnv, args: []string{"C1-API", "--json"}},
	{name: "no consumers of this contract", files: map[string]string{"x": "x\n"}, setup: consumerRepos, env: consumersEnv, args: []string{"C9-NONE"}},
	{name: "the ASK registry", files: map[string]string{"x": "x\n"}, setup: consumerRepos, env: []string{"ASK_REGISTRY=$ROOT/registry"}, args: []string{"C1-API"}},
	{name: "complete sections", files: map[string]string{"x": "x\n"}, setup: fullConsumers, env: consumersEnv, args: []string{"C1-API"}},
	{name: "complete sections, --json", files: map[string]string{"x": "x\n"}, setup: fullConsumers, env: consumersEnv, args: []string{"--json", "C1-API"}},
	{name: "a quote in a rationale, --json", files: map[string]string{"x": "x\n"}, setup: quotedConsumers, env: consumersEnv, args: []string{"--json", "C1-API"}},
	{name: "an ID that is a pattern", files: map[string]string{"x": "x\n"}, setup: consumerRepos, env: consumersEnv, args: []string{"C1-A.I"}},
}

func TestScanConsumersGolden(t *testing.T) {
	runGolden(t, scanConsumersCheck, scanConsumersCases)
}

// fullConsumers writes consumer repos whose sections have every field
// (scan-consumers ends at the first one that lacks one): alpha wants
// notices, beta does not, gamma leaves it open, broken's ask fails.
func fullConsumers(t *testing.T, root string) { consumersWith(t, root, "plain use") }

// quotedConsumers is fullConsumers with a quote in each rationale, which
// scan-consumers puts into its JSON unescaped.
func quotedConsumers(t *testing.T, root string) { consumersWith(t, root, `a "quoted" use`) }

func consumersWith(t *testing.T, root, rationale string) {
	owner := filepath.Base(root)
	entry := func(id, notify string) string {
		return "## " + owner + "/" + id + ".1.0\n- **version_pinned:** 1.0\n- **pin_date:** 2026-01-01\n- **rationale:** " + rationale + "\n- **notify_on_change:** " + notify + "\n\n"
	}
	testutil.WriteFile(t, root, "consumers/alpha/CONSUMES.md", entry("C1-API", "true")+entry("C2-LIB", "true # yes"))
	testutil.WriteFile(t, root, "consumers/beta/CONSUMES.md", entry("C1-API", "false"))
	testutil.WriteFile(t, root, "consumers/gamma/CONSUMES.md", entry("C1-API", "sometimes"))
	testutil.WriteFile(t, root, "consumers/broken/CONSUMES.md", entry("C2-LIB", "true"))
}

// fakeAsk is an ask command that logs its arguments to ask.log and fails
// for the consumer named broken.
func fakeAsk(t *testing.T, root string) {
	testutil.WriteExecutable(t, root, "bin/ask", "#!/bin/sh\nprintf '%s\\n---\\n' \"$*\" >> \""+root+"/ask.log\"\ncase \"$1\" in broken:*) exit 1;; esac\n")
}

var flushCheck = goldenCheck{name: "flush-notifications", golden: "flush-notifications", writes: true}

var outbox = "# Pending notifications\n\n" +
	"## C1-API: 1.0 → 2.0\n- **detected:** 2026-01-02T03:04:05Z\n- **status:** pending\n- **commit:** abc1234\n\n" +
	"## C2-LIB: 1.0 → 1.1\n- **status:** pending (retry)\n- **commit:** def5678\n\n" +
	"## C3-OLD: 1.0 → 1.0.1\n- **status:** sent\n"

var flushEnv = []string{
	"REBAR_REPOS=$ROOT/consumers/alpha,$ROOT/consumers/beta,$ROOT/consumers/gamma,$ROOT/consumers/broken",
	"ASK_BIN=$ROOT/bin/ask",
}

var flushFiles = map[string]string{"architecture/.state/pending-notifications.md": outbox}

var flushCases = []goldenCase{
	{name: "no outbox", files: map[string]string{"x": "x\n"}},
	{name: "--help", files: map[string]string{"x": "x\n"}, args: []string{"--help"}},
	{name: "unexpected argument", files: map[string]string{"x": "x\n"}, args: []string{"--now"}},
	{name: "invalid severity", files: flushFiles, args: []string{"--severity", "huge"}},
	{
		name: "--severity without a value", files: flushFiles, args: []string{"--severity"},
		divergesOn: "all", want: "flush-notifications: --severity needs a value\n", wantExit: 1,
	},
	{name: "nothing pending", files: map[string]string{"architecture/.state/pending-notifications.md": "## C3-OLD: 1.0 → 1.0.1\n- **status:** sent\n"}},
	{name: "dry run", files: flushFiles, setup: both(fullConsumers, fakeAsk), env: flushEnv, args: []string{"--dry-run"}},
	{name: "dispatch", files: flushFiles, setup: both(fullConsumers, fakeAsk), env: flushEnv},
	{name: "dispatch with a severity, unset preferences skipped", files: flushFiles, setup: both(fullConsumers, fakeAsk), env: append([]string{"REBAR_NOTIFY_DEFAULT=skip"}, flushEnv...), args: []string{"--severity", "doc-only"}},
	{name: "a consumer section without a rationale ends scan-consumers", files: flushFiles, setup: both(consumerRepos, fakeAsk), env: flushEnv},
	{name: "a quote in a rationale breaks the JSON", files: flushFiles, setup: both(quotedConsumers, fakeAsk), env: flushEnv},
	{name: "no consumers", files: flushFiles, setup: fakeAsk, env: []string{"REBAR_REPOS=$ROOT/none", "ASK_BIN=$ROOT/bin/ask"}},
	{
		name:  "REBAR_OUTBOX and contracts_dir",
		files: map[string]string{".rebarrc": "contracts_dir = specs\n", "specs/.state/pending-notifications.md": outbox, "elsewhere.md": "## C2-LIB: 1.0 → 1.1\n- **status:** pending\n"},
		setup: both(fullConsumers, fakeAsk), env: append([]string{"REBAR_OUTBOX=$ROOT/elsewhere.md"}, flushEnv...),
	},
	{
		name:  "contracts_dir default outbox",
		files: map[string]string{".rebarrc": "contracts_dir = specs\n", "specs/.state/pending-notifications.md": outbox},
		setup: both(fullConsumers, fakeAsk), env: flushEnv, args: []string{"--dry-run"},
	},
}

// utf8Locale sets LC_ALL to an installed UTF-8 locale, or skips the test.
func utf8Locale(t *testing.T) {
	t.Helper()
	out, _ := exec.Command("locale", "-a").Output()
	for _, want := range []string{"C.UTF-8", "C.utf8", "en_US.UTF-8", "en_US.utf8"} {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(l) == want {
				t.Setenv("LC_ALL", want)
				return
			}
		}
	}
	t.Skip("no UTF-8 locale installed")
}

func TestFlushNotificationsGolden(t *testing.T) {
	runGolden(t, flushCheck, flushCases)
}

// --- rebar-doc ---------------------------------------------------------

var rebarDocCheck = goldenCheck{name: "rebar-doc", golden: "rebar-doc"}

// docEnv points HOME into the tree and keeps the script's $PWD logical.
var docEnv = []string{"HOME=$ROOT/home", "PWD=$ROOT", "REBAR_ROOT="}

var rebarDocTree = map[string]string{
	"practices/inbox-watch.md":                        "# practice\n",
	"scripts/steward.sh":                              "#!/bin/sh\n",
	"agents/architect.md":                             "# agent\n",
	"conventions.md":                                  "# conventions\n",
	"CHARTER.md":                                      "# charter\n",
	"guide.md":                                        "# root guide\n",
	"docs/guide.md":                                   "# docs guide\n",
	"docs/only.md":                                    "# only in docs\n",
	"feedback/processed/old.md":                       "# processed\n",
	"rr/practices/elsewhere.md":                       "# from REBAR_ROOT\n",
	"home/dev/rebar/practices/co.md":                  "# from a checkout\n",
	"home/dev/rebar/scripts/steward.sh":               "#!/bin/sh\n",
	"home/dev/rebar/templates/project-bootstrap/x.md": "x\n",
	"home/.rebar/practices/co.md":                     "# not a source checkout\n",
}

var rebarDocCases = []goldenCase{
	{name: "missing ref", files: rebarDocTree, env: docEnv},
	{name: "--help", files: rebarDocTree, env: docEnv, args: []string{"x", "--help"}},
	{name: "unknown option", files: rebarDocTree, env: docEnv, args: []string{"--json"}},
	{name: "extra argument", files: rebarDocTree, env: docEnv, args: []string{"charter", "convention"}},
	{name: "practice", files: rebarDocTree, env: docEnv, args: []string{"rebar:practice/inbox-watch"}},
	{name: "script, agents, convention, charter", files: rebarDocTree, env: docEnv, args: []string{"script/steward"}},
	{name: "agents", files: rebarDocTree, env: docEnv, args: []string{"agents/architect"}},
	{name: "convention section", files: rebarDocTree, env: docEnv, args: []string{"rebar:convention/cross-repo-references"}},
	{name: "charter --cat", files: rebarDocTree, env: docEnv, args: []string{"--cat", "charter"}},
	{name: "doc at the root first", files: rebarDocTree, env: docEnv, args: []string{"doc/guide"}},
	{name: "doc in docs", files: rebarDocTree, env: docEnv, args: []string{"doc/only", "--cat"}},
	{name: "processed feedback", files: rebarDocTree, env: docEnv, args: []string{"feedback/old"}},
	{name: "REBAR_ROOT", files: rebarDocTree, env: []string{"HOME=$ROOT/home", "PWD=$ROOT", "REBAR_ROOT=$ROOT/rr"}, args: []string{"practice/elsewhere"}},
	{name: "a checkout", files: rebarDocTree, env: docEnv, args: []string{"practice/co"}},
	{name: "upstream", files: rebarDocTree, env: docEnv, args: []string{"doc/nowhere"}},
	{name: "no name", files: rebarDocTree, env: docEnv, args: []string{"practice"}},
	{name: "a slash in the name", files: rebarDocTree, env: docEnv, args: []string{"practice/a/b"}},
	{name: "whitespace in the name", files: rebarDocTree, env: docEnv, args: []string{"doc/a b"}},
	{name: "dots in the name", files: rebarDocTree, env: docEnv, args: []string{"feedback/a..b"}},
	{name: "charter with a name", files: rebarDocTree, env: docEnv, args: []string{"charter/x"}},
	{name: "unknown kind", files: rebarDocTree, env: docEnv, args: []string{"rebar:guide/x"}},
}

func TestRebarDocGolden(t *testing.T) {
	runGolden(t, rebarDocCheck, rebarDocCases)
}

// --- inbox-watch -------------------------------------------------------

var inboxWatchCheck = goldenCheck{name: "inbox-watch", golden: "inbox-watch"}

var inboxWatchCases = []goldenCase{
	{name: "--help", files: map[string]string{"x": "x\n"}, args: []string{"-i", "5", "--help"}},
	{name: "interval without a value", files: map[string]string{"x": "x\n"}, args: []string{"--interval"}},
	{name: "interval zero", files: map[string]string{"x": "x\n"}, args: []string{"--interval=00"}},
	{name: "interval not a number", files: map[string]string{"x": "x\n"}, args: []string{"-i", "1s"}},
	{name: "--self without a value", files: map[string]string{"x": "x\n"}, args: []string{"inbox", "--self"}},
	{name: "unknown option", files: map[string]string{"x": "x\n"}, args: []string{"-"}},
}

func TestInboxWatchGolden(t *testing.T) {
	runGolden(t, inboxWatchCheck, inboxWatchCases)
}

// TestInboxWatchSession runs the watcher for real, through the CLI: it
// waits for the armed line, adds deposits, waits for them to be reported,
// stops the watcher with SIGTERM and compares what it printed with the
// golden (captured from the script; exit 0, lock released). It waits on
// the watcher's output, polling every second, not on fixed sleeps.
func TestInboxWatchSession(t *testing.T) {
	session := func(how string) string {
		root := physical(t, writeTree(t, map[string]string{
			"inbox/2026-07-11-peer-old.md": "old\n",
		}, nil))
		env := testutil.IsolatedEnv("REBAR_PROJECT_ROOT="+root, "PWD="+root, "REBAR_TEST_RUN_CLI=1")
		cmd := exec.Command(testBinary(t), "native", "inbox-watch", "-i", "1", "--preview", "inbox", "missing")
		cmd.Dir, cmd.Env = root, env
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		lines := make(chan string, 64)
		var errLines []string
		armed := make(chan bool, 1)
		errDone := make(chan bool)
		go func() {
			sc := bufio.NewScanner(stderr)
			for sc.Scan() {
				errLines = append(errLines, sc.Text())
				if strings.Contains(sc.Text(), "armed") {
					armed <- true
				}
			}
			close(errDone)
		}()
		go func() {
			sc := bufio.NewScanner(stdout)
			for sc.Scan() {
				lines <- sc.Text()
			}
			close(lines)
		}()
		wait := func(what string, ch any) {
			select {
			case <-time.After(20 * time.Second):
				cmd.Process.Kill()
				t.Fatalf("%s: timed out waiting for %s", how, what)
			case <-ch.(chan bool):
			}
		}
		wait("the armed line", armed)
		lock, _ := os.ReadFile(filepath.Join(root, "inbox", ".inbox-watch.lock"))
		if strings.TrimSpace(string(lock)) != strconv.Itoa(cmd.Process.Pid) {
			t.Errorf("%s: lock holds %q, want the watcher's PID %d", how, lock, cmd.Process.Pid)
		}
		testutil.WriteFile(t, root, "inbox/2026-07-12-peer-ASK-new.md", "Subject: hello\nbody\n")
		testutil.WriteFile(t, root, "inbox/2026-07-12-"+filepath.Base(root)+"-REPLY-mine.md", "mine\n")
		testutil.WriteFile(t, root, "missing/late.md", "")
		var got []string
		deadline := time.After(20 * time.Second)
		for len(got) < 2 {
			select {
			case l := <-lines:
				got = append(got, l)
			case <-deadline:
				cmd.Process.Kill()
				t.Fatalf("%s: timed out; got %q", how, got)
			}
		}
		cmd.Process.Signal(syscall.SIGTERM)
		for l := range lines {
			got = append(got, l)
		}
		<-errDone
		err := cmd.Wait()
		if err != nil {
			t.Errorf("%s: exit after SIGTERM: %v", how, err)
		}
		if _, err := os.Stat(filepath.Join(root, "inbox", ".inbox-watch.lock")); !os.IsNotExist(err) {
			t.Errorf("%s: lock not released", how)
		}
		out := strings.Join(errLines, "\n") + "\n" + strings.Join(got, "\n")
		out = strings.ReplaceAll(out, "suppressed: "+filepath.Base(root), "suppressed: <root's name>")
		return strings.ReplaceAll(out, root, "$ROOT")
	}
	got := session("native")
	if !strings.Contains(got, "NEW INBOX DEPOSIT: inbox/2026-07-12-peer-ASK-new.md — Subject: hello") || !strings.Contains(got, "NEW INBOX DEPOSIT: missing/late.md") {
		t.Fatalf("session:\n%s", got)
	}
	checkGoldenText(t, "inbox-watch", "session", got)
}
