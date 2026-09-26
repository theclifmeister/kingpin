package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The ledger's one cursor (#87) walks the fronts, the routes and the
// offers in order and stays on the rows there are; the pane's first
// section names the row; enter turns a route's dial and opens the buy
// confirmation on an affordable offer, and on a front is the frame's.
func TestLedgerCursor(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("7"))
	if m.screen != screenLedger {
		t.Fatalf("screen %v", m.screen)
	}
	routes := m.ledgerRoutes()
	offers := m.frontRows()
	if len(w.Fronts) != 3 || len(routes) != 3 || len(offers) != 3 {
		t.Fatalf("fixture: %d fronts, %d routes, %d offers", len(w.Fronts), len(routes), len(offers))
	}
	first := func() string { return m.details()[0].title }
	m.Update(key("up")) // stays on the first front
	if got, want := first(), strings.ToUpper(w.Fronts[0].Name); got != want || m.ledgerCursor != 0 {
		t.Fatalf("at the top: %q (cursor %d), want %q", got, m.ledgerCursor, want)
	}
	// On a front enter is the frame's: it asks to end the day, and esc
	// declines.
	day := w.Day
	m.Update(key("enter"))
	if m.mode != modeConfirmEnd || w.Day != day {
		t.Fatalf("enter on a front: mode %v day %d -> %d", m.mode, day, w.Day)
	}
	m.Update(key("esc"))
	for i := 1; i < 3; i++ {
		m.Update(key("down"))
		if got, want := first(), strings.ToUpper(w.Fronts[i].Name); got != want {
			t.Fatalf("front %d: %q, want %q", i, got, want)
		}
	}
	// Onto the houses (#73): enter is the frame's there too.
	for i := range w.Houses {
		m.Update(key("down"))
		if got, want := first(), strings.ToUpper(w.Houses[i].Name); got != want {
			t.Fatalf("house %d: %q, want %q", i, got, want)
		}
	}
	m.Update(key("enter"))
	if m.mode != modeConfirmEnd || w.Day != day {
		t.Fatalf("enter on a house: mode %v day %d -> %d", m.mode, day, w.Day)
	}
	m.Update(key("esc"))
	// The routes are the map's (#245): the ledger lists none.
	// Onto the offers: b opens the buy picker on that offer.
	for i := range offers {
		m.Update(key("down"))
		if got, want := first(), strings.ToUpper(offers[i].Name); got != want {
			t.Fatalf("offer %d: %q, want %q", i, got, want)
		}
	}
	m.Update(key("down")) // stays on the last offer
	if got, want := first(), strings.ToUpper(offers[2].Name); got != want {
		t.Fatalf("past the bottom: %q, want %q", got, want)
	}
	// Back to the first offer, which the fixture can afford.
	m.Update(key("up"))
	m.Update(key("up"))
	o := offers[0]
	if o.Locked(w) || o.Cost > w.Player.DirtyCash {
		t.Fatalf("fixture: %s is not affordable (%+v, dirty %d)", o.Name, o, w.Player.DirtyCash)
	}
	if !strings.Contains(stripANSI(strings.Join(m.details()[0].lines, "\n")), "b  buy it through the picker") {
		t.Fatalf("the offer's pane does not say b buys it:\n%s", stripANSI(strings.Join(m.details()[0].lines, "\n")))
	}
	m.Update(key("b")) // b opens the picker on its kind page (#241: enter's shortcut onto the offer went with enter)
	if m.mode != modeFront || m.front.step != 0 {
		t.Fatalf("b on an offer: mode %v step %d", m.mode, m.front.step)
	}
	m.Update(key("1"))     // a front: the kind page's first row (#500: a digit moves)
	m.Update(key("enter")) // the offers
	assertFits(t, m.View(), 120, 40, "front confirmation from the ledger")
	m.Update(key("enter"))
	if m.mode != modePlay || len(w.Fronts) != 4 || w.Fronts[3].ID != o.ID {
		t.Fatalf("buying: mode %v fronts %d status %q", m.mode, len(w.Fronts), m.status)
	}
	// The rows moved under the cursor: it stays on a row there is.
	rows := m.ledgerRows()
	if sel := m.ledgerSelected(); m.ledgerCursor >= len(rows) || sel.kind < 0 {
		t.Fatalf("after the buy: cursor %d of %d rows, %+v", m.ledgerCursor, len(rows), sel)
	}
	assertFrame(t, m, "ledger after the buy")
}

