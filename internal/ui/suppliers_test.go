package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The connects in the grammar (#72): the market's SUPPLIERS block lists
// every connect in the city shown with their price for the product
// under the cursor; the arrows reach them off the bottom of the table
// (past the buyers where there are any) and leave off their top; while
// the cursor is on one the pane is the connect's sections, every line
// in paneTextW, the strip its title; turning the city puts the cursor
// back on the table; and it all fits at 80x24 and 120x40.
func TestSuppliersInTheGrammar(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		w := m.w
		m.Update(key("2"))
		view := stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "the market with the connects")
		if !strings.Contains(view, "SUPPLIERS · "+w.Home().Name) {
			t.Fatalf("%dx%d: no SUPPLIERS block:\n%s", sz[0], sz[1], view)
		}
		for _, sup := range m.supplierRows() {
			if !strings.Contains(view, sup.Name) {
				t.Fatalf("%dx%d: %s is not listed:\n%s", sz[0], sz[1], sup.Name, view)
			}
		}
		// Down off the table reaches the buyers, where the fixture has
		// any, then the connects, and the pane is the connect's.
		for i := 0; i < len(w.Products)+len(m.buyerRows()); i++ {
			m.Update(key("j"))
		}
		if !m.onSuppliers || m.supplierCursor != 0 {
			t.Fatalf("%dx%d: after walking off the table: on suppliers %v cursor %d (buyers %v)", sz[0], sz[1], m.onSuppliers, m.supplierCursor, m.onBuyers)
		}
		sup := m.selectedSupplier()
		if sup == nil {
			t.Fatal("no connect selected")
		}
		secs := m.marketDetails()
		if len(secs) < 2 || secs[0].title != strings.ToUpper(sup.Name)+" · "+strings.ToUpper(w.Home().Name) || secs[1].title != "RULES" {
			t.Fatalf("%dx%d: the pane on a connect: %+v", sz[0], sz[1], secs)
		}
		for _, sec := range secs {
			for _, l := range sec.lines {
				if lipgloss.Width(l) > paneTextW {
					t.Errorf("%dx%d: a pane line is %d wide, over %d: %q", sz[0], sz[1], lipgloss.Width(l), paneTextW, stripANSI(l))
				}
			}
		}
		view = stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "the market on a connect")
		if !strings.Contains(view, strings.ToUpper(sup.Name)) || !strings.Contains(view, "▸ "+sup.Name) {
			t.Fatalf("%dx%d: the connect is not marked and titled:\n%s", sz[0], sz[1], view)
		}
		// Down walks the connects, up off the first leaves them.
		m.Update(key("j"))
		if !m.onSuppliers || m.supplierCursor != min(1, len(m.supplierRows())-1) {
			t.Fatalf("down on the connects: cursor %d", m.supplierCursor)
		}
		for i := 0; i < 3; i++ {
			m.Update(key("k"))
		}
		if m.onSuppliers || m.onBuyers {
			t.Fatal("up off the first connect did not leave them")
		}
		for i := 0; i < len(w.Products)+len(m.buyerRows()); i++ {
			m.Update(key("j"))
		}
		m.Update(key("]"))
		if m.onSuppliers || m.city == w.Home().ID {
			t.Fatal("turning the city left the cursor on the connects")
		}
		if v := stripANSI(m.View()); !strings.Contains(v, "SUPPLIERS · "+w.CityName(m.city)) {
			t.Fatalf("the other city's connects:\n%s", v)
		}
		assertFits(t, m.View(), sz[0], sz[1], "the other city's connects")
	}
}

