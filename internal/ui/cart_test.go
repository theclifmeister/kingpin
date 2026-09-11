package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The dialog stays open after a buy and after an order (#103): from the
// dashboard, b, pick weed, quantity, enter; pick pills, quantity, enter;
// esc: both are bought in one visit, and the cart is under the table
// after the first. The same with s and two orders. Enter in the dialog
// never ends the day.
func TestDialogStaysOpenForTheNextLine(t *testing.T) {
	m := newTestModel(t, 100, 30)
	w := m.w
	home := w.Player.Location
	weed, pills := w.Products[0], w.Products[1]
	day := w.Day
	m.Update(key("b"))
	m.Update(key("enter"))
	m.Update(key("1"))
	m.Update(key("0"))
	m.Update(key("enter"))
	if m.mode != modeBuy || m.dlg.step != 0 || w.Stock(home, weed) != 10 {
		t.Fatalf("after the first buy: mode %v step %d weed %d err %q", m.mode, m.dlg.step, w.Stock(home, weed), m.dlg.err)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "CART") || !strings.Contains(view, "buy   Weed") || strings.Contains(view, "Pick a product") {
		t.Fatalf("the dialog does not show the cart under the table:\n%s", view)
	}
	m.Update(key("2"))
	m.Update(key("enter"))
	m.Update(key("4"))
	m.Update(key("enter"))
	if w.Stock(home, pills) != 4 || len(w.Buys) != 2 {
		t.Fatalf("after the second buy: pills %d, %d buys, err %q", w.Stock(home, pills), len(w.Buys), m.dlg.err)
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc on the product step: mode %v", m.mode)
	}
	m.Update(key("s"))
	for _, k := range []string{"1", "enter", "enter", "3", "enter"} {
		m.Update(key(k))
	}
	if m.mode != modeSell || m.dlg.step != 0 {
		t.Fatalf("after the first order: mode %v step %d err %q", m.mode, m.dlg.step, m.dlg.err)
	}
	for _, k := range []string{"2", "enter", "enter", "1", "enter", "esc"} {
		m.Update(key(k))
	}
	if m.mode != modePlay || len(w.Orders) != 2 || w.Day != day {
		t.Fatalf("after the second order: mode %v, %d orders, day %d", m.mode, len(w.Orders), w.Day)
	}
	if o, _ := w.Order(home, weed); o.Qty != 10 || o.Dial != events.DialAggressive {
		t.Fatalf("the weed order: %+v", o)
	}
	if o, _ := w.Order(home, pills); o.Qty != 4 || o.Dial != events.DialQuiet {
		t.Fatalf("the pills order: %+v", o)
	}
	// Deeper in, esc goes back a step, as before.
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc"))
	if m.mode != modeSell || m.dlg.step != 1 {
		t.Fatalf("esc on the dial step: mode %v step %d", m.mode, m.dlg.step)
	}
	m.Update(key("esc"))
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("mode %v after closing", m.mode)
	}
}

