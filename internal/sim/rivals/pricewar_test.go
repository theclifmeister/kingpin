package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// warWorld is world with the rival dug in on The Docks, next to Fourth
// & Main where you stand, and rich enough that nothing it does waits on
// cash. The market sim is not on the clock: the squeeze the price war
// leaves on the corner is written by hand each morning, as the market
// would, and the rival's answer is what is under test.
func warWorld(t *testing.T, cfg *content.Config, seed uint64, personality string) (*game.World, *rivals.Sim) {
	t.Helper()
	w, s := world(t, cfg, seed)
	w.Rival.Personality = personality
	w.Rival.Arrived, w.Rival.Cash, w.Rival.Muscle = 1, 10_000_000, 6
	w.Corner("docks").Owner = game.OwnerRival
	w.Corner("docks").Since = 1
	w.Day = 1
	return w, s
}

// squeeze is what the market leaves on a rival corner after a night's
// undercut: the share of its trade taken.
func squeeze(w *game.World, id string, share float64) { w.Corner(id).Squeeze = share }

// The rival's own undercutting leaves the squeeze on its corners alone
// (#68: that is the market's, the player's price war), and clears it
// on every corner that is not its own.
func TestRivalUndercutLeavesItsOwnSqueezeAlone(t *testing.T) {
	cfg := content.MustLoad()
	w, s := warWorld(t, cfg, 21, "defensive")
	squeeze(w, "docks", 0.2)
	w.Corner("oldmill").Squeeze = 0.3 // a free corner with a stale squeeze
	step(w, s)
	if w.Corner("docks").Squeeze != 0.2 {
		t.Fatalf("the rival step touched its own corner's squeeze: %.2f", w.Corner("docks").Squeeze)
	}
	if w.Corner("oldmill").Squeeze != 0 {
		t.Fatalf("the rival step left a free corner squeezed: %.2f", w.Corner("oldmill").Squeeze)
	}
	if w.Corner("fourth").Squeeze <= 0 {
		t.Fatalf("your contested corner is not undercut: %.2f", w.Corner("fourth").Squeeze)
	}
}

// A squeezed day is a day starved and costs war, once for the day
// whatever the corners, and no grudge until a corner is starved to the
// line; a rest of more than pricewar_days forgets the count and a
// shorter one keeps it, so a war fought every other day still reaches
// the line, later; an unsqueezed corner counts nothing.
func TestStarvedDaysAndTheCost(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Rivals.Pricewar
	w, s := warWorld(t, cfg, 22, "defensive")
	w.Corner("heights").Owner = game.OwnerRival
	war, grudge := w.Rival.War, w.Rival.Grudge
	squeeze(w, "docks", 0.2)
	squeeze(w, "heights", 0.2)
	tips := kinds(step(w, s))["RivalTippedPolice"]
	docks := w.Corner("docks")
	if docks.Starved != 1 || docks.StarvedDay != w.Day || w.Corner("heights").Starved != 1 {
		t.Fatalf("after one squeezed day: %+v", *docks)
	}
	// The war fades by war_decay at the end of the step; no grudge yet.
	if want := (war + tun.War) * (1 - cfg.Rivals.Rivals.WarDecay); w.Rival.War < want-1e-9 || w.Rival.Grudge+tips != grudge {
		t.Fatalf("war %.1f -> %.1f, grudge %d -> %d with %d tips: want +%.0f and no grudge", war, w.Rival.War, grudge, w.Rival.Grudge, tips, tun.War)
	}
	// Every other day: the count climbs by one every two days.
	for i := 0; i < 2*tun.PricewarDays-3; i++ {
		if i%2 == 0 {
			squeeze(w, "docks", 0)
		} else {
			squeeze(w, "docks", 0.2)
		}
		step(w, s)
	}
	if docks.Starved != tun.PricewarDays-1 {
		t.Fatalf("fought every other day for %d days: starved %d, want %d", 2*tun.PricewarDays-3, docks.Starved, tun.PricewarDays-1)
	}
	// A rest of more than pricewar_days days forgets it.
	squeeze(w, "docks", 0)
	for i := 0; i <= tun.PricewarDays; i++ {
		step(w, s)
	}
	squeeze(w, "docks", 0.2)
	step(w, s)
	if docks.Starved != 1 {
		t.Fatalf("after a rest of %d days: starved %d, want 1", tun.PricewarDays+1, docks.Starved)
	}
}

