package harness

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// The price war (#68): the player's third answer to a rival corner,
// money. These pin the mechanic's invariants and what it buys and costs
// against the other two answers. The issue's "starve the muscle" bullet
// is in rival_economy_test.go (#139, TestPricewarStarvesTheMuscle): the
// rival's muscle is what its take pays for, so a war fought in earnest
// costs it heads; the pricewar policy's one corner on the nights it
// sells takes one to four points of the take, under a head.

// pricewarRun plays a policy on a seed with the rival's temper forced,
// calling check every morning after the policy acts.
func pricewarRun(t *testing.T, cfg *content.Config, seed uint64, days int, personality string, policy Policy, check func(w *game.World)) Result {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	if personality != "" {
		w.Rival.Personality = personality
	}
	res, err := RunFrom(cfg, w, days, func(w *game.World) {
		policy(w)
		if check != nil {
			check(w)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// Invariants under the pricewar policy, every day of five seeds against
// every temper: every corner's squeeze is in 0..1; a rival corner is
// squeezed only by a price war the report names, so its squeeze is zero
// on any morning nobody undercut it; the units an undercut moves never
// exceed Steal of the corner's demand for the product (the morning's
// demand is the night's, the market drifts it after the orders); your
// own corners' demand reads the same as the corners' whatever is
// squeezed; and nothing moves under a truce or a tribute.
func TestPricewarInvariants(t *testing.T) {
	cfg := content.MustLoad()
	mk, err := market.New(cfg.Market, cfg.City, cfg.Routes.Shipping, cfg.Upgrades, cfg.Reputation.Effects, cfg.Buyers, cfg.Suppliers, cfg.Rivals.Pricewar)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range content.Personalities {
		for seed := uint64(1); seed <= 5; seed++ {
			// What tonight's undercuts may move at most, by corner and
			// product, read once the policy has queued them.
			caps := map[string]int{}
			var undercutDay int
			check := func(w *game.World) {
				home := w.Home()
				for _, c := range home.Corners {
					if c.Squeeze < 0 || c.Squeeze >= 1 {
						t.Fatalf("%s seed %d day %d: %s squeeze %.3f", p, seed, w.Day, c.ID, c.Squeeze)
					}
					if c.Squeeze > 0 && c.Owner == game.OwnerRival && c.StarvedDay != w.Day {
						t.Fatalf("%s seed %d day %d: %s squeezed with no price war on it last night", p, seed, w.Day, c.ID)
					}
				}
				for _, id := range w.Products {
					if m := w.Product(home.ID, id); m != nil && math.Abs(w.Demand(home.ID, id)-m.Demand*w.HeldShare(home.ID, id)) > 1e-9 {
						t.Fatalf("%s seed %d day %d: own demand %.2f is not the corners' %.2f", p, seed, w.Day, w.Demand(home.ID, id), m.Demand*w.HeldShare(home.ID, id))
					}
				}
				caps, undercutDay = map[string]int{}, w.Day+1
				for id, d := range w.Undercuts {
					c := w.Corner(id)
					for _, pid := range w.Products {
						if m := w.Product(home.ID, pid); m != nil {
							caps[id+"/"+pid] = int(math.Round(m.Demand * c.Full(pid) * mk.Steal(w, *c, d)))
						}
					}
				}
			}
			res := pricewarRun(t, cfg, seed, Horizon, p, Pricewar(cfg, 40, 3, events.DialNormal), check)
			peace := map[int]bool{}
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.PlayerUndercut:
					if ev.Day == undercutDay {
						if ev.Units > caps[ev.Corner+"/"+ev.Product] {
							t.Fatalf("%s seed %d day %d: %d %s off %s, over the share's %d", p, seed, ev.Day, ev.Units, ev.Product, ev.Corner, caps[ev.Corner+"/"+ev.Product])
						}
					}
					if ev.Units <= 0 || ev.Share <= 0 || ev.Share > 1 {
						t.Fatalf("%s seed %d day %d: %+v", p, seed, ev.Day, ev)
					}
					if peace[ev.Day] {
						t.Fatalf("%s seed %d day %d: an undercut resolved under a truce or a tribute", p, seed, ev.Day)
					}
				case events.DealAccepted:
					if ev.Deal != game.DealSplit {
						peace[ev.Day+1] = true
					}
				case events.TributePaid:
					peace[ev.Day+1] = true
				}
			}
		}
	}
}

// Under a truce nothing moves: the pricewar policy with a truce sealed
// from day 20 for 30 days undercuts before and after it and never
// during, and the action is refused on every morning of it.
func TestPricewarKeepsThePeace(t *testing.T) {
	cfg := content.MustLoad()
	policy := Pricewar(cfg, 40, 3, events.DialNormal)
	for seed := uint64(1); seed <= 3; seed++ {
		refused := 0
		res := pricewarRun(t, cfg, seed, 80, "defensive", policy, func(w *game.World) {
			if w.Day == 20 {
				w.Rival.Deals = append(w.Rival.Deals, game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Since: w.Day, Until: w.Day + 30})
			}
			if w.AtPeace() {
				for _, c := range w.Home().Corners {
					if c.Owner == game.OwnerRival && w.Undercut(c.ID, events.DialNormal) == game.ErrAtPeace {
						refused++
					}
				}
			}
		})
		during := 0
		for _, e := range res.Events {
			if u, ok := e.(events.PlayerUndercut); ok && u.Day > 20 && u.Day <= 50 {
				during++
			}
		}
		if during > 0 || refused == 0 {
			t.Fatalf("seed %d: %d undercuts during the truce, %d refusals", seed, during, refused)
		}
	}
}

// A price war cuts what the rival's corners earn: against a defensive
// rival, which never gives a corner up, what the squeeze takes off the
// rival's income (the corners' full income less what it books, summed
// over 120 days) is positive on every seed. It is a point or so of
// what the rival books: the squeeze is a share of the corner's trade
// on the nights the bag-limited player sells into the pool, and the
// rival is seldom next door before day 60. The territory player's
// rival's income is logged beside it, not pinned: the two runs diverge
// on the rival's answer and the day's prices by more than the squeeze
// (seed 2 reads 13% over at the same corner count). RivalBooks logs
// the take against the wage bill (#139: a head is a tenth of it).
func TestPricewarCutsTheRivalsIncome(t *testing.T) {
	cfg := content.MustLoad()
	rv := rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects, cfg.Upgrades)
	for seed := uint64(1); seed <= 5; seed++ {
		income, early := map[string]int{}, map[string]int{}
		corners := map[string]int{}
		var books [2]int
		taken := 0
		for _, pol := range []struct {
			name   string
			policy Policy
		}{{"territory", Territory(cfg, 40, 3)}, {"pricewar", Pricewar(cfg, 40, 3, events.DialNormal)}} {
			name := pol.name
			res := pricewarRun(t, cfg, seed, 120, "defensive", pol.policy, func(w *game.World) {
				in, _ := RivalBooks(cfg, w)
				income[name] += in
				if w.Day <= 60 {
					early[name] += in
				}
				if name == "pricewar" {
					full := 0.0
					for _, c := range w.Home().Corners {
						if c.Owner == game.OwnerRival {
							full += float64(rv.CornerIncome(w, c))
						}
					}
					taken += int(full) - in
				}
			})
			corners[name] = res.World.RivalHeld()
			books[0], books[1] = RivalBooks(cfg, res.World)
		}
		t.Logf("seed %d: rival income to day 60 territory %d, pricewar %d (%.1f%%); to day 120 %d and %d (%.1f%%), the squeeze took %d; corners %d and %d; day 120 income %d a day against wages %d",
			seed, early["territory"], early["pricewar"], 100*float64(early["pricewar"])/float64(early["territory"]),
			income["territory"], income["pricewar"], 100*float64(income["pricewar"])/float64(income["territory"]), taken, corners["territory"], corners["pricewar"], books[0], books[1])
		if taken <= 0 {
			t.Fatalf("seed %d: the price war took nothing off the rival's income", seed)
		}
	}
}

