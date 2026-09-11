package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// mapSelected returns the corner under the map cursor, in the city shown.
func (m *Model) mapSelected() *game.Corner {
	cs := m.shown().Corners
	if len(cs) == 0 {
		return nil
	}
	m.mapCursor = max(0, min(m.mapCursor, len(cs)-1))
	return &cs[m.mapCursor]
}

// mapMove walks the grid: dx moves along the row to the next corner in
// that direction (an empty cell is skipped, the edge is a no-op), dy to
// the corner in the adjacent row nearest by column. Down past the bottom
// row goes to the routes listed under the grid, and up from the first
// route comes back; the corner cursor stays an index into Corners, so
// the post and strike pickers read it as before.
func (m *Model) mapMove(dx, dy int) {
	if m.onRoutes {
		switch {
		case dy < 0 && m.routeCursor == 0:
			m.onRoutes = false
		case dy < 0:
			m.routeCursor--
		case dy > 0:
			m.routeCursor = min(m.routeCursor+1, len(m.mapRoutes())-1)
		}
		return
	}
	sel := m.mapSelected()
	if sel == nil {
		if dy > 0 && len(m.mapRoutes()) > 0 {
			m.onRoutes, m.routeCursor = true, 0
		}
		return
	}
	cs := m.shown().Corners
	best, bestD := -1, 0
	for i := range cs {
		c := &cs[i]
		var d int
		switch {
		case dx != 0:
			if c.Y != sel.Y || (c.X-sel.X)*dx <= 0 {
				continue
			}
			d = (c.X - sel.X) * dx
		default:
			if c.Y != sel.Y+dy {
				continue
			}
			d = max(c.X-sel.X, sel.X-c.X)
		}
		if best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	if best >= 0 {
		m.mapCursor = best
	} else if dy > 0 && len(m.mapRoutes()) > 0 {
		m.onRoutes, m.routeCursor = true, 0
	}
}

// ownerStyle is the colour a corner is drawn in: crew blue for yours,
// rivals purple for theirs, dim for nobody's.
func ownerStyle(owner string) lipgloss.Style {
	switch owner {
	case game.OwnerPlayer:
		return lipgloss.NewStyle().Foreground(theme.Crew)
	case game.OwnerRival:
		return theme.Rival
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
// the supplier lists, at its city's per-corner demand today.
func (m *Model) cornerUnits(c game.Corner) float64 {
	n := 0.0
	for _, id := range m.w.Products {
		if p := m.w.Product(c.City, id); p != nil {
			n += p.Demand * c.Share(id)
		}
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
		m.status = "No " + role + "s to post. Hire one " + screenPointer(screenCrew) + "."
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
		if who.ID == game.You && err == game.ErrElsewhere {
			m.status = fmt.Sprintf("You are in %s: go there first to stand on %s.", m.w.Here().Name, c.Name)
			return
		}
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
		return m.modal("POST", []string{"Nobody to post."}, m.modalFooter())
	}
	m.postCursor = max(0, min(m.postCursor, len(rows)-1))
	var cells [][]any
	for _, r := range rows {
		var where any = styled{theme.Subtle, "idle"}
		if p := m.w.PostOf(r.ID); p != nil {
			if p.ID == c.ID {
				where = styled{theme.Good, "already here"}
			} else {
				where = styled{theme.Warning, "on " + p.Name + ", will move"}
			}
		}
		var skill any = r.Skill
		if r.ID == game.You {
			skill = nil
		}
		cells = append(cells, []any{r.Name, skill, where})
	}
	m.modalFollow(1 + m.postCursor) // under the header
	body := table([]col{{"name", kText, 0}, {"skill", kInt, 0}, {"where", kText, 0}}, cells, m.postCursor, m.modalInner())
	what, title := "work", "POST A RUNNER"
	if m.postRole == "enforcer" {
		what, title = "guard", "POST AN ENFORCER"
	}
	body = append(body, "", theme.Subtle.Render(fmt.Sprintf("Who should %s %s?", what, c.Name)))
	return m.modal(title, body, m.modalFooter())
}

func (m *Model) viewMap() string {
	w := m.w
	city := m.shown()
	cs := city.Corners
	sel := m.mapSelected()
	var b strings.Builder

	held, worked, unserved := 0, 0, 0.0
	for _, c := range cs {
		if c.Held() {
			held++
		}
		if c.Worked() {
			worked++
		} else {
			unserved += m.cornerUnits(c)
		}
	}
	head := theme.PanelTitle.Render("MAP · ") + m.cityTabs() +
		theme.Subtle.Render(fmt.Sprintf(" %d/%d held · %d worked · ~%.0f/day unworked", held, len(cs), worked, unserved))
	if w.Rival.Arrived > 0 && city.ID == w.Home().ID {
		head += theme.Rival.Render(fmt.Sprintf(" · %s %d", m.rivalName(), w.RivalHeld()))
	}
	width := m.mainWidth()
	b.WriteString(truncate(head, width) + "\n")
	b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  heat %.0f · %s", city.Heat, m.stashLine(city.ID))), width) + "\n\n")

	// The grid. Cells are laid out by their x, y; the body width decides
	// how wide a cell can be.
	cols, rows := 0, 0
	for _, c := range cs {
		cols = max(cols, c.X+1)
		rows = max(rows, c.Y+1)
	}
	cellW := max(12, min(24, (width-2)/max(1, cols)))
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
			if sel != nil && c.ID == sel.ID && !m.onRoutes {
				name = theme.Selected.Render(name)
			} else if sel != nil && c.ID == sel.ID {
				name = st.Bold(true).Underline(true).Render(name)
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
				if s := w.Strike; s != nil && s.Corner == c.ID {
					who = theme.Warning.Render(fit(fmt.Sprintf("  ⚔ %s tonight", s.Force), cellW-1))
				} else {
					who = st.Render(fit("  theirs", cellW-1))
				}
			default:
				who = theme.Subtle.Render(fit("  free", cellW-1))
			}
			facts := fmt.Sprintf("  ~%.0f/day %s", m.cornerUnits(*c), heatWord(c.Heat))
			if c.Squeeze > 0 {
				facts = fmt.Sprintf("  ~%.0f/day undercut", m.cornerUnits(*c))
			}
			l1 = append(l1, name+" ")
			l2 = append(l2, who+" ")
			l3 = append(l3, theme.Subtle.Render(fit(facts, cellW-1))+" ")
		}
		b.WriteString(" " + strings.Join(l1, "") + "\n" + " " + strings.Join(l2, "") + "\n" + " " + strings.Join(l3, "") + "\n")
	}

	// The routes out of here, each an edge between the cities with its
	// dial and what is on it. The selected route's detail is the pane's.
	if lines := m.routeLines(); len(lines) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Logistics).Render("ROUTES") + "\n")
		for _, l := range lines {
			b.WriteString(truncate(l, width) + "\n")
		}
	}
	return b.String()
}

