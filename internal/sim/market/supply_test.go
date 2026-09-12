package market_test

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// marketOnly is a fresh world with the player on a corner at home and a
// market sim alone on the clock, so the supply contracts (#113) can be
// read without the other sims moving the cash.
func marketOnly(t *testing.T, cfg *content.Config, seed uint64) (*game.World, *market.Sim, *game.Clock) {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	if err := w.Post(w.Home().Corners[0].ID, game.You); err != nil {
		t.Fatal(err)
	}
	mk, err := market.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return w, mk, game.NewClock(nil, mk)
}

// A contract buys through the supplier's price pressure exactly as a
// buy by hand does: with the markup at one, two days of a 200-unit
// contract leave the cash, the stash and every price where two 200-unit
// buys by hand leave them; and at the file's markup the contract pays
// the markup a unit and nothing else moves.
func TestSupplyMatchesTheHand(t *testing.T) {
	cfg := content.MustLoad()
	flat := *cfg
	flat.Market.Supply.Markup = 1
	hand, _, hc := marketOnly(t, &flat, 11)
	kept, mk, kc := marketOnly(t, &flat, 11)
	home, weed := hand.Home().ID, hand.Products[0]
	for _, w := range []*game.World{hand, kept} {
		w.Player.DirtyCash = 1_000_000
		w.Player.CarryLimit = 1000
	}
	if err := kept.SetSupply(home, weed, 200); err != nil {
		t.Fatal(err)
	}
	pressure := mk.BuyPressure(hand)
	for day := 1; day <= 2; day++ {
		// The hand buys in the morning what the contract will buy at the
		// top of the tick, out of the same stash and the same price.
		need := 200 - hand.Stock(home, weed)
		if _, err := hand.Buy(hand.StreetSupplier(home).ID, weed, need, false, pressure); err != nil {
			t.Fatal(err)
		}
		if err := hand.PlaceSell(home, weed, 200, events.DialNormal); err != nil {
			t.Fatal(err)
		}
		if err := kept.PlaceSell(home, weed, 200, events.DialNormal); err != nil {
			t.Fatal(err)
		}
		hc.EndDay(hand)
		kc.EndDay(kept)
		a, b := hand.Product(home, weed), kept.Product(home, weed)
		if hand.Player.DirtyCash != kept.Player.DirtyCash || hand.Stock(home, weed) != kept.Stock(home, weed) || a.Price != b.Price || a.SupplierPrice != b.SupplierPrice || a.Glut != b.Glut {
			t.Fatalf("day %d: by hand cash %d stock %d %+v; by contract cash %d stock %d %+v", day,
				hand.Player.DirtyCash, hand.Stock(home, weed), *a, kept.Player.DirtyCash, kept.Stock(home, weed), *b)
		}
		if n, _ := kept.SuppliedToday(); n != need {
			t.Fatalf("day %d: the contract bought %d, the hand %d", day, n, need)
		}
	}
	// The markup: the same units at markup times the price, the pressure
	// they leave the same.
	marked, mk2, mc := marketOnly(t, cfg, 11)
	marked.Player.DirtyCash, marked.Player.CarryLimit = 1_000_000, 1000
	if err := marked.SetSupply(home, weed, 200); err != nil {
		t.Fatal(err)
	}
	before := marked.Product(home, weed).SupplierPrice
	var bought events.SupplyBought
	for _, e := range mc.EndDay(marked) {
		if ev, ok := e.(events.SupplyBought); ok {
			bought = ev
		}
	}
	if bought.Units != 200 || fmt.Sprintf("%.6f", bought.Price) != fmt.Sprintf("%.6f", before*mk2.Markup()) || mk2.Markup() <= 1 {
		t.Fatalf("at the markup: %+v, supplier was %v, markup %v", bought, before, mk2.Markup())
	}
}

