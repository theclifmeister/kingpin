package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The bought law (#42): $ on the ledger opens the bribe dialog (dirty
// cash wanted), the first page picks the chief or the DA, the second is
// a number field whose blank and m are the price, shift+tab goes back,
// esc closes from either, enter pays: the envelope is queued and the
// dirty cash gone. A corrupt chief takes it overnight: the report says
// so, the ledger's PAYOFFS block lists the deal under the cursor with
// its days left and the pane its section. $ on the map with the routes
// cursor on an edge buys the checkpoint after asking; on the grid it is
// silent, listed only on the routes.
func TestBribeKeys(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	w.Law.Chief.Personality, w.Law.Chief.Observed = "corrupt", true
	w.Law.DA.Stance = "moderate"
	price := m.set.Law.Bribes().ChiefPrice
	m.Update(key("7"))
	dirty := w.Player.DirtyCash
	w.Player.DirtyCash = 0
	m.Update(key("$"))
	if m.mode != modePlay || !strings.Contains(m.status, "dirty cash") {
		t.Fatalf("$ with no dirty cash: mode %v status %q", m.mode, m.status)
	}
	w.Player.DirtyCash = dirty
	m.Update(key("$"))
	if m.mode != modeBribe || m.modalStep() != 0 {
		t.Fatalf("$ on the ledger: mode %v step %d", m.mode, m.modalStep())
	}
	assertFits(t, m.View(), 80, 24, "bribe dialog")
	view := stripANSI(m.View())
	for _, want := range []string{"Chief " + w.Law.Chief.Name, "DA " + w.Law.DA.Name, "corrupt", "moderate", "enter next", money(price)} {
		if !strings.Contains(view, want) {
			t.Fatalf("bribe dialog lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("j"))
	if m.bribeTarget() != game.BribeDA {
		t.Fatalf("j did not pick the DA: %s", m.bribeTarget())
	}
	m.Update(key("1"))
	if m.bribeTarget() != game.BribeChief {
		t.Fatalf("1 did not pick the chief: %s", m.bribeTarget())
	}
	m.Update(key("enter"))
	if m.modalStep() != 1 {
		t.Fatalf("enter did not turn to the amount: step %d", m.modalStep())
	}
	assertFits(t, m.View(), 80, 24, "bribe amount")
	if view := stripANSI(m.View()); !strings.Contains(view, "enter pay") || !strings.Contains(view, "⇧tab back") {
		t.Fatalf("amount page footer:\n%s", view)
	}
	m.Update(key("m"))
	if got, _ := m.br.amt.Number(); got != price {
		t.Fatalf("m filled %d, want the price %d", got, price)
	}
	m.Update(key("shift+tab"))
	if m.modalStep() != 0 {
		t.Fatalf("shift+tab did not go back: step %d", m.modalStep())
	}
	m.Update(key("esc"))
	if m.mode != modePlay || len(w.Today.Bribes) != 0 || w.Player.DirtyCash != dirty {
		t.Fatal("esc paid something")
	}
	// Blank is the price.
	m.Update(key("$"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	if m.mode != modePlay || w.Player.DirtyCash != dirty-price || w.BribedToday(game.BribeChief) != price || !strings.Contains(m.status, "envelope") {
		t.Fatalf("enter: mode %v dirty %d bribed %d status %q", m.mode, w.Player.DirtyCash, w.BribedToday(game.BribeChief), m.status)
	}
	// One a day per target.
	m.Update(key("$"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	if m.mode != modeBribe || m.br.err == "" {
		t.Fatalf("a second envelope for the chief: mode %v err %q", m.mode, m.br.err)
	}
	m.Update(key("esc"))
	endDay(t, m)
	if m.mode != modeReport {
		t.Fatalf("mode %v", m.mode)
	}
	law := strings.Join(w.Report.Law, "\n")
	if !strings.Contains(law, "took the") || !w.Law.ChiefBoughtOn(w.Day) || w.Law.Leads != 1 {
		t.Fatalf("report %v law %+v", w.Report.Law, w.Law)
	}
	if !strings.Contains(strings.Join(w.Report.Money, "\n"), "Envelope") {
		t.Fatalf("money: %v", w.Report.Money)
	}
	assertFits(t, m.View(), 80, 24, "report with a bribe")
	m.Update(key("enter"))
	// The ledger's PAYOFFS block, under the cursor.
	view = stripANSI(m.View())
	if !strings.Contains(view, "PAYOFFS") || !strings.Contains(view, "Chief "+w.Law.Chief.Name) {
		t.Fatalf("ledger lacks the payoff:\n%s", view)
	}
	for m.ledgerSelected().kind != ledgerPayoff {
		before := m.ledgerCursor
		m.Update(key("down"))
		if m.ledgerCursor == before {
			t.Fatalf("the cursor never reached PAYOFFS:\n%s", stripANSI(m.View()))
		}
	}
	m.Update(key("120"[0:1]))
	m.Update(key("7"))
	if got := stripLine(m); !strings.HasPrefix(got, "▸ CHIEF "+strings.ToUpper(w.Law.Chief.Name)) {
		t.Fatalf("the strip does not name the payoff: %q", got)
	}
	m.Update(key("enter"))
	if m.mode != modeConfirmEnd {
		t.Fatalf("enter on a payoff row: mode %v (the frame's enter should ask to end the day)", m.mode)
	}
	m.Update(key("esc"))

	// The checkpoint, from the map's routes cursor.
	m.Update(key("5"))
	m.Update(key("]"))
	m.onRoutes = false
	m.status = ""
	m.Update(key("$"))
	if m.mode != modePlay || m.status != "" {
		t.Fatalf("$ on the grid should be silent (it is listed on the routes): mode %v status %q", m.mode, m.status)
	}
	m.onRoutes, m.routeCursor = true, 0
	r := *m.selectedRoute()
	if m.set.Logistics.Cut(w, r, w.Day) != 0 {
		t.Fatal("the route is cut before anything was bought")
	}
	m.Update(key("$"))
	if m.mode != modeConfirmCheckpoint {
		t.Fatalf("$ on a route: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "checkpoint confirmation")
	dirty = w.Player.DirtyCash
	m.Update(key("esc"))
	if m.mode != modePlay || w.Player.DirtyCash != dirty {
		t.Fatal("esc bought it")
	}
	m.Update(key("$"))
	m.Update(key("y"))
	until, live := w.Checkpoint(r.ID)
	if m.mode != modePlay || !live || until != w.Day+m.set.Law.Bribes().CheckpointDays || w.Player.DirtyCash != dirty-m.dealPrice(r) || !strings.Contains(m.status, "yours") {
		t.Fatalf("y: mode %v until %d live %v dirty %d status %q", m.mode, until, live, w.Player.DirtyCash, m.status)
	}
	// The fixture's routes carry no risk (captures are deterministic);
	// the cut is what the dice would take off (TestCheckpointCutsRisk).
	if got := m.set.Logistics.Cut(w, r, w.Day); got != m.set.Logistics.DealCut(r) || got <= 0 {
		t.Fatalf("the bought route's cut is %.2f", got)
	}
	m.Update(key("7"))
	if view := stripANSI(m.View()); !strings.Contains(view, r.Name+" ") || !strings.Contains(view, dealWord(r)) {
		t.Fatalf("PAYOFFS lacks the route:\n%s", view)
	}
}
