package ui

import (
	"strings"
	"testing"
)

// Supply contracts in the grammar (#113): the buy dialog's last step is
// once / keep at, and keep at sets a contract for the product where you
// stand at the quantity; the market's table carries the level in its
// keep column and the pane a contract row with what x does; x on the
// product clears the contract when it has no order and cancels the
// order when it has; the morning after, the contract's buy is a line
// marked contract in the cart, returnable through it, the street's
// facts carry the supply line, the END THE DAY summary counts it, the
// report's SALES section names it, and a sell order may count on what
// the contract brings.
func TestSupplyContractInTheGrammar(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home, weed := w.Player.Location, w.Products[0]
	w.Player.DirtyCash = 100_000
	// b: product, quantity, then once / keep at; right turns it, enter
	// at keep at sets the contract and the dialog stays open on the
	// product step for the next line.
	m.Update(key("b"))
	m.Update(key("enter"))
	for _, k := range []string{"3", "0", "enter"} {
		m.Update(key(k))
	}
	if m.mode != modeBuy || m.dlg.step != 2 || m.dlg.repeat != repeatOnce {
		t.Fatalf("after the quantity: mode %v step %d repeat %v", m.mode, m.dlg.step, m.dlg.repeat)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "[once]  keep at") || !strings.Contains(view, "enter buy") {
		t.Fatalf("the repeat step:\n%s", view)
	}
	m.Update(key("right"))
	view = stripANSI(m.View())
	if m.dlg.repeat != repeatKeep || !strings.Contains(view, "once  [keep at]") || !strings.Contains(view, "enter keep at") || !strings.Contains(view, "keep 30 here") {
		t.Fatalf("at keep at:\n%s", view)
	}
	m.Update(key("enter"))
	c, ok := w.Supplied(home, weed)
	if !ok || c.Units != 30 || m.mode != modeBuy || m.dlg.step != 0 || w.Stock(home, weed) != 0 || len(w.Buys) != 0 {
		t.Fatalf("keep at: contract %+v %v, mode %v step %d, stock %d, buys %d, err %q", c, ok, m.mode, m.dlg.step, w.Stock(home, weed), len(w.Buys), m.dlg.err)
	}
	if !strings.HasPrefix(m.status, "Keeping 30 Weed in Eastside") {
		t.Fatalf("status %q", m.status)
	}
	// Opening the same product again lands on the level at keep at.
	m.Update(key("enter"))
	if m.dlg.step != 1 || m.dlg.qty.Value() != "30" || m.dlg.repeat != repeatKeep {
		t.Fatalf("reopening a kept product: step %d quantity %q repeat %v", m.dlg.step, m.dlg.qty.Value(), m.dlg.repeat)
	}
	m.Update(key("esc"))
	// The market's keep column and the pane's contract row.
	m.Update(key("2"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "keep") || !strings.Contains(view, "contract    keep at 30") || !strings.Contains(view, "x  clear the contract") {
		t.Fatalf("the market with a contract:\n%s", view)
	}
	// A sell order may count on what the contract brings: nothing in the
	// stash, and the dialog opens on 30.
	m.Update(key("s"))
	m.Update(key("enter"))
	if m.mode != modeSell || m.dlg.step != 1 || m.qtyMax() != 30 {
		t.Fatalf("selling what the contract brings: mode %v step %d max %d status %q", m.mode, m.dlg.step, m.qtyMax(), m.status)
	}
	if !strings.Contains(stripANSI(m.View()), "0 stashed and 30 the contract brings") {
		t.Fatalf("the sell dialog does not say what the contract brings:\n%s", stripANSI(m.View()))
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc"))
	if o, ok := w.Order(home, weed); !ok || o.Qty != 30 {
		t.Fatalf("the order: %+v %v, status %q", o, ok, m.status)
	}
	// x cancels the order first and leaves the contract; a second x
	// clears the contract.
	m.Update(key("x"))
	if _, ok := w.Order(home, weed); ok || len(w.Supply) != 1 {
		t.Fatalf("x with an order: order %v, contracts %d", ok, len(w.Supply))
	}
	m.Update(key("x"))
	if len(w.Supply) != 0 || !strings.HasPrefix(m.status, "Contract cleared") {
		t.Fatalf("x with no order: contracts %d, status %q", len(w.Supply), m.status)
	}
	// Set it again, end the day: the morning brings the contract's buy.
	if err := w.SetSupply(home, weed, 30); err != nil {
		t.Fatal(err)
	}
	endDay(t, m)
	if !strings.Contains(strings.Join(w.Report.Sales, "\n"), "Supply contract bought 30 Weed") {
		t.Fatalf("the report's sales: %v", w.Report.Sales)
	}
	m.Update(key("enter"))
	if n, cost := w.SuppliedToday(); n != 30 || cost == 0 || w.Stock(home, weed) != 30 {
		t.Fatalf("the morning: supplied %d for %d, stock %d", n, cost, w.Stock(home, weed))
	}
	lines := m.cartLines()
	if len(lines) != 1 || !lines[0].contract || lines[0].qty != 30 {
		t.Fatalf("the cart: %+v", lines)
	}
	tot := totals(lines)
	if tot.buys != 1 || tot.contracts != 1 || tot.spent != tot.supplied || tot.spent == 0 {
		t.Fatalf("the totals: %+v", tot)
	}
	if s := m.cartSummary(); !strings.Contains(s, "1 by contract") {
		t.Fatalf("the summary: %q", s)
	}
	m.Update(key("1"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "supply 1 contract · ") || !strings.Contains(view, "this morning") {
		t.Fatalf("the street's facts:\n%s", view)
	}
	// The cart modal shows the line marked contract; x returns it whole
	// at the price paid.
	cash := w.Player.DirtyCash
	m.Update(key("c"))
	if !strings.Contains(stripANSI(m.View()), "contract  Weed") {
		t.Fatalf("the cart modal:\n%s", stripANSI(m.View()))
	}
	m.Update(key("x"))
	if w.Stock(home, weed) != 0 || w.Player.DirtyCash != cash+tot.spent || len(w.Buys) != 0 {
		t.Fatalf("returning the contract's line: stock %d cash %d (was %d, spent %d) buys %d status %q", w.Stock(home, weed), w.Player.DirtyCash, cash, tot.spent, len(w.Buys), m.status)
	}
	m.Update(key("esc"))
	// The dial keys on the buy's repeat step do not shadow the sell
	// dial's: 1-2 pick a notch there.
	m.Update(key("b"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("2"))
	if m.dlg.repeat != repeatKeep {
		t.Fatalf("2 on the repeat step: %v", m.dlg.repeat)
	}
	m.Update(key("1"))
	if m.dlg.repeat != repeatOnce {
		t.Fatalf("1 on the repeat step: %v", m.dlg.repeat)
	}
	m.Update(key("esc"))
}
