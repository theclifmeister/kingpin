package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The playtest's cursors (#462), order checks (#467) and crew refusals
// (#468): a picker opens where it says it does, one ▸ a screen, a
// quantity is checked where it is typed, and a refusal says why where
// the eye is.

// mainOf is MAIN's lines as text: the frame's left column, without the
// pane.
func mainOf(m *Model) string {
	main, _ := splitView(m)
	return stripANSI(strings.Join(main, "\n"))
}

// The buy and sell dialogs open on the product the dashboard and the
// market show selected, and name it in the title; from a screen with no
// product table they open on the first row, never on a selection
// another screen left behind (#462).
func TestDialogOpensWhereItSays(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	loc := w.Player.Location
	for _, id := range w.Products[:3] {
		w.SetStock(loc, id, 20)
	}
	m.Update(key("1"))
	m.cursor = 2
	m.Update(key("s"))
	if m.mode != modeSell || m.cursor != 2 {
		t.Fatalf("s on the dashboard: mode %v cursor %d", m.mode, m.cursor)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "SELL · "+strings.ToUpper(w.CityName(loc))+" · "+strings.ToUpper(w.ProductName(w.Products[2]))) {
		t.Fatalf("the title does not name the product:\n%s", v)
	}
	m.Update(key("esc"))
	// The journal has no product table: the dialog opens on the first
	// row, whatever the dashboard left selected.
	m.Update(key("3"))
	m.Update(key("s"))
	if m.mode != modeSell || m.cursor != 0 {
		t.Fatalf("s on the journal: mode %v cursor %d, want the first row", m.mode, m.cursor)
	}
	m.Update(key("esc"))
	m.cursor = 2
	m.Update(key("b"))
	if m.dlg.pick {
		m.Update(key("enter"))
	}
	if m.mode != modeBuy || m.cursor != 0 {
		t.Fatalf("b on the journal: mode %v cursor %d, want the first row (%q)", m.mode, m.cursor, m.status)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, " · "+strings.ToUpper(w.ProductName(w.Products[0]))) {
		t.Fatalf("the buy title does not name the product:\n%s", v)
	}
}

// One ▸ a screen (#462): on the market the product keeps the unfocused
// mark while the arrows are on the buyers or the connects, and ↓ and ↑
// walk every row in order: the products, the buyers, the connects.
func TestMarketHasOneCursor(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Contracts = append(w.Contracts, game.Contract{ID: 99, City: w.Player.Location, Name: "Test buyer", Product: w.Products[0], Units: 10, Status: game.ContractOffered, Expires: w.Day + 5})
	m.Update(key("2"))
	if len(m.buyerRows()) == 0 || len(m.supplierRows()) == 0 {
		t.Fatalf("the market has %d buyers and %d connects", len(m.buyerRows()), len(m.supplierRows()))
	}
	one := func(where string) {
		t.Helper()
		if n := strings.Count(mainOf(m), "▸"); n != 1 {
			t.Errorf("%s: %d ▸ on the market:\n%s", where, n, mainOf(m))
		}
	}
	one("on the products")
	last := len(w.Products) - 1
	for m.cursor < last {
		m.Update(key("down"))
	}
	m.Update(key("down"))
	if !m.onBuyers || m.buyerCursor != 0 {
		t.Fatalf("↓ off the last product: onBuyers %v onSuppliers %v", m.onBuyers, m.onSuppliers)
	}
	one("on the buyers")
	// The mark gutter may pad the mark (#473 keeps the tree's glyphs
	// beside it), so the product follows the mark after spaces.
	if !regexp.MustCompile(regexp.QuoteMeta(unfocusedMark) + ` +` + regexp.QuoteMeta(w.ProductName(w.Products[last]))).MatchString(mainOf(m)) {
		t.Errorf("the product lost its row's mark on the buyers:\n%s", mainOf(m))
	}
	for i := 1; i < len(m.buyerRows()); i++ {
		m.Update(key("down"))
	}
	m.Update(key("down"))
	if !m.onSuppliers || m.supplierCursor != 0 {
		t.Fatalf("↓ off the last buyer: onBuyers %v onSuppliers %v", m.onBuyers, m.onSuppliers)
	}
	one("on the connects")
	m.Update(key("up"))
	if !m.onBuyers {
		t.Fatalf("↑ off the first connect: onBuyers %v", m.onBuyers)
	}
	for m.onBuyers {
		m.Update(key("up"))
	}
	if m.onSuppliers || m.cursor != last {
		t.Fatalf("↑ off the first buyer: onSuppliers %v cursor %d", m.onSuppliers, m.cursor)
	}
}

