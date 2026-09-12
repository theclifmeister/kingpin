package game

import (
	"errors"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// PlaceStanding is checked as PlaceSell is (#114): a city, a product, a
// quantity above zero and no more than the stash plus what the supply
// contract brings; one per product per city, replaced by placing again;
// CancelStanding takes it off and leaves the map nil once it is empty,
// the pre-standing world.
func TestPlaceStanding(t *testing.T) {
	w := lieutenantWorld()
	w.Player.Stash["home"]["weed"] = 30
	for _, tc := range []struct {
		city, product string
		qty           int
		err           error
	}{
		{"nowhere", "weed", 10, ErrNoCity},
		{"home", "crack", 10, ErrUnknownProduct},
		{"home", "weed", 0, ErrBadQuantity},
		{"home", "weed", 10, nil},
		{"home", "weed", 30, nil},
	} {
		if err := w.PlaceStanding(tc.city, tc.product, tc.qty, events.DialNormal); !errors.Is(err, tc.err) {
			t.Fatalf("PlaceStanding(%s, %s, %d) = %v, want %v", tc.city, tc.product, tc.qty, err, tc.err)
		}
	}
	if err := w.PlaceStanding("home", "weed", 31, events.DialNormal); err == nil {
		t.Fatal("a standing order for more than the stash was placed")
	}
	if o, ok := w.YourStanding("home", "weed"); !ok || o.Qty != 30 || o.Dial != events.DialNormal || o.City != "home" || o.Product != "weed" {
		t.Fatalf("the standing order: %+v %v", o, ok)
	}
	// A supply contract's shortfall counts, as it does for PlaceSell.
	if err := w.SetSupply("home", "weed", 50); err != nil {
		t.Fatal(err)
	}
	if err := w.PlaceStanding("home", "weed", 50, events.DialAggressive); err != nil {
		t.Fatalf("a standing order for the level: %v", err)
	}
	if o, _ := w.YourStanding("home", "weed"); o.Qty != 50 || o.Dial != events.DialAggressive {
		t.Fatalf("placing again did not replace: %+v", o)
	}
	w.CancelStanding("home", "weed")
	if _, ok := w.YourStanding("home", "weed"); ok || w.Standing != nil {
		t.Fatalf("after cancelling: %v", w.Standing)
	}
	w.CancelStanding("home", "weed") // nothing to cancel is fine
}

// StandingOrder is yours first, then the lieutenant's (#114): in a city
// a lieutenant runs, a standing order you set is the one that stands;
// cancel it and theirs is back; DelegatedOrder is theirs alone.
func TestStandingOrderIsYoursFirst(t *testing.T) {
	w := lieutenantWorld()
	w.Player.Stash["hub"]["weed"] = 40
	if err := w.Assign(2, "hub"); err != nil {
		t.Fatal(err)
	}
	w.Delegate("hub", "weed", 10, events.DialQuiet)
	if o, ok := w.StandingOrder("hub", "weed"); !ok || o.Qty != 10 {
		t.Fatalf("the lieutenant's order does not stand: %+v %v", o, ok)
	}
	if err := w.PlaceStanding("hub", "weed", 25, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if o, ok := w.StandingOrder("hub", "weed"); !ok || o.Qty != 25 || o.Dial != events.DialNormal {
		t.Fatalf("yours does not win: %+v %v", o, ok)
	}
	if o, ok := w.DelegatedOrder("hub", "weed"); !ok || o.Qty != 10 {
		t.Fatalf("the lieutenant's is gone: %+v %v", o, ok)
	}
	w.CancelStanding("hub", "weed")
	if o, ok := w.StandingOrder("hub", "weed"); !ok || o.Qty != 10 {
		t.Fatalf("the lieutenant's order is not back: %+v %v", o, ok)
	}
	if _, ok := w.StandingOrder("home", "weed"); ok {
		t.Fatal("an order stands in a city nobody runs")
	}
}

// The clock clears the day's orders and never a standing one (#114),
// and lying low leaves it standing too: it is the market that sells
// nothing that day.
func TestClockKeepsTheStandingOrders(t *testing.T) {
	w := lieutenantWorld()
	w.Player.Stash["home"]["weed"] = 30
	if err := w.PlaceStanding("home", "weed", 20, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if err := w.PlaceSell("home", "weed", 5, events.DialQuiet); err != nil {
		t.Fatal(err)
	}
	c := NewClock(nil)
	c.EndDay(w)
	if _, ok := w.Order("home", "weed"); ok {
		t.Fatal("the day's order survived the clock")
	}
	if o, ok := w.YourStanding("home", "weed"); !ok || o.Qty != 20 {
		t.Fatalf("the standing order did not survive the clock: %+v %v", o, ok)
	}
	w.SetLieLow(true)
	if o, ok := w.YourStanding("home", "weed"); !ok || o.Qty != 20 {
		t.Fatalf("lying low cancelled the standing order: %+v %v", o, ok)
	}
}
