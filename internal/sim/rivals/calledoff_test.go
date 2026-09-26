package rivals_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestEveryStrikeHasAWord (#524): a boost queued on a crew's corner
// whose crew is finished by nightfall is not resolved, and the morning
// says so (StrikeCalledOff, "no target"), with nothing moved; one that
// lands has its own event and no called-off word; with nothing queued,
// no word.
func TestEveryStrikeHasAWord(t *testing.T) {
	cfg := duel()
	w, s := warWorld(t, cfg, 5, "defensive")
	w.Rival().Muscle = 2
	if err := w.Boost("docks", events.ForceHit); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	if find[events.RivalBoosted](evs) == nil || find[events.StrikeCalledOff](evs) != nil {
		t.Fatalf("a boost that went: %v", kinds(evs))
	}
	w.Today.Strike = nil
	if find[events.StrikeCalledOff](step(w, s)) != nil {
		t.Fatal("a called-off word with nothing queued")
	}
	if err := w.Boost("docks", events.ForceHit); err != nil {
		t.Fatal(err)
	}
	w.Rival().Fragmented = w.Day // the crew is finished before the night
	cash := w.Player.DirtyCash
	evs = step(w, s)
	ev := find[events.StrikeCalledOff](evs)
	if ev == nil || ev.Corner != "docks" || !ev.Boost || !strings.HasPrefix(ev.Why, "no target") {
		t.Fatalf("the boost on a finished crew: %+v (%v)", ev, kinds(evs))
	}
	if find[events.RivalBoosted](evs) != nil || w.Player.DirtyCash != cash || w.Corner("docks").Owner != game.OwnerRival {
		t.Fatalf("a called-off boost moved something: %v, dirty %d", kinds(evs), w.Player.DirtyCash)
	}
}
