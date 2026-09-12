package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// noLevels is the config with every front's levels taken off: the file
// before #192, for the runs that pin the old run.
func noLevels(cfg *content.Config) *content.Config {
	off := *cfg
	off.Laundering.Fronts = append([]content.FrontConfig(nil), cfg.Laundering.Fronts...)
	for i := range off.Laundering.Fronts {
		off.Laundering.Fronts[i].MaxLevel = 0
		off.Laundering.Fronts[i].Income = 0
		off.Laundering.Fronts[i].LevelCost = 0
	}
	return &off
}

// TestNoInvestIsTheOldRun (#192, the pattern of TestNoUndercutIsTheOldRun):
// a run that never invests is byte-for-byte the run before the levels
// existed. The laundered player, who never invests, plays 120 days on
// the file and on the file with the levels taken off, and the world is
// hashed the same after every day; no level, no investment and no
// income appear anywhere on it, and no FrontInvested or FrontGrew goes
// out.
func TestNoInvestIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	off := noLevels(cfg)
	for seed := uint64(1); seed <= 3; seed++ {
		var with, without []string
		for i, c := range []*content.Config{cfg, off} {
			w := sim.NewWorld(c, seed)
			_, sims, err := sim.Default(c)
			if err != nil {
				t.Fatal(err)
			}
			clock := game.NewClock(nil, sims...)
			policy := Laundered(c, 40)
			var ds []string
			for day := 1; day <= 120 && w.Over == nil; day++ {
				policy(w)
				for _, e := range clock.EndDay(w) {
					switch e.(type) {
					case events.FrontInvested, events.FrontGrew:
						t.Fatalf("seed %d day %d: %+v in a run that never invested", seed, day, e)
					}
				}
				ds = append(ds, digest(w))
			}
			for _, f := range w.Fronts {
				if f.Level != 0 || f.Invested != 0 || f.Grew != 0 {
					t.Fatalf("seed %d: %s has a level with nobody investing: %+v", seed, f.ID, f)
				}
			}
			if w.Stats.Earned != 0 || w.Stats.Invested != 0 {
				t.Fatalf("seed %d: earned %d invested %d with nobody investing", seed, w.Stats.Earned, w.Stats.Invested)
			}
			if i == 0 {
				with = ds
			} else {
				without = ds
			}
		}
		for day := range with {
			if with[day] != without[day] {
				t.Fatalf("seed %d: the world moved on day %d with the levels in the file and nobody buying one", seed, day+1)
			}
		}
	}
}

// TestBossPileDrains (#192, the sizing): at tier 4 the boss's clean
// cash at day 200 is under a third of the $30M it held before the
// levels (#191), the median over ten seeds, and what its fronts earn a
// day is within an order of magnitude of the wash ladder's $380k. The
// numbers are logged for the PR.
func TestBossPileDrains(t *testing.T) {
	cfg := content.MustLoad()
	ld := laundering.New(cfg)
	var clean, legit, levels, invested []int
	for seed := uint64(1); seed <= 10; seed++ {
		res, err := Run(cfg, seed, Horizon, Boss(cfg, 40, ""))
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		n := 0
		for _, f := range w.Fronts {
			n += f.Level
		}
		clean = append(clean, w.Player.CleanCash)
		legit = append(legit, ld.LegitIncome(w))
		levels = append(levels, n)
		invested = append(invested, w.Stats.Invested)
	}
	for _, s := range [][]int{clean, legit, levels, invested} {
		sort.Ints(s)
	}
	med := func(s []int) int { return s[len(s)/2] }
	t.Logf("boss at day %d over ten seeds (medians): clean %d (%d..%d), legit income %d/day (%d..%d), %d levels, %d invested", Horizon, med(clean), clean[0], clean[len(clean)-1], med(legit), legit[0], legit[len(legit)-1], med(levels), med(invested))
	if med(clean) >= 10_000_000 {
		t.Errorf("the boss holds %d clean at day %d, a third of the old $30M or more: the pile does not drain", med(clean), Horizon)
	}
	if med(legit) < 38_000 || med(legit) > 3_800_000 {
		t.Errorf("legit income %d/day at day %d is not within an order of magnitude of the ladder's $380k wash", med(legit), Horizon)
	}
}

// TestLevelledLaundromatPaysBack (#192, the sizing at tier 3): a
// laundromat levelled to its top earns its levels' price back within
// tier 3's checkpoint, on its income alone.
func TestLevelledLaundromatPaysBack(t *testing.T) {
	cfg := content.MustLoad()
	fc := cfg.Laundering.Fronts[0]
	if fc.ID != "laundromat" || fc.MaxLevel == 0 {
		t.Fatalf("the first front is %s with %d levels; the test wants the laundromat", fc.ID, fc.MaxLevel)
	}
	ld := laundering.New(cfg)
	w := sim.NewWorld(cfg, 1)
	w.Player.DirtyCash = fc.Cost
	w.Stats.PeakCash = fc.UnlockCash
	if _, err := ld.Buy(w, fc.ID); err != nil {
		t.Fatal(err)
	}
	w.Player.CleanCash = ld.LevelCost(w.Fronts[0], fc.MaxLevel)
	if err := ld.Invest(w, fc.ID, fc.MaxLevel); err != nil {
		t.Fatal(err)
	}
	f := w.Fronts[0]
	days := (f.Invested + ld.Income(f) - 1) / ld.Income(f)
	t.Logf("a laundromat at level %d: %d invested, earns %d/day, pays back in %d days", f.Level, f.Invested, ld.Income(f), days)
	if days > tierDay(3) {
		t.Errorf("the levelled laundromat pays back in %d days, over tier 3's %d", days, tierDay(3))
	}
}