// The cart lists every buy and order of the day with the totals, on the
// dashboard's and the market's pane (first, so the strip at 80 columns
// carries the totals) and in the cart modal; editing there changes the
// order the market resolves and the stash and cash of a buy; removing a
// line cancels the order or returns the buy; the END THE DAY? modal
// reads it.
func TestCartListsAndEdits(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	weed, pills := w.Products[0], w.Products[1]
	cash := w.Player.DirtyCash
	price := w.Product(home, weed).SupplierPrice
	m.Update(key("b"))
	for _, k := range []string{"1", "enter", "1", "0", "enter", "2", "enter", "4", "enter", "esc"} {
		m.Update(key(k))
	}
	m.Update(key("s"))
	for _, k := range []string{"1", "enter", "enter", "3", "enter", "2", "enter", "enter", "1", "enter", "esc"} {
		m.Update(key(k))
	}
	if len(w.Buys) != 2 || len(w.Orders) != 2 {
		t.Fatalf("%d buys, %d orders", len(w.Buys), len(w.Orders))
	}
	// The pane, on the dashboard and the market.
	for _, s := range []string{"1", "2"} {
		m.Update(key(s))
		_, pane := bodyRows(m)
		text := strings.Join(pane, "\n")
		for _, want := range []string{"CART", "buying      2 lines", "selling     2 lines", "heat        +", "buy         10 Weed", "buy         4 Pills", "sell        10 Weed aggr.", "sell        4 Pills quiet", "c  edit"} {
			if !strings.Contains(text, want) {
				t.Errorf("screen %s: the pane lacks %q:\n%s", s, want, text)
			}
		}
		if i, j := strings.Index(text, "CART"), strings.Index(text, "PILLS"); i < 0 || j < i {
			t.Errorf("screen %s: the cart is not the first section:\n%s", s, text)
		}
	}
	// The strip at 80 columns carries the totals.
	m.Update(key("1"))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if strip := stripANSI(strings.Split(m.View(), "\n")[22]); !strings.Contains(strip, "CART · buying 2 lines") {
		t.Errorf("the strip does not carry the cart's totals: %q", strip)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// The END THE DAY? modal reads the cart.
	m.Update(key("enter"))
	if v := stripANSI(m.View()); !strings.Contains(v, "Buying 2 lines for") || !strings.Contains(v, "selling 2 lines, ~$") || !strings.Contains(v, "heat.") {
		t.Errorf("the end-day modal does not read the cart:\n%s", v)
	}
	m.Update(key("esc"))
	// The cart modal: the lines under a cursor, buys first.
	m.Update(key("c"))
	if m.mode != modeCart {
		t.Fatalf("c: mode %v status %q", m.mode, m.status)
	}
	view := stripANSI(m.View())
	for _, want := range []string{"▸ buy   Weed", "buy   Pills", "sell  Weed", "sell  Pills", "aggr.", "quiet", "spent $", "expect ~$", "dirty cash $"} {
		if !strings.Contains(view, want) {
			t.Errorf("the cart modal lacks %q:\n%s", want, view)
		}
	}
	// The dial of an order: right on the pills order takes it to normal.
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("j"))
	m.Update(key("right"))
	if o, _ := w.Order(home, pills); o.Dial != events.DialNormal || o.Qty != 4 {
		t.Fatalf("right on the pills order: %+v (%s)", o, m.crt.err)
	}
	m.Update(key("1"))
	if o, _ := w.Order(home, pills); o.Dial != events.DialQuiet {
		t.Fatalf("1 on the pills order: %+v", o)
	}
	// The quantity of an order: enter, the number, enter.
	m.Update(key("enter"))
	if m.crt.step != 1 {
		t.Fatalf("enter did not open the quantity: step %d", m.crt.step)
	}
	m.crt.qty.SetValue("3")
	m.Update(key("enter"))
	if o, _ := w.Order(home, pills); o.Qty != 3 || m.crt.step != 0 {
		t.Fatalf("the pills order after the edit: %+v step %d err %q", o, m.crt.step, m.crt.err)
	}
	// Enter in the cart never ends the day.
	if w.Day != 0 {
		t.Fatalf("the day moved to %d", w.Day)
	}
	// Removing the order cancels it.
	m.Update(key("x"))
	if _, ok := w.Order(home, pills); ok || len(w.Orders) != 1 {
		t.Fatalf("x did not cancel the order: %+v", w.Orders)
	}
	// Shrinking a buy is a partial return; removing it a full one, which
	// leaves cash, stash, BoughtToday and the supplier price as before.
	m.crt.cursor = 0
	m.Update(key("enter"))
	m.crt.qty.SetValue("6")
	m.Update(key("enter"))
	if w.Stock(home, weed) != 6 || w.Bought(home, weed) != 6 || m.crt.err != "" {
		t.Fatalf("after shrinking the weed buy: stock %d bought %d err %q", w.Stock(home, weed), w.Bought(home, weed), m.crt.err)
	}
	// Growing it buys the difference from the supplier where you stand.
	m.Update(key("enter"))
	m.crt.qty.SetValue("8")
	m.Update(key("enter"))
	if w.Stock(home, weed) != 8 || w.Bought(home, weed) != 8 || m.crt.err != "" {
		t.Fatalf("after growing the weed buy: stock %d bought %d err %q", w.Stock(home, weed), w.Bought(home, weed), m.crt.err)
	}
	m.Update(key("x"))
	if w.Stock(home, weed) != 0 || w.Bought(home, weed) != 0 || w.Product(home, weed).BoughtToday != 0 || w.Product(home, weed).SupplierPrice != price || m.crt.err != "" {
		t.Fatalf("after returning the weed: stock %d bought %d today %d price %v -> %v err %q", w.Stock(home, weed), w.Bought(home, weed), w.Product(home, weed).BoughtToday, price, w.Product(home, weed).SupplierPrice, m.crt.err)
	}
	if !strings.HasPrefix(m.status, "Returned 8 Weed") {
		t.Errorf("the status after a return: %q", m.status)
	}
	// The weed order stays on the books, though the stash is gone: the
	// market sells what is there.
	if _, ok := w.Order(home, weed); !ok {
		t.Fatal("returning the buy took the order with it")
	}
	// The pills' units have left the stash (on the road): the return is
	// refused, in the modal and the status bar, and the line stays.
	w.Stash(home)[pills] = 1
	m.crt.cursor = 0
	m.Update(key("x"))
	if w.Bought(home, pills) != 4 || m.crt.err == "" || m.statusKind != statusWarning || !strings.HasPrefix(m.status, "Can't return") {
		t.Fatalf("returning shipped units: bought %d err %q status %q kind %v", w.Bought(home, pills), m.crt.err, m.status, m.statusKind)
	}
	w.Stash(home)[pills] = 4
	m.Update(key("x"))
	if w.Bought(home, pills) != 0 || w.Player.DirtyCash != cash {
		t.Fatalf("after returning everything: bought %d cash %d, had %d", w.Bought(home, pills), w.Player.DirtyCash, cash)
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc: mode %v", m.mode)
	}
	// An empty cart: no section in the pane, the modal says so.
	w.Orders = map[string]game.SellOrder{}
	if secs := m.cartSection(home); secs != nil {
		t.Errorf("an empty cart has a pane section: %+v", secs)
	}
	m.Update(key("c"))
	if v := stripANSI(m.View()); !strings.Contains(v, "Nothing in the cart.") {
		t.Errorf("the empty cart:\n%s", v)
	}
	m.Update(key("esc"))
	// The day ends: the scratch is gone and c pressed elsewhere points
	// at the screens that take it.
	m.Update(key("3"))
	m.Update(key("c"))
	if m.status != "Cart on the dashboard screen (1) or the market screen (2). Post runner on the map screen (5)." {
		t.Errorf("c on the journal: %q", m.status)
	}
}
