package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestLandedAlert (#503): a playtest's 917 designer, shipped to Eastside
// by the road, sat in the stash with no standing order, and nothing
// flagged it. A product a route on its dial keeps in a city, stashed
// there with no order selling it (yours today, your standing one, or
// the lieutenant's), raises one landed alert, keyed so a fast-forward
// stops once; an order for it, an empty stash or the route off clears
// it. It opens the market on the city.
func TestLandedAlert(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		set  func(w *game.World, home, product string)
		want bool
	}{
		{"stashed, nothing selling it", func(w *game.World, home, product string) {}, true},
		{"an order tonight", func(w *game.World, home, product string) {
			_ = w.PlaceSell(home, product, 10, events.DialNormal)
		}, false},
		{"a standing order", func(w *game.World, home, product string) {
			_ = w.PlaceStanding(home, product, game.AllUnits, events.DialNormal)
		}, false},
		{"nothing stashed", func(w *game.World, home, product string) { w.SetStock(home, product, 0) }, false},
		{"the route off", func(w *game.World, home, product string) { _ = w.SetRoute("interstate", events.RouteOff) }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, w := crewRun(t)
			home := w.Home().ID
			product := w.Products[0]
			if err := w.SetRoute("interstate", events.RouteNormal); err != nil {
				t.Fatal(err)
			}
			if err := w.SetRouteTarget("interstate", product, 200); err != nil {
				t.Fatal(err)
			}
			w.SetStock(home, product, 120)
			c.set(w, home, product)
			got := ofKind(s, engine.AlertLanded)
			if !c.want {
				if len(got) != 0 {
					t.Fatalf("a landed alert: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].City != home || got[0].Product != product || got[0].Count != 120 || got[0].Key == "" {
				t.Fatalf("landed alerts %+v, want one for the 120 %s at home", got, product)
			}
			if got[0].Act.Screen != engine.ScreenMarket || got[0].Act.Subject != engine.SubjectCity {
				t.Fatalf("the landed alert opens %+v, not the market on the city", got[0].Act)
			}
			// The same stash the next morning is the same alert: the key
			// does not move with the count.
			key := got[0].Key
			w.SetStock(home, product, 300)
			if again := ofKind(s, engine.AlertLanded); len(again) != 1 || again[0].Key != key {
				t.Fatalf("the key moved: %+v, was %q", again, key)
			}
		})
	}
}