// The ledger fits 80x24 with three fronts, three routes and three
// offers and no house, and the strip names the selected front; with the
// fixture's house (#73: a fourth table) and on a shorter terminal it
// scrolls with the cursor so every ON OFFER row is reachable, the
// cursor's table heading kept in view, and never clamps.
func TestLedgerScrolls(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	m.Update(key("7"))
	assertFrame(t, m, "ledger at 80x24")
	if got := stripLine(m); !strings.HasPrefix(got, "▸ "+strings.ToUpper(w.Fronts[0].Name)) {
		t.Fatalf("the strip does not name the first front: %q", got)
	}
	houses := w.Houses
	w.Houses = nil
	view := stripANSI(m.View())
	for _, o := range m.frontRows() {
		if !strings.Contains(view, o.Name) {
			t.Fatalf("the ledger at 80x24 lacks the offer %s:\n%s", o.Name, view)
		}
	}
	w.Houses = houses
	// Too short for the whole ledger: MAIN is 15 rows here.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 19})
	rows := m.ledgerRows()
	name := func(r ledgerRow) string {
		switch r.kind {
		case ledgerFront:
			return w.Fronts[r.i].Name
		case ledgerHouse:
			return w.Houses[r.i].Name
		case ledgerDeed:
			return w.Deeds()[r.i].Name
		case ledgerAsset:
			return w.Assets[r.i].Name
		case ledgerAssetOffer:
			return m.assetRows()[r.i].Name
		case ledgerLane:
			return m.exportLanes()[r.i].Name
		case ledgerTrophy:
			return w.Trophies[r.i].Name
		case ledgerTrophyOffer:
			return m.trophyRows()[r.i].Name
		case ledgerPayoff:
			return m.payoffRows()[r.i].Who
		}
		return m.frontRows()[r.i].Name
	}
	heading := func(r ledgerRow) string {
		return [...]string{"FRONTS", "STASH", "PROPERTY", "ASSETS", "ASSETS", "EXPORTS", "TROPHIES", "TROPHIES", "PAYOFFS", "ON OFFER"}[r.kind]
	}
	for i, r := range rows {
		assertFrame(t, m, "short ledger row "+name(r))
		view := stripANSI(m.View())
		main := strings.Join(strings.Split(view, "\n")[1:m.height-2], "\n")
		if !strings.Contains(main, "▸ "+name(r)) {
			t.Fatalf("row %d (%s) is not in view:\n%s", i, name(r), view)
		}
		if !strings.Contains(main, heading(r)) {
			t.Fatalf("row %d (%s): its table's heading %s is out of view:\n%s", i, name(r), heading(r), view)
		}
		if got := stripLine(m); !strings.HasPrefix(got, "▸ "+strings.ToUpper(name(r))) {
			t.Fatalf("the strip does not name %s: %q", name(r), got)
		}
		m.Update(key("down"))
	}
	if last := rows[len(rows)-1]; last.kind != ledgerOffer || !strings.Contains(stripANSI(m.View()), name(last)) {
		t.Fatalf("the last row is not an offer in view: %+v", last)
	}
	// Back up: the title comes back into view at the top.
	for range rows {
		m.Update(key("up"))
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "\nLEDGER") {
		t.Fatalf("scrolled back to the top, the title is out of view:\n%s", view)
	}
}

