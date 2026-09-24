package game

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// A standing order can be set to the level a supply contract keeps on
// a day the stash holds less (#418): the contract tops the stash up
// every morning, so the routine's sell side is sized to it, not to a
// thin day.
func TestStandingReachesTheContractLevel(t *testing.T) {
	w := testWorld()
	w.SetStock("test", "a", 5)
	if err := w.SetSupply("test", "a", 30); err != nil {
		t.Fatal(err)
	}
	if err := w.PlaceStanding("test", "a", 30, events.DialNormal); err != nil {
		t.Fatalf("a standing order at the contract's 30 on a day the stash holds 5: %v", err)
	}
	if o, ok := w.YourStanding("test", "a"); !ok || o.Qty != 30 {
		t.Fatalf("standing %+v %v", o, ok)
	}
	if err := w.PlaceStanding("test", "a", 31, events.DialNormal); err == nil {
		t.Fatal("a standing order past the contract's level and the stash")
	}
}
