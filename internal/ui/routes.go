package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The route dial (#61). Every route has a dial, off / slow / normal /
// fast, and a target stock for its far end per product; the logistics
// sim runs it every day. The map lists the routes out of the city shown
// under its grid, with a cursor the arrows reach past the bottom row: r
// turns the selected route's dial, R opens the target dialog. The ledger
// lists the same routes with their books.

// targetDialog is the state of the target modal: product, then units.
type targetDialog struct {
	route string // route id it sets
	step  int    // 0 product, 1 units
	units textinput.Model
	err   string
}

// mapRoutes are the routes touching the city the map shows, in file
// order: what the routes cursor walks.
func (m *Model) mapRoutes() []content.RouteConfig { return m.set.Logistics.Routes(m.shown().ID) }

// selectedRoute is the route under the routes cursor, or nil when the
// city shown has none.
func (m *Model) selectedRoute() *content.RouteConfig {
	routes := m.mapRoutes()
	if len(routes) == 0 {
		return nil
	}
	m.routeCursor = max(0, min(m.routeCursor, len(routes)-1))
	return &routes[m.routeCursor]
}

// cycleRoute turns the selected route's dial a notch: off, slow, normal,
// fast, off.
func (m *Model) cycleRoute() {
	if m.w.Over != nil {
		return
	}
	r := m.selectedRoute()
	if r == nil {
		m.refuse("Nothing to turn: no route out of " + m.shown().Name + ".")
		return
	}
	d := (m.w.Route(r.ID).Dial + 1) % (events.RouteFast + 1)
	if err := m.w.SetRoute(r.ID, d); err != nil {
		m.refuse("Can't turn the dial: " + err.Error())
		return
	}
	m.sayRouteDial(*r, d)
}

// sayRouteDial is the status after a route's dial is turned, on the
// map or the ledger: what the road does at the notch.
func (m *Model) sayRouteDial(r content.RouteConfig, d events.RouteDial) {
	if !d.On() {
		m.say(fmt.Sprintf("%s off: nothing moves on it. Its targets are kept.", r.Name))
		return
	}
	lg := m.set.Logistics
	line := fmt.Sprintf("%s %s: %s %s to %s, seized ~%.0f%%.", r.Name, d, plural(lg.Days(r, d.Ship()), "day"), r.Mode, m.w.CityName(r.To), lg.Risk(r, d.Ship())*100)
	if len(m.w.Route(r.ID).Target) == 0 {
		line += " It sends nothing without a target."
	}
	m.say(line)
}

// openTarget opens the target dialog for the selected route.
func (m *Model) openTarget() {
	if m.w.Over != nil {
		return
	}
	r := m.selectedRoute()
	if r == nil {
		m.refuse("Nothing to target: no route out of " + m.shown().Name + ".")
		return
	}
	ti := textinput.New()
	ti.Placeholder = "blank = none"
	ti.CharLimit = 6
	ti.Width = 8
	ti.Prompt = "> "
	m.tgt = targetDialog{route: r.ID, units: ti}
	m.mode = modeTarget
}

// targetRoute is the route the target dialog is about.
func (m *Model) targetRoute() *content.RouteConfig {
	if r := m.set.Logistics.Route(m.tgt.route); r != nil {
		return r
	}
	return m.selectedRoute()
}

func (m *Model) keyTarget(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.tgt
	d.err = ""
	r := m.targetRoute()
	if r == nil {
		m.mode = modePlay
		return m, nil
	}
	// Back is one key and close is one key (#110): esc closes from
	// either step, shift+tab leaves the units for the product (kept under
	// the cursor) and is silent on the first step, tab goes to the units
	// (a product is always chosen) and is silent on the last.
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "shift+tab":
		if d.step == 1 {
			d.step = 0
			d.units.SetValue("")
			d.units.Blur()
		}
		return m, nil
	case "tab":
		if d.step == 1 {
			return m, nil
		}
	case "q":
		if d.step == 0 {
			m.mode = modePlay
			return m, nil
		}
	}
	switch d.step {
	case 0:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.w.Products)-1 {
				m.cursor++
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(key[0] - '1'); i < len(m.w.Products) {
				m.cursor = i
			}
		case "enter", "right", "l", "tab":
			d.step = 1
			d.units.SetValue("")
			if t := m.w.Route(r.ID).Target[m.w.Products[m.cursor]]; t > 0 {
				d.units.SetValue(strconv.Itoa(t))
			}
			d.units.Focus()
			return m, textinput.Blink
		}
	case 1:
		if key == "enter" {
			return m.confirmTarget()
		}
		var cmd tea.Cmd
		d.units, cmd = d.units.Update(k)
		return m, cmd
	}
	return m, nil
}

