package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/willackerly/rebar/cli/internal/checks"
)

// inboxLockName is the per-inbox lock: a dotfile, so a listing never
// shows it.
const inboxLockName = ".inbox-watch.lock"

// inboxWatch is one scripts/inbox-watch.sh watcher: arm (Principle 5
// checks, per-inbox locks, the seen-ledger seeded with what is there), then
// poll every interval until SIGINT or SIGTERM, which exit 0 after releasing
// the locks. The script slept in the background and waited, so a signal
// lands at once; the port waits on a timer and the signal together.
type inboxWatch struct {
	ctx      *checks.Context
	dirs     []string
	interval string
	every    time.Duration
	preview  bool
	self     string
	pid      int
	locks    []string
	seen     []map[string]bool
}

// inboxWait is how the watcher waits for its next poll: a channel that
// fires after d (tests replace it to poll on demand).
var inboxWait = func(d time.Duration) <-chan time.Time { return time.After(d) }

// inboxSignals is the channel SIGINT and SIGTERM arrive on (tests replace
// it to stop a watcher).
var inboxSignals = func() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return ch, func() { signal.Stop(ch) }
}

// runInboxWatch is scripts/inbox-watch.sh: watch the inbox directories
// (default ./inbox) and print "NEW INBOX DEPOSIT: <name>" (with several
// directories, <dir>/<name>) for each file that appears after arming,
// except the repo's own outbound memos (--self, default the project
// directory's name); --preview adds the memo's first line.
func runInboxWatch(ctx *checks.Context, args []string) int {
	w, code, done := parseInboxWatch(ctx, args)
	if done {
		return code
	}
	w.arm()
	defer w.release()
	sig, stop := inboxSignals()
	defer stop()
	for {
		select {
		case <-sig:
			return 0
		case <-inboxWait(w.every):
			w.poll()
		}
	}
}

// parseInboxWatch reads the arguments; done is true when the run ends
// here (--help, a usage error) with code.
func parseInboxWatch(ctx *checks.Context, args []string) (w *inboxWatch, code int, done bool) {
	w = &inboxWatch{ctx: ctx, interval: "30", self: shellBasename(ctx.Root), pid: os.Getpid()}
	usage := func(format string, a ...any) (*inboxWatch, int, bool) {
		fmt.Fprintf(ctx.Stderr, "inbox-watch: "+format+"\n", a...)
		return nil, 2, true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-i" || a == "--interval":
			if i+1 >= len(args) {
				return usage("--interval needs a value")
			}
			i++
			w.interval = args[i]
		case strings.HasPrefix(a, "--interval="):
			w.interval = strings.TrimPrefix(a, "--interval=")
		case a == "--preview":
			w.preview = true
		case a == "--self":
			if i+1 >= len(args) {
				return usage("--self needs a value (use --self '' to disable)")
			}
			i++
			w.self = args[i]
		case strings.HasPrefix(a, "--self="):
			w.self = strings.TrimPrefix(a, "--self=")
		case a == "-h" || a == "--help":
			io.WriteString(ctx.Stdout, inboxWatchHelp)
			return nil, 0, true
		case a == "--":
			for _, d := range args[i+1:] {
				w.dirs = append(w.dirs, strings.TrimSuffix(d, "/"))
			}
			i = len(args)
		case strings.HasPrefix(a, "-"):
			return usage("unknown option '%s' (try --help)", a)
		default:
			w.dirs = append(w.dirs, strings.TrimSuffix(a, "/"))
		}
	}
	if w.interval == "" || strings.Trim(w.interval, "0123456789") != "" {
		return usage("interval must be a positive integer, got '%s'", w.interval)
	}
	n, err := strconv.ParseInt(w.interval, 10, 64)
	if err == nil && n < 1 {
		return usage("interval must be >= 1 second, got '%s'", w.interval)
	}
	if err != nil || n > int64(time.Duration(1<<62)/time.Second) {
		n = int64(time.Duration(1<<62) / time.Second) // sleep "forever"
	}
	w.every = time.Duration(n) * time.Second
	if len(w.dirs) == 0 {
		w.dirs = []string{"./inbox"}
	}
	return w, 0, false
}

// path resolves a watched directory against the working directory.
func (w *inboxWatch) path(dir string) string { return resolve(w.ctx.WorkDir(), dir) }

