package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The assets and the task force (#48). The task force is a rung the
// four-rung ladder's players never meet: it forms only with an asset
// owned or dirty cash over taskforce_cash, so every tier-1..4 row and
// the four difficulty tests play the ladder they always did. A run with
// the assets in their box is the run with them in the file, number for
// number. The cartel (boss with the assets) beats corrupt and
// distributor at the tier-5 checkpoint and stays within fifteen percent
// of the boss; the aggressive cartel meets the task force and loses an
// asset, or the run.

// NoAssets returns a copy of cfg with the assets boxed: no offer in the
// file and no cash line for the task force, so nothing can ever meet
// it. The routes an asset opens stay shut with nothing to open them.
func NoAssets(cfg *content.Config) *content.Config {
	c := *cfg
	c.Assets.Offers = nil
	c.Heat.Heat.TaskforceCash = 0
	return &c
}

// TestTaskForceNeverMeetsTierThree runs every money-curve row's policy
// to its checkpoint and the four difficulty tests' traders to the
// horizon and finds no task force formed and none fired: the rung is
// skipped for a player with no asset and a pile under the line, so
// nothing about those runs moves.
func TestTaskForceNeverMeetsTierThree(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	rows := []struct {
		name   string
		days   int
		policy Policy
	}{
		{"managed", TierDays[0], Managed(cfg, 50)},
		{"crewed", TierDays[1], Crewed(cfg, 40)},
		{"laundered", TierDays[2], Laundered(cfg, 40)},
		{"distributor", TierDays[3], Distributor(cfg, 40)},
		{"boss", TierDays[3], Boss(cfg, 40, "")},
		{"aggressive", Horizon, Trader(cfg, events.DialAggressive)},
		{"quiet", Horizon, Trader(cfg, events.DialQuiet)},
	}
	for _, row := range rows {
		for seed := uint64(1); seed <= 5; seed++ {
			res, err := Run(cfg, seed, row.days, row.policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.TaskForceFormed:
					t.Fatalf("%s seed %d day %d: a task force formed with no asset and %d dirty", row.name, seed, ev.Day, res.World.Player.DirtyCash)
				case events.Enforcement:
					if ev.Level == content.TaskForce {
						t.Fatalf("%s seed %d day %d: the task force fired", row.name, seed, ev.Day)
					}
				case events.AssetSeized, events.AssetBought:
					t.Fatalf("%s seed %d: %+v", row.name, seed, ev)
				}
			}
			if len(res.World.Assets) > 0 || res.World.Stats.TaskForces > 0 {
				t.Fatalf("%s seed %d: %d assets, %d task forces", row.name, seed, len(res.World.Assets), res.World.Stats.TaskForces)
			}
		}
	}
}

// TestNoAssetIsTheOldRun plays the boss to the tier-4 checkpoint on
// the file and with the assets boxed and finds the same run, day for
// day: every number the money curve reads (net worth, the two piles,
// the heat, the file) and the stats at the end. The boss crosses the
// first asset's line late in the run, so the file's run carries the
// door's announcement and the tier; nothing it plays moves.
func TestNoAssetIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	boxed := NoAssets(cfg)
	for _, seed := range []uint64{1, 2, 3} {
		type day struct {
			worth, dirty, clean, evidence int
			heat                          float64
		}
		play := func(c *content.Config) ([]day, game.Stats) {
			var days []day
			policy := Boss(c, 40, "")
			res, err := Run(c, seed, TierDays[3], func(w *game.World) {
				policy(w)
				days = append(days, day{w.NetWorth(), w.Player.DirtyCash, w.Player.CleanCash, w.Heat.Evidence, w.Home().Heat})
			})
			if err != nil {
				t.Fatal(err)
			}
			return days, res.World.Stats
		}
		file, fileStats := play(cfg)
		box, boxStats := play(boxed)
		if len(file) != len(box) {
			t.Fatalf("seed %d: %d days on the file, %d boxed", seed, len(file), len(box))
		}
		for i := range file {
			if file[i] != box[i] {
				t.Fatalf("seed %d day %d: the file plays %+v, the box %+v", seed, i, file[i], box[i])
			}
		}
		if fileStats != boxStats {
			t.Fatalf("seed %d: stats differ:\n%+v\n%+v", seed, fileStats, boxStats)
		}
	}
}