func (m *Model) confirmTarget() (tea.Model, tea.Cmd) {
	r := m.targetRoute()
	id := m.w.Products[m.cursor]
	units := 0
	if s := strings.TrimSpace(m.tgt.units.Value()); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			m.tgt.err = "enter a whole number, or nothing for none"
			return m, nil
		}
		units = n
	}
	if err := m.w.SetRouteTarget(r.ID, id, units); err != nil {
		m.tgt.err = err.Error()
		return m, nil
	}
	m.mode = modePlay
	switch {
	case units == 0:
		m.say(fmt.Sprintf("%s: no target for %s; the route leaves it alone.", r.Name, m.w.ProductName(id)))
	case !m.w.Route(r.ID).Dial.On():
		m.say(fmt.Sprintf("%s keeps %s at %d %s once its dial is on.", r.Name, m.w.CityName(r.To), units, m.w.ProductName(id)))
	default:
		m.say(fmt.Sprintf("%s keeps %s at %d %s: it sends the shortfall every day.", r.Name, m.w.CityName(r.To), units, m.w.ProductName(id)))
	}
	return m, nil
}

func (m *Model) viewTarget() string {
	w := m.w
	d := m.tgt
	r := m.targetRoute()
	if r == nil {
		return m.modal("TARGET", []string{"No route."}, m.modalFooter())
	}
	rs := w.Route(r.ID)
	body := []string{
		theme.Subtle.Render(fmt.Sprintf("%s keeps %s stocked: every day it sends what is short of", r.Name, w.CityName(r.To))),
		theme.Subtle.Render(fmt.Sprintf("the target, up to %d units, buying by the lot in %s.", r.Capacity, w.CityName(r.From))),
		"",
	}
	var rows [][]any
	for _, pid := range w.Products {
		var target any
		if t := rs.Target[pid]; t > 0 {
			target = t
		}
		rows = append(rows, []any{w.ProductName(pid), target, w.Stock(r.To, pid), w.Bound(r.To, pid), approx{w.Demand(r.To, pid)}})
	}
	m.modalFollow(len(body) + 1 + m.cursor) // under the header
	body = append(body, table([]col{{"product", kText, 0}, {"target", kInt, 0}, {"there", kInt, 0}, {"road", kInt, 0}, {"sells/day", kInt, 0}}, rows, m.cursor, m.modalInner())...)
	if d.step == 0 {
		body = append(body, "", theme.Subtle.Render("Pick a product."))
	} else {
		id := w.Products[m.cursor]
		body = append(body, "", fmt.Sprintf("Target    %s   %s", d.units.View(), theme.Subtle.Render(fmt.Sprintf("units of %s kept in %s", w.ProductName(id), w.CityName(r.To)))))
		if s := strings.TrimSpace(d.units.Value()); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				if src := w.Product(r.From, id); src != nil {
					o := m.set.Logistics.Wholesale()
					unit := src.SupplierPrice
					how := "at retail in " + w.CityName(r.From)
					if w.City(r.From).Wholesale && !o.Locked(w) {
						unit *= o.Mul
						how = "by the lot in " + w.CityName(r.From)
					}
					short := max(0, n-w.Stock(r.To, id)-w.Bound(r.To, id))
					body = append(body, theme.Subtle.Render(fmt.Sprintf("Short     %d: ~%s %s + %s fares", short, money(int(float64(short)*unit)), how, money(short*r.Cost))))
				}
			}
		}
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal("TARGET · "+r.Name, body, m.modalFooter())
}

