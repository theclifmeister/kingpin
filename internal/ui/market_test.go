package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
)

// bodyRows splits a play-mode view into MAIN and the pane, row by row:
// the body rows' first mainWidth cells and, where the pane sits beside
// MAIN, their last paneWidth cells, ANSI stripped.
func bodyRows(m *Model) (main, pane []string) {
	ls := strings.Split(m.View(), "\n")
	end := len(ls) - 1
	if !m.paneShown() && m.width < paneMinWidth {
		end-- // the strip
	}
	for i := 1; i < end; i++ {
		rs := []rune(stripANSI(ls[i]))
		if m.paneShown() && len(rs) >= paneWidth {
			pane = append(pane, string(rs[len(rs)-paneWidth:]))
			rs = rs[:len(rs)-paneWidth]
		}
		main = append(main, string(rs))
	}
	return main, pane
}

// The market's MAIN is the title with the city tabs, the product table
// and the buyers; every fact the detail block used to print under the
// table (the range, glut, margin, demand, what is elsewhere, the stash
// there, the wholesale note, where you are) is in the pane's sections
// at 120 columns and in the overlay at 80, and none of it is under the
// table (#84).
func TestMarketDetailInPane(t *testing.T) {
	m := newTestModel(t, 120, 40)
	home, other := m.w.CityOrder[0], m.w.CityOrder[1]
	weed := m.w.Products[0]
	m.w.Stash(home)[weed] = 40
	m.w.Stash(other)[weed] = 240
	m.Update(key("2"))
	facts := []string{"range 30d", "glut", "margin", "/day on", "per standard", "ELSEWHERE", m.w.CityName(other), "240 in " + m.w.CityName(other)}
	main, pane := bodyRows(m)
	all := strings.Join(main, "\n")
	if !strings.Contains(main[0], "MARKET · "+m.w.CityName(home)) || !strings.Contains(main[0], "[ ◉ "+m.w.CityName(home)+" ]  "+m.w.CityName(other)) {
		t.Errorf("the title is not `MARKET · <city>` with the city tabs: %q", main[0])
	}
	for _, f := range facts[:5] {
		if strings.Contains(all, f) {
			t.Errorf("MAIN carries the detail %q:\n%s", f, all)
		}
	}
	if !strings.Contains(all, "stash") || !strings.Contains(all, "demand/day") || strings.Contains(all, "stock") {
		t.Errorf("the table's columns are not `supplier stash demand/day`:\n%s", all)
	}
	paneText := strings.Join(pane, "\n")
	for _, f := range facts {
		if !strings.Contains(paneText, f) {
			t.Errorf("the pane at 120 lacks %q:\n%s", f, paneText)
		}
	}
	if !strings.Contains(paneText, strings.ToUpper(m.w.ProductName(weed))+" · "+strings.ToUpper(m.w.CityName(home))) {
		t.Errorf("the pane's selection is not the product in the city shown:\n%s", paneText)
	}
	// The other city is the wholesale city and not where you stand: the
	// notes say so, in the pane.
	m.Update(key("]"))
	main, pane = bodyRows(m)
	all, paneText = strings.Join(main, "\n"), strings.Join(pane, "\n")
	if strings.Contains(all, "You are in") || strings.Contains(all, "lots of") {
		t.Errorf("MAIN carries the notes:\n%s", all)
	}
	for _, f := range []string{"You are in " + m.w.CityName(home), "lots of", "40 in " + m.w.CityName(home)} {
		if !strings.Contains(paneText, f) {
			t.Errorf("the pane for %s lacks %q:\n%s", m.w.CityName(other), f, paneText)
		}
	}
	// At 80 columns the strip names the product and its range, and the
	// overlay carries the sections whole where the height allows.
	m.Update(key("["))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	ls := strings.Split(stripANSI(m.View()), "\n")
	strip := ls[len(ls)-2]
	if !strings.HasPrefix(strip, "▸ "+strings.ToUpper(m.w.ProductName(weed))) || !strings.Contains(strip, "range 30d $") {
		t.Errorf("the strip does not name the product and its range: %q", strip)
	}
	m.Update(key(" "))
	if m.mode != modeDetails {
		t.Fatalf("space at 80: mode %v", m.mode)
	}
	overlay := stripANSI(m.View())
	for _, f := range facts {
		if !strings.Contains(overlay, f) {
			t.Errorf("the overlay at 80 lacks %q:\n%s", f, overlay)
		}
	}
	m.Update(key("esc"))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assertFits(t, m.View(), 80, 24, "market at 80x24")
	main, _ = bodyRows(m)
	if all = strings.Join(main, "\n"); strings.Contains(all, "range") || strings.Contains(all, "glut") {
		t.Errorf("MAIN at 80 carries the detail:\n%s", all)
	}
}

