package ui

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The captain (#346): a trusted member named to run crew care for a
// city. c on the crew screen opens the picker: the cities, then
// nobody's, which takes the captaincy off them; ←→ turns the nightly
// pay-off budget through crew.toml [captain] budgets.

// captainRows are the choices in the captain picker: every city, then
// nobody's.
func (m *Model) captainRows() []string {
	return append(append([]string(nil), m.w.CityOrder...), "")
}

// budgets is the pay-off budgets a night the picker turns through.
func (m *Model) budgets() []int { return m.rules.Crew.Captaincy().Budgets }

// askCaptain opens the city picker for the member under the cursor, or
// says why they cannot be captain.
func (m *Model) askCaptain() {
	c, onPayroll, ok := m.crewSelected()
	cp := m.rules.Crew.Captaincy()
	switch {
	case !ok || !onPayroll:
		m.refuse("Can't name a captain: put the cursor on somebody on the payroll.")
		return
	case c.Lieutenant():
		m.refuse(fmt.Sprintf("Can't make %s captain: a lieutenant runs a city. Give them one with l.", c.Name))
		return
	case c.Captain == "" && !m.rules.Crew.CanCaptain(m.w, c):
		m.refuse(fmt.Sprintf("Can't make %s captain yet: it takes loyalty %.0f and %s on the payroll, at work.", c.Name, cp.Loyalty, plural(cp.Days, "day")))
		return
	}
	m.subjectID = c.ID
	m.pick.cursor, m.pick.budget = 0, 0
	at := c.Captain // theirs, else where they work, else where you stand
	if at == "" {
		at = m.w.Player.Location
		if p := m.w.PostOf(c.ID); p != nil {
			at = p.City
		}
	}
	for i, cid := range m.captainRows() {
		if cid == at {
			m.pick.cursor = i
		}
	}
	for i, b := range m.budgets() {
		if c.Captain != "" && b == c.Budget {
			m.pick.budget = i
		}
	}
	m.mode = modeCaptain
}

// confirmCaptain names them for the city under the cursor at the budget
// shown, or takes it off them.
func (m *Model) confirmCaptain() {
	rows := m.captainRows()
	m.mode = modePlay
	c := m.w.Crew.Member(m.subjectID)
	if c == nil {
		return
	}
	city := rows[max(0, min(m.pick.cursor, len(rows)-1))]
	if city == "" {
		if c.Captain == "" {
			m.say(fmt.Sprintf("%s is nobody's captain.", c.Name))
			return
		}
		if err := m.sess.DropCaptain(c.ID); err != nil {
			m.refuse("Can't take the captaincy off them: " + err.Error())
			return
		}
		m.say(fmt.Sprintf("%s is one of the crew again. Nobody looks after them tonight.", c.Name))
		return
	}
	budget := m.budget()
	if err := m.sess.NameCaptain(c.ID, city, budget); err != nil {
		m.refuse("Can't make them captain: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s is captain of %s from tonight: %s a night for pay-offs, keeps %s.", c.Name, m.w.CityName(city), money(budget), format.Pct(m.rules.Crew.Captaincy().Cut, 0)))
}

// budget is the budget the picker shows.
func (m *Model) budget() int {
	bs := m.budgets()
	if len(bs) == 0 {
		return 0
	}
	return bs[max(0, min(m.pick.budget, len(bs)-1))]
}

func (m *Model) viewCaptain() string {
	c := m.w.Crew.Member(m.subjectID)
	if c == nil {
		return m.modal("CAPTAIN", []string{"They are gone."}, m.modalFooter())
	}
	rows := m.captainRows()
	clamp(&m.pick.cursor, len(rows))
	var cells [][]any
	for _, cid := range rows {
		if cid == "" {
			cells = append(cells, []any{"nobody's", nil, styled{theme.Subtle, "take the captaincy off them"}})
			continue
		}
		var who any = "nobody"
		switch other := m.w.Crew.Captain(cid); {
		case other != nil && other.ID == c.ID:
			who = styled{theme.Good, "theirs now"}
		case other != nil:
			who = styled{theme.Warning, other.Name}
		}
		cells = append(cells, []any{m.w.CityName(cid), heldIn(m.w, cid), who})
	}
	cp := m.rules.Crew.Captaincy()
	budget := theme.Gold.Render(money(m.budget())) + theme.Subtle.Render(" a night  ←→")
	if m.budget() == 0 {
		budget = theme.Warning.Render("nothing") + theme.Subtle.Render(": no pay-offs  ←→")
	}
	return m.pickerModal("CAPTAIN "+c.Name, []string{row("budget", budget), ""}, []col{{"city", kText, 0}, {"corners", kInt, 0}, {"captain", kText, 0}}, cells, m.pick.cursor,
		theme.Subtle.Render("Each night: pull a suspected skimmer off a corner, post the idle"),
		theme.Subtle.Render(fmt.Sprintf("runners on held corners, pay off one near the walk-out. Cut %s.", format.Pct(cp.Cut, 0))),
		"", theme.Subtle.Render(fmt.Sprintf("Which city should %s look after?", c.Name)))
}

// keyCaptain is the captain picker's keys: the cities, and ←→ the
// budget.
func (m *Model) keyCaptain(key string) {
	switch key {
	case "left":
		stepCursor(&m.pick.budget, -1, len(m.budgets()))
	case "right":
		stepCursor(&m.pick.budget, 1, len(m.budgets()))
	default:
		m.pickerKey(key, len(m.captainRows()), m.confirmCaptain)
	}
}

// traitCell is the roster's trait column (#346): the word, green for a
// strength and amber for a flaw; nothing before one shows.
func (m *Model) traitCell(c game.CrewMember) any {
	if c.Trait == "" {
		return nil
	}
	if m.rules.Crew.Trait(c.Trait).Good {
		return styled{theme.Good, c.Trait}
	}
	return styled{theme.Warning, c.Trait}
}

// veteranLines are the pane's lines for a member's service (#346): the
// trait they showed and the days served, what it does, or how long
// until it shows; and the city they are captain of, at what budget.
func (m *Model) veteranLines(c game.CrewMember) []string {
	var lines []string
	served := m.w.Day - c.Hired
	switch days := m.rules.Crew.TraitDays(); {
	case c.Trait != "":
		tr := m.rules.Crew.Trait(c.Trait)
		style := theme.Warning
		if tr.Good {
			style = theme.Good
		}
		lines = append(lines, row("trait", style.Render(c.Trait)+theme.Subtle.Render(" · served "+plural(served, "day"))))
		if tr.Says != "" {
			lines = append(lines, theme.Subtle.Render("  "+tr.Says))
		}
	case days > 0:
		lines = append(lines, row("trait", theme.Subtle.Render("known in "+plural(max(1, days-served), "day"))))
	}
	if c.Captain != "" {
		lines = append(lines, row("captain", fmt.Sprintf("%s · %s a night", m.w.CityName(c.Captain), money(c.Budget))))
	}
	return lines
}
