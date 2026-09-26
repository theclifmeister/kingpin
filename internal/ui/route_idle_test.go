package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// TestIdleRouteSaysWhy (#459): a playtest's routes read "shipped 0" for
// days with no word, the till holding the cash. A route on its dial
// that would send nothing says why on its map row (the long words
// where they fit, the short where they do not), in its pane, and on the
// ledger under LOGISTICS; one that would send says nothing of the sort.
// Since #496 the fare of stock already stashed is committed and the
// float pays it, so the till holds a route only where it has to buy a
// lot: the source stash is empty and the wholesaler open.
func TestIdleRouteSaysWhy(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		w := m.w
		r := m.mapRoutes()[0]
		for _, id := range w.Products {
			_ = w.SetRouteTarget(r.ID, id, 0)
		}
		_ = w.SetRoute(r.ID, events.RouteNormal)
		_ = w.SetRouteTarget(r.ID, "coke", w.Stock(r.To, "coke")+40)
		w.SetStock(r.From, "coke", 0)
		sup := w.WholesaleSupplier(r.From)
		if sup == nil {
			t.Fatal("no wholesaler at the source")
		}
		w.Stats.PeakCash = max(w.Stats.PeakCash, sup.UnlockCash)
		w.Player.DirtyCash = m.till()
		m.Update(key("5"))
		m.onRoutes, m.routeCursor = true, 0
		main := stripANSI(mainText(m))
		long, short := "idle: no dirty cash over the "+cash(m.till())+" till", "idle: till"
		if !strings.Contains(main, long) && !strings.Contains(main, short) {
			t.Errorf("%dx%d: the map's route row does not say it is idle:\n%s", sz[0], sz[1], main)
		}
		if pane := paneProse(m); !strings.Contains(pane, "no dirty cash over the "+cash(m.till())+" till") {
			t.Errorf("%dx%d: the route's pane does not say why:\n%s", sz[0], sz[1], pane)
		}
		m.Update(key("7"))
		if view := stripANSI(m.View()); !strings.Contains(view, "idle: no dirty cash") {
			t.Errorf("%dx%d: the ledger does not say the route is idle:\n%s", sz[0], sz[1], view)
		}
		// Cash over the till for a lot: it sends, and nothing says idle.
		w.Player.DirtyCash = m.till() + 10_000_000
		m.Update(key("5"))
		m.onRoutes, m.routeCursor = true, 0
		if main := stripANSI(mainText(m)); strings.Contains(main, "idle:") {
			t.Errorf("%dx%d: a route that would send reads idle:\n%s", sz[0], sz[1], main)
		}
		// Stock at the source and the pile at the till: the float pays
		// its fare (#496), and nothing says idle.
		w.SetStock(r.From, "coke", 100)
		w.Player.DirtyCash = m.till()
		if main := stripANSI(mainText(m)); strings.Contains(main, "idle:") {
			t.Errorf("%dx%d: a route with its stock stashed reads idle at the till:\n%s", sz[0], sz[1], main)
		}
		// Nothing at the source and nobody there selling: the stash.
		w.SetStock(r.From, "coke", 0)
		w.Stats.PeakCash = min(w.Stats.PeakCash, sup.UnlockCash-1)
		if main := stripANSI(mainText(m)); !strings.Contains(main, "idle: nothing in the "+w.CityName(r.From)+" stash") && !strings.Contains(main, "idle: empty") {
			t.Errorf("%dx%d: an empty source does not say so:\n%s", sz[0], sz[1], main)
		}
	}
}
