package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// Over a laundered run neither cash pool ever goes negative, a day's wash
// never exceeds what the open fronts could do that morning, and a frozen
// front washes nothing.
func TestLaunderingInvariants(t *testing.T) {
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	laundered := Laundered(cfg, 40)
	washed := 0
	for seed := uint64(1); seed <= 5; seed++ {
		capacity := 0
		var frozen []string
		res, err := Run(cfg, seed, Horizon, func(w *game.World) {
			laundered(w)
			capacity = set.Laundering.Capacity(w)
			frozen = frozen[:0]
			for _, f := range w.Fronts {
				if f.Frozen(w.Day + 1) {
					frozen = append(frozen, f.ID)
				}
			}
			if w.Player.DirtyCash < 0 || w.Player.CleanCash < 0 {
				t.Fatalf("seed %d day %d: dirty %d clean %d", seed, w.Day, w.Player.DirtyCash, w.Player.CleanCash)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil {
			t.Fatalf("seed %d: laundered run ended on day %d: %s", seed, res.Days, res.Over.Cause)
		}
		for _, e := range res.Events {
			if cl, ok := e.(events.CashLaundered); ok {
				washed += cl.Amount
				if cl.Day == res.Days && cl.Amount > capacity {
					t.Fatalf("seed %d day %d: washed %d with capacity %d", seed, cl.Day, cl.Amount, capacity)
				}
			}
		}
		for _, f := range res.World.Fronts {
			for _, id := range frozen {
				if f.ID == id && f.WashedToday != 0 {
					t.Fatalf("seed %d: frozen front %s washed %d on the last day", seed, id, f.WashedToday)
				}
			}
		}
		if res.World.Player.CleanCash <= 0 || len(res.World.Fronts) == 0 {
			t.Fatalf("seed %d: laundered run ended with %d clean and %d fronts", seed, res.World.Player.CleanCash, len(res.World.Fronts))
		}
	}
	if washed == 0 {
		t.Fatal("nothing was washed in five laundered runs")
	}
}

// dialed plays Laundered but pins the launder dial.
func dialed(cfg *content.Config, d events.Launder) Policy {
	laundered := Laundered(cfg, 40)
	return func(w *game.World) {
		laundered(w)
		w.SetLaunderDial(d)
	}
}

// Greedy washes more per open day than careful, and is audited more often
// over the horizon: the dial has to be a real trade. Both are read over
// five seeds: a day's wash is what the till has over the float, so one
// seed's cash flow can put the two dials within a few percent of each
// other (seed 5 does), and a change elsewhere in the day can flip it.
func TestGreedyWashesMoreAndIsAuditedMore(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Heat.Heat.AuditEvidence = 0 // measure the wash, not how fast greedy is indicted
	audits := map[events.Launder]int{}
	perDay := map[events.Launder]float64{}
	for seed := uint64(1); seed <= 5; seed++ {
		for _, d := range []events.Launder{events.LaunderCareful, events.LaunderGreedy} {
			res, err := Run(cfg, seed, Horizon, dialed(cfg, d))
			if err != nil {
				t.Fatal(err)
			}
			total, days := 0, 0
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.CashLaundered:
					total += ev.Amount
					days++
				case events.FrontAudited:
					audits[d]++
				}
			}
			if days == 0 {
				t.Fatalf("seed %d %s: never washed", seed, d)
			}
			t.Logf("seed %d %s: washed %.0f a day over %d days", seed, d, float64(total)/float64(days), days)
			perDay[d] += float64(total) / float64(days)
		}
	}
	if perDay[events.LaunderGreedy] <= perDay[events.LaunderCareful] {
		t.Fatalf("over five runs greedy washed %.0f a day, careful %.0f", perDay[events.LaunderGreedy]/5, perDay[events.LaunderCareful]/5)
	}
	if audits[events.LaunderGreedy] <= audits[events.LaunderCareful] {
		t.Fatalf("greedy drew %d audits over five runs, careful %d", audits[events.LaunderGreedy], audits[events.LaunderCareful])
	}
}

// An audit is heat the morning after; only an audit of a front being run
// greedy is evidence (#27: the case is what you did, not what you have).
func TestAuditEvidenceOnlyWhenGreedy(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Heat.Heat.DirtyCashHeat = 0 // the pile is not what is measured here
	cfg.Laundering.Fronts[0].AuditRisk = 1
	cfg.Laundering.Dial.Careful.Risk = 1
	cfg.Laundering.Dial.Greedy.Risk = 1
	for _, d := range []events.Launder{events.LaunderCareful, events.LaunderNormal, events.LaunderGreedy} {
		w := sim.NewWorld(cfg, 1)
		w.Player.DirtyCash = 200_000
		w.Stats.PeakCash = w.Player.DirtyCash
		w.Fronts = []game.Front{{ID: cfg.Laundering.Fronts[0].ID, Name: "Front"}}
		w.SetLaunderDial(d)
		res, err := RunFrom(cfg, w, 2, Hide)
		if err != nil {
			t.Fatal(err)
		}
		var heat []events.HeatChanged
		audits := 0
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.HeatChanged:
				if ev.City == w.Home().ID { // one a day per city; the audit lands where you are
					heat = append(heat, ev)
				}
			case events.FrontAudited:
				audits++
			}
		}
		if audits != 1 || len(heat) != 2 {
			t.Fatalf("%s: %d audits, %d heat events", d, audits, len(heat))
		}
		// Day 1: audit fires after heat. Day 2: heat reads it.
		if heat[0].To-heat[0].From > 0 {
			t.Fatalf("%s: heat rose on the audit day itself: %+v", d, heat[0])
		}
		if rise := heat[1].To - heat[1].From; rise < cfg.Heat.Heat.AuditHeat/2 {
			t.Fatalf("%s: audit added %.1f heat the morning after, want about %.1f: %+v", d, rise, cfg.Heat.Heat.AuditHeat, heat[1])
		}
		want := 0
		if d == events.LaunderGreedy {
			want = cfg.Heat.Heat.AuditEvidence
		}
		if res.World.Heat.Evidence != want {
			t.Fatalf("%s: evidence %d, want %d", d, res.World.Heat.Evidence, want)
		}
	}
}