// logical is $(cd "$dir" 2>/dev/null && pwd || echo "$dir"): the
// directory's logical absolute path, or dir as given when cd fails.
func (w *inboxWatch) logical(dir string) string {
	info, err := os.Stat(w.path(dir))
	if err != nil || !info.IsDir() {
		return dir
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Clean(w.ctx.WorkDir() + "/" + dir)
}

// lockHolder is the PID in a lock file: the digits of its first line.
func lockHolder(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := bytes.Cut(data, []byte("\n"))
	var b strings.Builder
	for _, c := range line {
		if '0' <= c && c <= '9' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// alive is `kill -0 PID`: the process exists and may be signalled.
func alive(pid string) bool {
	n, err := strconv.Atoi(pid)
	if err != nil {
		return false
	}
	return syscall.Kill(n, 0) == nil
}

// arm runs the arm-time checks, takes the locks, seeds the seen-ledger
// and reports the watch armed, all on stderr.
func (w *inboxWatch) arm() {
	errw := w.ctx.Stderr
	pwd := w.ctx.WorkDir()
	me := strconv.Itoa(w.pid)
	for _, dir := range w.dirs {
		abs := w.logical(dir)
		if abs != pwd && !strings.HasPrefix(abs, pwd+"/") {
			fmt.Fprintf(errw, "inbox-watch: WARN — %s resolves outside the current repo (%s). Watch your OWN inbox only; a peer's inbox self-echoes your outbound deposits (Principle 5 / SOP 2026-07-06). Proceeding — make sure this is a seat you hold.\n", dir, pwd)
		}
		if info, err := os.Stat(w.path(dir)); err != nil || !info.IsDir() {
			continue
		}
		lock := abs + "/" + inboxLockName
		if info, err := os.Stat(w.path(lock)); err == nil && info.Mode().IsRegular() {
			if holder := lockHolder(w.path(lock)); holder != "" && holder != me && alive(holder) {
				fmt.Fprintf(errw, "inbox-watch: WARN — a live watcher (PID %s) already holds %s (its %s). Two watchers on one inbox = double coverage / split provenance; kill PID %s or point this watcher elsewhere (Principle 5).\n", holder, dir, inboxLockName, holder)
				continue
			}
		}
		if os.WriteFile(w.path(lock), []byte(me+"\n"), 0o644) == nil {
			w.locks = append(w.locks, lock)
		}
	}
	for _, dir := range w.dirs {
		if info, err := os.Stat(w.path(dir)); err != nil || !info.IsDir() {
			fmt.Fprintf(errw, "inbox-watch: WARN — %s does not exist yet; watching for it to appear\n", dir)
		}
		seen := map[string]bool{}
		for _, name := range w.list(dir) {
			seen[name] = true
		}
		w.seen = append(w.seen, seen)
	}
	if w.self != "" {
		fmt.Fprintf(errw, "inbox-watch: armed — watching %s every %ss (pre-existing files not reported; own-sender deposits suppressed: %s)\n", strings.Join(w.dirs, " "), w.interval, w.self)
	} else {
		fmt.Fprintf(errw, "inbox-watch: armed — watching %s every %ss (pre-existing files not reported; own-sender suppression OFF)\n", strings.Join(w.dirs, " "), w.interval)
	}
}

// list is `ls -1 "$dir" | LC_ALL=C sort`: the directory's entries without
// dotfiles, in byte order; a file lists as itself, a missing or unreadable
// directory as nothing.
func (w *inboxWatch) list(dir string) []string {
	info, err := os.Stat(w.path(dir))
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{dir}
	}
	entries, err := os.ReadDir(w.path(dir))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// senderRE takes the date off a memo name, before the sender.
var senderRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}-`)

// sender is the second segment of YYYY-MM-DD-<sender>-…: the name without
// its date, up to its first dash.
func sender(name string) string {
	s := senderRE.ReplaceAllString(name, "")
	s, _, _ = strings.Cut(s, "-")
	return s
}

// poll reports each name that appeared in a watched directory since it was
// last seen (once per name for the life of the watcher), except own-sender
// memos, which still enter the ledger.
func (w *inboxWatch) poll() {
	multi := len(w.dirs) > 1
	for i, dir := range w.dirs {
		cur := w.list(dir)
		for _, name := range cur {
			if w.seen[i][name] || name == "" {
				continue
			}
			if w.self != "" && sender(name) == w.self {
				continue
			}
			p := name
			if multi {
				p = dir + "/" + name
			}
			line := "NEW INBOX DEPOSIT: " + p
			if w.preview {
				if first := firstLine(w.path(dir + "/" + name)); first != "" {
					line += " — " + first
				}
			}
			fmt.Fprintln(w.ctx.Stdout, line)
		}
		for _, name := range cur {
			w.seen[i][name] = true
		}
	}
}

// firstLine is $(head -n 1 FILE) for a regular file: its first line, NUL
// bytes dropped; "" for anything else.
func firstLine(p string) string {
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	var line []byte
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if i := bytes.IndexByte(buf[:n], '\n'); i >= 0 {
			line = append(line, buf[:i]...)
			break
		}
		line = append(line, buf[:n]...)
		if err != nil {
			break
		}
	}
	return cmdSubst(line)
}

// release removes the locks this watcher still holds (a successor that
// reclaimed one keeps it).
func (w *inboxWatch) release() {
	me := strconv.Itoa(w.pid)
	for _, lock := range w.locks {
		if info, err := os.Stat(w.path(lock)); err == nil && info.Mode().IsRegular() && lockHolder(w.path(lock)) == me {
			os.Remove(w.path(lock))
		}
	}
}
