package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The cash-out dialog (#395): clean cash drawn back into the dirty
// pile, at once, less the banker's fee, for a run whose money sits on
// the books when the connect and the payroll want cash. One page, a
// number field in dollars (blank is what tonight's wages are short,
// the fee on top), what lands and the pile against the exposure line,
// in red past it. It is an amountDialog (#275).

// askCashOut opens the cash-out dialog.
func (m *Model) askCashOut() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.refuse("Can't cash out: there is no clean cash to draw.")
		return
	}
	m.openAmount(modeCashOut, "blank = the wages short", m.w.Player.CleanCash, true, "")
}

// wagesShort is what tonight's wages want over the dirty cash in hand.
func (m *Model) wagesShort() int {
	return max(0, m.rules.Crew.Wages(m.w, m.w.Crew.Pay)-m.w.Player.DirtyCash)
}

// cashOutFor is the clean cash to draw for want to land dirty, the fee
// on top, or the clean cash where that is less.
func (m *Model) cashOutFor(want int) int {
	l := m.rules.Laundering
	amt := want
	for amt < m.w.Player.CleanCash && amt-l.CashOutFee(amt) < want {
		amt += want - (amt - l.CashOutFee(amt))
	}
	return min(amt, m.w.Player.CleanCash)
}

// cashOutAmount is the amount the field reads: blank is what tonight's
// wages are short.
func (m *Model) cashOutAmount() (int, error) {
	return readQty(m.amt.numberField, m.cashOutFor(m.wagesShort()))
}

func (m *Model) keyCashOut(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.keyAmount(k, func() int { return m.w.Player.CleanCash }, m.confirmCashOut)
}

// confirmCashOut draws the amount, or shows why it cannot.
func (m *Model) confirmCashOut() {
	amt, err := m.cashOutAmount()
	if err != nil {
		m.amt.err = dialogError(err)
		return
	}
	fee := m.rules.Laundering.CashOutFee(amt)
	if err := m.sess.CashOut(amt); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
	say := fmt.Sprintf("Cashed out %s clean: %s dirty in hand, the banker kept %s.", money(amt), money(amt-fee), money(fee))
	if line := m.rules.Heat.ExposureLine(m.w); line > 0 && m.w.Player.DirtyCash > line {
		m.alarm(say + " The pile is over what your fronts cover: it draws heat.")
		return
	}
	m.say(say)
}

// viewCashOut is the dialog: the piles, the field, what lands and where
// the pile stands against the exposure line.
func (m *Model) viewCashOut() string {
	w := m.w
	l := m.rules.Laundering
	amt, err := m.cashOutAmount()
	if err != nil {
		amt = 0
	}
	body := []string{
		m.inHand(),
		row("amount", m.amountField(w.Player.CleanCash).View()),
	}
	if short := m.wagesShort(); short > 0 {
		body = append(body, row("wages", theme.Warning.Render(fmt.Sprintf("%s short tonight", money(short)))))
	}
	if amt > 0 {
		fee := l.CashOutFee(amt)
		style := theme.Gold
		if amt > w.Player.CleanCash {
			style = theme.Bad
		}
		pile := w.Player.DirtyCash + amt - fee
		where := theme.Good.Render("under the exposure line")
		if line := m.rules.Heat.ExposureLine(w); line > 0 && pile > line {
			where = theme.Bad.Render(fmt.Sprintf("%s over the exposure line: it draws heat", money(pile-line)))
		}
		body = append(body, row("lands", fmt.Sprintf("%s dirty, fee %s   %s", style.Render(money(amt-fee)), money(fee), where)))
	}
	body = append(body, "",
		theme.Subtle.Render(fmt.Sprintf("Stock and wages are paid dirty. The banker keeps %s of the draw.", format.Pct(float64(l.CashOutFee(1_000_000))/1e6, 0))),
		theme.Subtle.Render("It lands at once; a pile past what your fronts cover draws heat."))
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("CASH OUT", body, m.modalFooter())
}
