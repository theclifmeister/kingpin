package ui

import (
	"strconv"
	"strings"
	"testing"
)

// The buy's quantity step shows the cash and the room after the buy
// (#356), and a quantity past what fits is refused with an offer on the
// quantity step (#467): the field is set to what fits, and the next
// enters buy it.
func TestBuyOffersWhatFits(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Player.DirtyCash = 10_000_000
	city := m.w.Player.Location
	m.Update(key("b"))
	m.Update(key("enter"))
	for _, k := range "99999" {
		m.Update(key(string(k)))
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "after") || !strings.Contains(view, "dirty · stash ") || !strings.Contains(view, " in "+m.w.CityName(city)) {
		t.Fatalf("no after row on the quantity step:\n%s", view)
	}
	id := m.w.Products[m.cursor]
	fits := m.maxBuy(id)
	if fits <= 0 {
		t.Fatalf("nothing fits: %d", fits)
	}
	before := m.w.Stock(city, id)
	m.Update(key("enter")) // refused on the quantity step itself (#467)
	if m.mode != modeBuy || m.dlg.step != 1 || m.dlg.qty.Value() != strconv.Itoa(fits) || !strings.Contains(m.dlg.err, "what fits") {
		t.Fatalf("the refusal: mode %v step %d qty %q err %q, want %d", m.mode, m.dlg.step, m.dlg.qty.Value(), m.dlg.err, fits)
	}
	if m.w.Stock(city, id) != before {
		t.Fatal("the refused buy bought something")
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	if got := m.w.Stock(city, id); got != before+fits {
		t.Fatalf("the offer: stock %d, want %d (%q)", got, before+fits, m.dlg.err)
	}
}

// R on the market (#356) opens the restock: the plan under the days,
// nothing bought until enter, then the plan's lines bought into the
// cart.
func TestRestockFillsTheCart(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Player.DirtyCash = 10_000_000
	city := m.w.Player.Location
	m.Update(key("2"))
	m.Update(key("R"))
	if m.mode != modeRestock {
		t.Fatalf("R on the market: mode %v, status %q", m.mode, m.status)
	}
	plan := m.sess.RestockPlan(city, restockDays)
	if len(plan) == 0 {
		t.Fatal("the rich fixture restocks nothing")
	}
	buys := len(m.w.Today.Buys)
	view := stripANSI(m.View())
	for _, l := range plan {
		if !strings.Contains(view, m.w.ProductName(l.Product)) {
			t.Errorf("the plan's %s is not in the dialog:\n%s", l.Product, view)
		}
	}
	if len(m.w.Today.Buys) != buys {
		t.Fatal("the dialog bought before enter")
	}
	want := map[string]int{}
	for _, l := range plan {
		want[l.Product] = m.w.Stock(city, l.Product) + l.Units
	}
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "Restocked") {
		t.Fatalf("enter: mode %v, status %q", m.mode, m.status)
	}
	if got := len(m.w.Today.Buys) - buys; got != len(plan) {
		t.Errorf("%d buys in the cart, the plan had %d lines", got, len(plan))
	}
	for id, n := range want {
		if got := m.w.Stock(city, id); got != n {
			t.Errorf("%s: stock %d, want %d", id, got, n)
		}
	}
	// A second restock has nothing to buy, and says so.
	m.Update(key("R"))
	m.Update(key("enter"))
	if m.mode != modeRestock || !strings.HasPrefix(m.amt.err, "Nothing to buy") {
		t.Errorf("a restock of a stocked stash: mode %v, err %q", m.mode, m.amt.err)
	}
}
