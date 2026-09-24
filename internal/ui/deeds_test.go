package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The property (#194): d on the map opens the confirmation for the
// block under the cursor (the price, the rent, the DA's line), refused
// with no clean cash and silent-with-a-pointer on the routes; y buys it
// clean, the cell's mark turns to the deed's glyph and the inspector
// gains a DEED row; the ledger's PROPERTY block lists the deed under
// the one cursor with the pane's section, enter on its row being the
// frame's; a deed on the rival's block reads their defence cut in the
// strike picker. Fits 80x24 with the table in.
func TestDeedKeys(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	tr := m.rules.Territory
	m.Update(key("5"))
	m.mapCursor = 1 // Dre's corner, yours
	c := m.mapSelected()
	if c == nil || !c.Held() {
		t.Fatalf("the cursor is not on a corner of yours: %+v", c)
	}
	price := tr.DeedPrice(w, *c)
	if price <= 0 {
		t.Fatalf("no price on %s", c.Name)
	}
	// No clean cash: refused, and the inspector carries no d row.
	w.Player.CleanCash = 0
	if pane := paneText(m); strings.Contains(pane, "buy the block") {
		t.Fatalf("the inspector prices the block with nothing washed:\n%s", pane)
	}
	m.Update(key("d"))
	if m.mode != modeConfirm {
		t.Fatalf("d on the map: mode %v", m.mode)
	}
	m.Update(key("y"))
	if m.mode != modePlay || !strings.Contains(m.status, "clean cash only") || c.Deed != nil {
		t.Fatalf("y with no clean cash: mode %v status %q deed %+v", m.mode, m.status, c.Deed)
	}
	// On the routes d points at the grid.
	m.onRoutes = true
	m.Update(key("d"))
	if m.mode != modePlay || !strings.Contains(m.status, "Pick a corner") {
		t.Fatalf("d on the routes: mode %v status %q", m.mode, m.status)
	}
	m.onRoutes = false

	// With clean cash and a wash behind it: the row, the dialog, the
	// buy.
	w.Player.CleanCash = price + 100_000
	w.Stats.Laundered = 10 * price
	if pane := paneText(m); !strings.Contains(pane, "buy the block: "+cash(price)) {
		t.Fatalf("the inspector does not price the block:\n%s", pane)
	}
	m.Update(key("d"))
	if m.mode != modeConfirm {
		t.Fatalf("d on the map: mode %v", m.mode)
	}
	assertFits(t, m.View(), 80, 24, "deed confirmation")
	view := stripANSI(m.View())
	for _, want := range []string{"BUY THE BLOCK?", money(price), "y buy", "esc close", "DA's line"} {
		if !strings.Contains(view, want) {
			t.Fatalf("deed dialog lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("esc"))
	if m.mode != modePlay || c.Deed != nil {
		t.Fatalf("esc bought: mode %v deed %+v", m.mode, c.Deed)
	}
	m.Update(key("d"))
	m.Update(key("y"))
	if m.mode != modePlay || c.Deed == nil || c.Deed.Price != price || w.Player.CleanCash != 100_000 {
		t.Fatalf("y did not buy: mode %v deed %+v clean %d", m.mode, c.Deed, w.Player.CleanCash)
	}
	if !strings.Contains(m.status, "is yours") {
		t.Fatalf("status after the buy: %q", m.status)
	}
	if len(w.Today.DeedsBought) != 1 || w.Stats.Deeds != 1 || w.Stats.DeedCash != price {
		t.Fatalf("the buy was not recorded: %v %+v", w.Today.DeedsBought, w.Stats)
	}
	// The cell's glyph and the inspector's row.
	if got := m.cellMark(c); got != deedGlyph {
		t.Fatalf("the cell's mark is %q, want %q", got, deedGlyph)
	}
	if pane := paneText(m); !strings.Contains(pane, "DEED") || !strings.Contains(pane, money(tr.DeedRent(m.w, *c, c.Deed.Price))+"/day") || strings.Contains(pane, "buy the block") {
		t.Fatalf("the inspector after the buy:\n%s", pane)
	}
	assertFrame(t, m, "map with a deed")
	// Again: refused, the block is yours.
	m.Update(key("d"))
	if m.mode != modePlay || !strings.Contains(m.status, "yours already") {
		t.Fatalf("d on a deeded block: mode %v status %q", m.mode, m.status)
	}

	// The ledger's PROPERTY block, under the cursor.
	m.Update(key("7"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "PROPERTY") || !strings.Contains(view, c.Name) {
		t.Fatalf("ledger lacks the property:\n%s", view)
	}
	assertFrame(t, m, "ledger with a deed")
	for m.ledgerSelected().kind != ledgerDeed {
		before := m.ledgerCursor
		m.Update(key("down"))
		if m.ledgerCursor == before {
			t.Fatalf("the cursor never reached PROPERTY:\n%s", stripANSI(m.View()))
		}
	}
	if got := stripLine(m); !strings.HasPrefix(got, "▸ "+strings.ToUpper(c.Name)) {
		t.Fatalf("the strip does not name the deed: %q", got)
	}
	if pane := paneText(m); !strings.Contains(pane, "DA's line") || !strings.Contains(pane, "rent") {
		t.Fatalf("the deed's section:\n%s", pane)
	}
	m.Update(key("enter"))
	if m.mode != modeConfirmEnd {
		t.Fatalf("enter on a deed row: mode %v (the frame's enter should ask to end the day)", m.mode)
	}
	m.Update(key("esc"))
	// d on the ledger is still the launder dial: the map's d is the
	// map's alone.
	dial := w.Laundering.Dial
	m.Update(key("d"))
	if m.mode != modePlay || w.Laundering.Dial == dial {
		t.Fatalf("d on the ledger: mode %v dial %v (was %v)", m.mode, w.Laundering.Dial, dial)
	}

	// A deed on the rival's block cuts their defence: the picker's odds
	// on that corner read higher than the faction's.
	m.Update(key("5"))
	m.mapCursor = 0 // the rival's corner in the fixture
	rc := m.mapSelected()
	if rc == nil || rc.Owner != game.OwnerRival {
		t.Fatalf("the cursor is not on the rival's corner: %+v", rc)
	}
	f := m.factionOf(rc)
	before := m.sess.Sims().Rivals.OddsOn(w, f, rc, events.ForcePush)
	w.Player.CleanCash = tr.DeedPrice(w, *rc)
	m.Update(key("d"))
	if view := stripANSI(m.View()); !strings.Contains(view, f.Leader+"'s defence") {
		t.Fatalf("the dialog on a rival's block does not say what the deed does to them:\n%s", view)
	}
	m.Update(key("y"))
	if rc.Deed == nil {
		t.Fatalf("the rival's block was not bought: %q", m.status)
	}
	if after := m.sess.Sims().Rivals.OddsOn(w, f, rc, events.ForcePush); after <= before {
		t.Fatalf("the deed did not cut their defence: odds %.3f before, %.3f after", before, after)
	}
	if !strings.Contains(paneText(m), "DEED") {
		t.Fatalf("the rival corner's inspector lacks the DEED row:\n%s", paneText(m))
	}
}
