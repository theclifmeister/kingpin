package harness

import (
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// TestNoDeedIsTheOldRun (#194): a run that never buys a deed is
// byte-for-byte the run before the property existed. The laundered and
// the crewed players (neither buys one) are hashed daily on the file
// and on the file with the [deed] table boxed (harness.NoDeeds;
// assertOldRun: three seeds, 120 days), and nothing of
// the kind is emitted or counted.
func TestNoDeedIsTheOldRun(t *testing.T) {
	t.Parallel()
	assertOldRun(t, oldRunCase{
		never: "never bought a deed",
		seeds: 3,
		days:  120,
		box:   NoDeeds,
		policies: map[string]func(*content.Config) Policy{
			"laundered": func(c *content.Config) Policy { return Laundered(c, 40) },
			"crewed":    func(c *content.Config) Policy { return Crewed(c, 40) },
		},
		forbid: func(e events.Event) bool {
			switch e.(type) {
			case events.DeedBought, events.DeedsBought, events.DeedRent, events.DeedSeized:
				return true
			}
			return false
		},
		after: func(t *testing.T, w *game.World, _ bool) {
			if len(w.Deeds()) != 0 || w.Stats.Deeds != 0 || w.Stats.DeedCash != 0 || w.Stats.DeedRent != 0 || w.Stats.DeedsSeized != 0 || w.Law.Forfeited != 0 {
				t.Fatalf("the property moved with nobody buying: %+v", w.Stats)
			}
		},
	})
}

// landlord is the passive player with the deed to every block it
// holds (#194): Territory under harness.Landlord, with the clean cash
// to buy and a wash behind it so the DA never takes one back.
func landlord(cfg *content.Config, seed uint64, deeds bool) *game.World {
	w := sim.NewWorld(cfg, seed)
	w.Rival().Personality = "expansionist"
	if deeds {
		w.Player.CleanCash = 50_000_000
		w.Stats.Laundered = 1_000_000_000
	}
	return w
}

// TestDeedSlowsTheRivalNeverStopsIt (#194): the passive player with the
// deed to every block it holds, next to an expansionist, still loses a
// deeded corner by day 60 on some seed of ten and never on every seed,
// and over the seeds loses no more corners than the same player with
// no deed: push_mul slows the rival and never stops it, the Street
// branch's rule.
func TestDeedSlowsTheRivalNeverStopsIt(t *testing.T) {
	t.Parallel()
	// The duel (#43, harness.OneFaction): this pins a mechanism on a seed, and the table moves the seed's dice.
	cfg := OneFaction(content.MustLoad())
	const days = 60
	firstLoss := func(seed uint64, deeds bool) (day, lost int) {
		w := landlord(cfg, seed, deeds)
		policy := Territory(cfg, 40, 3)
		if deeds {
			policy = Landlord(cfg, policy)
		}
		res, err := RunFrom(cfg, w, days, policy)
		if err != nil {
			t.Fatal(err)
		}
		if res.World.Stats.Strikes != 0 {
			t.Fatalf("seed %d: the passive player sent enforcers", seed)
		}
		for _, e := range res.Events {
			if ct, ok := e.(events.CornerTaken); ok && ct.From == game.OwnerPlayer {
				if deeds && res.World.Corner(ct.Corner).Deed == nil {
					continue // a corner taken before its deed was bought
				}
				if day == 0 {
					day = ct.Day
				}
				lost++
			}
		}
		if deeds && res.World.Stats.DeedsSeized != 0 {
			t.Fatalf("seed %d: the DA took a deed off a player with a billion washed", seed)
		}
		return day, lost
	}
	plainDays, deedDays, plainLost, deedLost, seedsLost := 0, 0, 0, 0, 0
	for seed := uint64(1); seed <= 10; seed++ {
		pd, pl := firstLoss(seed, false)
		dd, dl := firstLoss(seed, true)
		if dl > 0 {
			seedsLost++
		}
		t.Logf("seed %d: first deeded corner lost on day %d (day %d bare); %d lost by day %d (%d bare)", seed, dd, pd, dl, days, pl)
		plainDays, deedDays, plainLost, deedLost = plainDays+pd, deedDays+dd, plainLost+pl, deedLost+dl
	}
	t.Logf("over 10 seeds: a deeded corner taken by day %d on %d seeds; %d corners lost with the deeds, %d bare", days, seedsLost, deedLost, plainLost)
	if seedsLost == 0 {
		t.Fatalf("no seed of ten lost a deeded corner by day %d: a deed must slow the rival, never stop it", days)
	}
	if seedsLost == 10 {
		t.Fatalf("every seed of ten lost a deeded corner by day %d: the deed slowed nothing", days)
	}
	if deedLost > plainLost {
		t.Fatalf("the deeds did not slow the rival: %d corners lost against %d bare", deedLost, plainLost)
	}
}

// TestForfeiture (#194): the boss that buys the block under its corners
// past the DA's line (harness.Landlord over BossAt at margin 1: every
// clean dollar into property, whatever the fronts have washed) loses
// deeds to the forfeiture and takes forfeit_evidence pages the morning
// after each; the boss as played keeps under the line and never loses
// one, over the same seeds.
func TestForfeiture(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	seized, pages := 0, 0
	for seed := uint64(1); seed <= 3; seed++ {
		res, err := Run(cfg, seed, Horizon, Landlord(cfg, BossAt(cfg, 40, "", 1)))
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		seized += w.Stats.DeedsSeized
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.DeedSeized:
				if ev.Spent <= ev.Limit || ev.Price <= 0 {
					t.Fatalf("seed %d: a seizure under the line: %+v", seed, ev)
				}
			case events.HeatChanged:
				for _, r := range ev.Reasons {
					if strings.Contains(r, "forfeiture") {
						pages++
					}
				}
			}
		}
		t.Logf("seed %d: the landlord bought %d deeds for %d, lost %d to the DA; %d washed", seed, w.Stats.Deeds, w.Stats.DeedCash, w.Stats.DeedsSeized, w.Stats.Laundered)
	}
	if seized == 0 || pages == 0 {
		t.Fatalf("the landlord over the line lost %d deeds and took pages %d times over three seeds", seized, pages)
	}
	if pages != seized {
		t.Fatalf("%d seizures filed pages %d times: one morning after each", seized, pages)
	}
	for seed := uint64(1); seed <= 3; seed++ {
		res, err := Run(cfg, seed, Horizon, Boss(cfg, 40, ""))
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		if w.Stats.DeedsSeized != 0 || w.Law.Forfeited != 0 {
			t.Fatalf("seed %d: the boss under the line lost %d deeds", seed, w.Stats.DeedsSeized)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.DeedSeized); ok {
				t.Fatalf("seed %d: %+v on the boss under the line", seed, ev)
			}
		}
		t.Logf("seed %d: the boss bought %d deeds for %d against a line of %d", seed, w.Stats.Deeds, w.Stats.DeedCash, int(cfg.City.Deed.ForfeitRatio*float64(w.Stats.Laundered)))
	}
}

