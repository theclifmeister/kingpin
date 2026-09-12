package game

import (
	"errors"
	"testing"
)

// A supply contract (#113) is a persistent setting keyed like an order:
// set, replaced, cleared; refused for an unknown product, a city whose
// supplier does not sell it, or no units; allowed where the player
// holds no capacity.
func TestSetSupply(t *testing.T) {
	w := twoCityWorld()
	if err := w.SetSupply("test", "a", 40); err != nil {
		t.Fatal(err)
	}
	c, ok := w.Supplied("test", "a")
	if !ok || c != (SupplyContract{City: "test", Product: "a", Units: 40, Since: 0}) {
		t.Fatalf("the contract: %+v %v", c, ok)
	}
	if err := w.SetSupply("test", "a", 60); err != nil || w.Supply[SupplyKey("test", "a")].Units != 60 {
		t.Fatalf("replacing it: %v %+v", err, w.Supply)
	}
	// Elsewhere, with no capacity there: allowed, it fills as far as
	// Free allows.
	if w.Capacity("port") != 0 {
		t.Fatalf("capacity in port %d", w.Capacity("port"))
	}
	if err := w.SetSupply("port", "a", 10); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		city, product string
		units         int
		want          error
	}{
		{"nowhere", "a", 10, ErrNoCity},
		{"test", "zzz", 10, ErrUnknownProduct},
		{"test", "a", 0, ErrBadQuantity},
		{"test", "a", -1, ErrBadQuantity},
	} {
		if err := w.SetSupply(c.city, c.product, c.units); !errors.Is(err, c.want) {
			t.Errorf("SetSupply(%s, %s, %d) = %v, want %v", c.city, c.product, c.units, err, c.want)
		}
	}
	w.Product("port", "a").NoSupply = true
	if err := w.SetSupply("port", "a", 10); !errors.Is(err, ErrNotSupplied) {
		t.Errorf("a product the supplier does not sell: %v", err)
	}
	w.ClearSupply("test", "a")
	w.ClearSupply("port", "a")
	if w.Supply != nil {
		t.Fatalf("cleared: %+v", w.Supply)
	}
	w.ClearSupply("test", "a") // nothing to clear is fine
}

