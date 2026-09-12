package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
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
		return theme.CrewText
	case game.OwnerRival:
		return theme.RivalText
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
		m.refuse("Can't post there: somebody else holds " + c.Name + ".")
		return
	}
	if len(m.postRows(role)) == 0 {
		m.refuse("Nothing to post: no " + format.Plurals(role) + ". Hire one " + screenPointer(screenCrew) + ".")
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
			m.refuse(fmt.Sprintf("Can't stand on %s from %s: go there first.", c.Name, m.w.Here().Name))
			return
		}
		m.refuse("Can't post: " + err.Error())
		return
	}
	if who.ID == game.You {
		m.say(fmt.Sprintf("You are working %s now.", c.Name))
	} else if m.postRole == "enforcer" {
		m.say(fmt.Sprintf("%s is guarding %s.", who.Name, c.Name))
	} else {
		m.say(fmt.Sprintf("%s is working %s.", who.Name, c.Name))
	}
}

func (m *Model) abandonSelected() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if err := m.w.Abandon(c.ID); err != nil {
		m.refuse("Can't abandon: " + err.Error())
		return
	}
	m.say(c.Name + " goes back to the street.")
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
		} else if h := m.w.GuardOf(r.ID); h != nil {
			where = styled{theme.Warning, "in " + h.Name + ", will move"}
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
	width := m.mainWidth()

	held, worked, free := 0, 0, 0.0
	for _, c := range cs {
		if c.Held() {
			held++
		}
		if c.Worked() {
			worked++
		} else {
			free += m.cornerUnits(c)
		}
	}
	// One title line: the city, the tabs, and the count; the rival's
	// count is the corner inspector's.
	head := theme.PanelTitle.Render("MAP · "+city.Name) + "  " + m.cityTabs() + "  " +
		theme.Subtle.Render(fmt.Sprintf("%d/%d held · %d worked · ~%.0f/day free", held, len(cs), worked, free))
	lines := []string{truncate(head, width)}

	// The grid. Cells are laid out by their x, y; the body width decides
	// how wide a cell can be. It gets the rows MAIN has left once the
	// routes under it are listed whole (a blank, ROUTES, one line a
	// route) and scrolls by row to keep the corner under the cursor in
	// view: a city with more corners than fit scrolls, the routes never
	// clamp.
	cols, rows := m.mapGrid()
	routes := m.routeLines(width)
	room := m.mainHeight() - 1
	if len(routes) > 0 {
		room -= 2 + len(routes)
	}
	visible := max(1, room/3)
	if sel != nil && !m.onRoutes {
		m.mapTop = max(min(m.mapTop, sel.Y), sel.Y-visible+1)
	}
	m.mapTop = max(0, min(m.mapTop, rows-visible))
	cellW := m.mapCellW()
	burning := m.mapBurning() // #158: the cells the scene is over, by corner
	grid := map[[2]int]*game.Corner{}
	for i := range cs {
		grid[[2]int{cs[i].X, cs[i].Y}] = &cs[i]
	}
	for y := m.mapTop; y < min(rows, m.mapTop+visible); y++ {
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
			eyed := m.eyed(c)
			_, undercut := w.Undercutting(c.ID)
			undercut = undercut && c.Owner == game.OwnerRival
			mark := m.cellMark(c)
			// The chosen corner's name cell is Selected, and stays so
			// while the cursor is on the routes; the tell's mark is the
			// rival's colour on any other row, the price war's the
			// market's. A cell the strike scene is over (#158) is the
			// scene's row, the cell burning from the colour it was to
			// the colour it is, until the scene ends.
			name := m.cellName(c, cellW)
			switch {
			case burning[c.ID] != "":
				name = fit(burning[c.ID], cellW-1)
			case sel != nil && c.ID == sel.ID:
				name = theme.Selected.Render(name)
			case eyed:
				name = theme.RivalText.Render(mark) + st.Render(name[len(mark):])
			case undercut:
				name = theme.MarketText.Render(mark) + st.Render(name[len(mark):])
			default:
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
				who = theme.Warning.Render(fit(fmt.Sprintf("  nobody, %dd left", m.driftLeft(c)), cellW-1))
			case c.Owner == game.OwnerRival:
				if s := w.Strike; s != nil && s.Corner == c.ID {
					who = theme.Warning.Render(fit(fmt.Sprintf("  ⚔ %s tonight", s.Force), cellW-1))
				} else {
					who = st.Render(fit("  theirs", cellW-1))
				}
			case eyed:
				who = theme.RivalText.Render(fit("  theirs tomorrow", cellW-1))
			default:
				who = theme.Subtle.Render(fit("  free", cellW-1))
			}
			facts := fmt.Sprintf("  ~%.0f/day %s", m.cornerUnits(*c), heatWord(c.Heat))
			if c.Squeeze > 0 {
				facts = fmt.Sprintf("  ~%.0f/day undercut", m.cornerUnits(*c))
			}
			if undercut {
				facts = fmt.Sprintf("  $ undercut %s", dialShort(m.w.Undercuts[c.ID]))
			}
			l1 = append(l1, name+" ")
			l2 = append(l2, who+" ")
			l3 = append(l3, theme.Subtle.Render(fit(facts, cellW-1))+" ")
		}
		lines = append(lines, " "+strings.Join(l1, ""), " "+strings.Join(l2, ""), " "+strings.Join(l3, ""))
	}

	// The routes out of here, each an edge between the cities with its
	// dial and what is on it. The selected route's detail is the pane's.
	if len(routes) > 0 {
		lines = append(lines, "", theme.RoadText.Render("ROUTES"))
		lines = append(lines, routes...)
	}
	return strings.Join(lines, "\n") + "\n"
}

