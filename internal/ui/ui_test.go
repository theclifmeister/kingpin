package ui

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

func newTestModel(t *testing.T, w, h int) *Model {
	t.Helper()
	t.Setenv("KINGPIN_HOME", t.TempDir())
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// endDay presses n and, if the news sim dealt a dilemma card overnight,
// answers it with the highlighted choice and reads the outcome, so the
// caller lands on the morning report the way it did before cards. Cards
// come at the seed's whim from day 5 on, and test seeds are wall-clock.
func endDay(t *testing.T, m *Model) {
	t.Helper()
	m.Update(key("n"))
	if m.mode == modeCard {
		assertFits(t, m.View(), m.width, m.height, "dilemma card")
		m.Update(key("enter"))
		if m.mode != modeCard || !m.cardDone {
			t.Fatalf("answering the card: mode %v done %v", m.mode, m.cardDone)
		}
		assertFits(t, m.View(), m.width, m.height, "dilemma outcome")
		m.Update(key("enter"))
	}
}

func assertFits(t *testing.T, view string, w, h int, what string) {
	t.Helper()
	ls := strings.Split(view, "\n")
	if len(ls) > h {
		t.Errorf("%s: %d lines > height %d", what, len(ls), h)
	}
	for i, l := range ls {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells wide > %d: %q", what, i, lw, w, l)
		}
		// A modal wider than the screen wraps its border onto the next
		// line: the top-right corner then starts a line of its own.
		if p := strings.TrimSpace(stripANSI(l)); strings.HasPrefix(p, "═") && strings.HasSuffix(p, "╗") && !strings.HasPrefix(p, "╔") {
			t.Errorf("%s: a modal wider than %d wrapped at line %d", what, w, i)
		}
	}
}

// assertFrame checks the frame every play-mode screen renders into: row
// 0 the title bar, row h-2 the ticker, row h-1 the status bar; from 100
// columns every body row ends in the pane's border column and the
// pane's last section is KEYS; under that row h-3 is the details strip.
func assertFrame(t *testing.T, m *Model, what string) {
	t.Helper()
	w, h := m.width, m.height
	view := m.View()
	assertFits(t, view, w, h, what)
	ls := strings.Split(view, "\n")
	if len(ls) != h {
		t.Errorf("%s: %d rows, want %d", what, len(ls), h)
		return
	}
	plain := make([]string, len(ls))
	for i, l := range ls {
		plain[i] = stripANSI(l)
	}
	if !strings.HasPrefix(plain[0], " KINGPIN") {
		t.Errorf("%s: row 0 is not the title bar: %q", what, plain[0])
	}
	if plain[h-2] != stripANSI(m.viewTicker()) {
		t.Errorf("%s: row %d is not the ticker: %q", what, h-2, plain[h-2])
	}
	if plain[h-1] != stripANSI(m.viewFooter()) {
		t.Errorf("%s: row %d is not the status bar: %q", what, h-1, plain[h-1])
	}
	switch {
	case m.paneShown():
		keysAt := -1
		for i := 1; i <= h-3; i++ {
			if lw := lipgloss.Width(ls[i]); lw != w {
				t.Errorf("%s: body row %d is %d cells, want %d: %q", what, i, lw, w, plain[i])
				continue
			}
			rs := []rune(plain[i])
			pane := string(rs[len(rs)-paneWidth:])
			if last := rs[len(rs)-1]; last != '│' && last != '╮' && last != '╯' {
				t.Errorf("%s: body row %d does not end in the pane's border: %q", what, i, plain[i])
			}
			if strings.HasPrefix(pane, "│ KEYS") {
				keysAt = i
			}
		}
		if keysAt < 0 {
			t.Errorf("%s: the pane has no KEYS section:\n%s", what, stripANSI(view))
			return
		}
		// Nothing but key rows between KEYS and the bottom border.
		var keys []string
		for _, b := range m.legend() {
			keys = append(keys, b.key)
		}
		for i := keysAt + 1; i < h-3; i++ {
			rs := []rune(plain[i])
			text := strings.TrimSpace(strings.Trim(string(rs[len(rs)-paneWidth:]), "│"))
			ok := false
			for _, k := range keys {
				if strings.HasPrefix(text, k) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: row %d after KEYS is not a key row: %q", what, i, text)
			}
		}
	case w < paneMinWidth:
		strip := strings.TrimRight(plain[h-3], " ")
		if !strings.HasPrefix(strip, "▸ ") || !strings.HasSuffix(strip, "␣ more") {
			t.Errorf("%s: row %d is not the details strip: %q", what, h-3, strip)
		}
	}
}

func TestRendersAtCommonSizes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {100, 30}} {
		m := newTestModel(t, sz[0], sz[1])
		// Play a few days with some trading so every panel has content.
		for i := 0; i < 5; i++ {
			m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 40
			m.Update(key("s"))
			m.Update(key("enter")) // product
			m.Update(key("enter")) // qty (blank = all)
			m.Update(key("3"))     // aggressive
			m.Update(key("enter")) // confirm
			endDay(t, m)           // end day -> report
			assertFits(t, m.View(), sz[0], sz[1], "report")
			m.Update(key("enter"))
		}
		// A full crew with a skim on record exercises every crew-screen line.
		m.Update(key("4"))
		m.w.Player.DirtyCash += 5000
		for i := 0; i < 6; i++ {
			m.Update(key("h"))
		}
		m.w.Crew.LastSkim = m.w.Day
		// Post the crew across the map so every cell shape is drawn.
		m.Update(key("5"))
		for i := range m.shown().Corners {
			m.mapCursor = i
			m.Update(key("c"))
			assertFits(t, m.View(), sz[0], sz[1], "post picker")
			m.Update(key("j"))
			m.Update(key("enter"))
			m.Update(key("e"))
			m.Update(key("enter"))
		}
		// A rival in town, at war, with the enforcers queued against it,
		// exercises the rival cells, the picker and the dashboard panel.
		m.Update(key("esc")) // a stray enter above may be asking to end the day
		m.w.Home().Corners[0].Owner, m.w.Home().Corners[0].Runner, m.w.Home().Corners[0].Enforcer = game.OwnerRival, 0, 0
		m.w.Rival.Arrived, m.w.Rival.Muscle, m.w.Rival.War, m.w.Rival.Observed = 1, 4, 47, true
		m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 900, Name: "Moose", Role: "enforcer", Skill: 70, Loyalty: 70, Nerve: 60, Wage: 65})
		m.w.Crew.NextID = 900
		m.mapCursor = 0
		m.Update(key("w"))
		assertFits(t, m.View(), sz[0], sz[1], "strike picker")
		m.Update(key("enter"))
		if m.w.Strike == nil {
			t.Fatalf("%dx%d: no strike queued: %q", sz[0], sz[1], m.status)
		}
		for i := range m.shown().Corners {
			m.mapCursor = i
			assertFits(t, m.View(), sz[0], sz[1], "map")
		}
		// The other city's map, and the ship dialog with something to send.
		m.Update(key("]"))
		for i := range m.shown().Corners {
			m.mapCursor = i
			assertFits(t, m.View(), sz[0], sz[1], "map elsewhere")
		}
		m.Update(key("["))
		// A route on with a target and a shipment in flight: the routes
		// under the grid with the cursor on them, the target dialog and
		// every screen that reports the road.
		route := m.set.Logistics.Routes(m.w.CityOrder[1])[0]
		m.w.Stash(route.From)[m.w.Products[0]] = 300
		m.w.Player.DirtyCash += m.set.Logistics.Float()
		m.Update(key("]"))
		for len(m.shown().Corners) > 0 && !m.onRoutes {
			m.Update(key("j"))
		}
		m.Update(key("r"))
		assertFits(t, m.View(), sz[0], sz[1], "map with the routes cursor")
		m.Update(key("R"))
		assertFits(t, m.View(), sz[0], sz[1], "target product")
		m.Update(key("enter"))
		for _, r := range "120" {
			m.Update(key(string(r)))
		}
		assertFits(t, m.View(), sz[0], sz[1], "target units")
		m.Update(key("enter"))
		if m.mode != modePlay || !m.w.Route(route.ID).Dial.On() || m.w.Route(route.ID).Target[m.w.Products[0]] != 120 {
			t.Fatalf("%dx%d: the target dialog left mode %v with %+v: %q %q", sz[0], sz[1], m.mode, m.w.Route(route.ID), m.status, m.tgt.err)
		}
		endDay(t, m)
		assertFits(t, m.View(), sz[0], sz[1], "report with the route")
		m.Update(key("enter"))
		if len(m.w.Shipments) != 1 {
			t.Fatalf("%dx%d: the route sent %d shipments: %v", sz[0], sz[1], len(m.w.Shipments), m.w.Report.Shipments)
		}
		assertFits(t, m.View(), sz[0], sz[1], "map with a shipment in flight")
		m.Update(key("["))
		m.Update(key("g"))
		assertFits(t, m.View(), sz[0], sz[1], "travel confirm")
		m.Update(key("esc"))
		for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
			m.Update(key(s))
			assertFrame(t, m, "screen "+s)
			// Space: the overlay where the strip is, the pane hidden and
			// shown again where it sits beside MAIN.
			m.Update(key(" "))
			if sz[0] < paneMinWidth {
				if m.mode != modeDetails {
					t.Fatalf("%dx%d: space on screen %s: mode %v", sz[0], sz[1], s, m.mode)
				}
				assertFits(t, m.View(), sz[0], sz[1], "details overlay "+s)
				m.Update(key("esc"))
			} else {
				if m.mode != modePlay || !m.paneHidden {
					t.Fatalf("%dx%d: space on screen %s: mode %v hidden %v", sz[0], sz[1], s, m.mode, m.paneHidden)
				}
				assertFrame(t, m, "screen "+s+" with the pane hidden")
				m.Update(key(" "))
			}
			if m.mode != modePlay || m.paneHidden {
				t.Fatalf("%dx%d: after space twice on screen %s: mode %v hidden %v", sz[0], sz[1], s, m.mode, m.paneHidden)
			}
		}
		// The table: a deal that holds, an offer waiting, a proposal for
		// tonight, and both pages of the propose dialog.
		m.Update(key("8"))
		m.w.Rival.Deals = []game.Deal{{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{m.w.Home().Corners[1].ID}}, Since: m.w.Day}}
		m.w.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Offered: true}, Expires: m.w.Day + 4}}
		assertFits(t, m.View(), sz[0], sz[1], "rivals screen")
		m.Update(key("d"))
		assertFits(t, m.View(), sz[0], sz[1], "propose kinds")
		m.Update(key("2"))
		assertFits(t, m.View(), sz[0], sz[1], "propose terms")
		m.Update(key("enter"))
		if m.w.Proposal == nil || m.w.Proposal.Kind != game.DealTribute {
			t.Fatalf("%dx%d: no tribute proposed: %q", sz[0], sz[1], m.status)
		}
		assertFits(t, m.View(), sz[0], sz[1], "rivals screen with a proposal")
		m.Update(key("1"))
		assertFits(t, m.View(), sz[0], sz[1], "dashboard with the table")
		m.w.Rival.Deals, m.w.Offers, m.w.Proposal = nil, nil, nil
		// Every node state on the tree: owned, available, short, locked,
		// and the buy confirmation.
		m.Update(key("6"))
		m.w.Player.DirtyCash += 20_000
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "upgrade confirm")
		m.Update(key("y"))
		for range m.upgradeRows() {
			assertFits(t, m.View(), sz[0], sz[1], "upgrades")
			m.Update(key("j"))
		}
		m.Update(key("4"))
		m.Update(key("f"))
		assertFits(t, m.View(), sz[0], sz[1], "fire confirm")
		m.Update(key("y"))
		m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 200
		m.Update(key("s"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		m.w.Corner(m.cfg.City.Territory.Start).Risk = 100 // a robbery for the report
		endDay(t, m)
		assertFits(t, m.View(), sz[0], sz[1], "report with crew")
		if len(m.w.Report.Territory) == 0 {
			t.Fatalf("%dx%d: report has no territory lines: %+v", sz[0], sz[1], m.w.Report)
		}
		m.Update(key("enter"))
		// The ledger: the picker, three fronts bought, one of them audited,
		// and an accountant on the books.
		m.Update(key("7"))
		m.w.Player.DirtyCash = 700_000
		m.w.Stats.PeakCash = 700_000
		m.Update(key("b"))
		assertFits(t, m.View(), sz[0], sz[1], "front picker")
		for i := 0; i < 3; i++ {
			m.Update(key("b"))
			m.Update(key("enter"))
		}
		if len(m.w.Fronts) != 3 {
			t.Fatalf("%dx%d: bought %d fronts: %q", sz[0], sz[1], len(m.w.Fronts), m.status)
		}
		m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 901, Name: "Books", Role: "accountant", Skill: 60, Loyalty: 60, Wage: 130})
		endDay(t, m)
		assertFits(t, m.View(), sz[0], sz[1], "report with fronts")
		if !strings.Contains(strings.Join(m.w.Report.Money, "\n"), "Washed") {
			t.Fatalf("%dx%d: report has no wash line: %v", sz[0], sz[1], m.w.Report.Money)
		}
		m.Update(key("enter"))
		m.w.Fronts[1].Audited = m.w.Day
		m.w.Fronts[1].FrozenUntil = m.w.Day + m.cfg.Laundering.Laundering.AuditFreezeDays
		m.w.Fronts[2].FrozenUntil = m.w.Day + 2
		m.Update(key("d"))
		for _, s := range []string{"7", "1", "4"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "ledger screen "+s)
		}
		m.Update(key("1"))
		m.Update(key("b"))
		assertFits(t, m.View(), sz[0], sz[1], "buy dialog")
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "buy qty")
		m.Update(key("esc"))
		m.Update(key("esc"))
		m.Update(key("?"))
		assertFits(t, m.View(), sz[0], sz[1], "help")
		m.Update(key("x"))

		// A cartel-scale world: ten-digit cash on every screen, then the
		// money moved clean (dirty cash on that scale is an arrest) so the
		// next day unlocks every rung of the product ladder.
		m.w.Player.DirtyCash = 1_234_567_890
		m.w.Stats.PeakCash = m.w.Player.DirtyCash
		for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
			m.Update(key(s))
			assertFrame(t, m, "rich screen "+s)
		}
		m.Update(key("b"))
		assertFits(t, m.View(), sz[0], sz[1], "rich front picker")
		m.Update(key("esc"))
		m.Update(key("1"))
		m.Update(key("b"))
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "rich buy qty")
		m.Update(key("esc"))
		m.Update(key("esc"))
		m.w.Player.CleanCash, m.w.Player.DirtyCash = m.w.Player.DirtyCash, 50_000
		endDay(t, m)
		m.Update(key("enter"))
		if got := len(m.w.Products); got != len(m.cfg.Market.Products) {
			t.Fatalf("%d of %d products unlocked with a billion in the bank", got, len(m.cfg.Market.Products))
		}
		last := m.w.Products[len(m.w.Products)-1]
		m.w.Stash(m.w.Player.Location)[last] = 20
		for _, s := range []string{"1", "2", "5"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "ladder screen "+s)
		}
		m.Update(key("1"))
		m.Update(key("s"))
		for range m.w.Products {
			m.Update(key("j"))
		}
		m.Update(key("enter"))
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "ladder sell dialog")
		m.Update(key("enter"))
		endDay(t, m)
		assertFits(t, m.View(), sz[0], sz[1], "ladder report")
		m.Update(key("enter"))
		m.w.Over = &game.Ending{Day: m.w.Day, Cause: "indicted", PeakCash: m.w.Stats.PeakCash}
		endDay(t, m)
		assertFits(t, m.View(), sz[0], sz[1], "rich game over")
	}
}

