package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The pickers and dialogs of #536: every key acts on what the player
// looks at, and a max says what the pay row allows.

// qtyRow is whether a dialog's view shows the quantity row at n of max.
func qtyRow(view string, n, max int) bool {
	return regexp.MustCompile(fmt.Sprintf(`quantity +> ?%d +/ %d max`, n, max)).MatchString(view)
}

// typeQty types a number into the open dialog's field, digit by digit.
func typeQty(m *Model, n int) {
	for _, k := range strconv.Itoa(n) {
		m.Update(key(string(k)))
	}
}

// The buy's max follows the pay row (#536): a number over what the cash
// buys is held to it on the quantity step without eating the enter, and
// turning the pay to credit (or the repeat to keep at) sets the
// quantity to as much of what was typed as the new max takes, the field
// reading the connect's book (or the stash's room) as its max.
func TestBuyMaxFollowsThePay(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("1"))
	m.Update(key("b"))
	if m.dlg.pick {
		m.Update(key("enter"))
	}
	// A product the cash buys some of and the book more.
	pick, cashMax, book := -1, 0, 0
	for _, dirty := range []int{2_000, 5_000, 20_000, 50_000} {
		w.Player.DirtyCash = dirty
		for i, id := range w.Products {
			m.cursor = i
			if c, b := m.maxBuyBy(id, false), m.maxBuyBy(id, true); c > 0 && b > c+1 {
				pick, cashMax, book = i, c, b
				break
			}
		}
		if pick >= 0 {
			break
		}
	}
	if pick < 0 {
		t.Fatal("no product the book covers more of than the cash")
	}
	id := w.Products[pick]
	m.Update(key("enter")) // the product
	if m.dlg.step != 1 || m.qtyMax() != cashMax {
		t.Fatalf("the quantity step: step %d, max %d, want %d on cash", m.dlg.step, m.qtyMax(), cashMax)
	}
	asked := book + 5
	typeQty(m, asked)
	m.Update(key("enter"))
	if m.dlg.step != 2 || m.dlg.qty.Value() != strconv.Itoa(cashMax) || !strings.Contains(m.dlg.note, "what fits") || m.dlg.err != "" {
		t.Fatalf("%d over the cash's %d: step %d qty %q note %q err %q (the clamp may not eat the enter)", asked, cashMax, m.dlg.step, m.dlg.qty.Value(), m.dlg.note, m.dlg.err)
	}
	if v := stripANSI(m.View()); !qtyRow(v, cashMax, cashMax) || !strings.Contains(v, "what fits") {
		t.Fatalf("the pay step does not show the held number and why:\n%s", v)
	}
	m.Update(key("c"))
	if !m.dlg.credit || m.qtyMax() != book || m.dlg.qty.Value() != strconv.Itoa(book) {
		t.Fatalf("c: credit %v, max %d (want the book's %d), qty %q", m.dlg.credit, m.qtyMax(), book, m.dlg.qty.Value())
	}
	if v := stripANSI(m.View()); !qtyRow(v, book, book) {
		t.Fatalf("the quantity row does not follow the pay:\n%s", v)
	}
	m.Update(key("c"))
	if m.dlg.credit || m.dlg.qty.Value() != strconv.Itoa(cashMax) {
		t.Fatalf("c again: credit %v, qty %q, want the cash's %d", m.dlg.credit, m.dlg.qty.Value(), cashMax)
	}
	// Keep at: the level is the stash's room, not today's cash.
	m.Update(key("right"))
	if keep := m.keepMax(m.dialogCity(), id); m.dlg.repeat != repeatKeep || m.qtyMax() != keep || m.dlg.qty.Value() != strconv.Itoa(min(asked, keep)) {
		t.Fatalf("keep at: repeat %v, max %d (want %d), qty %q", m.dlg.repeat, m.qtyMax(), keep, m.dlg.qty.Value())
	}
	// On credit from the start: a number over the book is held to the
	// book on the quantity step, and the enter goes on.
	m.Update(key("left"))
	m.Update(key("c"))
	m.Update(key("shift+tab"))
	if m.dlg.step != 1 || !m.dlg.credit {
		t.Fatalf("shift+tab: step %d credit %v", m.dlg.step, m.dlg.credit)
	}
	typeQty(m, asked)
	m.Update(key("enter"))
	if m.dlg.step != 2 || m.dlg.qty.Value() != strconv.Itoa(book) || m.dlg.note == "" {
		t.Fatalf("%d over the book's %d: step %d qty %q note %q", asked, book, m.dlg.step, m.dlg.qty.Value(), m.dlg.note)
	}
	before := w.Stock(m.dialogCity(), id)
	m.Update(key("enter"))
	if got := w.Stock(m.dialogCity(), id); got != before+book {
		t.Fatalf("the held number bought %d, want %d (%q)", got-before, book, m.dlg.err)
	}
}

