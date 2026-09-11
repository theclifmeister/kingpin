package harness

import (
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// medianDays plays seeds 1..n under a policy with the law fixed and
// returns the median run length.
func medianDays(t *testing.T, cfg *content.Config, n int, days int, chief, da string, policy func(*content.Config) Policy) int {
	t.Helper()
	var played []int
	for seed := uint64(1); seed <= uint64(n); seed++ {
		w := sim.NewWorld(cfg, seed)
		run := Appoint(cfg, w, chief, da)
		res, err := RunFrom(run, w, days, policy(run))
		if err != nil {
			t.Fatal(err)
		}
		played = append(played, res.Days)
	}
	sort.Ints(played)
	return played[len(played)/2]
}

// The chief and the DA are felt (#41): the always-aggressive trader is
// indicted sooner under a zealous chief (stings and raids come back
// faster) than under a lazy one, and the crewed player who runs hot is
// indicted sooner under a law-and-order DA (a thinner file will do, and
// the sting line is lower) than under a reformer.
func TestChiefAndDATable(t *testing.T) {
	cfg := content.MustLoad()
	aggressive := func(c *content.Config) Policy { return Trader(c, events.DialAggressive) }
	zealous := medianDays(t, cfg, 20, Horizon, "zealous", "moderate", aggressive)
	lazy := medianDays(t, cfg, 20, Horizon, "lazy", "moderate", aggressive)
	t.Logf("aggressive: indicted on day %d under a zealous chief, %d under a lazy one (medians)", zealous, lazy)
	if zealous >= lazy {
		t.Errorf("a zealous chief should indict the aggressive trader sooner than a lazy one: %d vs %d", zealous, lazy)
	}

	hot := func(c *content.Config) Policy { return Crewed(c, 50) }
	law := medianDays(t, cfg, 20, 2*Horizon, "corrupt", "law_and_order", hot)
	reform := medianDays(t, cfg, 20, 2*Horizon, "corrupt", "reform", hot)
	t.Logf("crewed at 50: lasts %d days under a law-and-order DA, %d under a reformer (medians)", law, reform)
	if law >= reform {
		t.Errorf("a law-and-order DA should indict the hot crewed player sooner than a reformer: %d vs %d", law, reform)
	}
}

// Pressure stays in 0..100 on every day of every policy, violence raises
// it (a hit war is louder than holding ground), and paying the town
// lowers it: the funded player ends quieter than the laundered one, and
// every dollar it gave was clean.
func TestPressureInvariantsAndSources(t *testing.T) {
	cfg := content.MustLoad()
	all := policies(cfg)
	all["funded"] = Funded(cfg, 40)
	all["hit"] = Warlike(cfg, 40, 3, events.ForceHit)
	for name, policy := range all {
		for seed := uint64(1); seed <= 3; seed++ {
			check := func(w *game.World) {
				for _, cid := range w.CityOrder {
					c := w.Cities[cid]
					if c.Pressure < 0 || c.Pressure > 100 || c.Goodwill < 0 || c.Goodwill > 100 {
						t.Fatalf("%s seed %d day %d: %s pressure %.2f goodwill %.2f out of 0..100", name, seed, w.Day, cid, c.Pressure, c.Goodwill)
					}
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

	at := 120
	mean := func(policy Policy) float64 {
		sum := 0.0
		for seed := uint64(1); seed <= 10; seed++ {
			res, _ := Run(cfg, seed, at, policy)
			sum += res.World.Home().Pressure
		}
		return sum / 10
	}
	war, ground := mean(Warlike(cfg, 40, 3, events.ForceHit)), mean(Territory(cfg, 40, 3))
	t.Logf("mean home pressure on day %d: hit war %.0f, territory %.0f", at, war, ground)
	if war <= ground {
		t.Errorf("a hit war should be louder than holding ground: %.1f vs %.1f", war, ground)
	}

	var funded, laundered []float64
	gave := 0
	for seed := uint64(1); seed <= 10; seed++ {
		f, _ := Run(cfg, seed, Horizon, Funded(cfg, 40))
		l, _ := Run(cfg, seed, Horizon, Laundered(cfg, 40))
		funded = append(funded, f.World.Here().Pressure)
		laundered = append(laundered, l.World.Here().Pressure)
		gave += f.World.Stats.Funded
		reported := 0
		for _, e := range f.Events {
			if ev, ok := e.(events.CityFunded); ok {
				reported += ev.Amount
				if ev.Goodwill <= 0 {
					t.Fatalf("seed %d day %d: $%d bought no goodwill", seed, ev.Day, ev.Amount)
				}
			}
		}
		if reported != f.World.Stats.Funded {
			t.Fatalf("seed %d: reported $%d funded, stats say $%d", seed, reported, f.World.Stats.Funded)
		}
		// Only clean cash pays: every dollar the funded player gave came
		// out of what its fronts washed (Fund refuses dirty cash outright;
		// TestFund pins that).
		if f.World.Stats.Funded > f.World.Stats.Laundered {
			t.Fatalf("seed %d: gave $%d but only washed $%d", seed, f.World.Stats.Funded, f.World.Stats.Laundered)
		}
	}
	sort.Float64s(funded)
	sort.Float64s(laundered)
	t.Logf("pressure at the horizon (medians): funded %.0f, laundered %.0f; $%d given over 10 runs", funded[5], laundered[5], gave)
	if gave == 0 {
		t.Fatal("the funded player never paid the town")
	}
	if funded[5] >= laundered[5] {
		t.Errorf("the funded player should end quieter than the laundered one: %.1f vs %.1f", funded[5], laundered[5])
	}
}

// Elections follow pressure: over 50 seeds, a vote held with the cities
// at 80 returns law-and-order more often than one held at 20, and every
// scheduled election fires on its day with a headline that names the
// winner.
func TestElectionsFollowPressure(t *testing.T) {
	cfg := content.MustLoad()
	count := func(pressure float64) int {
		n := 0
		for seed := uint64(1); seed <= 50; seed++ {
			w := sim.NewWorld(cfg, seed)
			for _, c := range w.Cities {
				c.Pressure = pressure
			}
			ev := Elect(cfg, w, 90)
			if ev.Name == "" || ev.Stance != w.Law.DA.Stance {
				t.Fatalf("seed %d: no election on the day: %+v %+v", seed, ev, w.Law.DA)
			}
			if ev.Stance == "law_and_order" {
				n++
			}
		}
		return n
	}
	loud, quiet := count(80), count(20)
	t.Logf("law-and-order won %d of 50 elections at pressure 80, %d of 50 at 20", loud, quiet)
	if loud <= quiet {
		t.Errorf("pressure should swing the vote to law-and-order: %d at 80 vs %d at 20", loud, quiet)
	}

	term := cfg.Law.Law.TermDays
	for seed := uint64(1); seed <= 5; seed++ {
		res, err := Run(cfg, seed, Horizon, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if res.Over != nil {
			continue
		}
		var days []int
		for _, e := range res.Events {
			if ev, ok := e.(events.DAElected); ok {
				days = append(days, ev.Day)
				named := false
				for _, h := range res.World.Journal {
					if h.Day == ev.Day && h.Source == "law" && strings.Contains(h.Text, ev.Name) {
						named = true
					}
				}
				if !named {
					t.Fatalf("seed %d day %d: no headline names DA %s", seed, ev.Day, ev.Name)
				}
			}
		}
		var want []int
		for d := term; d <= Horizon; d += term {
			want = append(want, d)
		}
		if len(days) != len(want) {
			t.Fatalf("seed %d: elections on days %v, want %v", seed, days, want)
		}
		for i := range want {
			if days[i] != want[i] {
				t.Fatalf("seed %d: elections on days %v, want %v", seed, days, want)
			}
		}
		if res.World.Stats.Elections != len(want) {
			t.Fatalf("seed %d: stats count %d elections, want %d", seed, res.World.Stats.Elections, len(want))
		}
	}
}

// #27 holds under every chief and every DA: the rich hider is stung and
// raided for the pile and never indicted, and no sting or raid on a day
// with no attempted sale adds a page.
func TestQuietDayRuleHoldsUnderEveryLaw(t *testing.T) {
	cfg := content.MustLoad()
	for _, chief := range content.ChiefPersonalities {
		for _, da := range content.DAStances {
			for seed := uint64(1); seed <= 2; seed++ {
				w := sim.NewWorld(cfg, seed)
				w.Player.DirtyCash = 5_000_000
				run := Appoint(cfg, w, chief, da)
				res, _ := RunFrom(run, w, 600, Hide)
				if res.Over != nil {
					t.Fatalf("%s/%s seed %d: rich hider ended on day %d: %s", chief, da, seed, res.Days, res.Over.Cause)
				}
				if res.World.Heat.Evidence != 0 {
					t.Fatalf("%s/%s seed %d: rich hider has %d evidence without ever selling", chief, da, seed, res.World.Heat.Evidence)
				}
				stings := 0
				for _, e := range res.Events {
					if ev, ok := e.(events.Enforcement); ok && (ev.Level == "sting" || ev.Level == "raid") {
						stings++
						if ev.Evidence != 0 {
							t.Fatalf("%s/%s seed %d day %d: %s on a quiet day added %d evidence", chief, da, seed, ev.Day, ev.Level, ev.Evidence)
						}
					}
				}
				if stings == 0 {
					t.Fatalf("%s/%s seed %d: the pile drew no stings or raids", chief, da, seed)
				}
			}
		}
	}
}

// The chief, the DA, pressure and goodwill survive a save, and a run
// that funds the town replays from its seed.
func TestLawSurvivesSave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	play := func(seed uint64) (Result, error) {
		w := sim.NewWorld(cfg, seed)
		w.Player.CleanCash = 50_000
		return RunFrom(cfg, w, 100, func(w *game.World) {
			Funded(cfg, 40)(w)
			if w.Day == 10 {
				_ = w.Fund(w.Home().ID, 5_000)
			}
		})
	}
	a, err := play(3)
	if err != nil {
		t.Fatal(err)
	}
	if a.World.Law.Chief.Name == "" || a.World.Law.DA.Name == "" || a.World.Home().Goodwill <= 0 || a.World.Home().Pressure <= 0 {
		t.Fatalf("nothing to save: %+v %+v", a.World.Law, a.World.Home())
	}
	if err := game.Save(1, a.World); err != nil {
		t.Fatal(err)
	}
	got, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if got.Law != a.World.Law {
		t.Fatalf("law after load %+v, want %+v", got.Law, a.World.Law)
	}
	for _, cid := range a.World.CityOrder {
		if got.Cities[cid].Pressure != a.World.Cities[cid].Pressure || got.Cities[cid].Goodwill != a.World.Cities[cid].Goodwill {
			t.Fatalf("%s after load: pressure %.2f goodwill %.2f, want %.2f %.2f", cid, got.Cities[cid].Pressure, got.Cities[cid].Goodwill, a.World.Cities[cid].Pressure, a.World.Cities[cid].Goodwill)
		}
	}
	if got.Stats.Funded != a.World.Stats.Funded || got.Stats.Elections != a.World.Stats.Elections {
		t.Fatalf("stats after load %+v", got.Stats)
	}
	b, _ := play(3)
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if x, y := a.Events[i], b.Events[i]; x.Kind() != y.Kind() {
			t.Fatalf("event %d differs: %+v vs %+v", i, x, y)
		}
	}
	if b.World.Law != a.World.Law || b.World.Home().Pressure != a.World.Home().Pressure {
		t.Fatalf("replay differs: %+v vs %+v", b.World.Law, a.World.Law)
	}
}