// The buy dialog's connect step (#72): at home, with one connect
// dealing, the dialog opens on the product step and its footer lists
// no back; in the port with the wholesaler's door open it opens on the
// connect step, the product step's footer lists back, shift+tab goes
// back to the connect step keeping the pick, and the title names the
// connect; the pay notch on the last step is off the footer where the
// connect gives no credit and on it where they do, c turns it, and a
// buy on credit goes on the book, not out of the till, the cart lists
// it as a credit line, the receipt returns off the book, and the
// dashboard carries the debt as a fact and, due tomorrow, as an alert
// that stops a fast-forward.
func TestBuyDialogConnectStep(t *testing.T) {
	m := richModelSeeded(t, 100, 30, 11)
	w := m.w
	home := w.Home().ID
	m.Update(key("1"))
	m.Update(key("b"))
	if m.mode != modeBuy || m.dlg.pick || m.dlg.paged {
		t.Fatalf("at home with one connect dealing: mode %v pick %v paged %v", m.mode, m.dlg.pick, m.dlg.paged)
	}
	footer := func() string {
		var keys []string
		for _, b := range m.modalFooter() {
			keys = append(keys, b.key+" "+b.label)
		}
		return strings.Join(keys, "  ")
	}
	if f := footer(); strings.Contains(f, "back") {
		t.Fatalf("the product step lists back with no connect step before it: %s", f)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, strings.ToUpper("BUY · "+w.Home().Name+" · "+w.StreetSupplier(home).Name)) {
		t.Fatalf("the title does not name the connect:\n%s", v)
	}
	m.Update(key("esc"))
	// The port: two connects dealing once the wholesaler's door is open.
	hub := w.CityOrder[1]
	whole := w.WholesaleSupplier(hub)
	w.Stats.PeakCash = max(w.Stats.PeakCash, whole.UnlockCash)
	w.Player.CarryLimit = 2_000 // room for a lot in the port, where the fixture's route stash sits
	if err := w.Travel(hub); err != nil {
		t.Fatal(err)
	}
	endDay(t, m)
	m.Update(key("enter"))
	m.Update(key("b"))
	if m.mode != modeBuy || !m.dlg.pick || !m.dlg.paged {
		t.Fatalf("in the port: mode %v pick %v paged %v", m.mode, m.dlg.pick, m.dlg.paged)
	}
	if f := footer(); strings.Contains(f, "back") || !strings.Contains(f, "↑↓ pick") || !strings.Contains(f, "enter next") {
		t.Fatalf("the connect step's footer: %s", f)
	}
	assertFits(t, m.View(), 100, 30, "the connect step")
	if v := stripANSI(m.View()); !strings.Contains(v, whole.Name) || !strings.Contains(v, w.StreetSupplier(hub).Name) || !strings.Contains(v, "connect") {
		t.Fatalf("the connect step:\n%s", v)
	}
	// Pick the wholesaler: the product step names them, lists back, and
	// shift+tab goes back with the pick kept.
	for i, sup := range m.connectsHere() {
		if sup.ID == whole.ID {
			m.dlg.supplier = i
		}
	}
	m.Update(key("enter"))
	if m.dlg.pick || m.dlg.step != 0 || m.modalStep() != 1 {
		t.Fatalf("after the pick: pick %v step %d page %d err %q", m.dlg.pick, m.dlg.step, m.modalStep(), m.dlg.err)
	}
	if f := footer(); !strings.Contains(f, "⇧tab back") {
		t.Fatalf("the product step after a connect step lists no back: %s", f)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, strings.ToUpper("BUY · "+w.CityName(hub)+" · "+whole.Name)) {
		t.Fatalf("the title after the pick:\n%s", v)
	}
	picked := m.dlg.supplier
	m.Update(key("shift+tab"))
	if !m.dlg.pick || m.dlg.supplier != picked {
		t.Fatalf("back from the product step: pick %v supplier %d (was %d)", m.dlg.pick, m.dlg.supplier, picked)
	}
	m.Update(key("enter"))
	// A whole lot on credit: the pay notch is listed, c turns it, the
	// quantity is the lot, enter puts it on the book.
	weed := w.Products[0]
	for i, id := range w.Products {
		if id == weed {
			m.cursor = i
		}
	}
	m.Update(key("enter"))
	m.dlg.qty.SetValue(fmt.Sprint(whole.Lot))
	m.Update(key("enter"))
	if m.dlg.step != 2 {
		t.Fatalf("after the quantity: step %d err %q", m.dlg.step, m.dlg.err)
	}
	if f := footer(); !strings.Contains(f, "c pay") {
		t.Fatalf("the last step's footer lists no pay notch with credit on offer: %s", f)
	}
	m.Update(key("c"))
	if !m.dlg.credit {
		t.Fatal("c did not turn the pay notch to credit")
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "[credit]") || !strings.Contains(v, "due day") {
		t.Fatalf("the last step on credit:\n%s", v)
	}
	cash, stock := w.Player.DirtyCash, w.Stock(hub, weed)
	m.Update(key("enter"))
	if m.dlg.err != "" || whole.Debt == 0 || w.Player.DirtyCash != cash || w.Stock(hub, weed) != stock+whole.Lot {
		t.Fatalf("a credit lot: err %q debt %d cash %d -> %d stock %d -> %d", m.dlg.err, whole.Debt, cash, w.Player.DirtyCash, stock, w.Stock(hub, weed))
	}
	if want := whole.Quote(weed, whole.Lot, true); whole.Debt != want {
		t.Fatalf("the book holds %d, want the credit quote %d", whole.Debt, want)
	}
	if !strings.Contains(m.status, "on credit") || !strings.Contains(m.status, "due day") {
		t.Fatalf("status %q", m.status)
	}
	// The cart: a credit line, returned off the book.
	m.Update(key("esc"))
	line := -1
	for i, l := range m.cartLines() {
		if l.credit && l.qty == whole.Lot {
			line = i
		}
	}
	if line < 0 {
		t.Fatalf("the cart has no credit line: %+v", m.cartLines())
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "credit") {
		t.Fatalf("the pane's cart does not mark the credit line:\n%s", v)
	}
	m.Update(key("c"))
	if m.mode != modeCart {
		t.Fatalf("c opened %v", m.mode)
	}
	m.crt.cursor = line
	m.Update(key("x"))
	if whole.Debt != 0 || w.Stock(hub, weed) != stock || w.Player.DirtyCash != cash || !strings.Contains(m.status, "off the book") {
		t.Fatalf("returning the credit line: debt %d stock %d cash %d status %q", whole.Debt, w.Stock(hub, weed), w.Player.DirtyCash, m.status)
	}
	m.Update(key("esc"))
	// A connect who gives no credit: the notch is not listed.
	whole.CreditDays = 0
	m.Update(key("b"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.dlg.qty.SetValue(fmt.Sprint(whole.Lot))
	m.Update(key("enter"))
	if f := footer(); strings.Contains(f, "c pay") {
		t.Fatalf("the pay notch is listed with no credit on offer: %s", f)
	}
	m.Update(key("c"))
	if m.dlg.credit {
		t.Fatal("c turned the notch with no credit on offer")
	}
	m.Update(key("esc"))
	whole.CreditDays = 10
	// The debt as a fact and an alert: due tomorrow stops a fast-forward
	// after one day, and the alert names the connect.
	whole.Debt, whole.DebtDue = 5_000, w.Day+2
	m.Update(key("1"))
	if v := stripANSI(m.View()); !strings.Contains(v, "owe $5,000") {
		t.Fatalf("the street does not carry the debt:\n%s", v)
	}
	if len(m.debtAlerts()) != 0 {
		t.Fatalf("an alert two days out: %+v", m.debtAlerts())
	}
	day := w.Day
	fast(t, m, 30)
	if w.Day != day+1 {
		t.Fatalf("F ran %d days past a debt due tomorrow", w.Day-day)
	}
	if m.mode == modeCard {
		// A card on the fixture's morning: the card is the stop's
		// reason, and the alert is still the morning's.
		m.Update(key("enter"))
		m.Update(key("enter"))
	} else if got, want := reportLine(t, m), "Stopped after 1 day: debt due tomorrow."; got != want {
		t.Fatalf("the report opens with %q, want %q", got, want)
	}
	closeMorning(t, m)
	if a := m.debtAlerts(); len(a) != 1 || !strings.Contains(stripANSI(a[0].text), whole.Name) || a[0].why != "debt due tomorrow" {
		t.Fatalf("the alert: %+v", a)
	}
	// The morning it is due it is paid: the market collects it at the
	// top of the day, and the report says so.
	m.Update(key("n"))
	if m.mode == modeCard {
		m.Update(key("enter"))
		m.Update(key("enter"))
	}
	if whole.Debt != 0 || w.Stats.Repaid != 5_000 {
		t.Fatalf("on the day: debt %d, repaid %d", whole.Debt, w.Stats.Repaid)
	}
	paid := false
	for _, l := range w.Report.Money {
		paid = paid || strings.Contains(l, "Paid "+whole.Name)
	}
	if !paid {
		t.Fatalf("the report's money lines: %v", w.Report.Money)
	}
}

// A connect who is not dealing is refused at the door with the reason,
// and the connect step refuses one who will not deal while the other
// sells (#72).
func TestBuyRefusalsNameTheConnect(t *testing.T) {
	m := richModel(t, 100, 30)
	w := m.w
	home := w.Home().ID
	street := w.StreetSupplier(home)
	street.FrozenUntil = w.Day + 3
	m.Update(key("1"))
	m.Update(key("b"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't buy: "+street.Name+" is not taking your calls") {
		t.Fatalf("frozen: mode %v status %q", m.mode, m.status)
	}
	street.FrozenUntil = 0
	street.Cap = street.BoughtToday
	m.Update(key("b"))
	if m.mode != modePlay || !strings.Contains(m.status, "nothing left today") {
		t.Fatalf("sold out: mode %v status %q", m.mode, m.status)
	}
	street.Cap = 10_000
	// A second connect at home who will not deal yet: the dialog skips
	// the connect step, and the market lists them as waiting.
	pool := (*game.Supplier)(nil)
	for i := range w.Suppliers {
		if s := &w.Suppliers[i]; s.City == home && s.UnlockRel > 0 {
			pool = s
		}
	}
	if pool != nil {
		m.Update(key("b"))
		if m.mode != modeBuy || m.dlg.pick {
			t.Fatalf("with a locked second connect: mode %v pick %v", m.mode, m.dlg.pick)
		}
		m.Update(key("esc"))
		m.Update(key("2"))
		if v := stripANSI(m.View()); !strings.Contains(v, pool.Name) || !strings.Contains(v, "won't deal yet") {
			t.Fatalf("the market on a locked connect:\n%s", v)
		}
	}
}