func TestCashFormatting(t *testing.T) {
	cases := map[int]string{
		0: "$0", 500: "$500", 9_999: "$9,999", -2_500: "-$2,500",
		10_000: "$10K", 45_000: "$45K", 123_456: "$123K", 999_499: "$999K", 999_600: "$1.0M",
		1_234_567: "$1.2M", 12_345_678: "$12M", 3_400_000_000: "$3.4B", -1_500_000: "-$1.5M",
		1_234_567_890_123: "$1.2T",
	}
	for n, want := range cases {
		if got := cash(n); got != want {
			t.Errorf("cash(%d) = %q, want %q", n, got, want)
		}
	}
	for v, want := range map[float64]string{19.5: "$19.50", 999.99: "$999.99", 2500: "$2,500", 10000: "$10,000"} {
		if got := price(v); got != want {
			t.Errorf("price(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestBuyThenSellFlow(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("b"))
	m.Update(key("enter")) // pick first product
	for _, r := range "10" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modePlay {
		t.Fatalf("buy did not complete: mode=%v err=%q", m.mode, m.dlg.err)
	}
	id := m.w.Products[0]
	if m.w.Stock(m.w.Player.Location, id) != 10 {
		t.Fatalf("stock after buy = %d", m.w.Stock(m.w.Player.Location, id))
	}
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter")) // blank = all
	m.Update(key("1"))     // quiet
	m.Update(key("enter"))
	if o, ok := m.w.Order(m.w.Player.Location, id); !ok || o.Qty != 10 {
		t.Fatalf("order not placed: %+v", m.w.Orders)
	}
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != 1 {
		t.Fatalf("end day: mode=%v day=%d", m.mode, m.w.Day)
	}
}

// The status bar never drops the message: at 80 columns a long status
// set after a key is on row h-1 on every screen, the legend giving way.
func TestStatusMessageAlwaysShows(t *testing.T) {
	m := newTestModel(t, 80, 24)
	msg := "Pay generous, $108/day. Loyalty climbs. The crew notice it too."
	if len(msg) < 60 {
		t.Fatalf("the message is %d characters", len(msg))
	}
	for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		m.Update(key(s))
		m.status = msg
		rows := strings.Split(m.View(), "\n")
		if got := stripANSI(rows[len(rows)-1]); !strings.Contains(got, msg) {
			t.Errorf("screen %s: the status bar lost the message: %q", s, got)
		}
	}
}

// The legend is a whole number of k() pairs at every width: pairs are
// dropped from the right, never cut in the middle.
func TestLegendNeverTruncatesMidPair(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		m := newTestModel(t, w, 24)
		for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
			m.Update(key(s))
			m.status = ""
			got := stripANSI(m.viewFooter())
			var prefixes []string
			for n := 0; n <= len(m.legend()); n++ {
				var p string
				for _, b := range m.legend()[:n] {
					p += stripANSI(k(b.key, b.label))
				}
				prefixes = append(prefixes, p)
			}
			whole := false
			for _, p := range prefixes {
				if got == p {
					whole = true
				}
			}
			if !whole {
				t.Errorf("%d columns, screen %s: the legend is not whole pairs: %q", w, s, got)
			}
			if lipgloss.Width(m.viewFooter()) > w {
				t.Errorf("%d columns, screen %s: the legend is wider than the terminal: %q", w, s, got)
			}
		}
	}
}

// Space opens the details as an overlay where the strip is, with the
// sections the pane shows beside MAIN at 120, and esc closes it; at 120
// space hides the pane and the main content takes the width.
func TestSpaceTogglesDetails(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("5"))
	secs, _ := m.details()
	if len(secs) == 0 {
		t.Fatal("the map has no details")
	}
	m.Update(key(" "))
	if m.mode != modeDetails {
		t.Fatalf("space at 80: mode %v", m.mode)
	}
	overlay := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "details overlay")
	for _, s := range secs {
		if !strings.Contains(overlay, s.title) {
			t.Errorf("the overlay lacks the section %q:\n%s", s.title, overlay)
		}
	}
	if !strings.Contains(overlay, "KEYS") {
		t.Errorf("the overlay has no KEYS:\n%s", overlay)
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc on the overlay: mode %v", m.mode)
	}
	m.Update(key(" "))
	m.Update(key(" "))
	if m.mode != modePlay {
		t.Fatalf("space twice: mode %v", m.mode)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	beside := stripANSI(m.View())
	for _, s := range secs {
		if !strings.Contains(beside, s.title) {
			t.Errorf("the pane at 120 lacks the section %q:\n%s", s.title, beside)
		}
	}
	m.Update(key(" "))
	if m.mode != modePlay || !m.paneHidden {
		t.Fatalf("space at 120: mode %v hidden %v", m.mode, m.paneHidden)
	}
	hidden := stripANSI(m.View())
	if strings.Contains(hidden, "DETAILS") || strings.Contains(hidden, "␣ more") {
		t.Errorf("the pane did not hide:\n%s", hidden)
	}
	if m.mainWidth() != 120 {
		t.Errorf("main is %d wide with the pane hidden", m.mainWidth())
	}
	m.Update(key(" "))
	if m.paneHidden || !strings.Contains(stripANSI(m.View()), "DETAILS") {
		t.Errorf("space again did not show the pane")
	}
}

func TestTickerNeverWiderThanTerminal(t *testing.T) {
	m := newTestModel(t, 60, 20)
	for i := 0; i < 30; i++ {
		endDay(t, m)
		m.Update(key("enter"))
	}
	for i := 0; i < 200; i++ {
		m.Update(tickMsg{})
		if w := lipgloss.Width(m.viewTicker()); w > 60 {
			t.Fatalf("ticker width %d at tick %d", w, i)
		}
	}
}

