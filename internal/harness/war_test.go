package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestNoWarIsTheOldRun (#229): a run that never declares war is
// byte-for-byte the run before the order existed. Eight policies,
// the fighters among them, hashed daily on the file and on the file
// with the war boxed (harness.NoWar; assertOldRun: one seed, 120
// days), identical, and no strike of the war's, no WarEnded.
func TestNoWarIsTheOldRun(t *testing.T) {
	t.Parallel()
	assertOldRun(t, oldRunCase{
		never: "never declared war",
		seeds: 1,
		days:  120,
		box:   NoWar,
		policies: map[string]func(*content.Config) Policy{
			"crewed":      func(c *content.Config) Policy { return Crewed(c, 40) },
			"war":         func(c *content.Config) Policy { return Warlike(c, 40, 0, events.ForceHit) },
			"diplomat":    func(c *content.Config) Policy { return Diplomat(c, 40, 0) },
			"boss":        func(c *content.Config) Policy { return Boss(c, 40, "") },
			"distributor": func(c *content.Config) Policy { return Distributor(c, 40) },
			"tipster":     func(c *content.Config) Policy { return Tipster(c, 40) },
			"saboteur":    func(c *content.Config) Policy { return Saboteur(c, 40) },
			"upgraded":    func(c *content.Config) Policy { return Upgraded(c, 40) },
		},
		forbid: func(e events.Event) bool {
			switch ev := e.(type) {
			case events.WarEnded:
				return true
			case events.CornerStruck:
				return ev.War
			}
			return false
		},
		after: func(t *testing.T, w *game.World, _ bool) {
			if w.War != "" {
				t.Fatalf("at war with nobody declaring: %q", w.War)
			}
		},
	})
}

// TestWarIsTheHandsStrikes (#229): a war night and the same strike sent
// by hand on the same seed produce the same world. The crewed player
// under the war order (Warlord) is played to day 120 and every strike
// of the war's recorded (the night, the corner, the dial); the crewed
// player then plays the same seed sending the enforcers by hand each
// morning to the corner the record says, and the two are hashed daily
// and read identical, the strikes counted the same, with the war's
// field, the morning report and the journal set aside: the one place
// the order names itself is the news sim's words (the STREET line says
// `The war on ...`, the night it ends has a headline), never a sim's
// state. The record, not a morning's guess, is what the hand replays:
// the war picks its corner at the rivals sim's step, where the map is
// as the night's earlier sims left it.
func TestWarIsTheHandsStrikes(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	if _, on := cfg.Rivals.War.Force(); !on {
		t.Fatal("the war is boxed in the file")
	}
	type order struct {
		corner string
		force  events.Force
	}
	for seed := uint64(1); seed <= 3; seed++ {
		record := map[int]order{}
		var byWar, byHand []string
		strikes := [2]int{}
		for i := 0; i < 2; i++ {
			w := sim.NewWorld(cfg, seed)
			_, sims, err := sim.Default(cfg)
			if err != nil {
				t.Fatal(err)
			}
			clock := game.NewClock(nil, sims...)
			policy := Crewed(cfg, 40)
			if i == 0 {
				policy = Warlord(cfg, 40)
			}
			var ds []string
			for day := 1; day <= 120 && w.Over == nil; day++ {
				policy(w)
				if i == 1 && w.Over == nil && w.Today.Strike == nil {
					if o, ok := record[day]; ok {
						if err := w.SendEnforcers(o.corner, o.force); err != nil {
							t.Fatalf("seed %d day %d: the hand could not send what the war sent: %v", seed, day, err)
						}
					}
				}
				for _, e := range clock.EndDay(w) {
					if ev, ok := e.(events.CornerStruck); ok && ev.War {
						if i == 1 {
							t.Fatalf("seed %d day %d: a war strike in the hand's run", seed, day)
						}
						record[day] = order{ev.Corner, ev.Force}
					}
				}
				war, rep, journal := w.War, w.Report, w.Journal
				w.War, w.Report, w.Journal = "", nil, nil
				ds = append(ds, digest(w))
				w.War, w.Report, w.Journal = war, rep, journal
			}
			strikes[i] = w.Stats.Strikes
			if i == 0 {
				byWar = ds
			} else {
				byHand = ds
			}
		}
		if len(byWar) != len(byHand) {
			t.Fatalf("seed %d: %d days at war against %d by hand", seed, len(byWar), len(byHand))
		}
		for day := range byWar {
			if byWar[day] != byHand[day] {
				t.Fatalf("seed %d: the war and the hand parted on day %d (%d strikes against %d)", seed, day+1, strikes[0], strikes[1])
			}
		}
		if strikes[0] != strikes[1] || strikes[0] != len(record) {
			t.Fatalf("seed %d: %d strikes at war, %d by hand, %d recorded", seed, strikes[0], strikes[1], len(record))
		}
		t.Logf("seed %d: %d strikes, the war and the hand the same world for %d days", seed, strikes[0], len(byWar))
	}
}

// TestWarlordFightsMore (#229): the warlord (crewed with the war order
// on the nearest faction) wins more corners by force than crewed over
// twenty seeds and 200 days, and ends indicted or taken out at least as
// often; the frequencies are logged. The war never ends a run itself:
// every ending is one of the causes the sims already write.
func TestWarlordFightsMore(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	type tally struct {
		won     []int
		endings map[string]int
	}
	play := func(policy func(*content.Config) Policy) tally {
		out := tally{endings: map[string]int{}}
		for seed := uint64(1); seed <= 20; seed++ {
			res, err := Run(cfg, seed, Horizon, policy(cfg))
			if err != nil {
				t.Fatal(err)
			}
			out.won = append(out.won, res.World.Stats.CornersWon)
			if res.Over != nil {
				out.endings[res.Over.Cause]++
			}
		}
		sort.Ints(out.won)
		return out
	}
	warlord := play(func(c *content.Config) Policy { return Warlord(c, 40) })
	crewed := play(func(c *content.Config) Policy { return Crewed(c, 40) })
	t.Logf("warlord: corners won median %d (%d..%d), endings %v; crewed: median %d, endings %v", warlord.won[10], warlord.won[0], warlord.won[19], warlord.endings, crewed.won[10], crewed.endings)
	if warlord.won[10] <= crewed.won[10] {
		t.Errorf("the warlord won a median of %d corners against crewed's %d", warlord.won[10], crewed.won[10])
	}
	fought := warlord.endings[content.CauseIndicted] + warlord.endings[content.CauseTakenOut] + warlord.endings[content.CauseArrested]
	quiet := crewed.endings[content.CauseIndicted] + crewed.endings[content.CauseTakenOut] + crewed.endings[content.CauseArrested]
	if fought < quiet {
		t.Errorf("the warlord was indicted, arrested or taken out on %d seeds, crewed on %d", fought, quiet)
	}
}
