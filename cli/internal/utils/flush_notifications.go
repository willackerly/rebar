package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/contracts"
)

// outboxEntryRE, outboxStatusRE and outboxCommitRE are the script's
// entry header, status and commit lines.
var (
	outboxEntryRE  = regexp.MustCompile(`^##[ \t\n\v\f\r]+([A-Z][A-Za-z0-9_.-]+):[ \t\n\v\f\r]+([0-9]+(\.[0-9]+){1,2})[ \t\n\v\f\r]+→[ \t\n\v\f\r]+([0-9]+(\.[0-9]+){1,2})[ \t\n\v\f\r]*$`)
	outboxStatusRE = regexp.MustCompile(`^-[ \t\n\v\f\r]+\*\*status:\*\*[ \t\n\v\f\r]+(pending|sent|dropped)`)
	outboxCommitRE = regexp.MustCompile(`^-[ \t\n\v\f\r]+\*\*commit:\*\*[ \t\n\v\f\r]+([a-f0-9]+)`)
)

// pendingNotification is a pending outbox entry.
type pendingNotification struct {
	contract, oldV, newV, commit string
}

// runFlushNotifications is scripts/flush-notifications.sh: dispatch the
// pending entries of the federation outbox (REBAR_OUTBOX, default
// <contracts_dir>/.state/pending-notifications.md) to each consumer that
// scan-consumers finds, through `$ASK_BIN <consumer>:featurerequest <msg>`,
// then mark the entries sent. It exits 0 when every dispatch succeeded (or
// there was nothing to send, or --dry-run) and 1 when one failed. It reads
// scan-consumers' JSON as the script does with jq.
func runFlushNotifications(ctx *checks.Context, args []string) int {
	repo := toplevel(ctx.WorkDir())
	outbox := os.Getenv("REBAR_OUTBOX")
	if outbox == "" {
		outbox = contracts.Dir(ctx.Root) + "/.state/pending-notifications.md"
	}
	dryRun, severity := false, ""
	notifyDefault := os.Getenv("REBAR_NOTIFY_DEFAULT")
	if notifyDefault == "" {
		notifyDefault = "notify"
	}
	askBin := os.Getenv("ASK_BIN")
	if askBin == "" {
		askBin = "ask"
	}
	out, errw := ctx.Stdout, ctx.Stderr
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--severity":
			if i+1 >= len(args) {
				fmt.Fprintln(errw, "flush-notifications: --severity needs a value")
				return 1 // the script: "$2: unbound variable"
			}
			severity = args[i+1]
			switch severity {
			case "breaking", "additive", "patch", "doc-only":
			default:
				fmt.Fprintf(errw, "flush-notifications: invalid severity '%s'\n", severity)
				return 2
			}
			i++
		case "-h", "--help":
			fmt.Fprintf(out, flushHelp, outbox, outbox)
			return 0
		default:
			fmt.Fprintf(errw, "flush-notifications: unexpected arg '%s'\n", args[i])
			return 2
		}
	}

	info, err := os.Stat(resolve(repo, outbox))
	if err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(out, "flush-notifications: no outbox at %s — nothing to flush\n", outbox)
		return 0
	}
	owner := shellBasename(repo)
	data, _ := os.ReadFile(resolve(repo, outbox))
	entries := pendingEntries(data)
	if len(entries) == 0 {
		fmt.Fprintf(out, "flush-notifications: no pending entries in %s\n", outbox)
		return 0
	}
	fmt.Fprintf(out, "flush-notifications: %s has %d pending notification(s)\n\n", owner, len(entries))

	processed, sent, skipped, failed := 0, 0, 0, 0
	for _, e := range entries {
		sev := severity
		if sev == "" {
			sev = inferSeverity(e.oldV, e.newV)
		}
		fmt.Fprintf(out, "→ %s: %s → %s (%s)\n", e.contract, e.oldV, e.newV, sev)
		// scan-consumers exits 1 when a consumer's section lacks a required
		// field; its stderr says which, and the valid consumers are still in
		// its output.
		var scanned strings.Builder
		_, code := scanConsumers(repo, &scanned, errw, e.contract, true)
		text := scanned.String()
		if code != 0 {
			fmt.Fprintf(errw, "    WARN: scan-consumers reported an error for %s (above)\n", e.contract)
		}
		if text == "" {
			text = `{"consumers":[]}`
		}
		lines := consumerLines(text)
		if len(lines) == 0 {
			fmt.Fprintln(out, "    (no consumers found — nothing to dispatch)")
			fmt.Fprintln(out)
			continue
		}
		for _, l := range lines {
			name, notify, _ := strings.Cut(l, "|")
			if name == "" {
				continue
			}
			switch notify {
			case "false":
				fmt.Fprintf(out, "    skip: %s (notify_on_change=false)\n", name)
				skipped++
				continue
			case "true":
			default:
				if notifyDefault == "skip" {
					fmt.Fprintf(out, "    skip: %s (notify_on_change unset, REBAR_NOTIFY_DEFAULT=skip)\n", name)
					skipped++
					continue
				}
			}
			msg := strings.TrimRight(composeNotice(owner, e, sev), "\n")
			if dryRun {
				fmt.Fprintf(out, "    DRY: ask_%s_featurerequest ← (%d chars)\n", name, shellLength(msg))
				processed++
				continue
			}
			if run(repo, nil, nil, askBin, name+":featurerequest", msg) == 0 {
				fmt.Fprintf(out, "    sent: %s\n", name)
				sent++
				processed++
			} else {
				fmt.Fprintf(errw, "    FAIL: %s (ask invocation failed; entry stays pending)\n", name)
				failed++
			}
		}
		fmt.Fprintln(out)
	}

	if !dryRun && sent > 0 {
		markSent(resolve(repo, outbox))
	}
	fmt.Fprintf(out, "flush-notifications: processed=%d sent=%d skipped=%d failed=%d\n", processed, sent, skipped, failed)
	if dryRun {
		fmt.Fprintln(out, "(dry-run — no FRs filed, no outbox state changed)")
	}
	if failed > 0 {
		return 1
	}
	return 0
}