// The plan (#113): with cash and room the shortfall; short of cash what
// the budget over the float leaves, the products before it in the
// ladder served first; short of room Free(city); nothing for a stash at
// its level or one on the road to it, and nothing with no contract.
func TestSupplyPlan(t *testing.T) {
	cfg := content.MustLoad()
	w, mk, _ := marketOnly(t, cfg, 3)
	home, weed, pills := w.Home().ID, w.Products[0], w.Products[1]
	w.Player.DirtyCash, w.Player.CarryLimit = 1_000_000, 500
	if plan := mk.Plan(w); plan != nil {
		t.Fatalf("a plan with no contract: %+v", plan)
	}
	if err := w.SetSupply(home, weed, 100); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSupply(home, pills, 50); err != nil {
		t.Fatal(err)
	}
	plan := mk.Plan(w)
	if len(plan) != 2 || plan[0].Units != 100 || plan[1].Units != 50 || plan[0].Why != "" || plan[1].Why != "" {
		t.Fatalf("with cash and room: %+v", plan)
	}
	if mk.Due(w, home, pills) != 50 || mk.Due(w, home, weed) != 100 {
		t.Fatalf("due: weed %d pills %d", mk.Due(w, home, weed), mk.Due(w, home, pills))
	}
	// Room: the weed takes it first, the pills get what is left.
	w.Player.CarryLimit = 120
	plan = mk.Plan(w)
	if plan[0].Units != 100 || plan[1].Units != 20 || plan[1].Why != "room" || plan[1].Short != 50 {
		t.Fatalf("short of room: %+v", plan)
	}
	// Cash: what is over the float, the weed first, never under it.
	w.Player.CarryLimit = 500
	unit := w.Product(home, weed).SupplierPrice * mk.Markup()
	w.Player.DirtyCash = mk.Float(w) + int(unit*60)
	plan = mk.Plan(w)
	if plan[0].Units > 60 || plan[0].Units < 59 || plan[0].Why != "cash" || plan[1].Units != 0 || plan[1].Why != "cash" || plan[0].Cost > mk.Budget(w) {
		t.Fatalf("short of cash (budget %d, unit %v): %+v", mk.Budget(w), unit, plan)
	}
	// A float from the file holds the line: with [supply] float set the
	// contract leaves it in the till.
	floated := *cfg
	floated.Market.Supply.Float = 10_000
	w2, _, c2 := marketOnly(t, &floated, 3)
	w2.Player.DirtyCash, w2.Player.CarryLimit = 10_000+int(unit*30), 500
	if err := w2.SetSupply(home, weed, 100); err != nil {
		t.Fatal(err)
	}
	c2.EndDay(w2)
	if n, _ := w2.SuppliedToday(); n < 29 || n > 30 || w2.Player.DirtyCash < 10_000 {
		t.Fatalf("over a float of $10,000: bought %d, cash %d", n, w2.Player.DirtyCash)
	}
	// At the level, or with the shortfall on the road: nothing.
	w.Player.DirtyCash = 1_000_000
	w.SetStock(home, weed, 100)
	w.Shipments = append(w.Shipments, game.Shipment{To: home, Product: pills, Units: 50, Arrives: 5})
	if plan := mk.Plan(w); len(plan) != 0 {
		t.Fatalf("at the level and on the road: %+v", plan)
	}
}

// A run with contracts is deterministic: the step buys with no dice, so
// the same seed and the same contracts replay the same events; and a
// run with none is byte-for-byte the run before the contracts (the
// receipts scratch stays nil).
func TestSupplyIsDiceless(t *testing.T) {
	cfg := content.MustLoad()
	play := func() []events.Event {
		w, _, c := marketOnly(t, cfg, 5)
		w.Player.DirtyCash = 100_000
		if err := w.SetSupply(w.Home().ID, w.Products[0], 50); err != nil {
			t.Fatal(err)
		}
		var all []events.Event
		for i := 0; i < 5; i++ {
			all = append(all, c.EndDay(w)...)
		}
		return all
	}
	a, b := play(), play()
	if fmt.Sprintf("%#v", a) != fmt.Sprintf("%#v", b) {
		t.Fatal("two runs with the same contract differ")
	}
	w, _, c := marketOnly(t, cfg, 5)
	c.EndDay(w)
	if w.Buys != nil {
		t.Fatalf("the receipts scratch of a run with no contract: %+v", w.Buys)
	}
}
