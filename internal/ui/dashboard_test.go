package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// dashPanel is one bordered panel read back off a rendered dashboard:
// its title and its inner lines, the borders and the padding off.
type dashPanel struct {
	title string
	lines []string
}

// panelsOf reads the panels off a plain (ANSI-stripped) view: every
// `╭─ TITLE ─…╮` opens one, its rows run to the `╰…╯` at the same
// column, and panels side by side are read apart by their columns.
func panelsOf(view string) []dashPanel {
	rows := strings.Split(view, "\n")
	cells := make([][]rune, len(rows))
	for i, r := range rows {
		cells[i] = []rune(r)
	}
	var out []dashPanel
	for y, row := range cells {
		for x := 0; x < len(row); x++ {
			if row[x] != '╭' {
				continue
			}
			end := x + 1
			for end < len(row) && row[end] != '╮' {
				end++
			}
			if end >= len(row) {
				continue
			}
			p := dashPanel{title: strings.TrimSpace(strings.Trim(string(row[x+1:end]), "─"))}
			for yy := y + 1; yy < len(cells); yy++ {
				r := cells[yy]
				if x >= len(r) || r[x] == '╰' {
					break
				}
				if end < len(r) {
					p.lines = append(p.lines, strings.TrimSpace(string(r[x+1:end])))
				}
			}
			out = append(out, p)
			x = end
		}
	}
	return out
}

// panelNamed is the panel whose title starts with name, or nil.
func panelNamed(ps []dashPanel, name string) *dashPanel {
	for i := range ps {
		if strings.HasPrefix(ps[i].title, name) {
			return &ps[i]
		}
	}
	return nil
}

// blankRun is the longest run of blank lines in a panel.
func blankRun(p dashPanel) int {
	run, most := 0, 0
	for _, l := range p.lines {
		if l == "" {
			run++
			most = max(most, run)
		} else {
			run = 0
		}
	}
	return most
}

// lastLine is a panel's last line that says anything.
func lastLine(p dashPanel) string {
	for i := len(p.lines) - 1; i >= 0; i-- {
		if p.lines[i] != "" {
			return p.lines[i]
		}
	}
	return ""
}

// A number cut with an ellipsis: `$3,4…`, `82…`.
var cutNumber = regexp.MustCompile(`[0-9]…`)

// TestDashboardPanels pins the dashboard's shape at every common size
// (#83), on every dashboard the rich fixture renders: no panel carries
// a run of more than two blank rows, STREET's last line is the line on
// tonight's sales, never cut, HEAT, CASH and LAW at 80x24 have four
// lines each with no number cut to an ellipsis, and the wide layout
// has RIVALS and CITIES where the narrow one folds them away.
func TestDashboardPanels(t *testing.T) {
	sales := []string{"No sales queued", "Orders queued for tonight", "Lying low today", "sells the stash here"}
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {100, 30}} {
		n := 0
		richFixture(t, sz, func(m *Model, view, what string) {
			if m.screen != screenDashboard || m.mode != modePlay {
				return
			}
			n++
			what = fmt.Sprintf("%dx%d %s", sz[0], sz[1], what)
			ps := panelsOf(stripANSI(view))
			for _, p := range ps {
				if p.title == "DETAILS" {
					continue
				}
				if run := blankRun(p); run > 2 {
					t.Errorf("%s: %s has %d blank rows in a run:\n%s", what, p.title, run, stripANSI(view))
				}
			}
			street := panelNamed(ps, "STREET · ")
			if street == nil {
				t.Fatalf("%s: no STREET panel:\n%s", what, stripANSI(view))
			}
			last := lastLine(*street)
			ok := false
			for _, s := range sales {
				ok = ok || strings.Contains(last, s)
			}
			if !ok || strings.HasSuffix(last, "…") {
				t.Errorf("%s: STREET's last line is not the sales line: %q", what, last)
			}
			for _, name := range []string{"HEAT", "CASH", "LAW"} {
				p := panelNamed(ps, name)
				if p == nil {
					t.Errorf("%s: no %s panel", what, name)
					continue
				}
				if sz[0] < paneMinWidth {
					// CASH's last line may be blank: no warning, one city.
					if len(p.lines) != 4 || (lastLine(*p) == "" && name != "CASH") {
						t.Errorf("%s: %s has %d lines, want four: %q", what, name, len(p.lines), p.lines)
					}
					if m := cutNumber.FindString(strings.Join(p.lines, "\n")); m != "" {
						t.Errorf("%s: %s cuts a number: %q in %q", what, name, m, p.lines)
					}
				}
			}
			if sz[0] >= paneMinWidth {
				if panelNamed(ps, "RIVALS") == nil {
					t.Errorf("%s: no RIVALS panel:\n%s", what, stripANSI(view))
				}
				if sz[1] >= 40 && panelNamed(ps, "CITIES") == nil {
					t.Errorf("%s: no CITIES panel:\n%s", what, stripANSI(view))
				}
			} else if panelNamed(ps, "RIVALS") != nil || panelNamed(ps, "CITIES") != nil {
				t.Errorf("%s: the narrow layout folds RIVALS and CITIES away:\n%s", what, stripANSI(view))
			}
		})
		if n == 0 {
			t.Fatalf("%dx%d: the fixture rendered no dashboard", sz[0], sz[1])
		}
	}
}

