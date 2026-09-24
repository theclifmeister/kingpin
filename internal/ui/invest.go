package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The invest dialog (#192): clean cash into the front under the
// ledger's cursor for levels that earn clean income of their own. One
// page, a number field in levels (blank is one), the price and the new
// income under it, red past the clean cash in hand. It is an
// amountDialog (#275) on the front's id.

// ledgerOnFront is the ledger's cursor being on a front: where u invests (#241).
func ledgerOnFront(m *Model) bool {
	return m.screen == screenLedger && m.ledgerSelected().kind == ledgerFront
}

// investFront is the front the dialog is open on, or nil.
func (m *Model) investFront() *game.Front { return m.w.Front(m.amt.subject) }

// investMax is the most levels the front can take at once: what is
// left to its top, and never more than the clean cash pays for; at
// least one, so the field can say what the next one costs.
func (m *Model) investMax(f game.Front) int {
	l := m.rules.Laundering
	room := l.MaxLevel(f) - f.Level
	n := 0
	for n < room && l.LevelCost(f, n+1) <= m.w.Player.CleanCash {
		n++
	}
	return max(1, n)
}

// askInvest opens the invest dialog on the front under the cursor.
func (m *Model) askInvest() {
	if m.w.Over != nil {
		return
	}
	sel := m.ledgerSelected()
	if sel.kind != ledgerFront {
		return
	}
	f := m.w.Fronts[sel.i]
	l := m.rules.Laundering
	if l.MaxLevel(f) == 0 {
		m.refuse(fmt.Sprintf("Can't invest in %s: it is what it is.", f.Name))
		return
	}
	if f.Level >= l.MaxLevel(f) {
		m.refuse(fmt.Sprintf("Can't invest in %s: it is as big as it gets (level %d).", f.Name, f.Level))
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.refuse("Can't invest: a level is bought with clean cash, and you have none.")
		return
	}
	m.openAmount(modeInvest, "blank = 1", m.investMax(f), false, f.ID)
}

// investLevels is the levels the field reads: blank is one.
func (m *Model) investLevels() (int, error) { return m.amt.Read(1) }

func (m *Model) keyInvest(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.investFront()
	if f == nil {
		m.amt.err = ""
		m.mode = modePlay
		return m, nil
	}
	return m.keyAmount(k, func() int { return m.investMax(*f) }, func() { m.confirmInvest(*f) })
}

// confirmInvest buys the levels the field reads, or shows why it
// cannot.
func (m *Model) confirmInvest(f game.Front) {
	n, err := m.investLevels()
	if err != nil {
		m.amt.err = dialogError(err)
		return
	}
	l := m.rules.Laundering
	o := l.Levels(f, n)
	if err := m.sess.Invest(f.ID, n); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
	now := *m.w.Front(f.ID)
	m.say(fmt.Sprintf("Invested %s clean in %s: level %d, earning %s/day clean from tomorrow.", money(o.Cost), f.Name, now.Level, money(l.Income(now))))
}

// viewInvest is the dialog: the front and its level, the field, what
// the levels cost and earn, and the clean cash in hand.
func (m *Model) viewInvest() string {
	f := m.investFront()
	if f == nil {
		return m.modal("INVEST", []string{"Nothing to invest in."}, m.modalFooter())
	}
	w := m.w
	l := m.rules.Laundering
	n, err := m.investLevels()
	if err != nil {
		n = 1
	}
	levels := m.amountField(m.investMax(*f))
	after := *f
	after.Level += n
	cost := l.LevelCost(*f, n)
	style := theme.Gold
	if cost > w.Player.CleanCash {
		style = theme.Bad
	}
	var body []string
	if r := m.cfg.Upgrades.Front(f.ID); r != nil && r.Role != "" {
		body = append(m.wrapLines(r.Role), "") // what the place is for (#344)
	}
	body = append(body,
		m.inHand(),
		row("now", fmt.Sprintf("level %d of %d · earns %s/day · washes %s/day · upkeep %s/day", f.Level, l.MaxLevel(*f), money(l.Income(*f)), money(l.Throughput(w, *f)), money(l.FrontUpkeep(w, *f)))),
		row("levels", levels.View()))
	if f.Level+n <= l.MaxLevel(*f) {
		body = append(body, row("buys", fmt.Sprintf("level %d for %s: earns %s/day, washes %s/day, upkeep %s/day, audit %s/day",
			after.Level, style.Render(money(cost)), theme.Good.Render(money(l.Income(after))), money(l.Throughput(w, after)), money(l.FrontUpkeep(w, after)), format.Pct(l.AuditRisk(w, after), 1))))
	} else {
		body = append(body, theme.Bad.Render(fmt.Sprintf("%s takes %s more at most.", f.Name, plural(l.MaxLevel(*f)-f.Level, "level"))))
	}
	g := l.Growth()
	body = append(body, "",
		theme.Subtle.Render("A level earns clean cash on its own, dirty or no dirty, and pays itself back in "+plural(paybackDays(l.LevelCost(*f, 1), l.Income(game.Front{ID: f.ID, Level: 1})), "day")+"."),
		theme.Subtle.Render(fmt.Sprintf("Each level adds %s to the audit risk; a wash over %.0fx the income is what the auditors find.", format.Pct(g.AuditLevel, 0), g.LegitRatio)))
	if g.HeadlineLevel > 0 && f.Level < g.HeadlineLevel {
		body = append(body, theme.Warning.Render(fmt.Sprintf("At level %d the growth makes the paper: pressure and notoriety, never evidence.", g.HeadlineLevel)))
	}
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("INVEST · "+f.Name, body, m.modalFooter())
}

// paybackDays is how many days a level takes to earn its price back.
func paybackDays(cost, income int) int {
	if income <= 0 {
		return 0
	}
	return (cost + income - 1) / income
}
