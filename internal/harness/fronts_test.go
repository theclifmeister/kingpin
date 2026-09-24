package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// frontProbe is one number a front's role can move (#344), read in a
// city by the sim that owns it: a route out of it, a corner's robbery,
// a sale's heat, the law's goodwill after a day, a block's price and
// rent, and the two with no city, the buyers' gaps and the offshore fee.
type frontProbe struct {
	name string
	city bool // read in a city; false is run-wide
	read func(set *sim.Set, clock *game.Clock, w *game.World, city string) float64
}

func frontProbes(cfg *content.Config) []frontProbe {
	product := cfg.Market.Products[0].ID
	corner := func(w *game.World, city string) game.Corner { return w.Cities[city].Corners[0] }
	return []frontProbe{
		{"route risk", true, func(set *sim.Set, _ *game.Clock, w *game.World, city string) float64 {
			for _, r := range cfg.Routes.Routes {
				if r.Other(city) != "" && r.Asset == "" {
					return set.Logistics.DayRisk(w, r, events.ShipNormal)
				}
			}
			return 0
		}},
		{"robbery", true, func(set *sim.Set, _ *game.Clock, w *game.World, city string) float64 {
			c := corner(w, city)
			return set.Territory.RobberyChance(w, &c)
		}},
		{"sale heat", true, func(set *sim.Set, _ *game.Clock, w *game.World, city string) float64 {
			return set.Heat.SaleHeat(w, city, product, 100, events.DialNormal)
		}},
		{"deed price", true, func(set *sim.Set, _ *game.Clock, w *game.World, city string) float64 {
			return float64(set.Territory.DeedPrice(w, corner(w, city)))
		}},
		{"deed rent", true, func(set *sim.Set, _ *game.Clock, w *game.World, city string) float64 {
			return float64(set.Territory.DeedRent(w, corner(w, city), 1_000_000))
		}},
		{"goodwill", true, func(_ *sim.Set, clock *game.Clock, w *game.World, city string) float64 {
			clock.EndDay(w) // a fresh world a read: the day is played on it
			return w.Cities[city].Goodwill
		}},
		{"buyer gaps", false, func(set *sim.Set, _ *game.Clock, w *game.World, _ string) float64 {
			lo, hi := set.Market.Gaps(w)
			return float64(lo + hi)
		}},
		{"offshore fee", false, func(set *sim.Set, _ *game.Clock, w *game.World, _ string) float64 {
			return float64(set.Laundering.Fee(w, 1_000_000))
		}},
	}
}

// TestFrontsPullTheirWay (#344): each kind of front, owned alone in
// home, moves the numbers its role names, the way its role says, and
// nothing else; a number with a city moves in the front's city and in
// no other, and the laundromat moves nothing (its role is the lowest
// audit risk in the file). Read on a fresh world through the sims that
// own each number, so the ledger's odds are the dice's.
func TestFrontsPullTheirWay(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	want := map[string]map[string]int{ // front -> probe -> +1 up, -1 down
		"laundromat":   {},
		"carwash":      {"route risk": -1},
		"restaurant":   {"robbery": -1, "sale heat": -1, "goodwill": +1},
		"nightclub":    {"sale heat": +1, "robbery": +1, "buyer gaps": -1},
		"construction": {"deed price": -1, "deed rent": +1},
		"exchange":     {"offshore fee": -1},
	}
	if len(want) != len(cfg.Laundering.Fronts) {
		t.Fatalf("%d fronts in the file, %d in the table", len(cfg.Laundering.Fronts), len(want))
	}
	set, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	// A fresh world a read, the same seed: one with the front in home,
	// one without.
	world := func(f *content.FrontConfig) *game.World {
		w := sim.NewWorld(cfg, 1)
		if f != nil {
			w.Fronts = append(w.Fronts, game.Front{ID: f.ID, Name: f.Name, Cost: f.Cost, City: w.Home().ID})
		}
		return w
	}
	fresh := world(nil)
	home, away := fresh.CityOrder[0], fresh.CityOrder[1]
	probes := frontProbes(cfg)
	for _, f := range cfg.Laundering.Fronts {
		for _, p := range probes {
			for _, city := range []string{home, away} {
				// A run-wide number is read once; every route in the file
				// joins home and the hub, so a road out of the hub is a
				// road out of home and is read there.
				if (!p.city || p.name == "route risk") && city == away {
					continue
				}
				before, after := p.read(set, clock, world(nil), city), p.read(set, clock, world(&f), city)
				dir := 0
				if city == home {
					dir = want[f.ID][p.name]
				}
				switch {
				case dir == 0 && after != before:
					t.Errorf("%s moved %s in %s: %v -> %v", f.ID, p.name, city, before, after)
				case dir > 0 && after <= before, dir < 0 && after >= before:
					t.Errorf("%s: %s in %s went %v -> %v, want it %s", f.ID, p.name, city, before, after, map[int]string{1: "up", -1: "down"}[dir])
				case dir != 0:
					t.Logf("%s: %s %v -> %v", f.ID, p.name, before, after)
				}
			}
		}
	}
}

