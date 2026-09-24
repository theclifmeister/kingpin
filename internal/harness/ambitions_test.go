package harness

import (
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The ambitions (#347, docs/ambitions.md): the endings as plans read off
// the world, never written into it; a full bar exactly when the
// ending's own condition holds; and a plan pinned or not, the same run.

// playMornings plays up to days days of policy on w through a session
// built on cfg with the deck and the weather boxed (as Run plays), and
// hands each morning, before the policy acts, and the world the run
// ends on to morning.
func playMornings(t *testing.T, cfg *content.Config, w *game.World, days int, policy Policy, morning func(s *engine.Session, w *game.World)) {
	t.Helper()
	boxed := *cfg
	boxed.Dilemmas.Cards, boxed.Incidents.Table = nil, nil
	sess, err := engine.New(&boxed)
	if err != nil {
		t.Fatal(err)
	}
	sess.Attach(w)
	for d := 0; d < days && w.Over == nil; d++ {
		morning(sess, w)
		if policy != nil {
			policy(w)
		}
		sess.EndDay()
	}
	morning(sess, w)
}

// TestAmbitionsNeverWriteTheWorld (#347): every morning of four
// players, and of the scenarios that reach the endings the plans are
// for, reading the plans, the view and the alerts leaves the world as
// it was (TestSeedDigest's walk of every exported value, and the pin).
func TestAmbitionsNeverWriteTheWorld(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	check := func(t *testing.T, s *engine.Session, w *game.World) {
		before := digest(w) + w.Ambition
		s.Ambitions()
		s.Plan()
		s.View()
		s.Alerts()
		if after := digest(w) + w.Ambition; before != after {
			t.Fatalf("day %d: reading the ambitions wrote the world", w.Day)
		}
	}
	for _, name := range []string{"boss", "informed", "retiree", "delegated"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := sim.NewWorld(cfg, 1)
			_ = w.PinAmbition(content.AmbitionRetire) // a plan pinned: its alert and its view read too
			playMornings(t, cfg, w, 90, policyNamed(t, cfg, name), func(s *engine.Session, w *game.World) { check(t, s, w) })
		})
	}
	for _, sc := range scenarios() {
		if content.AmbitionEnding[ambitionFor(sc.cause)] == "" {
			continue
		}
		t.Run(sc.cause, func(t *testing.T) {
			t.Parallel()
			c, w := scenarioWorld(cfg, sc)
			_ = w.PinAmbition(ambitionFor(sc.cause))
			playMornings(t, c, w, sc.days, sc.play(c), func(s *engine.Session, w *game.World) { check(t, s, w) })
		})
	}
}

// TestAmbitionProgressAgreesWithTheEnding (#347): the bar reads 100%
// exactly when the ending's own condition holds, every morning: Retire
// on the account and the quiet days (World.CanRetire's terms), the
// businessman on the laundering sim's count at legit_days, the city on
// the reign the rivals sim stamps (World.CanCrown), and the new identity
// on the tree (World.CanVanish). The scenarios that reach each ending
// read 100% on the morning it is taken, and the players that never
// reach one never read it.
func TestAmbitionProgressAgreesWithTheEnding(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	off, legit := cfg.Laundering.Offshore, cfg.Laundering.Businessman.LegitDays
	agree := func(t *testing.T, s *engine.Session, w *game.World, full map[string]bool) {
		fx := game.FoldEffects(w, cfg.Upgrades)
		holds := map[string]bool{
			content.AmbitionRetire: w.Offshore >= off.RetireCash && w.QuietDays >= off.RetireDays,
			content.AmbitionLegit:  w.LegitDays >= legit,
			content.AmbitionCity:   w.Reign > 0,
			content.AmbitionVanish: fx.Identities > 0,
		}
		for _, a := range s.Ambitions() {
			want, ok := holds[a.ID]
			if !ok {
				continue
			}
			if got := a.Progress() == 1; got != want {
				t.Fatalf("day %d: %s reads %.2f, the ending's condition %v", w.Day, a.ID, a.Progress(), want)
			}
			if a.Progress() == 1 {
				full[a.ID] = true
			}
			if (a.Next() == nil) != want {
				t.Fatalf("day %d: %s's next step %v with the condition %v", w.Day, a.ID, a.Next(), want)
			}
		}
	}
	for _, sc := range scenarios() {
		id := ambitionFor(sc.cause)
		if content.AmbitionEnding[id] == "" {
			continue
		}
		t.Run(sc.cause, func(t *testing.T) {
			t.Parallel()
			c, w := scenarioWorld(cfg, sc)
			full := map[string]bool{}
			playMornings(t, c, w, sc.days, sc.play(c), func(s *engine.Session, w *game.World) { agree(t, s, w, full) })
			if w.Over == nil || w.Over.Cause != sc.cause {
				t.Fatalf("the scenario ended %+v", w.Over)
			}
			if !full[id] {
				t.Errorf("%s ended the run and the %s plan never read 100%%", sc.cause, id)
			}
		})
	}
	for _, name := range []string{"boss", "retiree", "laundered", "crewed", "aggressive"} {
		for seed := uint64(1); seed <= 2; seed++ {
			t.Run(fmt.Sprintf("%s/seed%d", name, seed), func(t *testing.T) {
				t.Parallel()
				full := map[string]bool{}
				playMornings(t, cfg, sim.NewWorld(cfg, seed), 120, policyNamed(t, cfg, name), func(s *engine.Session, w *game.World) { agree(t, s, w, full) })
			})
		}
	}
}

