package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
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
// file and on the file with the node taken off (assertOldRun: three
// seeds, 120 days), and no bail is ever the lawyer's: BuyUpgrades
// skips the node, since no scripted policy bails by hand and the
// bondsman would change the policy rather than the tree, so no pinned
// number moves.
func TestNoBondsmanIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	if off := NoBondsman(cfg); len(off.Upgrades.Nodes) != len(cfg.Upgrades.Nodes)-1 {
		t.Fatalf("the tree has %d nodes with the bondsman and %d without", len(cfg.Upgrades.Nodes), len(off.Upgrades.Nodes))
	}
	assertOldRun(t, oldRunCase{
		never: "never bought the bondsman",
		seeds: 3,
		days:  120,
		box:   NoBondsman,
		policies: map[string]func(*content.Config) Policy{
			"crewed":   func(c *content.Config) Policy { return Crewed(c, 40) },
			"upgraded": func(c *content.Config) Policy { return Upgraded(c, 40) },
			"boss":     func(c *content.Config) Policy { return Boss(c, 40, "") },
		},
		forbid: func(e events.Event) bool {
			switch ev := e.(type) {
			case events.CrewBailed:
				return ev.Who != ""
			case events.CrewArrested:
				return ev.Sprung || ev.Short
			}
			return false
		},
		after: func(t *testing.T, w *game.World, _ bool) {
			if w.Owns("bondsman") {
				t.Fatal("bought the bondsman")
			}
		},
	})
}
