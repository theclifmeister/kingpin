package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// NoFavour returns a copy of cfg with the favour boxed (#228): no chief
// owes one, so nothing can be called in.
func NoFavour(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Law.Bribes.FavoursMax = 0
	return &boxed
}

// TestNoFavourIsTheOldRun (#228): a run that never calls in a favour is
// byte-for-byte the run before the favour existed. The corrupt player
// (who bribes and never calls) and the boss are hashed daily
// (assertOldRun: two seeds to the tier-4 checkpoint) on the file and
// on the file with favours_max at 0, the count of favours owed and the
// morning report set aside as #195's quiet days are (the count and the
// bribe line's `the chief owes you one` are what a taken envelope
// moves; the report is text the news sim writes, never a sim's read),
// and nothing falls through.
func TestNoFavourIsTheOldRun(t *testing.T) {
	t.Parallel()
	assertOldRun(t, oldRunCase{
		never: "never called a favour",
		seeds: 2,
		days:  Horizon,
		box:   NoFavour,
		policies: map[string]func(*content.Config) Policy{
			"corrupt": func(c *content.Config) Policy { return Corrupt(c, 40) },
			"boss":    func(c *content.Config) Policy { return Boss(c, 40, "") },
		},
		forbid: func(e events.Event) bool {
			_, ok := e.(events.RaidFellThrough)
			return ok
		},
		scrub: func(_ *testing.T, w *game.World, _ bool) func() {
			favours, rep := w.Law.Favours, w.Report
			w.Law.Favours, w.Report = 0, nil
			return func() { w.Law.Favours, w.Report = favours, rep }
		},
		after: func(t *testing.T, w *game.World, boxed bool) {
			if w.Law.FavourOwed != 0 || w.Stats.Favours != 0 || (boxed && w.Law.Favours != 0) {
				t.Fatalf("the favour moved with nobody calling: %+v stats %d", w.Law, w.Stats.Favours)
			}
		},
	})
}

// TestAggressiveStillIndicted (#228, #60's rule): the favour never
// lowers heat, so the aggressive trader with the chief bought and a
// favour granted on every envelope, calling it in whenever a raid is
// due, is indicted within a few days of when it is without: a skipped
// raid is a page in the file and the heat still saturates. Under a
// corrupt chief and a moderate DA on five seeds, the favoured one
// ends the same way and no more than favour_slack days later.
func TestAggressiveStillIndicted(t *testing.T) {
	t.Parallel()
	const slack = 6
	cfg := content.MustLoad()
	hs := heat.New(cfg)
	tun := cfg.Law.Bribes
	for seed := uint64(1); seed <= 5; seed++ {
		play := func(favours bool) Result {
			w := sim.NewWorld(cfg, seed)
			w.Player.DirtyCash = CorruptTableCash
			run := Appoint(cfg, w, "corrupt", "moderate")
			trader := Trader(run, events.DialAggressive)
			res, err := RunFrom(run, w, 2*Horizon, func(w *game.World) {
				trader(w)
				if w.Over != nil {
					return
				}
				if !w.Law.ChiefBoughtOn(w.Day) && w.BribedToday(game.BribeChief) == 0 && w.Player.DirtyCash >= tun.ChiefPrice {
					_ = w.Bribe(game.BribeChief, tun.ChiefPrice)
				}
				if favours {
					if due := hs.Due(w); w.CanCallFavour(due != "") {
						_ = w.CallFavour(true)
					}
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			return res
		}
		plain, favoured := play(false), play(true)
		if plain.Over == nil || favoured.Over == nil {
			t.Fatalf("seed %d: the aggressive trader played on: plain %+v favoured %+v", seed, plain.Over, favoured.Over)
		}
		t.Logf("seed %d: plain %s on day %d; favoured %s on day %d, %d favours called, %d responses fell through", seed, plain.Over.Cause, plain.Over.Day, favoured.Over.Cause, favoured.Over.Day, favoured.World.Stats.Favours, fellThrough(favoured))
		if favoured.Over.Day > plain.Over.Day+slack {
			t.Errorf("seed %d: the favour bought %d days (%d against %d): it must never make an always-aggressive player last longer", seed, favoured.Over.Day-plain.Over.Day, favoured.Over.Day, plain.Over.Day)
		}
	}
}

// fellThrough counts the responses a run's favours stopped.
func fellThrough(res Result) int {
	n := 0
	for _, e := range res.Events {
		if _, ok := e.(events.RaidFellThrough); ok {
			n++
		}
	}
	return n
}
