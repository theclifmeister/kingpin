package ui

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

// The help modal at 80x24 (#89): GLOBAL first, then a group per screen
// with only that screen's keys, WORDS last with a line a term, every
// line inside the modal, and the modal scrolls to its last row.
func TestHelpScrollsToTheLastRow(t *testing.T) {
	m := richModel(t, 80, 24)
	lines := m.helpLines()
	var titles []string
	for _, l := range lines {
		if p := stripANSI(l); p != "" && p == strings.ToUpper(p) {
			titles = append(titles, p)
		}
	}
	if titles[0] != "GLOBAL" || titles[len(titles)-1] != "WORDS" {
		t.Errorf("help's groups are %v, want GLOBAL first and WORDS last", titles)
	}
	for _, g := range helpGroups()[1:] {
		for _, b := range g.keys {
			if b.global || !b.names(screen(indexOf(screenOf, strings.ToLower(g.title)))) {
				t.Errorf("%s lists %s %s, which is not its own", g.title, b.key, b.label)
			}
		}
	}
	for _, l := range lines {
		if w := lipgloss.Width(l); w > m.modalInner() {
			t.Errorf("a help line is %d wide, over the modal's %d: %q", w, m.modalInner(), stripANSI(l))
		}
	}
	if len(words) != 10 {
		t.Errorf("WORDS has %d terms, want the dial, the float, the target, the file, drift, undercut, keep at, standing, the pane and the strip", len(words))
	}
	m.Update(key("?"))
	if m.mode != modeHelp {
		t.Fatalf("? opened mode %v", m.mode)
	}
	if !strings.Contains(stripANSI(m.View()), "↓ more") {
		t.Fatalf("help at 80x24 does not scroll:\n%s", stripANSI(m.View()))
	}
	for i := 0; i < 20 && strings.Contains(stripANSI(m.View()), "↓ more"); i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	view := stripANSI(m.View())
	if strings.Contains(view, "↓ more") || !strings.Contains(view, "↑ more") {
		t.Errorf("help did not scroll to its end:\n%s", view)
	}
	last := stripANSI(lines[len(lines)-1])
	if !strings.Contains(view, last) || !strings.Contains(view, "Greed is always available.") {
		t.Errorf("help's last rows are not shown at the end:\n%s", view)
	}
	if m.mode != modeHelp {
		t.Errorf("paging closed help: mode %v", m.mode)
	}
}

func indexOf(names map[screen]string, name string) int {
	for s, n := range names {
		if n == name {
			return int(s)
		}
	}
	return -1
}

var update = flag.Bool("update", false, "rewrite README.md's captures from the fixture")

// captureSeed is the seed the README's captures are rendered from; the
// fixture on it is the same run every time.
const captureSeed = 89

// readmeCaptures are the README's captures: the fixture on captureSeed
// at a size, turned to a screen, rendered plain. `go test ./internal/ui
// -run TestReadmeCaptures -update` writes them into README.md between
// their markers; without the flag the test diffs the README against
// them, so a screen change fails until the README is regenerated.
var readmeCaptures = []struct {
	name   string
	w, h   int
	screen screen
}{
	{"dashboard-80x24", 80, 24, screenDashboard},
	{"map-120x40", 120, 40, screenMap},
}

// capture renders one README capture: every line's trailing blanks
// dropped, so the README is stable under an editor that strips them.
func capture(t *testing.T, name string, w, h int, s screen) string {
	t.Helper()
	m := richModelSeeded(t, w, h, captureSeed)
	m.switchScreen(s)
	m.status = ""
	view := stripANSI(m.View())
	assertFits(t, m.View(), w, h, name)
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return "```text\n" + strings.Join(lines, "\n") + "\n```\n"
}

func TestReadmeCaptures(t *testing.T) {
	const path = "../../README.md"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	for _, c := range readmeCaptures {
		want := capture(t, c.name, c.w, c.h, c.screen)
		begin := ReadmeCaptureBegin(c.name)
		if *update {
			out, ok := SpliceReadme(readme, begin, ReadmeCaptureEnd, want)
			if !ok {
				t.Fatalf("README.md has no %s / %s markers", begin, ReadmeCaptureEnd)
			}
			readme = out
			continue
		}
		got, ok := ReadmeSection(readme, begin, ReadmeCaptureEnd)
		if !ok {
			t.Fatalf("README.md has no %s / %s markers", begin, ReadmeCaptureEnd)
		}
		if got != want {
			t.Errorf("README.md's %s capture is stale: run `go test ./internal/ui -run TestReadmeCaptures -update`\ngot:\n%s\nwant:\n%s", c.name, got, want)
		}
	}
	if *update {
		if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The fixture on the capture seed is the same run at both sizes and on
// every render: what the README's captures rely on.
func TestCaptureFixtureIsDeterministic(t *testing.T) {
	for _, c := range readmeCaptures {
		a := capture(t, c.name, c.w, c.h, c.screen)
		b := capture(t, c.name, c.w, c.h, c.screen)
		if a != b {
			t.Errorf("%s: two renders of the fixture on seed %d differ:\n%s\n%s", c.name, captureSeed, a, b)
		}
	}
}
