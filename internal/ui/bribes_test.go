package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
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
	price := m.rules.Law.Bribes().ChiefPrice
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
	m.Update(key("1")) // a digit moves (#500): the chief, still on the first page
	if m.bribeTarget() != game.BribeChief || m.modalStep() != 0 {
		t.Fatalf("1 did not move to the chief and stay: %s, step %d", m.bribeTarget(), m.modalStep())
	}
	m.Update(key("enter"))
	if m.bribeTarget() != game.BribeChief || m.modalStep() != 1 {
		t.Fatalf("enter did not turn to the chief's amount: %s, step %d", m.bribeTarget(), m.modalStep())
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
	if m.sess.Sims().Logistics.Cut(w, r, w.Day) != 0 {
		t.Fatal("the route is cut before anything was bought")
	}
	m.Update(key("$"))
	if m.mode != modeConfirm {
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
	if m.mode != modePlay || !live || until != w.Day+m.rules.Law.Bribes().CheckpointDays || w.Player.DirtyCash != dirty-m.dealPrice(r) || !strings.Contains(m.status, "yours") {
		t.Fatalf("y: mode %v until %d live %v dirty %d status %q", m.mode, until, live, w.Player.DirtyCash, m.status)
	}
	// The fixture's routes carry no risk (captures are deterministic);
	// the cut is what the dice would take off (TestCheckpointCutsRisk).
	if got := m.sess.Sims().Logistics.Cut(w, r, w.Day); got != m.sess.Sims().Logistics.DealCut(r) || got <= 0 {
		t.Fatalf("the bought route's cut is %.2f", got)
	}
	m.Update(key("7"))
	if view := stripANSI(m.View()); !strings.Contains(view, r.Name+" ") || !strings.Contains(view, m.dealWord(r)) {
		t.Fatalf("PAYOFFS lacks the route:\n%s", view)
	}
}

// The favour (#228): v on the ledger is refused with why while the
// chief owes nothing, nothing is due or the officials are cold; with a
// favour owed and a raid due it asks, y makes the call (the favour
// spent, the day stamped, a second call refused), the LAW panel reads
// `owes you one` until then and the ALERTS carry the call; the night
// falls through: the report says so, a fast-forward stops on it, and
// the file grew by a page while the heat did not drop.
func TestFavourKeys(t *testing.T) {
	m := richModel(t, 100, 30)
	w := m.w
	w.Law.Chief.Personality, w.Law.Chief.Observed = "corrupt", true
	w.Law.Chief.Name = "Kerr" // short enough for the panel to carry the debt at 100 columns
	w.Law.DA.Stance = "moderate"
	w.Law.ChiefBought = w.Day + 30
	w.Law.ChiefShare = 1
	m.Update(key("7"))
	m.Update(key("v"))
	if m.mode != modePlay || !strings.Contains(m.status, "owes you nothing") {
		t.Fatalf("v with no favour: mode %v status %q", m.mode, m.status)
	}
	w.Law.Favours = 1
	m.Update(key("v"))
	if m.mode != modePlay || !strings.Contains(m.status, "nothing is coming tonight") {
		t.Fatalf("v with nothing due: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("1"))
	if view := stripANSI(m.View()); !strings.Contains(view, "owes one") {
		t.Fatalf("the LAW panel does not say the chief owes you one:\n%s", view)
	}
	raid := m.sess.Sims().Heat.Thresholds()[2]
	w.Here().Heat = (m.sess.Sims().Heat.Threshold(w, raid, w.Here()) + 1) / (1 - m.sess.Sims().Heat.Decay(w))
	if due := m.rules.Heat.Due(w); due != "raid" {
		t.Fatalf("due %q on a raid's heat", due)
	}
	alerted := false
	for _, a := range m.alerts() {
		if strings.HasPrefix(a.key, "the favour") {
			alerted = true
		}
	}
	if !alerted {
		t.Fatalf("no alert for the favour: %+v", m.alerts())
	}
	m.Update(key("7"))
	m.Update(key("v"))
	if m.mode != modeConfirm {
		t.Fatalf("v with a raid due: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 100, 30, "favour confirm")
	if view := stripANSI(m.View()); !strings.Contains(view, "CALL IN THE FAVOUR?") || !strings.Contains(view, "raid due") || !strings.Contains(view, "call favour") {
		t.Fatalf("the confirmation:\n%s", view)
	}
	// What the rung would take (#479), off the ladder the POLICE
	// section reads.
	if view := stripANSI(m.View()); !strings.Contains(view, "What it saves: the raid would take") || !strings.Contains(view, "of the stock") {
		t.Fatalf("the confirmation does not say what the raid would take:\n%s", view)
	}
	m.Update(key("y"))
	if m.mode != modePlay || w.Law.Favours != 0 || !w.FavourCalled() || w.Stats.Favours != 1 {
		t.Fatalf("y: mode %v law %+v", m.mode, w.Law)
	}
	m.Update(key("v"))
	if !strings.Contains(m.status, "The call is made") {
		t.Fatalf("v after the call: %q", m.status)
	}
	file, heat, day := w.Heat.Evidence, w.Here().Heat, w.Day
	w.Today.LieLow = false
	fast(t, m, 5)
	if w.Day != day+1 { // the morning stops on the fall-through, or on a card or an unlock the fixture's run has that morning, which outrank it
		t.Fatalf("F did not stop on the favour's morning: %q on day %d", m.fastStop, w.Day)
	}
	if why := m.stopEvent(events.RaidFellThrough{Day: w.Day, City: w.Here().ID, Level: content.Raid}); why != "the raid fell through" {
		t.Fatalf("the stop reads %q", why)
	}
	// The night, read before the morning's card is answered (#292): a
	// card's choice can move the heat, and it is not the favour's.
	if rep := strings.Join(w.Report.Heat, "\n"); !strings.Contains(rep, "fell through") || !strings.Contains(rep, "the file grows by 1") {
		t.Fatalf("the report's HEAT section:\n%s", rep)
	}
	if w.Heat.Evidence != file+m.rules.Law.Bribes().FavourEvidence || w.Here().Heat > heat || w.Stats.Raids != 0 || w.Stats.Stings != 0 {
		t.Fatalf("the night: file %d -> %d, heat %.1f -> %.1f, raids %d stings %d", file, w.Heat.Evidence, heat, w.Here().Heat, w.Stats.Raids, w.Stats.Stings)
	}
	closeMorning(t, m)
	// Under the cold nobody takes the call.
	w.Law.Favours, w.Law.DA.Stance = 1, "law_and_order"
	m.Update(key("esc"))
	m.Update(key("7"))
	m.Update(key("v"))
	if !strings.Contains(m.status, "law-and-order") {
		t.Fatalf("v under the cold: %q", m.status)
	}
}
