package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The reserve dialog (#195): clean cash into the offshore account, the
// one place nothing takes it from and the one it never comes back
// from. One page, a number field in dollars (blank is a lot: what
// moves unnoticed, less tonight's clean upkeep, #458), the fee and the
// pages the DA files over the lot, in red past it, and a warning when
// the move leaves the clean pile under tonight's upkeep. It is an
// amountDialog (#275).

// askReserve opens the reserve dialog.
func (m *Model) askReserve() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.refuse("Can't reserve: the account takes clean cash, and you have none.")
		return
	}
	hint := "blank = a lot"
	if m.upkeepTonight() > 0 {
		hint = "blank = a lot, less upkeep"
	}
	m.openAmount(modeReserve, hint, m.w.Player.CleanCash, true, "")
}

// reserveBlank is what a blank amount moves: a lot, or the clean cash
// less tonight's upkeep where that is less (#458: a blank reserve used
// to move every clean dollar, and the next morning every front shut).
func (m *Model) reserveBlank() int {
	return max(0, min(m.rules.Laundering.Offshore().Lot, m.w.Player.CleanCash-m.upkeepTonight()))
}

// reserveAmount is the amount the field reads: blank is reserveBlank,
// and says so when that is nothing.
func (m *Model) reserveAmount() (int, error) {
	if blank := m.reserveBlank(); blank <= 0 && strings.TrimSpace(m.amt.Value()) == "" {
		return 0, fmt.Errorf("blank keeps %s clean back for tonight's upkeep, and that is all of it: type an amount to move it anyway", money(m.upkeepTonight()))
	}
	return readQty(m.amt.numberField, m.reserveBlank())
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
	say := fmt.Sprintf("%s clean goes offshore tonight, fee %s.", money(amt), money(l.Fee(m.w, amt)))
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
		body = append(body, row("moves", fmt.Sprintf("%s tonight, fee %s   %s", style.Render(money(amt)), money(l.Fee(m.w, amt)), pages)))
		body = append(body, m.upkeepWarning(w.Player.CleanCash-amt, w.Player.DirtyCash, m.upkeepTonight())...)
	}
	if due := m.upkeepTonight(); due > 0 {
		body = append(body, row("upkeep", fmt.Sprintf("%s clean tonight %s", money(due), theme.Subtle.Render("(blank keeps it back)"))))
	}
	body = append(body, "",
		theme.Subtle.Render(fmt.Sprintf("Up to %s a day moves unnoticed; every lot over it is a page. The account keeps %s.", money(off.Lot), format.Pct(off.Fee, 0))),
		theme.Subtle.Render("Nothing takes from the account and nothing comes back: it is the exit, and the score."))
	if off.RetireCash > 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Retiring takes %s offshore and %s quiet in a row: %s so far.", money(off.RetireCash), plural(off.RetireDays, "day"), m.quietCount())))
	}
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("RESERVE OFFSHORE", body, m.modalFooter())
}
