package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// offer puts a buyer's order on the table in the player's city for the
// tests: so many of the first product, due in a few days.
func offer(m *Model, units int, city string) game.Contract {
	w := m.w
	return w.OfferContract(game.Contract{
		Buyer: "club_owner", Name: "Marco at the Velvet Room", Pitch: "Marco wants some.",
		City: city, Product: w.Products[0], Units: units, Premium: 1.5, Penalty: 6, PenaltyCash: 0.2, HeatMul: 0.3,
		Street: w.Product(city, w.Products[0]).Price, Since: w.Day, Expires: w.Day + 2, Due: w.Day + 4,
	})
}

// toBuyers walks the market's cursor down the product table and off its
// bottom onto the first buyer.
func toBuyers(t *testing.T, m *Model) {
	t.Helper()
	for i := 0; i <= len(m.w.Products) && !m.onBuyers; i++ {
		m.Update(key("down"))
	}
	if !m.onBuyers || m.buyerCursor != 0 {
		t.Fatalf("the arrows did not reach the buyers: on %v cursor %d", m.onBuyers, m.buyerCursor)
	}
}

// The market screen's buyers: the arrows reach them off the bottom of
// the product table, a accepts, d queues the handoff out of the stash
// here, x declines; and on the product table x still cancels an order
// and d still turns the launder dial.
func TestMarketBuyersKeys(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	here := w.Player.Location
	w.SetStock(here, w.Products[0], 60)
	c := offer(m, 40, here)
	elsewhere := offer(m, 10, w.CityOrder[1])
	m.Update(key("2"))
	// Down through the products and off the bottom onto the buyers.
	toBuyers(t, m)
	m.Update(key("a"))
	if got := w.Contract(c.ID).Status; got != game.ContractAccepted {
		t.Fatalf("a did not accept: %v (%s)", got, m.status)
	}
	m.Update(key("d"))
	if q := w.QueuedDelivery(c.ID); q != 40 {
		t.Fatalf("d queued %d, want 40 (%s)", q, m.status)
	}
	// The other city's offer is on its own market: turn to it and decline.
	m.Update(key("right"))
	if m.onBuyers {
		t.Fatal("turning the city kept the buyers cursor")
	}
	toBuyers(t, m)
	m.Update(key("x"))
	if got := w.Contract(elsewhere.ID).Status; got != game.ContractDeclined {
		t.Fatalf("x did not decline: %v (%s)", got, m.status)
	}
	m.Update(key("left"))
	// Back on the product table, x cancels the order; d and a say to
	// pick a buyer and never turn the launder dial (#460).
	if err := w.PlaceSell(here, w.Products[0], 10, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	m.cursor = 0
	m.Update(key("x"))
	if _, ok := w.Order(here, w.Products[0]); ok {
		t.Fatal("x on the product table did not cancel the order")
	}
	dial := w.Laundering.Dial
	m.Update(key("d"))
	if w.Laundering.Dial != dial || !strings.Contains(m.status, "Pick a buyer to deliver to") {
		t.Fatalf("d on the product table: dial %v (was %v), status %q", w.Laundering.Dial, dial, m.status)
	}
	m.Update(key("a"))
	if m.mode != modePlay || !strings.Contains(m.status, "Pick a buyer to accept") {
		t.Fatalf("a on the product table: mode %v, status %q", m.mode, m.status)
	}
	// Up off the first buyer lands back on the table.
	toBuyers(t, m)
	m.Update(key("up"))
	if m.onBuyers {
		t.Fatal("up off the first buyer stayed on the buyers")
	}
	// The night: the handoff pays at street times the premium.
	m.Update(key("down"))
	m.Update(key("d"))
	cash := w.Player.DirtyCash
	street := w.Product(here, w.Products[0]).Price
	endDay(t, m)
	if got := w.Contract(c.ID); got == nil || got.Status != game.ContractDelivered || got.Delivered != 40 {
		t.Fatalf("the handoff did not resolve: %+v", got)
	}
	if want := int(street*1.5*40 + 0.5); w.Player.DirtyCash < cash+want-1 {
		t.Fatalf("paid %d, want about %d", w.Player.DirtyCash-cash, want)
	}
	if !strings.Contains(strings.Join(w.Report.Sales, "\n"), "Marco") {
		t.Fatalf("the report does not mention the handoff: %v", w.Report.Sales)
	}
}

// Every layout with buyers on it fits the common sizes: an offer, a
// contract with a handoff queued, the contract detail, the dashboard's
// line and the alert for one due tomorrow.
func TestBuyersRenderAtCommonSizes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := newTestModel(t, sz[0], sz[1])
		w := m.w
		here := w.Player.Location
		w.SetStock(here, w.Products[0], 60)
		c := offer(m, 40, here)
		offer(m, 25, here)
		m.Update(key("2"))
		assertFits(t, m.View(), sz[0], sz[1], "market with offers")
		toBuyers(t, m)
		assertFits(t, m.View(), sz[0], sz[1], "market on an offer")
		m.Update(key("a"))
		m.Update(key("d"))
		assertFits(t, m.View(), sz[0], sz[1], "market on a contract with a handoff queued")
		w.Contract(c.ID).Due = w.Day + 1
		m.Update(key("1"))
		view := m.View()
		assertFits(t, view, sz[0], sz[1], "dashboard with contracts")
		plain := stripANSI(view)
		if !strings.Contains(plain, "contract") || !strings.Contains(plain, "due tomorrow") {
			t.Errorf("%dx%d: the dashboard does not carry the contracts line: %q", sz[0], sz[1], plain)
		}
	}
}