// TestDeedSizing (#194): over ten seeds at tier 4's checkpoint the
// boss's clean cash in hand is under half of the $30M #191 measured
// (the deeds join the levels and the account in draining it), its
// rent a day is under a tenth of what its fronts wash a day (rent is
// cover, not income; the levels are the income), and it holds deeds.
func TestDeedSizing(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	tr := territory.New(cfg)
	ld := laundering.New(cfg)
	var clean, rents, washes, deeds []int
	for seed := uint64(1); seed <= 10; seed++ {
		res, err := Run(cfg, seed, Horizon, Boss(cfg, 40, ""))
		if err != nil {
			t.Fatal(err)
		}
		w := res.World
		rent := 0
		for _, c := range w.Deeds() {
			rent += tr.DeedRent(w, c, c.Deed.Price)
		}
		clean = append(clean, w.Player.CleanCash)
		rents = append(rents, rent)
		washes = append(washes, min(ld.Capacity(w), w.Stats.Laundered/max(1, res.Days)))
		deeds = append(deeds, len(w.Deeds()))
	}
	for _, s := range [][]int{clean, rents, washes, deeds} {
		sort.Ints(s)
	}
	med := func(s []int) int { return s[len(s)/2] }
	t.Logf("boss at day %d over ten seeds (medians): clean %d, %d deeds paying %d/day against a wash of %d/day", Horizon, med(clean), med(deeds), med(rents), med(washes))
	if med(clean) >= 15_000_000 {
		t.Errorf("the boss holds %d clean at day %d, half of #191's $30M or more", med(clean), Horizon)
	}
	if med(rents)*10 >= med(washes) {
		t.Errorf("rent %d/day is not under a tenth of the wash %d/day: rent is cover, not income", med(rents), med(washes))
	}
	if med(deeds) == 0 {
		t.Errorf("the boss holds no deed at day %d", Horizon)
	}
}

// TestDeedsSurviveSaveAndDetermine (#194): a run with deeds replays
// byte-for-byte on its seed, and a save taken with deeds on the map
// (yours and the rival's) loads with every deed, price and day intact.
func TestDeedsSurviveSaveAndDetermine(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := OneFaction(content.MustLoad())
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	play := func() (Result, error) {
		w := landlord(cfg, 4, true)
		w.Rival().Arrived = 1
		return RunFrom(cfg, w, 20, func(w *game.World) {
			Landlord(cfg, Territory(cfg, 40, 3))(w)
			// The rival's block too, once it has one.
			for _, c := range w.Corners() {
				if c.Owner == game.OwnerRival && c.Deed == nil {
					_ = w.BuyDeed(c.ID, territory.New(cfg).DeedPrice(w, c))
					break
				}
			}
		})
	}
	a, err := play()
	if err != nil {
		t.Fatal(err)
	}
	b, err := play()
	if err != nil {
		t.Fatal(err)
	}
	if digest(a.World) != digest(b.World) {
		t.Fatal("two runs on one seed with deeds differ")
	}
	deeds := a.World.Deeds()
	if len(deeds) < 2 {
		t.Fatalf("nothing to save: %d deeds", len(deeds))
	}
	theirs := false
	for _, c := range deeds {
		theirs = theirs || c.Owner == game.OwnerRival
	}
	if !theirs {
		t.Log("no deed on a rival block by day 20; the save carries yours alone")
	}
	if err := game.Save(1, a.World); err != nil {
		t.Fatal(err)
	}
	got, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Deeds()) != len(deeds) {
		t.Fatalf("%d deeds after load, want %d", len(got.Deeds()), len(deeds))
	}
	for _, c := range deeds {
		d := got.Corner(c.ID).Deed
		if d == nil || *d != *c.Deed {
			t.Fatalf("%s after load: %+v, want %+v", c.ID, d, c.Deed)
		}
	}
	if got.Stats.Deeds != a.World.Stats.Deeds || got.Stats.DeedCash != a.World.Stats.DeedCash || got.Stats.DeedRent != a.World.Stats.DeedRent {
		t.Fatalf("stats after load %+v, want %+v", got.Stats, a.World.Stats)
	}
}
