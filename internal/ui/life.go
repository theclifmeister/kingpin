package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Crew life on the screens (#46): the crew screen's tags, ages and kin,
// the bail, and the driver on a route.

// crewTag is the one-word state a member is in for the roster's where
// column, "" for one at work: jailed with the days to go (out tomorrow
// once bail is down), laid up with the days, retiring for one whose
// next birthday is the farewell. Lowercase, as every status in a table
// is (#238: `jailed` is the word in tables and summary rows, `a cell`
// prose only).
func (m *Model) crewTag(c game.CrewMember) string {
	day := m.w.Day
	switch {
	case c.Jailed(day) && c.Bailed:
		return "out tomorrow"
	case c.Jailed(day):
		return fmt.Sprintf("jailed %dd", c.JailedUntil-day)
	case c.Wounded(day):
		return fmt.Sprintf("laid up %dd", c.WoundedUntil-day) // the words of the refusal and the pane (#468)
	case m.rules.Crew.Retiring(c):
		return "retiring"
	}
	return ""
}

// kinGlyph marks a name whose owner has kin on the payroll or in the
// pool.
const kinGlyph = "♦"

// kinNames lists the names of a member's kin, on the payroll or looking
// for work, with the relation the pair has.
func (m *Model) kinNames(c game.CrewMember) []string {
	var out []string
	for _, id := range c.Kin {
		var k *game.CrewMember
		if k = m.w.Crew.Member(id); k == nil {
			for i := range m.w.Crew.Candidates {
				if m.w.Crew.Candidates[i].ID == id {
					k = &m.w.Crew.Candidates[i]
				}
			}
		}
		if k == nil {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s)", k.Name, relation(c.ID, k.ID)))
	}
	return out
}

// relation is the word for a pair of kin: a cousin, a partner or a
// friend, fixed by the pair and nothing saved.
func relation(a, b int) string {
	switch (a + b) % 3 {
	case 0:
		return "cousin"
	case 1:
		return "partner"
	}
	return "friend"
}

// ageLine is the pane's age row: the age, and when they retire.
func (m *Model) ageLine(c game.CrewMember) string {
	life := m.rules.Crew.Life()
	if !life.On() || c.Age == 0 {
		return ""
	}
	s := fmt.Sprintf("%d", c.Age)
	switch {
	case m.rules.Crew.Retiring(c):
		s += theme.Warning.Render(fmt.Sprintf(" · retires d%d", m.rules.Crew.Birthday(c, m.w.Day)))
	default:
		s += theme.Subtle.Render(fmt.Sprintf(" · retires at %d", life.RetireAge))
	}
	return s
}

func (m *Model) askBail() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.refuse("Can't bail: put the cursor on somebody on the payroll.")
		return
	}
	if !c.Jailed(m.w.Day) {
		m.refuse(fmt.Sprintf("Can't bail %s: they are not in a cell.", c.Name))
		return
	}
	if c.Bailed {
		m.refuse(fmt.Sprintf("Bail is already down for %s: they walk tomorrow.", c.Name))
		return
	}
	m.subjectID = c.ID
	m.ask("bail", (*Model).bailConfirm, (*Model).confirmBail)
}

func (m *Model) confirmBail() {
	m.mode = modePlay
	c := m.w.Crew.Member(m.subjectID)
	if c == nil {
		return
	}
	got, err := m.sess.Bail(c.ID)
	if err != nil {
		m.refuse("Can't bail them: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Bail is down for %s: they walk tomorrow, and they know who paid.", got.Name))
}

func (m *Model) bailConfirm() string {
	c := m.w.Crew.Member(m.subjectID)
	if c == nil {
		return m.modal("BAIL", []string{"They are gone."}, m.modalFooter())
	}
	cost := m.rules.Crew.BailCost(*c)
	life := m.rules.Crew.Life()
	body := []string{
		fmt.Sprintf("%s clean for %s: out tomorrow, loyalty %.0f %s %.0f.", money(cost), c.Name, c.Loyalty, format.Arrow, min(100, c.Loyalty+life.BailLoyalty)),
		theme.Subtle.Render(fmt.Sprintf("Left in, they are out in %s, sour, and the DA has had them a while.", plural(c.JailedUntil-m.w.Day, "day"))),
	}
	if cost > m.w.Player.CleanCash {
		body = append(body, theme.Bad.Render(fmt.Sprintf("Only %s clean: a bondsman takes a cheque, never the bag.", money(m.w.Player.CleanCash))))
	}
	return m.modal("BAIL "+c.Name+"?", body, m.modalFooter())
}

