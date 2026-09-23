package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestNoExpansionIsTheOldRun (#341): every policy in the registry, hashed
// daily to the horizon on the file and with the expansion boxed, reads
// the same world up to the day a faction first sends scouts, and the
// whole run for a policy whose take away from home never crosses
// take_min. The window on the take is kept either way: it is
// bookkeeping, and it rolls nothing.
func TestNoExpansionIsTheOldRun(t *testing.T) {
	t.Parallel()
	policies := map[string]func(*content.Config) Policy{}
	for _, p := range Policies {
		policies[p.Name] = func(c *content.Config) Policy { return p.Make(c, DefaultPolicyOpts()) }
	}
	assertOldRun(t, oldRunCase{
		never: "never drew a faction to a city",
		seeds: 1,
		days:  Horizon,
		box:   NoExpansion,
		asRun: true,
		until: func(_ *game.World, today []events.Event) bool {
			for _, e := range today {
				if _, ok := e.(events.RivalScouting); ok {
					return true
				}
			}
			return false
		},
		policies: policies,
	})
}

// expansionRun is one policy's run to the horizon with what the
// expansion did in it, by faction: the days of its scouts (the latest
// first sighting), its recruiting and its arrival away from home.
type expansionRun struct {
	res      Result
	scouts   map[string][]int
	recruits map[string][]int
	arrivals map[string]int
	withdrew map[string][]int
}

func playExpansion(t *testing.T, cfg *content.Config, seed uint64, policy Policy) expansionRun {
	t.Helper()
	res, err := Run(cfg, seed, Horizon, policy)
	if err != nil {
		t.Fatal(err)
	}
	x := expansionRun{res: res, scouts: map[string][]int{}, recruits: map[string][]int{}, arrivals: map[string]int{}, withdrew: map[string][]int{}}
	home := res.World.Home().ID
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.RivalScouting:
			x.scouts[ev.Faction] = append(x.scouts[ev.Faction], ev.Day)
		case events.RivalRecruiting:
			x.recruits[ev.Faction] = append(x.recruits[ev.Faction], ev.Day)
		case events.RivalWithdrew:
			x.withdrew[ev.Faction] = append(x.withdrew[ev.Faction], ev.Day)
		case events.RivalMovedIn:
			if r := res.World.Faction(ev.Faction); r != nil && r.Home != "" && r.Home != home {
				x.arrivals[ev.Faction] = ev.Day
			}
		}
	}
	return x
}

// TestExpansionIsTelegraphed (#341): no faction is seated in a city away
// from home fewer than scout_days + arrive_days after its scouts were
// seen there, it recruits no sooner than scout_days after them, and
// nobody arrives away from home unannounced (away = 0 by ruling, so
// every faction there came through the stages).
func TestExpansionIsTelegraphed(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	e := cfg.Rivals.Expansion
	arrived := 0
	for _, name := range []string{"distributor", "boss"} {
		np, _ := PolicyNamed(name)
		for seed := uint64(1); seed <= 6; seed++ {
			x := playExpansion(t, cfg, seed, np.Make(cfg, DefaultPolicyOpts()))
			for id, day := range x.arrivals {
				ss := x.scouts[id]
				if len(ss) == 0 {
					t.Fatalf("%s seed %d: %s arrived away from home on day %d with no scouts seen", name, seed, id, day)
				}
				last := ss[len(ss)-1]
				if day-last < e.ScoutDays+e.ArriveDays {
					t.Fatalf("%s seed %d: %s arrived on day %d, %d days after its scouts on day %d; want at least %d", name, seed, id, day, day-last, last, e.ScoutDays+e.ArriveDays)
				}
				rs := x.recruits[id]
				if len(rs) == 0 || rs[len(rs)-1] < last+e.ScoutDays || rs[len(rs)-1] > day {
					t.Fatalf("%s seed %d: %s scouted on day %d, recruited on %v and arrived on day %d: out of order", name, seed, id, last, rs, day)
				}
				arrived++
			}
		}
	}
	if arrived == 0 {
		t.Fatal("no faction arrived away from home in twelve Bayport-heavy runs: the test reads nothing")
	}
	t.Logf("%d arrivals away from home, every one telegraphed", arrived)
}