// The rivals screen (#462): with offers on the table the ▸ is the
// offers' and the faction shown keeps the unfocused mark, and `↑↓ pick`
// is listed only where there are offers to walk ([ ] turn the faction).
func TestRivalsHaveOneCursor(t *testing.T) {
	m := tableModel(t, 120, 40)
	m.Update(key("8"))
	if n := strings.Count(mainOf(m), "▸"); n != 1 || !strings.Contains(mainOf(m), unfocusedMark) {
		t.Fatalf("%d ▸ on the rivals screen with an offer:\n%s", n, mainOf(m))
	}
	listed := func() bool {
		for _, b := range m.keysFor(screenRivals) {
			if b.key == "↑↓" {
				return true
			}
		}
		return false
	}
	if listed() {
		t.Errorf("↑↓ pick is listed with one offer to walk")
	}
	o := m.w.Offers[0]
	o.ID = 2
	m.w.Offers = append(m.w.Offers, o)
	if !listed() {
		t.Errorf("↑↓ pick is not listed with two offers")
	}
	m.Update(key("down"))
	if m.dealCursor != 1 {
		t.Errorf("↓ with two offers: the cursor is on %d", m.dealCursor)
	}
	m.w.Offers = nil
	if n := strings.Count(mainOf(m), "▸"); n != 1 {
		t.Errorf("%d ▸ on the rivals screen with no offer:\n%s", n, mainOf(m))
	}
}

// A quantity past what the sale can take is refused on the quantity
// step, the field set to what there is (#467), a standing order edited
// the same as a new one.
func TestSellQuantityRefusedWhereTyped(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	loc := w.Player.Location
	id := w.Products[3]
	w.ClearSupply(loc, id)
	w.SetStock(loc, id, 13)
	m.Update(key("1"))
	m.cursor = 3
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("4"))
	m.Update(key("0"))
	m.Update(key("enter"))
	if m.dlg.step != 1 || m.dlg.qty.Value() != "13" || !strings.Contains(m.dlg.err, "Only 13 "+w.ProductName(id)) {
		t.Fatalf("40 against 13: step %d qty %q err %q", m.dlg.step, m.dlg.qty.Value(), m.dlg.err)
	}
	// A standing order edited over the stash: refused on the same step.
	if err := w.PlaceStanding(loc, id, 5, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	w.SetStock(loc, id, 3)
	m.Update(key("esc"))
	m.Update(key("s"))
	m.Update(key("enter"))
	if m.dlg.repeat != repeatStanding {
		t.Fatalf("the standing order did not open at standing: %v", m.dlg.repeat)
	}
	m.Update(key("2"))
	m.Update(key("0"))
	m.Update(key("enter"))
	if m.dlg.step != 1 || !strings.Contains(m.dlg.err, "Only 3 ") {
		t.Fatalf("a standing order edited to 20 over 3: step %d err %q", m.dlg.step, m.dlg.err)
	}
}

