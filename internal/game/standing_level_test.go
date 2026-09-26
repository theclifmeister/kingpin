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

// A standing order can be sized over the stash for what lands in the
// city tonight (#503): goods on the road due by tomorrow's day on a
// route not shut, and a chemist's batch ready by then (Landing), sold
// from the night after. A shipment due later, or on a shut route, is
// not counted; AllUnits keeps the order at the whole stash (All), and
// with nothing at all to sell it is refused.
func TestStandingSizedForWhatLands(t *testing.T) {
	w := testWorld()
	w.SetStock("test", "a", 10)
	w.Shipments = []Shipment{
		{ID: 1, Route: "r", To: "test", Product: "a", Units: 50, Arrives: w.Day + 1},
		{ID: 2, Route: "r", To: "test", Product: "a", Units: 70, Arrives: w.Day + 2}, // lands later
	}
	w.Crew.Cooks = []Cook{{City: "test", Product: "a", Units: 5, Ready: w.Day + 1}}
	if n := w.Landing("test", "a"); n != 55 {
		t.Fatalf("Landing %d, want the shipment's 50 and the batch's 5", n)
	}
	if err := w.PlaceStanding("test", "a", 65, events.DialNormal); err != nil {
		t.Fatalf("a standing order for the stash and what lands tonight: %v", err)
	}
	if err := w.PlaceStanding("test", "a", 66, events.DialNormal); err == nil {
		t.Fatal("a standing order past what lands tonight")
	}
	if err := w.PlaceSell("test", "a", 11, events.DialNormal); err == nil {
		t.Fatal("tonight's order counted what lands after the sales")
	}
	rs := w.Route("r")
	rs.ClosedUntil = w.Day + 3
	w.Routes = map[string]RouteSetting{"r": rs}
	if n := w.Landing("test", "a"); n != 5 {
		t.Fatalf("Landing %d on a shut road, want the batch's 5", n)
	}
	if err := w.PlaceStanding("test", "a", AllUnits, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if o, _ := w.YourStanding("test", "a"); !o.All || o.Qty != 15 {
		t.Fatalf("all of it: %+v, want All at the 15 there is", o)
	}
	w.SetStock("test", "a", 0)
	w.Shipments, w.Crew.Cooks = nil, nil
	if err := w.PlaceStanding("test", "a", AllUnits, events.DialNormal); err == nil {
		t.Fatal("all of nothing stood")
	}
}