// FillSupply is the same path as Buy (#113): with a markup of one the
// stash, the cash, BoughtToday and the supplier's price move exactly as
// a buy by hand moves them, and the receipt is marked a contract's and
// dated for the morning; the markup is on the unit price alone.
func TestFillSupplyIsBuysPath(t *testing.T) {
	hand, contract := testWorld(), testWorld()
	priceAt(hand, "test", "a", 10)
	priceAt(contract, "test", "a", 10)
	if _, err := hand.Buy("street", "a", 20, false, 0.5); err != nil {
		t.Fatal(err)
	}
	p, err := contract.FillSupply("street", "a", 20, 1, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := tally(hand, "test", "a"), tally(contract, "test", "a"); a != b {
		t.Fatalf("a contract's buy moved %+v, a buy by hand %+v", b, a)
	}
	if !p.Contract || p.Day != 1 || p.Cost != hand.Buys[0].Cost || p.UnitPrice != hand.Buys[0].UnitPrice {
		t.Fatalf("the receipt: %+v against the hand's %+v", p, hand.Buys[0])
	}
	// The markup: a fifth more a unit, the rest the same.
	marked := testWorld()
	priceAt(marked, "test", "a", 10)
	q, err := marked.FillSupply("street", "a", 20, 1.2, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if q.UnitPrice != 12 || q.Cost != 240 || marked.Player.DirtyCash != 500-240 || marked.Stock("test", "a") != 20 {
		t.Fatalf("at a markup of 1.2: %+v, cash %d", q, marked.Player.DirtyCash)
	}
	if marked.Home().Market["a"].SupplierPrice != contract.Home().Market["a"].SupplierPrice {
		t.Fatalf("the markup moved the pressure: %v against %v", marked.Home().Market["a"].SupplierPrice, contract.Home().Market["a"].SupplierPrice)
	}
	// Refused as a buy is: no such connect, none of that here, no units,
	// the supplier not selling it, more than the stash can hold.
	if _, err := marked.FillSupply("nowhere", "a", 1, 1, 0); !errors.Is(err, ErrNoSupplier) {
		t.Errorf("no such connect: %v", err)
	}
	if _, err := marked.FillSupply("street", "zzz", 1, 1, 0); !errors.Is(err, ErrUnknownProduct) {
		t.Errorf("no such product: %v", err)
	}
	if _, err := marked.FillSupply("street", "a", 0, 1, 0); !errors.Is(err, ErrBadQuantity) {
		t.Errorf("no units: %v", err)
	}
	if _, err := marked.FillSupply("street", "a", 1000, 1, 0); err == nil {
		t.Error("more than the stash holds was allowed")
	}
	marked.Home().Market["a"].NoSupply = true
	if _, err := marked.FillSupply("street", "a", 1, 1, 0); !errors.Is(err, ErrNotSupplied) {
		t.Errorf("a product the supplier does not sell: %v", err)
	}
}

// The clock keeps the morning's contract receipts through the day and
// drops the buys by hand and yesterday's (#113): the cart shows them
// and ReturnSupplied takes them back at the price paid, leaving the
// supplier's price alone (Prior is cleared: the market has reset it
// since); a run with no contract keeps the scratch nil as before.
func TestClockKeepsTheContractReceipts(t *testing.T) {
	w := testWorld()
	priceAt(w, "test", "a", 10)
	if _, err := w.Buy("street", "a", 5, false, 0); err != nil {
		t.Fatal(err)
	}
	c := NewClock(nil, &counter{})
	c.EndDay(w)
	if w.Buys != nil {
		t.Fatalf("the buys by hand survived the day: %+v", w.Buys)
	}
	// A contract's receipt made in the tick: the market sim's FillSupply
	// stands in for by a sim that fills it.
	c = NewClock(nil, simFunc(func(w *World, t *Tick) {
		if _, err := w.FillSupply("street", "a", 8, 1.5, 0.5); err != nil {
			panic(err)
		}
	}))
	c.EndDay(w)
	if len(w.Buys) != 1 || !w.Buys[0].Contract || w.Buys[0].Day != w.Day || w.Buys[0].Prior != 0 || w.Buys[0].Qty != 8 {
		t.Fatalf("the morning's receipts: %+v (day %d)", w.Buys, w.Day)
	}
	if w.SuppliedIn("test", "a") != 8 || w.Bought("test", "a") != 0 {
		t.Fatalf("supplied %d, bought by hand %d", w.SuppliedIn("test", "a"), w.Bought("test", "a"))
	}
	if units, cost := w.SuppliedToday(); units != 8 || cost != w.Buys[0].Cost {
		t.Fatalf("supplied today: %d for %d", units, cost)
	}
	// A buy by hand beside it: Return takes the hand's, ReturnSupplied
	// the contract's, and neither the other's.
	price := w.Home().Market["a"].SupplierPrice
	if _, err := w.Buy("street", "a", 3, false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Return("test", "a", 4); err == nil {
		t.Fatal("Return took back more than the hand bought")
	}
	cash := w.Player.DirtyCash
	refund, err := w.ReturnSupplied("test", "a", 8)
	if err != nil || refund != 120 {
		t.Fatalf("returning the contract's 8 at $15: %d %v", refund, err)
	}
	if w.Player.DirtyCash != cash+120 || w.Stock("test", "a") != 5+3 || len(w.Buys) != 1 || w.Buys[0].Contract {
		t.Fatalf("after the return: cash %d stock %d buys %+v", w.Player.DirtyCash, w.Stock("test", "a"), w.Buys)
	}
	if w.Home().Market["a"].SupplierPrice != price {
		t.Fatalf("the return walked the price to %v from %v", w.Home().Market["a"].SupplierPrice, price)
	}
	if _, err := w.ReturnSupplied("test", "a", 1); !errors.Is(err, ErrNothingBought) {
		t.Errorf("nothing of the contract's left: %v", err)
	}
	// Yesterday's contract receipts go with the day.
	c = NewClock(nil, &counter{})
	c.EndDay(w)
	if w.Buys != nil {
		t.Fatalf("yesterday's receipts survived: %+v", w.Buys)
	}
}

// simFunc is a Simulation from a function, for the clock tests.
type simFunc func(w *World, t *Tick)

func (simFunc) Name() string             { return "func" }
func (f simFunc) Step(w *World, t *Tick) { f(w, t) }

// A sell order may count on what the contract brings in the morning
// (#113): the stash plus the shortfall, no more.
func TestPlaceSellCountsOnTheContract(t *testing.T) {
	w := testWorld()
	w.SetStock("test", "a", 10)
	if err := w.PlaceSell("test", "a", 11, 1); err == nil {
		t.Fatal("an order over the stash with no contract was allowed")
	}
	if err := w.SetSupply("test", "a", 30); err != nil {
		t.Fatal(err)
	}
	if w.SupplyDue("test", "a") != 20 {
		t.Fatalf("due %d", w.SupplyDue("test", "a"))
	}
	if err := w.PlaceSell("test", "a", 30, 1); err != nil {
		t.Fatalf("an order for the stash and the contract's shortfall: %v", err)
	}
	if err := w.PlaceSell("test", "a", 31, 1); err == nil {
		t.Fatal("an order over the stash and the shortfall was allowed")
	}
	w.ClearSupply("test", "a")
	if w.SupplyDue("test", "a") != 0 {
		t.Fatalf("due %d with no contract", w.SupplyDue("test", "a"))
	}
}
