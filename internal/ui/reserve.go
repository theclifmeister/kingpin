package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The reserve dialog (#195): clean cash into the offshore account, the
// one place nothing takes it from and the one it never comes back
// from. One page, a number field in dollars (blank is a lot: what
// moves unnoticed), the fee and the pages the DA files over the lot, in
// red past it. It is an amountDialog (#275).

// askReserve opens the reserve dialog.
func (m *Model) askReserve() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.refuse("Can't reserve: the account takes clean cash, and you have none.")
		return
	}
	m.openAmount(modeReserve, "blank = a lot", m.w.Player.CleanCash, true, "")
}

// reserveAmount is the amount the field reads: blank is a lot, or the
// clean cash where that is less.
func (m *Model) reserveAmount() (int, error) {
	lot := min(m.rules.Laundering.Offshore().Lot, m.w.Player.CleanCash)
	return readQty(m.amt.numberField, lot)
}

func (m *Model) keyReserve(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.keyAmount(k, func() int { return m.w.Player.CleanCash }, m.confirmReserve)
}

// confirmReserve sends the amount, or shows why it cannot.
func (m *Model) confirmReserve() {
	amt, err := m.reserveAmount()
	if err != nil {
		m.amt.err = dialogError(err)
		return
	}
	if err := m.sess.Reserve(amt); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
	l := m.rules.Laundering
	say := fmt.Sprintf("%s clean goes offshore tonight, fee %s.", money(amt), money(l.Fee(amt)))
	if lots := l.Lots(m.w.ReservedToday()); lots > 0 {
		m.alarm(say + fmt.Sprintf(" Over the lot by %s: the DA will read it.", plural(lots, "lot")))
		return
	}
	m.say(say + " Under the lot: nobody reads it.")
}

// viewReserve is the dialog: the account, the field, the fee and what
// the DA reads.
func (m *Model) viewReserve() string {
	w := m.w
	l := m.rules.Laundering
	off := l.Offshore()
	amt, err := m.reserveAmount()
	if err != nil {
		amt = 0
	}
	field := m.amountField(w.Player.CleanCash)
	body := []string{
		m.inHand(),
		row("account", theme.Good.Render(cash(w.Offshore))+" offshore"),
		row("amount", field.View()),
	}
	if amt > 0 {
		total := amt + w.ReservedToday()
		lots := l.Lots(total)
		pages := theme.Good.Render("under the lot: nobody reads it")
		if lots > 0 {
			pages = theme.Bad.Render(fmt.Sprintf("over the lot by %s: %s in the DA's file tomorrow", plural(lots, "lot"), plural(lots*m.rules.Heat.StructureEvidence(), "page")))
		}
		style := theme.Gold
		if amt > w.Player.CleanCash {
			style = theme.Bad
		}
		body = append(body, row("moves", fmt.Sprintf("%s tonight, fee %s   %s", style.Render(money(amt)), money(l.Fee(amt)), pages)))
	}
	body = append(body, "",
		theme.Subtle.Render(fmt.Sprintf("Up to %s a day moves unnoticed; every lot over it is a page. The account keeps %s.", money(off.Lot), format.Pct(off.Fee, 0))),
		theme.Subtle.Render("Nothing takes from the account and nothing comes back: it is the exit, and the score."))
	if off.RetireCash > 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Retiring takes %s offshore and %s quiet in a row; %s so far.", money(off.RetireCash), plural(off.RetireDays, "day"), plural(w.QuietDays, "quiet day"))))
	}
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("RESERVE OFFSHORE", body, m.modalFooter())
}