// TestNoAmbitionIsTheOldRun (#347): no sim reads the plan, so every
// policy plays the same run pinned or not: the net worth every day, the
// ending and the world at the end (TestSeedDigest's walk, which leaves
// the pin out) are the same. A
// policy pins one plan (the list in turn) on its first morning.
func TestNoAmbitionIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for i, np := range Policies {
		id := content.AmbitionIDs[i%len(content.AmbitionIDs)]
		t.Run(np.Name, func(t *testing.T) {
			t.Parallel()
			bare, err := Run(cfg, 1, 90, np.Make(cfg, PolicyOpts{}))
			if err != nil {
				t.Fatal(err)
			}
			policy := np.Make(cfg, PolicyOpts{})
			pinned, err := Run(cfg, 1, 90, func(w *game.World) {
				if w.Ambition == "" {
					if err := w.PinAmbition(id); err != nil {
						t.Fatal(err)
					}
				}
				policy(w)
			})
			if err != nil {
				t.Fatal(err)
			}
			if pinned.World.Ambition != id {
				t.Fatalf("the plan pinned is %q, want %q", pinned.World.Ambition, id)
			}
			for d := range bare.NetWorth {
				if d >= len(pinned.NetWorth) || bare.NetWorth[d] != pinned.NetWorth[d] {
					t.Fatalf("pinned %s: the net worth moved on day %d", id, d+1)
				}
			}
			if len(bare.NetWorth) != len(pinned.NetWorth) || fmt.Sprint(bare.Over) != fmt.Sprint(pinned.Over) {
				t.Fatalf("pinned %s: the run ended %v, unpinned %v", id, pinned.Over, bare.Over)
			}
			if digest(bare.World) != digest(pinned.World) { // the digest's walk leaves the pin out
				t.Fatalf("pinned %s: the world at the end is not the unpinned one", id)
			}
		})
	}
}

// ambitionFor is the plan for an ending's cause, "" for one no plan is
// for.
func ambitionFor(cause string) string {
	for id, c := range content.AmbitionEnding {
		if c == cause {
			return id
		}
	}
	return ""
}

// scenarioWorld is a scenario's config and world, set up as
// TestEveryEndingIsReachable sets them up.
func scenarioWorld(base *content.Config, sc scenario) (*content.Config, *game.World) {
	cfg := base
	if sc.cfg != nil {
		cfg = sc.cfg(base)
	}
	w := sim.NewWorld(cfg, 1)
	if sc.world != nil {
		sc.world(cfg, w)
	}
	return cfg, w
}

// policyNamed is the registry's policy by name.
func policyNamed(t *testing.T, cfg *content.Config, name string) Policy {
	t.Helper()
	for _, np := range Policies {
		if np.Name == name {
			return np.Make(cfg, PolicyOpts{})
		}
	}
	t.Fatalf("no policy %q", name)
	return nil
}
