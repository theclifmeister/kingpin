package harness

import (
	"fmt"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
)

// The endings (#49, docs/endings.md): one scripted scenario a cause,
// the exit plans in order, the frequencies over fifty seeds, the score
// and that a run reaching none of the new endings is the run before.

// scenario is one cause's script: a world set up for it and the policy
// that plays it, over at most days days.
type scenario struct {
	cause string
	days  int
	cfg   func(*content.Config) *content.Config
	world func(cfg *content.Config, w *game.World)
	play  func(cfg *content.Config) Policy
}

// scenarios is the table: one a cause, each reaching exactly that cause.
// They run on the duel (OneFaction) and without crew life (NoLife) where
// a hand-built table or crew would otherwise be perturbed, the #46 and
// #43 pattern.
func scenarios() []scenario {
	// runners puts n runners of your own on home's first n corners: a
	// hand-built roster, since the pool on day 0 has a face or two.
	runners := func(w *game.World, n int) {
		home := w.Home()
		for i := 0; i < n; i++ {
			id := 900 + i
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: id, Name: fmt.Sprintf("Runner %d", i+1), Role: "runner", Skill: 60, Loyalty: 80, Nerve: 60, Wage: 60})
			if err := w.Post(home.Corners[i].ID, id); err != nil {
				panic(err)
			}
		}
		w.Crew.NextID = 900 + n
	}
	stand := func(w *game.World) *game.Corner { // the corner you stand on at home
		for i := range w.Home().Corners {
			if c := &w.Home().Corners[i]; c.Runner == game.You {
				return c
			}
		}
		return &w.Home().Corners[0]
	}
	return []scenario{
		{content.CauseIndicted, Horizon, nil, nil, func(cfg *content.Config) Policy { return Trader(cfg, events.DialAggressive) }},
		{content.CauseArrested, 30, func(cfg *content.Config) *content.Config {
			c := *cfg
			c.Heat.Heat.Decay = 0 // the heat holds where the script puts it
			return &c
		}, nil, func(cfg *content.Config) Policy {
			return func(w *game.World) {
				for _, c := range w.Cities {
					c.Heat = 100 // the arrest rung, with nothing in the file
				}
			}
		}},
		{content.CauseBroke, 5, nil, func(cfg *content.Config, w *game.World) {
			w.Player.DirtyCash = 0
		}, func(*content.Config) Policy { return Idle }},
		{content.CauseRetired, 5, nil, func(cfg *content.Config, w *game.World) {
			w.Offshore, w.QuietDays = cfg.Laundering.Offshore.RetireCash, cfg.Laundering.Offshore.RetireDays
		}, func(cfg *content.Config) Policy {
			ld := laundering.New(cfg)
			return func(w *game.World) { _ = ld.Retire(w) }
		}},
		{content.CauseBusinessman, 60, nil, func(cfg *content.Config, w *game.World) {
			// Three fronts at the top of their ladders, the city on your
			// side, nothing sold: the levels out-earn a street that
			// makes nothing, thirty days running.
			for _, fc := range cfg.Laundering.Fronts {
				w.Fronts = append(w.Fronts, game.Front{ID: fc.ID, Name: fc.Name, Cost: fc.Cost, Bought: 1, Level: fc.MaxLevel})
			}
			w.Player.DirtyCash, w.Player.CleanCash = 1_000_000, 1_000_000
			w.Home().Goodwill, w.Home().Pressure = 100, 0
		}, func(*content.Config) Policy { return Idle }},
		{content.CauseKingpin, 60, func(cfg *content.Config) *content.Config { return NoLife(Factions(cfg, 3)) }, func(cfg *content.Config, w *game.World) {
			// Every faction fallen on day 1 (#43's TestDominantScripted
			// with the last leader taken too), and the city held: six
			// of home's ten corners with a runner on each. The reign
			// begins on day 15 (#227) and the crown is taken the same
			// morning (Crowned).
			for _, r := range w.Rivals {
				r.Arrived, r.Fragmented = 1, 1
			}
			w.Player.DirtyCash = 500_000
			runners(w, 6)
		}, func(*content.Config) Policy { return Crowned(Idle) }},
		{content.CauseBetrayed, 5, func(cfg *content.Config) *content.Config { return NoLife(OneFaction(cfg)) }, func(cfg *content.Config, w *game.World) {
			// A lieutenant runs home with every corner you hold there,
			// four of them, and turns tonight: under the flip line, over
			// the quitting one.
			w.Player.DirtyCash = 500_000
			runners(w, 4)
			lt := Delegate(cfg, w, w.Home().ID, "steady")
			w.Crew.Member(lt.ID).Loyalty = cfg.Crew.Lieutenant.Flip - 1
		}, func(*content.Config) Policy { return Idle }},
		{content.CauseTakenOut, 120, func(cfg *content.Config) *content.Config { return NoLife(OneFaction(cfg)) }, func(cfg *content.Config, w *game.World) {
			// You on one corner with no muscle, an expansionist next
			// door with plenty and a war worth the name: the push that
			// lands takes the last corner you hold.
			r := w.Rival()
			r.Arrived, r.Muscle, r.Cash, r.War, r.Personality, r.Observed = 1, 12, 500_000, 60, "expansionist", true
			you := stand(w)
			for i := range w.Home().Corners {
				if c := &w.Home().Corners[i]; c.ID != you.ID && c.Borders(*you) {
					c.Owner, c.Faction, c.Since = game.OwnerRival, r.Faction(), 1
					break
				}
			}
		}, func(cfg *content.Config) Policy {
			return func(w *game.World) { w.Rival().War = max(w.Rival().War, 60) } // the war stays loud
		}},
		{content.CauseVanished, Horizon, nil, func(cfg *content.Config, w *game.World) {
			Own(cfg, w, "lawyer", "retainer", "identity")
		}, func(cfg *content.Config) Policy { return Trader(cfg, events.DialAggressive) }},
	}
}

