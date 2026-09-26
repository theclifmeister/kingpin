package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
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

// TestExportsAlertOnce (#505): played morning by morning, a run is
// pointed at the lanes abroad once, at the Cartel stage and never
// before it. The boss, which never owns the book, is told the morning
// it reaches the stage and the alert stands, one key, so a fast-forward
// stops on it once. The cartel buys the book the night the door opens
// and orders the next morning, before the stage is stamped, so it is
// told at most once and nothing stands once the lanes are in use.
func TestExportsAlertOnce(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	cartel := -1
	for i, tr := range cfg.Progression.Tiers {
		if tr.ID == "cartel" {
			cartel = i + 1
		}
	}
	for _, c := range []struct {
		name   string
		policy Policy
	}{{"cartel", Cartel(cfg, 40)}, {"boss", Boss(cfg, 40, "")}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := sim.NewWorld(cfg, 1)
			was, fresh, first := false, 0, -1
			playMornings(t, cfg, w, 300, c.policy, func(s *engine.Session, w *game.World) {
				now := false
				for _, a := range s.Alerts() {
					if a.Kind == engine.AlertExports {
						now = true
						if w.ReachedOn(cartel) < 0 {
							t.Fatalf("day %d: the lanes pointed at before the Cartel stage", w.Day)
						}
					}
				}
				if now && !was {
					fresh++
					if first < 0 {
						first = w.Day
					}
				}
				was = now
			})
			if w.ReachedOn(cartel) < 0 {
				t.Skipf("the %s never reached the Cartel stage by day 300", c.name)
			}
			if want := c.name == "boss"; fresh > 1 || want && fresh != 1 {
				t.Fatalf("the exports alert came new on %d mornings (first day %d)", fresh, first)
			}
			if c.name == "cartel" && (w.Stats.ExportLoads == 0 || was) {
				t.Fatalf("the cartel shipped %d loads and the alert still stands: %v", w.Stats.ExportLoads, was)
			}
			t.Logf("%s: Cartel stage day %d, pointed at the lanes from day %d, %d loads", c.name, w.ReachedOn(cartel), first, w.Stats.ExportLoads)
		})
	}
}
