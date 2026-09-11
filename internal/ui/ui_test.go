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
	"github.com/theclifmeister/kingpin/internal/ui/theme"
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
		// line: the top-right corner then starts a line of its own, a
		// one-cell overflow leaves a lone corner, and a body line loses
		// its closing bar.
		p := strings.TrimSpace(stripANSI(l))
		switch {
		case strings.HasPrefix(p, "═") && strings.HasSuffix(p, "╗") && !strings.HasPrefix(p, "╔"),
			p == "╗" || p == "╝",
			strings.Contains(p, "║") && !strings.HasSuffix(p, "║"):
			t.Errorf("%s: a modal wider than %d wrapped at line %d: %q", what, w, i, p)
		}
	}
}

// assertFrame checks the three-part frame every play-mode screen
// renders into: row 0 the title bar, rows 1..h-2 the body, row h-1 the
// status bar directly under it, and no ticker row (#95); from 100
// columns every body row ends in the pane's border column and the
// pane's last section is KEYS; under that the body is MAIN over the
// details strip on row h-2.
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
	if plain[h-1] != stripANSI(m.viewFooter()) {
		t.Errorf("%s: row %d is not the status bar: %q", what, h-1, plain[h-1])
	}
	if want := h - 2; m.bodyHeight() != want {
		t.Errorf("%s: the body is %d rows, want %d", what, m.bodyHeight(), want)
	}
	switch {
	case m.paneShown():
		if want := h - 2; m.mainHeight() != want {
			t.Errorf("%s: MAIN is %d rows beside the pane, want %d", what, m.mainHeight(), want)
		}
		keysAt := -1
		for i := 1; i <= h-2; i++ {
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
		for _, b := range m.paneKeys() {
			keys = append(keys, b.key)
		}
		for i := keysAt + 1; i < h-2; i++ {
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
		if want := h - 3; m.mainHeight() != want {
			t.Errorf("%s: MAIN is %d rows over the strip, want %d", what, m.mainHeight(), want)
		}
		strip := strings.TrimRight(plain[h-2], " ")
		if !strings.HasPrefix(strip, "▸ ") || !strings.HasSuffix(strip, "␣ more") {
			t.Errorf("%s: row %d is not the details strip: %q", what, h-2, strip)
		}
	}
}

// richFixture plays one run through every screen, dialog and picker the
// game has, with a crew, a rival at war, a route with a shipment in
// flight, three fronts, a billion in the bank and the whole ladder, and
// hands every view it reaches to check, with the model and a name
// for it.
func richFixture(t *testing.T, sz [2]int, check func(m *Model, view, what string)) {
	t.Helper()
	handed := check
	see := func(m *Model, what string) { handed(m, m.View(), what) }
	m := newTestModel(t, sz[0], sz[1])
	// Play a few days with some trading so every panel has content.
	for i := 0; i < 5; i++ {
		m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 40
		m.Update(key("s"))
		m.Update(key("enter")) // product
		m.Update(key("enter")) // qty (blank = all)
		m.Update(key("3"))     // aggressive
		see(m, "sell dial")
		m.Update(key("enter")) // confirm
		see(m, "sell dialog with the cart")
		m.Update(key("esc")) // the dialog stays open for the next line (#103)
		endDay(t, m)         // end day -> report
		see(m, "report")
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
		see(m, "post picker")
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
	see(m, "strike picker")
	m.Update(key("enter"))
	if m.w.Strike == nil {
		t.Fatalf("%dx%d: no strike queued: %q", sz[0], sz[1], m.status)
	}
	for i := range m.shown().Corners {
		m.mapCursor = i
		see(m, "map")
	}
	// The other city's map, and the ship dialog with something to send.
	m.Update(key("]"))
	for i := range m.shown().Corners {
		m.mapCursor = i
		see(m, "map elsewhere")
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
	see(m, "map with the routes cursor")
	m.Update(key("R"))
	see(m, "target product")
	m.Update(key("enter"))
	for _, r := range "120" {
		m.Update(key(string(r)))
	}
	see(m, "target units")
	m.Update(key("enter"))
	if m.mode != modePlay || !m.w.Route(route.ID).Dial.On() || m.w.Route(route.ID).Target[m.w.Products[0]] != 120 {
		t.Fatalf("%dx%d: the target dialog left mode %v with %+v: %q %q", sz[0], sz[1], m.mode, m.w.Route(route.ID), m.status, m.tgt.err)
	}
	endDay(t, m)
	see(m, "report with the route")
	m.Update(key("enter"))
	if len(m.w.Shipments) != 1 {
		t.Fatalf("%dx%d: the route sent %d shipments: %v", sz[0], sz[1], len(m.w.Shipments), m.w.Report.Shipments)
	}
	see(m, "map with a shipment in flight")
	m.Update(key("["))
	m.Update(key("g"))
	see(m, "travel confirm")
	m.Update(key("esc"))
	for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		m.Update(key(s))
		see(m, "screen "+s)
		// Space: the overlay where the strip is, the pane hidden and
		// shown again where it sits beside MAIN.
		m.Update(key(" "))
		if sz[0] < paneMinWidth {
			if m.mode != modeDetails {
				t.Fatalf("%dx%d: space on screen %s: mode %v", sz[0], sz[1], s, m.mode)
			}
			see(m, "details overlay "+s)
			m.Update(key("esc"))
		} else {
			if m.mode != modePlay || !m.paneHidden {
				t.Fatalf("%dx%d: space on screen %s: mode %v hidden %v", sz[0], sz[1], s, m.mode, m.paneHidden)
			}
			see(m, "screen "+s+" with the pane hidden")
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
	see(m, "rivals screen")
	m.Update(key("d"))
	see(m, "propose kinds")
	m.Update(key("2"))
	see(m, "propose terms")
	m.Update(key("enter"))
	if m.w.Proposal == nil || m.w.Proposal.Kind != game.DealTribute {
		t.Fatalf("%dx%d: no tribute proposed: %q", sz[0], sz[1], m.status)
	}
	see(m, "rivals screen with a proposal")
	m.Update(key("1"))
	see(m, "dashboard with the table")
	m.w.Rival.Deals, m.w.Offers, m.w.Proposal = nil, nil, nil
	// Every node state on the tree: owned, available, short, locked,
	// and the buy confirmation.
	m.Update(key("6"))
	m.w.Player.DirtyCash += 20_000
	m.Update(key("enter"))
	see(m, "upgrade confirm")
	m.Update(key("y"))
	for range m.upgradeRows() {
		see(m, "upgrades")
		m.Update(key("j"))
	}
	m.Update(key("4"))
	m.Update(key("f"))
	see(m, "fire confirm")
	m.Update(key("y"))
	m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 200
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc"))
	// A robbery for the report. The stick-up rolls only on a corner
	// you hold with somebody on it, and test seeds are wall-clock:
	// the rival, at war and bordering your corner, can push you off
	// it on the route day (#90), and a skill-88+ enforcer takes the
	// chance under one. So the corner is yours again, worked by you,
	// unguarded and unsqueezed before the roll; territory steps
	// before rivals, so the robbery lands whatever the rival does.
	start := m.w.Corner(m.cfg.City.Territory.Start)
	m.w.Recall(game.You)
	start.Owner, start.Runner, start.Enforcer, start.Squeeze, start.Risk = game.OwnerPlayer, game.You, 0, 0, 100
	endDay(t, m)
	see(m, "report with crew")
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
	see(m, "front picker")
	for i := 0; i < 3; i++ {
		m.Update(key("b"))
		m.Update(key("enter"))
	}
	if len(m.w.Fronts) != 3 {
		t.Fatalf("%dx%d: bought %d fronts: %q", sz[0], sz[1], len(m.w.Fronts), m.status)
	}
	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 901, Name: "Books", Role: "accountant", Skill: 60, Loyalty: 60, Wage: 130})
	endDay(t, m)
	see(m, "report with fronts")
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
		see(m, "ledger screen "+s)
	}
	// The ledger's cursor over every front, route and offer (#87), so
	// each row's pane section is rendered.
	m.Update(key("7"))
	for range m.ledgerRows() {
		see(m, "ledger row")
		m.Update(key("j"))
	}
	for range m.ledgerRows() {
		m.Update(key("k"))
	}
	m.Update(key("1"))
	m.Update(key("b"))
	see(m, "buy dialog")
	m.Update(key("enter"))
	see(m, "buy qty")
	m.Update(key("esc"))
	m.Update(key("esc"))
	m.Update(key("?"))
	see(m, "help")
	m.Update(key("x"))

	// A cartel-scale world: ten-digit cash on every screen, then the
	// money moved clean (dirty cash on that scale is an arrest) so the
	// next day unlocks every rung of the product ladder.
	m.w.Player.DirtyCash = 1_234_567_890
	m.w.Stats.PeakCash = m.w.Player.DirtyCash
	for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		m.Update(key(s))
		see(m, "rich screen "+s)
	}
	m.Update(key("b"))
	see(m, "rich front picker")
	m.Update(key("esc"))
	m.Update(key("1"))
	m.Update(key("b"))
	m.Update(key("enter"))
	see(m, "rich buy qty")
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
		see(m, "ladder screen "+s)
	}
	m.Update(key("1"))
	m.Update(key("s"))
	for range m.w.Products {
		m.Update(key("j"))
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	see(m, "ladder sell dialog")
	m.Update(key("enter"))
	see(m, "ladder sell dialog with the cart")
	m.Update(key("esc"))
	endDay(t, m)
	see(m, "ladder report")
	m.Update(key("enter"))
	m.w.Over = &game.Ending{Day: m.w.Day, Cause: "indicted", PeakCash: m.w.Stats.PeakCash}
	endDay(t, m)
	see(m, "rich game over")
}