// Buy and sell open on one rule (#536): the row under the market's
// cursor where the key is pressed on its product table, else the first
// row, the same for both (a sale no longer jumps to the first thing
// held), and the route target on its first row.
func TestProductPickersOpenOnOneRule(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	for _, id := range w.Products {
		w.SetStock(home, id, 10)
	}
	w.SetStock(home, w.Products[0], 0) // the first row holds none
	w.Player.DirtyCash = 1_000_000
	first := m.firstProduct(home)
	open := func(k string) int {
		t.Helper()
		m.Update(key(k))
		if m.mode != modeBuy && m.mode != modeSell {
			t.Fatalf("%s: mode %v status %q", k, m.mode, m.status)
		}
		got := m.cursor
		m.Update(key("esc"))
		return got
	}
	m.Update(key("1"))
	m.cursor = 3 // a row the dashboard's table left selected
	if b, s := open("b"), open("s"); b != first || s != first {
		t.Fatalf("from the dashboard: buy on %d, sell on %d, want the first row %d for both", b, s, first)
	}
	m.Update(key("2"))
	m.cursor = 2
	if b, s := open("b"), open("s"); b != 2 || s != 2 {
		t.Fatalf("from the market's row 2: buy on %d, sell on %d", b, s)
	}
	// The route target: its first row, whatever the market left.
	m.Update(key("5"))
	m.onRoutes, m.routeCursor = true, 0
	m.cursor = 3
	m.Update(key("R"))
	if m.mode != modeTarget || m.cursor != 0 {
		t.Fatalf("R: mode %v, the target on row %d", m.mode, m.cursor)
	}
}

// The market's cursor walks the rows its table shows (#536): a product
// the city does not deal in is skipped, so ↓ off the last row shown
// reaches the buyers at once, where it stepped onto a row nobody saw.
func TestMarketDownReachesTheBuyers(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("2"))
	city := m.shown()
	last := w.Products[len(w.Products)-1]
	delete(city.Market, last) // a product the city does not deal in, at the table's end
	if len(m.buyerRows()) == 0 {
		w.Contracts = append(w.Contracts, game.Contract{ID: 99, Name: "Vera", City: city.ID, Product: w.Products[0], Units: 10, Status: game.ContractOffered, Expires: w.Day + 3, Due: w.Day + 5, Premium: 1.2})
	}
	rows := m.productsIn(city.ID)
	m.cursor = rows[len(rows)-1]
	m.Update(key("down"))
	if !m.onBuyers {
		t.Fatalf("↓ off the last row shown: cursor %d, not on the buyers", m.cursor)
	}
	m.Update(key("up"))
	if m.onBuyers || m.cursor != rows[len(rows)-1] {
		t.Fatalf("↑ back: on buyers %v, cursor %d", m.onBuyers, m.cursor)
	}
}

