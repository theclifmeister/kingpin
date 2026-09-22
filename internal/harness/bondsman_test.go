package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// NoBondsman returns a copy of cfg with the bondsman (#230) taken off
// the tree: the file before the node existed.
func NoBondsman(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Upgrades.Nodes = nil
	for _, n := range cfg.Upgrades.Nodes {
		if !n.Effects.AutoBail {
			boxed.Upgrades.Nodes = append(boxed.Upgrades.Nodes, n)
		}
	}
	return &boxed
}

// TestNoBondsmanIsTheOldRun (#230): a run that never buys the bondsman
// is byte-for-byte the run before the node existed. The crewed player
// (no tree), the upgraded one (the tree at 3x) and the boss (the tree
// at BossMargin with the clean cash for it) are hashed daily on the
// file and on the file with the node taken off, 120 days, three seeds,
// and no bail is ever the lawyer's: BuyUpgrades skips the node, since
// no scripted policy bails by hand and the bondsman would change the
// policy rather than the tree, so no pinned number moves.
func TestNoBondsmanIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	off := NoBondsman(cfg)
	if len(off.Upgrades.Nodes) != len(cfg.Upgrades.Nodes)-1 {
		t.Fatalf("the tree has %d nodes with the bondsman and %d without", len(cfg.Upgrades.Nodes), len(off.Upgrades.Nodes))
	}
	for name, policy := range map[string]func(*content.Config) Policy{
		"crewed":   func(c *content.Config) Policy { return Crewed(c, 40) },
		"upgraded": func(c *content.Config) Policy { return Upgraded(c, 40) },
		"boss":     func(c *content.Config) Policy { return Boss(c, 40, "") },
	} {
		for seed := uint64(1); seed <= 3; seed++ {
			var with, without []string
			for i, c := range []*content.Config{cfg, off} {
				w := sim.NewWorld(c, seed)
				_, sims, err := sim.Default(c)
				if err != nil {
					t.Fatal(err)
				}
				clock := game.NewClock(nil, sims...)
				p := policy(c)
				var ds []string
				for day := 1; day <= 120 && w.Over == nil; day++ {
					p(w)
					for _, e := range clock.EndDay(w) {
						switch ev := e.(type) {
						case events.CrewBailed:
							if ev.Who != "" {
								t.Fatalf("%s seed %d day %d: %+v in a run that never bought the bondsman", name, seed, day, ev)
							}
						case events.CrewArrested:
							if ev.Sprung || ev.Short {
								t.Fatalf("%s seed %d day %d: %+v in a run that never bought the bondsman", name, seed, day, ev)
							}
						}
					}
					ds = append(ds, digest(w))
				}
				if w.Owns("bondsman") {
					t.Fatalf("%s seed %d bought the bondsman", name, seed)
				}
				if i == 0 {
					with = ds
				} else {
					without = ds
				}
			}
			for day := range with {
				if with[day] != without[day] {
					t.Fatalf("%s seed %d: the world moved on day %d with the bondsman in the file and nobody buying it", name, seed, day+1)
				}
			}
		}
	}
}
