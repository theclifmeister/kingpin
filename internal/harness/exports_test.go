package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestExportsSaveAndReplay (#391) runs the cartel to a day it has
// loads out abroad, saves, loads and plays on beside the unsaved run:
// the same world every day after, seizures and landings alike. A fresh
// run on the seed replays the same.
func TestExportsSaveAndReplay(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	const seed, at, on = 1, 260, 30
	a, _ := Run(cfg, seed, at+on, Cartel(cfg, 40))
	b, _ := Run(cfg, seed, at+on, Cartel(cfg, 40))
	if digest(a.World) != digest(b.World) {
		t.Fatal("the same seed diverged with the lanes in play")
	}
	c, _ := Run(cfg, seed, at, Cartel(cfg, 40))
	w := c.World
	if len(w.Exports.Loads) == 0 || len(w.Exports.Orders) == 0 {
		t.Fatalf("day %d: no load out abroad (%d loads left so far)", at, w.Stats.ExportLoads)
	}
	if err := game.Save(1, w); err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Exports.Loads) != len(w.Exports.Loads) || loaded.Exports.Loads[0] != w.Exports.Loads[0] || len(loaded.Exports.Glut) != len(w.Exports.Glut) {
		t.Fatalf("the save lost the lanes: %+v / %+v", loaded.Exports, w.Exports)
	}
	straight, _ := RunFrom(cfg, w, on, Cartel(cfg, 40))
	replayed, _ := RunFrom(cfg, loaded, on, Cartel(cfg, 40))
	if digest(straight.World) != digest(replayed.World) || digest(straight.World) != digest(a.World) {
		t.Fatalf("the run diverged after the save: %d / %d / %d", straight.World.NetWorth(), replayed.World.NetWorth(), a.World.NetWorth())
	}
}

// TestNoBookIsTheOldRun (#391): the lanes need the book, and every
// policy below the cartel never buys it, so the boss plays the same
// day by day with the lanes in the file and with them boxed: the lanes
// and the cartel's wash are a run's only once it owns the book.
func TestNoBookIsTheOldRun(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	boxed := *cfg
	boxed.Exports.Lanes = nil
	for seed := uint64(1); seed <= 3; seed++ {
		a, _ := Run(cfg, seed, 200, Boss(cfg, 40, ""))
		b, _ := Run(&boxed, seed, 200, Boss(&boxed, 40, ""))
		if digest(a.World) != digest(b.World) {
			t.Fatalf("seed %d: the boss's run moved with the lanes in the file: %d / %d", seed, a.World.NetWorth(), b.World.NetWorth())
		}
		if a.World.Stats.ExportLoads != 0 || len(a.World.Exports.Orders) != 0 {
			t.Fatalf("seed %d: the boss shipped abroad", seed)
		}
	}
}
