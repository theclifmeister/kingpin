package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// assignRows are the choices in the assign picker: every city, then
// nobody's, which takes the lieutenant off theirs.
func (m *Model) assignRows() []string {
	return append(append([]string(nil), m.w.CityOrder...), "")
}

// askAssign opens the city picker for the lieutenant under the cursor
// on the crew screen.
func (m *Model) askAssign() {
	c, onPayroll, ok := m.crewSelected()
	switch {
	case !ok || !onPayroll:
		m.refuse("Can't give a city: put the cursor on a lieutenant on the payroll.")
		return
	case !c.Lieutenant():
		m.refuse(fmt.Sprintf("Can't give a city to %s: only a lieutenant runs one, and they are %s.", c.Name, format.A(c.Role)))
		return
	}
	m.subjectID = c.ID
	m.pick.cursor = 0
	for i, cid := range m.assignRows() {
		if cid == c.City {
			m.pick.cursor = i
		}
	}
	m.mode = modeAssign
}

func (m *Model) confirmAssign() {
	rows := m.assignRows()
	m.mode = modePlay
	lt := m.w.Crew.Member(m.subjectID)
	if lt == nil {
		return
	}
	city := rows[max(0, min(m.pick.cursor, len(rows)-1))]
	if city == "" {
		if err := m.sess.Unassign(lt.ID); err != nil {
			m.refuse("Can't take the city back: " + err.Error())
			return
		}
		m.say(fmt.Sprintf("%s runs nothing now. The crew they posted stay where they are.", lt.Name))
		return
	}
	if err := m.sess.Assign(lt.ID, city); err != nil {
		m.refuse("Can't give them the city: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s runs %s from tonight: posts the idle crew, sells the stash, keeps %s.", lt.Name, m.w.CityName(city), format.Pct(m.rules.Crew.Cut(), 0)))
}

func (m *Model) viewAssign() string {
	lt := m.w.Crew.Member(m.subjectID)
	if lt == nil {
		return m.modal("ASSIGN", []string{"They are gone."}, m.modalFooter())
	}
	rows := m.assignRows()
	clamp(&m.pick.cursor, len(rows))
	t := m.rules.Crew.Lieutenancy() // the terms the dialog explains the role with (#455)
	var cells [][]any
	for _, cid := range rows {
		if cid == "" {
			cells = append(cells, []any{"nobody's", nil, nil, styled{theme.Subtle, "take them off the city"}})
			continue
		}
		var runs any = "nobody"
		switch other := m.w.Crew.Lieutenant(cid); {
		case other != nil && other.ID == lt.ID:
			runs = styled{theme.Good, "theirs now"}
		case other != nil:
			runs = styled{theme.Warning, other.Name}
		}
		cells = append(cells, []any{m.w.CityName(cid), heldIn(m.w, cid), m.w.StockIn(cid), runs})
	}
	// Two lines that fit the modal's width.
	return m.pickerModal("ASSIGN "+lt.Name, nil, []col{{"city", kText, 0}, {"corners", kInt, 0}, {"units", kInt, 0}, {"runs", kText, 0}}, cells, m.pick.cursor,
		theme.Subtle.Render("Each night they post the idle crew, drop a corner robbed twice and sell"),
		theme.Subtle.Render(fmt.Sprintf("the stash at their dial; your own order wins. Cut %s, +%d crew slots.", format.Pct(t.Cut, 0), t.Crew)),
		theme.Subtle.Render(fmt.Sprintf("Their temper shows after %s running it: it sets the dial and the heat.", plural(t.RevealDays, "day"))),
		theme.Warning.Render(fmt.Sprintf("Under %.0f loyalty they talk to the police; at %.0f they walk with the city.", t.Flip, t.Quit)),
		"", theme.Subtle.Render(fmt.Sprintf("Which city should %s run?", lt.Name)))
}

// lieutenantHint says how the lieutenants come (#455), once the run has
// reached the tier whose card promises them and until they do: they wait
// on corners held in two cities, and the way to one in a city where you
// hold none, from where you stand, is a runner posted from the map.
// Empty before that tier and once two cities are held.
func (m *Model) lieutenantHint() string {
	w := m.w
	if m.rules.Crew.LieutenantsWanted(w) || !m.reached("distribution") {
		return ""
	}
	for _, cid := range w.CityOrder {
		if heldIn(w, cid) == 0 {
			return fmt.Sprintf("Lieutenants come looking once you hold corners in two cities: post a runner on a corner in %s %s.", w.CityName(cid), screenPointer(screenMap))
		}
	}
	return ""
}

// reached is whether the run has entered the tier with this id.
func (m *Model) reached(id string) bool {
	for n := 1; n <= m.w.Tier(); n++ {
		if t := m.cfg.Progression.Tier(n); t != nil && t.ID == id {
			return true
		}
	}
	return false
}

// heldIn counts the corners the player holds in a city.
func heldIn(w *game.World, city string) int {
	n := 0
	if c := w.Cities[city]; c != nil {
		for _, k := range c.Corners {
			if k.Held() {
				n++
			}
		}
	}
	return n
}

// runsLine is one line per city with a lieutenant, for the dashboard and
// the crew screen: who runs it, and their temper or how long until it
// shows.
func (m *Model) runsLine() string {
	var parts []string
	for _, cid := range m.w.CityOrder {
		if lt := m.w.Crew.Lieutenant(cid); lt != nil {
			part := fmt.Sprintf("%s runs %s", lt.Name, m.w.CityName(cid))
			if lt.Observed {
				part += " (" + lt.Personality + ")"
			} else {
				left := max(1, m.rules.Crew.RevealDays()-(m.w.Day-lt.Assigned))
				part += fmt.Sprintf("; temper unknown for %d more day%s", left, map[bool]string{true: "s"}[left != 1])
			}
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " · ")
}

// standingHere counts the lieutenant's standing orders in the city the
// player is in.
func (m *Model) standingHere() int {
	n := 0
	for _, id := range m.w.Products {
		if _, ok := m.w.DelegatedOrder(m.w.Player.Location, id); ok {
			n++
		}
	}
	return n
}

// keyAssign is the assign picker's keys: the cities a lieutenant runs.
func (m *Model) keyAssign(key string) { m.pickerKey(key, len(m.assignRows()), m.confirmAssign) }