// driverRows are the choices in the driver picker: nobody, then every
// driver on the payroll.
func (m *Model) driverRows() []game.CrewMember {
	rows := []game.CrewMember{{ID: 0, Name: "Nobody", Role: game.RoleDriver}}
	for _, c := range m.w.Crew.Members {
		if c.Role == game.RoleDriver {
			rows = append(rows, c)
		}
	}
	return rows
}

func (m *Model) askDriver() {
	r := m.selectedRoute()
	if r == nil || m.w.Over != nil {
		return
	}
	if len(m.driverRows()) == 1 {
		m.refuse("Nobody to put on the road: no drivers. Hire one " + screenPointer(screenCrew) + ".")
		return
	}
	m.pick.cursor = 0
	for i, c := range m.driverRows() {
		if c.ID != 0 && m.w.Route(r.ID).Driver == c.ID {
			m.pick.cursor = i
		}
	}
	m.mode = modeDriver
}

func (m *Model) confirmDriver() {
	r := m.selectedRoute()
	rows := m.driverRows()
	m.mode = modePlay
	if r == nil || len(rows) == 0 {
		return
	}
	who := rows[max(0, min(m.pick.cursor, len(rows)-1))]
	if err := m.sess.SetRouteDriver(r.ID, who.ID); err != nil {
		m.refuse("Can't put them on the road: " + err.Error())
		return
	}
	if who.ID == 0 {
		m.say("Nobody drives " + r.Name + " now.")
		return
	}
	m.say(fmt.Sprintf("%s drives %s from tomorrow: risk −%s a day on the road, and jailed if a shipment is seized.", who.Name, r.Name, format.Pct(m.rules.Logistics.DriverCut(who.Skill), 0)))
}

func (m *Model) keyDriver(key string) { m.pickerKey(key, len(m.driverRows()), m.confirmDriver) }

func (m *Model) viewDriver() string {
	r := m.selectedRoute()
	rows := m.driverRows()
	if r == nil {
		return m.modal("DRIVER", []string{"No route selected."}, m.modalFooter())
	}
	clamp(&m.pick.cursor, len(rows))
	var cells [][]any
	for _, c := range rows {
		var where any = styled{theme.Subtle, "idle"}
		var skill, cut any = c.Skill, "−" + format.Pct(m.rules.Logistics.DriverCut(c.Skill), 0)
		switch {
		case c.ID == 0:
			where, skill, cut = styled{theme.Subtle, "takes the driver off"}, nil, nil
		case m.w.Route(r.ID).Driver == c.ID:
			where = styled{theme.Good, "drives it now"}
		case m.w.DrivenRoute(c.ID) != "":
			where = styled{theme.Warning, "on " + m.routeName(m.w.DrivenRoute(c.ID)) + ", will move"}
		case !c.Fit(m.w.Day):
			where = styled{theme.Bad, strings.ToLower(m.crewTag(c))}
		}
		cells = append(cells, []any{c.Name, skill, cut, where})
	}
	return m.pickerModal("DRIVER · "+strings.ToUpper(r.Name), nil, []col{{"name", kText, 0}, {"skill", kInt, 0}, {"risk", kText, 0}, {"where", kText, 0}}, cells, m.pick.cursor, m.subtle(fmt.Sprintf("Who drives %s? They ride every shipment on it, and a seized one takes them with it.", r.Name))...)
}

// routeName is a route's name by id, or the id.
func (m *Model) routeName(id string) string {
	if r := m.rules.Logistics.Route(id); r != nil {
		return r.Name
	}
	return id
}

// driverLine is the route pane's driver row: who drives it and what
// they take off the risk, or that nobody does.
func (m *Model) driverLine(route string) string {
	rs := m.w.Route(route)
	if rs.Driver == 0 {
		return theme.Subtle.Render("nobody")
	}
	c := m.w.Crew.Driver(rs.Driver)
	if c == nil {
		return theme.Warning.Render("nobody: the driver is gone")
	}
	if !c.Fit(m.w.Day) {
		return theme.Warning.Render(fmt.Sprintf("%s · %s", c.Name, strings.ToLower(m.crewTag(*c))))
	}
	return fmt.Sprintf("%s · risk −%s", c.Name, format.Pct(m.rules.Logistics.DriverCut(c.Skill), 0))
}
