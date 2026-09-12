package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// footerKeys is the modal's footer as `key label` pairs, for reading
// what a step lists.
func footerKeys(m *Model) string {
	var keys []string
	for _, b := range m.modalFooter() {
		keys = append(keys, b.key+" "+b.label)
	}
	return strings.Join(keys, "  ")
}

// The trade is one dialog (#168): b and s on the product step turn it
// to the other side in place, the product under the cursor and the cart
// kept; on the quantity step the field ignores them, so a typed
// quantity is never lost; and the footer lists the other side's key on
// the product step alone.
func TestDialogTogglesSide(t *testing.T) {
	m := newTestModel(t, 100, 30)
	w := m.w
	home := w.Player.Location
	weed := w.Products[0]
	w.SetStock(home, weed, 20)
	m.Update(key("b"))
	if m.mode != modeBuy || m.cursor != 0 {
		t.Fatalf("b: mode %v cursor %d", m.mode, m.cursor)
	}
	// A line in the cart, so the toggle has something to keep.
	for _, k := range []string{"enter", "5", "enter", "enter"} {
		m.Update(key(k))
	}
	if len(w.Buys) != 1 || m.dlg.step != 0 {
		t.Fatalf("the buy did not complete: %d buys, step %d, err %q", len(w.Buys), m.dlg.step, m.dlg.err)
	}
	if f := footerKeys(m); !strings.Contains(f, "s sell") || strings.Contains(f, "b buy") {
		t.Fatalf("the buy dialog's product step lists %q", f)
	}
	m.Update(key("b")) // the side's own key does nothing
	if m.mode != modeBuy {
		t.Fatalf("b on the buy dialog: mode %v", m.mode)
	}
	m.Update(key("s"))
	if m.mode != modeSell || m.cursor != 0 || m.dlg.step != 0 || m.dlg.err != "" {
		t.Fatalf("s on the buy dialog: mode %v cursor %d step %d err %q", m.mode, m.cursor, m.dlg.step, m.dlg.err)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "SELL · ") || !strings.Contains(v, "CART") || !strings.Contains(v, "buy   Weed") {
		t.Fatalf("the sell dialog does not keep the cart:\n%s", v)
	}
	if f := footerKeys(m); !strings.Contains(f, "b buy") || strings.Contains(f, "s sell") {
		t.Fatalf("the sell dialog's product step lists %q", f)
	}
	// On the quantity step b and s are the field's to ignore, and the
	// footer lists neither.
	m.Update(key("tab"))
	for _, r := range "12" {
		m.Update(key(string(r)))
	}
	m.Update(key("b"))
	if m.mode != modeSell || m.dlg.step != 1 || m.dlg.qty.Value() != "12" {
		t.Fatalf("b on the quantity step: mode %v step %d qty %q", m.mode, m.dlg.step, m.dlg.qty.Value())
	}
	if f := footerKeys(m); strings.Contains(f, "b buy") || strings.Contains(f, "s sell") {
		t.Fatalf("the quantity step lists %q", f)
	}
	m.Update(key("tab"))
	m.Update(key("s"))
	if m.mode != modeSell || m.dlg.step != 2 {
		t.Fatalf("s on the dial step: mode %v step %d", m.mode, m.dlg.step)
	}
	if f := footerKeys(m); strings.Contains(f, "b buy") || strings.Contains(f, "s sell") {
		t.Fatalf("the dial step lists %q", f)
	}
	m.Update(key("shift+tab"))
	m.Update(key("shift+tab"))
	m.Update(key("b"))
	if m.mode != modeBuy || m.cursor != 0 || m.dlg.step != 0 || m.dlg.qty.Value() != "" || m.dlg.err != "" {
		t.Fatalf("b on the sell dialog: mode %v cursor %d step %d qty %q err %q", m.mode, m.cursor, m.dlg.step, m.dlg.qty.Value(), m.dlg.err)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "BUY · ") || !strings.Contains(v, "CART") {
		t.Fatalf("the buy dialog does not keep the cart:\n%s", v)
	}
	// The product stays under the cursor through a turn.
	m.Update(key("j"))
	m.Update(key("s"))
	if m.mode != modeSell || m.cursor != 1 {
		t.Fatalf("s with the cursor moved: mode %v cursor %d", m.mode, m.cursor)
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc: mode %v", m.mode)
	}
}

// A side that cannot open is the error line, not a close: lying low
// there is no sale, and the buy dialog stays up saying so.
func TestDialogToggleRefusalStays(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("b"))
	m.w.SetLieLow(true)
	m.Update(key("s"))
	if m.mode != modeBuy || !strings.Contains(m.dlg.err, "lying low") {
		t.Fatalf("s lying low: mode %v err %q", m.mode, m.dlg.err)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "lying low") {
		t.Fatalf("the error is not in the dialog:\n%s", v)
	}
}

// A turn that changes the city says so (#168): on the market screen
// turned to the other city a sale is there and a buy is where you
// stand, so the sell dialog turned to its buy side reads `Buying in
// Eastside.` and turned back `Selling in Bayport.`; at home on the
// dashboard nothing is said. (A fresh b on the market turned to a city
// nobody runs for you is the pointer, #174, TestBuyDialogFollowsTheLieutenant;
// the turn inside the dialog still goes where a buy can be.)
func TestDialogToggleNamesTheCity(t *testing.T) {
	m := richModelSeeded(t, 100, 30, 11)
	w := m.w
	hub := w.CityOrder[1]
	w.SetStock(hub, w.Products[0], 40)
	m.Update(key("2"))
	m.Update(key("right"))
	if m.shown().ID != hub {
		t.Fatalf("the market is turned to %s", m.shown().ID)
	}
	m.Update(key("s"))
	if m.mode != modeSell || m.dlg.turned || m.dialogCity() != hub {
		t.Fatalf("s: mode %v turned %v city %s: %q", m.mode, m.dlg.turned, m.dialogCity(), m.status)
	}
	if v := stripANSI(m.View()); strings.Contains(v, "Buying in") || strings.Contains(v, "Selling in") {
		t.Fatalf("a dialog opened fresh names its city:\n%s", v)
	}
	m.Update(key("b"))
	if m.mode != modeBuy || m.dialogCity() != w.Player.Location {
		t.Fatalf("b: mode %v city %s err %q", m.mode, m.dialogCity(), m.dlg.err)
	}
	rows := strings.Split(stripANSI(m.View()), "\n")
	if want := "Buying in " + w.CityName(w.Player.Location) + "."; !strings.Contains(rows[5], want) {
		t.Fatalf("the first body line is not %q:\n%s", want, strings.Join(rows, "\n"))
	}
	m.Update(key("s"))
	rows = strings.Split(stripANSI(m.View()), "\n")
	if want := "Selling in " + w.CityName(hub) + "."; m.mode != modeSell || !strings.Contains(rows[5], want) {
		t.Fatalf("the first body line is not %q:\n%s", want, strings.Join(rows, "\n"))
	}
	m.Update(key("esc"))
	// At home on the dashboard the sides are one city: nothing to say.
	m.Update(key("1"))
	m.Update(key("b"))
	m.Update(key("s"))
	if m.mode != modeSell || m.dlg.turned {
		t.Fatalf("s at home: mode %v turned %v err %q", m.mode, m.dlg.turned, m.dlg.err)
	}
	if v := stripANSI(m.View()); strings.Contains(v, "Selling in") {
		t.Fatalf("a turn within one city names it:\n%s", v)
	}
	_ = game.You
}
