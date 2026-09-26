package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The playtest's input issues (#499, #500): the buy and sell dialogs'
// last rows in view at 80x24, esc on a card, the crew cursor after a
// hire or a fire, a flow that grows a step, a buy that never defaults
// to the contract, and the pickers that opened on what was last used.

// boxText is the open modal as the player sees it: the box's rows only,
// what scrolled out of it left out.
func boxText(t *testing.T, m *Model) string {
	t.Helper()
	_, _, box := modalBox(t, m.View())
	return stripANSI(strings.Join(box, "\n"))
}

// At 80x24 with every product unlocked and a slump line, the buy's
// repeat and pay rows and the sale's dial and repeat rows are in the
// box (#499): the product table is the product picked past its step,
// and the step's rows are followed into view, since ↓ and pgdn are the
// field's and the dial's there and cannot scroll to them.
func TestBuyAndSellRowsFitAt80x24(t *testing.T) {
	for _, cart := range []bool{false, true} {
		m := richModel(t, 80, 24)
		w := m.w
		if len(w.Products) != 6 {
			t.Fatalf("the fixture has %d products, not all six", len(w.Products))
		}
		loc := w.Player.Location
		for _, id := range w.Products {
			p := w.Product(loc, id)
			p.ShockDays, p.ShockFactor, p.ShockSlump = 3, 0.6, true
		}
		w.Player.DirtyCash = 700_000
		if cart {
			fillCart(t, m)
		}
		// A contract on the first product: its line is on the quantity
		// step and its warning on the terms at keep at.
		if err := m.sess.SetSupply(loc, w.Products[0], 30); err != nil {
			t.Fatal(err)
		}
		m.Update(key("1"))
		m.Update(key("b"))
		if m.dlg.pick {
			m.Update(key("enter"))
		}
		m.Update(key("enter"))
		m.Update(key("enter"))
		if m.mode != modeBuy || m.dlg.step != 2 {
			t.Fatalf("cart %v: the buy's terms: mode %v step %d err %q", cart, m.mode, m.dlg.step, m.dlg.err)
		}
		for _, repeat := range []string{"once", "keep at"} {
			if repeat == "keep at" {
				m.Update(key("right"))
			}
			box := boxText(t, m)
			for _, want := range []string{"repeat ", "pay ", "slump"} {
				if want == "slump" && cart {
					continue // the terms are what must show; the price rows may scroll above
				}
				if !strings.Contains(box, want) {
					t.Errorf("cart %v, %s: the buy's %q row is under the fold:\n%s", cart, repeat, want, box)
				}
			}
		}
		m.Update(key("esc"))
		w.SetStock(loc, w.Products[0], 40)
		m.Update(key("s"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		if box := boxText(t, m); m.dlg.step != 2 || !strings.Contains(box, "dial ") || !strings.Contains(box, "expect ") {
			t.Errorf("cart %v: the sale's dial is under the fold (step %d):\n%s", cart, m.dlg.step, box)
		}
		m.Update(key("enter"))
		if box := boxText(t, m); m.dlg.step != 3 || !strings.Contains(box, "repeat ") {
			t.Errorf("cart %v: the sale's repeat is under the fold (step %d):\n%s", cart, m.dlg.step, box)
		}
		m.Update(key("esc"))
	}
}

// esc on a dilemma card closes it without answering (#500): the report
// opens, nothing is paid, and the card is back, still unanswered, when
// the report closes; the report's jumps wait for the answer.
func TestEscSetsTheCardAside(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	if w.Report == nil {
		t.Fatal("the fixture has no report")
	}
	w.Dilemmas.Pending = testCard(w.Day)
	m.showCard()
	if foot := stripANSI(legend(m.modalFooter())); !strings.Contains(foot, "esc close") {
		t.Fatalf("the card's footer does not list esc: %q", foot)
	}
	dirty, heat := w.Player.DirtyCash, w.Here().Heat
	m.Update(key("down"))
	m.Update(key("esc"))
	if m.mode != modeReport || w.Dilemmas.Pending == nil || m.cardDone || w.Player.DirtyCash != dirty || w.Here().Heat != heat {
		t.Fatalf("esc: mode %v pending %v done %v dirty %d → %d", m.mode, w.Dilemmas.Pending != nil, m.cardDone, dirty, w.Player.DirtyCash)
	}
	if hasLead(m) || stoppedOnAlert(m) {
		t.Error("the report offers a jump while the card waits")
	}
	m.Update(key("enter"))
	if m.mode != modeCard || m.cardCursor != noChoice || w.Dilemmas.Pending == nil {
		t.Fatalf("the report closed: mode %v cursor %d", m.mode, m.cardCursor)
	}
	m.Update(key("1"))
	m.Update(key("enter"))
	if !m.cardDone || w.Dilemmas.Pending != nil {
		t.Fatalf("answering after esc: done %v", m.cardDone)
	}
	m.Update(key("enter")) // the outcome closes on the report
	m.Update(key("enter"))
	if m.mode != modePlay {
		t.Fatalf("after the answer: mode %v", m.mode)
	}
}

// After a hire or a fire the crew cursor stays on the same person or
// the same row (#500): a fire keeps it on its row of ON THE PAYROLL
// (the last member's row once the last is fired, never the first face
// LOOKING FOR WORK), and a hire keeps it on its row of the candidates.
func TestCrewCursorAfterHireAndFire(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Player.DirtyCash = 1_000_000
	m.Update(key("4"))
	fire := func(row int) {
		t.Helper()
		m.crewCursor = row
		m.Update(key("f"))
		m.Update(key("y"))
	}
	// A middle row: the next member up is under the cursor.
	next := w.Crew.Members[2].ID
	fire(1)
	if c, onPayroll, _ := m.crewSelected(); !onPayroll || c.ID != next || m.crewCursor != 1 {
		t.Fatalf("fired row 1: cursor %d on %s (payroll %v)", m.crewCursor, c.Name, onPayroll)
	}
	// The last row: the cursor stays on the payroll.
	last := len(w.Crew.Members) - 1
	fire(last)
	if c, onPayroll, _ := m.crewSelected(); !onPayroll || m.crewCursor != last-1 {
		t.Fatalf("fired the last member: cursor %d on %s (payroll %v)", m.crewCursor, c.Name, onPayroll)
	}
	// Hires: the cursor keeps its row among the faces looking for work.
	w.Crew.Candidates = []game.CrewMember{
		{ID: 201, Name: "Ace", Role: "runner", Skill: 40, Units: 80, Loyalty: 60, Wage: 40, Fee: 100},
		{ID: 202, Name: "Bee", Role: "runner", Skill: 40, Units: 80, Loyalty: 60, Wage: 40, Fee: 100},
		{ID: 203, Name: "Cee", Role: "runner", Skill: 40, Units: 80, Loyalty: 60, Wage: 40, Fee: 100},
	}
	m.crewCursor = len(w.Crew.Members) + 1 // Bee
	m.Update(key("h"))
	if w.Crew.Member(202) == nil {
		t.Fatalf("Bee was not hired: %q", m.status)
	}
	if c, onPayroll, _ := m.crewSelected(); onPayroll || c.ID != 203 {
		t.Fatalf("after the hire: cursor %d on %s (payroll %v), want Cee", m.crewCursor, c.Name, onPayroll)
	}
	m.Update(key("h"))
	if c, onPayroll, _ := m.crewSelected(); w.Crew.Member(203) == nil || onPayroll || c.ID != 201 {
		t.Fatalf("after the last face's hire: cursor %d on %s (payroll %v), want Ace", m.crewCursor, c.Name, onPayroll)
	}
}

// A flow that grows a step says so the first time (#500): the buy's
// connect step, which a second connect where you stand adds before
// the product, opens once with a line saying it is new.
func TestConnectStepSaysItIsNew(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	if open, _ := m.openConnects(); len(open) < 2 {
		// A second connect where you stand, a copy of the first.
		sup := *w.SuppliersIn(w.Player.Location)[0]
		sup.ID, sup.Name = "second", "Second"
		w.Suppliers = append(w.Suppliers, sup)
	}
	if open, _ := m.openConnects(); len(open) < 2 {
		t.Fatalf("the fixture has %d connects dealing", len(open))
	}
	m.connectSeen = false
	m.Update(key("1"))
	m.Update(key("b"))
	if !m.dlg.pick || !strings.Contains(boxText(t, m), "New: ") {
		t.Fatalf("the connect step does not say it is new:\n%s", boxText(t, m))
	}
	m.Update(key("esc"))
	m.Update(key("b"))
	if !m.dlg.pick || strings.Contains(boxText(t, m), "New: ") {
		t.Fatalf("the second time still says new:\n%s", boxText(t, m))
	}
}

// The buy never defaults to replacing a contract (#500): a product kept
// at 30 opens at once, and `22 enter enter`, typed for a buyer, buys 22
// and leaves the contract at 30.
func TestBuyNeverDefaultsToTheContract(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	loc, id := w.Player.Location, w.Products[0]
	w.Player.DirtyCash = 100_000
	if err := m.sess.SetSupply(loc, id, 30); err != nil {
		t.Fatal(err)
	}
	m.Update(key("2"))
	m.cursor = 0
	m.Update(key("b"))
	if m.dlg.pick {
		m.Update(key("enter"))
	}
	m.Update(key("enter"))
	if m.dlg.repeat != repeatOnce || m.dlg.qty.Value() != "" {
		t.Fatalf("the kept product opened at %v on %q", m.dlg.repeat, m.dlg.qty.Value())
	}
	stock := w.Stock(loc, id)
	for _, k := range []string{"2", "2", "enter", "enter"} {
		m.Update(key(k))
	}
	if c, ok := w.Supplied(loc, id); !ok || c.Units != 30 || w.Stock(loc, id) != stock+22 {
		t.Fatalf("22 enter enter: contract %+v %v, stock %d → %d, err %q", c, ok, stock, w.Stock(loc, id), m.dlg.err)
	}
}

// The pickers that remembered too much open fresh (#500): the ledger's
// b on the front row whatever kind was bought last, and the upgrades
// screen on its first branch's first node whatever branch was shown.
func TestPickersOpenFresh(t *testing.T) {
	m := richModel(t, 120, 40)
	m.Update(key("7"))
	m.Update(key("b"))
	m.Update(key("2")) // the house row
	if m.front.kind != pickHouse {
		t.Fatalf("2 on the kind page: kind %d", m.front.kind)
	}
	m.Update(key("esc"))
	m.Update(key("b"))
	if m.mode != modeFront || m.front.step != 0 || m.front.kind != pickFront {
		t.Fatalf("b again: mode %v step %d kind %d, not the front row", m.mode, m.front.step, m.front.kind)
	}
	m.Update(key("esc"))
	m.Update(key("6"))
	m.Update(key("right"))
	m.Update(key("down"))
	if m.branch != 1 {
		t.Fatalf("→ on the upgrades: branch %d", m.branch)
	}
	m.Update(key("1"))
	m.Update(key("6"))
	if m.branch != 0 || *m.nodeCursor() != 0 {
		t.Fatalf("the upgrades again: branch %d node %d, not the first", m.branch, *m.nodeCursor())
	}
	// Turning within the screen still keeps each branch's node.
	m.Update(key("down"))
	m.Update(key("right"))
	m.Update(key("left"))
	if *m.nodeCursor() != 1 {
		t.Fatalf("a branch left and come back to within the screen: node %d", *m.nodeCursor())
	}
}
