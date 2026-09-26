package rivals_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestWarEndSaysWhy (#520): a declared war that ends is reported with
// its reason: lost the night you hold no corner left anywhere (Lost),
// won the night their last corner where you hold ground goes, and
// called off by your own word that day (Called: report-only, no
// headline, no stop). A playtest lost its last corner to a third crew
// mid-war and the war ended with no word but a headline that it was
// won.
func TestWarEndSaysWhy(t *testing.T) {
	cfg := table(2)
	setup := func() (*game.World, *game.RivalState, func() []events.Event) {
		w, s := world(t, cfg, 5)
		f1, f2 := w.Rivals[0], w.Rivals[1]
		f2.Arrived, f2.Absorbed = 1, 1
		home := w.Home()
		for i := range home.Corners {
			if c := &home.Corners[i]; c.Held() {
				c.Owner, c.Runner, c.Enforcer = game.OwnerNone, 0, 0
			}
		}
		home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerPlayer, 1
		seat(w, f1, home.Corners[1].ID, 6)
		w.Day = 10
		if err := w.DeclareWar(f1.Faction()); err != nil {
			t.Fatal(err)
		}
		return w, f1, func() []events.Event { return step(w, s) }
	}

	// Lost: the last corner you held gone (to anyone), the war is lost.
	w, f1, night := setup()
	w.Home().Corners[0].Owner, w.Home().Corners[0].Runner = game.OwnerNone, 0
	ended := find[events.WarEnded](night())
	if ended == nil || !ended.Lost || ended.Called || ended.Faction != f1.Faction() || w.War != "" || ended.Why == "" {
		t.Fatalf("the war with no corner left: %+v", ended)
	}

	// Their last corner where you hold ground gone: not lost.
	w, f1, night = setup()
	w.Home().Corners[1].Owner, w.Home().Corners[1].Faction = game.OwnerNone, ""
	ended = find[events.WarEnded](night())
	if ended == nil || ended.Lost || ended.Called || ended.Faction != f1.Faction() {
		t.Fatalf("the war with their last corner gone: %+v", ended)
	}

	// Called off: reported the night after your word, and no more.
	w, f1, night = setup()
	if err := w.CallOffWar(); err != nil {
		t.Fatal(err)
	}
	ended = find[events.WarEnded](night())
	if ended == nil || !ended.Called || ended.Lost || ended.Faction != f1.Faction() || ended.Rival != f1.Leader || ended.Why != "you called it off" {
		t.Fatalf("the war called off: %+v", ended)
	}
	w.Today = game.Today{}
	if again := find[events.WarEnded](night()); again != nil {
		t.Fatalf("the war called off was reported twice: %+v", again)
	}
}
