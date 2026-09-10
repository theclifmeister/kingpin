package harness

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// policies is every scripted player the harness knows, for the tests
// that must hold whoever is playing.
func policies(cfg *content.Config) map[string]Policy {
	return map[string]Policy{
		"idle":       Idle,
		"hide":       Hide,
		"quiet":      Trader(cfg, events.DialQuiet),
		"normal":     Trader(cfg, events.DialNormal),
		"aggressive": Trader(cfg, events.DialAggressive),
		"careful":    Careful(cfg, 35),
		"managed":    Managed(cfg, 50),
		"upgraded":   Upgraded(cfg, 50),
		"crewed":     Crewed(cfg, 40),
		"vigilant":   Vigilant(cfg, 40),
		"territory":  Territory(cfg, 40, 3),
		"war":        Warlike(cfg, 40, 3, events.ForcePush),
		"laundered":  Laundered(cfg, 40),
	}
}

// Every axis stays in 0..100 on every day of every policy, the three
// never add up to more than the street's attention, and no run ends, or
// passes a day, with all three above 70: you cannot max all three.
func TestReputationInvariants(t *testing.T) {
	cfg := content.MustLoad()
	total := cfg.Reputation.Reputation.Total
	for name, policy := range policies(cfg) {
		for seed := uint64(1); seed <= 3; seed++ {
			check := func(w *game.World) {
				r := w.Player.Reputation
				for _, a := range game.Axes {
					if v := *r.Axis(a); v < 0 || v > 100 {
						t.Fatalf("%s seed %d day %d: %s %.2f out of 0..100", name, seed, w.Day, a, v)
					}
				}
				if sum := r.Fear + r.Respect + r.Notoriety; sum > total+1e-6 {
					t.Fatalf("%s seed %d day %d: reputation adds up to %.2f, over %.0f", name, seed, w.Day, sum, total)
				}
				if r.Fear > 70 && r.Respect > 70 && r.Notoriety > 70 {
					t.Fatalf("%s seed %d day %d: feared, respected and notorious at once: %+v", name, seed, w.Day, r)
				}
			}
			res, err := Run(cfg, seed, Horizon, func(w *game.World) {
				check(w)
				policy(w)
			})
			if err != nil {
				t.Fatal(err)
			}
			check(res.World)
		}
	}
}

// The street's attention is a hard cap, not a tuning accident: gains that
// would push the three past it shrink them all to fit, so a player fed
// every source at once cannot end above 70 on all three.
func TestReputationCannotMaxAllThree(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 1)
	w.Player.DirtyCash = 10_000_000
	w.SetPay(events.PayGenerous)
	for i := 0; i < 3; i++ {
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 100 + i, Name: fmt.Sprintf("E%d", i), Role: "enforcer", Skill: 90, Loyalty: 90, Nerve: 90, Wage: 55})
	}
	w.Crew.NextID = 103
	w.Rival.Arrived, w.Rival.Cash = 1, 1_000_000
	res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
		w.Player.DirtyCash = 10_000_000 // whatever it costs
		w.Home().Heat, w.Heat.Evidence = 0, 0
		// A rival that is always there to be hit, a war that never
		// brings the crackdown that would rout it.
		w.Rival.Muscle, w.Rival.War, w.Rival.Routed = 5, 0, 0
		for _, id := range []string{"docks", "railyard"} {
			if c := w.Corner(id); c.Owner != game.OwnerRival {
				c.Owner, c.Runner, c.Enforcer, c.Since = game.OwnerRival, 0, 0, w.Day
			}
		}
		// Violence every night, a pay-off every day, the bag turned over.
		if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, func(c game.Corner) float64 { return c.Demand }); c != nil {
			_ = w.SendEnforcers(c.ID, events.ForceHit)
		}
		for _, m := range w.Crew.Members {
			_, _ = w.PayOff(m.ID, 1, 0)
			break
		}
		for _, id := range w.Products {
			w.Stash(w.Home().ID)[id] = 500
			_ = w.PlaceSell(w.Home().ID, id, 500, events.DialAggressive)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	r := res.World.Player.Reputation
	t.Logf("fed every source for %d days: fear %.0f respect %.0f notoriety %.0f", res.Days, r.Fear, r.Respect, r.Notoriety)
	if r.Fear > 70 && r.Respect > 70 && r.Notoriety > 70 {
		t.Fatalf("feared, respected and notorious at once: %+v", r)
	}
	if r.Fear+r.Respect+r.Notoriety < 150 {
		t.Fatalf("fed every source and only got to %+v; the cap is not what stopped it", r)
	}
}