// TestEveryEndingIsReachable (#49): one scripted scenario a cause
// reaches exactly that cause, every cause has a row in endings.toml,
// the ending is written through World.End (the score stamped), and the
// same script on the same seed ends the same way on the same day.
func TestEveryEndingIsReachable(t *testing.T) {
	t.Parallel()
	base := content.MustLoad()
	seen := map[string]bool{}
	for _, sc := range scenarios() {
		sc := sc
		t.Run(sc.cause, func(t *testing.T) {
			t.Parallel()
			cfg := base
			if sc.cfg != nil {
				cfg = sc.cfg(base)
			}
			play := func() Result {
				w := sim.NewWorld(cfg, 1)
				if sc.world != nil {
					sc.world(cfg, w)
				}
				res, err := RunFrom(cfg, w, sc.days, sc.play(cfg))
				if err != nil {
					t.Fatal(err)
				}
				return res
			}
			res := play()
			if res.Over == nil {
				t.Fatalf("%s: the run was still going on day %d", sc.cause, res.Days)
			}
			if res.Over.Cause != sc.cause {
				t.Fatalf("%s: the run ended %s on day %d", sc.cause, res.Over.Cause, res.Over.Day)
			}
			if base.Endings.Ending(sc.cause) == nil {
				t.Errorf("%s: no row in endings.toml", sc.cause)
			}
			if want := res.World.Offshore / (1 + res.World.Stats.Bodies); res.World.Stats.Score != want {
				t.Errorf("%s: score %d, want the account over one plus the bodies, %d", sc.cause, res.World.Stats.Score, want)
			}
			if again := play(); again.Over == nil || again.Over.Cause != sc.cause || again.Over.Day != res.Over.Day || again.World.Stats.Score != res.World.Stats.Score {
				t.Errorf("%s: the same script ended %+v the second time, %+v the first", sc.cause, again.Over, res.Over)
			}
			t.Logf("%s on day %d (who %q, score %d)", sc.cause, res.Over.Day, res.Over.Who, res.World.Stats.Score)
			seen[sc.cause] = true
		})
	}
	t.Cleanup(func() {
		for _, c := range content.Causes {
			if !seen[c] {
				t.Errorf("no scenario reaches %s", c)
			}
		}
	})
}