// Enter confirms dialogs and closes the report, so a stray extra press must
// never burn a day: on the play screen it only asks. n ends the day at once;
// enter ends it after a confirmation.
func TestEnterDoesNotEndDay(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("n"))
	m.Update(key("enter")) // close report
	day := m.w.Day
	m.Update(key("enter"))
	if m.w.Day != day || m.mode != modeConfirmEnd {
		t.Fatalf("one enter: day %d -> %d, mode %v", day, m.w.Day, m.mode)
	}
	assertFits(t, m.View(), 80, 24, "end-day confirm")
	m.Update(key("esc"))
	if m.w.Day != day || m.mode != modePlay {
		t.Fatalf("esc on the confirm: day %d -> %d, mode %v", day, m.w.Day, m.mode)
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	if m.w.Day != day+1 || m.mode != modeReport {
		t.Fatalf("enter, enter: day %d -> %d, mode %v", day, m.w.Day, m.mode)
	}
	m.Update(key("enter")) // close report: never a day
	m.Update(key("enter"))
	m.Update(key("y"))
	if m.w.Day != day+2 {
		t.Fatalf("enter, y: day %d -> %d", day+1, m.w.Day)
	}
	m.Update(key("enter"))
	m.Update(key("n"))
	if m.w.Day != day+3 {
		t.Fatalf("n did not advance the day: %d -> %d", day+2, m.w.Day)
	}
	// The target dialog: enter picks the product, enter sets the target,
	// and neither is a day.
	m.Update(key("enter"))
	m.Update(key("5"))
	m.Update(key("R"))
	if m.mode != modeTarget {
		t.Fatalf("R on the map: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("enter"))
	m.Update(key("7"))
	m.Update(key("enter"))
	if m.w.Day != day+3 || m.mode != modePlay {
		t.Fatalf("enter in the target dialog: day %d -> %d, mode %v (%s)", day+3, m.w.Day, m.mode, m.tgt.err)
	}
	route := m.set.Logistics.Routes(m.w.Player.Location)[0]
	if m.w.Route(route.ID).Target[m.w.Products[0]] != 7 {
		t.Fatalf("the target was not set: %+v", m.w.Route(route.ID))
	}
}

// The crew screen: h hires the selected candidate, f asks before firing,
// p cycles the pay dial, and none of it happens from other screens.
func TestCrewScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 5000
	m.Update(key("h"))
	if len(m.w.Crew.Members) != 0 {
		t.Fatal("hired from the dashboard")
	}
	m.Update(key("4"))
	if m.screen != screenCrew {
		t.Fatalf("screen = %v", m.screen)
	}
	m.Update(key("h")) // cursor starts on the first candidate when the roster is empty
	if len(m.w.Crew.Members) != 1 {
		t.Fatalf("hire failed: %q", m.status)
	}
	hired := m.w.Crew.Members[0]
	if m.w.Capacity(m.w.Player.Location) != m.w.Player.CarryLimit+hired.Units || m.w.Player.DirtyCash != 5000-hired.Fee {
		t.Fatalf("after hire: capacity %d cash %d, member %+v", m.w.Capacity(m.w.Player.Location), m.w.Player.DirtyCash, hired)
	}
	m.Update(key("p"))
	if m.w.Crew.Pay != events.PayGenerous {
		t.Fatalf("pay after one p = %v", m.w.Crew.Pay)
	}
	m.Update(key("p"))
	m.Update(key("p"))
	if m.w.Crew.Pay != events.PayFair {
		t.Fatalf("pay after three p = %v", m.w.Crew.Pay)
	}
	// Cursor is on the new hire; f asks, anything but y backs out.
	m.Update(key("f"))
	if m.mode != modeConfirmFire {
		t.Fatalf("f did not ask: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || len(m.w.Crew.Members) != 1 {
		t.Fatal("esc fired someone")
	}
	m.Update(key("f"))
	m.Update(key("y"))
	if len(m.w.Crew.Members) != 0 || len(m.w.Crew.FiredToday) != 1 {
		t.Fatalf("y did not fire: %+v", m.w.Crew)
	}
	m.Update(key("n"))
	if m.mode != modeReport || len(m.w.Report.Crew) == 0 {
		t.Fatalf("report has no crew lines: %+v", m.w.Report)
	}
	if !strings.Contains(strings.Join(m.w.Report.Crew, "\n"), hired.Name) {
		t.Fatalf("report does not mention %s: %v", hired.Name, m.w.Report.Crew)
	}
}

// v1World is the shape a schema-1 save had: one city, its market and the
// player's stock on World and Player themselves, no crew, corners, rival,
// fronts or second city. gob fills what it does not carry with zero
// values, the way a real old save reads today.
type v1World struct {
	SchemaVersion int
	Seed          uint64
	Day           int
	City          string
	Player        struct {
		DirtyCash  int
		Stock      map[string]int
		CarryLimit int
	}
	Products []string
	Market   map[string]*game.ProductMarket
	Heat     struct{ Value float64 }
	Upgrades map[string]bool
	Orders   map[string]game.SellOrder
}

// A schema-1 save (before the crew) continues: the run is upgraded with a
// hiring pool and fair pay, corners, a rival, the launder dial, a second
// city with routes to it, and the day is kept.
func TestOldSaveIsMigrated(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	fresh := sim.NewWorld(cfg, 1)
	old := v1World{SchemaVersion: 1, Seed: 1, Day: 9, City: "Eastside", Products: fresh.Products, Market: fresh.Home().Market, Upgrades: map[string]bool{}, Orders: map[string]game.SellOrder{}}
	old.Player.DirtyCash, old.Player.Stock, old.Player.CarryLimit = 4321, map[string]int{fresh.Products[0]: 7}, 100
	old.Heat.Value = 12
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old); err != nil {
		t.Fatal(err)
	}
	p, _ := game.SavePath()
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(key("c"))
	if m.mode != modePlay || m.w.Day != 9 || m.w.SchemaVersion != game.SchemaVersion {
		t.Fatalf("continue: mode %v day %d schema %d status %q", m.mode, m.w.Day, m.w.SchemaVersion, m.status)
	}
	if len(m.w.Crew.Candidates) == 0 || m.w.Crew.Pay != events.PayFair {
		t.Fatalf("migrated crew state: %+v", m.w.Crew)
	}
	if m.w.Worked() != 1 || m.w.Corner(m.cfg.City.Territory.Start).Runner != game.You {
		t.Fatalf("migrated territory: %d worked, corners %+v", m.w.Worked(), m.w.Home().Corners)
	}
	if m.w.Rival.Leader == "" || m.w.Rival.Personality == "" || m.w.Rival.Arrived != 0 {
		t.Fatalf("migrated rival: %+v", m.w.Rival)
	}
	if len(m.w.Fronts) != 0 || m.w.Laundering.Dial != events.LaunderNormal {
		t.Fatalf("migrated laundering: fronts %+v dial %v", m.w.Fronts, m.w.Laundering.Dial)
	}
	if l := m.w.Law; l.Chief.Name == "" || l.Chief.Personality == "" || l.DA.Name == "" || l.DA.Stance == "" || l.Chief.Since != 9 || l.DA.ElectedDay != 9 {
		t.Fatalf("migrated law: %+v", l)
	}
	home := m.cfg.City.Home().ID
	if len(m.w.CityOrder) != len(m.cfg.City.Cities) || m.w.Player.Location != home || m.w.Home().Heat != 12 || m.w.Stock(home, m.w.Products[0]) != 7 || m.w.Player.DirtyCash != 4321 {
		t.Fatalf("migrated cities: %v in %s heat %.0f stock %d cash %d", m.w.CityOrder, m.w.Player.Location, m.w.Home().Heat, m.w.Stock(home, m.w.Products[0]), m.w.Player.DirtyCash)
	}
	for _, cid := range m.w.CityOrder {
		if c := m.w.Cities[cid]; len(c.Corners) != len(m.cfg.City.City(cid).Corners) || len(c.Market) != len(m.w.Products) {
			t.Fatalf("migrated %s: %d corners, %d products", cid, len(c.Corners), len(c.Market))
		}
	}
	if !reflect.DeepEqual(m.w.Home().Market, fresh.Home().Market) {
		t.Fatal("the home market did not survive the migration")
	}
	m.Update(key("7"))
	assertFits(t, m.View(), 80, 24, "ledger after migration")
	m.Update(key("4"))
	assertFits(t, m.View(), 80, 24, "crew screen after migration")
	m.Update(key("5"))
	assertFits(t, m.View(), 80, 24, "map after migration")
	m.Update(key("]"))
	assertFits(t, m.View(), 80, 24, "the new city after migration")
	m.Update(key("2"))
	assertFits(t, m.View(), 80, 24, "market of the new city after migration")
	m.Update(key("1"))
	endDay(t, m)
	if m.w.Day != 10 || m.w.Over != nil {
		t.Fatalf("the migrated save did not play on: day %d over %v", m.w.Day, m.w.Over)
	}
}

// A save this build cannot read is refused with a readable message and the
// start menu moves the player to New run.
func TestUnreadableSaveIsRefused(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := sim.NewWorld(content.MustLoad(), 1)
	w.SchemaVersion = game.SchemaVersion + 1
	if err := game.Save(w); err != nil {
		t.Fatal(err)
	}
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.mode != modeStart {
		t.Fatalf("mode = %v, want the start menu", m.mode)
	}
	m.Update(key("c"))
	if m.mode != modeStart || !strings.Contains(m.status, "newer version") || m.startChoice != 1 {
		t.Fatalf("continue: mode %v status %q choice %d", m.mode, m.status, m.startChoice)
	}
	assertFits(t, m.View(), 80, 24, "start menu with error")
	m.Update(key("enter"))
	if m.mode != modePlay || m.w.SchemaVersion != game.SchemaVersion || m.w.Day != 0 {
		t.Fatalf("new run: mode %v schema %d day %d", m.mode, m.w.SchemaVersion, m.w.Day)
	}
}

// Left and right never switch tabs (#36): on every play screen they leave
// the screen alone, while tab, shift+tab and the digits still switch. In
// the sell dialog they still move the dial.
func TestArrowsStayOnScreen(t *testing.T) {
	m := newTestModel(t, 80, 24)
	if m.screen != screenDashboard {
		t.Fatalf("start screen %v", m.screen)
	}
	for i := 0; i < int(screenCount); i++ {
		m.Update(key(string(rune('1' + i))))
		if m.screen != screen(i) {
			t.Fatalf("digit %d: screen %v", i+1, m.screen)
		}
		m.Update(key("left"))
		m.Update(key("right"))
		m.Update(key("right"))
		if m.screen != screen(i) || m.mode != modePlay {
			t.Fatalf("arrows on screen %v went to %v (mode %v)", screen(i), m.screen, m.mode)
		}
	}
	m.Update(key("tab"))
	if m.screen != screenDashboard {
		t.Fatalf("tab from the last tab went to %v", m.screen)
	}
	m.Update(key("shift+tab"))
	if m.screen != screenCount-1 {
		t.Fatalf("shift+tab from the first tab went to %v", m.screen)
	}
	m.Update(key("1"))
	m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 5
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("right"))
	if m.mode != modeSell || m.dlg.dial != events.DialAggressive || m.screen != screenDashboard {
		t.Fatalf("right in the sell dialog: mode %v dial %v screen %v", m.mode, m.dlg.dial, m.screen)
	}
}