// The crew cursor stays on the person (#536), in both lists: when the
// rows move under it overnight (a member gone above, a face new in the
// pool), it follows them; when they are gone it keeps the row, the next
// one in the list. A hire keeps it in LOOKING FOR WORK and a fire in ON
// THE PAYROLL (TestCrewCursorAfterHireAndFire).
func TestCrewCursorFollowsThePerson(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("4"))
	if len(w.Crew.Members) < 3 || len(w.Crew.Candidates) < 2 {
		t.Fatalf("fixture: %d members, %d candidates", len(w.Crew.Members), len(w.Crew.Candidates))
	}
	// ON THE PAYROLL: the member above leaves overnight.
	m.Update(key("down"))
	m.Update(key("down"))
	c, onPayroll, _ := m.crewSelected()
	if !onPayroll {
		t.Fatal("two downs left the payroll")
	}
	w.Crew.Members = w.Crew.Members[1:]
	m.View()
	if got, _, _ := m.crewSelected(); got.ID != c.ID {
		t.Fatalf("a member gone above: the cursor is on %s, was on %s", got.Name, c.Name)
	}
	// LOOKING FOR WORK: a new face comes in at the top of the pool.
	m.crewCursor = len(w.Crew.Members) + 1
	c, onPayroll, _ = m.crewSelected()
	if onPayroll {
		t.Fatal("the cursor is not in the pool")
	}
	w.Crew.Candidates = append([]game.CrewMember{{ID: 901, Name: "Newface", Role: game.RoleRunner, Skill: 30, Fee: 100}}, w.Crew.Candidates...)
	if !strings.Contains(stripANSI(mainText(m)), "▸ "+c.Name) {
		t.Fatalf("the ▸ is not on %s after a face came in:\n%s", c.Name, stripANSI(mainText(m)))
	}
	if got, _, _ := m.crewSelected(); got.ID != c.ID {
		t.Fatalf("a face new in the pool: the cursor is on %s, was on %s", got.Name, c.Name)
	}
	// The one under the cursor gone: the row stays, on the next face.
	at := m.crewCursor
	next := w.Crew.Candidates[at-len(w.Crew.Members)+1]
	w.Crew.Candidates = append(w.Crew.Candidates[:at-len(w.Crew.Members)], w.Crew.Candidates[at-len(w.Crew.Members)+1:]...)
	if got, onPayroll, _ := m.crewSelected(); onPayroll || got.ID != next.ID {
		t.Fatalf("the face under the cursor gone: on %s (payroll %v), want %s", got.Name, onPayroll, next.Name)
	}
	// h hires who the ▸ is on.
	w.Player.DirtyCash = 1_000_000
	m.Update(key("h"))
	if w.Crew.Member(next.ID) == nil {
		t.Fatalf("h did not hire %s: %q", next.Name, m.status)
	}
}

// The post picker opens on somebody free (#536): an enforcer posted on
// another corner is listed, never the row enter takes by default.
func TestPostPickerOpensOnSomebodyFree(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	var enforcers []int
	for _, c := range w.Crew.Members {
		if c.Role == game.RoleEnforcer {
			enforcers = append(enforcers, c.ID)
		}
	}
	for len(enforcers) < 2 {
		id := 900 + len(enforcers)
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: id, Name: fmt.Sprintf("Tee%d", id), Role: game.RoleEnforcer, Skill: 50, Loyalty: 80})
		enforcers = append(enforcers, id)
	}
	// And one free at the list's end, whatever the fixture posted.
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 950, Name: "Free", Role: game.RoleEnforcer, Skill: 50, Loyalty: 80})
	// The first enforcer guards a corner of yours; the picker is opened
	// on another.
	var held []string
	for _, c := range w.Home().Corners {
		if c.Owner == game.OwnerPlayer {
			held = append(held, c.ID)
		}
	}
	if len(held) < 2 {
		t.Fatalf("fixture: %d corners held", len(held))
	}
	if err := w.Post(held[0], enforcers[0]); err != nil {
		t.Fatal(err)
	}
	m.Update(key("5"))
	for i, c := range m.shown().Corners {
		if c.ID == held[1] {
			m.mapCursor = i
		}
	}
	m.onRoutes = false
	m.Update(key("e"))
	if m.mode != modePost {
		t.Fatalf("e: mode %v status %q", m.mode, m.status)
	}
	if who := m.postRows(game.RoleEnforcer)[m.pick.cursor]; w.PostOf(who.ID) != nil {
		t.Fatalf("the picker opened on %s, posted on %s", who.Name, w.PostOf(who.ID).Name)
	}
}

