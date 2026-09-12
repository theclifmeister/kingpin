package harness

import (
	"fmt"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// The rival's economy is priced in the ladder's unit (#139): its take is
// margin of what its corners move in the products the street sells at
// home, its wage, its hires and its claims are corner-days of that, and
// the muscle is what the take pays for. These pin the acceptance: a
// rival left alone holds a chest of ten to thirty days of wages at day
// 120 and never runs dry, a price war fought in earnest costs it heads,
// and the tribute at the middle cut is reported as a share of its take.

// A rival left alone (territory, five seeds, every temper) has a chest
// of between 10 and 30 days of its wage bill at day 120, median over
// the seeds, and never runs it dry: it does not starve on its own; and
// it ends owing its muscle less than a day's wages (the books are a
// running account, so a tail under a wage is a payroll the take covers).
func TestRivalEconomyBinds(t *testing.T) {
	cfg := content.MustLoad()
	for _, p := range content.Personalities {
		var days []float64
		var chests, wages, incomes []int
		for seed := uint64(1); seed <= 5; seed++ {
			w := sim.NewWorld(cfg, seed)
			w.Rival.Personality = p
			minCash := 1 << 40
			res, err := RunFrom(cfg, w, 120, func(w *game.World) {
				Territory(cfg, 40, 3)(w)
				if w.Rival.Arrived > 0 {
					minCash = min(minCash, w.Rival.Cash)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			fw := res.World
			income, bill := RivalBooks(cfg, fw)
			if minCash <= 0 || bill <= 0 {
				t.Fatalf("%s seed %d: the rival's chest fell to %d (wage bill %d a day at the end)", p, seed, minCash, bill)
			}
			if fw.Rival.Arrears >= float64(bill) {
				t.Fatalf("%s seed %d: %.0f owed at day 120, over a day's wages (%d), with %d a day coming in", p, seed, fw.Rival.Arrears, bill, income)
			}
			days = append(days, float64(fw.Rival.Cash)/float64(bill))
			chests, wages, incomes = append(chests, fw.Rival.Cash), append(wages, bill), append(incomes, income)
		}
		sort.Float64s(days)
		t.Logf("%s: chest at day 120 %.0f days of wages (median; %.0f..%.0f), chests %v, wages %v, take %v a day", p, days[2], days[0], days[4], chests, wages, incomes)
		if days[2] < 10 || days[2] > 30 {
			t.Fatalf("%s: a chest of %.0f days of wages at day 120; the acceptance is 10 to 30", p, days[2])
		}
	}
}

// pricewarFixture is a price war fought in earnest: a defensive rival
// dug in on the top row of home (docks, railyard, oldmill), the player
// working the three corners under it (fourth, depot, projects) with a
// runner and an enforcer on each, heat off so nobody lies low, the
// push flip boxed (push_flip 0: the rival's pushback on the cutter
// still comes and still costs war, but a corner it won would be a
// corner more of take, and the fixture measures money, not the fight),
// a stash that fills every pool, and, when war is on, every rival
// corner undercut at normal every night. It returns the rival's take
// summed over the second half of the run, its muscle at the end and
// the lowest it fell to.
func pricewarFixture(t *testing.T, seed uint64, days int, war bool) (take, muscle, low int) {
	t.Helper()
	cfg := NoHeat(content.MustLoad())
	cfg.Rivals.Rivals.PushFlip = 0
	w := sim.NewWorld(cfg, seed)
	home := w.Home().ID
	w.Rival.Personality, w.Rival.Arrived, w.Rival.Muscle = "defensive", 1, 8
	for _, id := range []string{"docks", "railyard", "oldmill"} {
		c := w.Corner(id)
		c.Owner, c.Since = game.OwnerRival, 0
	}
	w.Player.DirtyCash = 1_000_000
	w.Player.CarryLimit = 100_000
	next := 100
	for i, id := range []string{"fourth", "depot", "projects"} {
		if i > 0 {
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: next, Name: fmt.Sprintf("R%d", i), Role: "runner", Skill: 60, Units: 400, Loyalty: 90, Nerve: 60, Wage: 50})
			if err := w.Post(id, next); err != nil {
				t.Fatal(err)
			}
			next++
		}
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: next, Name: fmt.Sprintf("E%d", i), Role: "enforcer", Skill: 60, Loyalty: 90, Nerve: 80, Wage: 55})
		if err := w.Post(id, next); err != nil {
			t.Fatal(err)
		}
		next++
	}
	w.Crew.NextID = next
	low = 1 << 30
	res, err := RunFrom(cfg, w, days, func(w *game.World) {
		w.Player.DirtyCash = 1_000_000
		for _, p := range w.Products {
			w.SetStock(home, p, 5000)
			_ = w.PlaceSell(home, p, int(3*w.Demand(home, p))+200, events.DialNormal)
		}
		if war {
			for _, c := range w.Home().Corners {
				if w.CanUndercut(c.ID) == nil {
					_ = w.Undercut(c.ID, events.DialNormal)
				}
			}
		}
		if w.Day > days/2 {
			in, _ := RivalBooks(cfg, w)
			take += in
		}
		if w.Day > 10 {
			low = min(low, w.Rival.Muscle)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return take, res.World.Rival.Muscle, low
}

// A price war fought in earnest starves the muscle (#139, the bullet
// #68 had to drop): in the fixture the rival's take over the second
// month is a fifth or so under the same fixture with no undercuts, and
// its muscle at day 60 is at least a head under it on every seed, with
// no dice in it (the squeeze is the market's arithmetic, the arrears
// the rival's). The harness's pricewar policy at normal is logged
// against territory as muscle per corner held (the raw count is
// confounded by #68's pushback, which can win the rival ground): with
// one corner undercut on the nights it sells, the crewed player's bag
// fills the pool only as far as its own orders go, so the take falls
// one to four points against a head that is a tenth of it, and the
// median reads level (2.67 against 2.67), so what is pinned is that it
// is never above: the number is in the log.
func TestPricewarStarvesTheMuscle(t *testing.T) {
	const days = 60
	for seed := uint64(1); seed <= 5; seed++ {
		quietTake, quietMuscle, quietLow := pricewarFixture(t, seed, days, false)
		warTake, warMuscle, warLow := pricewarFixture(t, seed, days, true)
		t.Logf("seed %d: take over days %d-%d %d against %d (%.0f%%), muscle at day %d %d against %d (lowest %d and %d)",
			seed, days/2+1, days, warTake, quietTake, 100*float64(warTake)/float64(max(1, quietTake)), days, warMuscle, quietMuscle, warLow, quietLow)
		if warTake >= quietTake || warMuscle >= quietMuscle {
			t.Fatalf("seed %d: the price war left the rival %d a day and %d muscle against %d and %d in peace", seed, warTake, warMuscle, quietTake, quietMuscle)
		}
	}
	cfg := content.MustLoad()
	var perCorner [2][]float64
	for i, policy := range []Policy{Territory(cfg, 40, 3), Pricewar(cfg, 40, 3, events.DialNormal)} {
		for seed := uint64(1); seed <= 5; seed++ {
			w := sim.NewWorld(cfg, seed)
			w.Rival.Personality = "defensive"
			res, err := RunFrom(cfg, w, 120, policy)
			if err != nil {
				t.Fatal(err)
			}
			perCorner[i] = append(perCorner[i], float64(res.World.Rival.Muscle)/float64(max(1, res.World.RivalHeld())))
		}
		sort.Float64s(perCorner[i])
	}
	t.Logf("harness pricewar at normal against defensive over 120 days: muscle per corner held %.2f (median) against territory's %.2f", perCorner[1][2], perCorner[0][2])
	if perCorner[1][2] > perCorner[0][2] {
		t.Fatalf("the price war fed the rival's muscle: %.2f a corner against territory's %.2f", perCorner[1][2], perCorner[0][2])
	}
}

// The tribute at the middle cut (tribute_cuts[1] of rivals.Sim.TributeBase,
// the player's daily street value in the products the rival deals in,
// #162) as a share of the rival's take under territory at day 120: the
// median over five seeds lands between 0.3x and 1x. At equal ground the
// ratio is the cut over the rival's margin, 0.10 / 0.30; before #162 the
// base was World.StreetValue, which counted the port's product on the
// player's corners once the wholesaler's line was crossed while the take
// did not, and the middle cut read 6.1x the take (2.9x to 13.5x by seed).
func TestTributeShareOfTheTake(t *testing.T) {
	cfg := content.MustLoad()
	rv := rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects, cfg.Upgrades)
	var shares []float64
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Rival.Personality = "defensive"
		res, err := RunFrom(cfg, w, 120, Territory(cfg, 40, 3))
		if err != nil {
			t.Fatal(err)
		}
		fw := res.World
		income, _ := RivalBooks(cfg, fw)
		cut := rv.Cut(fw, cfg.Rivals.Diplomacy.TributeCuts[1])
		shares = append(shares, float64(cut)/float64(max(1, income)))
		t.Logf("seed %d: a tribute at the middle cut is %d a day against a take of %d (%.2fx)", seed, cut, income, float64(cut)/float64(max(1, income)))
	}
	sort.Float64s(shares)
	t.Logf("median: the middle cut is %.2fx the rival's take at day 120", shares[2])
	if shares[2] < 0.3 || shares[2] > 1 {
		t.Fatalf("the middle cut is %.2fx the rival's take at day 120, want 0.3x to 1x", shares[2])
	}
}
