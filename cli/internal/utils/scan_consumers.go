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
	text, _, code := scanConsumers(ctx.WorkDir(), id, asJSON)
	io.WriteString(ctx.Stdout, text)
	return code
}

// scanConsumers is the script's output for contract id, run in dir, the
// consumers it found, and its exit code. A consumer section without one of
// version_pinned, pin_date, notify_on_change and rationale ends the script
// there with exit 1 (the field's `grep | head | sed` pipeline fails under
// set -e and pipefail), before that consumer is printed; with --json
// nothing is printed at all.
func scanConsumers(dir, id string, asJSON bool) (string, []consumer, int) {
	var b strings.Builder
	owner := shellBasename(toplevel(dir))
	paths := scanPaths(owner)
	if len(paths) == 0 {
		if asJSON {
			b.WriteString(`{"contract":"` + owner + "/" + id + `","consumers":[]}` + "\n")
		} else {
			b.WriteString("scan-consumers: no repos to scan (registry empty + REBAR_REPOS unset)\n")
		}
		return b.String(), nil, 0
	}
	head := regexp.MustCompile(`^## ` + regexp.QuoteMeta(owner+"/"+id) + `\.`)
	headBRE := checks.BRE("^## " + owner + "/" + id + "\\.")
	var found []consumer
	var results []string
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
		c := consumer{name: shellBasename(repo), path: repo}
		var vals [4]string
		for i, name := range []string{"version_pinned", "pin_date", "notify_on_change", "rationale"} {
			v, ok := sectionField(section, name)
			if !ok {
				if asJSON {
					return "", nil, 1
				}
				return b.String(), nil, 1
			}
			vals[i] = v
		}
		c.pin = strings.ReplaceAll(vals[0], " ", "")
		c.pinDate = strings.ReplaceAll(vals[1], " ", "")
		// awk '{print $1}': fields split on blanks and tabs.
		if f := strings.FieldsFunc(vals[2], func(r rune) bool { return r == ' ' || r == '\t' }); len(f) > 0 {
			c.notify = f[0]
		}
		c.rationale = vals[3]
		found = append(found, c)
		if asJSON {
			results = append(results, fmt.Sprintf(`{"name":"%s","path":"%s","version_pinned":"%s","pin_date":"%s","notify_on_change":"%s","rationale":"%s"}`,
				c.name, c.path, c.pin, c.pinDate, c.notify, c.rationale))
			continue
		}
		if len(results) == 0 {
			b.WriteString("Consumers of " + owner + "/" + id + ":\n\n")
		}
		notify := c.notify
		if notify == "" {
			notify = "(unset; owner default applies)"
		}
		fmt.Fprintf(&b, "  %s → %s\n", c.name, c.path)
		fmt.Fprintf(&b, "    version_pinned:    %s\n", c.pin)
		fmt.Fprintf(&b, "    pin_date:          %s\n", c.pinDate)
		fmt.Fprintf(&b, "    notify_on_change:  %s\n", notify)
		if c.rationale != "" {
			fmt.Fprintf(&b, "    rationale:         %s\n", c.rationale)
		}
		b.WriteString("\n")
		results = append(results, "found")
	}
	switch {
	case asJSON:
		b.WriteString(`{"contract":"` + owner + "/" + id + `","consumers":[` + strings.Join(results, ",") + "]}\n")
	case len(results) == 0:
		b.WriteString("No consumers of " + owner + "/" + id + " found in scanned repos.\n")
		fmt.Fprintf(&b, "(Searched %d repo(s); cross-machine discovery is out of scope per CHARTER §2.10.)\n", len(paths))
	}
	return b.String(), found, 0
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

// sectionField is the value of the first "- **<name>:**" line of section,
// as the script's sed leaves it (after the last "<name>:**" and the spaces
// after it); false when there is no such line.
func sectionField(section []string, name string) (string, bool) {
	prefix := "- **" + name + ":**"
	marker := name + ":**"
	for _, l := range section {
		if strings.HasPrefix(l, prefix) {
			v := l[strings.LastIndex(l, marker)+len(marker):]
			return strings.TrimLeft(v, " "), true
		}
	}
	return "", false
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