// The cart follows the digit rule (#536): a digit is the row, never the
// dial; and x leaves the cursor on the next line down, so two x's in a
// row remove two lines in a row.
func TestCartDigitsAndTwoRemovals(t *testing.T) {
	m := richModel(t, 120, 40)
	fillCart(t, m)
	m.Update(key("c"))
	lines := m.cartModalLines()
	if len(lines) < 4 {
		t.Fatalf("fixture: %d cart lines", len(lines))
	}
	// A digit on an order's row moves the cursor and leaves its dial.
	var order int
	for i, l := range lines {
		if !l.buy && !l.keep {
			order = i
			break
		}
	}
	m.crt.cursor = order
	dial := lines[order].dial
	m.Update(key("1"))
	if m.crt.cursor != 0 {
		t.Fatalf("1: the cursor is on row %d", m.crt.cursor)
	}
	if o, _ := m.w.Order(lines[order].city, lines[order].product); o.Dial != dial {
		t.Fatalf("1 turned the order's dial: %v, was %v", o.Dial, dial)
	}
	m.Update(key(strconv.Itoa(order + 1)))
	if m.crt.cursor != order {
		t.Fatalf("%d: the cursor is on row %d", order+1, m.crt.cursor)
	}
	// Two x's from the first order line: it and the one under it go,
	// and the cursor stands on the line after them.
	before := m.cartModalLines()
	if order+2 >= len(before) {
		t.Fatalf("fixture: no line after the two orders (%d lines, first order %d)", len(before), order)
	}
	gone1, gone2, after := before[order], before[order+1], before[order+2]
	m.Update(key("x"))
	if l := m.cartSelected(); l == nil || l.product != gone2.product || l.city != gone2.city || l.buy != gone2.buy || l.keep != gone2.keep {
		t.Fatalf("after one x the cursor is on %+v, want the next line %+v", l, gone2)
	}
	m.Update(key("x"))
	for _, l := range m.cartModalLines() {
		if l == gone1 || (l.product == gone2.product && l.city == gone2.city && l.buy == gone2.buy && l.keep == gone2.keep && l.standing == gone2.standing) {
			t.Fatalf("a line x removed is still in the cart: %+v", l)
		}
	}
	if l := m.cartSelected(); l == nil || l.product != after.product || l.city != after.city || l.keep != after.keep {
		t.Fatalf("after two x's the cursor is on %+v, want %+v", l, after)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "1-9 pick") || strings.Contains(v, "1-3 dial") {
		t.Fatalf("the cart's footer:\n%s", v)
	}
}

// A map key acts on the target the pane shows (#536): with the cursor
// on the routes, a corner's key says so and does nothing to the corner
// the grid's cursor last sat on (`e` asked who should guard The Strip
// with COAST ROAD in the pane).
func TestMapCornerKeysRefuseOnTheRoutes(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("5"))
	if len(m.routeLines(m.mainWidth())) == 0 {
		t.Fatal("fixture: no routes on the map")
	}
	m.onRoutes, m.routeCursor = true, 0
	for _, k := range []string{"c", "e", "a", "w", "u", "t", "d"} {
		m.mode, m.status = modePlay, ""
		m.Update(key(k))
		if m.mode != modePlay || !strings.HasPrefix(m.status, "Pick a corner:") {
			t.Errorf("%s on the routes: mode %v status %q", k, m.mode, m.status)
		}
	}
	for _, b := range m.keysFor(screenMap) {
		if b.label == "post enforcer" || b.label == "send enforcers" {
			t.Errorf("KEYS lists %s %s with the cursor on the routes", b.key, b.label)
		}
	}
	m.onRoutes = false
	m.mode, m.status = modePlay, ""
	m.Update(key("e"))
	if m.mode != modePost && !strings.HasPrefix(m.status, "Can't") && !strings.HasPrefix(m.status, "Nothing") {
		t.Errorf("e on a corner: mode %v status %q", m.mode, m.status)
	}
}

// In the route target's kind, → stops at days and ← at units (#536: →
// wrapped from days back to units, and 7 and 4 were set as units).
func TestRouteKindStopsAtTheEnds(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("5"))
	m.onRoutes, m.routeCursor = true, 0
	m.Update(key("R"))
	m.Update(key("enter"))
	if m.mode != modeTarget || m.tgt.step != 1 {
		t.Fatalf("the kind: mode %v step %d", m.mode, m.tgt.step)
	}
	for _, step := range []struct {
		key  string
		days bool
	}{{"left", false}, {"left", false}, {"right", true}, {"right", true}, {"l", true}, {"h", false}} {
		m.Update(key(step.key))
		if m.tgt.days != step.days {
			t.Fatalf("%s: days %v, want %v", step.key, m.tgt.days, step.days)
		}
	}
}