// TestDashboardCornersAreYours pins the colour of the street's corner
// line: it is yours, in Crew blue (theme.CrewText), never the rival's
// purple (#83). Colours are only emitted under a colour profile, so the
// test renders under TrueColor (termenv's zero profile) and restores
// the one the tests run with.
func TestDashboardCornersAreYours(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor
	defer lipgloss.SetColorProfile(profile)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := newTestModel(t, sz[0], sz[1])
		m.w.Home().Corners[1].Owner = game.OwnerRival
		m.w.Rival.Arrived = 1
		line := m.cornersLine(m.w.Here().ID)
		if !strings.Contains(line, "corners 1 worked, 1 held of 10, 1 theirs") {
			t.Fatalf("the corner line reads %q", line)
		}
		view := m.View()
		if !strings.Contains(view, theme.CrewText.Render(line)) {
			t.Errorf("%dx%d: the corner line is not in Crew blue:\n%s", sz[0], sz[1], stripANSI(view))
		}
		if strings.Contains(view, theme.RivalText.Render(line)) {
			t.Errorf("%dx%d: the corner line is in the rival's purple", sz[0], sz[1])
		}
	}
}

// TestDashboardEstimateMatchesDialog pins the pane's `s sell ~N at
// <dial> = ~$X` row to the sell dialog's expect row for the same
// product and dial (#83): with nothing queued the dial is normal, and
// with an order queued it is the order's, as the dialog opens.
func TestDashboardEstimateMatchesDialog(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	id := w.Products[0]
	w.SetStock(w.Player.Location, id, 80)
	expect := regexp.MustCompile(`expect\s+~(\d+) of \d+ at ~\S+ = ~(\S+)`)
	sell := regexp.MustCompile(`s\s+sell ~(\d+) at (\S+) = ~(\S+)`)
	for _, dial := range []events.Dial{events.DialNormal, events.DialAggressive} {
		if dial != events.DialNormal {
			m.Update(key("s"))
			m.Update(key("enter"))
			m.Update(key("enter"))
			m.Update(key("3"))
			m.Update(key("enter"))
			m.Update(key("enter"))
			m.Update(key("esc")) // the dialog stays open for the next line (#103)
			if o, ok := w.Order(w.Player.Location, id); !ok || o.Dial != dial {
				t.Fatalf("queuing at %s: %+v", dial, w.Today.Orders)
			}
		}
		pane := sell.FindStringSubmatch(stripANSI(m.View()))
		if pane == nil {
			t.Fatalf("no sell estimate in the pane at %s:\n%s", dial, stripANSI(m.View()))
		}
		if pane[2] != dialShort(dial) {
			t.Errorf("the pane estimates at %s, want %s", pane[2], dialShort(dial))
		}
		m.Update(key("s"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		if m.mode != modeSell || m.dlg.step != 2 || m.dlg.dial != dial {
			t.Fatalf("the sell dialog: mode %v step %d dial %s", m.mode, m.dlg.step, m.dlg.dial)
		}
		dlg := expect.FindStringSubmatch(stripANSI(m.View()))
		if dlg == nil {
			t.Fatalf("no expect row in the dialog:\n%s", stripANSI(m.View()))
		}
		if pane[1] != dlg[1] || pane[3] != dlg[2] {
			t.Errorf("at %s the pane says ~%s = ~%s, the dialog ~%s = ~%s", dial, pane[1], pane[3], dlg[1], dlg[2])
		}
		m.Update(key("esc"))
		m.Update(key("esc"))
		m.Update(key("esc"))
		if m.mode != modePlay {
			t.Fatalf("mode %v after closing the dialog", m.mode)
		}
	}
}

// TestDashboardCapture writes the rich fixture's dashboard at 80x24
// and 120x40 to the directory KINGPIN_CAPTURE names, for the eye. It
// is a development aid, not an assertion.
func TestDashboardCapture(t *testing.T) {
	dir := os.Getenv("KINGPIN_CAPTURE")
	if dir == "" {
		t.Skip("set KINGPIN_CAPTURE=<dir> to write the dashboard captures")
	}
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {100, 30}} {
		n := 0
		richFixture(t, sz, func(m *Model, view, what string) {
			if m.screen != screenDashboard || m.mode != modePlay {
				return
			}
			n++
			name := filepath.Join(dir, fmt.Sprintf("%dx%d-%02d-%s.txt", sz[0], sz[1], n, what))
			if err := os.WriteFile(name, []byte(stripANSI(view)+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	}
}