// Washing the pile is what lets a crewed operation keep going: laundered
// beats crewed on median net worth over the horizon, and is never indicted
// for sitting on what it earned. Medians, not seed by seed: whether the
// rival happens to crush a run is the bigger roll of the dice.
func TestLaunderedBeatsCrewed(t *testing.T) {
	cfg := content.MustLoad()
	var laundered, crewed []int
	for seed := uint64(1); seed <= 20; seed++ {
		l, _ := Run(cfg, seed, Horizon, Laundered(cfg, 40))
		if l.Over != nil {
			t.Fatalf("seed %d: laundered run ended on day %d: %s", seed, l.Days, l.Over.Cause)
		}
		c, _ := Run(cfg, seed, Horizon, Crewed(cfg, 40))
		laundered = append(laundered, l.NetWorthAt(Horizon))
		crewed = append(crewed, c.NetWorthAt(Horizon))
	}
	sort.Ints(laundered)
	sort.Ints(crewed)
	lm, cm := laundered[len(laundered)/2], crewed[len(crewed)/2]
	t.Logf("day %d median net worth: laundered %d, crewed %d", Horizon, lm, cm)
	if lm <= cm {
		t.Fatalf("laundered median net worth %d at day %d does not beat crewed %d", lm, Horizon, cm)
	}
}