// The market at every size (#84, the epic's mockups): the title line
// is `MARKET · <city>` with the tabs, the table's rows are followed by
// exactly one blank row and then BUYERS, and at 80 columns the strip
// names the selected product and its range.
func TestMarketRendersInTheGrammar(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		richFixture(t, sz, func(m *Model, view, what string) {
			if m.mode != modePlay || m.screen != screenMarket || m.onBuyers {
				return
			}
			main, _ := bodyRows(m)
			if !strings.HasPrefix(main[0], "MARKET · "+m.shown().Name+"   ") {
				t.Errorf("%dx%d %s: the title is %q", sz[0], sz[1], what, main[0])
			}
			header := -1
			for i, l := range main {
				if strings.HasPrefix(l, "  product") {
					header = i
				}
			}
			if header < 0 {
				t.Fatalf("%dx%d %s: no product table:\n%s", sz[0], sz[1], what, strings.Join(main, "\n"))
			}
			i := header + 1
			for i < len(main) && strings.TrimSpace(main[i]) != "" {
				i++
			}
			if i+1 >= len(main) || strings.TrimSpace(main[i]) != "" || !strings.HasPrefix(main[i+1], "BUYERS") {
				t.Errorf("%dx%d %s: the table is not followed by one blank row and BUYERS:\n%s", sz[0], sz[1], what, strings.Join(main, "\n"))
			}
			if sz[0] < paneMinWidth {
				ls := strings.Split(stripANSI(view), "\n")
				strip := ls[len(ls)-2]
				name := strings.ToUpper(m.w.ProductName(m.w.Products[m.cursor]))
				if !strings.HasPrefix(strip, "▸ "+name) || !strings.Contains(strip, "range 30d $") {
					t.Errorf("%dx%d %s: the strip does not name the product and its range: %q", sz[0], sz[1], what, strip)
				}
			}
		})
	}
}

// Left and right on the market turn the city; the cursor stays on the
// product and the pane follows to the city shown.
func TestMarketArrowsTurnTheCity(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.Update(key("2"))
	m.Update(key("j"))
	m.Update(key("right"))
	if m.city != m.w.CityOrder[1] || m.cursor != 1 || m.screen != screenMarket {
		t.Fatalf("right: city %s cursor %d screen %v", m.city, m.cursor, m.screen)
	}
	_, pane := bodyRows(m)
	if want := strings.ToUpper(m.w.ProductName(m.w.Products[1])) + " · " + strings.ToUpper(m.w.CityName(m.w.CityOrder[1])); !strings.Contains(strings.Join(pane, "\n"), want) {
		t.Errorf("the pane is not %q:\n%s", want, strings.Join(pane, "\n"))
	}
	m.Update(key("left"))
	if m.city != m.w.CityOrder[0] {
		t.Fatalf("left: city %s", m.city)
	}
	if lw := lipgloss.Width(m.viewTitle()); lw > 120 {
		t.Errorf("the title bar is %d wide", lw)
	}
	_ = game.You
}