// The rival answers: over 200 days on five seeds an opportunist gives
// corners up to the price war and an expansionist pushes back on the
// corner doing the cutting; a defensive rival never gives one up.
func TestPricewarIsAnswered(t *testing.T) {
	cfg := content.MustLoad()
	policy := Pricewar(cfg, 40, 3, events.DialNormal)
	count := func(p string) (abandons, pushes, taken int) {
		for seed := uint64(1); seed <= 5; seed++ {
			res := pricewarRun(t, cfg, seed, Horizon, p, policy, nil)
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.RivalAbandoned:
					abandons++
				case events.RivalPushed:
					if ev.Pricewar {
						pushes++
					}
				case events.CornerTaken:
					if ev.Pricewar {
						pushes++
						taken++
					}
				}
			}
		}
		return
	}
	for _, p := range content.Personalities {
		abandons, pushes, taken := count(p)
		t.Logf("%s: %d corners given up, %d pushes back (%d took the corner) over 5 seeds and %d days", p, abandons, pushes, taken, Horizon)
		switch p {
		case "opportunist":
			if abandons == 0 {
				t.Fatalf("an opportunist never gave a corner up")
			}
		case "expansionist":
			if pushes == 0 {
				t.Fatalf("an expansionist never pushed back")
			}
		case "defensive":
			if abandons != 0 {
				t.Fatalf("a defensive rival gave %d corners up", abandons)
			}
		}
	}
}