// A contract set today counts for tonight's sales (#467), as the README
// says, a standing order against it included; where it brings nothing,
// the refusal says why rather than "none of that here".
func TestSameDayContractSells(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	loc := w.Player.Location
	id := w.Products[2]
	w.SetStock(loc, id, 0)
	w.Player.DirtyCash = 100_000
	if err := m.sess.SetSupply(loc, id, 20); err != nil {
		t.Fatal(err)
	}
	m.Update(key("1"))
	m.cursor = 2
	m.Update(key("s"))
	m.Update(key("enter"))
	if m.dlg.step != 1 {
		t.Fatalf("a sale against today's contract: step %d err %q", m.dlg.step, m.dlg.err)
	}
	m.Update(key("enter")) // blank: all the contract brings
	m.Update(key("enter")) // the dial
	m.Update(key("right")) // standing
	m.Update(key("enter"))
	if o, ok := w.YourStanding(loc, id); !ok || o.Qty != 20 {
		t.Fatalf("the standing order against today's contract: %+v %v, err %q", o, ok, m.dlg.err)
	}
	m.Update(key("esc"))
	w.CancelStanding(loc, id)
	w.Player.DirtyCash = 0
	m.Update(key("s"))
	m.Update(key("3")) // the product: the dialog lands on one held
	m.Update(key("enter"))
	if m.dlg.step != 0 || !strings.Contains(m.dlg.err, "the contract brings none tonight") || !strings.Contains(m.dlg.err, "cash") {
		t.Fatalf("a contract the cash cannot fill: step %d err %q", m.dlg.step, m.dlg.err)
	}
}

// A buy nothing in cash covers says what it costs and what the cash is,
// and points at the connect's book (#467), never "Nothing to do.".
func TestBuyShortSaysWhy(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("1"))
	m.Update(key("b"))
	if m.dlg.pick {
		m.Update(key("enter"))
	}
	// A product one unit of which the cash does not cover and the book
	// does.
	pick := -1
	w.Player.DirtyCash = 200
	for i, id := range w.Products {
		m.cursor = i
		if m.maxBuyBy(id, false) == 0 && m.maxBuyBy(id, true) > 0 {
			pick = i
			break
		}
	}
	if pick < 0 {
		t.Fatal("no product the book covers and the cash does not")
	}
	id := w.Products[pick]
	m.Update(key("enter")) // the product
	m.Update(key("enter")) // blank: the pay step takes it
	if m.dlg.step != 2 {
		t.Fatalf("blank with the book to cover it: step %d err %q", m.dlg.step, m.dlg.err)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "c puts it on") {
		t.Errorf("the pay step does not say the book covers it:\n%s", v)
	}
	m.Update(key("enter"))
	if m.dlg.err == "" || strings.Contains(m.dlg.err, "Nothing to do") || !strings.Contains(m.dlg.err, "one "+w.ProductName(id)) || !strings.Contains(m.dlg.err, "$200") || !strings.Contains(m.dlg.err, "c puts it on") {
		t.Fatalf("enter on cash that covers none: step %d err %q", m.dlg.step, m.dlg.err)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "BUY · ") || !strings.Contains(v, strings.ToUpper(w.ProductName(id))) {
		t.Errorf("the title does not name the product:\n%s", v)
	}
	// With no book either, the quantity step itself says why.
	w.Player.DirtyCash = 0
	sup := m.buySupplier(id)
	sup.Limit = 0
	sup.Debt = 0
	m.Update(key("esc"))
	m.cursor = pick
	m.Update(key("b"))
	if m.dlg.pick {
		m.Update(key("enter"))
	}
	m.Update(key("enter"))
	if strings.Contains(m.dlg.err, "Nothing to do") {
		t.Fatalf("still nothing to do: %q", m.dlg.err)
	}
}

// Keep at says when it buys (#467): at the end of the day, before the
// night's sales, not "each morning".
func TestKeepAtSaysWhenItBuys(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("1"))
	m.cursor = 0
	m.w.ClearSupply(m.w.Player.Location, m.w.Products[0])
	m.Update(key("b"))
	if m.dlg.pick {
		m.Update(key("enter"))
	}
	m.Update(key("enter"))
	m.Update(key("5"))
	m.Update(key("enter"))
	if v := stripANSI(m.View()); !strings.Contains(v, "every night, before the sales") {
		t.Fatalf("the once line does not say when keep at buys:\n%s", v)
	}
	m.Update(key("right"))
	if v := stripANSI(m.View()); !strings.Contains(v, "topped up nightly before the sales") {
		t.Fatalf("the contract row does not say when it buys:\n%s", v)
	}
}

