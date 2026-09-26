package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestTillFromTheLedger (#496): T on the ledger opens the till, the
// dirty cash the wash leaves in hand; enter sets the amount typed, and
// blank sets it back to the float. It says what the wash would take on
// the cash in hand and fits at every size.
func TestTillFromTheLedger(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := newTestModel(t, sz[0], sz[1])
		w := m.w
		w.Player.DirtyCash = 200_000
		w.Fronts = append(w.Fronts, game.Front{ID: "laundromat", Name: "Laundromat"})
		m.Update(key("7"))
		m.Update(key("T"))
		if m.mode != modeTill {
			t.Fatalf("T on the ledger: mode %v, status %q", m.mode, m.status)
		}
		for _, k := range []string{"8", "0", "0", "0", "0"} {
			m.Update(key(k))
		}
		view := stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "till")
		for _, want := range []string{"THE TILL", "the wash takes up to"} {
			if !strings.Contains(squash(view), want) {
				t.Errorf("%dx%d: the till dialog lacks %q:\n%s", sz[0], sz[1], want, view)
			}
		}
		m.Update(key("enter"))
		if w.Laundering.Till != 80_000 || m.rules.Laundering.Till(w) != 80_000 || m.till() != 80_000 {
			t.Fatalf("%dx%d: till %d (rules %d), want 80,000: %q", sz[0], sz[1], w.Laundering.Till, m.rules.Laundering.Till(w), m.amt.err)
		}
		if !strings.Contains(m.status, "$80,000") {
			t.Errorf("%dx%d: the status %q does not say the till", sz[0], sz[1], m.status)
		}
		m.Update(key("T"))
		m.amt.SetValue("")
		m.Update(key("enter"))
		if w.Laundering.Till != 0 || m.till() != m.floatLine() {
			t.Fatalf("%dx%d: blank left the till at %d", sz[0], sz[1], w.Laundering.Till)
		}
	}
}

// TestLieLowAsksOverAHandoff (#503): a playtest queued a handoff, lay low
// and found it gone. With a handoff queued, l asks first and names it;
// any other key keeps dealing, y lies low, and the handoff stays queued
// (held tonight, the buyers' row says so) rather than dropped.
func TestLieLowAsksOverAHandoff(t *testing.T) {
	m := newTestModel(t, 160, 40) // wide enough for the buyers' row whole
	w := m.w
	home, weed := w.Player.Location, w.Products[0]
	w.SetStock(home, weed, 40)
	c := w.OfferContract(game.Contract{Buyer: "x", Name: "Vera", City: home, Product: weed, Units: 30, Premium: 1.5, HeatMul: 0.4, Expires: w.Day + 2, Due: w.Day + 5})
	if err := w.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.Deliver(c.ID, 25); err != nil {
		t.Fatal(err)
	}
	m.Update(key("1"))
	m.Update(key("l"))
	if m.mode != modeConfirm || w.Today.LieLow {
		t.Fatalf("l over a queued handoff: mode %v, lying low %v", m.mode, w.Today.LieLow)
	}
	view := squash(stripANSI(m.View()))
	if !strings.Contains(view, "25 "+w.ProductName(weed)+" to Vera") {
		t.Errorf("the confirmation does not name the handoff:\n%s", view)
	}
	assertFits(t, m.View(), 160, 40, "lie low over a handoff")
	m.Update(key("n"))
	if w.Today.LieLow {
		t.Fatal("n lay low")
	}
	m.Update(key("l"))
	m.Update(key("y"))
	if !w.Today.LieLow || w.QueuedDelivery(c.ID) != 25 {
		t.Fatalf("y: lying low %v, queued %d; want lying low with the 25 kept", w.Today.LieLow, w.QueuedDelivery(c.ID))
	}
	if rows := strings.Join(m.buyersLines(), "\n"); !strings.Contains(stripANSI(rows), "25 held") {
		t.Errorf("the buyers' row does not say the handoff is held:\n%s", stripANSI(rows))
	}
	// Back on the corner: l at once, and the handoff still goes tonight.
	m.Update(key("l"))
	if w.Today.LieLow || m.mode != modePlay || w.QueuedDelivery(c.ID) != 25 {
		t.Fatalf("l back: lying low %v, mode %v, queued %d", w.Today.LieLow, m.mode, w.QueuedDelivery(c.ID))
	}
}