// Ground bought with money: the price war's median net worth at day 120
// is under crewed's (the discount and the glut are real) over five
// seeds against the rival the seed draws; territory's at the same
// corners is logged (the ground an opportunist gives up is worth more
// to the price war than the discount costs it).
func TestPricewarCostsMargin(t *testing.T) {
	cfg := content.MustLoad()
	median := func(policy Policy) int {
		var worths []int
		for seed := uint64(1); seed <= 5; seed++ {
			res := pricewarRun(t, cfg, seed, 120, "", policy, nil)
			worths = append(worths, res.NetWorthAt(120))
		}
		sort.Ints(worths)
		return worths[len(worths)/2]
	}
	war, crewed, territory := median(Pricewar(cfg, 40, 3, events.DialNormal)), median(Crewed(cfg, 40)), median(Territory(cfg, 40, 3))
	t.Logf("median net worth on day 120: pricewar %d, crewed %d, territory %d", war, crewed, territory)
	if war >= crewed {
		t.Fatalf("the price war paid: %d against crewed's %d", war, crewed)
	}
}

// Cheaper than the enforcers: against a defensive rival over 120 days
// the price war draws less heat from the fight (the rival's calls; a
// hit war's strikes, calls and crackdowns) and never gets the war as
// loud (Rival.War at its loudest over the run) as the hit war does,
// over five seeds. Both policies lie low at the same line, so the heat
// gauge at day 120 reads the same for both: what differs is what the
// fight added. The corners each leaves the rival are logged: the hit
// war routs a defensive rival in its first weeks and the war then
// fades, the price war simmers for the run at a few points of war.
func TestPricewarIsQuieterThanAHitWar(t *testing.T) {
	cfg := content.MustLoad()
	var heat, war [2][]float64
	var corners, lowDays [2][]int
	for i, policy := range []Policy{Pricewar(cfg, 40, 3, events.DialNormal), Warlike(cfg, 40, 3, events.ForceHit)} {
		for seed := uint64(1); seed <= 5; seed++ {
			peak, low := 0.0, 0
			res := pricewarRun(t, cfg, seed, 120, "defensive", policy, func(w *game.World) {
				peak = math.Max(peak, w.Rival.War)
				if w.LieLow {
					low++
				}
			})
			added := 0.0
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.CornerStruck:
					added += ev.Heat
				case events.RivalTippedPolice:
					added += ev.Heat
				case events.WarEscalated:
					added += ev.Heat
				}
			}
			heat[i] = append(heat[i], added)
			war[i] = append(war[i], peak)
			corners[i] = append(corners[i], res.World.RivalHeld())
			lowDays[i] = append(lowDays[i], low)
		}
		sort.Float64s(heat[i])
		sort.Float64s(war[i])
		sort.Ints(corners[i])
		sort.Ints(lowDays[i])
	}
	t.Logf("medians over 120 days: pricewar heat from the fight %.0f, war at its loudest %.0f, %d days lying low, rival corners %d at the end; hit war heat %.0f, war %.0f, %d days lying low, rival corners %d",
		heat[0][2], war[0][2], lowDays[0][2], corners[0][2], heat[1][2], war[1][2], lowDays[1][2], corners[1][2])
	if heat[0][2] >= heat[1][2] || war[0][2] >= war[1][2] {
		t.Fatalf("the price war is not the quieter answer: heat %.0f against %.0f, war %.0f against %.0f", heat[0][2], heat[1][2], war[0][2], war[1][2])
	}
}

