package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestExportsAlert (#505): the lanes abroad are the late game's money
// and no playtester found them before the Cartel stage. The exports
// alert points at them from the morning the run is at the Cartel stage
// until a load is ordered or has gone out: once, keyed so a
// fast-forward stops on it once a run, to the ledger, naming the lane's
// city, what a night carries, the best margin's product at its price
// abroad and its cost off the book, and whether the book is owned. Not
// before the stage, and not once the lanes are in use.
func TestExportsAlert(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	cartel := -1
	for i, tr := range cfg.Progression.Tiers {
		if tr.ID == "cartel" {
			cartel = i + 1
		}
	}
	if cartel < 0 {
		t.Fatal("no cartel stage in the file")
	}
	book := cfg.Assets.ByEffect(content.AssetSupplier)
	lane := cfg.Exports.Lanes[0]
	stage := func(w *game.World) { w.Reach(cartel, w.Day) }
	own := func(w *game.World) {
		stage(w)
		w.Assets = append(w.Assets, game.Asset{ID: book.ID, Name: book.Name, Effect: book.Effect, City: book.City})
	}
	for _, c := range []struct {
		name  string
		set   func(w *game.World)
		want  bool
		ready bool
	}{
		{"before the stage", func(w *game.World) {}, false, false},
		{"the stage before the book", stage, true, false},
		{"the book owned", own, true, true},
		{"a lane ordered", func(w *game.World) {
			own(w)
			if err := w.SetExport(lane.ID, lane.Products[0], 100); err != nil {
				t.Fatal(err)
			}
		}, false, false},
		{"a load out", func(w *game.World) {
			own(w)
			w.Exports.Loads = []game.ExportLoad{{ID: 1, Lane: lane.ID, Product: lane.Products[0], Units: 100, Lands: w.Day + 3}}
		}, false, false},
		{"a load gone before", func(w *game.World) { own(w); w.Stats.ExportLoads = 1 }, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, w := crewRun(t)
			s.EndDay() // the prices stamped
			c.set(w)
			got := ofKind(s, engine.AlertExports)
			if !c.want {
				if len(got) != 0 {
					t.Fatalf("an exports alert: %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("exports alerts %+v, want one", got)
			}
			a := got[0]
			if a.City != lane.City || a.Count <= 0 || a.Ready != c.ready || a.Key == "" {
				t.Fatalf("the alert %+v, want the lane out of %s, ready %v", a, lane.City, c.ready)
			}
			if a.Product == "" || a.Amount <= a.Have || a.Have <= 0 {
				t.Fatalf("the alert names no margin: %+v", a)
			}
			if a.Act.Screen != engine.ScreenLedger {
				t.Fatalf("the alert opens %+v, not the ledger", a.Act)
			}
			// Keyed once: the book bought changes the words, not the
			// key, so a fast-forward stops on it once a run.
			own(w)
			if again := ofKind(s, engine.AlertExports); len(again) != 1 || again[0].Key != a.Key {
				t.Fatalf("the key moved: %+v then %+v", a, again)
			}
		})
	}
}