// The map is walked as a grid: from every cell, right selects the next
// corner along the row (a no-op at the edge), down the nearest corner in
// the row below, and the arrows alone reach every corner in city.toml.
func TestMapArrowsWalkGrid(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("5"))
	cs := m.shown().Corners
	// want is the corner the arrow should land on from i, or i itself.
	want := func(i, dx, dy int) int {
		best, bestD := i, 0
		for j, c := range cs {
			var d int
			switch {
			case dx != 0:
				if c.Y != cs[i].Y || (c.X-cs[i].X)*dx <= 0 {
					continue
				}
				d = (c.X - cs[i].X) * dx
			default:
				if c.Y != cs[i].Y+dy {
					continue
				}
				d = max(c.X-cs[i].X, cs[i].X-c.X)
			}
			if best == i || d < bestD {
				best, bestD = j, d
			}
		}
		return best
	}
	moves := []struct {
		key    string
		dx, dy int
	}{{"right", 1, 0}, {"left", -1, 0}, {"down", 0, 1}, {"up", 0, -1}}
	for i := range cs {
		for _, mv := range moves {
			m.mapCursor, m.onRoutes = i, false
			m.Update(key(mv.key))
			// Down off the bottom row reaches the routes under the grid;
			// the corner cursor stays put.
			if bottom := want(i, 0, 1) == i; mv.dy > 0 && bottom != m.onRoutes {
				t.Errorf("down from %s (%d,%d): on the routes %v", cs[i].Name, cs[i].X, cs[i].Y, m.onRoutes)
			}
			if got, w := m.mapCursor, want(i, mv.dx, mv.dy); got != w {
				t.Errorf("%s from %s (%d,%d): got %s (%d,%d), want %s (%d,%d)", mv.key,
					cs[i].Name, cs[i].X, cs[i].Y, cs[got].Name, cs[got].X, cs[got].Y, cs[w].Name, cs[w].X, cs[w].Y)
			}
			if m.screen != screenMap || m.mode != modePlay {
				t.Fatalf("%s left the map: screen %v mode %v", mv.key, m.screen, m.mode)
			}
		}
	}
	// A right at the end of a row stays put; the grid has a hole at (3,0).
	for i, c := range cs {
		if c.X == 2 && c.Y == 0 {
			m.mapCursor = i
			m.Update(key("right"))
			if m.mapCursor != i {
				t.Fatalf("right from %s jumped to %s", c.Name, cs[m.mapCursor].Name)
			}
		}
	}
	// Reachability: a flood fill by arrows from corner 0 visits every corner.
	seen := map[int]bool{0: true}
	queue := []int{0}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		for _, mv := range moves {
			m.mapCursor = i
			m.Update(key(mv.key))
			if !seen[m.mapCursor] {
				seen[m.mapCursor] = true
				queue = append(queue, m.mapCursor)
			}
		}
	}
	if len(seen) != len(cs) {
		t.Fatalf("arrows reach %d of %d corners", len(seen), len(cs))
	}
	// The picker reads the same index: the corner it posts on is the one
	// the arrows selected.
	m.mapCursor = 0
	m.Update(key("down"))
	m.Update(key("right"))
	sel := m.mapSelected().ID
	m.w.Player.DirtyCash = 5000
	m.Update(key("c"))
	if m.mode != modePost {
		t.Fatalf("c after arrows: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("enter"))
	if m.w.Corner(sel).Runner != game.You {
		t.Fatalf("posted on %s, not the arrowed corner %s", m.shown().Corners[m.mapCursor].ID, sel)
	}
}

// The upgrades tree is walked as three columns: right from a node in
// Operations selects the Security node at the same row, or the last one
// if that column is shorter; left at the first column is a no-op; up and
// down stay within a column; enter still buys the node under the cursor.
func TestUpgradeArrowsMoveColumns(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 8000
	m.Update(key("6"))
	ops := m.cfg.Upgrades.Branch("operations")
	sec := m.cfg.Upgrades.Branch("security")
	legal := m.cfg.Upgrades.Branch("legal")
	if len(ops) <= len(sec) || len(sec) <= len(legal) {
		t.Skipf("branches are %d/%d/%d nodes; the test wants them shrinking", len(ops), len(sec), len(legal))
	}
	id := func() string {
		u, _ := m.upgradeSelected()
		return u.ID
	}
	m.Update(key("left"))
	if id() != ops[0].ID {
		t.Fatalf("left at the first column moved to %s", id())
	}
	m.Update(key("down"))
	m.Update(key("right"))
	if id() != sec[1].ID {
		t.Fatalf("right from %s selected %s, want %s", ops[1].ID, id(), sec[1].ID)
	}
	m.Update(key("left"))
	if id() != ops[1].ID {
		t.Fatalf("left back selected %s, want %s", id(), ops[1].ID)
	}
	// Down to the bottom of Operations, then right lands on the last of
	// each shorter column and up/down never leave it.
	for range ops {
		m.Update(key("down"))
	}
	if id() != ops[len(ops)-1].ID {
		t.Fatalf("down past the end of Operations selected %s", id())
	}
	m.Update(key("right"))
	if id() != sec[len(sec)-1].ID {
		t.Fatalf("right from the bottom of Operations selected %s, want %s", id(), sec[len(sec)-1].ID)
	}
	m.Update(key("down"))
	if id() != sec[len(sec)-1].ID {
		t.Fatalf("down at the bottom of Security selected %s", id())
	}
	m.Update(key("right"))
	m.Update(key("right"))
	if id() != legal[len(legal)-1].ID {
		t.Fatalf("right at the last column selected %s, want %s", id(), legal[len(legal)-1].ID)
	}
	for range legal {
		m.Update(key("up"))
	}
	if id() != legal[0].ID {
		t.Fatalf("up past the top of Legal selected %s", id())
	}
	if m.screen != screenUpgrades || m.mode != modePlay {
		t.Fatalf("arrows left the tree: screen %v mode %v", m.screen, m.mode)
	}
	// Enter buys the node under the cursor: back to the top of Security.
	m.Update(key("left"))
	m.Update(key("left"))
	m.Update(key("right"))
	if id() != sec[0].ID {
		t.Fatalf("cursor is on %s, want %s", id(), sec[0].ID)
	}
	m.Update(key("enter"))
	if m.mode != modeConfirmUpgrade || m.upgradeID != sec[0].ID {
		t.Fatalf("enter: mode %v id %q status %q", m.mode, m.upgradeID, m.status)
	}
	m.Update(key("y"))
	if !m.w.Owns(sec[0].ID) || m.w.Day != 0 {
		t.Fatalf("y did not buy %s: owns %v day %d status %q", sec[0].ID, m.w.Owns(sec[0].ID), m.w.Day, m.status)
	}
}

// The map screen: c posts a runner (you, or one of the crew) on the
// selected corner and claims it, e posts an enforcer, a abandons; the
// dashboard says so when nothing is held; none of it happens elsewhere.
func TestMapScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 5000
	start := m.w.Corner(m.cfg.City.Territory.Start)
	if m.screen != screenDashboard || m.shown().Corners[m.mapCursor].ID != start.ID {
		t.Fatalf("cursor starts on corner %d, not yours", m.mapCursor)
	}
	m.Update(key("c"))
	if m.mode != modePlay || !strings.Contains(m.status, "map") {
		t.Fatalf("c on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.w.Crew.Members = append(m.w.Crew.Members,
		game.CrewMember{ID: 101, Name: "Dre", Role: "runner", Skill: 60, Loyalty: 70, Units: 120, Wage: 56},
		game.CrewMember{ID: 102, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 70, Wage: 55},
	)
	m.w.Crew.NextID = 102
	runner, enforcer := m.w.Crew.Member(101), m.w.Crew.Member(102)
	m.Update(key("5"))
	if m.screen != screenMap {
		t.Fatalf("screen %v", m.screen)
	}
	// Move to a free corner and post the runner: the picker lists you first.
	m.Update(key("j"))
	target := m.shown().Corners[m.mapCursor]
	if target.ID == start.ID {
		m.Update(key("j"))
		target = m.shown().Corners[m.mapCursor]
	}
	m.Update(key("c"))
	if m.mode != modePost || m.postRole != "runner" {
		t.Fatalf("c on the map: mode %v role %q", m.mode, m.postRole)
	}
	rows := m.postRows("runner")
	if rows[0].ID != game.You {
		t.Fatalf("picker rows %+v", rows)
	}
	for i, r := range rows {
		if r.ID == runner.ID {
			for j := 0; j < i; j++ {
				m.Update(key("j"))
			}
		}
	}
	m.Update(key("enter"))
	c := m.w.Corner(target.ID)
	if m.mode != modePlay || !c.Worked() || c.Runner != runner.ID || m.w.Held() != 2 {
		t.Fatalf("after posting: mode %v corner %+v held %d status %q", m.mode, *c, m.w.Held(), m.status)
	}
	m.Update(key("e"))
	m.Update(key("enter"))
	if c.Enforcer != enforcer.ID {
		t.Fatalf("after posting an enforcer: %+v status %q", *c, m.status)
	}
	assertFits(t, m.View(), 100, 30, "map with posts")
	// Firing the runner leaves the corner held but unworked; the day
	// report and the dashboard both say so once it drifts.
	m.Update(key("4"))
	for i, r := range m.crewRows() {
		if r.ID == runner.ID {
			m.crewCursor = i
		}
	}
	m.Update(key("f"))
	m.Update(key("y"))
	if c.Runner != 0 || !c.Held() {
		t.Fatalf("after firing the runner: %+v", *c)
	}
	for i := 0; i < m.cfg.City.Territory.DriftDays; i++ {
		endDay(t, m)
		m.Update(key("enter"))
	}
	if c.Held() || c.Enforcer != 0 {
		t.Fatalf("corner did not drift: %+v", *c)
	}
	if !strings.Contains(strings.Join(m.w.Report.Territory, "\n"), target.Name) {
		t.Fatalf("report does not mention %s: %v", target.Name, m.w.Report.Territory)
	}
	// Abandon your own corner: nothing sells and the dashboard says why.
	m.Update(key("5"))
	m.mapCursor = m.yourCorner()
	m.Update(key("a"))
	if m.w.Worked() != 0 {
		t.Fatalf("abandon: %d worked, status %q", m.w.Worked(), m.status)
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "hold no corner") {
		t.Fatal("dashboard does not say why nothing sells")
	}
	m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 10
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	assertFits(t, m.View(), 100, 30, "sell dialog with no corner")
	m.Update(key("enter"))
	m.Update(key("n"))
	if m.w.Stock(m.w.Player.Location, m.w.Products[0]) != 10 {
		t.Fatalf("sold %d units with no corner", 10-m.w.Stock(m.w.Player.Location, m.w.Products[0]))
	}
}

// The map screen's w: enforcers go against a rival corner at a force the
// picker chooses, one strike a day that can be called off, and nothing
// happens from other screens, on your own corners, or without enforcers.
func TestStrikeKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, "map") {
		t.Fatalf("w on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("5"))
	m.mapCursor = m.yourCorner()
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, "rival") {
		t.Fatalf("w on your own corner: mode %v status %q", m.mode, m.status)
	}
	docks := m.w.Corner("docks")
	docks.Owner = game.OwnerRival
	m.w.Rival.Arrived, m.w.Rival.Muscle = 1, 3
	for i, c := range m.shown().Corners {
		if c.ID == "docks" {
			m.mapCursor = i
		}
	}
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, "enforcers") {
		t.Fatalf("w with no enforcers: mode %v status %q", m.mode, m.status)
	}
	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 102, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 70, Nerve: 60, Wage: 55})
	m.w.Crew.NextID = 102
	m.Update(key("w"))
	if m.mode != modeStrike || len(m.strikeRows()) != 3 {
		t.Fatalf("w with an enforcer: mode %v rows %v", m.mode, m.strikeRows())
	}
	m.Update(key("3")) // hit
	if m.mode != modePlay || m.w.Strike == nil || m.w.Strike.Corner != "docks" || m.w.Strike.Force != events.ForceHit {
		t.Fatalf("after picking hit: mode %v strike %+v status %q", m.mode, m.w.Strike, m.status)
	}
	if !strings.Contains(stripANSI(m.View()), "hit tonight") {
		t.Fatal("the map does not show where the enforcers go")
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "tonight") {
		t.Fatal("the dashboard does not show where the enforcers go")
	}
	m.Update(key("5"))
	m.Update(key("w"))
	rows := m.strikeRows()
	if len(rows) != 4 {
		t.Fatalf("picker with a strike queued: %v", rows)
	}
	m.Update(key("4")) // stop
	if m.w.Strike != nil {
		t.Fatalf("stop did not call it off: %+v", m.w.Strike)
	}
	m.Update(key("w"))
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("enter")) // hit again
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Strike != nil || m.w.Stats.Strikes != 1 {
		t.Fatalf("after the night: mode %v strike %+v stats %+v", m.mode, m.w.Strike, m.w.Stats)
	}
	if !strings.Contains(strings.Join(m.w.Report.Territory, "\n"), "The Docks") {
		t.Fatalf("report does not mention the strike: %v", m.w.Report.Territory)
	}
	if !strings.Contains(strings.Join(m.w.Report.Heat, "\n"), "enforcers") {
		t.Fatalf("report does not charge heat for it: %v", m.w.Report.Heat)
	}
}

