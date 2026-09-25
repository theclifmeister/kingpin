package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestPortAlert (#476): a playtest played Eastside alone for 519 days
// and found Bayport's free corners, designer and the Dutchman's margin
// on the first visit. The port alert points there when the wholesaler's
// door opens or once the lead has said the one city's corners have a
// ceiling (#446's road hint, World.Progression.Ceiling), while the port
// is untouched, and names the facts: the
// free corners, the dearest thing listed there at its price, the
// wholesaler and his share of street. Not before either line, and not
// once you work, ship, buy, hold stock or stand there.
func TestPortAlert(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	port := func(w *game.World) string {
		for _, cid := range w.CityOrder {
			if cid != w.Home().ID && w.WholesaleSupplier(cid) != nil {
				return cid
			}
		}
		t.Fatal("no port in the file")
		return ""
	}
	door := func(w *game.World) { w.Stats.PeakCash = w.WholesaleSupplier(port(w)).UnlockCash }
	ceiling := func(w *game.World) { w.Reach(2, 1); w.Reach(3, 2); w.Progression.Ceiling = 40 }
	for _, c := range []struct {
		name string
		set  func(w *game.World)
		want bool
	}{
		{"neither line", func(w *game.World) {}, false},
		{"the wholesaler's door open", door, true},
		{"the ceiling said at Territory", ceiling, true},
		{"at Territory, no ceiling said", func(w *game.World) { w.Reach(2, 1); w.Reach(3, 2) }, false},
		{"a route on", func(w *game.World) {
			door(w)
			for id := range w.Routes {
				w.Routes[id] = game.RouteSetting{Dial: events.RouteNormal}
			}
			if len(w.Routes) == 0 {
				w.Routes = map[string]game.RouteSetting{cfg.Routes.Routes[0].ID: {Dial: events.RouteNormal}}
			}
		}, false},
		{"standing in the port", func(w *game.World) { door(w); w.Player.Location = port(w) }, false},
		{"stock in the port", func(w *game.World) { door(w); w.AddStock(port(w), w.Products[0], 5, 0) }, false},
		{"a corner there once held", func(w *game.World) { door(w); w.Cities[port(w)].Corners[0].Yours = true }, false},
		{"bought from the wholesaler", func(w *game.World) { door(w); w.WholesaleSupplier(port(w)).Bought = 100 }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, w := crewRun(t)
			s.EndDay() // the connects' prices stamped
			c.set(w)
			got := ofKind(s, engine.AlertPort)
			if !c.want {
				if len(got) != 0 {
					t.Fatalf("a port alert: %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("port alerts %+v, want one", got)
			}
			a, far := got[0], port(w)
			free := 0
			for _, k := range w.Cities[far].Corners {
				if k.Owner == game.OwnerNone {
					free++
				}
			}
			if a.City != far || a.Count != free || a.Supplier != w.WholesaleSupplier(far).ID || a.Key == "" {
				t.Fatalf("the alert %+v, want %s with %d free corners", a, far, free)
			}
			if m := w.Product(far, a.Product); m == nil || a.Amount <= 0 {
				t.Fatalf("the alert names no product priced in %s: %+v", far, a)
			}
			if a.Share <= 0 || a.Share >= 1 {
				t.Fatalf("the wholesaler's share of street is %v", a.Share)
			}
			if a.Act.Screen != engine.ScreenMap || a.Act.Subject != engine.SubjectCity {
				t.Fatalf("the alert opens %+v, not the map on the city", a.Act)
			}
			// Keyed once: the next morning's is the same key, so a
			// fast-forward stops on it once.
			if again := ofKind(s, engine.AlertPort); len(again) != 1 || again[0].Key != a.Key {
				t.Fatalf("the key moved: %+v then %+v", a, again)
			}
		})
	}
}