// A key a screen binds differently from the global table is in that
// screen's KEYS (#536: `l` on the crew screen is assign, not lie low),
// and enter off the dashboard ends nothing: it points at the dashboard,
// whose enter is the night's preview.
func TestScreenKeysOverGlobalsAreListed(t *testing.T) {
	m := richModel(t, 300, 60)
	globals := map[string]bool{}
	for _, b := range bindings {
		if b.global && !b.quiet {
			for _, k := range rawKeys(b) {
				globals[k] = true
			}
		}
	}
	for s := screen(0); s < screenCount; s++ {
		m.switchScreen(s)
		listed := map[string]bool{}
		for _, b := range m.keysFor(s) {
			listed[b.key+" "+b.label] = true
		}
		for _, b := range bindings {
			if b.global || b.quiet || !b.names(s) || !b.live(m) || (b.listed != nil && !b.listed(m)) {
				continue
			}
			for _, k := range rawKeys(b) {
				if globals[k] && !listed[b.key+" "+b.label] {
					t.Errorf("%s: %s %s shadows a global key and is not in KEYS", screens[s].word, b.key, b.label)
				}
			}
		}
	}
	m.switchScreen(screenCrew)
	if !slicesHasLabel(m.keysFor(screenCrew), "l", "assign") {
		t.Error("the crew screen's KEYS lacks l assign")
	}
	for _, s := range []string{"2", "4", "5", "7"} {
		day := m.w.Day
		m.Update(key(s))
		m.Update(key("enter"))
		if m.mode != modePlay || m.w.Day != day || !strings.Contains(m.status, "dashboard screen (1)") {
			t.Errorf("enter on screen %s: mode %v day %d -> %d status %q", s, m.mode, day, m.w.Day, m.status)
		}
	}
	m.Update(key("1"))
	m.Update(key("enter"))
	if m.mode != modeConfirmEnd {
		t.Errorf("enter on the dashboard: mode %v", m.mode)
	}
}

func slicesHasLabel(bs []binding, key, label string) bool {
	for _, b := range bs {
		if b.key == key && b.label == label {
			return true
		}
	}
	return false
}

// On the rivals screen w acts on the faction under the cursor (#536):
// with a war on one faction and another selected, w declares nothing
// and names the war; turned to the one at war, it calls it off, naming
// it. The propose dialog names who it goes to.
func TestWarKeyActsOnTheSelectedFaction(t *testing.T) {
	m := tableModel(t, 120, 40)
	w := m.w
	at, other := w.Rivals[0], w.Rivals[1]
	w.War = at.Faction()
	m.Update(key("8"))
	for i, r := range w.Rivals {
		if r == other {
			m.factionCursor = i
		}
	}
	if atWar(m) {
		t.Fatal("atWar with another faction selected")
	}
	m.Update(key("w"))
	if m.mode != modePlay || !strings.Contains(m.status, m.rivalName(at)) || !strings.Contains(m.status, m.rivalName(other)) || w.War != at.Faction() {
		t.Fatalf("w on %s at war with %s: mode %v status %q war %q", m.rivalName(other), m.rivalName(at), m.mode, m.status, w.War)
	}
	m.Update(key("d"))
	if v := stripANSI(m.View()); m.mode != modePropose || !strings.Contains(v, "PROPOSE TO "+strings.ToUpper(m.rivalName(other))) {
		t.Fatalf("d on %s: mode %v\n%s", m.rivalName(other), m.mode, v)
	}
	m.Update(key("esc"))
	for i, r := range w.Rivals {
		if r == at {
			m.factionCursor = i
		}
	}
	m.Update(key("w"))
	if v := stripANSI(m.View()); m.mode != modeConfirm || !strings.Contains(v, "CALL OFF THE WAR ON "+strings.ToUpper(m.rivalName(at))) {
		t.Fatalf("w on %s: mode %v\n%s", m.rivalName(at), m.mode, v)
	}
	m.Update(key("esc"))
}