// A handoff is what only you can do somewhere: d on the other city's
// market explains, and queues nothing.
func TestDeliverElsewhereIsRefused(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	other := w.CityOrder[1]
	w.SetStock(other, w.Products[0], 60)
	c := offer(m, 40, other)
	if err := w.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	m.Update(key("2"))
	m.Update(key("right"))
	toBuyers(t, m)
	m.Update(key("d"))
	if q := w.QueuedDelivery(c.ID); q != 0 || !strings.Contains(m.status, "go there") {
		t.Fatalf("queued %d with status %q", q, m.status)
	}
}

// An offer only in the other city (#440): the dashboard's pointer names
// the city, and the market, which opens on the city you stand in, says
// where the offer is instead of that nobody is asking.
func TestOfferElsewhereNamesItsCity(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := newTestModel(t, sz[0], sz[1])
		w := m.w
		other := w.CityOrder[1]
		offer(m, 40, other)
		name := w.CityName(other)
		if got := stripANSI(m.contractsLine()); got != "1 offer in "+name+" on the market screen (2)" {
			t.Errorf("%dx%d: the dashboard's line is %q", sz[0], sz[1], got)
		}
		m.Update(key("2"))
		view := m.View()
		assertFits(t, view, sz[0], sz[1], "market with an offer elsewhere")
		if plain := stripANSI(view); !strings.Contains(plain, "Nobody is asking here. 1 offer in "+name) {
			t.Errorf("%dx%d: the market does not point at the offer in %s: %q", sz[0], sz[1], name, plain)
		}
		// One here as well: the line counts both and names the one away.
		offer(m, 10, w.Player.Location)
		if got := stripANSI(m.contractsLine()); got != "2 offers (1 in "+name+") on the market screen (2)" {
			t.Errorf("%dx%d: with one here, the dashboard's line is %q", sz[0], sz[1], got)
		}
	}
}

// TestMarketKeysNeverTurnTheLaunderDial (#460): the market's d is
// deliver wherever its cursor is: off the buyers it says to pick one
// and leaves the launder dial alone, as a and o say what to pick
// rather than nothing; on the ledger d is still the dial.
func TestMarketKeysNeverTurnTheLaunderDial(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	here := w.Player.Location
	m.Update(key("2"))
	dial := w.Laundering.Dial
	checkDial := func(what string) {
		t.Helper()
		if w.Laundering.Dial != dial || m.mode != modePlay || strings.Contains(m.status, "aunder") {
			t.Fatalf("%s: dial %v (was %v), mode %v, status %q", what, w.Laundering.Dial, dial, m.mode, m.status)
		}
	}
	m.Update(key("d"))
	checkDial("d with no buyer")
	if !strings.Contains(m.status, "Nobody to deliver to in "+w.CityName(here)) {
		t.Fatalf("d with no buyer: %q", m.status)
	}
	offer(m, 10, here)
	regions := []struct {
		name              string
		buyers, suppliers bool
	}{{"the product table", false, false}, {"the connects", false, true}}
	for _, r := range regions {
		m.onBuyers, m.onSuppliers = r.buyers, r.suppliers
		for _, k := range []string{"d", "a"} {
			m.Update(key(k))
			checkDial(k + " on " + r.name)
			if !strings.Contains(m.status, "Pick a buyer to ") || !strings.Contains(m.status, "the cursor is on "+r.name) {
				t.Fatalf("%s on %s: %q", k, r.name, m.status)
			}
		}
	}
	m.onBuyers, m.onSuppliers = true, false
	for _, k := range []string{"o", "%"} {
		m.Update(key(k))
		if m.mode != modePlay || !strings.Contains(m.status, "Pick a product to ") || !strings.Contains(m.status, "the cursor is on the buyers") {
			t.Fatalf("%s on the buyers: mode %v status %q", k, m.mode, m.status)
		}
	}
	m.Update(key("7"))
	m.Update(key("d"))
	if w.Laundering.Dial == dial {
		t.Fatalf("d on the ledger did not turn the dial (%q)", m.status)
	}
}