func TestRendersAtCommonSizes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {100, 30}} {
		richFixture(t, sz, func(m *Model, view, what string) {
			assertFits(t, view, sz[0], sz[1], what)
			if m.mode == modePlay {
				assertFrame(t, m, what)
			}
			checkMap(t, m, view, what)
		})
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
	// A table column is one of the three, by kind.
	for _, c := range []struct {
		kind colKind
		v    any
		want string
	}{{kCash, 1_234_567, cash(1_234_567)}, {kMoney, 1_234_567, money(1_234_567)}, {kPrice, 19.5, price(19.5)}, {kCash, 9_999, "$9,999"}, {kMoney, 9_999, "$9,999"}} {
		if got, _ := cellText(c.kind, 0, c.v); got != c.want {
			t.Errorf("%s cell of %v = %q, want %q", kindName(c.kind), c.v, got, c.want)
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
	if m.mode != modeBuy || m.dlg.step != 0 || m.dlg.err != "" {
		t.Fatalf("buy did not complete: mode=%v step=%d err=%q", m.mode, m.dlg.step, m.dlg.err)
	}
	m.Update(key("esc")) // the dialog stays open for the next line (#103)
	if m.mode != modePlay {
		t.Fatalf("esc did not close the dialog: mode=%v", m.mode)
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
	m.Update(key("esc"))
	if o, ok := m.w.Order(m.w.Player.Location, id); !ok || o.Qty != 10 {
		t.Fatalf("order not placed: %+v", m.w.Orders)
	}
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != 1 {
		t.Fatalf("end day: mode=%v day=%d", m.mode, m.w.Day)
	}
}

// The status bar never drops the message: at 80 columns a long status
// set after a key is on row h-1 on every screen, `? help` giving way.
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

// statusBarShape is what the status bar must be in play mode (#109):
// the message at the left, `? help` at the right, the row exactly the
// width, and nothing else; with no message the row is `? help` alone;
// a message too long to share the row shows alone, cut only when it
// alone does not fit.
func statusBarShape(t *testing.T, m *Model, what string) {
	t.Helper()
	got := stripANSI(m.viewFooter())
	w := m.width
	help := stripANSI(m.helpPair())
	if lipgloss.Width(got) != w {
		t.Errorf("%s: the status bar is %d cells, want %d: %q", what, lipgloss.Width(got), w, got)
	}
	if help != " ? help " {
		t.Errorf("%s: the help pair is %q", what, help)
	}
	switch {
	case m.status == "":
		if want := strings.Repeat(" ", w-len(help)) + help; got != want {
			t.Errorf("%s: the empty bar is %q, want %q", what, got, want)
		}
	case 1+len([]rune(m.status))+len(help) <= w:
		if want := " " + m.status + strings.Repeat(" ", w-1-len([]rune(m.status))-len(help)) + help; got != want {
			t.Errorf("%s: the bar is %q, want %q", what, got, want)
		}
	default:
		if want := truncate(" "+m.status, w); got != want {
			t.Errorf("%s: the long message is %q, want %q", what, got, want)
		}
		if strings.Contains(got, "? help") {
			t.Errorf("%s: the legend shares the row with a message too long for it: %q", what, got)
		}
	}
}

// The status bar is the message and `? help` at every width, on every
// screen, with and without a message: no legend, no pair but the last
// (#109; #85's whole-pair legend is gone with the legend).
func TestStatusBarIsMessageAndHelp(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		m := newTestModel(t, w, 24)
		for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
			m.Update(key(s))
			for _, msg := range []string{"", "Bought 10 units.", "Pay generous, $108/day. Loyalty climbs. The crew notice it too.", strings.Repeat("Somebody is talking. ", 8)} {
				m.status = msg
				statusBarShape(t, m, fmt.Sprintf("%d columns, screen %s, status %q", w, s, msg))
			}
		}
	}
}

// The status bar of the rich fixture on every play-mode screen at 80x24
// and 120x40 is the message (or nothing) and `? help`, and nothing
// else; inside a modal it is the modal's footer.
func TestStatusBarIsTheMessage(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		richFixture(t, sz, func(m *Model, view, what string) {
			rows := strings.Split(view, "\n")
			bar := stripANSI(rows[len(rows)-1])
			if m.mode != modePlay {
				if want := stripANSI(fit(legend(m.modalFooter()), m.width)); bar != want {
					t.Errorf("%dx%d %s: the bar in a modal is %q, want the footer %q", sz[0], sz[1], what, bar, want)
				}
				return
			}
			statusBarShape(t, m, fmt.Sprintf("%dx%d %s", sz[0], sz[1], what))
			if bar != stripANSI(m.viewFooter()) {
				t.Errorf("%dx%d %s: the last row %q is not the status bar", sz[0], sz[1], what, bar)
			}
		})
	}
}

