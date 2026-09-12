package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The buy dialog follows the lieutenant (#174): b on the market turned
// to a city they run opens on that city's connects, the title naming
// them, the price line carrying the markup in words and the buy landing
// in the stash there through them; on the dashboard b is where you
// stand; without a lieutenant, b on the market turned to the other city
// is the pointer; and the market's keep column reads the lieutenant's
// contract as `120 (lt)`, the pane its row, and x leaves it alone.
func TestBuyDialogFollowsTheLieutenant(t *testing.T) {
	m := richModelSeeded(t, 100, 30, 11)
	w := m.w
	hub := w.CityOrder[1]
	weed := w.Products[0]
	lt := w.Crew.Member(4)
	if lt == nil || !lt.Lieutenant() {
		t.Fatal("the fixture's lieutenant is not member 4")
	}
	// A runner of theirs on a hub corner, so the stash there has room
	// (the fixture's split with the rival keeps every other corner).
	w.Rival.Deals = nil
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 6, Name: "Sly", Role: "runner", Skill: 50, Units: 500, Loyalty: 80, Nerve: 50, Wage: 50}) // room over the route stash the fixture leaves there
	posted := false
	for _, c := range w.City(hub).Corners {
		if w.Post(c.ID, 6) == nil {
			posted = true
			break
		}
	}
	if !posted {
		t.Fatal("no hub corner takes a runner")
	}
	m.Update(key("2"))
	m.Update(key("right"))
	if m.shown().ID != hub {
		t.Fatalf("the market is turned to %s", m.shown().ID)
	}
	// Nobody runs the hub yet: the pointer, not a dialog.
	m.Update(key("b"))
	if m.mode != modePlay || !strings.Contains(m.status, "nobody runs it for you") || !strings.Contains(m.status, screenPointer(screenCrew)) {
		t.Fatalf("b turned to a city nobody runs: mode %v status %q", m.mode, m.status)
	}
	// Given the hub, the buy dialog opens there, through them.
	if err := w.Assign(lt.ID, hub); err != nil {
		t.Fatal(err)
	}
	m.Update(key("b"))
	if m.mode != modeBuy || m.dialogCity() != hub || m.dlg.turned {
		t.Fatalf("b with a lieutenant: mode %v city %s turned %v status %q", m.mode, m.dialogCity(), m.dlg.turned, m.status)
	}
	for _, sup := range m.connectsHere() {
		if sup.City != hub {
			t.Fatalf("the connect step lists %s in %s", sup.Name, sup.City)
		}
	}
	if v := stripANSI(m.View()); !strings.Contains(v, strings.ToUpper("through "+lt.Name)) || !strings.Contains(v, strings.ToUpper("BUY · "+w.CityName(hub))) {
		t.Fatalf("the title does not name the city and the lieutenant:\n%s", v)
	}
	for i, id := range w.Products {
		if id == weed {
			m.cursor = i
		}
	}
	if m.dlg.pick {
		// The connect step, where two deal: the street connect, whose
		// lot the room there holds.
		for i, sup := range m.connectsHere() {
			if sup.ID == w.StreetSupplier(hub).ID {
				m.dlg.supplier = i
			}
		}
		m.Update(key("enter"))
	}
	if sup := m.buySupplier(weed); sup == nil || sup.City != hub {
		t.Fatalf("the buy's connect: %+v", sup)
	}
	m.Update(key("enter"))
	if m.dlg.step != 1 {
		t.Fatalf("after the product: step %d err %q", m.dlg.step, m.dlg.err)
	}
	want := "+5% through " + lt.Name
	if v := stripANSI(m.View()); !strings.Contains(v, want) {
		t.Fatalf("the price line does not carry %q:\n%s", want, v)
	}
	max := m.qtyMax()
	if max <= 0 || max > w.Free(hub) {
		t.Fatalf("the quantity's max %d, room in the hub %d", max, w.Free(hub))
	}
	before, cash := w.Stock(hub, weed), w.Player.DirtyCash
	m.dlg.qty.SetValue("5")
	m.Update(key("enter"))
	m.Update(key("enter"))
	if w.Stock(hub, weed) != before+5 || len(w.Today.Buys) == 0 || w.Today.Buys[len(w.Today.Buys)-1].Lieutenant != lt.Name {
		t.Fatalf("the buy: stock %d (was %d), receipts %+v, err %q", w.Stock(hub, weed), before, w.Today.Buys, m.dlg.err)
	}
	p := w.Today.Buys[len(w.Today.Buys)-1]
	if p.Cost != w.Quote(w.Supplier(p.Supplier), weed, 5, false) || float64(p.Cost) < 5*w.Supplier(p.Supplier).Price[weed] || cash-w.Player.DirtyCash != p.Cost {
		t.Fatalf("the receipt %+v against the quote %d, cash moved %d", p, w.Quote(w.Supplier(p.Supplier), weed, 5, false), cash-w.Player.DirtyCash)
	}
	if !strings.Contains(m.status, "through "+lt.Name) {
		t.Fatalf("the status: %q", m.status)
	}
	m.Update(key("esc"))
	assertFits(t, m.View(), 100, 30, "the market with a delegated city")

	// The keep column and the pane read the lieutenant's contract.
	w.DelegateSupply(hub, weed, 120)
	m.cursor = 0
	if v := stripANSI(m.View()); !strings.Contains(v, "120 (lt)") || !strings.Contains(v, "keep at 120 (lt)") {
		t.Fatalf("the keep column or the pane does not read the lieutenant's contract:\n%s", v)
	}
	if v := stripANSI(m.View()); strings.Contains(v, "clear the contract") {
		t.Fatalf("x offers to clear the lieutenant's contract:\n%s", v)
	}
	m.Update(key("x"))
	if _, ok := w.DelegatedSupplied(hub, weed); !ok {
		t.Fatal("x cleared the lieutenant's contract")
	}

	// On the dashboard b is where you stand, lieutenant or not.
	m.Update(key("1"))
	m.Update(key("b"))
	if m.mode != modeBuy || m.dialogCity() != w.Player.Location {
		t.Fatalf("b on the dashboard: mode %v city %s", m.mode, m.dialogCity())
	}
	if v := stripANSI(m.View()); strings.Contains(v, "THROUGH") {
		t.Fatalf("a buy where you stand names a lieutenant:\n%s", v)
	}
	m.Update(key("esc"))
}
