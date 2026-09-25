package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The sweep dialog (#478, docs/laundering.md): the offshore account's
// standing order. S on the ledger opens it; the field is the clean cash
// to keep in hand (blank keeps tonight's upkeep and nothing over it),
// enter turns the sweep on at that line, and while it is on x turns it
// off. Every night the laundering sim moves the clean over the line into
// the account, up to what is left of the day's lot after anything
// reserved by hand, so it never files a page, and never into the
// night's upkeep. Off by default: a run that never sets it is the run
// before. It is an amountDialog (#275).

// askSweep opens the sweep dialog, on the line the sweep keeps if one
// is on.
func (m *Model) askSweep() {
	if m.w.Over != nil {
		return
	}
	m.openAmount(modeSweep, "blank = the upkeep", max(m.w.Player.CleanCash, m.w.Laundering.Sweep.Keep), true, "")
	if sw := m.w.Laundering.Sweep; sw.On && sw.Keep > 0 {
		m.amt.Set(sw.Keep)
	}
}

// sweepOn is the sweep being on, for the dialog's x.
func sweepOn(m *Model) bool { return m.w.Laundering.Sweep.On }

func (m *Model) keySweep(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "x" && sweepOn(m) {
		m.amt.err = ""
		if err := m.sess.StopSweep(); err != nil {
			m.amt.err = dialogError(err)
			return m, nil
		}
		m.mode = modePlay
		m.say("The sweep is off: the clean stays in hand.")
		return m, nil
	}
	return m.keyAmount(k, func() int { return max(m.w.Player.CleanCash, m.w.Laundering.Sweep.Keep) }, m.confirmSweep)
}

// sweepKeep is the line the field reads: blank is 0, the upkeep alone.
func (m *Model) sweepKeep() (int, error) {
	return m.amt.Read(0)
}

// confirmSweep turns the sweep on at the field's line.
func (m *Model) confirmSweep() {
	keep, err := m.sweepKeep()
	if err != nil {
		m.amt.err = dialogError(err)
		return
	}
	if err := m.sess.SetSweep(keep); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
	m.say(fmt.Sprintf("The sweep is on: clean over %s goes offshore every night, up to a lot.", money(max(keep, m.upkeepTonight()))))
}

// sweepTonight is what the sweep would move tonight at keep, the
// laundering sim's own sum (Sweepable) on the line the field reads.
func (m *Model) sweepTonight(keep int) int {
	off := m.rules.Laundering.Offshore()
	return max(0, min(off.Lot-m.w.ReservedToday(), m.w.Player.CleanCash-max(keep, m.upkeepTonight())))
}

// viewSweep is the dialog: the account, the field, what the line moves
// tonight and the rules of the sweep.
func (m *Model) viewSweep() string {
	w := m.w
	l := m.rules.Laundering
	off := l.Offshore()
	keep, err := m.sweepKeep()
	if err != nil {
		keep = 0
	}
	state := theme.Subtle.Render("off")
	if sw := w.Laundering.Sweep; sw.On {
		state = theme.Good.Render("on") + ", keeping " + money(max(sw.Keep, m.upkeepTonight()))
	}
	field := m.amountField(max(w.Player.CleanCash, w.Laundering.Sweep.Keep))
	body := []string{
		m.inHand(),
		row("account", theme.Good.Render(cash(w.Offshore))+" offshore"),
		row("sweep", state),
		row("keep", field.View()),
	}
	if n := m.sweepTonight(keep); n > 0 {
		body = append(body, row("tonight", fmt.Sprintf("%s moves, fee %s   %s", theme.Gold.Render(money(n)), money(l.Fee(w, n)), theme.Good.Render("under the lot: nobody reads it"))))
	} else {
		body = append(body, row("tonight", theme.Subtle.Render("nothing over the line to move")))
	}
	if due := m.upkeepTonight(); due > 0 {
		body = append(body, row("upkeep", fmt.Sprintf("%s clean kept back every night", money(due))))
	}
	body = append(body, "")
	body = append(body, m.subtle(fmt.Sprintf("Every night the clean over the line goes offshore, up to %s a day less what you reserve by hand, so it never files a page. The account keeps its fee. Off unless you turn it on.", money(off.Lot)))...)
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("SWEEP OFFSHORE", body, m.modalFooter())
}
