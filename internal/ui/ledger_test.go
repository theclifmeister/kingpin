package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
)

// stripLine is the details strip of a view at a width under the pane's:
// row h-2, `▸ …  ␣ more`.
func stripLine(m *Model) string {
	ls := strings.Split(stripANSI(m.View()), "\n")
	return strings.TrimRight(ls[m.height-2], " ")
}

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
	// Onto the routes: enter turns the selected route's dial a notch.
	for i := range routes {
		m.Update(key("j"))
		if got, want := first(), strings.ToUpper(routes[i].Name); got != want {
			t.Fatalf("route %d: %q, want %q", i, got, want)
		}
	}
	last := routes[2]
	before := w.Route(last.ID).Dial
	m.Update(key("enter"))
	if got := w.Route(last.ID).Dial; got != (before+1)%(events.RouteFast+1) || m.mode != modePlay {
		t.Fatalf("enter on a route: dial %v -> %v, mode %v, status %q", before, got, m.mode, m.status)
	}
	if !strings.Contains(m.status, last.Name) {
		t.Fatalf("status after the dial: %q", m.status)
	}
	// Onto the offers: enter opens the buy confirmation on that offer.
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
	if !strings.Contains(stripANSI(strings.Join(m.details()[0].lines, "\n")), "enter buy it") {
		t.Fatalf("the offer's pane does not say enter buys it:\n%s", stripANSI(strings.Join(m.details()[0].lines, "\n")))
	}
	m.Update(key("enter"))
	if m.mode != modeFront || m.frontCursor != 0 {
		t.Fatalf("enter on an offer: mode %v cursor %d", m.mode, m.frontCursor)
	}
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
// offers, and the strip names the selected front; a shorter terminal
// scrolls it with the cursor so every ON OFFER row is reachable, the
// cursor's table heading kept in view, and never clamps.
func TestLedgerScrolls(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	m.Update(key("7"))
	assertFrame(t, m, "ledger at 80x24")
	if got := stripLine(m); !strings.HasPrefix(got, "▸ "+strings.ToUpper(w.Fronts[0].Name)) {
		t.Fatalf("the strip does not name the first front: %q", got)
	}
	view := stripANSI(m.View())
	for _, o := range m.frontRows() {
		if !strings.Contains(view, o.Name) {
			t.Fatalf("the ledger at 80x24 lacks the offer %s:\n%s", o.Name, view)
		}
	}
	// Too short for the whole ledger: MAIN is 15 rows here.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 19})
	rows := m.ledgerRows()
	name := func(r ledgerRow) string {
		switch r.kind {
		case ledgerFront:
			return w.Fronts[r.i].Name
		case ledgerRoute:
			return m.ledgerRoutes()[r.i].Name
		}
		return m.frontRows()[r.i].Name
	}
	heading := func(r ledgerRow) string { return [...]string{"FRONTS", "LOGISTICS", "ON OFFER"}[r.kind] }
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
	// With no fronts the cursor starts on the first route.
	if sel := m.ledgerSelected(); sel.kind != ledgerRoute || sel.i != 0 {
		t.Fatalf("selected %+v with no fronts", sel)
	}
	if got := m.details()[0].title; got != strings.ToUpper(m.ledgerRoutes()[0].Name) {
		t.Fatalf("the pane's first section is %q", got)
	}
	// The wash section is there whatever is selected.
	secs := m.details()
	if secs[len(secs)-1].title != "WASH" {
		t.Fatalf("the last section is %q, want WASH", secs[len(secs)-1].title)
	}
}
