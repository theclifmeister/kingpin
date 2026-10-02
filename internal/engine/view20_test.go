package engine_test

import "testing"

// TestViewCarriesTheBuysAndTheConnects (#582): view 20 carries what
// Street Edition's cart and connects read: today's buys as the world
// has them, the room a city keeps without you (World.CapacityAway), a
// connect's door, terms and record, the corners worked and a product's
// glut and served demand.
func TestViewCarriesTheBuysAndTheConnects(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	here := w.Player.Location
	if v := s.View(); v.Buys == nil || len(v.Buys) != 0 {
		t.Fatalf("a fresh run's buys: %#v", v.Buys)
	}
	sup := w.StreetSupplier(here)
	if sup == nil {
		t.Fatal("no street connect where you stand")
	}
	pid := ""
	for _, id := range w.Products {
		if w.Available(sup, id) {
			pid = id
			break
		}
	}
	if _, err := s.Buy(sup.ID, pid, 2, false); err != nil {
		t.Fatal(err)
	}
	sup.Late, sup.FrozenUntil, sup.Warned = 2, w.Day+3, w.Day
	v := s.View()
	if len(v.Buys) != 1 {
		t.Fatalf("%d buys in the view", len(v.Buys))
	}
	b, want := v.Buys[0], w.Today.Buys[0]
	if b.City != here || b.Product != pid || b.Qty != 2 || b.Cost != want.Cost || b.Unit != want.UnitPrice || b.Supplier != sup.ID || b.Credit || b.Contract {
		t.Errorf("the buy: %+v, the world's %+v", b, want)
	}
	if got, want := v.You.Away[here], w.CapacityAway(here); got != want || got >= w.Capacity(here) {
		t.Errorf("away here %d, want %d (under the capacity %d: your carry leaves with you)", got, want, w.Capacity(here))
	}
	if v.You.StreetQuality != w.StreetQuality() {
		t.Errorf("street quality %v", v.You.StreetQuality)
	}
	for _, c := range v.Connects {
		if c.ID != sup.ID {
			continue
		}
		// Frozen today: day 0's warning is no warning (sup.Warned is 0, never).
		if c.DayCap != sup.Cap || c.FrozenUntil != w.Day+3 || c.Late != 2 || c.Warned != (w.Day > 0) {
			t.Errorf("the connect: %+v", c)
		}
	}
	for _, c := range v.Cities {
		if c.ID != here {
			continue
		}
		if c.Worked != w.WorkedIn(here) {
			t.Errorf("worked %d, want %d", c.Worked, w.WorkedIn(here))
		}
		for _, p := range c.Products {
			if p.Served != w.Demand(here, p.ID) || p.Glut != w.Product(here, p.ID).Glut {
				t.Errorf("%s: served %v glut %v", p.ID, p.Served, p.Glut)
			}
		}
	}
}