// TestExitPlansRunInOrder (#49): on an indictment the fall guy takes
// the first, the new identity turns the next into the vanished ending,
// and the account survives both (TestTheAccountIsSafe pins the fall
// guy's half). The identity alone vanishes on the day the same tree
// without it is indicted; the fall guy alone ends the same way as the
// tree without it, later; the two together vanish on the fall guy's
// day. Every run owns the Legal branch to the retainer (the fall guy
// and the identity hang off it) and the Security branch to the
// safehouse (the fall guy needs it), so the file is the same file.
func TestExitPlansRunInOrder(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	base := []string{"stash", "burners", "lookouts", "safehouse", "lawyer", "retainer"}
	play := func(ids ...string) Result {
		w := sim.NewWorld(cfg, 3)
		w.Offshore = 200_000
		Own(cfg, w, append(append([]string(nil), base...), ids...)...)
		res, err := RunFrom(cfg, w, Horizon, Trader(cfg, events.DialAggressive))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	police := func(cause string) bool { return cause == content.CauseIndicted || cause == content.CauseArrested }
	plain := play()
	if plain.Over == nil || !police(plain.Over.Cause) {
		t.Fatalf("the aggressive trader ended %+v, not by the police", plain.Over)
	}
	alone := play("identity")
	if alone.Over == nil || alone.Over.Cause != content.CauseVanished || alone.Over.Day != plain.Over.Day || alone.World.FallsTaken != 0 {
		t.Errorf("the identity alone ended %+v (%d falls); want vanished on the plain run's day %d", alone.Over, alone.World.FallsTaken, plain.Over.Day)
	}
	fall := play("fallguy")
	if fall.Over == nil || !police(fall.Over.Cause) || fall.World.FallsTaken != 1 || fall.Over.Day <= plain.Over.Day {
		t.Errorf("the fall guy alone ended %+v (%d falls); want the police's ending after the fall, later than day %d", fall.Over, fall.World.FallsTaken, plain.Over.Day)
	}
	both := play("fallguy", "identity")
	if both.Over == nil || both.Over.Cause != content.CauseVanished || both.World.FallsTaken != 1 || both.Over.Day != fall.Over.Day {
		t.Errorf("with a fall guy and an identity the run ended %+v after %d falls; want vanished on the fall guy's day %d", both.Over, both.World.FallsTaken, fall.Over.Day)
	}
	if both.World.Offshore != 200_000 || both.World.Stats.Score != 200_000/(1+both.World.Stats.Bodies) {
		t.Errorf("the account read %d after the fall guy and the vanishing, score %d", both.World.Offshore, both.World.Stats.Score)
	}
	t.Logf("plain %s day %d; identity %s day %d; fall guy %s day %d; both %s day %d", plain.Over.Cause, plain.Over.Day, alone.Over.Cause, alone.Over.Day, fall.Over.Cause, fall.Over.Day, both.Over.Cause, both.Over.Day)
	// Vanish is the player's too, on any morning, with the identity and
	// not without it.
	w := sim.NewWorld(cfg, 3)
	fx := game.FoldEffects(w, cfg.Upgrades)
	if err := w.Vanish(fx); err != game.ErrNoIdentity || w.Over != nil {
		t.Fatalf("vanishing with no identity: %v %+v", err, w.Over)
	}
	Own(cfg, w, "lawyer", "retainer", "identity")
	fx = game.FoldEffects(w, cfg.Upgrades)
	if !w.CanVanish(fx) {
		t.Fatal("the identity owned and Vanish not open")
	}
	if err := w.Vanish(fx); err != nil || w.Over == nil || w.Over.Cause != content.CauseVanished {
		t.Fatalf("vanishing with the identity: %v %+v", err, w.Over)
	}
}

// TestEndingFrequencies (#49): over fifty seeds the aggressive trader
// ends indicted on most, the retiree retired on most
// (TestRetireeRetires has the count), and the quiet, managed and crewed
// players never end kingpin or a businessman: nobody reaches either by
// accident, and a run that ends ends the way it did before.
func TestEndingFrequencies(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for _, c := range []struct {
		name   string
		policy func(*content.Config) Policy
		mostly string
	}{
		{"aggressive", func(cfg *content.Config) Policy { return Trader(cfg, events.DialAggressive) }, content.CauseIndicted},
		{"retiree", func(cfg *content.Config) Policy { return Retiree(cfg, 40) }, content.CauseRetired},
		{"quiet", func(cfg *content.Config) Policy { return Trader(cfg, events.DialQuiet) }, ""},
		{"managed", func(cfg *content.Config) Policy { return Managed(cfg, 50) }, ""},
		{"crewed", func(cfg *content.Config) Policy { return Crewed(cfg, 40) }, ""},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			counts := map[string]int{}
			for seed := uint64(1); seed <= 50; seed++ {
				res, err := Run(cfg, seed, Horizon, c.policy(cfg))
				if err != nil {
					t.Fatal(err)
				}
				if res.Over != nil {
					counts[res.Over.Cause]++
				} else {
					counts["still free"]++
				}
			}
			t.Logf("%s: %v", c.name, counts)
			if c.mostly != "" && counts[c.mostly] <= 25 {
				t.Errorf("%s ended %s on %d of 50 seeds; most should", c.name, c.mostly, counts[c.mostly])
			}
			if n := counts[content.CauseKingpin] + counts[content.CauseBusinessman]; n > 0 {
				t.Errorf("%s ended kingpin or a businessman on %d of 50 seeds by accident: %v", c.name, n, counts)
			}
		})
	}
}

