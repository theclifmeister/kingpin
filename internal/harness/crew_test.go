package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// hireAll signs everyone in the pool on day 0 and keeps the crew paid at p.
// It never trades, so heat stays at zero and nothing dangerous happens.
func hireAll(cfg *content.Config, p events.Pay) Policy {
	return func(w *game.World) {
		w.SetPay(p)
		if w.Day == 0 {
			for _, c := range append([]game.CrewMember(nil), w.Crew.Candidates...) {
				if _, err := w.Hire(c.ID, cfg.Crew.Crew.MaxCrew); err != nil {
					panic(err)
				}
			}
		}
	}
}

// hireAffordable signs everyone in the pool the player can pay for and
// still have the starting stake left to trade with.
func hireAffordable(cfg *content.Config, w *game.World) {
	for _, c := range append([]game.CrewMember(nil), w.Crew.Candidates...) {
		if w.Player.DirtyCash >= c.Fee+cfg.Market.Market.StartCash {
			_, _ = w.Hire(c.ID, cfg.Crew.Crew.MaxCrew)
		}
	}
}

// Loyalty after 100 quiet days must be monotone in pay for every member:
// generous >= fair >= stingy. Someone who quit counts as zero.
func TestLoyaltyMonotoneInPay(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Heat.Heat.DirtyCashHeat = 0 // a rich, idle player is not in danger
	cfg.Market.Market.StartCash = 1_000_000
	dials := []events.Pay{events.PayStingy, events.PayFair, events.PayGenerous}
	for seed := uint64(1); seed <= 10; seed++ {
		loyalty := make([]map[int]float64, len(dials))
		for i, p := range dials {
			res, err := Run(cfg, seed, 100, hireAll(cfg, p))
			if err != nil {
				t.Fatal(err)
			}
			if res.Over != nil {
				t.Fatalf("seed %d %s: run ended: %s", seed, p, res.Over.Cause)
			}
			loyalty[i] = map[int]float64{}
			for _, m := range res.World.Crew.Members {
				loyalty[i][m.ID] = m.Loyalty
			}
			for _, e := range res.Events {
				if _, ok := e.(events.Enforcement); ok {
					t.Fatalf("seed %d %s: enforcement fired in a no-danger run", seed, p)
				}
			}
		}
		for id := range loyalty[2] {
			s, f, g := loyalty[0][id], loyalty[1][id], loyalty[2][id]
			if !(g >= f && f >= s) {
				t.Fatalf("seed %d member %d: stingy %.1f fair %.1f generous %.1f is not monotone", seed, id, s, f, g)
			}
		}
	}
}

// A member never skims on a day they woke up at or above the threshold.
// Stingy pay with a full crew is where skimming happens, so the same runs
// also prove the mechanic fires at all, and that people at the bottom
// leave, by walking or by going over to the rival.
func TestSkimOnlyBelowThreshold(t *testing.T) {
	cfg := content.MustLoad()
	thr := cfg.Crew.Crew.SkimThreshold
	trade := Trader(cfg, events.DialQuiet)
	skims, quits := 0, 0
	for seed := uint64(1); seed <= 10; seed++ {
		minLoyalty := 100.0
		var days []float64 // morning minimum loyalty, indexed by day-1
		policy := func(w *game.World) {
			w.SetPay(events.PayStingy)
			hireAffordable(cfg, w)
			trade(w)
			minLoyalty = 100
			for _, m := range w.Crew.Members {
				minLoyalty = min(minLoyalty, m.Loyalty)
			}
			days = append(days, minLoyalty)
		}
		res, err := Run(cfg, seed, Horizon, policy)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.CrewSkimmed:
				skims++
				if ev.Amount <= 0 || ev.Skimmers <= 0 {
					t.Fatalf("seed %d day %d: empty skim event %+v", seed, ev.Day, ev)
				}
				if morning := days[ev.Day-1]; morning >= thr {
					t.Fatalf("seed %d day %d: skim with lowest loyalty %.1f >= threshold %.0f", seed, ev.Day, morning, thr)
				}
			case events.CrewQuit, events.CrewDefected:
				quits++
			}
		}
	}
	if skims == 0 || quits == 0 {
		t.Fatalf("stingy pay produced %d skims and %d quits or defections over 10 seeds; the mechanic is dead", skims, quits)
	}
}

