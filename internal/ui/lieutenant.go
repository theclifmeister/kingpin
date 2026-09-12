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
	m.fireID = c.ID
	m.assignCursor = 0
	for i, cid := range m.assignRows() {
		if cid == c.City {
			m.assignCursor = i
		}
	}
	m.mode = modeAssign
}

func (m *Model) confirmAssign() {
	rows := m.assignRows()
	m.mode = modePlay
	lt := m.w.Crew.Member(m.fireID)
	if lt == nil {
		return
	}
	city := rows[max(0, min(m.assignCursor, len(rows)-1))]
	if city == "" {
		if err := m.w.Unassign(lt.ID); err != nil {
			m.refuse("Can't take the city back: " + err.Error())
			return
		}
		m.say(fmt.Sprintf("%s runs nothing now. The crew they posted stay where they are.", lt.Name))
		return
	}
	if err := m.w.Assign(lt.ID, city); err != nil {
		m.refuse("Can't give them the city: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s runs %s from tonight: posts the idle crew, sells the stash, keeps %.0f%%.", lt.Name, m.w.CityName(city), m.set.Crew.Cut()*100))
}

func (m *Model) viewAssign() string {
	lt := m.w.Crew.Member(m.fireID)
	if lt == nil {
		return m.modal("ASSIGN", []string{"They are gone."}, m.modalFooter())
	}
	rows := m.assignRows()
	m.assignCursor = max(0, min(m.assignCursor, len(rows)-1))
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
		cells = append(cells, []any{m.w.CityName(cid), heldIn(m.w, cid), m.w.Player.StockIn(cid), runs})
	}
	m.modalFollow(1 + m.assignCursor) // under the header
	body := table([]col{{"city", kText, 0}, {"corners", kInt, 0}, {"units", kInt, 0}, {"runs", kText, 0}}, cells, m.assignCursor, m.modalInner())
	// Two lines that fit the modal's width.
	body = append(body, "",
		theme.Subtle.Render("Each night they post the idle crew, drop a corner robbed twice and sell"),
		theme.Subtle.Render(fmt.Sprintf("the stash at their dial; your own order wins. Cut %.0f%%, +%d crew slots.", m.set.Crew.Cut()*100, m.cfg.Crew.Role[game.RoleLieutenant].Crew)),
		"", theme.Subtle.Render(fmt.Sprintf("Which city should %s run?", lt.Name)))
	return m.modal("ASSIGN "+lt.Name, body, m.modalFooter())
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
				left := max(1, m.set.Crew.RevealDays()-(m.w.Day-lt.Assigned))
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