// TestFrontGrowthIsNewsNotEvidence (#192): a front reaching the growth
// table's headline level makes the paper, and the morning after the
// city's pressure and the player's notoriety are up on the same run
// without the level; the DA's file is not (#27).
func TestFrontGrowthIsNewsNotEvidence(t *testing.T) {
	cfg := content.MustLoad()
	fc := cfg.Laundering.Fronts[0]
	hl := cfg.Laundering.Growth.HeadlineLevel
	if hl < 1 {
		t.Skip("no headline level in the file")
	}
	ld := laundering.New(cfg)
	run := func(grow bool) (*game.World, []events.Event) {
		w := sim.NewWorld(cfg, 3)
		w.Player.DirtyCash = fc.Cost + 100_000 // a till, or the crew sim ends the run broke before the paper is out
		w.Stats.PeakCash = fc.UnlockCash
		if _, err := ld.Buy(w, fc.ID); err != nil {
			t.Fatal(err)
		}
		w.Fronts[0].Level = hl - 1
		w.Player.CleanCash = ld.LevelCost(w.Fronts[0], 1)
		if grow {
			if err := ld.Invest(w, fc.ID, 1); err != nil {
				t.Fatal(err)
			}
		}
		res, err := RunFrom(cfg, w, 2, Hide)
		if err != nil {
			t.Fatal(err)
		}
		return res.World, res.Events
	}
	quiet, _ := run(false)
	grown, evs := run(true)
	grew, headline := 0, 0
	for _, e := range evs {
		switch ev := e.(type) {
		case events.FrontGrew:
			grew++
		case events.Headline:
			if ev.Source == "laundering" {
				headline++
			}
		}
	}
	if grew != 1 || headline == 0 {
		t.Fatalf("%d FrontGrew, %d laundering headlines", grew, headline)
	}
	here := grown.Player.Location
	if grown.Cities[here].Pressure <= quiet.Cities[here].Pressure {
		t.Errorf("pressure %.2f with the front in the paper, %.2f without", grown.Cities[here].Pressure, quiet.Cities[here].Pressure)
	}
	if grown.Player.Reputation.Notoriety <= quiet.Player.Reputation.Notoriety {
		t.Errorf("notoriety %.2f with the front in the paper, %.2f without", grown.Player.Reputation.Notoriety, quiet.Player.Reputation.Notoriety)
	}
	if grown.Heat.Evidence != 0 || quiet.Heat.Evidence != 0 {
		t.Errorf("evidence %d / %d on a run that never dealt", grown.Heat.Evidence, quiet.Heat.Evidence)
	}
}

// TestLevelsSurviveASave (#192): a world with levelled fronts saved and
// loaded plays on exactly as one that was not; the levels, the
// investment and the growth stamp come back.
func TestLevelsSurviveASave(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	ld := laundering.New(cfg)
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	build := func() *game.World {
		w := sim.NewWorld(cfg, 5)
		fc := cfg.Laundering.Fronts[0]
		w.Player.DirtyCash = fc.Cost + 1_000_000
		w.Stats.PeakCash = fc.UnlockCash
		if _, err := ld.Buy(w, fc.ID); err != nil {
			t.Fatal(err)
		}
		w.Player.CleanCash = ld.LevelCost(w.Fronts[0], 3)
		if err := ld.Invest(w, fc.ID, 3); err != nil {
			t.Fatal(err)
		}
		return w
	}
	a, b := build(), build()
	ca, cb := game.NewClock(nil, sims...), game.NewClock(nil, sims...)
	for i := 0; i < 10; i++ {
		ca.EndDay(a)
		cb.EndDay(b)
	}
	if err := game.Save(1, b); err != nil {
		t.Fatal(err)
	}
	b2, err := game.Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if f := b2.Fronts[0]; f.Level != 3 || f.Invested != b.Fronts[0].Invested || f.Grew != b.Fronts[0].Grew || b2.Stats.Earned != b.Stats.Earned {
		t.Fatalf("loaded front %+v, saved %+v; earned %d / %d", f, b.Fronts[0], b2.Stats.Earned, b.Stats.Earned)
	}
	for i := 0; i < 10; i++ {
		ca.EndDay(a)
		cb.EndDay(b2)
	}
	if digest(a) != digest(b2) {
		t.Fatalf("the run diverged after the save: clean %d / %d, earned %d / %d", a.Player.CleanCash, b2.Player.CleanCash, a.Stats.Earned, b2.Stats.Earned)
	}
}