// The laundering sim draws from the tick RNG; a laundered run must replay
// exactly, fronts and all.
func TestLaunderedIsDeterministic(t *testing.T) {
	cfg := content.MustLoad()
	a, _ := Run(cfg, 5, 150, Laundered(cfg, 40))
	b, _ := Run(cfg, 5, 150, Laundered(cfg, 40))
	if a.PeakCash != b.PeakCash || len(a.Events) != len(b.Events) || a.World.Player.CleanCash != b.World.Player.CleanCash {
		t.Fatalf("laundered runs diverged: %d/%d events, peak %d/%d, clean %d/%d", len(a.Events), len(b.Events), a.PeakCash, b.PeakCash, a.World.Player.CleanCash, b.World.Player.CleanCash)
	}
	if len(a.World.Fronts) != len(b.World.Fronts) || len(a.World.Fronts) == 0 {
		t.Fatalf("fronts differ: %d vs %d", len(a.World.Fronts), len(b.World.Fronts))
	}
	for i := range a.World.Fronts {
		if a.World.Fronts[i] != b.World.Fronts[i] {
			t.Fatalf("front %d differs: %+v vs %+v", i, a.World.Fronts[i], b.World.Fronts[i])
		}
	}
}

// The wash invariants hold with the whole Laundering branch owned
// (#118), and the thinner float is one number for the wash and the
// road: over a laundered run neither pool goes negative, a day's wash
// never exceeds the folded capacity, a frozen front washes nothing,
// every wash leaves the till at or over the folded float (half the
// file's) and the road's budget is what is over that same float.
func TestLaunderingInvariantsUnderTheBranch(t *testing.T) {
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	laundered := Laundered(cfg, 40)
	half := cfg.Laundering.Laundering.Float / 2
	washed := 0
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		for _, n := range cfg.Upgrades.Branch("laundering") {
			grant(w, n.ID)
		}
		if got := set.Laundering.Float(w); got != half || set.Logistics.Budget(w) != max(0, w.Player.DirtyCash-half) {
			t.Fatalf("seed %d: float %d, budget %d, want %d and %d", seed, got, set.Logistics.Budget(w), half, max(0, w.Player.DirtyCash-half))
		}
		capacity := 0
		var frozen []string
		res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
			laundered(w)
			capacity = set.Laundering.Capacity(w)
			frozen = frozen[:0]
			for _, f := range w.Fronts {
				if f.Frozen(w.Day + 1) {
					frozen = append(frozen, f.ID)
				}
			}
			if w.Player.DirtyCash < 0 || w.Player.CleanCash < 0 {
				t.Fatalf("seed %d day %d: dirty %d clean %d", seed, w.Day, w.Player.DirtyCash, w.Player.CleanCash)
			}
			if w.Player.DirtyCash > half && set.Logistics.Budget(w) != w.Player.DirtyCash-half {
				t.Fatalf("seed %d day %d: the road's budget is %d with %d dirty over a float of %d", seed, w.Day, set.Logistics.Budget(w), w.Player.DirtyCash, half)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil {
			t.Fatalf("seed %d: laundered run ended on day %d: %s", seed, res.Days, res.Over.Cause)
		}
		for _, e := range res.Events {
			if cl, ok := e.(events.CashLaundered); ok && cl.Amount > 0 {
				washed += cl.Amount
				if cl.Day == res.Days && cl.Amount > capacity {
					t.Fatalf("seed %d day %d: washed %d with capacity %d", seed, cl.Day, cl.Amount, capacity)
				}
			}
		}
		for _, f := range res.World.Fronts {
			for _, id := range frozen {
				if f.ID == id && f.WashedToday != 0 {
					t.Fatalf("seed %d: frozen front %s washed %d on the last day", seed, id, f.WashedToday)
				}
			}
		}
		if res.World.Player.DirtyCash < half && res.World.Stats.Laundered > 0 && res.World.Fronts[0].WashedToday > 0 {
			t.Fatalf("seed %d: the wash took the till to %d, under the float of %d", seed, res.World.Player.DirtyCash, half)
		}
	}
	if washed == 0 {
		t.Fatal("nothing was washed in five laundered runs")
	}
}
