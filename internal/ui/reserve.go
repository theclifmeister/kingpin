package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The reserve dialog (#195): clean cash into the offshore account, the
// one place nothing takes it from and the one it never comes back
// from. One page, a number field in dollars (blank is a lot: what
// moves unnoticed), the fee and the pages the DA files over the lot, in
// red past it.
type reserveDialog struct {
	amt numberField
	err string
}

// askReserve opens the reserve dialog.
func (m *Model) askReserve() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.refuse("Can't reserve: the account takes clean cash, and you have none.")
		return
	}
	m.rsv = reserveDialog{amt: newNumberField("blank = a lot")}
	m.rsv.amt.money = true
	m.rsv.amt.max = m.w.Player.CleanCash
	m.rsv.amt.Focus()
	m.mode = modeReserve
}

// reserveAmount is the amount the field reads: blank is a lot, or the
// clean cash where that is less.
func (m *Model) reserveAmount() (int, error) {
	lot := min(m.set.Laundering.Offshore().Lot, m.w.Player.CleanCash)
	return parseQtyInput(m.rsv.amt.Value(), lot)
}

func (m *Model) keyReserve(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	m.rsv.err = ""
	switch key {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "enter":
		m.confirmReserve()
		return m, nil
	}
	m.rsv.amt.max = m.w.Player.CleanCash
	return m, m.rsv.amt.Update(k)
}

// confirmReserve sends the amount, or shows why it cannot.
func (m *Model) confirmReserve() {
	amt, err := m.reserveAmount()
	if err != nil {
		m.rsv.err = dialogError(err)
		return
	}
	if err := m.w.Reserve(amt); err != nil {
		m.rsv.err = dialogError(err)
		return
	}
	m.mode = modePlay
	l := m.set.Laundering
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
	l := m.set.Laundering
	off := l.Offshore()
	amt, err := m.reserveAmount()
	if err != nil {
		amt = 0
	}
	field := m.rsv.amt
	field.max = w.Player.CleanCash
	body := []string{
		fmt.Sprintf("Account   %s offshore · %s clean in hand", theme.Good.Render(cash(w.Offshore)), cash(w.Player.CleanCash)),
		fmt.Sprintf("Amount    %s", field.View()),
	}
	if amt > 0 {
		total := amt + w.ReservedToday()
		lots := l.Lots(total)
		pages := theme.Good.Render("under the lot: nobody reads it")
		if lots > 0 {
			pages = theme.Bad.Render(fmt.Sprintf("over the lot by %s: %s in the DA's file tomorrow", plural(lots, "lot"), plural(lots*m.set.Heat.StructureEvidence(), "page")))
		}
		style := theme.Gold
		if amt > w.Player.CleanCash {
			style = theme.Bad
		}
		body = append(body, fmt.Sprintf("Moves     %s tonight, fee %s   %s", style.Render(money(amt)), money(l.Fee(amt)), pages))
	}
	body = append(body, "",
		theme.Subtle.Render(fmt.Sprintf("Up to %s a day moves unnoticed; every lot over it is a page. The account keeps %.0f%%.", money(off.Lot), off.Fee*100)),
		theme.Subtle.Render("Nothing takes from the account and nothing comes back: it is the exit, and the score."))
	if off.RetireCash > 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Retiring takes %s offshore and %s quiet in a row; %s so far.", money(off.RetireCash), plural(off.RetireDays, "day"), plural(w.QuietDays, "quiet day"))))
	}
	if m.rsv.err != "" {
		body = append(body, "", theme.Bad.Render(m.rsv.err))
	}
	return m.modal("RESERVE OFFSHORE", body, m.modalFooter())
}
