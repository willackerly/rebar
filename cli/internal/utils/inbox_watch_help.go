package utils

// inboxWatchHelp is inbox-watch.sh --help: its header comment, as
// `sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'` prints it (the --help
// parity case keeps it in step).
const inboxWatchHelp = "" +
	"inbox-watch.sh — Live watch on peer inbox/ dirs; one line per new deposit.\n" +
	"rebar-scripts: 2026.07.11\n" +
	"\n" +
	"Canonical implementation of practices/inbox-watch.md and federation\n" +
	"Principle 5 (\"a held inbox is a watched inbox\"). Polls one or more\n" +
	"inbox directories and emits\n" +
	"\n" +
	"  NEW INBOX DEPOSIT: <path>\n" +
	"\n" +
	"once for each file that appears after the watch is armed. Pre-existing\n" +
	"files are never reported (the arming snapshot is the baseline). With more\n" +
	"than one watched directory, the path is prefixed with the directory the\n" +
	"deposit landed in.\n" +
	"\n" +
	"SOP (2026-07-06, ratified as Principle 5 2026-07-11):\n" +
	"  - Watch YOUR OWN inbox only — watching a peer's inbox self-echoes\n" +
	"    your own outbound deposits. Multi-dir mode is for seats that hold\n" +
	"    several repos' own inboxes, never for peer surveillance.\n" +
	"  - One watcher per inbox — each watcher drops a PID lock\n" +
	"    (.inbox-watch.lock, a hidden dotfile) in the inbox and warns at arm\n" +
	"    time ONLY if another LIVE watcher already holds the SAME inbox (double\n" +
	"    coverage / split provenance). A stale lock from a crashed watcher is\n" +
	"    reclaimed silently. (This replaced a process-global check that\n" +
	"    false-positived on every sibling seat in a multi-seat single machine.)\n" +
	"\n" +
	"Runs until killed — arm it as a persistent background monitor at session\n" +
	"start (coordination-seat cold start; see practices/session-lifecycle.md).\n" +
	"Each emitted line is an event the hosting harness can surface into the\n" +
	"agent's session.\n" +
	"\n" +
	"Usage:\n" +
	"  ./scripts/inbox-watch.sh [options] [dir ...]\n" +
	"\n" +
	"  dir ...            inbox directories to watch (default: ./inbox)\n" +
	"\n" +
	"Options:\n" +
	"  -i, --interval N   poll every N seconds (default: 30)\n" +
	"  --preview          append the memo's first line to each deposit line\n" +
	"  -h, --help         show this header\n" +
	"\n" +
	"A watched directory that does not exist yet is warned about once (stderr)\n" +
	"and kept in the watch: it lists as empty until created, so files present\n" +
	"at creation are reported as new deposits.\n" +
	"\n" +
	"Zero dependencies beyond POSIX tools. Bash 3.2 compatible (macOS default).\n" +
	"\n"