// TestStandingSizedForTheRoad (#503): standing orders were capped at
// the stash and the contract, so one could not be sized for goods
// arriving by road; and "blank = max" froze at the day's number. A
// standing order may be typed over the stash up to what lands tonight,
// once refuses the same number, and a blank quantity at standing is
// kept as all of the stash.
func TestStandingSizedForTheRoad(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home, weed := w.Player.Location, w.Products[0]
	w.SetStock(home, weed, 10)
	w.Shipments = []game.Shipment{{ID: 1, Route: "interstate", From: "bayport", To: home, Product: weed, Units: 50, Arrives: w.Day + 1}}
	m.Update(key("s"))
	m.Update(key("enter"))
	for _, k := range []string{"6", "0"} {
		m.Update(key(k))
	}
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "50 land tonight after the sales") {
		t.Errorf("the quantity step does not say what lands:\n%s", view)
	}
	m.Update(key("enter")) // the dial
	m.Update(key("enter")) // the repeat, at once
	m.Update(key("enter"))
	if _, ok := w.Order(home, weed); ok || !strings.Contains(m.dlg.err, "pick standing") {
		t.Fatalf("once over the stash: order %v, err %q", ok, m.dlg.err)
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("2"))
	m.Update(key("enter"))
	if o, ok := w.YourStanding(home, weed); !ok || o.Qty != 60 || o.All {
		t.Fatalf("standing over the stash: %+v %v, err %q", o, ok, m.dlg.err)
	}
	// Blank at standing is all of it, kept as all.
	m.Update(key("esc"))
	m.Update(key("s"))
	m.Update(key("enter"))
	m.dlg.qty.SetValue("")
	m.Update(key("enter"))
	m.Update(key("enter"))
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "all of the stash at normal nightly") {
		t.Errorf("the repeat step does not say all of the stash:\n%s", view)
	}
	m.Update(key("enter"))
	if o, ok := w.YourStanding(home, weed); !ok || !o.All {
		t.Fatalf("blank at standing: %+v %v, err %q", o, ok, m.dlg.err)
	}
	m.Update(key("esc"))
	if rows := stripANSI(strings.Join(m.standingRows(home, weed), "\n")); !strings.Contains(rows, "all normal") && !strings.Contains(rows, "all norm") {
		t.Errorf("the pane does not say the order is for all of it:\n%s", rows)
	}
}

// TestRouteTargetSaysWhatTheRouteDoes (#496): the target dialog said a
// route buys "by the lot", and a playtest's route idled on an empty
// stash. It says the route ships out of the source stash and buys lots
// only while the wholesaler deals.
func TestRouteTargetSaysWhatTheRouteDoes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		w := m.w
		r := m.mapRoutes()[0]
		sup := w.WholesaleSupplier(r.From)
		if sup == nil {
			t.Fatal("no wholesaler at the source")
		}
		m.Update(key("5"))
		m.onRoutes, m.routeCursor = true, 0
		m.Update(key("R"))
		if m.mode != modeTarget {
			t.Fatalf("R on a route: mode %v, status %q", m.mode, m.status)
		}
		view := squash(stripANSI(m.View()))
		assertFits(t, m.View(), sz[0], sz[1], "target")
		if want := "out of the " + w.CityName(r.From) + " stash"; !strings.Contains(view, want) {
			t.Errorf("%dx%d: the target dialog lacks %q:\n%s", sz[0], sz[1], want, view)
		}
		want := "It buys nothing until " + sup.Name
		if sup.Open(w) {
			want = "it buys by the lot from " + sup.Name
		}
		if !strings.Contains(view, want) {
			t.Errorf("%dx%d: the target dialog lacks %q:\n%s", sz[0], sz[1], want, view)
		}
	}
}

// TestLandedAndHoldingAreSaid (#503): route-landed stock with no order
// reads as an alert that opens the market on the city, and a contract
// held by the road says so in the pane.
func TestLandedAndHoldingAreSaid(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home, weed := w.Player.Location, w.Products[0]
	_ = w.SetRoute("interstate", events.RouteNormal)
	_ = w.SetRouteTarget("interstate", weed, 200)
	w.SetStock(home, weed, 120)
	var landed *engine.Alert
	for _, a := range m.sess.Alerts() {
		if a.Kind == engine.AlertLanded {
			landed = &a
		}
	}
	if landed == nil {
		t.Fatal("no landed alert")
	}
	if text := stripANSI(strings.Join(m.alertLines(120, 12), "\n")); !strings.Contains(squash(text), "120 "+w.ProductName(weed)+" landed in "+w.CityName(home)+" by the road") {
		t.Errorf("the alerts lack the landed stock:\n%s", text)
	}
	m.openAlert(*landed)
	if m.screen != screenMarket || m.city != home || w.Products[m.cursor] != weed {
		t.Fatalf("the landed alert opened screen %v city %q on %q", m.screen, m.city, w.Products[m.cursor])
	}
	if err := w.SetSupply(home, weed, 200); err != nil {
		t.Fatal(err)
	}
	w.Shipments = []game.Shipment{{ID: 1, Route: "interstate", From: "bayport", To: home, Product: weed, Units: 60, Arrives: w.Day + 3}}
	if rows := stripANSI(strings.Join(m.contractRows(home, weed), "\n")); !strings.Contains(rows, "holding: 60 on the road") {
		t.Errorf("the contract rows do not say the road holds it:\n%s", rows)
	}
}
