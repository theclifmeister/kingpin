package logistics_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestRouteSaysWhyItIsIdle (#459): a playtest's routes read "shipped 0"
// for days with no word. A route on its dial that sends nothing says
// why: no dirty cash over the till, nothing at the source to send,
// the target met, no target, shut; the morning's RouteIdle carries the
// first two, the ones where it is short, and Idle, the map's, reads the
// same reason off the world before the step. A route that would send
// is not idle, and its dice do not move.
func TestRouteSaysWhyItIsIdle(t *testing.T) {
	cfg := content.MustLoad()
	coke := "coke"
	for _, c := range []struct {
		name  string
		set   func(w *game.World, r content.RouteConfig)
		why   string
		event bool
		sends bool
	}{
		{"off", func(w *game.World, r content.RouteConfig) { _ = w.SetRoute(r.ID, events.RouteOff) }, "", false, false},
		{"sends", func(w *game.World, r content.RouteConfig) {}, "", false, true},
		{"the till", func(w *game.World, r content.RouteConfig) {
			w.Player.DirtyCash = w.Float(cfg.Upgrades, cfg.Laundering.Laundering.Float)
		}, events.IdleTill, true, false},
		{"under the till", func(w *game.World, r content.RouteConfig) { w.Player.DirtyCash = 20_000 }, events.IdleTill, true, false},
		{"an empty stash", func(w *game.World, r content.RouteConfig) { w.SetStock(r.From, coke, 0) }, events.IdleStock, true, false},
		{"the target met", func(w *game.World, r content.RouteConfig) { w.SetStock(r.To, coke, 60) }, events.IdleMet, false, false},
		{"no target", func(w *game.World, r content.RouteConfig) { _ = w.SetRouteTarget(r.ID, coke, 0) }, events.IdleNoTarget, false, false},
		{"short of a lot", func(w *game.World, r content.RouteConfig) {
			// The wholesaler open, the stash empty, a hundred over the
			// till: not a lot's worth.
			w.SetStock(r.From, coke, 0)
			w.Stats.PeakCash = w.WholesaleSupplier(r.From).UnlockCash
			w.Player.DirtyCash = w.Float(cfg.Upgrades, cfg.Laundering.Laundering.Float) + 100
		}, events.IdleTill, true, false},
		{"shut", func(w *game.World, r content.RouteConfig) {
			rs := w.Route(r.ID)
			rs.ClosedUntil = w.Day + 3
			w.Routes[r.ID] = rs
		}, events.IdleClosed, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			w, s, r := world(t, cfg, 0)
			w.SetStock(r.From, w.Products[0], 0)
			w.SetStock(r.From, coke, 100)
			w.Player.DirtyCash = 100_000
			// The wholesaler stays shut (peak cash under their door), so
			// an empty stash is nothing to send.
			_ = w.SetRoute(r.ID, events.RouteFast)
			_ = w.SetRouteTarget(r.ID, coke, 60)
			c.set(w, r)
			if got := s.Idle(w, r); got != c.why {
				t.Fatalf("Idle %q, want %q", got, c.why)
			}
			var idle []events.RouteIdle
			for _, e := range step(w, s) {
				if ev, ok := e.(events.RouteIdle); ok {
					idle = append(idle, ev)
				}
			}
			if sent := len(w.Shipments) > 0; sent != c.sends {
				t.Fatalf("sent %v, want %v: %+v", sent, c.sends, w.Shipments)
			}
			switch {
			case c.event && (len(idle) != 1 || idle[0].Why != c.why || idle[0].Route != r.ID || len(idle[0].Products) != 1 || idle[0].Products[0] != coke):
				t.Fatalf("RouteIdle %+v, want one %q for %s", idle, c.why, coke)
			case !c.event && len(idle) != 0:
				t.Fatalf("RouteIdle %+v on a route that is not short or sent", idle)
			}
			if c.why == events.IdleTill && (idle[0].Till != w.Float(cfg.Upgrades, cfg.Laundering.Laundering.Float) || idle[0].Dirty != w.Player.DirtyCash) {
				t.Fatalf("RouteIdle %+v: the till or the cash", idle[0])
			}
		})
	}
}
