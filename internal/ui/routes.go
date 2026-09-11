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
		m.status = "No route out of " + m.shown().Name + "."
		return
	}
	d := (m.w.Route(r.ID).Dial + 1) % (events.RouteFast + 1)
	if err := m.w.SetRoute(r.ID, d); err != nil {
		m.status = "Can't turn the dial: " + err.Error()
		return
	}
	if !d.On() {
		m.status = fmt.Sprintf("%s off: nothing moves on it. Its targets are kept.", r.Name)
		return
	}
	lg := m.set.Logistics
	line := fmt.Sprintf("%s %s: %d day(s) %s to %s, seized ~%.0f%%.", r.Name, d, lg.Days(*r, d.Ship()), r.Mode, m.w.CityName(r.To), lg.Risk(*r, d.Ship())*100)
	if len(m.w.Route(r.ID).Target) == 0 {
		line += " Set a target (R) or it sends nothing."
	}
	m.status = line
}

// openTarget opens the target dialog for the selected route.
func (m *Model) openTarget() {
	if m.w.Over != nil {
		return
	}
	r := m.selectedRoute()
	if r == nil {
		m.status = "No route out of " + m.shown().Name + "."
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
	switch key {
	case "esc":
		if d.step == 0 {
			m.mode = modePlay
		} else {
			d.step = 0
			d.units.Blur()
		}
		return m, nil
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
		case "enter", "right", "l":
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
		m.status = fmt.Sprintf("%s: no target for %s; the route leaves it alone.", r.Name, m.w.ProductName(id))
	case !m.w.Route(r.ID).Dial.On():
		m.status = fmt.Sprintf("%s keeps %s at %d %s once its dial is on (r).", r.Name, m.w.CityName(r.To), units, m.w.ProductName(id))
	default:
		m.status = fmt.Sprintf("%s keeps %s at %d %s: it sends the shortfall every day.", r.Name, m.w.CityName(r.To), units, m.w.ProductName(id))
	}
	return m, nil
}

func (m *Model) viewTarget() string {
	w := m.w
	d := m.tgt
	r := m.targetRoute()
	if r == nil {
		return m.modal("TARGET", "No route.")
	}
	rs := w.Route(r.ID)
	var b strings.Builder
	lineW := max(20, m.width-10) // inside the modal's frame
	b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("%s keeps %s stocked: every day it sends what is short of", r.Name, w.CityName(r.To))), lineW) + "\n")
	b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("the target, up to %d units, buying by the lot in %s.", r.Capacity, w.CityName(r.From))), lineW) + "\n\n")
	var rows [][]any
	for _, pid := range w.Products {
		var target any
		if t := rs.Target[pid]; t > 0 {
			target = t
		}
		rows = append(rows, []any{w.ProductName(pid), target, w.Stock(r.To, pid), w.Bound(r.To, pid), approx{w.Demand(r.To, pid)}})
	}
	for _, l := range table([]col{{"product", kText, 0}, {"target", kInt, 0}, {"there", kInt, 0}, {"road", kInt, 0}, {"sells/day", kInt, 0}}, rows, m.cursor, lineW) {
		b.WriteString(l + "\n")
	}
	if d.step == 0 {
		b.WriteString("\n" + theme.Subtle.Render("Pick a product, then enter.") + "\n")
	} else {
		id := w.Products[m.cursor]
		b.WriteString(fmt.Sprintf("\nTarget    %s   %s\n", d.units.View(), theme.Subtle.Render(fmt.Sprintf("units of %s kept in %s", w.ProductName(id), w.CityName(r.To)))))
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
					b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("Short     %d: ~%s %s + %s fares", short, money(int(float64(short)*unit)), how, money(short*r.Cost))), lineW) + "\n")
				}
			}
		}
	}
	if d.err != "" {
		b.WriteString("\n" + theme.Bad.Render(d.err) + "\n")
	}
	return m.modal("TARGET · "+strings.ToUpper(r.Name), strings.TrimRight(b.String(), "\n"))
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
			parts = append(parts, fmt.Sprintf("%d %s %dd", units, w.ProductName(id), soonest))
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

