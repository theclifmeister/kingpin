package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// mapSelected returns the corner under the map cursor.
func (m *Model) mapSelected() *game.Corner {
	cs := m.w.Territory.Corners
	if len(cs) == 0 {
		return nil
	}
	m.mapCursor = max(0, min(m.mapCursor, len(cs)-1))
	return &cs[m.mapCursor]
}

// ownerStyle is the colour a corner is drawn in: crew blue for yours,
// rivals purple for theirs, dim for nobody's.
func ownerStyle(owner string) lipgloss.Style {
	switch owner {
	case game.OwnerPlayer:
		return lipgloss.NewStyle().Foreground(theme.Crew)
	case game.OwnerRival:
		return lipgloss.NewStyle().Foreground(theme.Rivals)
	default:
		return theme.Subtle
	}
}

// workerName is who is posted with an id, for the map.
func (m *Model) workerName(id int) string {
	switch id {
	case 0:
		return ""
	case game.You:
		return "you"
	}
	if c := m.w.Crew.Member(id); c != nil {
		return c.Name
	}
	return "?"
}

// cornerUnits is how many units a day a corner adds across every product
// the supplier lists, at today's per-corner demand.
func (m *Model) cornerUnits(c game.Corner) float64 {
	n := 0.0
	for _, id := range m.w.Products {
		n += m.w.Market[id].Demand * c.Share(id)
	}
	return n
}

// postRows lists who can be posted for a role: you and every runner, or
// every enforcer.
func (m *Model) postRows(role string) []game.CrewMember {
	var rows []game.CrewMember
	if role == "runner" {
		rows = append(rows, game.CrewMember{ID: game.You, Name: "You", Role: "runner", Skill: 0})
	}
	for _, c := range m.w.Crew.Members {
		if c.Role == role {
			rows = append(rows, c)
		}
	}
	return rows
}

// askPost opens the picker for posting a role on the selected corner.
func (m *Model) askPost(role string) {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Owner == game.OwnerRival {
		m.status = "Somebody else holds " + c.Name + "."
		return
	}
	if len(m.postRows(role)) == 0 {
		m.status = "No " + role + "s to post. Hire one on the crew screen (4)."
		return
	}
	m.postRole = role
	m.postCursor = 0
	m.mode = modePost
}

func (m *Model) confirmPost() {
	c := m.mapSelected()
	rows := m.postRows(m.postRole)
	m.mode = modePlay
	if c == nil || len(rows) == 0 {
		return
	}
	who := rows[max(0, min(m.postCursor, len(rows)-1))]
	if err := m.w.Post(c.ID, who.ID); err != nil {
		m.status = "Can't post: " + err.Error()
		return
	}
	if who.ID == game.You {
		m.status = fmt.Sprintf("You are working %s now.", c.Name)
	} else if m.postRole == "enforcer" {
		m.status = fmt.Sprintf("%s is guarding %s.", who.Name, c.Name)
	} else {
		m.status = fmt.Sprintf("%s is working %s.", who.Name, c.Name)
	}
}

func (m *Model) abandonSelected() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if err := m.w.Abandon(c.ID); err != nil {
		m.status = "Can't abandon: " + err.Error()
		return
	}
	m.status = c.Name + " goes back to the street."
}

func (m *Model) viewPost() string {
	c := m.mapSelected()
	rows := m.postRows(m.postRole)
	if c == nil || len(rows) == 0 {
		return m.modal("POST", "Nobody to post.")
	}
	m.postCursor = max(0, min(m.postCursor, len(rows)-1))
	var b strings.Builder
	for i, r := range rows {
		where := theme.Subtle.Render("idle")
		if p := m.w.PostOf(r.ID); p != nil {
			if p.ID == c.ID {
				where = theme.Good.Render("already here")
			} else {
				where = theme.Warning.Render("on " + p.Name + ", will move")
			}
		}
		skill := fmt.Sprintf("skill %2d", r.Skill)
		if r.ID == game.You {
			skill = "in person"
		}
		line := fmt.Sprintf("%-8s %-10s ", fit(r.Name, 8), skill)
		if i == m.postCursor {
			b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(line) + " " + where + "\n")
		} else {
			b.WriteString("  " + line + " " + where + "\n")
		}
	}
	what := "work"
	if m.postRole == "enforcer" {
		what = "guard"
	}
	b.WriteString("\n" + theme.Subtle.Render(fmt.Sprintf("Who should %s %s?  ", what, c.Name)) + theme.Key.Render("enter") + " post  " + theme.Key.Render("esc") + " back")
	return m.modal("POST A "+strings.ToUpper(m.postRole), strings.TrimRight(b.String(), "\n"))
}

