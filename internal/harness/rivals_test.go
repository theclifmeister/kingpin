package harness

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// A passive player who holds corners and never sends enforcers loses
// ground to an expansionist rival within 60 days: the rival is pressure,
// not scenery.
func TestPassivePlayerLosesCornersToExpansionist(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Rival.Personality = "expansionist"
		res, err := RunFrom(cfg, w, 60, Territory(cfg, 40, 3))
		if err != nil {
			t.Fatal(err)
		}
		if res.World.Stats.Strikes != 0 {
			t.Fatalf("seed %d: the passive player sent enforcers", seed)
		}
		lost := 0
		for _, e := range res.Events {
			if ct, ok := e.(events.CornerTaken); ok && ct.From == game.OwnerPlayer {
				lost++
			}
		}
		if lost == 0 {
			t.Fatalf("seed %d: held %d corners for 60 days next to an expansionist and lost none (rival holds %d)", seed, res.World.Held(), res.World.RivalHeld())
		}
	}
}

// An all-out hit war draws heat faster than always-aggressive selling:
// violence is the loudest thing you can do. The warmonger starts with
// three enforcers and a rival dug in on four corners, and never sells;
// the seller never strikes.
func TestHitWarHeatsFasterThanAggressiveSelling(t *testing.T) {
	cfg := content.MustLoad()
	const days = 10
	for seed := uint64(1); seed <= 5; seed++ {
		w := sim.NewWorld(cfg, seed)
		w.Player.DirtyCash = 20_000
		for i := 0; i < 3; i++ {
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 100 + i, Name: fmt.Sprintf("E%d", i), Role: "enforcer", Skill: 50, Loyalty: 80, Nerve: 80, Wage: 55})
		}
		w.Crew.NextID = 103
		w.Rival.Arrived = 1
		w.Rival.Muscle = 6
		for _, id := range []string{"docks", "railyard", "oldmill", "depot"} {
			c := w.Corner(id)
			c.Owner, c.Since = game.OwnerRival, 0
		}
		war, err := RunFrom(cfg, w, days, func(w *game.World) {
			if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, func(c game.Corner) float64 { return c.Demand }); c != nil {
				if err := w.SendEnforcers(c.ID, events.ForceHit); err != nil {
					t.Fatal(err)
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		// A hit war rarely lasts the window: the crackdown ends it.
		if war.World.Stats.Strikes < 3 {
			t.Fatalf("seed %d: only %d strikes in %d days; the rival never showed", seed, war.World.Stats.Strikes, days)
		}
		// Faster means fewer days to the patrol line.
		line := cfg.Heat.Responses[0].Threshold
		sell, _ := Run(cfg, seed, 3*days, Trader(cfg, events.DialAggressive))
		warDay, sellDay := daysToHeat(war, line), daysToHeat(sell, line)
		t.Logf("seed %d: %d hits reached heat %.0f on day %d; aggressive selling on day %d", seed, war.World.Stats.Strikes, line, warDay, sellDay)
		if warDay == 0 || (sellDay != 0 && warDay >= sellDay) {
			t.Fatalf("seed %d: hits reached heat %.0f on day %d, aggressive selling on day %d; a war should be louder", seed, line, warDay, sellDay)
		}
		loud := false
		for _, e := range war.Events {
			if we, ok := e.(events.WarEscalated); ok && we.Stage == "open" {
				loud = true
			}
		}
		if !loud {
			t.Fatalf("seed %d: %d hits and the war never got loud", seed, war.World.Stats.Strikes)
		}
	}
}

// The rival's books and the map stay sane through peace and war: its cash
// and muscle never go negative, it never holds more corners than exist,
// every corner has exactly one owner, and nobody stands on ground that is
// not the player's.
func TestRivalInvariants(t *testing.T) {
	cfg := content.MustLoad()
	policies := map[string]func(*content.Config) Policy{
		"territory": func(cfg *content.Config) Policy { return Territory(cfg, 40, 4) },
		"war-push":  func(cfg *content.Config) Policy { return Warlike(cfg, 40, 4, events.ForcePush) },
		"war-hit":   func(cfg *content.Config) Policy { return Warlike(cfg, 60, 4, events.ForceHit) },
	}
	for name, mk := range policies {
		for seed := uint64(1); seed <= 5; seed++ {
			policy := mk(cfg)
			check := func(w *game.World) {
				r := w.Rival
				if r.Cash < 0 || r.Muscle < 0 {
					t.Fatalf("%s seed %d day %d: rival cash %d muscle %d", name, seed, w.Day, r.Cash, r.Muscle)
				}
				if r.War < 0 || r.War > 100 {
					t.Fatalf("%s seed %d day %d: war %.1f", name, seed, w.Day, r.War)
				}
				if n := w.RivalHeld(); n > len(w.Territory.Corners) || n+w.Held() > len(w.Territory.Corners) {
					t.Fatalf("%s seed %d day %d: rival holds %d, you %d of %d corners", name, seed, w.Day, n, w.Held(), len(w.Territory.Corners))
				}
				for _, c := range w.Territory.Corners {
					switch c.Owner {
					case game.OwnerNone, game.OwnerPlayer, game.OwnerRival:
					default:
						t.Fatalf("%s seed %d day %d: %s owned by %q", name, seed, w.Day, c.ID, c.Owner)
					}
					if c.Owner != game.OwnerPlayer && (c.Runner != 0 || c.Enforcer != 0) {
						t.Fatalf("%s seed %d day %d: %s is %s's but %d/%d stand on it", name, seed, w.Day, c.ID, c.Owner, c.Runner, c.Enforcer)
					}
					if c.Squeeze < 0 || c.Squeeze >= 1 {
						t.Fatalf("%s seed %d day %d: %s squeeze %.2f", name, seed, w.Day, c.ID, c.Squeeze)
					}
					if c.Squeeze > 0 && !w.Contested(c) {
						t.Fatalf("%s seed %d day %d: %s squeezed but not contested", name, seed, w.Day, c.ID)
					}
				}
			}
			res, err := Run(cfg, seed, 300, func(w *game.World) {
				check(w)
				policy(w)
			})
			if err != nil {
				t.Fatal(err)
			}
			check(res.World)
			if res.World.Rival.Leader == "" || res.World.Rival.Arrived == 0 {
				t.Fatalf("%s seed %d: rival %+v never arrived", name, seed, res.World.Rival)
			}
		}
	}
}

// The rival steps from the tick RNG: a run at war must replay exactly.
func TestRivalIsDeterministic(t *testing.T) {
	cfg := content.MustLoad()
	a, _ := Run(cfg, 9, 150, Warlike(cfg, 40, 4, events.ForcePush))
	b, _ := Run(cfg, 9, 150, Warlike(cfg, 40, 4, events.ForcePush))
	if len(a.Events) != len(b.Events) || a.PeakCash != b.PeakCash {
		t.Fatalf("runs diverged: %d/%d events, peak %d/%d", len(a.Events), len(b.Events), a.PeakCash, b.PeakCash)
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	if a.World.Rival != b.World.Rival {
		t.Fatalf("rival state differs: %+v vs %+v", a.World.Rival, b.World.Rival)
	}
	if a.World.Stats.Strikes == 0 {
		t.Fatal("the warlike player never struck")
	}
}

// Sending enforcers takes ground and the rival pays it back: over a long
// push war the player wins corners, gets tipped to the police, and a loud
// enough war ends in a crackdown that clears both sides.
func TestWarTakesGroundAndTheRivalTipsPolice(t *testing.T) {
	cfg := content.MustLoad()
	won, tips, crackdowns, cleared := 0, 0, 0, 0
	for seed := uint64(1); seed <= 10; seed++ {
		res, _ := Run(cfg, seed, Horizon, Warlike(cfg, 60, 4, events.ForcePush))
		won += res.World.Stats.CornersWon
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.RivalTippedPolice:
				tips++
			case events.WarEscalated:
				if ev.Stage == "crackdown" {
					crackdowns++
					if len(ev.Lost) == 0 {
						t.Fatalf("seed %d day %d: a crackdown that cleared nothing", seed, ev.Day)
					}
					cleared += len(ev.Lost)
				}
			case events.CornerStruck:
				if ev.Heat <= 0 {
					t.Fatalf("seed %d day %d: a strike with no heat: %+v", seed, ev.Day, ev)
				}
			}
		}
	}
	if won == 0 || tips == 0 || crackdowns == 0 {
		t.Fatalf("over 10 push wars: %d corners won, %d tips, %d crackdowns; the war is missing a piece", won, tips, crackdowns)
	}
	t.Logf("10 push wars: %d corners won, %d tips, %d crackdowns clearing %d corners", won, tips, crackdowns, cleared)
}

// daysToHeat is the first day a run's heat reached v, or 0 if it never did.
func daysToHeat(r Result, v float64) int {
	for _, e := range r.Events {
		if hc, ok := e.(events.HeatChanged); ok && hc.To >= v {
			return hc.Day
		}
	}
	return 0
}
