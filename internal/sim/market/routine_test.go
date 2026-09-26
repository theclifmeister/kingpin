package market_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestContractHeldByTheRoadSaysSo (#503): a playtest's Eastside contract
// counted a route's heroin on the road and stopped buying, two weeks of
// stockouts with nothing saying why. A contract of yours held under its
// level by stock on the road to it emits SupplyShort with Why "road" and
// the units it counts; with nothing on the road it says nothing.
func TestContractHeldByTheRoadSaysSo(t *testing.T) {
	cfg := content.MustLoad()
	for _, road := range []int{0, 60} {
		w, _, c := marketOnly(t, cfg, 11)
		home, weed := w.Home().ID, w.Products[0]
		w.Player.DirtyCash, w.Player.CarryLimit = 1_000_000, 1000
		if err := w.SetSupply(home, weed, 200); err != nil {
			t.Fatal(err)
		}
		if road > 0 {
			w.Shipments = append(w.Shipments, game.Shipment{ID: 1, Route: "coast", From: "bayport", To: home, Product: weed, Units: road, Sent: w.Day, Arrives: w.Day + 3})
		}
		var held []events.SupplyShort
		for _, e := range c.EndDay(w) {
			if ev, ok := e.(events.SupplyShort); ok && ev.Why == events.SupplyRoad {
				held = append(held, ev)
			}
		}
		if got := w.Stock(home, weed); got != 200-road {
			t.Fatalf("road %d: the contract kept %d, want %d", road, got, 200-road)
		}
		switch {
		case road == 0 && len(held) != 0:
			t.Fatalf("nothing on the road, and it says it holds: %+v", held)
		case road > 0 && (len(held) != 1 || held[0].Short != road || held[0].City != home || held[0].Product != weed):
			t.Fatalf("road %d: held %+v, want one of %d", road, held, road)
		}
	}
}

// TestLieLowHoldsTheHandoff (#503): a playtest queued a handoff, lay low,
// and it was gone the next morning with no word. Lying low keeps the
// handoff queued and hands nothing over (everyone's day off), and the
// night says so with HandoffHeld; turned off again, it goes.
func TestLieLowHoldsTheHandoff(t *testing.T) {
	cfg := content.MustLoad()
	for _, low := range []bool{true, false} {
		w, _, c := marketOnly(t, cfg, 11)
		home, weed := w.Home().ID, w.Products[0]
		w.SetStock(home, weed, 40)
		k := w.OfferContract(game.Contract{Buyer: "x", Name: "a tester", City: home, Product: weed, Units: 30, Premium: 1.5, HeatMul: 0.4, Expires: w.Day + 2, Due: w.Day + 5})
		if err := w.AcceptContract(k.ID); err != nil {
			t.Fatal(err)
		}
		if err := w.Deliver(k.ID, 25); err != nil {
			t.Fatal(err)
		}
		w.SetLieLow(true)
		if w.QueuedDelivery(k.ID) != 25 {
			t.Fatalf("lying low dropped the handoff")
		}
		if !low {
			w.SetLieLow(false)
		}
		var held, handed int
		for _, e := range c.EndDay(w) {
			switch ev := e.(type) {
			case events.HandoffHeld:
				held += ev.Units
				if ev.ID != k.ID || ev.Owed != 30 {
					t.Fatalf("held %+v", ev)
				}
			case events.ContractDelivered:
				handed += ev.Units
			}
		}
		if low && (held != 25 || handed != 0 || w.Stock(home, weed) != 40) {
			t.Fatalf("lying low: held %d, handed %d, stash %d", held, handed, w.Stock(home, weed))
		}
		if !low && (held != 0 || handed != 25) {
			t.Fatalf("lie low turned off: held %d, handed %d", held, handed)
		}
	}
}

// TestStandingForAllSellsTheStash (#503): a standing order's "blank =
// max" froze at the day's 16, and the report said "only 15 stashed"
// every night after. An order placed at game.AllUnits is kept as All:
// every night it is for whatever the stash holds, short only when it
// holds nothing.
func TestStandingForAllSellsTheStash(t *testing.T) {
	cfg := content.MustLoad()
	w, _, c := marketOnly(t, cfg, 11)
	home, weed := w.Home().ID, w.Products[0]
	w.Player.CarryLimit = 1000
	w.SetStock(home, weed, 16)
	if err := w.PlaceStanding(home, weed, game.AllUnits, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if o, _ := w.YourStanding(home, weed); !o.All || o.Qty != 16 {
		t.Fatalf("kept as %+v, want all of the 16", o)
	}
	for _, stash := range []int{15, 40, 0} {
		w.SetStock(home, weed, stash)
		var wanted int
		var short []events.StandingShort
		for _, e := range c.EndDay(w) {
			switch ev := e.(type) {
			case events.PlayerSold:
				wanted = ev.Wanted
			case events.StandingShort:
				short = append(short, ev)
			}
		}
		switch {
		case stash > 0 && (wanted != stash || len(short) != 0):
			t.Fatalf("stash %d: wanted %d, short %+v; want all of it and no shortfall", stash, wanted, short)
		case stash == 0 && (len(short) != 1 || !short[0].All):
			t.Fatalf("an empty stash: short %+v, want one for all of it", short)
		}
	}
}