// The ledger's empty states are one sentence in the one register, and
// the rest of the screen and the legend carry no key text of their own.
func TestLedgerEmptyStates(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("7"))
	view := stripANSI(m.View())
	for _, want := range []string{"No fronts yet. A front washes dirty cash clean; press b to buy one.", "FRONTS · 0 owned", "LOGISTICS · shipped 0 units in 0 runs · seized 0", "ON OFFER"} {
		if !strings.Contains(view, want) {
			t.Errorf("the empty ledger lacks %q:\n%s", want, view)
		}
	}
	for _, stale := range []string{"ON OFFER  b to buy", "d turns it", "r and R", "route(s)", "run(s)"} {
		if strings.Contains(view, stale) {
			t.Errorf("the ledger still says %q:\n%s", stale, view)
		}
	}
	// With no fronts the cursor starts on the first offer (#245: the
	// road is the map's).
	if sel := m.ledgerSelected(); sel.kind != ledgerOffer || sel.i != 0 {
		t.Fatalf("selected %+v with no fronts", sel)
	}
	if got := m.details()[0].title; got != strings.ToUpper(m.frontRows()[0].Name) {
		t.Fatalf("the pane's first section is %q", got)
	}
	// The wash section is there whatever is selected.
	secs := m.details()
	if secs[len(secs)-1].title != "WASH" {
		t.Fatalf("the last section is %q, want WASH", secs[len(secs)-1].title)
	}
}

// The dirty pile's line is one number on both screens (#350): the
// dashboard's CASH panel and the ledger (its till lines and the WASH
// section) warn at the threshold plus what the fronts cover, the line
// the heat sim charges against, and the ledger names the cover. A pile
// over the bare threshold but under the cover warns on neither.
func TestExposureLineIsOneNumber(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	thr, cover := m.rules.Heat.DirtyCashThreshold(w), m.rules.Heat.Cover(w)
	line := m.rules.Heat.ExposureLine(w)
	if len(w.Fronts) == 0 || cover <= 0 || line != thr+cover {
		t.Fatalf("fixture: %d fronts, threshold %d, cover %d, line %d", len(w.Fronts), thr, cover, line)
	}
	screens := func() (dash, ledger string) {
		m.Update(key("1"))
		dash = strings.Join(viewLines(m), "\n")
		m.Update(key("7"))
		ledger = strings.Join(viewLines(m), "\n")
		return dash, ledger
	}

	w.Player.DirtyCash = thr + cover/2
	dash, ledger := screens()
	if strings.Contains(dash, ": heat") || strings.Contains(ledger, "draws heat") {
		t.Fatalf("under the cover the screens warn:\n%s\n---\n%s", dash, ledger)
	}

	w.Player.DirtyCash = line + 1
	dash, _ = screens()
	if want := "over " + cash(line) + ": heat"; !strings.Contains(dash, want) {
		t.Fatalf("the dashboard lacks %q:\n%s", want, dash)
	}
	want := "Dirty cash over " + cash(line) + " draws heat every day: " + cash(thr) + ", plus " + cash(cover) + " your fronts cover."
	if till := mainText(m); !strings.Contains(till, "▲ "+want) {
		t.Fatalf("the till lines lack %q:\n%s", want, till)
	}
	if pane := paneProse(m); !strings.Contains(pane, want) {
		t.Fatalf("WASH lacks %q:\n%s", want, pane)
	}
}

// A front with nothing to wash says why where it is seen (#417): a
// laundromat read "open" and washed $0 a week, the till's line below
// the fold at 100 columns.
func TestIdleFrontSaysWhy(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		if len(m.w.Fronts) == 0 {
			t.Fatal("the rich fixture owns no front")
		}
		m.w.Fronts[0].Bought = 0 // open, not opening tomorrow
		m.w.Player.DirtyCash = m.till() - 1
		m.Update(key("7"))
		view := stripANSI(m.View())
		till := m.till()
		if want := fmt.Sprintf("idle: dirty %s is under the %s till", money(till-1), money(till)); !strings.Contains(view, want) {
			t.Errorf("%dx%d: the ledger does not say the wash is idle:\n%s", sz[0], sz[1], view)
		}
		if sz[0] >= 100 && !strings.Contains(view, "idle: under the till") {
			t.Errorf("%dx%d: the open front's status is not idle:\n%s", sz[0], sz[1], view)
		}
		m.w.Player.DirtyCash = m.till() + 10_000
		m.Update(key("7"))
		if view := stripANSI(m.View()); strings.Contains(view, "idle: dirty") || strings.Contains(view, "idle: under") {
			t.Errorf("%dx%d: an idle wash over the till:\n%s", sz[0], sz[1], view)
		}
	}
}