const flushHelp = `Usage: flush-notifications.sh [--dry-run] [--severity LEVEL]

Reads %s and dispatches pending notifications to consumers via
ask_<consumer>_featurerequest. Marks entries "sent" on success.

  --dry-run                   show what would be sent, don't dispatch
  --severity breaking|additive|patch|doc-only
                              override auto-classification

Environment:
  REBAR_OUTBOX               outbox file path (default: %s)
  REBAR_NOTIFY_DEFAULT       behavior when notify_on_change absent in
                             consumer's CONSUMES.md: "notify" or "skip"
                             (default: notify)
  ASK_BIN                    path to ask CLI (default: ask)
`

// pendingEntries walks the outbox and returns its pending entries, in
// order.
func pendingEntries(data []byte) []pendingNotification {
	var entries []pendingNotification
	var cur pendingNotification
	in, status := false, ""
	flush := func() {
		if in && status == "pending" {
			entries = append(entries, cur)
		}
	}
	for _, line := range splitLines(data) {
		if m := outboxEntryRE.FindStringSubmatch(line); m != nil {
			flush()
			cur = pendingNotification{contract: m[1], oldV: m[2], newV: m[4]}
			status, in = "", true
		} else if m := outboxStatusRE.FindStringSubmatch(line); m != nil {
			status = m[1]
		} else if m := outboxCommitRE.FindStringSubmatch(line); m != nil {
			cur.commit = m[1]
		}
	}
	flush()
	return entries
}

// inferSeverity classifies a version change: breaking (major), additive
// (minor) or patch.
func inferSeverity(oldV, newV string) string {
	om, _, _ := strings.Cut(oldV, ".")
	nm, _, _ := strings.Cut(newV, ".")
	if om != nm {
		return "breaking"
	}
	if cutField(oldV, 2) != cutField(newV, 2) {
		return "additive"
	}
	return "patch"
}

// cutField is `cut -d. -fN` of s (all of s when it has no dot).
func cutField(s string, n int) string {
	f := strings.Split(s, ".")
	if len(f) == 1 {
		return s
	}
	if n <= len(f) {
		return f[n-1]
	}
	return ""
}

// composeNotice is the featurerequest question for an entry.
func composeNotice(owner string, e pendingNotification, severity string) string {
	return fmt.Sprintf(`Upstream change notice (auto-generated): %[1]s/%[2]s has been revised.

  %[2]s: %[3]s → %[4]s (%[5]s)
  commit: %[6]s

You're listed as a consumer of this contract per your CONSUMES.md. This
notification is filed via the rebar federation outbox flow (CHARTER §1.6).

Suggested triage:
  rebar contract drift-check
  # Then either bump your version_pinned in CONSUMES.md if you've
  # adopted the new version, or pin a planned upgrade date.

Severity classification (semver delta):
  breaking = MAJOR bump (likely interface-breaking)
  additive = MINOR bump (new behavior, backwards-compat expected)
  patch    = PATCH bump (clarification, doc, small-fix)

Source: %[1]s repo, commit %[6]s
`, owner, e.contract, e.oldV, e.newV, severity, e.commit)
}

// consumerLines is `jq -r '.consumers[] | "\(.name)|\(.notify_on_change)"'`
// on scan-consumers' JSON: nothing when it does not parse (the script
// builds it without escaping, so a quote in a rationale breaks it).
func consumerLines(text string) []string {
	var doc struct {
		Consumers []map[string]any `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil
	}
	var lines []string
	for _, c := range doc.Consumers {
		lines = append(lines, jqInterp(c["name"])+"|"+jqInterp(c["notify_on_change"]))
	}
	return lines
}

// jqInterp is a value as jq's string interpolation writes it.
func jqInterp(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// shellLength is ${#msg}: its length in characters, as bash counts it in
// a UTF-8 locale (bash 3.2 in the C locale counted bytes; bash 5 on macOS
// counts characters even there).
func shellLength(s string) int { return utf8.RuneCountInString(s) }

// markSent is the script's sed: every "- **status:** pending" line starts
// "- **status:** sent" instead; the file is replaced, as mv replaces it
// with the mktemp file (mode 0600).
func markSent(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.SplitAfter(string(data), "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "- **status:** pending") {
			lines[i] = "- **status:** sent" + strings.TrimPrefix(l, "- **status:** pending")
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".flush-notifications.*")
	if err != nil {
		return
	}
	_, werr := io.WriteString(tmp, strings.Join(lines, ""))
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}