// Pinning loyalty above the threshold must silence skimming entirely, even
// with a full crew on stingy pay and money on the table.
func TestNoSkimWhenLoyal(t *testing.T) {
	cfg := content.MustLoad()
	trade := Trader(cfg, events.DialQuiet)
	for seed := uint64(1); seed <= 5; seed++ {
		res, err := Run(cfg, seed, Horizon, func(w *game.World) {
			w.SetPay(events.PayStingy)
			hireAffordable(cfg, w)
			for i := range w.Crew.Members {
				w.Crew.Members[i].Loyalty = cfg.Crew.Crew.SkimThreshold
			}
			trade(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.CrewSkimmed); ok {
				t.Fatalf("seed %d day %d: skim from a crew pinned at the threshold", seed, ev.Day)
			}
		}
	}
}

// Runners are how you move more than you can carry and hold more than one
// corner: a player who builds a crew must out-earn the same player without
// one, and building it must not be what ends the run. The check stops at
// the tier-3 checkpoint because past it the crewed player sits on a
// dirty-cash pile that is its own heat source until #27 lands; the game
// itself has no day cap.
func TestCrewedBeatsManaged(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 10; seed++ {
		crewed, _ := Run(cfg, seed, Horizon, Crewed(cfg, 40))
		if crewed.Over != nil && crewed.Days <= TierDays[2] {
			t.Fatalf("seed %d: crewed trader ended on day %d: %s", seed, crewed.Days, crewed.Over.Cause)
		}
		if len(crewed.World.Crew.Members) == 0 {
			t.Fatalf("seed %d: crewed trader ended with no crew", seed)
		}
		managed, _ := Run(cfg, seed, Horizon, Managed(cfg, 50))
		if crewed.PeakCash <= managed.PeakCash {
			t.Fatalf("seed %d: crewed peaked at %d, managed at %d; runners should pay", seed, crewed.PeakCash, managed.PeakCash)
		}
	}
}

// The crew sim draws from the tick RNG; a crewed run must replay exactly.
func TestCrewedIsDeterministic(t *testing.T) {
	cfg := content.MustLoad()
	a, _ := Run(cfg, 5, 150, Crewed(cfg, 40))
	b, _ := Run(cfg, 5, 150, Crewed(cfg, 40))
	if a.PeakCash != b.PeakCash || len(a.Events) != len(b.Events) {
		t.Fatalf("crewed runs diverged: %d/%d events, peak %d/%d", len(a.Events), len(b.Events), a.PeakCash, b.PeakCash)
	}
	if len(a.World.Crew.Members) != len(b.World.Crew.Members) {
		t.Fatalf("rosters differ: %d vs %d", len(a.World.Crew.Members), len(b.World.Crew.Members))
	}
	for i := range a.World.Crew.Members {
		if a.World.Crew.Members[i] != b.World.Crew.Members[i] {
			t.Fatalf("member %d differs: %+v vs %+v", i, a.World.Crew.Members[i], b.World.Crew.Members[i])
		}
	}
}

// The crew invariants hold with the whole Crew branch owned (#118):
// loyalty stays monotone in pay, nobody skims on a day they woke up over
// the line, and the roster fills the room the tree bought and not a seat
// more (Hire takes the cap from crew.Sim.MaxCrew, which folds
// crew_slots).
func TestCrewInvariantsUnderTheBranch(t *testing.T) {
	cfg := content.MustLoad()
	crewSim := crew.New(cfg)
	branch := func(w *game.World) {
		for _, n := range cfg.Upgrades.Branch("crew") {
			grant(w, n.ID)
		}
	}
	// Room for four more, and not five: hire everyone the pool offers,
	// day after day, until the door shuts.
	w := sim.NewWorld(cfg, 1)
	branch(w)
	w.Player.DirtyCash = 1_000_000
	if got, want := crewSim.MaxCrew(w), cfg.Crew.Crew.MaxCrew+4; got != want {
		t.Fatalf("max crew with the branch is %d, want %d", got, want)
	}
	res, _ := RunFrom(cfg, w, 30, func(w *game.World) {
		for _, c := range append([]game.CrewMember(nil), w.Crew.Candidates...) {
			if _, err := w.Hire(c.ID, crewSim.MaxCrew(w)); err != nil && err != game.ErrCrewFull {
				t.Fatalf("day %d: hire: %v", w.Day, err)
			}
		}
		if len(w.Crew.Members) > crewSim.MaxCrew(w) {
			t.Fatalf("day %d: %d on the payroll, room for %d", w.Day, len(w.Crew.Members), crewSim.MaxCrew(w))
		}
	})
	if len(res.World.Crew.Members) != crewSim.MaxCrew(res.World) {
		t.Fatalf("after 30 days of hiring the payroll is %d of %d", len(res.World.Crew.Members), crewSim.MaxCrew(res.World))
	}

	// Monotone in pay, a quiet rich player.
	quiet := *cfg
	quiet.Heat.Heat.DirtyCashHeat = 0
	quiet.Market.Market.StartCash = 1_000_000
	dials := []events.Pay{events.PayStingy, events.PayFair, events.PayGenerous}
	for seed := uint64(1); seed <= 5; seed++ {
		loyalty := make([]map[int]float64, len(dials))
		for i, p := range dials {
			w := sim.NewWorld(&quiet, seed)
			branch(w)
			res, err := RunFrom(&quiet, w, 100, hireAll(&quiet, p))
			if err != nil || res.Over != nil {
				t.Fatalf("seed %d %s: %v %v", seed, p, err, res.Over)
			}
			loyalty[i] = map[int]float64{}
			for _, m := range res.World.Crew.Members {
				loyalty[i][m.ID] = m.Loyalty
			}
		}
		for id := range loyalty[2] {
			s, f, g := loyalty[0][id], loyalty[1][id], loyalty[2][id]
			if !(g >= f && f >= s) {
				t.Fatalf("seed %d member %d: stingy %.1f fair %.1f generous %.1f is not monotone", seed, id, s, f, g)
			}
		}
	}

	// No skim over the line, stingy pay and a full crew trading.
	thr := cfg.Crew.Crew.SkimThreshold
	trade := Trader(cfg, events.DialQuiet)
	for seed := uint64(1); seed <= 5; seed++ {
		var days []float64
		w := sim.NewWorld(cfg, seed)
		branch(w)
		res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
			w.SetPay(events.PayStingy)
			hireAffordable(cfg, w)
			trade(w)
			lowest := 100.0
			for _, m := range w.Crew.Members {
				lowest = min(lowest, m.Loyalty)
			}
			days = append(days, lowest)
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.CrewSkimmed); ok && days[ev.Day-1] >= thr {
				t.Fatalf("seed %d day %d: skim with lowest loyalty %.1f >= threshold %.0f", seed, ev.Day, days[ev.Day-1], thr)
			}
		}
	}
}