// TestRichCityDrawsAFaction (#341): the distributor, who moves to the
// hub and earns there, sees a faction's scouts before day 120 on at
// least 80% of the seeds (17 of 20 at the ruling), and one arrives on
// most of them. Two of the seeds it misses are the ones its trade failed
// on (net worth under $1M at the horizon), which never cross the line.
func TestRichCityDrawsAFaction(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	const seeds, by = 20, 120
	drawn, arrived := 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		x := playExpansion(t, cfg, seed, Distributor(cfg, 40))
		first := 0
		for _, ss := range x.scouts {
			if first == 0 || ss[0] < first {
				first = ss[0]
			}
		}
		if first > 0 && first < by {
			drawn++
		}
		if len(x.arrivals) > 0 {
			arrived++
		}
		t.Logf("seed %d: scouts first on day %d, %d arrival(s) away, net worth %d at day %d", seed, first, len(x.arrivals), x.res.NetWorthAt(Horizon), Horizon)
	}
	t.Logf("scouts before day %d on %d of %d seeds; an away faction arrived on %d", by, drawn, seeds, arrived)
	if drawn*10 < seeds*8 {
		t.Fatalf("scouts before day %d on %d of %d seeds, want at least 80%%", by, drawn, seeds)
	}
}

// TestScoutsLeaveWhenTheMoneyDoes (#341): the distributor stops selling
// in the city a faction is scouting the morning it sees the scouts; the
// window falls under take_min before the recruiting, and every one of
// those moves ends in RivalWithdrew with no recruiting and no arrival,
// the seat back in the wings at home.
func TestScoutsLeaveWhenTheMoneyDoes(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	moves := 0
	for seed := uint64(1); seed <= 6; seed++ {
		dist := Distributor(cfg, 40)
		policy := func(w *game.World) {
			dist(w)
			for _, r := range w.Rivals {
				if r == nil || !r.Scouting() {
					continue
				}
				for _, p := range w.Products {
					w.CancelSell(r.ScoutingCity, p)
					w.CancelStanding(r.ScoutingCity, p)
				}
			}
		}
		x := playExpansion(t, cfg, seed, policy)
		for id, ss := range x.scouts {
			moves += len(ss)
			if len(x.recruits[id]) > 0 || x.arrivals[id] > 0 {
				t.Fatalf("seed %d: %s recruited on %v and arrived on day %d with the take gone", seed, id, x.recruits[id], x.arrivals[id])
			}
			// The distributor sells there again once they are gone, so
			// they come back and go home again: every move but one still
			// under way at the horizon ends in a withdrawal.
			open := len(ss) - len(x.withdrew[id])
			if open < 0 || open > 1 || (open == 1 && ss[len(ss)-1] <= Horizon-cfg.Rivals.Expansion.ScoutDays) {
				t.Fatalf("seed %d: %s scouted on %v and went home on %v", seed, id, ss, x.withdrew[id])
			}
		}
	}
	if moves == 0 {
		t.Fatal("no faction scouted in six distributor runs: the test reads nothing")
	}
	t.Logf("%d moves, every one sent home", moves)
}

// The window is the rivals sim's own bookkeeping and never a number a
// policy reads: a home player's run keeps none.
func TestHomePlayerKeepsNoWindow(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 3; seed++ {
		res, _ := RunFrom(cfg, sim.NewWorld(cfg, seed), TierDays[2], Crewed(cfg, 40))
		if res.World.Takes != nil || res.World.Stats.Moves != 0 {
			t.Fatalf("seed %d: the crewed player at home kept a window %v and drew %d moves", seed, res.World.Takes, res.World.Stats.Moves)
		}
	}
}
