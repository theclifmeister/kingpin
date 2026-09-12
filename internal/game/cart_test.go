package game

import (
	"errors"
	"math"
	"testing"
)

// receipt is what a buy moves and a return must put back.
type receipt struct {
	cash, stock, bought int
	price               float64
}

func tally(w *World, city, product string) receipt {
	m := w.Product(city, product)
	return receipt{w.Player.DirtyCash, w.Stock(city, product), m.BoughtToday, m.SupplierPrice}
}

// Return is the exact inverse of Buy (#103): returning the whole of a
// buy leaves cash, stash, BoughtToday and the supplier price as they
// were and the buy out of the day's receipts; a part of one is returned
// in proportion, the cost at the price paid and the nudge scaled to
// what is kept; the last buy is undone first.
func TestReturnIsTheInverseOfBuy(t *testing.T) {
	w := testWorld()
	priceAt(w, "test", "a", 10)
	before := tally(w, "test", "a")
	if _, err := w.Buy("street", "a", 10, false, 0.5); err != nil {
		t.Fatal(err)
	}
	after := tally(w, "test", "a")
	if after.cash != before.cash-100 || after.stock != 10 || after.bought != 10 || after.price <= before.price {
		t.Fatalf("the buy: %+v -> %+v", before, after)
	}
	if got := w.Buys[0].Prior; got != before.price {
		t.Fatalf("the receipt records a prior price of %v, not %v", got, before.price)
	}
	if w.Bought("test", "a") != 10 {
		t.Fatalf("bought today = %d", w.Bought("test", "a"))
	}
	// Half back: half the cost, half the nudge, the receipt halved.
	refund, err := w.Return("test", "a", 5)
	if err != nil || refund != 50 {
		t.Fatalf("return 5: %d %v", refund, err)
	}
	half := tally(w, "test", "a")
	wantPrice := before.price * (1 + (after.price/before.price-1)*0.5)
	if half.cash != before.cash-50 || half.stock != 5 || half.bought != 5 || math.Abs(half.price-wantPrice) > 1e-9 {
		t.Fatalf("after half back: %+v, want price %v", half, wantPrice)
	}
	if len(w.Buys) != 1 || w.Buys[0].Qty != 5 || w.Buys[0].Cost != 50 {
		t.Fatalf("the receipt: %+v", w.Buys)
	}
	// The rest back: exactly as before, and no receipt.
	refund, err = w.Return("test", "a", 5)
	if err != nil || refund != 50 {
		t.Fatalf("return the rest: %d %v", refund, err)
	}
	if got := tally(w, "test", "a"); got != before {
		t.Fatalf("after a full return: %+v, want %+v", got, before)
	}
	if w.Buys != nil || w.Bought("test", "a") != 0 {
		t.Fatalf("receipts left: %+v", w.Buys)
	}

	// Two buys: the second is undone first, then the first, and the
	// price walks back through both prior prices.
	if _, err := w.Buy("street", "a", 4, false, 0.5); err != nil {
		t.Fatal(err)
	}
	mid := tally(w, "test", "a")
	if _, err := w.Buy("street", "a", 6, false, 0.5); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Return("test", "a", 6); err != nil {
		t.Fatal(err)
	}
	if got := tally(w, "test", "a"); got != mid || len(w.Buys) != 1 || w.Buys[0].Qty != 4 {
		t.Fatalf("after undoing the second buy: %+v, want %+v; receipts %+v", got, mid, w.Buys)
	}
	if _, err := w.Return("test", "a", 4); err != nil {
		t.Fatal(err)
	}
	if got := tally(w, "test", "a"); got != before || w.Buys != nil {
		t.Fatalf("after undoing both: %+v, want %+v", got, before)
	}
}

// A return is refused for nothing, for more than was bought, for units
// that have left the stash, after the day ends and after the run.
func TestReturnRefusals(t *testing.T) {
	w := testWorld()
	if _, err := w.Return("test", "a", 1); err != ErrNothingBought {
		t.Fatalf("nothing bought: %v", err)
	}
	if _, err := w.Buy("street", "a", 10, false, 0.5); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Return("test", "a", 0); err != ErrBadQuantity {
		t.Fatalf("zero: %v", err)
	}
	if _, err := w.Return("test", "b", 1); err != ErrUnknownProduct {
		t.Fatalf("no such product: %v", err)
	}
	if _, err := w.Return("test", "a", 11); err == nil {
		t.Fatal("more than was bought")
	}
	// On the road: the stash no longer holds them.
	w.Stash("test")["a"] = 3
	if _, err := w.Return("test", "a", 5); !errors.Is(err, ErrReturnGone) {
		t.Fatalf("gone: %v", err)
	}
	if _, err := w.Return("test", "a", 3); err != nil {
		t.Fatalf("what is left: %v", err)
	}
	w.Stash("test")["a"] = 7
	// The day ends: the receipts are gone with it.
	NewClock(nil, &counter{}).EndDay(w)
	if w.Buys != nil {
		t.Fatalf("receipts survived the night: %+v", w.Buys)
	}
	if _, err := w.Return("test", "a", 1); err != ErrNothingBought {
		t.Fatalf("the day after: %v", err)
	}
	w.Over = &Ending{Day: w.Day, Cause: "test"}
	if _, err := w.Return("test", "a", 1); err != ErrGameOver {
		t.Fatalf("after the end: %v", err)
	}
}