// Space opens the details as an overlay where the strip is, with the
// sections the pane shows beside MAIN at 120, and esc closes it; at 120
// space hides the pane and the main content takes the width.
func TestSpaceTogglesDetails(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("5"))
	secs := m.details()
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

// The UI redraws only on a key or a resize (#95): nothing in the frame
// moves without you, so Init starts no ticker.
func TestInitStartsNothing(t *testing.T) {
	m := newTestModel(t, 80, 24)
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init returned a command")
	}
}

// The title bar says when there is news you have not read: headlines
// that land after the journal was last shown count on its tab, in the
// news accent, and the count clears on entering the screen. The count
// goes with the tab's name, never widening the bar past the terminal,
// and the digits-only bar carries none.
func TestJournalUnreadCount(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.Update(key("3"))
	m.Update(key("1"))
	if n := m.journalUnread(); n != 0 {
		t.Fatalf("%d unread before any news landed", n)
	}
	for _, text := range []string{"one", "two", "three"} {
		m.w.Journal = append(m.w.Journal, game.Headline{Day: m.w.Day, Source: "heat", Text: text})
	}
	title := m.viewTitle()
	if !strings.Contains(stripANSI(title), "3 Journal 3 ") {
		t.Fatalf("three unread headlines do not read Journal 3: %q", stripANSI(title))
	}
	if !strings.Contains(title, theme.NewsText.Render("3")) {
		t.Fatalf("the count is not in the news accent: %q", title)
	}
	if lw := lipgloss.Width(title); lw > 120 {
		t.Fatalf("the title bar is %d wide", lw)
	}
	// The count goes with the name: the digits-only bar carries none.
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	if got := stripANSI(m.viewTitle()); lipgloss.Width(got) > 40 || strings.Contains(got, "Journal") || strings.Contains(got, "3 3") {
		t.Fatalf("the digits-only bar carries the count or is too wide: %q", got)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Entering the journal reads it.
	m.Update(key("3"))
	if got := stripANSI(m.View()); !strings.Contains(got, "3 Journal ") || strings.Contains(got, "Journal 3") {
		t.Fatalf("the count did not clear on the journal screen:\n%s", got)
	}
	m.Update(key("1"))
	if got := stripANSI(m.viewTitle()); strings.Contains(got, "Journal 3") {
		t.Fatalf("the count came back after the journal was shown: %q", got)
	}
	// More news counts again, and a new run starts with none.
	m.w.Journal = append(m.w.Journal, game.Headline{Day: m.w.Day, Source: "crew", Text: "four"})
	if got := stripANSI(m.viewTitle()); !strings.Contains(got, "Journal 1 ") {
		t.Fatalf("a new headline does not count: %q", got)
	}
	m.newRun()
	if m.journalUnread() != 0 {
		t.Fatalf("a new run starts with %d unread", m.journalUnread())
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
	m.Update(key("3"))
	m.Update(key("c"))
	if m.mode != modePlay || !strings.Contains(m.status, "map") {
		t.Fatalf("c on the journal: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("1"))
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
	m.Update(key("esc"))
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
	// t ships nothing by hand any more: it points at the crew screen.
	m.Update(key("t"))
	if m.mode != modePlay || m.status != "Assign on the crew screen (4)." {
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
	if v := stripANSI(m.View()); !strings.Contains(v, "CITIES") || !strings.Contains(v, "◂ 30 units in") {
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
	m.Update(key("esc"))
	if m.mode != modePlay || w.Stock(hub, product) <= 20 || w.Stock(home, product) != stock {
		t.Fatalf("buy elsewhere: mode %v err %q hub %d home %d", m.mode, m.dlg.err, w.Stock(hub, product), w.Stock(home, product))
	}
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc"))
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
	m.Update(key("esc"))
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
	if m.mode != modePlay || !strings.Contains(m.status, "nobody is contesting") || m.statusKind != statusWarning {
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

// f funds the city from the ledger and nowhere else: it wants clean
// cash, the dialog turns between the cities and takes an amount, the
// gift lands as goodwill overnight and the report says so; the dashboard
// carries the LAW panel with the chief, the DA and the pressure bar.
func TestFundKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	view := stripANSI(m.View())
	for _, want := range []string{"LAW", "Chief " + w.Law.Chief.Name, "DA " + w.Law.DA.Name, "pressure"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard lacks %q:\n%s", want, view)
		}
	}
	// The election countdown needs the wide LAW panel (#83): `election
	// in 90d`, or `election 90d` under a long name on a long ticket.
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if view := stripANSI(m.View()); !strings.Contains(view, "election") {
		t.Fatalf("wide dashboard lacks the election countdown:\n%s", view)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
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

// richModel is a run with something behind every modal: a report with
// sales, a crew posted across the map with a lieutenant and an
// accountant, the rival in town at war, a route on with a shipment in
// flight, fronts on the ledger, a card, a deal, an offer and cash for
// the tree.
func richModel(t *testing.T, w, h int) *Model {
	t.Helper()
	return richModelSeeded(t, w, h, game.NewSeed())
}

// richModelSeeded is richModel on a seed of the caller's: the same run
// every time, which is what the README's captures are diffed against.
func richModelSeeded(t *testing.T, w, h int, seed uint64) *Model {
	t.Helper()
	m := newTestModel(t, w, h)
	m.startRun(seed)
	world := m.w
	for i := 0; i < 3; i++ {
		world.Stash(world.Player.Location)[world.Products[0]] = 40
		m.Update(key("s"))
		m.Update(key("enter")) // product
		m.Update(key("enter")) // qty (blank = all)
		m.Update(key("3"))     // aggressive
		m.Update(key("enter")) // confirm
		m.Update(key("esc"))   // the dialog stays open for the next line (#103)
		endDay(t, m)
		m.Update(key("enter"))
	}
	world.Player.DirtyCash = 700_000
	world.Stats.PeakCash = 700_000
	world.Crew.Members = append(world.Crew.Members,
		game.CrewMember{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 80, Nerve: 50, Wage: 50},
		game.CrewMember{ID: 2, Name: "Gato", Role: "runner", Skill: 40, Units: 90, Loyalty: 40, Nerve: 30, Wage: 45},
		game.CrewMember{ID: 3, Name: "Moose", Role: "enforcer", Skill: 70, Loyalty: 70, Nerve: 60, Wage: 65},
		game.CrewMember{ID: 4, Name: "Vasquez", Role: game.RoleLieutenant, Skill: 70, Loyalty: 80, Nerve: 50, Wage: 150, Personality: "steady"},
		game.CrewMember{ID: 5, Name: "Books", Role: "accountant", Skill: 60, Loyalty: 60, Wage: 130},
	)
	world.Crew.NextID = 5
	world.Crew.LastSkim = world.Day
	home := world.Home()
	if err := world.Post(home.Corners[1].ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := world.Post(home.Corners[2].ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := world.Post(home.Corners[2].ID, 3); err != nil {
		t.Fatal(err)
	}
	home.Corners[0].Owner, home.Corners[0].Runner, home.Corners[0].Enforcer = game.OwnerRival, 0, 0
	world.Rival.Arrived, world.Rival.Muscle, world.Rival.War, world.Rival.Observed = 1, 4, 47, true
	world.Rival.Deals = []game.Deal{{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{home.Corners[1].ID}}, Since: world.Day}}
	world.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Offered: true}, Expires: world.Day + 4}}
	// A route on with a target, and a day for it to send a shipment.
	route := m.set.Logistics.Routes(world.CityOrder[1])[0]
	m.cfg.Routes.Routes[0].Risk = 0
	world.Stash(route.From)[world.Products[0]] = 300
	if err := world.SetRoute(route.ID, events.RouteNormal); err != nil {
		t.Fatal(err)
	}
	if err := world.SetRouteTarget(route.ID, world.Products[0], 120); err != nil {
		t.Fatal(err)
	}
	world.Stash(world.Player.Location)[world.Products[0]] = 60
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc"))
	endDay(t, m)
	m.Update(key("enter"))
	if len(world.Shipments) != 1 {
		t.Fatalf("the route sent %d shipments: %v", len(world.Shipments), world.Report.Shipments)
	}
	// Three fronts, one of them audited.
	m.Update(key("7"))
	for i := 0; i < 3; i++ {
		m.Update(key("b"))
		m.Update(key("enter"))
	}
	if len(world.Fronts) != 3 {
		t.Fatalf("bought %d fronts: %q", len(world.Fronts), m.status)
	}
	world.Fronts[1].Audited = world.Day
	world.Fronts[1].FrozenUntil = world.Day + m.cfg.Laundering.Laundering.AuditFreezeDays
	world.Player.CleanCash = 50_000
	world.Stash(world.Player.Location)[world.Products[0]] = 40 // something to sell
	m.Update(key("1"))
	m.cursor, m.crewCursor, m.mapCursor, m.upgradeCursor = 0, 0, 0, 0
	m.status = ""
	return m
}

// fillCart lines up two buys and two orders in one visit each (#103),
// so the cart, the dialogs and the panes have a full cart to show.
func fillCart(t *testing.T, m *Model) {
	t.Helper()
	w := m.w
	w.Stash(w.Player.Location)[w.Products[1]] = 20
	m.Update(key("1"))
	m.Update(key("b"))
	for _, k := range []string{"enter", "5", "enter", "j", "enter", "3", "enter", "esc"} {
		m.Update(key(k))
	}
	m.Update(key("s"))
	for _, k := range []string{"1", "enter", "enter", "3", "enter", "2", "enter", "enter", "1", "enter", "esc"} {
		m.Update(key(k))
	}
	if m.mode != modePlay || len(w.Buys) != 2 || len(w.Orders) != 2 {
		t.Fatalf("filling the cart: mode %v, %d buys, %d orders, %q %q", m.mode, len(w.Buys), len(w.Orders), m.dlg.err, m.status)
	}
}

// testCard is a dilemma card for the fixture; its prose wraps.
func testCard(day int) *game.Card {
	return &game.Card{ID: "test", Day: day, Title: "A test", Text: strings.Repeat("A long question about what to do next. ", 6),
		Choices: []game.Choice{
			{Label: "Pay", Outcome: "You paid. " + strings.Repeat("And that was the end of it. ", 4), Effects: map[string]float64{"dirty_cash": -100}},
			{Label: "Shout", Outcome: "You shouted.", Effects: map[string]float64{"heat": 7}},
			{Label: "Walk away", Outcome: "You walked."},
		}}
}

// modalBox finds the modal in a view: its top border's line index, its
// width, and its lines from the top border to the bottom one.
func modalBox(t *testing.T, view string) (top, width int, box []string) {
	t.Helper()
	ls := strings.Split(view, "\n")
	top = -1
	for i, l := range ls {
		p := strings.TrimSpace(stripANSI(l))
		switch {
		case strings.HasPrefix(p, "╔"):
			if top >= 0 {
				t.Fatalf("two modals in the view:\n%s", stripANSI(view))
			}
			top, width = i, lipgloss.Width(p)
		case strings.HasPrefix(p, "╚"):
			if top < 0 {
				t.Fatalf("a bottom border with no top:\n%s", stripANSI(view))
			}
			box = ls[top : i+1]
			return top, width, box
		}
	}
	t.Fatalf("no modal in the view:\n%s", stripANSI(view))
	return
}

// Every mode's modal is the one modal (#81): min(width-4, 76) wide, on
// body row 2, a footer line of the mode's bindings, and the status bar
// repeating that footer. The table opens every mode in the enum from the
// rich fixture, the dialogs on each of their steps.
func TestModalsFit(t *testing.T) {
	type open struct {
		name string
		mode mode
		open func(t *testing.T, m *Model)
	}
	cases := []open{
		{"start", modeStart, func(t *testing.T, m *Model) { m.mode = modeStart }},
		{"report", modeReport, func(t *testing.T, m *Model) { m.mode = modeReport }},
		{"buy product", modeBuy, func(t *testing.T, m *Model) { m.Update(key("b")) }},
		{"buy quantity", modeBuy, func(t *testing.T, m *Model) { m.Update(key("b")); m.Update(key("enter")) }},
		{"sell product", modeSell, func(t *testing.T, m *Model) { m.Update(key("s")) }},
		{"sell quantity", modeSell, func(t *testing.T, m *Model) { m.Update(key("s")); m.Update(key("enter")) }},
		{"sell dial", modeSell, func(t *testing.T, m *Model) {
			m.Update(key("s"))
			m.Update(key("enter"))
			m.Update(key("enter"))
			m.Update(key("3"))
		}},
		{"game over", modeOver, func(t *testing.T, m *Model) {
			m.w.Over = &game.Ending{Day: m.w.Day, Cause: "indicted", PeakCash: m.w.Stats.PeakCash}
			m.mode = modeOver
		}},
		{"confirm new", modeConfirmNew, func(t *testing.T, m *Model) { m.Update(key("N")) }},
		{"confirm fire", modeConfirmFire, func(t *testing.T, m *Model) { m.Update(key("4")); m.Update(key("f")) }},
		{"confirm end", modeConfirmEnd, func(t *testing.T, m *Model) { m.Update(key("enter")) }},
		{"help", modeHelp, func(t *testing.T, m *Model) { m.Update(key("?")) }},
		{"post", modePost, func(t *testing.T, m *Model) { m.Update(key("5")); m.mapCursor = 1; m.Update(key("c")) }},
		{"strike", modeStrike, func(t *testing.T, m *Model) { m.Update(key("5")); m.mapCursor = 0; m.Update(key("w")) }},
		{"confirm upgrade", modeConfirmUpgrade, func(t *testing.T, m *Model) { m.Update(key("6")); m.Update(key("enter")) }},
		{"front", modeFront, func(t *testing.T, m *Model) { m.Update(key("7")); m.Update(key("b")) }},
		{"confirm investigate", modeConfirmInvestigate, func(t *testing.T, m *Model) { m.Update(key("4")); m.Update(key("i")) }},
		{"confirm pay off", modeConfirmPayOff, func(t *testing.T, m *Model) { m.Update(key("4")); m.Update(key("$")) }},
		{"card", modeCard, func(t *testing.T, m *Model) { m.w.Dilemmas.Pending = testCard(m.w.Day); m.showCard() }},
		{"card outcome", modeCard, func(t *testing.T, m *Model) {
			m.w.Dilemmas.Pending = testCard(m.w.Day)
			m.showCard()
			m.Update(key("enter"))
			if !m.cardDone {
				t.Fatal("the card was not answered")
			}
		}},
		{"target product", modeTarget, func(t *testing.T, m *Model) {
			m.Update(key("5"))
			m.Update(key("]"))
			m.onRoutes = true
			m.Update(key("R"))
		}},
		{"target units", modeTarget, func(t *testing.T, m *Model) {
			m.Update(key("5"))
			m.Update(key("]"))
			m.onRoutes = true
			m.Update(key("R"))
			m.Update(key("enter"))
		}},
		{"confirm travel", modeConfirmTravel, func(t *testing.T, m *Model) { m.Update(key("g")) }},
		{"propose kinds", modePropose, func(t *testing.T, m *Model) { m.Update(key("8")); m.Update(key("d")) }},
		{"propose terms", modePropose, func(t *testing.T, m *Model) { m.Update(key("8")); m.Update(key("d")); m.Update(key("2")) }},
		{"assign", modeAssign, func(t *testing.T, m *Model) { m.Update(key("4")); m.crewCursor = 3; m.Update(key("t")) }},
		{"fund", modeFund, func(t *testing.T, m *Model) { m.Update(key("7")); m.Update(key("f")) }},
		{"details", modeDetails, func(t *testing.T, m *Model) { m.Update(key("5")); m.mode = modeDetails }},
		// The cart (#103): the modal on its lines and on a quantity, and
		// the dialogs with a full cart under the table.
		{"cart", modeCart, func(t *testing.T, m *Model) { fillCart(t, m); m.Update(key("c")) }},
		{"cart quantity", modeCart, func(t *testing.T, m *Model) { fillCart(t, m); m.Update(key("c")); m.Update(key("enter")) }},
		{"buy with cart", modeBuy, func(t *testing.T, m *Model) { fillCart(t, m); m.Update(key("b")) }},
		{"sell with cart", modeSell, func(t *testing.T, m *Model) { fillCart(t, m); m.Update(key("s")) }},
	}
	covered := map[mode]bool{}
	for _, c := range cases {
		covered[c.mode] = true
	}
	for md := modeStart; md < modeCount; md++ {
		if md != modePlay && !covered[md] {
			t.Errorf("mode %d has no case in the table", md)
		}
	}
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		for _, c := range cases {
			m := richModel(t, sz[0], sz[1])
			c.open(t, m)
			if m.mode != c.mode {
				t.Fatalf("%dx%d %s: mode %v, not %v: %q", sz[0], sz[1], c.name, m.mode, c.mode, m.status)
			}
			view := m.View()
			what := fmt.Sprintf("%dx%d %s", sz[0], sz[1], c.name)
			assertFits(t, view, sz[0], sz[1], what)
			top, width, box := modalBox(t, view)
			if top != 2 {
				t.Errorf("%s: the modal starts on row %d, not 2:\n%s", what, top, stripANSI(view))
			}
			if width != want {
				t.Errorf("%s: the modal is %d wide, not %d:\n%s", what, width, want, stripANSI(view))
			}
			for i, l := range box {
				if lw := lipgloss.Width(strings.TrimSpace(stripANSI(l))); lw != want {
					t.Errorf("%s: box line %d is %d wide, not %d: %q", what, i, lw, want, stripANSI(l))
				}
			}
			// The title, a blank, the body, a blank, the footer.
			inner := func(i int) string {
				return strings.TrimSpace(strings.Trim(strings.TrimSpace(stripANSI(box[i])), "║"))
			}
			if len(box) < 6 {
				t.Fatalf("%s: a modal of %d lines", what, len(box))
			}
			if title := inner(1); title == "" || title != strings.ToUpper(title) {
				t.Errorf("%s: the title is %q", what, title)
			}
			if inner(2) != "" || inner(len(box)-3) != "" {
				t.Errorf("%s: no blank around the body:\n%s", what, stripANSI(strings.Join(box, "\n")))
			}
			foot := strings.TrimSpace(stripANSI(legend(m.modalFooter())))
			got := inner(len(box) - 2)
			if got != foot && got != foot+"  ↓ more" {
				t.Errorf("%s: the footer is %q, not %q", what, got, foot)
			}
			if c.mode != modeHelp && c.mode != modeReport && c.mode != modeDetails { // the overlay's KEYS section lists keys on purpose
				for _, l := range box[3 : len(box)-3] {
					p := stripANSI(l)
					for _, hint := range []string{"enter ", "esc ", "any other key", "any key"} {
						if strings.Contains(p, hint) {
							t.Errorf("%s: a key hint in the body: %q", what, strings.TrimSpace(p))
						}
					}
				}
			}
			if c.mode != modeStart {
				ls := strings.Split(view, "\n")
				if bar := strings.TrimSpace(stripANSI(ls[len(ls)-1])); bar != foot {
					t.Errorf("%s: the status bar shows %q, not the footer %q", what, bar, foot)
				}
			}
		}
	}
}

// A modal taller than the room scrolls instead of clamping: the report
// with thirty lines says ↓ more at 80x24, ↓ reaches its last line and
// enter closes it; help the same.
func TestReportScrolls(t *testing.T) {
	m := newTestModel(t, 80, 24)
	var news []string
	for i := 1; i <= 30; i++ {
		news = append(news, fmt.Sprintf("Headline number %d", i))
	}
	m.w.Report = &game.DayReport{Day: m.w.Day, News: news}
	for _, c := range []struct {
		name string
		open string
		last string
	}{
		{"report", "r", "Headline number 30"},
		{"help", "?", "Greed is always available."},
	} {
		m.Update(key(c.open))
		view := stripANSI(m.View())
		if !strings.Contains(view, "↓ more") || strings.Contains(view, c.last) {
			t.Fatalf("%s at 80x24 does not scroll:\n%s", c.name, view)
		}
		assertFits(t, m.View(), 80, 24, c.name)
		for i := 0; i < 60; i++ {
			m.Update(key("down"))
		}
		view = stripANSI(m.View())
		if strings.Contains(view, "↓ more") || !strings.Contains(view, "↑ more") || !strings.Contains(view, c.last) {
			t.Fatalf("%s after sixty ↓ does not show its last line:\n%s", c.name, view)
		}
		assertFits(t, m.View(), 80, 24, c.name+" scrolled")
		day := m.w.Day
		m.Update(key("enter"))
		if m.mode != modePlay || m.w.Day != day {
			t.Fatalf("enter on the %s: mode %v day %d -> %d", c.name, m.mode, day, m.w.Day)
		}
	}
}

// stripANSI removes escape sequences so a render can be read as text.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && r == 'm':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
