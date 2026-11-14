package migrate

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// Setting is one `key = value` line proposed for .rebarrc. Source names the
// legacy script it replaces, and becomes a comment above its group.
type Setting struct {
	Key    string
	Value  string
	Source string
}

// Block markers around the settings rebar migrate writes. A later run adds to
// the existing block instead of starting a new one.
const (
	blockStart = "# --- rebar migrate: settings moved here from legacy script copies ---"
	blockEnd   = "# --- end rebar migrate ---"
)

// rcKeyOf returns the key of a .rebarrc line, parsed the way _rebar_config
// and config.RCValue read it: the text before `=`, at the start of the line,
// with trailing blanks removed. Comment and blank lines have no key.
func rcKeyOf(line string) (string, bool) {
	k, _, ok := strings.Cut(line, "=")
	if !ok {
		return "", false
	}
	k = strings.TrimRight(k, " \t")
	if k == "" || strings.ContainsAny(k, " \t#") {
		return "", false
	}
	return k, true
}

// RCKeys returns the keys a .rebarrc defines, in file order.
func RCKeys(rc []byte) []string {
	var keys []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(rc))
	for sc.Scan() {
		if k, ok := rcKeyOf(sc.Text()); ok && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

// RCValue returns the effective value of key in rc: the first line for the
// key wins and `#` starts a comment (as in _rebar_config).
func RCValue(rc []byte, key string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(rc))
	for sc.Scan() {
		line := sc.Text()
		k, ok := rcKeyOf(line)
		if !ok || k != key {
			continue
		}
		_, v, _ := strings.Cut(line, "=")
		if i := strings.Index(v, "#"); i >= 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v), true
	}
	return "", false
}

// HasMetric reports whether rc already declares any metric.<name>.* field.
func HasMetric(rc []byte, name string) bool {
	prefix := "metric." + name + "."
	for _, k := range RCKeys(rc) {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

// ValidValue reports whether v survives a round trip through .rebarrc:
// `#` would start a comment and a newline would end the line.
func ValidValue(v string) bool {
	return !strings.ContainsAny(v, "#\n\r")
}

// MergeRC returns rc with settings added in the rebar migrate block, and
// the settings actually added. It never edits or reorders an existing line:
// a key rc already defines is skipped (the existing line wins anyway, since
// the first line for a key is the one that counts), as is a setting whose
// value cannot be written. When rc has a block from an earlier run, new
// lines go at the end of it; otherwise a new block is appended.
func MergeRC(rc []byte, settings []Setting) ([]byte, []Setting) {
	have := map[string]bool{}
	for _, k := range RCKeys(rc) {
		have[k] = true
	}
	var added []Setting
	for _, s := range settings {
		if have[s.Key] || !ValidValue(s.Value) || s.Key == "" {
			continue
		}
		have[s.Key] = true
		added = append(added, s)
	}
	if len(added) == 0 {
		return rc, nil
	}

	body := renderSettings(added)
	text := string(rc)
	if i := strings.Index(text, blockStart+"\n"); i >= 0 {
		if j := strings.Index(text[i:], blockEnd); j >= 0 {
			at := i + j
			return []byte(text[:at] + body + text[at:]), added
		}
	}
	var b strings.Builder
	b.WriteString(text)
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
	}
	if len(text) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(blockStart + "\n")
	b.WriteString(body)
	b.WriteString(blockEnd + "\n")
	return []byte(b.String()), added
}

// renderSettings formats settings grouped by source, with the `=` of each
// group aligned, and a blank line between metric groups.
func renderSettings(settings []Setting) string {
	var b strings.Builder
	source := "\x00"
	for i := 0; i < len(settings); {
		if settings[i].Source != source {
			source = settings[i].Source
			if source != "" {
				fmt.Fprintf(&b, "# from %s\n", source)
			}
		}
		// A run of settings sharing a source and metric name aligns as one group.
		j := i + 1
		for j < len(settings) && settings[j].Source == source && groupOf(settings[j].Key) == groupOf(settings[i].Key) {
			j++
		}
		width := 0
		for _, s := range settings[i:j] {
			if len(s.Key) > width {
				width = len(s.Key)
			}
		}
		for _, s := range settings[i:j] {
			fmt.Fprintf(&b, "%-*s = %s\n", width, s.Key, s.Value)
		}
		if strings.HasPrefix(settings[i].Key, "metric.") && j < len(settings) {
			b.WriteString("\n")
		}
		i = j
	}
	return b.String()
}

// groupOf is the metric a metric.<name>.<field> key belongs to; other keys
// are each their own group, except that plain keys align together.
func groupOf(key string) string {
	if strings.HasPrefix(key, "metric.") {
		if i := strings.LastIndex(key, "."); i > len("metric.") {
			return key[:i]
		}
	}
	return ""
}
