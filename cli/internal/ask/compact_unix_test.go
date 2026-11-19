//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package ask

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// compactTree is a project whose agent x has a memory.md and a log of n
// entries, and a stub claude (body) first on PATH. It returns the project.
func compactTree(t *testing.T, n int, claude string) string {
	t.Helper()
	proj := t.TempDir()
	testutil.WriteFile(t, proj, "agents/x/AGENT.md", "# X\n")
	testutil.WriteFile(t, proj, "agents/x/memory.md", "# Agent Memory: x\n\n<!-- Compacted at then. -->\n\nOld fact: the API is REST.\n")
	var log strings.Builder
	for i := 1; i <= n; i++ {
		log.WriteString("---\n**T** Q: q" + strconv.Itoa(i) + "\nA: a" + strconv.Itoa(i) + "\n")
	}
	testutil.WriteFile(t, proj, "agents/x/memory.log.md", log.String())
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "claude", claude)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ASK_COMPACT_ENTRIES", "4")
	t.Setenv("ASK_COMPACT_KEEP", "2")
	t.Setenv("HOME", t.TempDir())
	t.Setenv(noRegisterEnv, "1")
	return proj
}

func compactRun(t *testing.T, proj string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code := RunIn(proj, []string{"compact", "x"}, &out, &out)
	return out.String(), code
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestCompactKeepsAppendsAndOldMemory: an entry appended while claude
// summarizes survives, the old memory.md goes into the summary prompt, and
// the files are replaced without leftovers.
func TestCompactKeepsAppendsAndOldMemory(t *testing.T) {
	proj := compactTree(t, 6, `#!/bin/sh
cat > "$PWD/prompt.txt"
printf -- '---\n**T** Q: during\nA: compaction\n' >> agents/x/memory.log.md
echo "- summary"
`)
	out, code := compactRun(t, proj)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	logText := readFile(t, filepath.Join(proj, "agents/x/memory.log.md"))
	for _, want := range []string{"Q: q5", "Q: q6", "Q: during"} {
		if !strings.Contains(logText, want) {
			t.Errorf("log lacks %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "Q: q1\n") {
		t.Errorf("log kept a compacted entry:\n%s", logText)
	}
	if mem := readFile(t, filepath.Join(proj, "agents/x/memory.md")); !strings.Contains(mem, "- summary") {
		t.Errorf("memory.md:\n%s", mem)
	}
	if prompt := readFile(t, filepath.Join(proj, "prompt.txt")); !strings.Contains(prompt, "CURRENT MEMORY") || !strings.Contains(prompt, "Old fact: the API is REST.") {
		t.Errorf("the prompt lacks the old memory:\n%s", prompt)
	}
	left, _ := filepath.Glob(filepath.Join(proj, "agents/x/.*.tmp"))
	if len(left) > 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

// TestCompactFallbackKeepsOldMemory: without a summary the old memory is
// kept below the truncated entries.
func TestCompactFallbackKeepsOldMemory(t *testing.T) {
	proj := compactTree(t, 6, "#!/bin/sh\ncat >/dev/null\n")
	if out, code := compactRun(t, proj); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	mem := readFile(t, filepath.Join(proj, "agents/x/memory.md"))
	if !strings.Contains(mem, "older entries truncated") || !strings.Contains(mem, "Old fact: the API is REST.") {
		t.Errorf("memory.md:\n%s", mem)
	}
}

// TestCompactTimeout: a claude that never answers is cut off after
// ASK_TIMEOUT and the compaction falls back to truncation.
func TestCompactTimeout(t *testing.T) {
	proj := compactTree(t, 6, "#!/bin/sh\nexec sleep 60\n")
	t.Setenv("ASK_TIMEOUT", "1")
	start := time.Now()
	out, code := compactRun(t, proj)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("compaction took %v with ASK_TIMEOUT=1", d)
	}
	if mem := readFile(t, filepath.Join(proj, "agents/x/memory.md")); !strings.Contains(mem, "older entries truncated") {
		t.Errorf("memory.md:\n%s", mem)
	}
}

// TestCompactOneAtATime: a second compaction of the same agent returns at
// once and touches nothing.
func TestCompactOneAtATime(t *testing.T) {
	proj := compactTree(t, 6, "#!/bin/sh\necho summary\n")
	release, ok := lockFile(filepath.Join(proj, "agents/x/AGENT.md"), false)
	if !ok {
		t.Fatal("could not take the compaction lock")
	}
	defer release()
	before := readFile(t, filepath.Join(proj, "agents/x/memory.log.md"))
	out, code := compactRun(t, proj)
	if code != 0 || !strings.Contains(out, "a compaction is already running") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if after := readFile(t, filepath.Join(proj, "agents/x/memory.log.md")); after != before {
		t.Error("the log changed")
	}
}

// TestCompactLogReplaced: a log reset while claude ran is left alone.
func TestCompactLogReplaced(t *testing.T) {
	proj := compactTree(t, 6, "#!/bin/sh\necho fresh > agents/x/memory.log.md\necho summary\n")
	out, code := compactRun(t, proj)
	if code != 0 || !strings.Contains(out, "memory log changed during compaction") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if got := readFile(t, filepath.Join(proj, "agents/x/memory.log.md")); got != "fresh\n" {
		t.Errorf("log = %q", got)
	}
	if mem := readFile(t, filepath.Join(proj, "agents/x/memory.md")); !strings.Contains(mem, "Old fact") || strings.Contains(mem, "summary") {
		t.Errorf("memory.md changed:\n%s", mem)
	}
}

// TestMaybeCompactStaysInGroup: the background compaction runs in the
// asking process's group (whoever ends the ask's group ends it too), with
// the agent's directories and auto-registration off.
func TestMaybeCompactStaysInGroup(t *testing.T) {
	proj := compactTree(t, 6, "#!/bin/sh\n")
	t.Setenv("ASK_COMPACT_SIZE", "10")
	out := filepath.Join(t.TempDir(), "compact.out")
	exe := testutil.WriteExecutable(t, t.TempDir(), "rebar", "#!/bin/sh\n{ echo \"$*\"; ps -o pgid= -p $$ | tr -d ' '; echo \"$ASK_AGENTS_DIR $"+noRegisterEnv+"\"; } > '"+out+".tmp'\nmv '"+out+".tmp' '"+out+"'\n")
	a := New(nil, &bytes.Buffer{}, &bytes.Buffer{})
	a.Exe = exe
	a.agentsDir = filepath.Join(proj, "agents")
	a.maybeCompact("x")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if data, err := os.ReadFile(out); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 3 || lines[0] != "ask compact x" || lines[2] != a.agentsDir+" 1" {
				t.Errorf("compaction ran as:\n%s", data)
			}
			if pgid, _ := strconv.Atoi(lines[1]); pgid != syscall.Getpgrp() {
				t.Errorf("compaction in group %d, the asking process in %d", pgid, syscall.Getpgrp())
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the background compaction did not run")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
