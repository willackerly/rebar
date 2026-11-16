package utils

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestTail(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"", 3, ""},
		{"a\nb\nc\n", 2, "b\nc\n"},
		{"a\nb\nc\n", 5, "a\nb\nc\n"},
		{"a\nb\nc", 2, "b\nc"},
		{"a\n\n\n", 2, "\n\n"},
	}
	for _, tc := range cases {
		if got := tail(tc.in, tc.n); got != tc.want {
			t.Errorf("tail(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestGrepCountAndLastSynced(t *testing.T) {
	todo := []byte("- [ ] a\n- [x] b\n  - [ ] nested\n- [ ] c")
	if got := grepCount(todo, "- [ ]"); got != "2" {
		t.Errorf("open = %q", got)
	}
	if got := grepCount(todo, "- [X]"); got != "0\n0" {
		t.Errorf("no match = %q, want the script's 0<newline>0", got)
	}
	if got := lastSynced([]byte("x last-synced: 2026-01-02 and last-synced: 2026-03-04\nlast-synced: 26-01-01\n")); got != "2026-01-02\n2026-03-04" {
		t.Errorf("lastSynced = %q", got)
	}
	if _, ok := syncDay("2026-02-30"); ok {
		t.Error("2026-02-30 accepted")
	}
	if _, ok := syncDay("2026-01-02\n2026-03-04"); ok {
		t.Error("two dates accepted")
	}
}

func TestIFSAndPaths(t *testing.T) {
	for in, want := range map[string][2]string{
		"x=y=": {"x", "y"}, "x=y==": {"x", "y=="}, "x=": {"x", ""}, "x==": {"x", ""}, "x=a=b=": {"x", "a=b="}, "x": {"x", ""},
	} {
		if a, b := ifsEquals(in); a != want[0] || b != want[1] {
			t.Errorf("ifsEquals(%q) = %q %q, want %q", in, a, b, want)
		}
	}
	for in, want := range map[string][]string{
		"a,,b,": {"a", "", "b"}, ",": {""}, ",a": {"", "a"}, "a,b,,": {"a", "b", ""}, "a\nb,c": {"a"}, "\nx": nil,
	} {
		t.Setenv("REBAR_REPOS", in)
		if got := scanPaths("me"); strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("REBAR_REPOS=%q: %q, want %q", in, got, want)
		}
	}
	t.Setenv("REBAR_REPOS", "")
	reg := testutil.WriteFile(t, t.TempDir(), "projects", "me = /p/me\n other\t= \t/p/other\n\n=/p/none\nlast=/p/last")
	t.Setenv("ASK_REGISTRY", reg)
	if got := scanPaths("me"); strings.Join(got, "|") != "/p/other" {
		t.Errorf("registry paths = %q (the owner, a nameless entry and an unterminated last line are skipped)", got)
	}
}

func TestSenderAndSeverity(t *testing.T) {
	for in, want := range map[string]string{
		"2026-07-11-rebar-ANSWER-x.md": "rebar", "notes-x.md": "notes", "2026-07-11-solo.md": "solo.md", "plain": "plain",
	} {
		if got := sender(in); got != want {
			t.Errorf("sender(%q) = %q, want %q", in, got, want)
		}
	}
	for _, tc := range [][3]string{{"1.0", "2.0", "breaking"}, {"1.0", "1.1", "additive"}, {"1.0.1", "1.0.2", "patch"}, {"1.2", "1.2.0", "patch"}} {
		if got := inferSeverity(tc[0], tc[1]); got != tc[2] {
			t.Errorf("inferSeverity(%s, %s) = %s, want %s", tc[0], tc[1], got, tc[2])
		}
	}
}

func TestPendingEntries(t *testing.T) {
	outbox := "## C1-A: 1.0 → 2.0\n- **status:** pending\n- **commit:** abc123\n\n" +
		"## C2-B: 1.0 → 1.1\n- **status:** sent\n" +
		"## lower: 1.0 → 2.0\n- **status:** pending\n" +
		"## C3-C: 1.0.0 → 1.0.1  \n- **commit:** def\n- **status:** pendingly\n"
	got := pendingEntries([]byte(outbox))
	// A header the pattern rejects (lower) does not start an entry: its
	// status lands on the entry before it, so C2-B counts as pending; the
	// status pattern has no end anchor, so "pendingly" is pending.
	want := []pendingNotification{{"C1-A", "1.0", "2.0", "abc123"}, {"C2-B", "1.0", "1.1", ""}, {"C3-C", "1.0.0", "1.0.1", "def"}}
	if len(got) != len(want) {
		t.Fatalf("pendingEntries = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// watchFixture is a watcher armed on dirs below a fresh working directory.
func watchFixture(t *testing.T, args ...string) (*inboxWatch, *strings.Builder, *strings.Builder, string) {
	t.Helper()
	root := t.TempDir()
	var out, errw strings.Builder
	ctx := &checks.Context{Root: filepath.Join(root, "myrepo"), Dir: root, Stdout: &out, Stderr: &errw}
	w, code, done := parseInboxWatch(ctx, args)
	if done {
		t.Fatalf("parse %v: exit %d\n%s", args, code, errw.String())
	}
	return w, &out, &errw, root
}

func touch(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestInboxWatchPolls drives the watcher's polls by hand: pre-existing
// files are old news, each new name is reported once for the life of the
// watcher (a git operation that removes and restores files re-reports
// nothing), own-sender memos are suppressed but remembered, and dotfiles
// (the lock) never show.
func TestInboxWatchPolls(t *testing.T) {
	w, out, errw, root := watchFixture(t, "--self", "me", "inbox")
	inbox := filepath.Join(root, "inbox")
	touch(t, filepath.Join(inbox, "2026-07-11-ACK-old.md"), "old\n")
	w.arm()
	defer w.release()
	if lock := filepath.Join(inbox, inboxLockName); lockHolder(lock) != strconv.Itoa(os.Getpid()) {
		t.Errorf("lock holds %q", lockHolder(lock))
	}
	if !strings.Contains(errw.String(), "inbox-watch: armed — watching inbox every 30s (pre-existing files not reported; own-sender deposits suppressed: me)") {
		t.Errorf("arm output:\n%s", errw.String())
	}
	w.poll()
	if out.String() != "" {
		t.Fatalf("baseline poll reported: %q", out.String())
	}
	touch(t, filepath.Join(inbox, "2026-07-11-Zed-new.md"), "first\n")
	touch(t, filepath.Join(inbox, "2026-07-11-me-REPLY-x.md"), "mine\n")
	touch(t, filepath.Join(inbox, "2026-07-11-alpha-new.md"), "")
	w.poll()
	want := "NEW INBOX DEPOSIT: 2026-07-11-Zed-new.md\nNEW INBOX DEPOSIT: 2026-07-11-alpha-new.md\n"
	if out.String() != want {
		t.Errorf("deposits:\n%s\nwant:\n%s", out.String(), want)
	}
	// Transient removal and restore (a merge or a checkout).
	moved := filepath.Join(root, "moved")
	if err := os.Rename(inbox, moved); err != nil {
		t.Fatal(err)
	}
	w.poll()
	if err := os.Rename(moved, inbox); err != nil {
		t.Fatal(err)
	}
	w.poll()
	w.self = ""
	w.poll()
	if out.String() != want {
		t.Errorf("re-reported after a transient removal or a --self change:\n%s", out.String())
	}
}

// TestInboxWatchMultiAndPreview: several directories prefix the path; a
// directory that does not exist yet reports what it holds once created;
// --preview adds the first line.
func TestInboxWatchMultiAndPreview(t *testing.T) {
	w, out, errw, root := watchFixture(t, "--preview", "--self=", "a/", "b")
	touch(t, filepath.Join(root, "a", "x.md"), "old\n")
	w.arm()
	defer w.release()
	for _, s := range []string{"WARN — b does not exist yet", "watching a b every 30s", "own-sender suppression OFF"} {
		if !strings.Contains(errw.String(), s) {
			t.Errorf("arm output lacks %q:\n%s", s, errw.String())
		}
	}
	touch(t, filepath.Join(root, "b", "y.md"), "Subject line\r\nbody\n")
	touch(t, filepath.Join(root, "b", "z.md"), "\nsecond line\n")
	if err := os.Mkdir(filepath.Join(root, "a", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.poll()
	want := "NEW INBOX DEPOSIT: a/sub\nNEW INBOX DEPOSIT: b/y.md — Subject line\r\nNEW INBOX DEPOSIT: b/z.md\n"
	if out.String() != want {
		t.Errorf("deposits:\n%q\nwant:\n%q", out.String(), want)
	}
}

// TestInboxWatchLocks: a live holder's lock is left alone, with a warning;
// a stale one is reclaimed silently; release removes only locks still held.
func TestInboxWatchLocks(t *testing.T) {
	w, _, errw, root := watchFixture(t, "live", "stale")
	live, stale := filepath.Join(root, "live"), filepath.Join(root, "stale")
	me := strconv.Itoa(os.Getpid()) // a live process this test may signal
	touch(t, filepath.Join(live, inboxLockName), me+"\n")
	touch(t, filepath.Join(stale, inboxLockName), "x999999999\n")
	w.pid = 4242424
	w.arm()
	if !strings.Contains(errw.String(), "a live watcher (PID "+me+") already holds live") {
		t.Errorf("no live-holder warning:\n%s", errw.String())
	}
	if strings.Contains(errw.String(), "stale") && strings.Contains(errw.String(), "already holds stale") {
		t.Errorf("warned about a stale lock:\n%s", errw.String())
	}
	if got := lockHolder(filepath.Join(stale, inboxLockName)); got != "4242424" {
		t.Errorf("stale lock not reclaimed: %q", got)
	}
	w.release()
	if _, err := os.Stat(filepath.Join(stale, inboxLockName)); !os.IsNotExist(err) {
		t.Errorf("own lock not released: %v", err)
	}
	if got := lockHolder(filepath.Join(live, inboxLockName)); got != me {
		t.Errorf("live holder's lock touched: %q", got)
	}
}

// TestInboxWatchLoop runs the watcher's loop with the timer and the signal
// replaced: each tick polls, a signal ends it with exit 0 and releases the
// lock.
func TestInboxWatchLoop(t *testing.T) {
	ticks := make(chan time.Time)
	sig := make(chan os.Signal)
	oldWait, oldSignals := inboxWait, inboxSignals
	inboxWait = func(d time.Duration) <-chan time.Time {
		if d != 7*time.Second {
			t.Errorf("waits %v, want 7s", d)
		}
		return ticks
	}
	inboxSignals = func() (<-chan os.Signal, func()) { return sig, func() {} }
	defer func() { inboxWait, inboxSignals = oldWait, oldSignals }()

	root := t.TempDir()
	inbox := filepath.Join(root, "inbox")
	touch(t, filepath.Join(inbox, "old.md"), "")
	var out, errw strings.Builder
	ctx := &checks.Context{Root: root, Dir: root, Stdout: &out, Stderr: &errw}
	done := make(chan int)
	go func() { done <- runInboxWatch(ctx, []string{"-i", "7"}) }()
	ticks <- time.Now() // the first wait is entered only after arming
	touch(t, filepath.Join(inbox, "new.md"), "")
	ticks <- time.Now()
	ticks <- time.Now() // the second poll has finished once the third wait starts
	sig <- os.Interrupt
	if code := <-done; code != 0 {
		t.Errorf("exit %d", code)
	}
	if out.String() != "NEW INBOX DEPOSIT: new.md\n" {
		t.Errorf("output %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(inbox, inboxLockName)); !os.IsNotExist(err) {
		t.Errorf("lock left behind: %v", err)
	}
}

func TestInboxWatchUsage(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-i"}, "inbox-watch: --interval needs a value\n"},
		{[]string{"--interval=0"}, "inbox-watch: interval must be >= 1 second, got '0'\n"},
		{[]string{"-i", "1.5"}, "inbox-watch: interval must be a positive integer, got '1.5'\n"},
		{[]string{"--interval="}, "inbox-watch: interval must be a positive integer, got ''\n"},
		{[]string{"--self"}, "inbox-watch: --self needs a value (use --self '' to disable)\n"},
		{[]string{"-x"}, "inbox-watch: unknown option '-x' (try --help)\n"},
	} {
		var out, errw strings.Builder
		_, code, done := parseInboxWatch(&checks.Context{Root: "/r", Dir: "/", Stdout: &out, Stderr: &errw}, tc.args)
		if !done || code != 2 || errw.String() != tc.want {
			t.Errorf("%v: done=%v exit %d %q, want %q", tc.args, done, code, errw.String(), tc.want)
		}
	}
	w, _, _ := parseInboxWatch(&checks.Context{Root: "/r/proj/", Dir: "/"}, []string{"--", "-odd/", "x//"})
	if strings.Join(w.dirs, "|") != "-odd|x/" || w.self != "proj" {
		t.Errorf("dirs %q self %q", w.dirs, w.self)
	}
}
