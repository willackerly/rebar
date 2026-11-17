package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/willackerly/rebar/cli/internal/checks"
)

const scanConsumersHelp = `Usage: scan-consumers.sh <contract-id> [--json]

  scan-consumers.sh C1-AGENTS         # text output
  scan-consumers.sh C1-AGENTS --json  # machine-readable

Looks for CONSUMES.md across registered repos (per ~/.config/ask/projects)
and reports any that declare a dep on <owner>/<contract-id>.<*>.

Override search set: REBAR_REPOS=path1,path2,... scan-consumers.sh ID
`

// consumer is one repo's CONSUMES.md entry for the contract.
type consumer struct {
	name, path, pin, pinDate, notify, rationale string
}

// runScanConsumers is scripts/scan-consumers.sh: the repos (REBAR_REPOS, a
// comma-separated list, else the ASK registry ASK_REGISTRY or
// ~/.config/ask/projects, without this repo) whose CONSUMES.md has a
// "## <owner>/<id>." section, with their pin, as text or --json.
func runScanConsumers(ctx *checks.Context, args []string) int {
	id, asJSON := "", false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "-h", "--help":
			io.WriteString(ctx.Stdout, scanConsumersHelp)
			return 0
		default:
			if id != "" {
				fmt.Fprintf(ctx.Stderr, "scan-consumers: unexpected arg '%s'\n", a)
				return 2
			}
			id = a
		}
	}
	if id == "" {
		fmt.Fprintln(ctx.Stderr, "Usage: scan-consumers.sh <contract-id>")
		return 2
	}
	_, code := scanConsumers(ctx.WorkDir(), ctx.Stdout, ctx.Stderr, id, asJSON)
	return code
}

// scanConsumers writes the script's output for contract id, run in dir, to
// out (as the script prints it, so it interleaves with errw as the
// script's stdout and stderr do) and returns the consumers it found and its
// exit code. Only version_pinned is required:
// a consumer section without it is reported on errw and left out, and the
// exit code is 1 once every consumer has been listed. pin_date,
// notify_on_change and rationale default to "".
func scanConsumers(dir string, out, errw io.Writer, id string, asJSON bool) ([]consumer, int) {
	b := out
	owner := shellBasename(toplevel(dir))
	paths := scanPaths(owner)
	if len(paths) == 0 {
		if asJSON {
			io.WriteString(b, `{"contract":"` + owner + "/" + id + `","consumers":[]}` + "\n")
		} else {
			io.WriteString(b, "scan-consumers: no repos to scan (registry empty + REBAR_REPOS unset)\n")
		}
		return nil, 0
	}
	head := regexp.MustCompile(`^## ` + regexp.QuoteMeta(owner+"/"+id) + `\.`)
	headBRE := checks.BRE("^## " + owner + "/" + id + "\\.")
	var found []consumer
	var results []string
	errors := 0
	for _, repo := range paths {
		consumes := repo + "/CONSUMES.md"
		info, err := os.Stat(resolve(dir, consumes))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, _ := os.ReadFile(resolve(dir, consumes))
		lines := splitLines(data)
		if headBRE == nil || !anyMatch(headBRE, lines) {
			continue
		}
		hdr := awkRegexp(owner, id, head)
		var section []string
		in := false
		for _, l := range lines {
			switch {
			case hdr.MatchString(l):
				in = true
				section = append(section, l)
			case in && strings.HasPrefix(l, "## "):
				in = false
			case in:
				section = append(section, l)
			}
		}
		c := consumer{
			name:      shellBasename(repo),
			path:      repo,
			pin:       fieldValue(sectionField(section, "version_pinned")),
			pinDate:   fieldValue(sectionField(section, "pin_date")),
			notify:    fieldValue(sectionField(section, "notify_on_change")),
			rationale: sectionField(section, "rationale"),
		}
		if c.pin == "" {
			hdrLine := ""
			if len(section) > 0 {
				hdrLine = section[0]
			}
			fmt.Fprintf(errw, "scan-consumers: %s: section '%s' has no version_pinned (required: the semver this consumer is pinned to, CHARTER §1.6) — skipped\n", consumes, hdrLine)
			errors++
			continue
		}
		found = append(found, c)
		if asJSON {
			results = append(results, fmt.Sprintf(`{"name":"%s","path":"%s","version_pinned":"%s","pin_date":"%s","notify_on_change":"%s","rationale":"%s"}`,
				c.name, c.path, c.pin, c.pinDate, c.notify, c.rationale))
			continue
		}
		if len(results) == 0 {
			io.WriteString(b, "Consumers of " + owner + "/" + id + ":\n\n")
		}
		notify := c.notify
		if notify == "" {
			notify = "(unset; owner default applies)"
		}
		pinDate := c.pinDate
		if pinDate == "" {
			pinDate = "(unset)"
		}
		fmt.Fprintf(b, "  %s → %s\n", c.name, c.path)
		fmt.Fprintf(b, "    version_pinned:    %s\n", c.pin)
		fmt.Fprintf(b, "    pin_date:          %s\n", pinDate)
		fmt.Fprintf(b, "    notify_on_change:  %s\n", notify)
		if c.rationale != "" {
			fmt.Fprintf(b, "    rationale:         %s\n", c.rationale)
		}
		io.WriteString(b, "\n")
		results = append(results, "found")
	}
	switch {
	case asJSON:
		io.WriteString(b, `{"contract":"` + owner + "/" + id + `","consumers":[` + strings.Join(results, ",") + "]}\n")
	case len(results) == 0:
		io.WriteString(b, "No consumers of " + owner + "/" + id + " found in scanned repos.\n")
		fmt.Fprintf(b, "(Searched %d repo(s); cross-machine discovery is out of scope per CHARTER §2.10.)\n", len(paths))
	}
	if errors > 0 {
		return found, 1
	}
	return found, 0
}