// mapGrid is the shown city's grid: how many columns and rows its
// corners lay out on.
func (m *Model) mapGrid() (cols, rows int) {
	for _, c := range m.shown().Corners {
		cols = max(cols, c.X+1)
		rows = max(rows, c.Y+1)
	}
	return cols, rows
}

// mapCellW is how wide a cell of the grid is: the body width shared
// among the columns, 12 at least and 24 at most.
func (m *Model) mapCellW() int {
	cols, _ := m.mapGrid()
	return max(12, min(24, (m.mainWidth()-2)/max(1, cols)))
}

// cellMark is the glyph before a corner's name: ▪ held, ▴ the rival's,
// $ the price war's (#68: tonight's orders sell cheap here), ? the
// tell's (#69: the rival sets up here tomorrow), · free.
func (m *Model) cellMark(c *game.Corner) string {
	_, undercut := m.w.Undercutting(c.ID)
	switch {
	case c.Held():
		return "▪"
	case undercut && c.Owner == game.OwnerRival:
		return "$"
	case c.Owner == game.OwnerRival:
		return "▴"
	case m.eyed(c):
		return "?"
	}
	return "·"
}

// cellName is a corner's name cell, plain: the mark and the name in
// caps, fit to the cell.
func (m *Model) cellName(c *game.Corner, cellW int) string {
	return fit(m.cellMark(c)+" "+strings.ToUpper(c.Name), cellW-1)
}

// eyed reports whether the corner is the one the rival telegraphed
// (#69): free today, theirs tomorrow unless somebody is posted on it.
func (m *Model) eyed(c *game.Corner) bool {
	return c.Owner == game.OwnerNone && c.ID == m.w.Rival.Eyeing
}

// driftLeft is the days a held corner nobody works has before it goes
// back to the street.
func (m *Model) driftLeft(c *game.Corner) int {
	return max(1, m.set.Territory.DriftDays(m.w)-c.Idle)
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
	sec := m.cornerSection(sel)
	if head, ok := m.mapHead(sel); ok {
		sec.title = head // #158: the name sliding in, rendered
	}
	return []section{sec}
}

