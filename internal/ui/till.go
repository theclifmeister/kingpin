package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The till dialog (#496, docs/laundering.md): the dirty cash the wash
// leaves in hand every night. T on the ledger opens it; the field is the
// till, blank the file's float (the till as it was), m all the dirty cash
// in hand, and enter sets it. A playtest with two big fronts sat at
// exactly $50,000 dirty every morning and could not save for a chemist's
// lot, a dirty-priced upgrade or the next front: the wash took the rest.
// Under the float it is the float. The supply contracts' morning is kept
// back on its own (laundering.Sim.Line), whatever the till. It is an
// amountDialog (#275).
//
// Its range is the float to the rot line (#526, tillMax): a playtest's
// field read `/ $23,689 max`, the dirty in hand, beside `never under the
// $50,000 float`, and took 1,250,000 against a $35,000 max. The till is
// a line to save up to, so it may be over the cash in hand; over the rot
// line it saves nothing (the pile rots there). A number under the float
// is the float, said; one over the top is set to the top, said, and the
// next enter sets it. The field opens blank, so blank = the float does
// what it says (it opened on the till set, and enter kept it); the till
// row above it reads the till as it stands.

// askTill opens the till dialog, the field blank.
func (m *Model) askTill() {
	if m.w.Over != nil {
		return
	}
	m.openAmount(modeTill, "blank = the float", m.tillMax(), true, "")
	m.amt.min = m.floatLine()
}

// tillMax is the most the till can be (#526): the rot line
// (laundering.toml rot_line), over which the pile rots, or with no rot
// the most the field can type.
func (m *Model) tillMax() int {
	if l := m.rules.Laundering.Tuning().RotLine; l > 0 {
		return max(l, m.floatLine())
	}
	return 999_999_999
}

func (m *Model) keyTill(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.amt.min = m.floatLine()
	return m.keyAmount(k, m.tillMax, m.confirmTill)
}

// tillField is the till the field reads: blank is 0, the float.
func (m *Model) tillField() (int, error) { return m.amt.Read(0) }

// confirmTill sets the till at the field's amount.
func (m *Model) confirmTill() {
	t, err := m.tillField()
	if err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.amt.min, m.amt.max = m.floatLine(), m.tillMax()
	if m.amt.Outside() > 0 {
		// Over the top (#526): set to the top and said; the next enter
		// sets it, as a buy over the room is set to what fits.
		m.amt.Set(m.tillMax())
		m.amt.err = fmt.Sprintf("The till tops out at %s, the rot line: dirty cash over it rots. The till is now %s; enter sets it.", money(m.tillMax()), money(m.tillMax()))
		return
	}
	under := m.amt.Outside() < 0
	if under {
		t = 0 // under the float is the float (#526), said below
	}
	if err := m.sess.SetTill(t); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
	if under {
		m.say(fmt.Sprintf("The till is never under the %s float: it is the float, and the wash leaves %s dirty in hand.", money(m.floatLine()), money(m.floatLine())))
		return
	}
	if t <= m.floatLine() {
		m.say(fmt.Sprintf("The till is the float again: the wash leaves %s dirty in hand.", money(m.floatLine())))
		return
	}
	m.say(fmt.Sprintf("The till is %s: the wash leaves that much dirty in hand every night.", money(t)))
}

// floatLine is the file's float folded through the tree, the least the
// till can be.
func (m *Model) floatLine() int { return m.w.Float(m.cfg.Upgrades, m.cfg.Laundering.Laundering.Float) }

// viewTill is the dialog: the till, the field, what the wash would take
// tonight at it and the rules of the till.
func (m *Model) viewTill() string {
	w := m.w
	t, err := m.tillField()
	if err != nil {
		t = 0
	}
	line := max(t, m.floatLine())
	state := money(m.till())
	if w.Laundering.Till <= m.floatLine() {
		state += theme.Subtle.Render(" (the float)")
	}
	body := []string{
		m.inHand(),
		row("till", state),
		row("set to", m.amountField(m.tillMax()).View()),
	}
	keep := max(line, w.SupplyOutlay())
	if n := min(m.rules.Laundering.Capacity(w), max(0, w.Player.DirtyCash-keep)); n > 0 {
		body = append(body, row("tonight", fmt.Sprintf("on the cash in hand the wash takes up to %s, leaves %s", theme.Gold.Render(money(n)), money(w.Player.DirtyCash-n))))
	} else {
		body = append(body, row("tonight", theme.Subtle.Render("nothing over the line to wash")))
	}
	if out := w.SupplyOutlay(); out > line {
		body = append(body, row("contracts", fmt.Sprintf("%s kept back for the morning's buys", money(out))))
	}
	body = append(body, "")
	body = append(body, m.subtle(fmt.Sprintf("The wash takes only the dirty cash over the till, after the night's sales. Raise it to save dirty cash for a contract, a chemist's lot or the next front, over the cash in hand if you like; never under the %s float, never over the %s rot line. The contracts' morning is kept back whatever the till.", money(m.floatLine()), money(m.tillMax())))...)
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("THE TILL", body, m.modalFooter())
}
