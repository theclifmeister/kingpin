package ui

import (
	"fmt"
	"strings"

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
		m.status = "Move the cursor to a lieutenant on the payroll, then press t."
		return
	case !c.Lieutenant():
		m.status = fmt.Sprintf("%s is a %s. Only a lieutenant can run a city.", c.Name, c.Role)
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
			m.status = "Can't: " + err.Error()
			return
		}
		m.status = fmt.Sprintf("%s runs nothing now. The crew they posted stay where they are.", lt.Name)
		return
	}
	if err := m.w.Assign(lt.ID, city); err != nil {
		m.status = "Can't: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s runs %s from tonight: posts the idle crew, sells the stash, keeps %.0f%%.", lt.Name, m.w.CityName(city), m.set.Crew.Cut()*100)
}

func (m *Model) viewAssign() string {
	lt := m.w.Crew.Member(m.fireID)
	if lt == nil {
		return m.modal("ASSIGN", "They are gone.")
	}
	rows := m.assignRows()
	m.assignCursor = max(0, min(m.assignCursor, len(rows)-1))
	var b strings.Builder
	for i, cid := range rows {
		label, note := "nobody's", theme.Subtle.Render("take them off the city")
		if cid != "" {
			label = m.w.CityName(cid)
			switch other := m.w.Crew.Lieutenant(cid); {
			case other != nil && other.ID == lt.ID:
				note = theme.Good.Render("theirs now")
			case other != nil:
				note = theme.Warning.Render(other.Name + " runs it")
			default:
				note = theme.Subtle.Render(fmt.Sprintf("%d corner(s) held, %d units stashed", heldIn(m.w, cid), m.w.Player.StockIn(cid)))
			}
		}
		line := fmt.Sprintf("%-12s", fit(label, 12))
		if i == m.assignCursor {
			b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(line) + " " + note + "\n")
		} else {
			b.WriteString("  " + line + " " + note + "\n")
		}
	}
	b.WriteString("\n" + theme.Subtle.Render(fmt.Sprintf("Every night they post the idle crew on the city's corners, give up a corner\nrobbed twice, and sell the stash there at their dial; your own order wins.\nThey keep %.0f%% of the city's takings and bring %d people of their own.", m.set.Crew.Cut()*100, m.cfg.Crew.Role[game.RoleLieutenant].Crew)) + "\n")
	b.WriteString("\n" + theme.Subtle.Render(fmt.Sprintf("Which city should %s run?  ", lt.Name)) + theme.Key.Render("enter") + " assign  " + theme.Key.Render("esc") + " back")
	return m.modal("ASSIGN "+strings.ToUpper(lt.Name), strings.TrimRight(b.String(), "\n"))
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

// temper is a lieutenant's personality as the roster shows it: the word
// once you have seen enough of them, a question mark until then.
func (m *Model) temper(c game.CrewMember) string {
	if !c.Lieutenant() {
		return ""
	}
	if c.Observed {
		return c.Personality
	}
	return "?"
}

// runsLine is one line per city with a lieutenant, for the dashboard.
func (m *Model) runsLine() string {
	var parts []string
	for _, cid := range m.w.CityOrder {
		if lt := m.w.Crew.Lieutenant(cid); lt != nil {
			part := fmt.Sprintf("%s runs %s", lt.Name, m.w.CityName(cid))
			if lt.Observed {
				part += " (" + lt.Personality + ")"
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
		if _, ok := m.w.StandingOrder(m.w.Player.Location, id); ok {
			n++
		}
	}
	return n
}
