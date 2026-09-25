package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// collapse is s with every run of spaces one space, so a wrapped line
// read back through modalProse compares with the line it was.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// wrapLine (#463) breaks a line wider than the box at its spaces, the
// words whole and in order, the styles kept, the rows after the first
// hanging under the value; a line that fits is itself, a word wider
// than the box is cut through with nothing lost, and a pointer to a
// screen is never broken.
func TestWrapLine(t *testing.T) {
	if got := wrapLine("short line", 40); len(got) != 1 || got[0] != "short line" {
		t.Errorf("a line that fits: %q", got)
	}
	prose := "  Your enforcers went for Ivory's takings on Precinct Row and came back with $4,210 and one of them limping."
	for _, width := range []int{30, 40, 72} {
		got := wrapLine("\x1b[31m"+prose+"\x1b[0m", width)
		if len(got) < 2 {
			t.Fatalf("width %d: not wrapped: %q", width, got)
		}
		var plain []string
		for i, l := range got {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("width %d: row %d is %d wide: %q", width, i, w, ansi.Strip(l))
			}
			if !strings.Contains(l, "\x1b[") {
				t.Errorf("width %d: row %d lost its style: %q", width, i, l)
			}
			if i > 0 && !strings.HasPrefix(ansi.Strip(l), "    ") {
				t.Errorf("width %d: row %d does not hang under the line: %q", width, i, ansi.Strip(l))
			}
			plain = append(plain, ansi.Strip(l))
		}
		if collapse(strings.Join(plain, " ")) != collapse(prose) {
			t.Errorf("width %d: the words moved:\n%q\n%q", width, strings.Join(plain, " "), prose)
		}
	}
	// A row's value hangs under itself, past its label.
	r := row("terms", "keep 60 here, the shortfall bought each morning at $12.03 (the connect's price) up to the float")
	got := wrapLine(r, 50)
	if len(got) < 2 || !strings.HasPrefix(ansi.Strip(got[1]), strings.Repeat(" ", paneLabelW+1)) {
		t.Errorf("a row's value does not hang under itself: %q", got)
	}
	// A word wider than the box is cut through, nothing lost.
	long := strings.Repeat("x", 25)
	if got := wrapLine(long, 10); strings.ReplaceAll(strings.Join(got, ""), " ", "") != long {
		t.Errorf("a long word lost cells: %q", got)
	}
	// A pointer stays on one row wherever a row holds it (a row after
	// the first hangs two cells in).
	ptr := "A mover who deals by the case will take 24 Coke. Answer it on the market screen (2) in Bayport."
	for width := len("on the market screen (2)") + 2; width < len(ptr); width++ {
		joined := strings.Join(wrapLine(ptr, width), "\n")
		if !strings.Contains(joined, "on the market screen (2)") {
			t.Fatalf("width %d: the pointer broke:\n%s", width, joined)
		}
	}
}

// longLines are report lines as long as the playtest met (#463): the
// strike, the robbery, a short contract, a buyer's reward, a sale's
// tags, a headline.
var longLines = map[string]string{
	"territory": "Your enforcers went for Ivory's takings on Precinct Row and came back with $4,210, one of them limping and the other two talking about the next one.",
	"crew":      "An enforcer on the Old Mill corner was robbed on the way home: the take was light and the street noticed, so the corner pays less for a week.",
	"shipments": "Supply contract short: Pills in Eastside kept 40 of 60, there was no cash over the float to buy the shortfall this morning.",
	"sales":     "A promoter who throws the warehouse parties needs 78 Weed in Eastside inside 5 days and pays 1.4x street for it. Answer it on the market screen (2): it stands 5 days.",
	"heat":      "Weed sold 45/45 at $21.77 avg = +$979 (normal, standing, cut $5, quality 50, the corner's tax $12) in Eastside tonight.",
	"news":      "Police are working Fourth & Main and the Rail Yard in shifts this week, the chief says, until the dealing on the corners there stops for good.",
}

// No prose in the report, the walk-away, the new run or the details
// overlay is cut (#463): the modal wraps a long line under itself at
// 80x24, 100x30 and 120x40, a fast-forward's stop line included; the
// walk-away's terms and shortfalls read whole on the cursor's page;
// and the dashboard's alerts read whole in the overlay.
func TestModalsNeverCutProse(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		what := fmt.Sprintf("%dx%d", sz[0], sz[1])
		m := richModel(t, sz[0], sz[1])
		r := m.w.Report
		r.Territory = append(r.Territory, longLines["territory"])
		r.Crew = append(r.Crew, longLines["crew"])
		r.Shipments = append(r.Shipments, longLines["shipments"])
		r.Sales = append(r.Sales, longLines["sales"])
		r.Heat = append(r.Heat, longLines["heat"])
		r.News = append(r.News, longLines["news"])
		m.fastStop = "Stopped: the standing order for Coke in Eastside is short of stock, and the contract that feeds it ran out of float last night."
		m.mode = modeReport
		prose := scrolledProse(t, m)
		for sec, l := range longLines {
			if !strings.Contains(prose, collapse(l)) {
				t.Errorf("%s report: the %s line is not whole:\n%s", what, sec, strings.ReplaceAll(prose, ". ", ".\n"))
			}
		}
		if !strings.Contains(prose, collapse(m.fastStop)) {
			t.Errorf("%s report: the stop line is not whole:\n%s", what, prose)
		}
		if strings.Contains(prose, "…") {
			t.Errorf("%s report: a line is cut:\n%s", what, prose)
		}

		// The walk-away: each way out's terms, and what is short, on
		// the page the cursor is on.
		m.mode, m.fastStop = modePlay, ""
		m.Update(key("1"))
		m.Update(key("w"))
		if m.mode != modeExit {
			t.Fatalf("%s: w opened mode %v", what, m.mode)
		}
		rows := m.exitRows()
		for i, row := range rows {
			m.exit.cursor = i
			page := modalProse(t, m.View())
			if !strings.Contains(page, collapse(row.terms)) {
				t.Errorf("%s walk away on %s: the terms %q are not whole:\n%s", what, row.name, row.terms, page)
			}
			if !row.open && !strings.Contains(page, collapse(row.short)) {
				t.Errorf("%s walk away on %s: what is short, %q, is not whole:\n%s", what, row.name, row.short, page)
			}
			if strings.Contains(page, "…") {
				t.Errorf("%s walk away on %s: a line is cut:\n%s", what, row.name, page)
			}
		}

		// The details overlay opens the alerts whole.
		m.mode = modePlay
		m.Update(key("1"))
		m.mode = modeDetails
		prose = scrolledProse(t, m)
		for _, a := range m.alerts() {
			if !strings.Contains(prose, collapse(a.text)) {
				t.Errorf("%s details: the alert %q is not whole:\n%s", what, a.text, prose)
			}
		}
	}
}