// cornerSection is the corner inspector: whose it is and since when,
// its size, heat and risk, what it draws, who works and guards it, and
// what the keys would do to it in the state it is in.
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
		lines = append(lines, theme.RivalText.Render(fmt.Sprintf("%s's since day %d", w.Rival.Leader, sel.Since)))
		if s := w.Strike; s != nil && s.Corner == sel.ID {
			lines = append(lines, theme.Warning.Render(fmt.Sprintf("⚔ %s tonight", s.Force)))
		}
		lines = append(lines, row("holds", theme.RivalText.Render(plural(w.RivalHeld(), "corner"))))
	case m.eyed(sel):
		lines = append(lines, theme.RivalText.Render("free · they set up here tomorrow"))
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
	switch {
	case sel.Squeeze > 0 && sel.Owner == game.OwnerRival:
		// The price war (#68): what last night's orders took off it.
		lines = append(lines, row("squeezed", theme.MarketText.Render(fmt.Sprintf("-%.0f%% by you, %s", sel.Squeeze*100, plural(sel.Starved, "day")))))
	case sel.Squeeze > 0:
		lines = append(lines, row("undercut", theme.RivalText.Render(fmt.Sprintf("-%.0f%% (%s)", sel.Squeeze*100, w.Rival.Leader))))
	}
	if d, ok := w.Undercutting(sel.ID); ok && sel.Owner == game.OwnerRival {
		lines = append(lines, row("undercut", theme.MarketText.Render(fmt.Sprintf("%s · takes ~%.0f/day", d, m.set.Market.UndercutUnits(w, *sel, d)))))
	}
	if sel.Held() && w.Contested(*sel) {
		lines = append(lines, row("push flips", theme.RivalText.Render(fmt.Sprintf("~%.0f%%", m.set.Rivals.PushOdds(w, sel)*100))))
	}
	// Demand per product, biggest first, as many to a line as the value
	// column holds whole (two, mostly).
	ids := append([]string(nil), w.Products...)
	demand := func(id string) float64 {
		if p := w.Product(city.ID, id); p != nil {
			return p.Demand * sel.Share(id)
		}
		return 0
	}
	sort.SliceStable(ids, func(i, j int) bool { return demand(ids[i]) > demand(ids[j]) })
	label, line := "demand", ""
	for _, id := range ids {
		item := fmt.Sprintf("%s ~%.0f", w.ProductName(id), demand(id))
		switch {
		case line == "":
			line = item
		case lipgloss.Width(line)+3+lipgloss.Width(item) <= paneTextW-paneLabelW-1:
			line += " · " + item
		default:
			lines = append(lines, row(label, line))
			label, line = "", item
		}
	}
	if line != "" {
		lines = append(lines, row(label, line))
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
	// What the keys would do to it, by its state.
	switch {
	case sel.Worked():
		lines = append(lines, keyRow("c", "move a runner here"), keyRow("e", "post an enforcer"), keyRow("a", "abandon the corner"))
	case sel.Held():
		lines = append(lines,
			theme.Warning.Render(fmt.Sprintf("back to the street in %s", plural(m.driftLeft(sel), "day"))),
			keyRow("c", "post a runner here"), keyRow("e", "post an enforcer"), keyRow("a", "abandon the corner"))
	case sel.Owner == game.OwnerRival:
		if n := w.Crew.Role("enforcer"); n > 0 {
			lines = append(lines, keyRow("w", fmt.Sprintf("push takes it ~%.0f%%, hit ~%.0f%%",
				m.set.Rivals.Odds(w, events.ForcePush)*100, m.set.Rivals.Odds(w, events.ForceHit)*100)))
			lines = append(lines, keyRow("w", fmt.Sprintf("boost: the till, ~%s", cash(m.set.Rivals.BoostTake(w, *sel)))))
		} else {
			lines = append(lines, wrapped(theme.Subtle, "Taking it is a matter for the enforcers. Hire some "+screenPointer(screenCrew)+".")...)
		}
		// The books (#70): the police, tipped off, take the corner.
		tp := m.set.Rivals.TipTuning()
		if s := w.Tipoff; s != nil && s.Corner == sel.ID {
			lines = append(lines, keyRow("t", fmt.Sprintf("tipped tonight: police %.0f → %.0f", w.Rival.Heat, min(100, w.Rival.Heat+tp.Heat))))
		} else {
			lines = append(lines, keyRow("t", fmt.Sprintf("tip the police: at %.0f of %.0f", w.Rival.Heat, tp.PoliceNotice)))
		}
		// The price war (#68): the third answer, from next door.
		switch err := w.CanUndercut(sel.ID); {
		case err == nil:
			lines = append(lines, keyRow("u", fmt.Sprintf("undercut: takes ~%.0f%% at normal, no heat", m.set.Market.Steal(w, *sel, events.DialNormal)*100)))
		case err == game.ErrNotNextDoor:
			lines = append(lines, wrapped(theme.Subtle, "Work a corner next door and you can undercut it.")...)
		}
	default:
		if m.eyed(sel) {
			lines = append(lines, keyRow("c", "post a runner to keep them off"))
		} else {
			lines = append(lines, keyRow("c", "post a runner to claim it"))
		}
		if city.ID != w.Player.Location {
			lines = append(lines, keyRow("g", "go there to stand on it"))
		}
	}
	return section{strings.ToUpper(sel.Name), lines}
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
