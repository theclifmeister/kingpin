package game

import (
	"errors"
	"fmt"
	"testing"
)

// A buy past the room is refused with a RoomError (#356): it matches
// ErrNoRoom, carries the room there is and names the city, and the cut
// and the cook refuse the same way.
func TestErrNoRoomIsTyped(t *testing.T) {
	w := twoCityWorld()
	priceAt(w, "test", "a", 10)
	w.Player.DirtyCash = 1_000_000
	w.Supplier("street").Cap = 1_000_000
	free := w.Free("test")
	if free <= 0 {
		t.Fatalf("no room in the fixture: %d", free)
	}
	_, err := w.Buy("street", "a", free+1, false, 0)
	var room *RoomError
	if !errors.Is(err, ErrNoRoom) || !errors.As(err, &room) || room.Free != free || room.City != w.CityName("test") {
		t.Fatalf("a buy one past the room: %v (%+v), want Free %d", err, room, free)
	}
	if err.Error() != fmt.Sprintf("can only hold %d more units in Testville", free) {
		t.Errorf("the words moved: %q", err)
	}
	if _, err := w.Buy("street", "a", free, false, 0); err != nil {
		t.Fatalf("a buy of the room: %v", err)
	}
	// Full: the cut past the room is refused the same way.
	if _, err := w.Cut("test", "a", 0.5, 1, 0, 0, ""); !errors.Is(err, ErrNoRoom) {
		t.Errorf("a cut into a full stash: %v", err)
	}
}

// Buying MaxBuy is never refused (#356), whichever of the cash, the room
// and the connect's day binds, and one more is, for the reason that
// bound it.
func TestMaxBuyNeverRefused(t *testing.T) {
	for _, c := range []struct {
		name       string
		cash, left int
		want       error
	}{
		{"cash", 55, 1_000, nil},
		{"room", 1_000_000, 1_000, ErrNoRoom},
		{"their day", 1_000_000, 3, ErrSupplierCapacity},
		{"small lot", 1_000_000, 1_000, ErrNoRoom},
	} {
		w := twoCityWorld()
		priceAt(w, "test", "a", 10)
		sup := w.Supplier("street")
		sup.Cap = c.left
		if c.name == "small lot" {
			sup.Lot, sup.SmallLot = 1_000, 1.5
		}
		w.Player.DirtyCash = c.cash
		n := w.MaxBuy(sup, "a", false)
		if n <= 0 {
			t.Fatalf("%s: MaxBuy is %d", c.name, n)
		}
		p, err := w.Buy("street", "a", n, false, 0)
		if err != nil || p.Qty != n {
			t.Fatalf("%s: buying MaxBuy's %d: %v", c.name, n, err)
		}
		w2 := twoCityWorld()
		priceAt(w2, "test", "a", 10)
		sup2 := w2.Supplier("street")
		sup2.Cap = c.left
		if c.name == "small lot" {
			sup2.Lot, sup2.SmallLot = 1_000, 1.5
		}
		w2.Player.DirtyCash = c.cash
		_, err = w2.Buy("street", "a", n+1, false, 0)
		var short *ShortError
		switch {
		case err == nil:
			t.Errorf("%s: one past MaxBuy's %d was bought", c.name, n)
		case c.want == nil && !errors.As(err, &short):
			t.Errorf("%s: one past MaxBuy: %v, want short of cash", c.name, err)
		case c.want != nil && !errors.Is(err, c.want):
			t.Errorf("%s: one past MaxBuy: %v, want %v", c.name, err, c.want)
		}
	}
	w := twoCityWorld()
	if n := w.MaxBuy(nil, "a", false); n != 0 {
		t.Errorf("no connect: %d", n)
	}
}

// The restock plan (#356) tops each product up to StockLevels less what
// is stashed and on the road, from the cheapest connect, cut to the
// room and to the cash over what it keeps, and changes nothing.
func TestRestockPlan(t *testing.T) {
	w := twoCityWorld()
	priceAt(w, "test", "a", 10)
	w.Supplier("street").Cap = 1_000_000
	w.Player.DirtyCash = 1_000_000
	levels := w.StockLevels("test", 2)
	if levels["a"] <= 0 {
		t.Fatalf("no level for a: %v (demand %.1f)", levels, w.Demand("test", "a"))
	}
	w.AddStock("test", "a", 1, 1)
	before := w.Player.DirtyCash
	plan := w.RestockPlan("test", 2, 0)
	if w.Player.DirtyCash != before || w.Stock("test", "a") != 1 {
		t.Fatal("the plan bought something")
	}
	var line *RestockLine
	for i := range plan {
		if plan[i].Product == "a" {
			line = &plan[i]
		}
	}
	if line == nil || line.Units != levels["a"]-1 || line.Have != 1 || line.Level != levels["a"] || line.Supplier != "street" || line.Cost != w.Quote(w.Supplier("street"), "a", line.Units, false) {
		t.Fatalf("the line for a: %+v (level %d)", line, levels["a"])
	}
	// Cash over what it keeps: keeping all but $25 buys two at $10.
	w.Player.DirtyCash = 1_025
	plan = w.RestockPlan("test", 2, 1_000)
	for _, l := range plan {
		if l.Product == "a" && (l.Units != 2 || l.Cost != 20) {
			t.Errorf("cut to the cash: %+v", l)
		}
	}
	// Nothing where you cannot buy.
	if plan := w.RestockPlan("port", 2, 0); plan != nil {
		t.Errorf("a plan in a city you are not in: %+v", plan)
	}
}