// dialStyle is the colour a route dial is drawn in.
func dialStyle(d events.RouteDial) lipgloss.Style {
	switch d {
	case events.RouteFast:
		return theme.Bad
	case events.RouteSlow, events.RouteNormal:
		return theme.Gold
	default:
		return theme.Subtle
	}
}

// routeLines are the routes out of the city shown, one line each, drawn
// as an edge between the cities with the dial, the route's terms at that
// dial and what is on it, the selected one marked. The line after the
// selected route is its targets.
func (m *Model) routeLines() []string {
	w := m.w
	lg := m.set.Logistics
	var lines []string
	for i, r := range m.mapRoutes() {
		rs := w.Route(r.ID)
		d := rs.Dial
		mark := "  "
		if i == m.routeCursor {
			mark = theme.Gold.Render("▸ ")
		}
		edge := fmt.Sprintf("%s %s %s", fit(w.CityName(r.From), 8), edge(r.Mode), fit(w.CityName(r.To), 8))
		terms := fmt.Sprintf("%dd %du %s/u ~%.0f%%", lg.Days(r, d.Ship()), r.Capacity, money(r.Cost), lg.Risk(r, d.Ship())*100)
		line := mark + fit(r.Name, 12) + " " + theme.Subtle.Render(edge) + "  " + dialStyle(d).Render(fit(d.String(), 6)) + " " + theme.Subtle.Render(terms)
		road := m.roadOn(r.ID)
		if road != "" && i != m.routeCursor {
			units := 0
			for _, sh := range w.Shipments {
				if sh.Route == r.ID {
					units += sh.Units
				}
			}
			line += lipgloss.NewStyle().Foreground(theme.Logistics).Render(fmt.Sprintf(" ▪%d", units))
		}
		lines = append(lines, line)
		if i == m.routeCursor {
			var detail string
			switch t := m.targetLine(r.ID); {
			case t == "" && d.On():
				detail = theme.Warning.Render("no target: it sends nothing. R sets one.")
			case t == "":
				detail = theme.Subtle.Render("target  none · r turns the dial, R sets a target")
			default:
				detail = theme.Subtle.Render("target  "+w.CityName(r.To)+" keeps ") + t
			}
			if road != "" {
				detail += theme.Subtle.Render(" · on the road ") + lipgloss.NewStyle().Foreground(theme.Logistics).Render(road)
			}
			lines = append(lines, "   "+detail)
		}
	}
	return lines
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
		m.status = "There is nowhere else to go."
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
		m.status = "Can't go: " + err.Error()
		return
	}
	m.city = to
	m.mapCursor = m.yourCorner()
	m.status = fmt.Sprintf("You are in %s. Your stock stayed where it was; post yourself on a corner here (5, c).", m.w.CityName(to))
}

func (m *Model) travelConfirm() string {
	to := m.travelTo()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Leave %s for %s today?\n\n", m.w.Here().Name, m.w.CityName(to)))
	if c := m.w.PostOf(game.You); c != nil {
		b.WriteString(theme.Warning.Render(fmt.Sprintf("You step off %s: it drifts back to the street\nunless a runner takes it.", c.Name)) + "\n")
	} else {
		b.WriteString(theme.Subtle.Render("You stand on no corner here to leave.") + "\n")
	}
	b.WriteString(theme.Subtle.Render("Stock stays where it is; the routes move it (map, r).\nThe supplier sells to you where you stand. Go for what\nonly you can do there: stand on a corner, hire, ask around.") + "\n\n")
	b.WriteString(theme.Key.Render("y") + " go   " + theme.Key.Render("any other key") + " stay")
	return m.modal("GO TO "+strings.ToUpper(m.w.CityName(to))+"?", b.String())
}
