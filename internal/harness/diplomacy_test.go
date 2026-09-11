package harness

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// contestedWorld is an expansionist dug in next to the player's corners
// with muscle to spare, the day after a truce of days days was struck.
func contestedWorld(t *testing.T, cfg *content.Config, seed uint64, days int) *game.World {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	w.Rival.Personality = "expansionist"
	w.Rival.Arrived, w.Rival.Muscle, w.Rival.Cash, w.Rival.Grudge = 1, 8, 50_000, 3
	for _, id := range []string{"railyard", "depot", "strip"} {
		c := w.Corner(id)
		c.Owner, c.Since = game.OwnerRival, 0
	}
	w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: days}, Since: 0, Until: days}}
	return w
}

// Under a live truce the rival contests no corner and tips no police for
// its duration; the day after it runs out, contesting resumes.
func TestTruceHoldsThenLapses(t *testing.T) {
	cfg := content.MustLoad()
	const truce = 20
	resumed := 0
	for seed := uint64(1); seed <= 5; seed++ {
		w := contestedWorld(t, cfg, seed, truce)
		res, err := RunFrom(cfg, w, 80, Territory(cfg, 40, 4))
		if err != nil {
			t.Fatal(err)
		}
		ended := 0
		for _, e := range res.Events {
			day, hostile := 0, false
			switch ev := e.(type) {
			case events.RivalPushed:
				day, hostile = ev.Day, true
			case events.CornerTaken:
				day, hostile = ev.Day, ev.From == game.OwnerPlayer
			case events.RivalTippedPolice:
				day, hostile = ev.Day, true
			case events.RivalUndercut:
				day, hostile = ev.Day, true
			case events.DealEnded:
				ended = ev.Day
			}
			if !hostile {
				continue
			}
			if day < truce {
				t.Fatalf("seed %d day %d: %+v under a truce that runs to day %d", seed, day, e, truce)
			}
			resumed++
		}
		if ended != truce-1 {
			t.Fatalf("seed %d: the truce ended on day %d, want %d (the night before day %d)", seed, ended, truce-1, truce)
		}
		if res.World.Player.Reputation.Respect <= 0 {
			t.Fatalf("seed %d: a truce kept for %d days earned no respect", seed, truce)
		}
	}
	if resumed == 0 {
		t.Fatal("the rival never came back after the truce ran out")
	}
}

// Buying peace: against an expansionist, the diplomat holds more ground
// at day 120 than the passive player, wins fewer corners than a hit war
// and runs cooler than it.
func TestDiplomatHoldsMoreThanPassive(t *testing.T) {
	cfg := content.MustLoad()
	const days = 120
	held := map[string]int{}
	won := map[string]int{}
	heat := map[string]float64{}
	deals := 0
	policies := map[string]func() Policy{
		"territory": func() Policy { return Territory(cfg, 40, 4) },
		"diplomat":  func() Policy { return Diplomat(cfg, 40, 4) },
		"war":       func() Policy { return Warlike(cfg, 60, 4, events.ForceHit) },
	}
	for name, mk := range policies {
		for seed := uint64(1); seed <= 8; seed++ {
			w := sim.NewWorld(cfg, seed)
			w.Rival.Personality = "expansionist"
			res, err := RunFrom(cfg, w, days, mk())
			if err != nil {
				t.Fatal(err)
			}
			held[name] += res.World.Held()
			won[name] += res.World.Stats.CornersWon
			heat[name] += res.World.Heat.Peak
			if name == "diplomat" {
				deals += res.World.Stats.Deals
				if res.World.Stats.Strikes > 0 {
					t.Fatalf("seed %d: the diplomat sent the enforcers", seed)
				}
			}
		}
	}
	t.Logf("held at day %d over 8 seeds: %v; corners won: %v; peak heat: %v; %d deals struck", days, held, won, heat, deals)
	if deals == 0 {
		t.Fatal("the diplomat never struck a deal")
	}
	if held["diplomat"] <= held["territory"] {
		t.Fatalf("the diplomat held %d corners to the passive player's %d; buying peace should keep ground", held["diplomat"], held["territory"])
	}
	if won["diplomat"] >= won["war"] {
		t.Fatalf("the diplomat won %d corners to the war's %d", won["diplomat"], won["war"])
	}
	if heat["diplomat"] >= heat["war"] {
		t.Fatalf("the diplomat's peak heat %.0f is not under the war's %.0f", heat["diplomat"], heat["war"])
	}
}

// talker plays like Territory and asks for a short truce every night it
// has none, taking every offer: the player who keeps the table busy.
func talker(cfg *content.Config) Policy {
	territory := Territory(cfg, 40, 4)
	dip := cfg.Rivals.Diplomacy
	return func(w *game.World) {
		territory(w)
		for _, o := range w.Offers {
			_, _ = w.Accept(o.ID)
		}
		if w.Rival.Arrived > 0 && w.Deal(game.DealTruce) == nil && w.Proposal == nil {
			_ = w.Propose(game.DealTruce, game.Terms{Days: dip.TruceDays[0]})
		}
	}
}