// The answer, by personality, once a corner is starved pricewar_days
// days: an opportunist gives it up (RivalAbandoned, the corner free and
// unsqueezed), a defensive rival never does and pushes back on the
// corner doing the cutting on some seed, an expansionist likewise, and
// a chaotic one does both across seeds. A push is the usual path
// (RivalPushed or CornerTaken, marked Pricewar), and a corner given up
// starts the count over.
func TestPricewarAnswerByPersonality(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Rivals.Pricewar
	run := func(personality string, seed uint64, days int) map[string]int {
		w, s := warWorld(t, cfg, seed, personality)
		w.Rival.Grudge = 0
		got := map[string]int{}
		for i := 0; i < days; i++ {
			if w.Corner("docks").Owner == game.OwnerRival {
				squeeze(w, "docks", 0.2)
			}
			grudge := w.Rival.Grudge
			evs := step(w, s)
			if w.Rival.Grudge+kinds(evs)["RivalTippedPolice"] > grudge {
				got["grudge"]++
			}
			for _, e := range evs {
				switch ev := e.(type) {
				case events.RivalAbandoned:
					if ev.Corner != "docks" || ev.Reason != "pricewar" {
						t.Fatalf("%s seed %d: abandoned %+v", personality, seed, ev)
					}
					if c := w.Corner("docks"); c.Owner != game.OwnerNone || c.Squeeze != 0 || c.Starved != 0 {
						t.Fatalf("%s seed %d: the corner given up: %+v", personality, seed, *c)
					}
					got["abandon"]++
				case events.RivalPushed:
					if ev.Pricewar {
						if ev.Corner != "fourth" {
							t.Fatalf("%s seed %d: pushed on %s over the price war, want the corner doing the cutting", personality, seed, ev.Corner)
						}
						got["push"]++
					}
				case events.CornerTaken:
					if ev.Pricewar {
						got["push"]++
						got["taken"]++
					}
				}
			}
			if w.Corner("fourth").Owner == game.OwnerRival {
				break // the war is over: your corner is theirs
			}
		}
		return got
	}
	// The opportunist gives up on the day the line is reached, with no
	// dice, and holds the grudge for it.
	if got := run("opportunist", 31, tun.PricewarDays); got["abandon"] != 1 || got["grudge"] != 1 {
		t.Fatalf("opportunist: %v after %d days, want one corner given up and a grudge", got, tun.PricewarDays)
	}
	for _, p := range []string{"defensive", "expansionist", "chaotic"} {
		abandons, pushes := 0, 0
		for seed := uint64(31); seed <= 40; seed++ {
			got := run(p, seed, 200)
			abandons += got["abandon"]
			pushes += got["push"]
		}
		switch p {
		case "defensive":
			if abandons != 0 || pushes == 0 {
				t.Fatalf("defensive: %d corners given up, %d pushes over 10 seeds; want none and some", abandons, pushes)
			}
		case "expansionist":
			if abandons != 0 || pushes == 0 {
				t.Fatalf("expansionist: %d corners given up, %d pushes over 10 seeds; want none and some", abandons, pushes)
			}
		case "chaotic":
			if abandons == 0 || pushes == 0 {
				t.Fatalf("chaotic: %d corners given up, %d pushes over 10 seeds; want both", abandons, pushes)
			}
		}
	}
}

// CornerIncome is what a corner earns the rival with no squeeze on it,
// and Income is that less the squeeze: what the picker shows as the
// loss is what the books lose.
func TestCornerIncomeAndTheSqueeze(t *testing.T) {
	cfg := content.MustLoad()
	w, s := warWorld(t, cfg, 23, "defensive")
	docks := w.Corner("docks")
	whole := s.CornerIncome(w, *docks)
	if whole <= 0 || s.Income(w) != whole {
		t.Fatalf("income %d, the one corner's %d", s.Income(w), whole)
	}
	squeeze(w, "docks", 0.25)
	if got, want := s.Income(w), int(float64(whole)*0.75); got < want-1 || got > want+1 {
		t.Fatalf("income squeezed a quarter: %d, want ~%d", got, want)
	}
	if s.CornerIncome(w, *docks) != whole {
		t.Fatalf("CornerIncome moved with the squeeze: %d", s.CornerIncome(w, *docks))
	}
}