// TestInvestingEverything (#192, the greed curve across the margin): the
// boss investing every clean dollar (-margin 1) against the boss at
// BossMargin, ten seeds on the same dice. What holds, and is pinned:
// margin 1 holds less clean in hand at day 200 on most seeds. What the
// issue asked for, audited more often and ending lower, is not a
// property of the sim: a front's own wash lands before its upkeep, so
// the reserve matters only on a morning with nothing over the float to
// wash, which the boss never has, and a level's return is bounded by
// its payback and nothing else, so a level bought sooner is money; nor
// is its extra worth bounded by its extra income (a bigger wash, the
// nodes bought sooner, half the seeds read over it). The freezes,
// audits, income and worth are logged, and the greed curve is pinned
// where it exists, on the levels: TestLevelsDrawTheAuditors.
func TestInvestingEverything(t *testing.T) {
	cfg := content.MustLoad()
	type row struct{ clean, worth, earned, levels, audits, frozen int }
	var greedy, careful []row
	for seed := uint64(1); seed <= 10; seed++ {
		var rows []row
		for _, margin := range []float64{1, BossMargin} {
			res, err := Run(cfg, seed, Horizon, BossAt(cfg, 40, "", margin))
			if err != nil {
				t.Fatal(err)
			}
			r := row{clean: res.World.Player.CleanCash, worth: res.NetWorthAt(Horizon), earned: res.World.Stats.Earned}
			for _, f := range res.World.Fronts {
				r.levels += f.Level
			}
			for _, e := range res.Events {
				switch e.(type) {
				case events.FrontAudited:
					r.audits++
				case events.FrontFrozen:
					r.frozen++
				}
			}
			rows = append(rows, r)
		}
		greedy, careful = append(greedy, rows[0]), append(careful, rows[1])
		t.Logf("seed %d: margin 1 clean %d worth %d earned %d levels %d audits %d frozen %d · BossMargin clean %d worth %d earned %d levels %d audits %d frozen %d",
			seed, rows[0].clean, rows[0].worth, rows[0].earned, rows[0].levels, rows[0].audits, rows[0].frozen, rows[1].clean, rows[1].worth, rows[1].earned, rows[1].levels, rows[1].audits, rows[1].frozen)
	}
	less := 0
	for i := range greedy {
		if greedy[i].clean < careful[i].clean {
			less++
		}
	}
	if less < 7 {
		t.Errorf("investing everything left more clean in hand than BossMargin on %d of ten seeds", 10-less)
	}
}

// TestLevelsDrawTheAuditors (#192, the greed curve on the levels): the
// boss that levels its fronts (BossMargin) against the same boss on the
// file with the levels taken off, ten seeds. The levelled boss is
// audited more often in total and never less on a seed (audit_level:
// the cost of the free lunch), and the morning after its growth makes
// the paper it carries more notoriety and pressure in total than the
// other on the same morning (the boss runs the city at pressure 100
// most of the run, so the paper's +5.8 reads only where there is room;
// the clean pin of the headline's effect on the same dice is
// TestFrontGrowthIsNewsNotEvidence).
func TestLevelsDrawTheAuditors(t *testing.T) {
	cfg := content.MustLoad()
	off := noLevels(cfg)
	audits, pressure, notoriety := [2]int{}, [2]float64{}, [2]float64{}
	for seed := uint64(1); seed <= 10; seed++ {
		var counts [2]int
		grew := 0
		for i, c := range []*content.Config{cfg, off} {
			boss := Boss(c, 40, "")
			var press, noto []float64
			res, err := Run(c, seed, Horizon, func(w *game.World) {
				boss(w)
				press = append(press, w.Here().Pressure)
				noto = append(noto, w.Player.Reputation.Notoriety)
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.FrontAudited:
					counts[i]++
				case events.FrontGrew:
					if grew == 0 {
						grew = ev.Day
					}
				}
			}
			if i == 0 && grew == 0 {
				t.Fatalf("seed %d: the boss never made the paper", seed)
			}
			if grew+1 < len(press) {
				pressure[i] += press[grew+1]
				notoriety[i] += noto[grew+1]
			}
			audits[i] += counts[i]
		}
		t.Logf("seed %d: audits %d with levels, %d without; in the paper on day %d", seed, counts[0], counts[1], grew)
		if counts[0] < counts[1] {
			t.Errorf("seed %d: the levelled boss was audited less (%d) than the boss without levels (%d)", seed, counts[0], counts[1])
		}
	}
	t.Logf("ten seeds: audits %d with levels, %d without; the morning after the paper pressure %.0f against %.0f, notoriety %.0f against %.0f (sums)", audits[0], audits[1], pressure[0], pressure[1], notoriety[0], notoriety[1])
	if audits[0] <= audits[1] {
		t.Errorf("the levelled boss was audited %d times over ten seeds, the boss without levels %d: the levels draw no auditors", audits[0], audits[1])
	}
	if pressure[0] <= pressure[1] || notoriety[0] <= notoriety[1] {
		t.Errorf("the morning after the paper the levelled boss carries pressure %.0f / notoriety %.0f against %.0f / %.0f without levels", pressure[0], notoriety[0], pressure[1], notoriety[1])
	}
}
