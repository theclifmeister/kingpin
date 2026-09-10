package harness

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The demand the market serves is exactly the sum over the corners the
// player works, product by product, on every day of a run that takes
// ground: nothing sells on a corner nobody is standing on.
func TestHeldDemandIsServed(t *testing.T) {
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	crewed := Crewed(cfg, 40)
	checked := 0
	res, err := Run(cfg, 3, 150, func(w *game.World) {
		crewed(w)
		for _, cid := range w.CityOrder {
			for _, id := range w.Products {
				want := 0.0
				for _, c := range w.Cities[cid].Corners {
					if c.Owner == game.OwnerPlayer && c.Runner != 0 {
						share := c.Demand
						if taste, ok := cfg.City.Corner(c.ID).Taste[id]; ok {
							share *= taste
						}
						share *= 1 - c.Squeeze // less what the rival undercuts away
						want += share * w.Product(cid, id).Demand
					}
				}
				if got := w.Demand(cid, id); math.Abs(got-want) > 1e-9 {
					t.Fatalf("day %d %s %s: served demand %.2f, corners add up to %.2f", w.Day, cid, id, got, want)
				}
				if cap := set.Market.Capacity(w, cid, id, events.DialNormal); cap != int(math.Round(want*set.Market.Fill(w, events.DialNormal))) {
					t.Fatalf("day %d %s %s: capacity %d for demand %.2f", w.Day, cid, id, cap, want)
				}
				checked++
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.World.Worked() < 3 || checked == 0 {
		t.Fatalf("the run never took ground: %d corners worked, %d checks", res.World.Worked(), checked)
	}
	// And what actually sold never exceeded it.
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok && ps.Sold > ps.Wanted {
			t.Fatalf("day %d: sold %d of %d wanted", ps.Day, ps.Sold, ps.Wanted)
		}
	}
}

// Losing every runner loses every corner they held within drift_days: a
// corner is only yours while somebody works it. The rival is kept
// defensive so the corners are lost to the street, not to it.
func TestLosingRunnersLosesCorners(t *testing.T) {
	cfg := content.MustLoad()
	drift := cfg.City.Territory.DriftDays
	crewed := Crewed(cfg, 40)
	fired := 0
	var heldBefore int
	start := sim.NewWorld(cfg, 5)
	start.Rival.Personality = "defensive"
	res, err := RunFrom(cfg, start, 80, func(w *game.World) {
		switch {
		case w.Day < 40:
			crewed(w)
		case w.Day == 40:
			heldBefore = w.Held()
			for _, m := range append([]game.CrewMember(nil), w.Crew.Members...) {
				if _, err := w.Fire(m.ID); err == nil {
					fired++
				}
			}
			w.Recall(game.You) // and step off your own corner
		case w.Day > 40+drift:
			if w.Held() != 0 {
				t.Fatalf("day %d: still holding %d corners %d days after losing the crew", w.Day, w.Held(), w.Day-40)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if fired < 3 || heldBefore < 4 {
		t.Fatalf("fired %d with %d corners held; the setup never built an operation", fired, heldBefore)
	}
	lost := 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.CornerLost:
			if ev.Day > 40 {
				lost++
			}
		case events.CornerTaken:
			if ev.Day > 40 && ev.From == game.OwnerPlayer {
				lost++ // an idle corner the rival walked onto
			}
		}
	}
	if lost != heldBefore {
		t.Fatalf("%d CornerLost events for %d corners", lost, heldBefore)
	}
	if res.World.Worked() != 0 || res.World.Demand(res.World.Home().ID, res.World.Products[0]) != 0 {
		t.Fatalf("with nobody on a corner: %d worked, demand %.1f", res.World.Worked(), res.World.Demand(res.World.Home().ID, res.World.Products[0]))
	}
}

// A player with no corners sells nothing, however much they queue.
func TestNoCornersSellsNothing(t *testing.T) {
	cfg := content.MustLoad()
	res, err := Run(cfg, 2, 5, func(w *game.World) {
		if w.Day == 0 {
			for _, c := range w.Corners() {
				if c.Owner == game.OwnerPlayer {
					if err := w.Abandon(c.ID); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		w.Stash(w.Home().ID)[w.Products[0]] = 100
		if err := w.PlaceSell(w.Home().ID, w.Products[0], 100, events.DialAggressive); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	sales := 0
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok {
			sales++
			if ps.Sold != 0 || ps.Revenue != 0 {
				t.Fatalf("day %d: sold %d for $%d with no corner", ps.Day, ps.Sold, ps.Revenue)
			}
		}
	}
	if sales == 0 {
		t.Fatal("no sale was even attempted")
	}
}

// Taking ground is the tier-3 multiplier: working three corners must
// out-earn the single-corner managed player by the end of tier 2, and the
// robberies that come with it must stay a cost, not a wipe-out. A guarded
// corner can go 70 days unrobbed, so that they happen at all is checked
// across the seeds.
func TestTerritoryPaysAndRobberiesCost(t *testing.T) {
	cfg := content.MustLoad()
	robberies := 0
	for seed := uint64(1); seed <= 5; seed++ {
		three, _ := Run(cfg, seed, TierDays[1], Territory(cfg, 40, 3))
		if three.Over != nil {
			t.Fatalf("seed %d: three-corner player ended on day %d: %s", seed, three.Days, three.Over.Cause)
		}
		managed, _ := Run(cfg, seed, TierDays[1], Managed(cfg, 50))
		if three.NetWorthAt(TierDays[1]) < 2*managed.NetWorthAt(TierDays[1]) {
			t.Fatalf("seed %d: three corners worth %d on day %d, one corner %d; ground should pay", seed, three.NetWorthAt(TierDays[1]), TierDays[1], managed.NetWorthAt(TierDays[1]))
		}
		for _, e := range three.Events {
			if _, ok := e.(events.CornerRobbed); ok {
				robberies++
			}
		}
		if lost := three.World.Stats.Robbed; lost > three.World.Stats.TotalRevenue/5 {
			t.Fatalf("seed %d: robbed of %d out of %d revenue", seed, lost, three.World.Stats.TotalRevenue)
		}
	}
	if robberies < 5 {
		t.Fatalf("%d robberies over five %d-day runs on three corners; they should be a fact of life", robberies, TierDays[1])
	}
}
