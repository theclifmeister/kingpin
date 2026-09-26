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

// askTill opens the till dialog on the till the player set, if any.
func (m *Model) askTill() {
	if m.w.Over != nil {
		return
	}
	m.openAmount(modeTill, "blank = the float", m.w.Player.DirtyCash, true, "")
	if t := m.w.Laundering.Till; t > m.floatLine() {
		m.amt.Set(t)
	}
}

func (m *Model) keyTill(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.keyAmount(k, func() int { return m.w.Player.DirtyCash }, m.confirmTill)
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
	if err := m.sess.SetTill(t); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
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
		row("set to", m.amountField(w.Player.DirtyCash).View()),
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
	body = append(body, m.subtle(fmt.Sprintf("The wash takes only the dirty cash over the till, after the night's sales. Raise it to save dirty cash for a contract, a chemist's lot or the next front; never under the %s float. The contracts' morning is kept back whatever the till.", money(m.floatLine())))...)
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("THE TILL", body, m.modalFooter())
}