// TestCartelBeatsCorruptAndDistributor is the issue's ordering at the
// tier-5 checkpoint: the cartel's median net worth on day 300 over ten
// seeds is above corrupt's and distributor's, and it owns assets.
func TestCartelBeatsCorruptAndDistributor(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	day := TierDays[4]
	var cartel, corrupt, distributor []int
	assets := 0
	for seed := uint64(1); seed <= 10; seed++ {
		c, err := Run(cfg, seed, day, Cartel(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		assets += c.World.Stats.Assets
		k, _ := Run(cfg, seed, day, Corrupt(cfg, 40))
		d, _ := Run(cfg, seed, day, Distributor(cfg, 40))
		cartel = append(cartel, c.NetWorthAt(day))
		corrupt = append(corrupt, k.NetWorthAt(day))
		distributor = append(distributor, d.NetWorthAt(day))
	}
	sort.Ints(cartel)
	sort.Ints(corrupt)
	sort.Ints(distributor)
	t.Logf("day %d median net worth: cartel %d, corrupt %d, distributor %d; %.1f assets bought per run", day, cartel[5], corrupt[5], distributor[5], float64(assets)/10)
	if cartel[5] <= corrupt[5] || cartel[5] <= distributor[5] {
		t.Fatalf("the cartel does not beat corrupt and distributor: %d / %d / %d", cartel[5], corrupt[5], distributor[5])
	}
	if assets == 0 {
		t.Fatal("the cartel bought no asset in ten runs")
	}
}

// TestCartelDwarfsBoss pins what the export lanes (#391) make of the
// cartel at the tier-5 checkpoint. Before them the cartel was the boss
// that put clean cash into supply-side assets instead of levels, and it
// read a few percent under the boss (#48's ruling: a demand-bound
// operation gains nothing from cheaper supply). The lanes are demand the
// corners do not bound, bought off the book the cartel owns, so the
// cartel now reads several times the boss: on every seed where a lane
// shipped it beats the boss, and its median is at least five times the
// boss's (about nine on ten seeds).
func TestCartelDwarfsBoss(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	day := TierDays[4]
	var cartel, boss []int
	for seed := uint64(1); seed <= 10; seed++ {
		c, err := Run(cfg, seed, day, Cartel(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if c.Over != nil {
			t.Fatalf("seed %d: the cartel ended on day %d: %s", seed, c.Days, c.Over.Cause)
		}
		b, _ := Run(cfg, seed, day, Boss(cfg, 40, ""))
		cartel = append(cartel, c.NetWorthAt(day))
		boss = append(boss, b.NetWorthAt(day))
		st := c.World.Stats
		t.Logf("seed %d: cartel %d, boss %d (x%.1f), %d assets, %d loads abroad for %d", seed, c.NetWorthAt(day), b.NetWorthAt(day), float64(c.NetWorthAt(day))/float64(b.NetWorthAt(day)), len(c.World.Assets), st.ExportLoads, st.ExportCash)
		if st.ExportLoads > 0 && c.NetWorthAt(day) <= b.NetWorthAt(day) {
			t.Errorf("seed %d: the cartel shipped %d loads abroad and reads %d, under the boss's %d", seed, st.ExportLoads, c.NetWorthAt(day), b.NetWorthAt(day))
		}
	}
	sort.Ints(cartel)
	sort.Ints(boss)
	c, b := cartel[5], boss[5]
	t.Logf("day %d median net worth: cartel %d, boss %d (x%.1f)", day, c, b, float64(c)/float64(b))
	if c < 5*b {
		t.Fatalf("cartel median %d is under five times the boss's %d", c, b)
	}
}

// TestAggressiveCartelLosesItsAssets is the issue's "money at tier 5 is
// not safety": the reckless cartel plays managed until it owns an
// asset, then never lies low again and sells everything at the
// aggressive dial, and on most seeds a task force forms, is announced
// the day before, seizes exactly one asset a firing, and an asset is
// gone or the run is over before day 300. (A boss that merely stops
// lying low at this scale is not enough: its Security branch and its
// lawyers hold a normal-dial operation at heat 50 and the file at
// nothing, so the task force's line, 88 less the pressure's cut, takes
// the loud dial to reach; 8 of 10 seeds on main with #43's table. With
// a faction drawn to the hub (#341) the loud dial alone reached it on
// 2: the faction took the hub's corners and the volume with them, so
// the reckless cartel now goes to war as well (#379), 9 of 10.)
func TestAggressiveCartelLosesItsAssets(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	punished := 0
	for seed := uint64(1); seed <= 10; seed++ {
		res, err := Run(cfg, seed, TierDays[4], Reckless(cfg))
		if err != nil {
			t.Fatal(err)
		}
		formed, seized, fired := 0, 0, 0
		announced := map[int]bool{}
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.TaskForceFormed:
				formed++
				announced[ev.Day] = true
			case events.AssetSeized:
				seized++
			case events.Enforcement:
				if ev.Level == content.TaskForce {
					fired++
					if !announced[ev.Day-1] {
						t.Fatalf("seed %d day %d: the task force fired with no announcement the day before", seed, ev.Day)
					}
				}
			}
		}
		if seized > fired {
			t.Fatalf("seed %d: %d assets seized in %d task forces; one a firing", seed, seized, fired)
		}
		over := ""
		if res.Over != nil {
			over = res.Over.Cause
		}
		t.Logf("seed %d: %d task forces formed, %d fired, %d assets seized, %d owned at the end, %d lost; %s on day %d", seed, formed, fired, seized, len(res.World.Assets), res.World.Stats.AssetsLost, over, res.Days)
		if seized > 0 || res.Over != nil {
			punished++
		}
	}
	if punished < 6 {
		t.Fatalf("the reckless cartel was punished on %d of 10 seeds; money at tier 5 is not safety", punished)
	}
}

// TestAssetsSaveAndReplay runs the cartel to a day it owns assets and
// runs the plane or the tunnel, saves, loads and plays on beside the
// unsaved run: the same world every day after. A fresh run on the seed
// replays the same, assets and all.
func TestAssetsSaveAndReplay(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Day 270: the book is the first door since #391 and the tunnel the
	// second, standing on this seed from about day 255.
	const seed, at, on = 1, 270, 30
	a, _ := Run(cfg, seed, at+on, Cartel(cfg, 40))
	b, _ := Run(cfg, seed, at+on, Cartel(cfg, 40))
	if digest(a.World) != digest(b.World) {
		t.Fatal("the same seed diverged with assets in play")
	}
	c, _ := Run(cfg, seed, at, Cartel(cfg, 40))
	w := c.World
	if len(w.Assets) == 0 {
		t.Fatalf("day %d: the cartel owns no asset", at)
	}
	open := 0
	for _, r := range cfg.Routes.Routes {
		if r.Asset != "" && w.AssetLive(r.Asset) && w.Route(r.ID).Dial.On() {
			open++
		}
	}
	if open == 0 {
		t.Fatalf("day %d: neither the plane nor the tunnel is on", at)
	}
	if err := game.Save(1, w); err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Assets) != len(w.Assets) || loaded.Assets[0] != w.Assets[0] || loaded.Stats.PeakClean != w.Stats.PeakClean {
		t.Fatalf("the save lost the assets: %+v / %+v", loaded.Assets, w.Assets)
	}
	policy := Cartel(cfg, 40)
	straight, _ := RunFrom(cfg, w, on, policy)
	replayed, _ := RunFrom(cfg, loaded, on, Cartel(cfg, 40))
	if digest(straight.World) != digest(replayed.World) || digest(straight.World) != digest(a.World) {
		t.Fatalf("the run diverged after the save: %d / %d / %d", straight.World.NetWorth(), replayed.World.NetWorth(), a.World.NetWorth())
	}
}