// pinned plays a fixed day: reputation held where the test wants it,
// the same order on the same corner every night, so two runs differ in
// nothing but the name the player carries.
func pinned(rep game.Reputation, qty int) Policy {
	return func(w *game.World) {
		w.Player.Reputation = rep
		w.Stash(w.Home().ID)["weed"] = qty
		_ = w.PlaceSell(w.Home().ID, "weed", qty, events.DialNormal)
	}
}

// heatCarried is the sum of the day's closing heat over a run, and the
// units the player tried to move.
func heatCarried(r Result) (heat float64, wanted int) {
	for _, e := range r.Events {
		switch ev := e.(type) {
		case events.HeatChanged:
			heat += ev.To
		case events.PlayerSold:
			wanted += ev.Wanted
		}
	}
	return heat, wanted
}

// Fear costs heat: at the same sales volume a feared player carries more
// heat over the horizon than a respected one, because their heat never
// cools past the floor fear puts under it.
func TestFearPaysMoreHeatThanRespect(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		feared, err := Run(cfg, seed, Horizon, pinned(game.Reputation{Fear: 100}, 30))
		if err != nil {
			t.Fatal(err)
		}
		respected, _ := Run(cfg, seed, Horizon, pinned(game.Reputation{Respect: 100}, 30))
		fh, fw := heatCarried(feared)
		rh, rw := heatCarried(respected)
		if feared.Over != nil || respected.Over != nil {
			t.Fatalf("seed %d: a pinned run ended: %v %v", seed, feared.Over, respected.Over)
		}
		if fw != rw || fw == 0 {
			t.Fatalf("seed %d: volumes differ, %d vs %d units", seed, fw, rw)
		}
		t.Logf("seed %d: over %d days the feared player carried %.0f heat, the respected one %.0f", seed, Horizon, fh, rh)
		if fh <= rh {
			t.Fatalf("seed %d: feared carried %.0f heat, respected %.0f; fear should cost heat", seed, fh, rh)
		}
	}
}

// Each effect pulls the way the design says, read by the sim that owns
// the number: fear slows the rival's pushes, respect slows loyalty's
// decay and cheapens the supplier, notoriety cheapens hiring and heats
// the units you move yourself.
func TestReputationEffects(t *testing.T) {
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Fear: the expansionist pushes a guarded frontier less often.
	pushes := map[float64]int{}
	for _, fear := range []float64{0, 100} {
		for seed := uint64(1); seed <= 5; seed++ {
			w := sim.NewWorld(cfg, seed)
			w.Rival.Personality = "expansionist"
			res, _ := RunFrom(cfg, w, 100, func(w *game.World) {
				w.Player.Reputation.Fear = fear
				Territory(cfg, 40, 3)(w)
			})
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.RivalPushed:
					pushes[fear]++
				case events.CornerTaken:
					if ev.From == game.OwnerPlayer {
						pushes[fear]++
					}
				}
			}
		}
	}
	t.Logf("pushes over five 100-day runs: %d at fear 0, %d at fear 100", pushes[0], pushes[100])
	if pushes[100] >= pushes[0] || pushes[0] == 0 {
		t.Fatalf("fear 100 was pushed on %d times, fear 0 %d; fear should keep the rival off", pushes[100], pushes[0])
	}

	// Respect: the same crew on stingy pay keeps more loyalty, and the
	// supplier's quote is lower, and both are what the sims report.
	w := sim.NewWorld(cfg, 1)
	if set.Market.SupplierRatio(w) != cfg.Market.Market.SupplierRatio {
		t.Fatalf("a nobody pays %.3f of street, want %.3f", set.Market.SupplierRatio(w), cfg.Market.Market.SupplierRatio)
	}
	w.Player.Reputation.Respect = 100
	if ratio := set.Market.SupplierRatio(w); ratio >= cfg.Market.Market.SupplierRatio {
		t.Fatalf("respected and still paying %.3f of street", ratio)
	}
	loyalty := map[float64]float64{}
	for _, respect := range []float64{0, 100} {
		w := sim.NewWorld(cfg, 1)
		w.Player.DirtyCash = 1_000_000
		w.SetPay(events.PayStingy)
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Tank", Role: "enforcer", Skill: 50, Loyalty: 80, Nerve: 90, Greed: 50, Wage: 55})
		w.Crew.NextID = 900
		res, _ := RunFrom(cfg, w, 20, func(w *game.World) { w.Player.Reputation.Respect = respect })
		if m := res.World.Crew.Member(900); m != nil {
			loyalty[respect] = m.Loyalty
		}
	}
	t.Logf("loyalty after 20 stingy days: %.1f at respect 0, %.1f at respect 100", loyalty[0], loyalty[100])
	if loyalty[100] <= loyalty[0] || loyalty[100] >= 80 {
		t.Fatalf("respect 100 kept loyalty at %.1f, respect 0 at %.1f; respect should slow the decay, not stop it", loyalty[100], loyalty[0])
	}

	// Notoriety: a candidate asks less to sign, and the unit you move
	// yourself draws more heat while a runner's does not.
	w = sim.NewWorld(cfg, 1)
	plain := set.Crew.HireFee(w, 50)
	w.Player.Reputation.Notoriety = 100
	if fee := set.Crew.HireFee(w, 50); fee >= plain || fee <= 0 {
		t.Fatalf("notorious and a skill-50 candidate asks $%d, a nobody's asks $%d", fee, plain)
	}
	w = sim.NewWorld(cfg, 1)
	product := w.Products[0]
	quiet := set.Heat.SaleHeat(w, w.Home().ID, product, 30, events.DialNormal)
	w.Player.Reputation.Notoriety = 100
	if loud := set.Heat.SaleHeat(w, w.Home().ID, product, 30, events.DialNormal); loud <= quiet {
		t.Fatalf("notorious and moving 30 units yourself draws %.2f heat, a nobody %.2f", loud, quiet)
	}
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 901, Name: "Runner", Role: "runner", Skill: 60, Units: 120, Loyalty: 80, Nerve: 50, Wage: 50})
	w.Crew.NextID = 901
	start := w.PostOf(game.You)
	w.Recall(game.You)
	if err := w.Post(start.ID, 901); err != nil {
		t.Fatal(err)
	}
	byRunner := set.Heat.SaleHeat(w, w.Home().ID, product, 30, events.DialNormal)
	w.Player.Reputation.Notoriety = 0
	if nobody := set.Heat.SaleHeat(w, w.Home().ID, product, 30, events.DialNormal); byRunner != nobody {
		t.Fatalf("a runner's units draw %.2f heat under a notorious boss, %.2f under a nobody; notoriety is personal", byRunner, nobody)
	}
}