// TestNoEndingIsTheOldRun (#49): a run that reaches none of the new
// endings is byte-for-byte the run before they existed. The boss, the
// laundered and the distributor players are hashed daily on the file
// and on the file with the detectors boxed (harness.NoEndings;
// assertOldRun: one seed to the tier-4 checkpoint), and nothing
// ends either way: the detectors are reads on state the sims already
// had, bar the count of legit days, which is set aside as #195's quiet
// days are (the boss's peaks at 3 of 30 over fifty seeds) and never
// moves with the table boxed.
func TestNoEndingIsTheOldRun(t *testing.T) {
	t.Parallel()
	assertOldRun(t, oldRunCase{
		never: "reached none of the new endings",
		seeds: 1,
		days:  Horizon,
		box:   NoEndings,
		policies: map[string]func(*content.Config) Policy{
			"boss":        func(c *content.Config) Policy { return Boss(c, 40, "") },
			"laundered":   func(c *content.Config) Policy { return Laundered(c, 40) },
			"distributor": func(c *content.Config) Policy { return Distributor(c, 40) },
		},
		scrub: func(t *testing.T, w *game.World, boxed bool) func() {
			// The count of legit days is the one number the table
			// moves in a run that ends nothing (#195's QuietDays
			// pattern): set aside on the file, logged on every day it
			// counts (the peak is read off the log), never counted
			// boxed.
			legit := w.LegitDays
			if boxed && legit != 0 {
				t.Fatalf("day %d: the count of legit days moved to %d with the table boxed", w.Day, legit)
			}
			if legit != 0 {
				t.Logf("day %d: legit days %d of %d", w.Day, legit, content.MustLoad().Laundering.Businessman.LegitDays)
			}
			w.LegitDays = 0
			return func() { w.LegitDays = legit }
		},
		after: func(t *testing.T, w *game.World, _ bool) {
			if w.Over != nil {
				t.Fatalf("the run ended %+v", w.Over)
			}
		},
	})
}

// TestScoreIsTheAccount (#49, #195's ruling): the score is the offshore
// account over one plus the bodies, whatever the pile, the days or the
// cause; sorted by score, the pile left behind never orders two runs.
func TestScoreIsTheAccount(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 1)
	w.Offshore, w.Stats.Bodies, w.Player.CleanCash, w.Player.DirtyCash = 900_000, 2, 50_000_000, 10_000_000
	if got := w.Score(); got != 300_000 {
		t.Fatalf("score %d, want 900000 / 3", got)
	}
	w.Stats.Bodies = 0
	e := w.End(content.CauseRetired, 40, "")
	if e.Cause != content.CauseRetired || e.Day != 40 || w.Stats.Score != 900_000 {
		t.Fatalf("End wrote %+v, score %d", e, w.Stats.Score)
	}
	scores := []int{}
	for _, bodies := range []int{0, 1, 4} {
		w.Stats.Bodies = bodies
		scores = append(scores, w.Score())
	}
	if !sort.SliceIsSorted(scores, func(i, j int) bool { return scores[i] > scores[j] }) {
		t.Errorf("the bodies do not divide the score: %v", scores)
	}
}

// Crowned wraps a policy with the crown (#227): the morning the reign
// is on (World.CanCrown) it takes it, ending the run a kingpin the way
// the detector did before the reign was the player's to live.
func Crowned(policy Policy) Policy {
	return func(w *game.World) {
		if w.CanCrown() {
			_ = w.Crown()
			return
		}
		policy(w)
	}
}

// TestCrownIsTheScoreAsItStands (#227): on the kingpin scenario, taking
// the crown k days after the reign begins never raises the score (the
// account over one plus the bodies, as TestRetireeRetires pins for the
// retiree); the reign is lived, and the summary's day is the crown's.
func TestCrownIsTheScoreAsItStands(t *testing.T) {
	t.Parallel()
	var sc scenario
	for _, s := range scenarios() {
		if s.cause == content.CauseKingpin {
			sc = s
		}
	}
	cfg := sc.cfg(content.MustLoad())
	var first int
	for _, k := range []int{0, 10, 30} {
		w := sim.NewWorld(cfg, 1)
		sc.world(cfg, w)
		w.Offshore = 1_200_000
		waited := 0
		res, err := RunFrom(cfg, w, 120, func(w *game.World) {
			if w.CanCrown() {
				if waited < k {
					waited++
					return
				}
				_ = w.Crown()
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Over == nil || res.Over.Cause != content.CauseKingpin {
			t.Fatalf("k=%d: the run ended %+v", k, res.Over)
		}
		if k == 0 {
			first = res.World.Stats.Score
		}
		if res.World.Reign == 0 || res.Over.Day != res.World.Reign+k || res.World.ReignDay() != k+1 {
			t.Fatalf("k=%d: crowned on day %d with the reign from day %d (day %d of it)", k, res.Over.Day, res.World.Reign, res.World.ReignDay())
		}
		if res.World.Stats.Score > first {
			t.Errorf("k=%d: score %d, over the crown taken at once (%d): playing the reign on paid", k, res.World.Stats.Score, first)
		}
		t.Logf("k=%d: crowned on day %d, day %d of the reign, score %d", k, res.Over.Day, res.World.ReignDay(), res.World.Stats.Score)
	}
}
