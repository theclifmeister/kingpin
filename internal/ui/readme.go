// The README's generated sections: the key table from the bindings and
// the screen captures, spliced between their markers by cmd/keys and
// TestReadmeCaptures (#275: out of keys.go).

package ui

import (
	"fmt"
	"strings"
)

// ReadmeKeys is the README's key table, rendered from the same bindings
// help is: one row a binding, grouped as help groups them. cmd/keys
// prints it and TestReadmeMatchesKeys holds the README to it.
func ReadmeKeys() string {
	var b strings.Builder
	b.WriteString("| Key | Legend | What it does | Where |\n|---|---|---|---|\n")
	for _, g := range helpGroups() {
		for _, k := range g.keys {
			where := "everywhere"
			if !k.global {
				var names []string
				for _, s := range k.screens {
					names = append(names, screens[s].word)
				}
				where = strings.Join(names, ", ")
			}
			label := strings.ReplaceAll(k.label, "<city>", `\<city\>`)
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", k.key, label, k.help, where)
		}
	}
	return b.String()
}

// tutorialLine is the one line of prose that names keys: the
// dashboard's `No sales queued. Press s to sell, n to end the day.`, the
// first thing a new player reads, its two keys drawn the pane's way.
// Every other key the player is told about is in the pane's KEYS or a
// modal's footer.
func tutorialLine() string {
	return emptyState("No sales queued. Press ", "s", " to sell, ", "n", " to end the day.")
}

// The README's key table sits between these markers; cmd/keys writes it
// there and TestReadmeMatchesKeys reads it back. The captures sit between
// ReadmeCaptureBegin(name) and ReadmeCaptureEnd the same way, written by
// `go test ./internal/ui -run TestReadmeCaptures -update` from the rich
// fixture on a fixed seed and read back by the same test.
const (
	ReadmeKeysBegin  = "<!-- keys:begin -->"
	ReadmeKeysEnd    = "<!-- keys:end -->"
	ReadmeCaptureEnd = "<!-- capture:end -->"
)

// ReadmeCaptureBegin is the marker a named capture starts at:
// `<!-- capture:dashboard-80x24 -->`.
func ReadmeCaptureBegin(name string) string { return "<!-- capture:" + name + " -->" }

// SpliceReadme replaces what sits between the begin and end markers with
// body; it reports false when the markers are missing.
func SpliceReadme(readme, begin, end, body string) (string, bool) {
	i := strings.Index(readme, begin)
	if i < 0 {
		return readme, false
	}
	j := strings.Index(readme[i:], end)
	if j < 0 {
		return readme, false
	}
	return readme[:i+len(begin)] + "\n" + body + readme[i+j:], true
}

// ReadmeSection is what sits between the begin and end markers, the
// newline after the begin marker dropped: what the README holds.
func ReadmeSection(readme, begin, end string) (string, bool) {
	i := strings.Index(readme, begin)
	if i < 0 {
		return "", false
	}
	j := strings.Index(readme[i:], end)
	if j < 0 {
		return "", false
	}
	return readme[i+len(begin)+1 : i+j], true
}

// SpliceReadmeKeys replaces the table between the README's key markers.
func SpliceReadmeKeys(readme, table string) (string, bool) {
	return SpliceReadme(readme, ReadmeKeysBegin, ReadmeKeysEnd, table)
}

// ReadmeKeysSection is the table as the README carries it: what cmd/keys
// writes and what the README must hold.
func ReadmeKeysSection(readme string) (string, bool) {
	return ReadmeSection(readme, ReadmeKeysBegin, ReadmeKeysEnd)
}