// mapDetails is the map's pane: the inspector for the corner under the
// cursor, or the route's detail while the cursor is on the routes, and
// the keys.
func (m *Model) mapDetails() []section {
	if m.onRoutes {
		if r := m.selectedRoute(); r != nil {
			return []section{m.routeSection(*r)}
		}
	}
	sel := m.mapSelected()
	if sel == nil {
		return nil
	}
	return []section{m.cornerSection(sel)}
}

// cornerSection is the corner inspector: whose it is and since when,
// its size, heat and risk, what it draws, who works and guards it, and
// what the keys would do to it.
func (m *Model) cornerSection(sel *game.Corner) section {
	w := m.w
	city := m.shown()
	var lines []string
	switch {
	case sel.Held():
		owner := fmt.Sprintf("yours since day %d", sel.Since)
		if sel.Runner == game.You {
			owner += " · you work it"
		}
		lines = append(lines, theme.Subtle.Render(owner))
	case sel.Owner == game.OwnerRival:
		lines = append(lines, theme.Rival.Render(fmt.Sprintf("%s's since day %d", w.Rival.Leader, sel.Since)))
		if s := w.Strike; s != nil && s.Corner == sel.ID {
			lines = append(lines, theme.Warning.Render(fmt.Sprintf("⚔ %s tonight", s.Force)))
		}
	default:
		lines = append(lines, theme.Subtle.Render("free"))
	}
	lines = append(lines,
		row("size", fmt.Sprintf("×%.1f", sel.Demand)),
		row("heat", fmt.Sprintf("×%.1f %s", sel.Heat, heatWord(sel.Heat))),
		row("risk", fmt.Sprintf("×%.1f %s", sel.Risk, riskWord(sel.Risk))))
	if sel.Held() {
		lines = append(lines, row("robbery", fmt.Sprintf("%.1f%%/day", m.set.Territory.RobberyChance(w, sel)*100)))
	}
	if sel.Squeeze > 0 {
		lines = append(lines, row("undercut", theme.Rival.Render(fmt.Sprintf("-%.0f%%", sel.Squeeze*100))))
	}
	if sel.Held() && w.Contested(*sel) {
		lines = append(lines, row("push flips", theme.Rival.Render(fmt.Sprintf("~%.0f%%", m.set.Rivals.PushOdds(w, sel)*100))))
	}
	// Demand per product, biggest first, packed into the value column.
	ids := append([]string(nil), w.Products...)
	demand := func(id string) float64 {
		if p := w.Product(city.ID, id); p != nil {
			return p.Demand * sel.Share(id)
		}
		return 0
	}
	sort.SliceStable(ids, func(i, j int) bool { return demand(ids[i]) > demand(ids[j]) })
	var dem []string
	for _, id := range ids {
		dem = append(dem, fmt.Sprintf("%s ~%.0f", w.ProductName(id), demand(id)))
	}
	label := "demand"
	for _, l := range wrap(strings.Join(dem, " · "), paneTextW-paneLabelW-1) {
		lines = append(lines, row(label, l))
		label = ""
	}
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
	lines = append(lines, row("runner", runner), row("enforcer", enforcer))
	switch {
	case sel.Worked():
		lines = append(lines, keyRow("c", "move a runner here"), keyRow("e", "post an enforcer"), keyRow("a", "abandon the corner"))
	case sel.Held():
		lines = append(lines, wrapped(theme.Warning, "Nobody is working it: it goes back to the street unless a runner takes it.")...)
	case sel.Owner == game.OwnerRival:
		if n := w.Crew.Role("enforcer"); n > 0 {
			lines = append(lines,
				keyRow("w", fmt.Sprintf("push takes it ~%.0f%%", m.set.Rivals.Odds(w, events.ForcePush)*100)),
				keyRow("", fmt.Sprintf("hit takes it ~%.0f%%", m.set.Rivals.Odds(w, events.ForceHit)*100)))
		} else {
			lines = append(lines, wrapped(theme.Subtle, "Taking it is a matter for the enforcers. Hire some "+screenPointer(screenCrew)+".")...)
		}
	default:
		if city.ID == w.Player.Location {
			lines = append(lines, wrapped(theme.Subtle, "Post a runner or yourself to claim it. Its demand is yours while it is worked.")...)
		} else {
			lines = append(lines, wrapped(theme.Subtle, "Post a runner to claim it; you would have to go there to stand on it yourself.")...)
		}
	}
	return section{strings.ToUpper(sel.Name), lines}
}

// stashLine is what you hold in a city, product by product.
func (m *Model) stashLine(city string) string {
	var parts []string
	for _, id := range m.w.Products {
		if q := m.w.Stock(city, id); q > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", q, m.w.ProductName(id)))
		}
	}
	if len(parts) == 0 {
		return "stash empty"
	}
	return "stash " + strings.Join(parts, ", ")
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
