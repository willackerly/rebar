package checks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/sourcescan"
)

// check-freshness: flag markdown whose currency marker (freshness: or
// last-synced: YYYY-MM-DD) is older than the threshold. Tier 2.
// Native port of scripts/check-freshness.sh.
func init() {
	register(&Check{Name: "check-freshness", Run: runFreshness})
}

// now is the clock, replaced in tests.
var now = time.Now

var (
	freshnessRE  = regexp.MustCompile(`freshness: [0-9]{4}-[0-9]{2}-[0-9]{2}`)
	lastSyncedRE = regexp.MustCompile(`last-synced: [0-9]{4}-[0-9]{2}-[0-9]{2}`)
)

func runFreshness(ctx *Context, args []string) int {
	if Skip(ctx, MinTier("check-freshness"), "check-freshness") {
		return 0
	}
	maxAge := ""
	if len(args) > 0 {
		maxAge = args[0]
	}
	if maxAge == "" {
		maxAge = config.RCValueOr(ctx.Root, "freshness_max_days", "", "14")
	}
	limit, limitOK := TestInt(maxAge)

	today := dayOf(now())
	stale, total, unmarked := 0, 0, 0
	w := ctx.Stdout
	for _, f := range markdownFiles(ctx.WorkDir()) {
		if strings.HasPrefix(f.Display, "./templates/") {
			continue
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			unmarked++ // grep finds no marker in a file it cannot read
			continue
		}
		if bytes.Contains(data, []byte("freshness: archived")) {
			continue
		}
		marker, date := "freshness", firstMarkerDate(data, freshnessRE)
		if date == "" {
			marker, date = "last-synced", firstMarkerDate(data, lastSyncedRE)
		}
		if date == "" {
			unmarked++
			continue
		}
		// grep -o prints "Binary file … matches" for a binary file, which
		// the script counts and then fails to parse as a date.
		if sourcescan.TextPart(data) == nil {
			total++
			continue
		}
		if date == "0000-00-00" {
			continue
		}
		total++
		doc, ok := parseDay(date)
		if !ok || doc.Unix() == 0 {
			continue
		}
		age := ageDays(today, doc)
		if limitOK && age > limit {
			fmt.Fprintf(w, "STALE (%d days): %s — %s: %s\n", age, f.Display, marker, date)
			stale++
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Checked %d docs with currency markers (freshness:/last-synced:), %d stale (>%s days).\n", total, stale, maxAge)
	fmt.Fprintf(w, "%d .md file(s) carry no currency marker at all — unchecked, not verified.\n", unmarked)
	if stale > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Stale docs may contain outdated status claims.")
		fmt.Fprintln(w, "Review and update the freshness date after verifying content.")
		return 1
	}
	return 0
}

// firstMarkerDate returns the date of the first match of re in data, "" if
// none.
func firstMarkerDate(data []byte, re *regexp.Regexp) string {
	m := re.Find(data)
	if m == nil {
		return ""
	}
	return string(m[len(m)-len("YYYY-MM-DD"):])
}

// markdownFiles lists what the script's
//
//	find . -name "*.md" -not -path "./.git/*" -not -path "*/node_modules/*" -type f
//
// lists, below dir, in lexical order (find lists directory order): regular
// files named *.md, not under the top-level .git or any node_modules.
// Symbolic links are neither listed nor followed. Display is the path as
// find prints it (./docs/a.md).
func markdownFiles(dir string) []sourcescan.File {
	var files []sourcescan.File
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != dir {
				return fs.SkipDir
			}
			return nil
		}
		if p == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			// Nothing below these is listed.
			if rel == ".git" || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		display := "./" + filepath.ToSlash(rel)
		if strings.HasPrefix(display, "./.git/") || strings.Contains(display, "/node_modules/") {
			return nil
		}
		files = append(files, sourcescan.File{Display: display, Path: p})
		return nil
	})
	return files
}

// dayOf is local midnight of t's calendar day.
func dayOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// parseDay is local midnight of a YYYY-MM-DD date, as GNU `date -d` reads
// it: false for a date that does not exist (2026-02-30, month 13), where
// BSD `date -j -f` rolls over into the next month; the native check
// follows GNU on every platform. A day whose midnight a DST change skipped
// (2018-11-04 in Sao Paulo) is valid, an hour off midnight, which ageDays
// absorbs.
func parseDay(s string) (time.Time, bool) {
	return parseDayIn(s, time.Local)
}

// parseDayIn is parseDay in loc.
func parseDayIn(s string, loc *time.Location) (time.Time, bool) {
	y, errY := strconv.Atoi(s[0:4])
	m, errM := strconv.Atoi(s[5:7])
	d, errD := strconv.Atoi(s[8:10])
	if errY != nil || errM != nil || errD != nil {
		return time.Time{}, false
	}
	if c := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC); c.Year() != y || int(c.Month()) != m || c.Day() != d {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc), true
}

// ageDays is the number of calendar days from doc to today, both local
// midnights: their difference in seconds, plus half a day so a DST change
// between them (a day of 23 or 25 hours) does not shift it, divided by a
// day, truncated toward zero, as the script's shell arithmetic does.
func ageDays(today, doc time.Time) int64 {
	return (today.Unix() - doc.Unix() + 43200) / 86400
}
