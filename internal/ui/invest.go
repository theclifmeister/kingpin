package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The invest dialog (#192): clean cash into the front under the
// ledger's cursor for levels that earn clean income of their own. One
// page, a number field in levels (blank is one), the price and the new
// income under it, red past the clean cash in hand.
type investDialog struct {
	front  string // front id
	levels numberField
	err    string
}

// ledgerOnFront is the ledger's cursor being on a front: where i invests.
func ledgerOnFront(m *Model) bool {
	return m.screen == screenLedger && m.ledgerSelected().kind == ledgerFront
}

// investFront is the front the dialog is open on, or nil.
func (m *Model) investFront() *game.Front { return m.w.Front(m.inv.front) }

// investMax is the most levels the front can take at once: what is
// left to its top, and never more than the clean cash pays for; at
// least one, so the field can say what the next one costs.
func (m *Model) investMax(f game.Front) int {
	l := m.set.Laundering
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
	l := m.set.Laundering
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
	m.inv = investDialog{front: f.ID, levels: newNumberField("blank = 1")}
	m.inv.levels.max = m.investMax(f)
	m.inv.levels.Focus()
	m.mode = modeInvest
}

// investLevels is the levels the field reads: blank is one.
func (m *Model) investLevels() (int, error) {
	if strings.TrimSpace(m.inv.levels.Value()) == "" {
		return 1, nil
	}
	n, ok := m.inv.levels.Number()
	if !ok || n <= 0 {
		return 0, fmt.Errorf("enter a whole number above zero")
	}
	return n, nil
}

func (m *Model) keyInvest(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	m.inv.err = ""
	f := m.investFront()
	if f == nil {
		m.mode = modePlay
		return m, nil
	}
	switch key {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "enter":
		m.confirmInvest(*f)
		return m, nil
	}
	m.inv.levels.max = m.investMax(*f)
	return m, m.inv.levels.Update(k)
}

// confirmInvest buys the levels the field reads, or shows why it
// cannot.
func (m *Model) confirmInvest(f game.Front) {
	n, err := m.investLevels()
	if err != nil {
		m.inv.err = dialogError(err)
		return
	}
	l := m.set.Laundering
	o := l.Levels(f, n)
	if err := m.w.Invest(o); err != nil {
		m.inv.err = dialogError(err)
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
	l := m.set.Laundering
	n, err := m.investLevels()
	if err != nil {
		n = 1
	}
	levels := m.inv.levels
	levels.max = m.investMax(*f)
	after := *f
	after.Level += n
	cost := l.LevelCost(*f, n)
	style := theme.Gold
	if cost > w.Player.CleanCash {
		style = theme.Bad
	}
	body := []string{
		fmt.Sprintf("Now       level %d of %d · earns %s/day · washes %s/day · upkeep %s/day", f.Level, l.MaxLevel(*f), money(l.Income(*f)), money(l.Throughput(w, *f)), money(l.FrontUpkeep(w, *f))),
		fmt.Sprintf("Levels    %s   %s", levels.View(), theme.Subtle.Render("clean "+cash(w.Player.CleanCash))),
	}
	if f.Level+n <= l.MaxLevel(*f) {
		body = append(body, fmt.Sprintf("Buys      level %d for %s: earns %s/day, washes %s/day, upkeep %s/day, audit %.1f%%/day",
			after.Level, style.Render(money(cost)), theme.Good.Render(money(l.Income(after))), money(l.Throughput(w, after)), money(l.FrontUpkeep(w, after)), l.AuditRisk(w, after)*100))
	} else {
		body = append(body, theme.Bad.Render(fmt.Sprintf("%s takes %s more at most.", f.Name, plural(l.MaxLevel(*f)-f.Level, "level"))))
	}
	g := l.Growth()
	body = append(body, "",
		theme.Subtle.Render("A level earns clean cash on its own, dirty or no dirty, and pays itself back in "+plural(paybackDays(l.LevelCost(*f, 1), l.Income(game.Front{ID: f.ID, Level: 1})), "day")+"."),
		theme.Subtle.Render(fmt.Sprintf("Each level adds %.0f%% to the audit risk; a wash over %.0fx the income is what the auditors find.", g.AuditLevel*100, g.LegitRatio)))
	if g.HeadlineLevel > 0 && f.Level < g.HeadlineLevel {
		body = append(body, theme.Warning.Render(fmt.Sprintf("At level %d the growth makes the paper: pressure and notoriety, never evidence.", g.HeadlineLevel)))
	}
	if m.inv.err != "" {
		body = append(body, "", theme.Bad.Render(m.inv.err))
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
