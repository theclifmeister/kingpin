package harness

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// Veterans and the captain (#346).

// TestTraitsFoldTheirWord: every trait in crew.toml moves the number
// each of its words names, for the member who has it and by what the
// file says, and moves at least one: loyalty_loss_mul the night's drift
// (a danger night, so it is a loss), skim_chance_mul their roll to skim,
// deterrence another member's roll, sharp and heat the city's
// sloppiness (the heat sim's), buyer_gap_mul the buyers' gaps (the
// market sim's). The zero trait is every old number.
func TestTraitsFoldTheirWord(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	build := func(trait string) *game.World {
		w := sim.NewWorld(cfg, 1)
		home := w.Home()
		c := &home.Corners[0]
		c.Hand(game.OwnerPlayer, "", w.Day)
		w.Crew.Members = []game.CrewMember{
			{ID: 1, Name: "Vet", Role: game.RoleRunner, Skill: 20, Loyalty: 50, Greed: 50, Nerve: 20, Wage: 50, Units: 40, Trait: trait},
			{ID: 2, Name: "Kid", Role: game.RoleRunner, Skill: 60, Loyalty: 50, Greed: 50, Nerve: 20, Wage: 50, Units: 40},
		}
		c.Runner = 1
		w.Heat.LastResponse = map[string]int{content.Sting: w.Day} // a danger night: the drift is a loss
		return w
	}
	base := build("")
	home := base.Home().ID
	drift := set.Crew.Drift(base, base.Crew.Members[0])
	skim := set.Crew.SkimOdds(base, base.Crew.Members[0])
	other := set.Crew.SkimOdds(base, base.Crew.Members[1])
	sloppy := set.Heat.Sloppiness(base, home)
	lo, hi := set.Market.Gaps(base)
	if drift >= 0 || skim <= 0 || sloppy <= 0 {
		t.Fatalf("the base does not read: drift %v, skim %v, sloppiness %v", drift, skim, sloppy)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	for _, name := range cfg.Crew.TraitNames() {
		tr := cfg.Crew.Trait[name]
		w := build(name)
		vet, kid := w.Crew.Members[0], w.Crew.Members[1]
		moved := 0
		if got := set.Crew.Drift(w, vet); !near(got, drift*tr.LossMul()) {
			t.Errorf("%s: drift %v, want %v x loyalty_loss_mul %v", name, got, drift, tr.LossMul())
		} else if tr.LoyaltyLossMul != 0 {
			moved++
		}
		deter := math.Pow(1-cfg.Crew.Role[game.RoleEnforcer].Deterrence, tr.Deterrence)
		if got := set.Crew.SkimOdds(w, vet); !near(got, skim*tr.SkimMul()*deter) {
			t.Errorf("%s: their skim odds %v, want %v x skim_chance_mul %v x deterrence", name, got, skim, tr.SkimMul())
		} else if tr.SkimChanceMul != 0 {
			moved++
		}
		if got := set.Crew.SkimOdds(w, kid); !near(got, other*deter) {
			t.Errorf("%s: another's skim odds %v, want %v x %v", name, got, other, deter)
		} else if tr.Deterrence != 0 {
			moved++
		}
		got := set.Heat.Sloppiness(w, home)
		switch {
		case tr.Sharp && tr.Heat == 0 && got != 0:
			t.Errorf("%s: a sharp runner's corner is %v sloppy", name, got)
		case tr.Heat > 0 && !(got > sloppy):
			t.Errorf("%s: heat %v left the sloppiness at %v (was %v)", name, tr.Heat, got, sloppy)
		case !tr.Sharp && tr.Heat == 0 && got != sloppy:
			t.Errorf("%s: moved the sloppiness %v to %v with neither word", name, sloppy, got)
		case tr.Sharp || tr.Heat > 0:
			moved++
		}
		l, h := set.Market.Gaps(w)
		switch {
		case tr.BuyerGapMul != 0 && tr.BuyerGapMul < 1 && !(l+h < lo+hi):
			t.Errorf("%s: buyer_gap_mul %v left the gaps at %d..%d (was %d..%d)", name, tr.BuyerGapMul, l, h, lo, hi)
		case tr.BuyerGapMul == 0 && (l != lo || h != hi):
			t.Errorf("%s: moved the gaps with no buyer_gap_mul", name)
		case tr.BuyerGapMul != 0:
			moved++
		}
		if moved == 0 {
			t.Errorf("%s moves no number", name)
		}
		t.Logf("%s: %d word(s) moved", name, moved)
	}
}

// TestNoTraitIsTheOldRun: with crew.toml's [traits] boxed (days = 0,
// harness.NoTraits) nobody shows a trait and nothing lived is written,
// so the crewed, captained and boss players are the old run: on the
// file, the run is the box's to the day before the first veteran shows
// a trait (what they lived through, the draw's weights, is set aside:
// it is written on the file and read by nothing until then), and under
// the box nothing of the feature is ever on the ground.
func TestNoTraitIsTheOldRun(t *testing.T) {
	t.Parallel()
	assertOldRun(t, oldRunCase{
		never: "showed no trait yet",
		box:   NoTraits,
		policies: map[string]func(*content.Config) Policy{
			"crewed":    func(c *content.Config) Policy { return Crewed(c, 40) },
			"captained": func(c *content.Config) Policy { return Captained(c, 40) },
			"boss":      func(c *content.Config) Policy { return Boss(c, 40, "") },
		},
		seeds: 3,
		days:  Horizon,
		asRun: true,
		until: func(_ *game.World, today []events.Event) bool {
			for _, e := range today {
				if _, ok := e.(events.CrewTrait); ok {
					return true
				}
			}
			return false
		},
		scrub: func(_ *testing.T, w *game.World, _ bool) func() {
			lived := make([][]string, len(w.Crew.Members))
			for i := range w.Crew.Members {
				lived[i], w.Crew.Members[i].Lived = w.Crew.Members[i].Lived, nil
			}
			return func() {
				for i := range w.Crew.Members {
					w.Crew.Members[i].Lived = lived[i]
				}
			}
		},
	})
	cfg := NoTraits(content.MustLoad())
	for seed := uint64(1); seed <= 3; seed++ {
		res, err := Run(cfg, seed, Horizon, Captained(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Events {
			if ev, ok := e.(events.CrewTrait); ok {
				t.Fatalf("seed %d: %+v with the traits boxed", seed, ev)
			}
		}
		for _, m := range res.World.Crew.Members {
			if m.Trait != "" || len(m.Lived) > 0 {
				t.Fatalf("seed %d: %+v carries a veteran's state with the traits boxed", seed, m)
			}
		}
	}
}

// TestNoCaptainIsTheOldRun: a run in which nobody is named captain is
// the run before the feature. Captained is crewed plus the naming, so
// the two are the same world, digest for digest, every day until the
// morning captained names one; and a crewed run never files a captain's
// night or carries a captaincy.
func TestNoCaptainIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := asRun(content.MustLoad())
	for seed := uint64(1); seed <= 3; seed++ {
		_, sims, err := sim.Default(cfg)
		if err != nil {
			t.Fatal(err)
		}
		_, sims2, err := sim.Default(cfg)
		if err != nil {
			t.Fatal(err)
		}
		a, b := sim.NewWorld(cfg, seed), sim.NewWorld(cfg, seed)
		ca, cb := game.NewClock(nil, sims...), game.NewClock(nil, sims2...)
		crewed, captained := Crewed(cfg, 40), Captained(cfg, 40)
		named := 0
		for day := 1; day <= Horizon && a.Over == nil && b.Over == nil; day++ {
			crewed(a)
			captained(b)
			if named == 0 {
				for _, m := range b.Crew.Members {
					if m.Captain != "" {
						named = day
					}
				}
			}
			for _, e := range ca.EndDay(a) {
				if ev, ok := e.(events.CaptainActed); ok {
					t.Fatalf("seed %d: crewed filed %+v", seed, ev)
				}
			}
			cb.EndDay(b)
			if named == 0 && digest(a) != digest(b) {
				t.Fatalf("seed %d: crewed and captained part on day %d with nobody named", seed, day)
			}
		}
		for _, m := range a.Crew.Members {
			if m.Captain != "" || m.Budget != 0 {
				t.Fatalf("seed %d: crewed carries a captaincy: %+v", seed, m)
			}
		}
		t.Logf("seed %d: the same run to day %d, the morning captained named one", seed, named)
	}
}

// TestCaptainedKeepsTheCrew: a captain at home is worth having. Over
// CaptainedSeeds seeds to Horizon, captained loses no more corners to
// idle drift and no more members to walking (a quit or a defection)
// than crewed on at least four seeds in five, and fewer of each in all.
// A seed can tie: the losses before anybody has served captain_days,
// or before anybody is loyal enough, happen to both runs.
func TestCaptainedKeepsTheCrew(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	losses := func(res Result) (idle, left int) {
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.CornerLost:
				if ev.Reason == "idle" {
					idle++
				}
			case events.CrewQuit, events.CrewDefected:
				left++
			}
		}
		return idle, left
	}
	kept, idleA, idleB, leftA, leftB := 0, 0, 0, 0, 0
	for seed := uint64(1); seed <= CaptainedSeeds; seed++ {
		a, err := Run(cfg, seed, Horizon, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		b, err := Run(cfg, seed, Horizon, Captained(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		ai, al := losses(a)
		bi, bl := losses(b)
		idleA, idleB, leftA, leftB = idleA+ai, idleB+bi, leftA+al, leftB+bl
		if bi <= ai && bl <= al {
			kept++
		}
	}
	t.Logf("over %d seeds to day %d: corners lost to drift %d (crewed) against %d (captained), members who walked %d against %d; captained no worse on both on %d seeds", CaptainedSeeds, Horizon, idleA, idleB, leftA, leftB, kept)
	if kept*5 < CaptainedSeeds*4 {
		t.Errorf("captained no worse than crewed on %d of %d seeds, want four in five", kept, CaptainedSeeds)
	}
	if idleB >= idleA || leftB >= leftA {
		t.Errorf("captained lost %d corners to drift and %d members, crewed %d and %d: a captain should keep both", idleB, leftB, idleA, leftA)
	}
}

// CaptainedSeeds is how many seeds TestCaptainedKeepsTheCrew reads.
const CaptainedSeeds = 20