// The axes come from what happened: a won corner is fear and notoriety,
// generous pay and a pay-off are respect, volume is notoriety, and a
// crossed band is a headline.
func TestReputationSourcesAndHeadlines(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 1)
	w.Player.DirtyCash = 1_000_000
	w.SetPay(events.PayGenerous)
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Tank", Role: "enforcer", Skill: 90, Loyalty: 80, Nerve: 90, Wage: 55})
	w.Crew.NextID = 900
	w.Rival.Arrived, w.Rival.Muscle = 1, 1
	c := w.Corner("docks")
	c.Owner, c.Since = game.OwnerRival, 0
	res, err := RunFrom(cfg, w, 40, func(w *game.World) {
		w.Player.DirtyCash = 1_000_000
		w.Home().Heat = 0
		if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, func(c game.Corner) float64 { return c.Demand }); c != nil {
			_ = w.SendEnforcers(c.ID, events.ForceHit)
		}
		_, _ = w.PayOff(900, 1, 0)
		for _, id := range w.Products {
			w.Stash(w.Home().ID)[id] = 300
			_ = w.PlaceSell(w.Home().ID, id, 300, events.DialNormal)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	r := res.World.Player.Reputation
	if r.Fear <= 0 || r.Respect <= 0 || r.Notoriety <= 0 {
		t.Fatalf("40 days of violence, generosity and volume left %+v", r)
	}
	shifts, headlines := 0, 0
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.ReputationShifted:
			shifts++
			if ev.Axis != "fear" && ev.Axis != "respect" && ev.Axis != "notoriety" {
				t.Fatalf("shift on axis %q", ev.Axis)
			}
		case events.Headline:
			if ev.Source == "reputation" {
				headlines++
			}
		}
	}
	if shifts == 0 || headlines != shifts {
		t.Fatalf("%d band crossings made %d headlines", shifts, headlines)
	}
	t.Logf("40 days: %+v, %d bands crossed", r, shifts)

	// A nobody who does nothing stays one: the day-one claim of the
	// starting corner is the only headline about them.
	idle, _ := Run(cfg, 1, 60, Idle)
	if r := idle.World.Player.Reputation; r.Fear != 0 || r.Respect != 0 || r.Notoriety > 1 {
		t.Fatalf("an idle player earned %+v", r)
	}
}