// TestNoFrontEffectIsTheOldRun (#344): the roles at the identity (every
// multiplier a front carries set to 1, every sum to 0) are the run
// before the roles, byte for byte, for every policy that buys fronts,
// so the fold of a front adds nothing the file does not say; and a run
// that owns no front is the same on the file and with the roles boxed
// (harness.NoFrontRoles). cmd/balance -roles off prints the pre-#344
// numbers to the dollar.
func TestNoFrontEffectIsTheOldRun(t *testing.T) {
	t.Parallel()
	t.Run("identity", func(t *testing.T) {
		t.Parallel()
		assertOldRun(t, oldRunCase{
			never: "owned fronts whose roles do nothing",
			file:  frontRolesAtIdentity,
			box:   NoFrontRoles,
			seeds: 2,
			days:  120,
			every: 5,
			policies: map[string]func(*content.Config) Policy{
				"laundered":   func(c *content.Config) Policy { return Laundered(c, 40) },
				"boss":        func(c *content.Config) Policy { return Boss(c, 40, "") },
				"distributor": func(c *content.Config) Policy { return Distributor(c, 40) },
			},
		})
	})
	t.Run("no front", func(t *testing.T) {
		t.Parallel()
		assertOldRun(t, oldRunCase{
			never: "never bought a front",
			box:   NoFrontRoles,
			seeds: 3,
			days:  120,
			policies: map[string]func(*content.Config) Policy{
				"crewed":  func(c *content.Config) Policy { return Crewed(c, 40) },
				"managed": func(c *content.Config) Policy { return Managed(c, 50) },
			},
			after: func(t *testing.T, w *game.World, _ bool) {
				if len(w.Fronts) != 0 {
					t.Fatalf("a policy that buys no front owns %d", len(w.Fronts))
				}
			},
		})
	})
}

// frontRolesAtIdentity is cfg with every front's role kept and its
// effects at the identity: each multiplier it carries 1, each sum 0.
func frontRolesAtIdentity(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Upgrades.Fronts = nil
	for _, r := range cfg.Upgrades.Fronts {
		e := r.Effects
		for _, m := range []*float64{&e.RouteRiskMul, &e.RobberyMul, &e.SaleHeatMul, &e.DeedCostMul, &e.RentMul, &e.BuyerGapMul, &e.OffshoreFeeMul} {
			if *m != 0 {
				*m = 1
			}
		}
		e.GoodwillDay = 0
		r.Effects = e
		boxed.Upgrades.Fronts = append(boxed.Upgrades.Fronts, r)
	}
	return &boxed
}

// TestNoFrontDominates (#344): the laundered player that buys one kind
// of front and no other (harness.OneFront), one variant a kind, over
// twenty seeds to the tier-4 checkpoint: no kind is the best on every
// one of the median net worth, the fewest days hot (the hottest city
// at retire_heat or over, the patrol line) and the most corners held at
// the end. A choice of front is a choice about the business, not a
// ladder to climb.
func TestNoFrontDominates(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	line := cfg.Laundering.Offshore.RetireHeat
	type score struct{ worth, hot, held float64 }
	scores := map[string]score{}
	var ids []string
	for _, f := range cfg.Laundering.Fronts {
		ids = append(ids, f.ID)
	}
	results := make([]score, len(ids))
	t.Run("kinds", func(t *testing.T) {
		for i, id := range ids {
			t.Run(id, func(t *testing.T) {
				t.Parallel()
				var worth, hot, held []int
				for seed := uint64(1); seed <= 20; seed++ {
					w := sim.NewWorld(cfg, seed)
					policy := OneFront(cfg, 40, id)
					days := 0
					res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
						policy(w)
						if hottest(w).Heat >= line {
							days++
						}
					})
					if err != nil {
						t.Fatal(err)
					}
					worth = append(worth, res.NetWorthAt(Horizon))
					hot = append(hot, days)
					held = append(held, res.World.Held())
				}
				results[i] = score{median(worth), mean(hot), mean(held)}
			})
		}
	})
	for i, id := range ids {
		scores[id] = results[i]
		t.Logf("%-12s net worth %.0f, days hot %.1f, corners held %.2f", id, results[i].worth, results[i].hot, results[i].held)
	}
	for _, id := range ids {
		s := scores[id]
		best := true
		for _, other := range ids {
			o := scores[other]
			if other != id && (o.worth > s.worth || o.hot < s.hot || o.held > s.held) {
				best = false
				break
			}
		}
		if best {
			t.Errorf("%s is the best front on every count: %+v", id, scores)
		}
	}
}

// mean is the average of xs.
func mean(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return float64(sum) / float64(len(xs))
}

// median is the middle of xs as a float (the mean of the two middles
// for an even count).
func median(xs []int) float64 {
	s := append([]int(nil), xs...)
	sort.Ints(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return float64(s[n/2])
	}
	return float64(s[n/2-1]+s[n/2]) / 2
}
