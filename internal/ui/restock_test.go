package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
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

// The restock says what it buys (#470): it sizes to what the corners
// could sell, an order or not, so its `sells` column names the order
// that sells each line tonight, `none` where no order does, and a line
// under the plan names those products.
func TestRestockSaysWhatNoOrderSells(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Player.DirtyCash = 10_000_000
	city := m.w.Player.Location
	plan := m.sess.RestockPlan(city, restockDays)
	if len(plan) < 2 {
		t.Fatalf("the rich fixture restocks %d lines", len(plan))
	}
	for _, l := range plan {
		m.sess.CancelSell(city, l.Product)
		m.sess.CancelStanding(city, l.Product)
	}
	sold := plan[0].Product
	if err := m.sess.PlaceStanding(city, sold, 7, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	m.Update(key("2"))
	m.Update(key("R"))
	plain := strings.Join(strings.Fields(stripANSI(m.View())), " ")
	if !strings.Contains(plain, "sells") || !strings.Contains(plain, "7 normal ↻") || !strings.Contains(plain, "none") {
		t.Fatalf("no sells column:\n%s", plain)
	}
	note := strings.Index(plain, "No order sells")
	if note < 0 || !strings.Contains(plain[note:], m.w.ProductName(plan[1].Product)) {
		t.Fatalf("no line naming what no order sells:\n%s", plain)
	}
	if strings.Contains(plain[note:strings.Index(plain[note:], "tonight")+note], m.w.ProductName(sold)) {
		t.Fatalf("the line names %s, which a standing order sells:\n%s", sold, plain)
	}
}