// The upgrades screen: enter (or u) asks before buying the selected node,
// y buys it and the effect lands at once, a locked or unaffordable node
// only explains itself, the dashboard lists what is owned, the report
// lists the purchase, and none of it happens from other screens.
func TestUpgradesScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 8000
	m.Update(key("u"))
	if m.mode != modePlay || !strings.Contains(m.status, "tree") {
		t.Fatalf("u on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("6"))
	if m.screen != screenUpgrades {
		t.Fatalf("screen = %v", m.screen)
	}
	rows := m.upgradeRows()
	if rows[0].ID != "stash" {
		t.Fatalf("first node is %s", rows[0].ID)
	}
	// Enter asks; anything but y backs out.
	m.Update(key("enter"))
	if m.mode != modeConfirmUpgrade || m.upgradeID != "stash" {
		t.Fatalf("enter did not ask: mode %v id %q status %q", m.mode, m.upgradeID, m.status)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || m.w.Owns("stash") || m.w.Day != 0 {
		t.Fatalf("esc bought something or ended the day: owns %v day %d", m.w.Owns("stash"), m.w.Day)
	}
	carry := m.w.Capacity(m.w.Player.Location)
	m.Update(key("u"))
	m.Update(key("y"))
	if !m.w.Owns("stash") || m.w.Player.DirtyCash != 3000 || m.w.Capacity(m.w.Player.Location) != carry+50 {
		t.Fatalf("y did not buy: owns %v cash %d capacity %d status %q", m.w.Owns("stash"), m.w.Player.DirtyCash, m.w.Capacity(m.w.Player.Location), m.status)
	}
	// Owned, locked and unaffordable nodes explain themselves without a modal.
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "already") {
		t.Fatalf("buying twice: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("j")) // stash2: needs nothing more, but $15K
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "costs") {
		t.Fatalf("unaffordable: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("j"))
	m.Update(key("j")) // supplier2: locked behind supplier
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "Supplier contact") {
		t.Fatalf("locked: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "upgrades stash") {
		t.Fatal("dashboard does not list the stash")
	}
	m.Update(key("n"))
	if m.mode != modeReport || len(m.w.Report.Upgrades) != 1 || !strings.Contains(m.w.Report.Upgrades[0], "Stash spot") {
		t.Fatalf("report: mode %v upgrades %v", m.mode, m.w.Report.Upgrades)
	}
	assertFits(t, m.View(), 100, 30, "report with an upgrade")
	if !strings.Contains(strings.Join(m.w.Report.Money, "\n"), "Stash spot -$5000") {
		t.Fatalf("money section: %v", m.w.Report.Money)
	}
	m.Update(key("enter"))
	if m.w.Report.CashBefore != 8000 {
		t.Fatalf("cash before = %d, want the morning's 8000", m.w.Report.CashBefore)
	}
	// It survives a save and load.
	m.Update(key("ctrl+s"))
	w, err := game.Load(m.set.Migrations()...)
	if err != nil || !w.Owns("stash") {
		t.Fatalf("load: %v owns %v", err, w != nil && w.Owns("stash"))
	}
}

// The ledger: b buys a front through the picker (elsewhere it still buys
// from the supplier), d cycles the launder dial from anywhere, the
// dashboard and report say what the fronts did, and a locked or
// unaffordable front is refused with a reason.
func TestLedgerScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	cheapest := m.set.Laundering.Offers()[0]
	m.Update(key("b"))
	if m.mode != modeBuy {
		t.Fatalf("b on the dashboard: mode %v", m.mode)
	}
	m.Update(key("esc"))
	m.Update(key("d"))
	if m.w.Laundering.Dial != events.LaunderGreedy || !strings.Contains(m.status, "greedy") {
		t.Fatalf("d from the dashboard: dial %v status %q", m.w.Laundering.Dial, m.status)
	}
	m.Update(key("d"))
	m.Update(key("d"))
	if m.w.Laundering.Dial != events.LaunderNormal {
		t.Fatalf("three d: dial %v", m.w.Laundering.Dial)
	}
	m.Update(key("7"))
	if m.screen != screenLedger {
		t.Fatalf("screen %v", m.screen)
	}
	// Too poor, then unlocked but short, then bought.
	m.Update(key("b"))
	if m.mode != modeFront {
		t.Fatalf("b on the ledger: mode %v", m.mode)
	}
	m.Update(key("enter"))
	if m.mode != modePlay || len(m.w.Fronts) != 0 || !strings.Contains(m.status, "Can't buy") {
		t.Fatalf("bought with $%d: fronts %d status %q", m.w.Player.DirtyCash, len(m.w.Fronts), m.status)
	}
	m.w.Stats.PeakCash = cheapest.UnlockCash
	m.w.Player.DirtyCash = cheapest.Cost - 1
	m.Update(key("b"))
	m.Update(key("enter"))
	if len(m.w.Fronts) != 0 || !strings.Contains(m.status, "only have") {
		t.Fatalf("bought short: fronts %d status %q", len(m.w.Fronts), m.status)
	}
	m.w.Player.DirtyCash = cheapest.Cost + m.cfg.Laundering.Laundering.Float + 10_000
	m.Update(key("b"))
	m.Update(key("enter"))
	if len(m.w.Fronts) != 1 || m.w.Fronts[0].ID != cheapest.ID || m.w.Player.DirtyCash != m.cfg.Laundering.Laundering.Float+10_000 {
		t.Fatalf("buy: fronts %+v cash %d status %q", m.w.Fronts, m.w.Player.DirtyCash, m.status)
	}
	m.Update(key("b"))
	if rows := m.frontRows(); len(rows) != len(m.set.Laundering.Offers())-1 || rows[0].ID == cheapest.ID {
		t.Fatalf("picker still offers what you own: %+v", rows)
	}
	m.Update(key("esc"))
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "/day") {
		t.Fatal("dashboard does not show the wash rate")
	}
	m.Update(key("n"))
	if m.mode != modeReport {
		t.Fatalf("mode %v", m.mode)
	}
	money := strings.Join(m.w.Report.Money, "\n")
	if !strings.Contains(money, cheapest.Name) || !strings.Contains(money, "Washed") {
		t.Fatalf("report: %v", m.w.Report.Money)
	}
	if m.w.Player.CleanCash <= 0 || m.w.Fronts[0].WashedToday <= 0 {
		t.Fatalf("nothing washed: clean %d front %+v", m.w.Player.CleanCash, m.w.Fronts[0])
	}
	m.Update(key("enter"))
	m.Update(key("7"))
	if !strings.Contains(stripANSI(m.View()), "open") {
		t.Fatal("ledger does not show the front open")
	}
}

// i and $ work only on the crew screen, ask first, and the tell shows on
// the dashboard and the crew screen once the file has grown twice without
// a bust; an investigation that names somebody marks them on the roster.
func TestInvestigateAndPayOffKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.w.Player.DirtyCash = 20_000
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "crew screen") {
		t.Fatalf("i on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("4"))
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "Nobody") {
		t.Fatalf("i with no crew: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("h"))
	if len(m.w.Crew.Members) != 1 {
		t.Fatalf("hire failed: %q", m.status)
	}
	hired := m.w.Crew.Members[0]
	cash := m.w.Player.DirtyCash
	m.Update(key("i"))
	if m.mode != modeConfirmInvestigate {
		t.Fatalf("i did not ask: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "investigate confirmation")
	m.Update(key("esc"))
	if m.mode != modePlay || m.w.Investigation != nil || m.w.Player.DirtyCash != cash {
		t.Fatal("esc queued an investigation")
	}
	m.Update(key("i"))
	m.Update(key("y"))
	if m.w.Investigation == nil || m.w.Player.DirtyCash != cash-m.set.Crew.InvestigateCost() {
		t.Fatalf("y did not queue: %+v cash %d status %q", m.w.Investigation, m.w.Player.DirtyCash, m.status)
	}
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "already") {
		t.Fatalf("second i: mode %v status %q", m.mode, m.status)
	}
	if !strings.Contains(stripANSI(m.View()), "Questions get asked tonight") {
		t.Fatal("crew screen does not show the queued investigation")
	}

	// Pay off the new hire.
	cash = m.w.Player.DirtyCash
	m.Update(key("$"))
	if m.mode != modeConfirmPayOff {
		t.Fatalf("$ did not ask: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "pay-off confirmation")
	m.Update(key("y"))
	c := m.w.Crew.Member(hired.ID)
	if c.Loyalty != min(100, hired.Loyalty+m.set.Crew.PayoffLoyalty()) || m.w.Player.DirtyCash != cash-m.set.Crew.PayoffCost(hired) || len(m.w.Crew.PaidOffToday) != 1 {
		t.Fatalf("pay off: loyalty %.0f -> %.0f cash %d -> %d status %q", hired.Loyalty, c.Loyalty, cash, m.w.Player.DirtyCash, m.status)
	}

	// The tell, and a named snitch, render everywhere they should.
	m.w.Heat.Leaks = 2
	m.w.Crew.Exposed = hired.ID
	for _, s := range []string{"1", "4"} {
		m.Update(key(s))
		view := stripANSI(m.View())
		assertFits(t, m.View(), 80, 24, "screen "+s+" with the tell")
		if !strings.Contains(view, "Somebody is talking") {
			t.Fatalf("screen %s does not hint at the informant:\n%s", s, view)
		}
	}
	if !strings.Contains(stripANSI(m.View()), "SNITCH") {
		t.Fatal("the roster does not mark the named informant")
	}
	m.Update(key("n"))
	if m.mode != modeReport {
		t.Fatalf("mode after n: %v", m.mode)
	}
	all := strings.Join(append(append([]string(nil), m.w.Report.Crew...), m.w.Report.Money...), "\n")
	if !strings.Contains(all, "investigation") || !strings.Contains(all, "Paid off "+hired.Name) || !strings.Contains(all, "Investigation -$") {
		t.Fatalf("report does not cover the night's questions and the pay-off:\n%s", all)
	}
	assertFits(t, m.View(), 80, 24, "report after an investigation")
	m.Update(key("enter"))
	if m.w.Report.CashBefore != 20_000 {
		t.Fatalf("cash before = %d, want the morning's 20000", m.w.Report.CashBefore)
	}
}

// A dilemma card dealt overnight is shown before the morning report:
// enter inside it decides, shows the outcome and then opens the report,
// and never ends the day; 1-3 pick a choice directly; the effects land
// at once and the outcome goes in the journal. A save on a card brings
// the same card back.
func TestCardBeforeReport(t *testing.T) {
	m := newTestModel(t, 80, 24)
	deal := func() {
		m.w.Dilemmas.Pending = &game.Card{ID: "test", Day: m.w.Day + 1, Title: "A test", Text: strings.Repeat("A long question about what to do next. ", 6),
			Choices: []game.Choice{
				{Label: "Pay", Outcome: "You paid.", Effects: map[string]float64{"dirty_cash": -100}},
				{Label: "Shout", Outcome: "You shouted.", Effects: map[string]float64{"heat": 7}},
				{Label: "Walk away", Outcome: "You walked."},
			}}
	}
	deal()
	cash, day := m.w.Player.DirtyCash, m.w.Day
	m.Update(key("n"))
	if m.mode != modeCard || m.w.Day != day+1 || m.w.Dilemmas.Pending == nil {
		t.Fatalf("after n: mode %v day %d pending %v", m.mode, m.w.Day, m.w.Dilemmas.Pending)
	}
	assertFits(t, m.View(), 80, 24, "card")
	if !strings.Contains(stripANSI(m.View()), "A TEST") {
		t.Fatal("card does not show its title")
	}
	m.Update(key("j"))
	m.Update(key("k"))
	m.Update(key("enter")) // decide: Pay
	if m.w.Day != day+1 || m.mode != modeCard || !m.cardDone || m.w.Dilemmas.Pending != nil {
		t.Fatalf("after deciding: day %d mode %v done %v pending %v", m.w.Day, m.mode, m.cardDone, m.w.Dilemmas.Pending)
	}
	if m.w.Player.DirtyCash != cash-100 {
		t.Fatalf("effect did not land: $%d -> $%d", cash, m.w.Player.DirtyCash)
	}
	if !strings.Contains(stripANSI(m.View()), "You paid.") {
		t.Fatal("outcome not shown")
	}
	assertFits(t, m.View(), 80, 24, "outcome")
	if last := m.w.Journal[len(m.w.Journal)-1]; last.Text != "You paid." || last.Source != "dilemma" {
		t.Fatalf("journal: %+v", last)
	}
	m.Update(key("enter")) // to the report
	if m.mode != modeReport || m.w.Day != day+1 {
		t.Fatalf("after the outcome: mode %v day %d", m.mode, m.w.Day)
	}
	m.Update(key("enter")) // close the report
	if m.mode != modePlay || m.w.Day != day+1 {
		t.Fatalf("after the report: mode %v day %d", m.mode, m.w.Day)
	}

	// Digits pick directly; the outcome's heat shows up.
	deal()
	heat := m.w.Here().Heat
	m.Update(key("n"))
	m.Update(key("2"))
	if !m.cardDone || m.w.Here().Heat != heat+7 || m.w.Day != day+2 {
		t.Fatalf("digit pick: done %v heat %v -> %v day %d", m.cardDone, heat, m.w.Here().Heat, m.w.Day)
	}
	m.Update(key("enter"))
	m.Update(key("enter"))

	// Saved on a card: continuing shows it again, and the choice still works.
	deal()
	m.Update(key("n"))
	if m.mode != modeCard {
		t.Fatalf("mode %v", m.mode)
	}
	m.Update(key("ctrl+c")) // quit saves
	m2, err := New(m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	m2.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m2.mode != modeStart {
		t.Fatalf("no continue offered: mode %v", m2.mode)
	}
	m2.Update(key("c"))
	if m2.mode != modeCard || m2.w.Dilemmas.Pending == nil || m2.w.Dilemmas.Pending.ID != "test" {
		t.Fatalf("continue: mode %v pending %+v", m2.mode, m2.w.Dilemmas.Pending)
	}
	m2.Update(key("3"))
	if !m2.cardDone || m2.w.Dilemmas.Pending != nil || m2.w.Day != day+3 {
		t.Fatalf("after continuing and deciding: done %v pending %v day %d", m2.cardDone, m2.w.Dilemmas.Pending, m2.w.Day)
	}
	m2.Update(key("enter"))
	if m2.mode != modeReport {
		t.Fatalf("mode %v", m2.mode)
	}
}

// The route: [ and ] (and the arrows on the market) turn the market and
// map to the other city without leaving the screen; s there sells out of
// that city's stash; on the map the arrows walk down past the grid to
// the routes, r turns the selected route's dial and R sets its target
// through the dialog (product, units), the report says what the route
// bought and sent and what landed, and t ships nothing by hand; g asks
// before moving you, and moving you steps you off your corner and leaves
// the stock behind.
func TestRouteAndTravelKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	home, hub := w.Home().ID, w.CityOrder[1]
	product := w.Products[0]
	m.w.Player.DirtyCash = 20_000 + m.set.Logistics.Float()
	m.Update(key("2"))
	m.Update(key("right"))
	if m.screen != screenMarket || m.mode != modePlay || m.city != hub {
		t.Fatalf("right on the market: screen %v mode %v city %s", m.screen, m.mode, m.city)
	}
	assertFits(t, m.View(), 80, 24, "the other city's market")
	m.Update(key("s"))
	if m.mode != modePlay || !strings.Contains(m.status, "Nothing") {
		t.Fatalf("s with nothing there: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("["))
	if m.city != home {
		t.Fatalf("[ went to %s", m.city)
	}
	m.Update(key("5"))
	m.Update(key("]"))
	if m.screen != screenMap || m.mode != modePlay || m.city != hub || len(m.shown().Corners) != len(m.cfg.City.City(hub).Corners) {
		t.Fatalf("] on the map: screen %v mode %v city %s corners %d", m.screen, m.mode, m.city, len(m.shown().Corners))
	}
	assertFits(t, m.View(), 80, 24, "the other city's map")
	// You cannot stand on a corner there from here.
	m.Update(key("c"))
	m.Update(key("enter"))
	if m.mode != modePlay || w.PostOf(game.You).City != home || !strings.Contains(m.status, "go there first") {
		t.Fatalf("posting yourself elsewhere: mode %v status %q", m.mode, m.status)
	}
	// t ships nothing by hand any more.
	m.Update(key("t"))
	if m.mode != modePlay || !strings.Contains(m.status, "routes") {
		t.Fatalf("t: mode %v status %q", m.mode, m.status)
	}

	// The routes out of the hub run into home. Down past the grid reaches
	// them, j and k walk them, k off the top comes back to the grid.
	routes := m.set.Logistics.Routes(hub)
	if len(routes) < 2 {
		t.Skipf("%d route(s) out of %s", len(routes), hub)
	}
	route := routes[1]
	for i := 0; i < 10 && !m.onRoutes; i++ {
		m.Update(key("j"))
	}
	if !m.onRoutes || m.routeCursor != 0 {
		t.Fatalf("down past the grid: on routes %v cursor %d", m.onRoutes, m.routeCursor)
	}
	m.Update(key("j"))
	if m.routeCursor != 1 {
		t.Fatalf("j on the routes: cursor %d", m.routeCursor)
	}
	assertFits(t, m.View(), 80, 24, "map on the routes")
	m.Update(key("k"))
	m.Update(key("k"))
	if m.onRoutes {
		t.Fatal("k off the top of the routes did not return to the grid")
	}
	m.Update(key("j"))
	m.Update(key("j"))
	if !m.onRoutes || m.routeCursor != 1 {
		t.Fatalf("back on the routes: on %v cursor %d", m.onRoutes, m.routeCursor)
	}
	// r turns the dial: off -> slow -> normal -> fast -> off.
	for i, want := range []events.RouteDial{events.RouteSlow, events.RouteNormal, events.RouteFast, events.RouteOff, events.RouteSlow} {
		m.Update(key("r"))
		if got := w.Route(route.ID).Dial; got != want {
			t.Fatalf("r %d: dial %v, want %v (%s)", i+1, got, want, m.status)
		}
	}
	if !strings.Contains(m.status, "target") {
		t.Fatalf("a dial with no target does not say so: %q", m.status)
	}
	// R sets the target: product, then units; esc backs out of the units.
	m.Update(key("R"))
	if m.mode != modeTarget || m.tgt.step != 0 {
		t.Fatalf("R: mode %v step %d status %q", m.mode, m.tgt.step, m.status)
	}
	assertFits(t, m.View(), 80, 24, "target: product")
	m.Update(key("enter"))
	if m.tgt.step != 1 {
		t.Fatalf("after the product: step %d err %q", m.tgt.step, m.tgt.err)
	}
	m.Update(key("esc"))
	if m.mode != modeTarget || m.tgt.step != 0 {
		t.Fatalf("esc on the units: mode %v step %d", m.mode, m.tgt.step)
	}
	m.Update(key("enter"))
	for _, r := range "30" {
		m.Update(key(string(r)))
	}
	assertFits(t, m.View(), 80, 24, "target: units")
	m.Update(key("enter"))
	if m.mode != modePlay || w.Route(route.ID).Target[product] != 30 || !strings.Contains(m.status, "30") {
		t.Fatalf("after the target: mode %v route %+v err %q status %q", m.mode, w.Route(route.ID), m.tgt.err, m.status)
	}
	if !strings.Contains(stripANSI(m.View()), "30") || !strings.Contains(stripANSI(m.View()), "slow") {
		t.Fatalf("the map does not show the dial and target:\n%s", stripANSI(m.View()))
	}
	// A target with nothing at the source and no wholesaler open sends
	// nothing; stocked, the route sends the shortfall the night the day
	// ends and the report says so, with the fare in the money.
	w.Stash(route.From)[product] = 50
	m.cfg.Routes.Routes[1].Risk = 0 // the sim shares the slice it was built with
	// The seed is wall-clock and the corner you stand on rolls for a
	// stick-up every day: one on the day the shipment lands would take
	// half of it out of the home stash (and Stats.Robbed counts cash only),
	// so the corner carries no risk while this test counts the road.
	w.PostOf(game.You).Risk = 0
	cash := w.Player.DirtyCash
	days := m.set.Logistics.Days(route, events.ShipSlow)
	endDay(t, m)
	if len(w.Shipments) != 1 || w.Shipments[0].Units != 30 || w.Shipments[0].Dial != events.ShipSlow || w.Shipments[0].Route != route.ID {
		t.Fatalf("after the day: shipments %+v report %v", w.Shipments, w.Report.Shipments)
	}
	if w.Stock(route.From, product) != 20 || w.Player.DirtyCash > cash-30*route.Cost || w.Bound(home, product) != 30 {
		t.Fatalf("stock %d cash %d -> %d bound %d", w.Stock(route.From, product), cash, w.Player.DirtyCash, w.Bound(home, product))
	}
	if !strings.Contains(strings.Join(w.Report.Shipments, "\n"), "left") || !strings.Contains(strings.Join(w.Report.Money, "\n"), "lots and fares") {
		t.Fatalf("report: %v %v", w.Report.Shipments, w.Report.Money)
	}
	assertFits(t, m.View(), 80, 24, "report with a shipment")
	m.Update(key("enter"))
	if !strings.Contains(stripANSI(m.View()), "30 ") {
		t.Fatal("the map does not show what is on the road")
	}
	// The dashboard says so: in a line at 80x24, in the CITIES panel
	// once the terminal is tall enough for one under the law.
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "30 units on the road") {
		t.Fatalf("the dashboard does not show what is on the road:\n%s", stripANSI(m.View()))
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if v := stripANSI(m.View()); !strings.Contains(v, "CITIES") || !strings.Contains(v, "◂30") {
		t.Fatalf("the tall dashboard has no CITIES panel with the road:\n%s", v)
	}
	assertFits(t, m.View(), 120, 40, "tall dashboard with the road")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(key("7"))
	if v := stripANSI(m.View()); !strings.Contains(v, "LOGISTICS") || !strings.Contains(v, route.Name) {
		t.Fatalf("the ledger does not list the route:\n%s", v)
	}
	m.Update(key("1"))
	for i := 0; i < days; i++ {
		endDay(t, m)
		m.Update(key("enter"))
	}
	if len(w.Shipments) != 0 || w.Stock(home, product) < 30 || w.Stats.Seizures+w.Stats.SeizedOnRoad != 0 {
		t.Fatalf("after %d days: shipments %+v home stash %d stats %+v", days, w.Shipments, w.Stock(home, product), w.Stats)
	}
	if !strings.Contains(strings.Join(w.Report.Shipments, "\n"), "landed") {
		t.Fatalf("report does not mention the arrival: %v", w.Report.Shipments)
	}
	// The dial is off again and the route rests.
	m.Update(key("5"))
	m.Update(key("]"))
	for i := 0; i < 10 && !m.onRoutes; i++ {
		m.Update(key("j"))
	}
	m.Update(key("j"))
	m.Update(key("r"))
	m.Update(key("r"))
	m.Update(key("r"))
	if w.Route(route.ID).Dial != events.RouteOff || w.Route(route.ID).Target[product] != 30 {
		t.Fatalf("off: %+v", w.Route(route.ID))
	}
	m.Update(key("["))
	m.Update(key("1"))

	// Travel: g asks, esc stays, y goes; your corner is left, stock stays.
	m.Update(key("g"))
	if m.mode != modeConfirmTravel {
		t.Fatalf("g: mode %v", m.mode)
	}
	assertFits(t, m.View(), 80, 24, "travel confirm")
	if !strings.Contains(stripANSI(m.View()), w.PostOf(game.You).Name) {
		t.Fatal("the travel confirmation does not name the corner you leave")
	}
	m.Update(key("esc"))
	if m.mode != modePlay || w.Player.Location != home {
		t.Fatal("esc travelled")
	}
	mine := w.PostOf(game.You)
	stock := w.Stock(home, product)
	m.Update(key("g"))
	m.Update(key("y"))
	if w.Player.Location != hub || m.city != hub || w.PostOf(game.You) != nil || !mine.Held() || w.Stock(home, product) != stock {
		t.Fatalf("after y: in %s (shown %s), posted %v, corner %+v, home stash %d", w.Player.Location, m.city, w.PostOf(game.You), *mine, w.Stock(home, product))
	}
	assertFits(t, m.View(), 80, 24, "dashboard elsewhere")
	// Now b buys here, and s sells out of the stash here.
	m.Update(key("b"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	if m.mode != modePlay || w.Stock(hub, product) <= 20 || w.Stock(home, product) != stock {
		t.Fatalf("buy elsewhere: mode %v err %q hub %d home %d", m.mode, m.dlg.err, w.Stock(hub, product), w.Stock(home, product))
	}
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	if o, ok := w.Order(hub, product); !ok || o.City != hub {
		t.Fatalf("sell elsewhere: %+v %v status %q", o, ok, m.status)
	}
	if _, ok := w.Order(home, product); ok {
		t.Fatal("sold at home from elsewhere")
	}
	m.Update(key("x"))
	if _, ok := w.Order(hub, product); ok {
		t.Fatal("x did not cancel the order here")
	}
	// And g goes home again, to your empty corner.
	m.Update(key("g"))
	m.Update(key("y"))
	if w.Player.Location != home {
		t.Fatalf("back home: in %s", w.Player.Location)
	}
	m.Update(key("5"))
	m.mapCursor = m.yourCorner()
	m.Update(key("c"))
	m.Update(key("enter"))
	if w.PostOf(game.You) == nil {
		t.Fatalf("could not step back on: %q", m.status)
	}
}

// The wholesaler is the routes' supplier, not yours: in the city that
// sells by the lot, once the door is open, the buy dialog still sells
// single units held to the stash and says where the lots go, and the
// market says so too.
func TestWholesaleFeedsTheRoutes(t *testing.T) {
	m := newTestModel(t, 100, 30)
	w := m.w
	hub := ""
	for _, c := range m.cfg.City.Cities {
		if c.Wholesale {
			hub = c.ID
		}
	}
	if hub == "" {
		t.Skip("no city sells by the lot")
	}
	offer := m.set.Logistics.Wholesale()
	w.Player.DirtyCash = 1_000_000
	product := w.Products[0]
	if err := w.Travel(hub); err != nil {
		t.Fatal(err)
	}
	m.city = hub
	w.Stats.PeakCash = offer.UnlockCash
	m.Update(key("2"))
	if v := stripANSI(m.View()); !strings.Contains(v, "Wholesale") || !strings.Contains(v, "routes") {
		t.Fatalf("the market does not say the lots feed the routes:\n%s", v)
	}
	m.Update(key("b"))
	m.Update(key("enter"))
	if mx := m.maxBuy(product); mx != w.Free(hub) {
		t.Fatalf("max %d units, free %d: the lots are not yours to buy", mx, w.Free(hub))
	}
	assertFits(t, m.View(), 100, 30, "buy where the wholesaler deals")
	if !strings.Contains(stripANSI(m.View()), "wholesaler") {
		t.Fatal("the buy dialog does not say where the lots go")
	}
	m.Update(key("enter"))
	if m.mode != modePlay || w.Stock(hub, product) != w.Capacity(hub) || w.Free(hub) != 0 {
		t.Fatalf("a buy at the hub: mode %v err %q stash %d capacity %d", m.mode, m.dlg.err, w.Stock(hub, product), w.Capacity(hub))
	}
}

// The rivals screen: d proposes through the two-page dialog (elsewhere
// it still cycles the launder dial), the proposal is answered in the
// morning, y and x answer the selected offer, enter in the dialog never
// ends the day, and a joint shipment is listed but not for sale.
func TestRivalsScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	w := m.w
	m.Update(key("8"))
	if m.screen != screenRivals {
		t.Fatalf("screen %v", m.screen)
	}
	m.Update(key("d"))
	if m.mode != modePlay || !strings.Contains(m.status, "Nobody") {
		t.Fatalf("d with no rival in town: mode %v status %q", m.mode, m.status)
	}
	// A rival dug in next door, with a grudge.
	w.Home().Corners[1].Owner, w.Home().Corners[1].Since = game.OwnerRival, 1
	w.Rival.Arrived, w.Rival.Muscle, w.Rival.Cash, w.Rival.Observed = 1, 4, 30_000, true
	// Full trust, a feared player and a war that is not yet loud: a short
	// truce is a certainty whatever the seed's personality.
	w.Rival.Trust, w.Rival.War, w.Player.Reputation.Fear = 100, 30, 100
	day := w.Day
	m.Update(key("d"))
	if m.mode != modePropose || m.proposeStep != 0 {
		t.Fatalf("d on the rivals screen: mode %v step %d", m.mode, m.proposeStep)
	}
	m.Update(key("4")) // shipment: listed, locked
	if m.mode != modePropose || !strings.Contains(m.status, "routes") {
		t.Fatalf("shipment: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("1")) // truce -> terms page
	if m.proposeStep != 1 || proposeKinds[m.proposeKind] != game.DealTruce {
		t.Fatalf("after picking truce: step %d kind %d", m.proposeStep, m.proposeKind)
	}
	m.Update(key("esc"))
	if m.mode != modePropose || m.proposeStep != 0 {
		t.Fatalf("esc on the terms page should go back a page: mode %v step %d", m.mode, m.proposeStep)
	}
	m.Update(key("enter")) // truce again
	m.Update(key("enter")) // the standard term
	if m.mode != modePlay || w.Proposal == nil || w.Proposal.Kind != game.DealTruce || w.Proposal.Terms.Days != m.cfg.Rivals.Diplomacy.TruceDays[1] {
		t.Fatalf("proposing: mode %v proposal %+v status %q", m.mode, w.Proposal, m.status)
	}
	if w.Day != day {
		t.Fatal("enter in the dialog ended the day")
	}
	m.Update(key("d"))
	if rows := m.proposeRows(); rows != len(proposeKinds)+1 {
		t.Fatalf("no withdraw row with a proposal queued: %d rows", rows)
	}
	m.Update(key("5")) // withdraw
	if w.Proposal != nil || m.mode != modePlay {
		t.Fatalf("withdraw: %+v mode %v", w.Proposal, m.mode)
	}
	m.Update(key("d"))
	m.Update(key("1"))
	m.Update(key("1")) // the short truce
	if w.Proposal == nil || m.set.Rivals.Chance(w, *w.Proposal) < 1 {
		t.Fatalf("propose: %+v status %q", w.Proposal, m.status)
	}
	endDay(t, m)
	m.Update(key("enter"))
	if d := w.Deal(game.DealTruce); d == nil || d.Terms.Days != m.cfg.Rivals.Diplomacy.TruceDays[0] {
		t.Fatalf("the morning after: deals %+v report %v", w.Rival.Deals, w.Report.Territory)
	}
	if !strings.Contains(strings.Join(w.Report.Territory, "\n"), "ACCEPTED") {
		t.Fatalf("report: %v", w.Report.Territory)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "Truce") || !strings.Contains(view, "left") {
		t.Fatalf("rivals screen does not show the truce:\n%s", view)
	}
	// d elsewhere is still the launder dial.
	m.Update(key("1"))
	m.Update(key("d"))
	if m.mode != modePlay || w.Laundering.Dial == events.LaunderNormal {
		t.Fatalf("d on the dashboard: mode %v dial %v", m.mode, w.Laundering.Dial)
	}
	m.Update(key("d"))
	m.Update(key("d"))
	// Offers: x declines the selected one, y accepts, a lapsed one is refused.
	m.Update(key("8"))
	w.Offers = []game.Offer{
		{ID: 7, Deal: game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: 400}, Offered: true}, Expires: w.Day + 2},
		{ID: 8, Deal: game.Deal{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{w.Home().Corners[0].ID}}, Offered: true}, Expires: w.Day - 1},
	}
	m.Update(key("down"))
	m.Update(key("y"))
	if len(w.Accepted) != 0 || !strings.Contains(m.status, "lapsed") {
		t.Fatalf("accepting a lapsed offer: accepted %+v status %q", w.Accepted, m.status)
	}
	m.Update(key("x"))
	if len(w.Offers) != 1 || w.Offers[0].ID != 7 {
		t.Fatalf("declining: offers %+v", w.Offers)
	}
	m.Update(key("y"))
	if len(w.Offers) != 0 || len(w.Accepted) != 1 || w.Accepted[0].ID != 7 {
		t.Fatalf("accepting: offers %+v accepted %+v status %q", w.Offers, w.Accepted, m.status)
	}
	w.Player.DirtyCash += 10_000
	endDay(t, m)
	if d := w.Deal(game.DealTribute); d == nil || d.Terms.PerDay != 400 || !d.Offered {
		t.Fatalf("the morning after accepting: %+v", w.Rival.Deals)
	}
	if !strings.Contains(strings.Join(w.Report.Money, "\n"), "Tribute") {
		t.Fatalf("report money: %v", w.Report.Money)
	}
	m.Update(key("enter"))
	// x elsewhere still cancels an order.
	m.Update(key("1"))
	home := w.Home().ID
	w.Stash(home)[w.Products[0]] = 5
	if err := w.PlaceSell(home, w.Products[0], 5, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	m.Update(key("x"))
	if _, ok := w.Order(home, w.Products[0]); ok {
		t.Fatal("x on the dashboard did not cancel the order")
	}
}

// t on the crew screen gives the selected lieutenant a city through a
// picker (and takes it away again); anywhere else it only points at the
// map, where the routes run themselves. The roster
// shows the city and hides the temper until it has been observed, and
// the dashboard says who runs what.
func TestAssignLieutenantKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	w.Player.DirtyCash = 50_000
	w.Crew.Members = append(w.Crew.Members,
		game.CrewMember{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 80, Nerve: 50, Wage: 50},
		game.CrewMember{ID: 2, Name: "Marcus", Role: game.RoleLieutenant, Skill: 70, Loyalty: 80, Nerve: 50, Wage: 150, Personality: "violent"},
	)
	w.Crew.NextID = 2
	other := w.CityOrder[1]

	m.Update(key("t")) // dashboard: a hint, not the picker
	if m.mode != modePlay || !strings.Contains(m.status, "routes") {
		t.Fatalf("t on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("4"))
	m.crewCursor = 0
	m.Update(key("t")) // a runner
	if m.mode != modePlay || !strings.Contains(m.status, "Only a lieutenant") {
		t.Fatalf("t on a runner: mode %v status %q", m.mode, m.status)
	}
	view := m.View()
	if !strings.Contains(view, "no city") || strings.Contains(view, "violent") || strings.Contains(view, "temper") {
		t.Fatalf("crew screen before assigning:\n%s", view)
	}
	m.crewCursor = 1
	m.Update(key("t"))
	if m.mode != modeAssign {
		t.Fatalf("mode after t on a lieutenant = %v (%s)", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "assign picker")
	m.Update(key("2")) // the second city
	if m.mode != modePlay {
		t.Fatalf("mode after picking = %v", m.mode)
	}
	if lt := w.Crew.Lieutenant(other); lt == nil || lt.ID != 2 || lt.Assigned != w.Day {
		t.Fatalf("after assigning: %+v (%s)", w.Crew.Members[1], m.status)
	}
	view = m.View()
	if !strings.Contains(view, "runs "+w.CityName(other)) || !strings.Contains(view, "temper unknown for 10 more days") || strings.Contains(view, "Bayport ?") {
		t.Fatalf("crew screen after assigning:\n%s", view)
	}
	assertFits(t, view, 80, 24, "crew screen with a lieutenant")
	m.Update(key("1"))
	if view := m.View(); !strings.Contains(view, "Marcus runs "+w.CityName(other)) {
		t.Fatalf("dashboard after assigning:\n%s", view)
	}
	// The night's report says what they did, and the temper shows once
	// observed.
	endDay(t, m)
	if m.mode != modeReport {
		t.Fatalf("mode after the day = %v", m.mode)
	}
	found := false
	for _, l := range w.Report.Crew {
		found = found || strings.Contains(l, "Marcus runs "+w.CityName(other))
	}
	if !found {
		t.Fatalf("report crew lines: %v", w.Report.Crew)
	}
	m.Update(key("enter"))
	w.Crew.Member(2).Observed = true
	m.Update(key("4"))
	if view := m.View(); !strings.Contains(view, "violent") || strings.Contains(view, "temper unknown") {
		t.Fatalf("crew screen with the temper observed:\n%s", view)
	}
	// And back off the city: the last row of the picker.
	m.crewCursor = 1
	m.Update(key("t"))
	m.Update(key("down"))
	m.Update(key("down"))
	m.Update(key("enter"))
	if w.Crew.Lieutenant(other) != nil || w.Crew.Member(2).City != "" {
		t.Fatalf("after unassigning: %+v (%s)", *w.Crew.Member(2), m.status)
	}
	assertFits(t, m.View(), 80, 24, "crew screen after unassigning")
}

// TestCrewScreenAccountantIsNotIdle: an accountant works every front you
// own and has no post, so the status column says so instead of idle and
// the idle footer does not count them (#62).
func TestCrewScreenAccountantIsNotIdle(t *testing.T) {
	m := newTestModel(t, 100, 30)
	fronts := m.cfg.Laundering.Fronts
	m.w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Nadia", Role: "accountant", Skill: 50, Loyalty: 70, Nerve: 50, Wage: 60}}
	m.Update(key("4"))
	m.Update(key(" ")) // the pane cuts the crew's long lines at 100 columns; #86 moves them into it

	m.w.Fronts = []game.Front{{ID: fronts[0].ID, Name: fronts[0].Name}}
	v := m.View()
	if !strings.Contains(v, "the books") {
		t.Fatalf("one front: no 'the books' on the accountant's row:\n%s", v)
	}
	if strings.Contains(v, "idle") {
		t.Fatalf("one front: the accountant is called idle:\n%s", v)
	}
	tun := m.cfg.Laundering.Laundering
	want := fmt.Sprintf("+%s/day through each front, audit risk cut %.0f%%", money(int(tun.AccountantThroughput*0.5)), tun.AccountantRiskCut*0.5*100)
	if !strings.Contains(v, want) {
		t.Fatalf("one front: footer does not say %q:\n%s", want, v)
	}

	m.w.Fronts = append(m.w.Fronts, game.Front{ID: fronts[1].ID, Name: fronts[1].Name})
	if v := m.View(); !strings.Contains(v, "2 fronts") || strings.Contains(v, "idle") {
		t.Fatalf("two fronts: want '2 fronts' and no idle:\n%s", v)
	}

	m.w.Fronts = nil
	v = m.View()
	if !strings.Contains(v, "no front") || !strings.Contains(v, "An accountant with no front is a wage") {
		t.Fatalf("no front: want the warning:\n%s", v)
	}
	if strings.Contains(v, "idle") || strings.Contains(v, "the books") {
		t.Fatalf("no front: accountant is idle or on the books:\n%s", v)
	}
}

// TestCrewScreenUnpostedEnforcer: an enforcer without a corner is unposted
// and the hint names guarding; a runner without one is idle with the map
// hint; the idle count is runners only (#62).
func TestCrewScreenUnpostedEnforcer(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Moose", Role: "enforcer", Skill: 70, Loyalty: 70, Nerve: 60, Wage: 65}}
	m.Update(key("4"))
	m.Update(key(" ")) // the pane cuts the crew's long lines at 100 columns; #86 moves them into it
	v := m.View()
	if !strings.Contains(v, "unposted") || !strings.Contains(v, "post them on a corner to guard it") {
		t.Fatalf("enforcer: want 'unposted' and the guarding hint:\n%s", v)
	}
	if strings.Contains(v, "idle") {
		t.Fatalf("enforcer: called idle:\n%s", v)
	}

	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 2, Name: "Ray", Role: "runner", Skill: 40, Loyalty: 70, Nerve: 50, Wage: 40, Units: 20})
	v = m.View()
	if !strings.Contains(v, "idle") || !strings.Contains(v, "1 idle: a runner earns nothing off a corner. Post them on the map (5).") {
		t.Fatalf("runner: want 'idle' and the map hint counting one runner:\n%s", v)
	}
	if !strings.Contains(v, "1 unposted") {
		t.Fatalf("runner: enforcer no longer unposted:\n%s", v)
	}

	// The hiring pool says what each role does.
	m.w.Crew.Candidates = []game.CrewMember{
		{ID: 3, Name: "Pat", Role: "accountant", Skill: 50, Loyalty: 60, Nerve: 50, Wage: 60, Fee: 100},
		{ID: 4, Name: "Bo", Role: "enforcer", Skill: 50, Loyalty: 60, Nerve: 50, Wage: 60, Fee: 100},
	}
	v = m.View()
	if !strings.Contains(v, "works fronts") || !strings.Contains(v, "guards corner") {
		t.Fatalf("pool: want the role blurbs:\n%s", v)
	}
}

// f funds the city from the ledger and nowhere else: it wants clean
// cash, the dialog turns between the cities and takes an amount, the
// gift lands as goodwill overnight and the report says so; the dashboard
// carries the LAW panel with the chief, the DA and the pressure bar.
func TestFundKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	view := stripANSI(m.View())
	for _, want := range []string{"LAW", "Chief " + w.Law.Chief.Name, "DA " + w.Law.DA.Name, "pressure", "election in"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("f"))
	if m.mode != modePlay || !strings.Contains(m.status, "ledger") {
		t.Fatalf("f on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("7"))
	m.Update(key("f"))
	if m.mode != modePlay || !strings.Contains(m.status, "clean cash") {
		t.Fatalf("f with no clean cash: mode %v status %q", m.mode, m.status)
	}
	w.Player.CleanCash = 30_000
	m.Update(key("f"))
	if m.mode != modeFund {
		t.Fatalf("f on the ledger: mode %v", m.mode)
	}
	assertFits(t, m.View(), 80, 24, "fund dialog")
	m.Update(key("right"))
	if c := m.fundCity(); c.ID != w.CityOrder[1] {
		t.Fatalf("right did not turn to the other city: %s", c.ID)
	}
	m.Update(key("left"))
	if c := m.fundCity(); c.ID != w.Player.Location {
		t.Fatalf("left did not turn back: %s", c.ID)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || len(w.Funded) != 0 {
		t.Fatal("esc funded")
	}
	m.Update(key("f"))
	for _, r := range "5000" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modePlay || w.Player.CleanCash != 25_000 || w.FundedToday(w.Player.Location) != 5_000 || !strings.Contains(m.status, "Goodwill") {
		t.Fatalf("enter: mode %v clean %d funded %d status %q", m.mode, w.Player.CleanCash, w.FundedToday(w.Player.Location), m.status)
	}
	// Blank fills goodwill to 100, capped by the clean cash in hand.
	m.Update(key("f"))
	m.Update(key("enter"))
	if w.Player.CleanCash != 0 || w.FundedToday(w.Player.Location) != 30_000 {
		t.Fatalf("blank amount: clean %d funded %d status %q", w.Player.CleanCash, w.FundedToday(w.Player.Location), m.status)
	}
	m.Update(key("1"))
	endDay(t, m)
	if m.mode != modeReport {
		t.Fatalf("mode %v", m.mode)
	}
	law := strings.Join(w.Report.Law, "\n")
	if !strings.Contains(law, "goodwill") || w.Here().Goodwill <= 0 || w.Stats.Funded != 30_000 {
		t.Fatalf("report %v goodwill %.1f stats %d", w.Report.Law, w.Here().Goodwill, w.Stats.Funded)
	}
	if !strings.Contains(strings.Join(w.Report.Money, "\n"), "Funded") {
		t.Fatalf("money: %v", w.Report.Money)
	}
	assertFits(t, m.View(), 80, 24, "report with a gift")
	m.Update(key("enter"))
	if !strings.Contains(stripANSI(m.View()), "goodwill") {
		t.Fatal("dashboard does not show the goodwill")
	}
}