func (m *Model) viewMap() string {
	w := m.w
	cs := w.Territory.Corners
	sel := m.mapSelected()
	var b strings.Builder

	unserved := 0.0
	for _, c := range cs {
		if !c.Worked() {
			unserved += m.cornerUnits(c)
		}
	}
	b.WriteString(truncate(theme.PanelTitle.Render("MAP · "+w.City)+
		theme.Subtle.Render(fmt.Sprintf("  %d/%d held · %d worked · ~%.0f units/day unworked", w.Held(), len(cs), w.Worked(), unserved)), m.width) + "\n\n")

	// The grid. Cells are laid out by their x, y; the body width decides
	// how wide a cell can be.
	cols, rows := 0, 0
	for _, c := range cs {
		cols = max(cols, c.X+1)
		rows = max(rows, c.Y+1)
	}
	cellW := max(12, min(24, (m.width-2)/max(1, cols)))
	grid := map[[2]int]*game.Corner{}
	for i := range cs {
		grid[[2]int{cs[i].X, cs[i].Y}] = &cs[i]
	}
	for y := 0; y < rows; y++ {
		var l1, l2, l3 []string
		for x := 0; x < cols; x++ {
			c := grid[[2]int{x, y}]
			if c == nil {
				l1 = append(l1, strings.Repeat(" ", cellW))
				l2 = append(l2, strings.Repeat(" ", cellW))
				l3 = append(l3, strings.Repeat(" ", cellW))
				continue
			}
			st := ownerStyle(c.Owner)
			mark := "·"
			if c.Held() {
				mark = "▪"
			} else if c.Owner == game.OwnerRival {
				mark = "▴"
			}
			name := fit(mark+" "+strings.ToUpper(c.Name), cellW-1)
			if sel != nil && c.ID == sel.ID {
				name = theme.Selected.Render(name)
			} else {
				name = st.Render(name)
			}
			var who string
			switch {
			case c.Worked():
				who = m.workerName(c.Runner)
				if c.Enforcer != 0 {
					who += " ⚔ " + m.workerName(c.Enforcer)
				}
				who = st.Render(fit("  "+who, cellW-1))
			case c.Held():
				who = theme.Warning.Render(fit(fmt.Sprintf("  nobody, %dd left", max(1, m.set.Territory.Tuning().DriftDays-c.Idle)), cellW-1))
			case c.Owner == game.OwnerRival:
				who = st.Render(fit("  theirs", cellW-1))
			default:
				who = theme.Subtle.Render(fit("  free", cellW-1))
			}
			l1 = append(l1, name+" ")
			l2 = append(l2, who+" ")
			l3 = append(l3, theme.Subtle.Render(fit(fmt.Sprintf("  ~%.0f/day %s", m.cornerUnits(*c), heatWord(c.Heat)), cellW-1))+" ")
		}
		b.WriteString(" " + strings.Join(l1, "") + "\n" + " " + strings.Join(l2, "") + "\n" + " " + strings.Join(l3, "") + "\n\n")
	}

	// The inspector for the selected corner.
	if sel == nil {
		return b.String()
	}
	st := ownerStyle(sel.Owner)
	head := st.Bold(true).Render(strings.ToUpper(sel.Name))
	switch {
	case sel.Held():
		head += theme.Subtle.Render(fmt.Sprintf("  yours since day %d", sel.Since))
	case sel.Owner == game.OwnerRival:
		head += theme.Subtle.Render("  held by a rival")
	default:
		head += theme.Subtle.Render("  free")
	}
	b.WriteString(truncate(head, m.width) + "\n")
	facts := fmt.Sprintf("  size x%.1f · heat x%.1f %s · risk x%.1f %s", sel.Demand, sel.Heat, heatWord(sel.Heat), sel.Risk, riskWord(sel.Risk))
	if sel.Held() {
		facts += fmt.Sprintf(" · robbery %.1f%%/day", m.set.Territory.RobberyChance(w, sel)*100)
	}
	b.WriteString(truncate(theme.Subtle.Render(facts), m.width) + "\n")
	var dem []string
	ids := append([]string(nil), w.Products...)
	sort.SliceStable(ids, func(i, j int) bool {
		return sel.Share(ids[i])*w.Market[ids[i]].Demand > sel.Share(ids[j])*w.Market[ids[j]].Demand
	})
	for _, id := range ids {
		dem = append(dem, fmt.Sprintf("%s ~%.0f", w.ProductName(id), w.Market[id].Demand*sel.Share(id)))
	}
	b.WriteString("  demand   " + truncate(strings.Join(dem, " · "), max(10, m.width-12)) + "\n")
	runner, enforcer := m.workerName(sel.Runner), m.workerName(sel.Enforcer)
	if runner == "" {
		runner = theme.Subtle.Render("nobody")
	} else if c := w.Crew.Member(sel.Runner); c != nil {
		runner += fmt.Sprintf(" (skill %d)", c.Skill)
	}
	if enforcer == "" {
		enforcer = theme.Subtle.Render("nobody")
	} else if c := w.Crew.Member(sel.Enforcer); c != nil {
		enforcer += fmt.Sprintf(" (skill %d)", c.Skill)
	}
	b.WriteString(truncate(fmt.Sprintf("  runner   %s   enforcer  %s", runner, enforcer), m.width) + "\n")
	var hint string
	switch {
	case sel.Worked():
		hint = theme.Subtle.Render("  c move a runner here · e post an enforcer · a abandon the corner")
	case sel.Held():
		hint = theme.Warning.Render("  Nobody is working it: it goes back to the street unless you post a runner (c).")
	case sel.Owner == game.OwnerRival:
		hint = theme.Subtle.Render("  Taking it back is a matter for the enforcers. Soon.")
	default:
		hint = theme.Subtle.Render("  Post a runner (c) or yourself to claim it. Its demand is yours while it is worked.")
	}
	b.WriteString(truncate(hint, m.width) + "\n")
	return b.String()
}

func heatWord(h float64) string {
	switch {
	case h >= 1.4:
		return "hot"
	case h >= 1.05:
		return "warm"
	case h <= 0.7:
		return "quiet"
	default:
		return "average"
	}
}

func riskWord(r float64) string {
	switch {
	case r >= 1.4:
		return "rough"
	case r >= 1.05:
		return "edgy"
	case r <= 0.7:
		return "safe"
	default:
		return "average"
	}
}