// A defensive rival never breaks a deal, on any seed, however long the
// peace runs; a chaotic one does.
func TestDefensiveNeverBetrays(t *testing.T) {
	cfg := content.MustLoad()
	broken := map[string]int{}
	struck := map[string]int{}
	for _, p := range []string{"defensive", "chaotic"} {
		for seed := uint64(1); seed <= 8; seed++ {
			w := sim.NewWorld(cfg, seed)
			w.Rival.Personality = p
			res, err := RunFrom(cfg, w, Horizon, talker(cfg))
			if err != nil {
				t.Fatal(err)
			}
			struck[p] += res.World.Stats.Deals
			for _, e := range res.Events {
				if db, ok := e.(events.DealBroken); ok && db.By == "rival" {
					broken[p]++
					if p == "defensive" {
						t.Fatalf("%s seed %d day %d: %+v", p, seed, db.Day, db)
					}
				}
			}
		}
	}
	t.Logf("deals struck %v, broken by the rival %v", struck, broken)
	if struck["defensive"] < 8 || broken["chaotic"] == 0 {
		t.Fatalf("%d deals with a defensive rival and %d broken by a chaotic one; the table test tested nothing", struck["defensive"], broken["chaotic"])
	}
}

// Deals, trust and offers survive a save, and a run with proposals on
// fixed days replays exactly.
func TestDiplomacyIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	dip := cfg.Rivals.Diplomacy
	policy := func() Policy {
		territory := Territory(cfg, 40, 4)
		return func(w *game.World) {
			territory(w)
			switch {
			case w.Day%9 == 3 && w.Deal(game.DealTruce) == nil:
				_ = w.Propose(game.DealTruce, game.Terms{Days: dip.TruceDays[0]})
			case w.Day%9 == 6 && w.Deal(game.DealSplit) == nil:
				_ = w.Propose(game.DealSplit, game.Terms{Corners: w.SplitLines()[0]})
			}
			for _, o := range w.Offers {
				_, _ = w.Accept(o.ID)
			}
		}
	}
	a, err := Run(cfg, 5, 120, policy())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Run(cfg, 5, 120, policy())
	if len(a.Events) != len(b.Events) {
		t.Fatalf("runs diverged: %d vs %d events", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	if a.World.Stats.Deals == 0 {
		t.Fatal("no deal was struck in 120 days of asking")
	}

	// Save mid-run, reload, and play on: the same events as the straight run.
	c, _ := Run(cfg, 5, 60, policy())
	if c.World.Rival.Trust == 0 {
		t.Fatal("trust is zero at day 60")
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
	if fmt.Sprintf("%+v", loaded.Rival) != fmt.Sprintf("%+v", c.World.Rival) || fmt.Sprintf("%+v", loaded.Offers) != fmt.Sprintf("%+v", c.World.Offers) {
		t.Fatalf("the save lost the table:\n%+v %+v\n%+v %+v", c.World.Rival, c.World.Offers, loaded.Rival, loaded.Offers)
	}
	d, _ := RunFrom(cfg, loaded, 60, policy())
	if got, want := len(c.Events)+len(d.Events), len(a.Events); got != want {
		t.Fatalf("save and continue: %d events, straight run %d", got, want)
	}
	for i, e := range d.Events {
		if fmt.Sprintf("%#v", e) != fmt.Sprintf("%#v", a.Events[len(c.Events)+i]) {
			t.Fatalf("event %d after reload differs:\n%#v\n%#v", i, e, a.Events[len(c.Events)+i])
		}
	}
}

// The corner invariants hold through a split: every corner has one
// owner, nobody stands on ground that is not the player's, the rival
// never claims or pushes on the player's side of the line while the
// split holds, and the player is never posted past it.
func TestSplitKeepsTheLine(t *testing.T) {
	cfg := content.MustLoad()
	splits := 0
	for seed := uint64(1); seed <= 6; seed++ {
		territory := Territory(cfg, 40, 5)
		var line []string
		res, err := Run(cfg, seed, Horizon, func(w *game.World) {
			territory(w)
			d := w.Deal(game.DealSplit)
			if d == nil && w.Rival.Arrived > 0 && w.Proposal == nil {
				_ = w.Propose(game.DealSplit, game.Terms{Corners: w.SplitLines()[1]})
			}
			if d == nil {
				line = nil
				return
			}
			line = d.Terms.Corners
			for _, c := range w.Home().Corners {
				if c.Owner == game.OwnerRival && d.Covers(c.ID) && c.Since > d.Since {
					t.Fatalf("seed %d day %d: the rival took %s on your side of the line", seed, w.Day, c.ID)
				}
				if c.Held() && !d.Covers(c.ID) && c.Since > d.Since {
					t.Fatalf("seed %d day %d: you hold %s past the line", seed, w.Day, c.ID)
				}
				if c.Owner != game.OwnerPlayer && (c.Runner != 0 || c.Enforcer != 0) {
					t.Fatalf("seed %d day %d: %s is %s's with people on it", seed, w.Day, c.ID, c.Owner)
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if line != nil {
			splits++
		}
		if res.World.Stats.Betrayals > 0 {
			t.Fatalf("seed %d: the policy betrayed a split it never acted against", seed)
		}
	}
	if splits == 0 {
		t.Fatal("no split ever held to the horizon")
	}
}
