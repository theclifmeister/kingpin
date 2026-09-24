package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// amountDialog is every one-field dialog (#275): invest (modeInvest),
// reserve (modeReserve), cash out (modeCashOut, #395), pay a cop (modePayCop), buy off
// (modeConfirmBuyOff), fast-forward (modeConfirmFast) and restock
// (modeRestock, #356). Each is one
// page with one number field and the error under it, and only one is
// ever open, so they share Model.amt, which each askX sets whole as it
// opens its dialog. subject is what the dialog is open on where that
// can move under it (the invest dialog's front id), "" for the rest.
type amountDialog struct {
	numberField
	subject string
	err     string
}

func (d *amountDialog) page() int           { return 0 }
func (d *amountDialog) field() *numberField { return &d.numberField }

// openAmount opens a one-field dialog in md: the field with its
// placeholder, its max and its money flag, focused, on subject.
func (m *Model) openAmount(md mode, placeholder string, mx int, isMoney bool, subject string) {
	m.amt = amountDialog{numberField: newNumberField(placeholder), subject: subject}
	m.amt.money = isMoney
	m.amt.max = mx
	m.amt.Focus()
	m.mode = md
}

// keyAmount is every one-field dialog's keys (#275): the error clears
// on any key, esc and q close, enter commits (every number dialog
// commits on enter; y is a confirmation's yes, #241), and the rest goes
// to the field with its max set first, since the max moves with the
// world (digits, backspace and the field's shortcuts; a letter never
// lands in it).
func (m *Model) keyAmount(k tea.KeyMsg, mx func() int, commit func()) (tea.Model, tea.Cmd) {
	m.amt.err = ""
	switch k.String() {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "enter":
		commit()
		return m, nil
	}
	m.amt.max = mx()
	return m, m.amt.Update(k)
}

// amountField is the dialog's field as its view draws it: a copy with
// the max the world gives it now.
func (m *Model) amountField(mx int) numberField {
	f := m.amt.numberField
	f.max = mx
	return f
}