// roadOn is what is on the road on a route: units per product with the
// soonest arrival, in product order, or "" when it is empty.
func (m *Model) roadOn(route string) string {
	w := m.w
	var parts []string
	for _, id := range w.Products {
		units, soonest := 0, 0
		for _, sh := range w.Shipments {
			if sh.Route != route || sh.Product != id {
				continue
			}
			units += sh.Units
			if d := sh.DaysLeft(w.Day); soonest == 0 || d < soonest {
				soonest = d
			}
		}
		if units > 0 {
			parts = append(parts, fmt.Sprintf("%d %s, %dd", units, w.ProductName(id), soonest))
		}
	}
	return strings.Join(parts, ", ")
}

// targetLine is a route's targets in one line, or "" when it has none.
func (m *Model) targetLine(route string) string {
	rs := m.w.Route(route)
	var parts []string
	for _, id := range m.w.Products {
		if t := rs.Target[id]; t > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", m.w.ProductName(id), t))
		}
	}
	return strings.Join(parts, " · ")
}

// dialStyle is the colour a route dial is drawn in: the dial's accent
// when it is on, red at fast (the risk is the road's danger), Subtle
// off. The pane's row is one bracketed notch, `[slow]`, because four
// notches do not fit its value column.
func dialStyle(d events.RouteDial) lipgloss.Style {
	switch d {
	case events.RouteFast:
		return theme.Bad
	default:
		return theme.Dial(d.On())
	}
}

// routeLines are the routes out of the city shown, one line each, drawn
// as an edge between the cities with the dial, the route's terms at that
// dial (`3d · 60 units · $8/u · ~3%`; `60u` where the long form would
// not fit the width, and no terms at all where that would not either:
// the pane has them) and what is on it (`▪60`); the routes cursor's row
// is marked ▸ and drawn Selected across while the cursor is on the
// routes. The selected route's targets are the pane's (routeSection).
func (m *Model) routeLines(width int) []string {
	w := m.w
	lg := m.set.Logistics
	routes := m.mapRoutes()
	nameW, cityW := 0, 0
	for _, r := range routes {
		nameW = max(nameW, lipgloss.Width(r.Name))
		cityW = max(cityW, lipgloss.Width(w.CityName(r.From)), lipgloss.Width(w.CityName(r.To)))
	}
	draw := func(units string) (lines []string, widest int) {
		for i, r := range routes {
			d := w.Route(r.ID).Dial
			name := fit(r.Name, nameW)
			edge := fmt.Sprintf("%s %s %s", fit(w.CityName(r.From), cityW), edge(r.Mode), fit(w.CityName(r.To), cityW))
			dial := fit(d.String(), 6)
			terms := ""
			if units != "" {
				terms = fmt.Sprintf("  %dd · %d%s · %s/u · ~%.0f%%", lg.Days(r, d.Ship()), r.Capacity, units, money(r.Cost), lg.Risk(r, d.Ship())*100)
			}
			road := ""
			if n := m.unitsOn(r.ID); n > 0 {
				road = fmt.Sprintf("  ▪%d", n)
			}
			plain := name + "  " + edge + "  " + dial + terms + road
			widest = max(widest, 2+lipgloss.Width(plain))
			mark := "  "
			if i == m.routeCursor {
				mark = theme.Gold.Render("▸ ")
				if m.onRoutes {
					lines = append(lines, mark+theme.Selected.Render(plain))
					continue
				}
			}
			lines = append(lines, mark+name+"  "+theme.Subtle.Render(edge)+"  "+dialStyle(d).Render(dial)+theme.Subtle.Render(terms)+theme.RoadText.Render(road))
		}
		return lines, widest
	}
	lines, widest := draw(" units")
	if widest > width {
		lines, widest = draw("u")
	}
	if widest > width {
		lines, _ = draw("")
	}
	for i, l := range lines {
		lines[i] = truncate(l, width)
	}
	return lines
}

// unitsOn is how many units are on the road on a route.
func (m *Model) unitsOn(route string) int {
	units := 0
	for _, sh := range m.w.Shipments {
		if sh.Route == route {
			units += sh.Units
		}
	}
	return units
}

