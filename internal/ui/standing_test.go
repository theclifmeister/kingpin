package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Standing orders in the grammar (#114): the sell dialog's last step,
// after the dial, is once / standing, and standing sets a standing
// order for the product in the dialog's city at the quantity and the
// dial; the market's and the dashboard's order column reads `30 normal
// ↻`, the pane a standing row with the cut and what x does; an order
// of the day sits over it (the column reads the order, x cancels the
// order first and the standing one second); the cart lists it as a
// line marked standing that edits like an order and whose x cancels
// it; the END THE DAY summary counts it; the market resolves it at the
// cut and the report's SALES line says so; and the dialog reopens on
// its units at standing.
func TestStandingOrderInTheGrammar(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home, weed := w.Player.Location, w.Products[0]
	w.Player.DirtyCash = 100_000
	w.SetStock(home, weed, 50)
	m.Update(key("s"))
	m.Update(key("enter"))
	for _, k := range []string{"3", "0", "enter"} {
		m.Update(key(k))
	}
	if m.mode != modeSell || m.dlg.step != 2 {
		t.Fatalf("after the quantity: mode %v step %d", m.mode, m.dlg.step)
	}
	m.Update(key("3")) // aggressive
	m.Update(key("enter"))
	if m.dlg.step != 3 || m.dlg.repeat != repeatOnce {
		t.Fatalf("after the dial: step %d repeat %v", m.dlg.step, m.dlg.repeat)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "[once]  standing") || !strings.Contains(view, "enter sell") || strings.Contains(view, "enter sell nightly") {
		t.Fatalf("the repeat step:\n%s", view)
	}
	m.Update(key("right"))
	view = stripANSI(m.View())
	if m.dlg.repeat != repeatStanding || !strings.Contains(view, "once  [standing]") || !strings.Contains(view, "enter sell nightly") || !strings.Contains(view, "30 at aggressive nightly") || !strings.Contains(view, "keep 5%") {
		t.Fatalf("at standing:\n%s", view)
	}
	// shift+tab from the repeat step keeps the dial and the quantity.
	m.Update(key("shift+tab"))
	if m.dlg.step != 2 || m.dlg.dial != events.DialAggressive || m.dlg.qty.Value() != "30" {
		t.Fatalf("back from the repeat: step %d dial %s quantity %q", m.dlg.step, m.dlg.dial, m.dlg.qty.Value())
	}
	m.Update(key("tab"))
	m.Update(key("enter"))
	o, ok := w.YourStanding(home, weed)
	if !ok || o.Qty != 30 || o.Dial != events.DialAggressive || m.mode != modeSell || m.dlg.step != 0 || len(w.Orders) != 0 {
		t.Fatalf("standing: order %+v %v, mode %v step %d, orders %d, err %q", o, ok, m.mode, m.dlg.step, len(w.Orders), m.dlg.err)
	}
	if !strings.HasPrefix(m.status, "Standing: 30 Weed in Eastside, aggressive") {
		t.Fatalf("status %q", m.status)
	}
	// Reopening the product lands on its units, at standing.
	m.Update(key("enter"))
	if m.dlg.step != 1 || m.dlg.qty.Value() != "30" || m.dlg.repeat != repeatStanding {
		t.Fatalf("reopening a standing product: step %d quantity %q repeat %v", m.dlg.step, m.dlg.qty.Value(), m.dlg.repeat)
	}
	m.Update(key("esc"))
	// The order column and the pane, on the market and the dashboard;
	// the END THE DAY summary and the street's last line.
	for _, s := range []string{"2", "1"} {
		m.Update(key(s))
		view = stripANSI(m.View())
		for _, want := range []string{"30 aggr. ↻", "standing    30 aggr. · cut 5%", "x  cancel the standing order"} {
			if !strings.Contains(view, want) {
				t.Fatalf("screen %s lacks %q:\n%s", s, want, view)
			}
		}
	}
	if !strings.Contains(stripANSI(m.View()), "Standing orders sell tonight") {
		t.Fatalf("the street's last line:\n%s", stripANSI(m.View()))
	}
	if s := m.cartSummary(); !strings.Contains(s, "Selling 1 line (1 standing)") {
		t.Fatalf("the summary: %q", s)
	}
	lines := m.cartLines()
	if len(lines) != 1 || !lines[0].standing || lines[0].qty != 30 || lines[0].take == 0 {
		t.Fatalf("the cart: %+v", lines)
	}
	// An order of the day sits over the standing one: the column reads
	// the order, the cart lists the order alone, x cancels the order
	// first and the standing one second.
	if err := w.PlaceSell(home, weed, 10, events.DialQuiet); err != nil {
		t.Fatal(err)
	}
	view = stripANSI(m.View())
	if !strings.Contains(view, "10 quiet") || strings.Contains(view, "↻") || !strings.Contains(view, "x  cancel the 10 quiet") || strings.Contains(view, "x  cancel the standing order") {
		t.Fatalf("with an order over the standing one:\n%s", view)
	}
	if lines := m.cartLines(); len(lines) != 1 || lines[0].standing {
		t.Fatalf("the cart with an order over the standing one: %+v", lines)
	}
	m.Update(key("x"))
	if _, ok := w.Order(home, weed); ok || len(w.Standing) != 1 {
		t.Fatalf("x with an order: order %v, standing %d", ok, len(w.Standing))
	}
	// The cart modal: the line marked standing, its quantity and dial
	// edited as an order's are, x cancels it.
	m.Update(key("c"))
	view = stripANSI(m.View())
	if !strings.Contains(view, "standing  Weed") {
		t.Fatalf("the cart modal:\n%s", view)
	}
	m.Update(key("left"))
	if o, _ := w.YourStanding(home, weed); o.Dial != events.DialNormal {
		t.Fatalf("left on the standing line: %+v", o)
	}
	m.Update(key("enter"))
	m.crt.qty.SetValue("20")
	m.Update(key("enter"))
	if o, _ := w.YourStanding(home, weed); o.Qty != 20 || m.crt.step != 0 {
		t.Fatalf("a quantity for the standing line: %+v step %d err %q", o, m.crt.step, m.crt.err)
	}
	m.Update(key("x"))
	if len(w.Standing) != 0 || m.status != "Standing order cancelled." {
		t.Fatalf("x on the standing line: %d standing, status %q", len(w.Standing), m.status)
	}
	m.Update(key("esc"))
	// x on the product with nothing but the standing order cancels it.
	if err := w.PlaceStanding(home, weed, 30, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	m.Update(key("x"))
	if len(w.Standing) != 0 || !strings.HasPrefix(m.status, "Standing order cancelled") {
		t.Fatalf("x with only a standing order: %d standing, status %q", len(w.Standing), m.status)
	}
	// Set it again and end the day: the market sells it at the cut and
	// the report says so; the order stands the next morning.
	if err := w.PlaceStanding(home, weed, 30, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	endDay(t, m)
	sales := strings.Join(w.Report.Sales, "\n")
	if !strings.Contains(sales, "normal, standing, cut $") {
		t.Fatalf("the report's sales: %v", w.Report.Sales)
	}
	if !strings.Contains(strings.Join(w.Report.Money, "\n"), "cut on the standing orders") {
		t.Fatalf("the report's money: %v", w.Report.Money)
	}
	if o, ok := w.YourStanding(home, weed); !ok || o.Qty != 30 {
		t.Fatalf("the morning after: %+v %v", o, ok)
	}
	m.Update(key("enter"))
	// Lying low: the cart lists no standing line and the summary says
	// so; the order still stands.
	m.Update(key("l"))
	if lines := m.cartLines(); len(lines) != 0 || m.endDayLine() != "Lying low today." {
		t.Fatalf("lying low: cart %+v, %q", lines, m.endDayLine())
	}
	if _, ok := w.YourStanding(home, weed); !ok {
		t.Fatal("lying low cancelled the standing order")
	}
}