// awkRegexp is the section header the script's awk matches: the dynamic
// regex "^## <owner>/<id>\.", an ERE, where a character awk cannot compile
// makes it match nothing.
func awkRegexp(owner, id string, literal *regexp.Regexp) *regexp.Regexp {
	re, err := regexp.Compile(`^## ` + owner + "/" + id + `\.`)
	if err != nil {
		return literal
	}
	return re
}

func anyMatch(re *regexp.Regexp, lines []string) bool {
	for _, l := range lines {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// sectionField is the script's `field NAME`: the rest of the first line of
// section that starts "- **<name>:**", without the blanks and tabs around
// it; "" when there is no such line.
func sectionField(section []string, name string) string {
	prefix := "- **" + name + ":**"
	for _, l := range section {
		if strings.HasPrefix(l, prefix) {
			return strings.Trim(l[len(prefix):], " \t")
		}
	}
	return ""
}

// fieldValue is the script's `value` filter (sed 's/#.*//;
// s/[[:space:]]*$//'): v up to its first #, without trailing whitespace.
func fieldValue(v string) string {
	v, _, _ = strings.Cut(v, "#")
	return strings.TrimRight(v, " \t\n\v\f\r")
}

// scanPaths is the set of repos to scan: REBAR_REPOS split on commas (as
// `IFS=, read -ra` splits it: up to its first newline, a trailing empty
// field dropped), else the registry's paths, without the owner's entry.
func scanPaths(owner string) []string {
	if v := os.Getenv("REBAR_REPOS"); v != "" {
		v, _, _ = strings.Cut(v, "\n")
		if v == "" {
			return nil
		}
		fields := strings.Split(v, ",")
		if fields[len(fields)-1] == "" {
			fields = fields[:len(fields)-1]
		}
		return fields
	}
	reg := os.Getenv("ASK_REGISTRY")
	if reg == "" {
		reg = filepath.Join(os.Getenv("HOME"), ".config", "ask", "projects")
	}
	data, err := os.ReadFile(reg)
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range readLines(data) {
		name, path := ifsEquals(line)
		name = strings.TrimRight(name, " \t\n\v\f\r")
		path = strings.TrimLeft(path, " \t\n\v\f\r")
		if name == "" || name == owner {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// ifsEquals is `IFS='=' read -r name path` on line: name up to the first
// =, path the rest, losing a single trailing = when it is path's only one.
func ifsEquals(line string) (string, string) {
	name, rest, ok := strings.Cut(line, "=")
	if !ok {
		return name, ""
	}
	if strings.Count(rest, "=") == 1 && strings.HasSuffix(rest, "=") {
		rest = strings.TrimSuffix(rest, "=")
	}
	return name, rest
}