// routeSection is the route's detail for the pane: the edge, the dial
// and the terms at it, the targets, what is on the road, and what r and
// R do.
func (m *Model) routeSection(r content.RouteConfig) section {
	w := m.w
	lg := m.set.Logistics
	d := w.Route(r.ID).Dial
	lines := []string{
		theme.Subtle.Render(fmt.Sprintf("%s %s %s", w.CityName(r.From), edge(r.Mode), w.CityName(r.To))),
		row("dial", dialStyle(d).Render("["+d.String()+"]")),
		row("days", fmt.Sprintf("%d · capacity %d", lg.Days(r, d.Ship()), r.Capacity)),
		row("fare", fmt.Sprintf("%s/u · seized ~%.0f%%", money(r.Cost), lg.Risk(r, d.Ship())*100)),
	}
	switch t := m.targetLine(r.ID); {
	case t == "" && d.On():
		lines = append(lines, row("target", theme.Warning.Render("none: it sends nothing")))
	case t == "":
		lines = append(lines, row("target", theme.Subtle.Render("none")))
	default:
		label := "target"
		for _, l := range wrap(w.CityName(r.To)+" keeps "+t, paneTextW-paneLabelW-1) {
			lines = append(lines, row(label, l))
			label = ""
		}
	}
	if road := m.roadOn(r.ID); road != "" {
		label := "on the road"
		for _, l := range wrap(road, paneTextW-paneLabelW-1) {
			lines = append(lines, row(label, theme.RoadText.Render(l)))
			label = ""
		}
	}
	lines = append(lines, keyRow("r", "turn the dial"), keyRow("R", "set a target"))
	return section{strings.ToUpper(r.Name), lines}
}

// edge draws a route's mode as an arrow of fixed width, so the routes
// line up: ──car──▶, ─truck─▶.
func edge(mode string) string {
	mode = fit(mode, 5)
	pad := 5 - lipgloss.Width(strings.TrimRight(mode, " "))
	mode = strings.TrimRight(mode, " ")
	return strings.Repeat("─", 1+pad/2) + mode + strings.Repeat("─", 1+pad-pad/2) + "▶"
}

// askTravel asks before moving you to the other city.
func (m *Model) askTravel() {
	if m.w.Over != nil {
		return
	}
	if len(m.w.CityOrder) < 2 {
		m.refuse("Can't go: there is nowhere else.")
		return
	}
	m.mode = modeConfirmTravel
}

// travelTo is the city g goes to: the one shown if you are not in it,
// else the next one.
func (m *Model) travelTo() string {
	if m.city != m.w.Player.Location {
		return m.city
	}
	order := m.w.CityOrder
	for i, id := range order {
		if id == m.w.Player.Location {
			return order[(i+1)%len(order)]
		}
	}
	return order[0]
}

func (m *Model) confirmTravel() {
	to := m.travelTo()
	m.mode = modePlay
	if err := m.w.Travel(to); err != nil {
		m.refuse("Can't go: " + err.Error())
		return
	}
	m.city = to
	m.mapCursor = m.yourCorner()
	m.say(fmt.Sprintf("You are in %s. Your stock stayed where it was; post yourself on a corner %s.", m.w.CityName(to), screenPointer(screenMap)))
}

func (m *Model) travelConfirm() string {
	to := m.travelTo()
	body := []string{fmt.Sprintf("Leave %s for %s today?", m.w.Here().Name, m.w.CityName(to)), ""}
	if c := m.w.PostOf(game.You); c != nil {
		body = append(body, theme.Warning.Render(fmt.Sprintf("You step off %s: it drifts back to the street", c.Name)), theme.Warning.Render("unless a runner takes it."))
	} else {
		body = append(body, theme.Subtle.Render("You stand on no corner here to leave."))
	}
	for _, l := range []string{
		"Stock stays where it is; the routes " + screenPointer(screenMap) + " move it.",
		"The supplier sells to you where you stand. Go for what",
		"only you can do there: stand on a corner, hire, ask around.",
	} {
		body = append(body, theme.Subtle.Render(l))
	}
	return m.modal("GO TO "+m.w.CityName(to)+"?", body, m.modalFooter())
}