// The post picker (#468): one laid up or in a cell is listed with why,
// never "idle", and the picker opens on the first who can work.
func TestPostPickerSaysWhoCannotWork(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	hurt := game.CrewMember{ID: 9, Name: "Hurt", Role: game.RoleEnforcer, Skill: 50, Loyalty: 70, Wage: 60, WoundedUntil: w.Day + 3}
	w.Crew.Members = append([]game.CrewMember{hurt}, w.Crew.Members...)
	m.Update(key("5"))
	for i, c := range m.shown().Corners {
		if c.Owner == game.OwnerPlayer && c.Enforcer == 0 {
			m.mapCursor = i
			break
		}
	}
	m.Update(key("e"))
	if m.mode != modePost {
		t.Fatalf("e on the map: mode %v status %q", m.mode, m.status)
	}
	rows := m.postRows(game.RoleEnforcer)
	if rows[0].ID != hurt.ID || m.pick.cursor == 0 {
		t.Fatalf("the picker opens on %d of %v", m.pick.cursor, rows)
	}
	v := stripANSI(m.View())
	if !strings.Contains(v, "laid up 3d") {
		t.Fatalf("the laid-up enforcer is not marked:\n%s", v)
	}
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "Hurt") && strings.Contains(l, "idle") {
			t.Fatalf("the laid-up enforcer reads idle: %q", l)
		}
	}
}

// A second scout the same night is refused and says why (#468: the
// playtest read it as nothing; the refusal was there).
func TestScoutTwiceSaysWhy(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("8"))
	m.Update(key("i"))
	m.Update(key("y"))
	if m.w.Today.Scouting == nil {
		t.Fatalf("the first scout: %q", m.status)
	}
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(stripANSI(m.View()), "Can't scout twice") {
		t.Fatalf("the second scout: mode %v status %q", m.mode, m.status)
	}
}

// Back from the other city, standing in it and working nothing, the
// idle corner's alert says how to work it yourself (#468: travel takes
// you off your corner).
func TestIdleCornerSaysWorkItYourself(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Recall(game.You)
	c := w.Here().Corners[1]
	text, _ := m.idleCornerAlert(engine.Alert{Kind: engine.AlertIdleCorner, City: w.Here().ID, Corner: c.ID, Days: 2})
	if want := fmt.Sprintf("Work it yourself: c %s, then You.", screenPointer(screenMap)); !strings.Contains(stripANSI(text), want) {
		t.Fatalf("the alert: %q, want %q", stripANSI(text), want)
	}
	if err := w.Post(c.ID, game.You); err != nil {
		t.Fatal(err)
	}
	text, _ = m.idleCornerAlert(engine.Alert{Kind: engine.AlertIdleCorner, City: w.Here().ID, Corner: w.Here().Corners[3].ID, Days: 2})
	if !strings.Contains(stripANSI(text), "Post a runner") {
		t.Fatalf("working a corner, the alert: %q", stripANSI(text))
	}
}

// A second chemist is paid and waits (#468): the roster says so rather
// than "cooking" beside the one who cooks.
func TestSecondChemistSaysSo(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Crew.Members = append(w.Crew.Members,
		game.CrewMember{ID: 20, Name: "Walt", Role: game.RoleChemist, Skill: 80, Loyalty: 70, Wage: 90},
		game.CrewMember{ID: 21, Name: "Jesse", Role: game.RoleChemist, Skill: 40, Loyalty: 70, Wage: 60},
	)
	if s, ok := m.post(w.Crew.Members[len(w.Crew.Members)-1]).(styled); !ok || s.v != "second chemist" {
		t.Fatalf("the second chemist's post: %#v", m.post(w.Crew.Members[len(w.Crew.Members)-1]))
	}
	if s, ok := m.post(w.Crew.Members[len(w.Crew.Members)-2]).(styled); !ok || s.v == "second chemist" {
		t.Fatalf("the best chemist's post: %#v", m.post(w.Crew.Members[len(w.Crew.Members)-2]))
	}
}