// Determinism and the save: the same seed plays the same twice with
// undercuts queued every day, and a run saved on a morning with one
// queued replays the rest of the run exactly.
func TestPricewarIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	policy := func() Policy { return Pricewar(cfg, 40, 3, events.DialNormal) }
	rivalSet := func(w *game.World) { w.Rival.Personality = "opportunist" }
	play := func(days int) Result {
		w := sim.NewWorld(cfg, 6)
		rivalSet(w)
		res, err := RunFrom(cfg, w, days, policy())
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	a, b := play(120), play(120)
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	undercuts := 0
	for _, e := range a.Events {
		if _, ok := e.(events.PlayerUndercut); ok {
			undercuts++
		}
	}
	if undercuts == 0 {
		t.Fatal("the pricewar player never undercut")
	}
	// Save on a morning with an undercut queued (the first from day 40):
	// the queue is per-day scratch and the policy queues it again after
	// the load, so the save carries the corner's squeeze and starved
	// days, not the queue.
	c := play(40)
	policy()(c.World)
	for len(c.World.Undercuts) == 0 && c.World.Day < 100 {
		c = play(c.World.Day + 1)
		policy()(c.World)
	}
	if len(c.World.Undercuts) == 0 {
		t.Fatal("between day 40 and 100 of seed 6 there was never anything to undercut")
	}
	if err := game.Save(1, c.World); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range c.World.Home().Corners {
		l := loaded.Corner(c.ID)
		if l.Squeeze != c.Squeeze || l.Starved != c.Starved || l.StarvedDay != c.StarvedDay {
			t.Fatalf("%s loaded as %+v, saved %+v", c.ID, *l, c)
		}
	}
	d, _ := RunFrom(cfg, loaded, 120-c.World.Day, policy())
	rest := a.Events[len(c.Events):]
	if len(d.Events) != len(rest) {
		t.Fatalf("after loading, %d events for the last %d days, want %d", len(d.Events), 120-c.World.Day, len(rest))
	}
	for i := range rest {
		if fmt.Sprintf("%#v", rest[i]) != fmt.Sprintf("%#v", d.Events[i]) {
			t.Fatalf("event %d after the save differs:\n%#v\n%#v", i, rest[i], d.Events[i])
		}
	}
	if d.World.NetWorth() != a.World.NetWorth() {
		t.Fatalf("net worth %d after the save, %d straight through", d.World.NetWorth(), a.World.NetWorth())
	}
}

// A run that never undercuts is the old run: the territory player's
// rival corners are never squeezed and the report never names a price
// war, so every pinned number stands (TestMoneyCurve is the guard).
func TestNoUndercutIsTheOldRun(t *testing.T) {
	cfg := content.MustLoad()
	rv := rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects, cfg.Upgrades)
	res := pricewarRun(t, cfg, 1, 120, "", Territory(cfg, 40, 3), func(w *game.World) {
		for _, c := range w.Home().Corners {
			if c.Owner == game.OwnerRival && (c.Squeeze != 0 || c.Starved != 0) {
				t.Fatalf("day %d: %s squeezed %.2f starved %d with no price war", w.Day, c.ID, c.Squeeze, c.Starved)
			}
		}
		_ = rv
	})
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.PlayerUndercut, events.RivalAbandoned:
			t.Fatalf("%+v in a run with no price war", ev)
		case events.PlayerSold:
			if ev.Undercut != 0 {
				t.Fatalf("day %d: %d units undercut with no price war", ev.Day, ev.Undercut)
			}
		}
	}
}
