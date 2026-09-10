package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// shipDialog is the state of the ship modal: product, route, quantity,
// dial. It ships out of the city shown on the market and map screens.
type shipDialog struct {
	from  string // city id it ships from
	step  int    // 0 product, 1 route, 2 quantity, 3 dial
	route int
	qty   textinput.Model
	dial  events.Ship
	err   string
}

// shipFrom is the city the ship dialog sends from: the one the market or
// map is turned to, where you are from any other screen. It is fixed when
// the dialog opens.
func (m *Model) shipFrom() *game.City {
	if c := m.w.City(m.shp.from); c != nil {
		return c
	}
	return m.w.City(m.actionCity())
}

// shipRoutes are the routes out of the city the dialog sends from.
func (m *Model) shipRoutes() []content.RouteConfig { return m.set.Logistics.Routes(m.shipFrom().ID) }

// openShip opens the ship dialog.
func (m *Model) openShip() {
	if m.w.Over != nil {
		return
	}
	m.shp = shipDialog{from: m.actionCity()}
	city := m.shipFrom()
	if len(m.shipRoutes()) == 0 {
		m.status = "No route out of " + city.Name + "."
		return
	}
	if m.w.Player.StockIn(city.ID) == 0 {
		m.status = fmt.Sprintf("Nothing stashed in %s to ship. Buy some there first (b).", city.Name)
		return
	}
	ti := textinput.New()
	ti.Placeholder = "blank = max"
	ti.CharLimit = 6
	ti.Width = 14
	ti.Prompt = "> "
	m.shp = shipDialog{from: city.ID, qty: ti, dial: events.ShipNormal}
	if m.w.Stock(city.ID, m.w.Products[m.cursor]) == 0 {
		for i, id := range m.w.Products {
			if m.w.Stock(city.ID, id) > 0 {
				m.cursor = i
				break
			}
		}
	}
	m.mode = modeShip
}

// shipRoute is the route under the cursor.
func (m *Model) shipRoute() content.RouteConfig {
	routes := m.shipRoutes()
	m.shp.route = max(0, min(m.shp.route, len(routes)-1))
	return routes[m.shp.route]
}

// maxShip is the most of a product the route will carry out of the city
// today: the stash, the route's capacity, and what the cost leaves.
func (m *Model) maxShip(id string) int {
	r := m.shipRoute()
	n := min(m.w.Stock(m.shipFrom().ID, id), r.Capacity)
	if r.Cost > 0 {
		n = min(n, m.w.Player.DirtyCash/r.Cost)
	}
	return max(0, n)
}

func (m *Model) keyShip(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.shp
	d.err = ""
	switch key {
	case "esc":
		if d.step == 0 {
			m.mode = modePlay
		} else {
			d.step--
			d.qty.Blur()
		}
		return m, nil
	case "q":
		if d.step != 2 {
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
			if m.w.Stock(m.shipFrom().ID, m.w.Products[m.cursor]) == 0 {
				d.err = "you have none of that here"
				return m, nil
			}
			d.step = 1
		}
	case 1:
		routes := m.shipRoutes()
		switch key {
		case "up", "k":
			if d.route > 0 {
				d.route--
			}
		case "down", "j":
			if d.route < len(routes)-1 {
				d.route++
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(key[0] - '1'); i < len(routes) {
				d.route = i
			}
		case "enter", "right", "l":
			if m.maxShip(m.w.Products[m.cursor]) == 0 {
				d.err = "you can't afford to send any that way"
				return m, nil
			}
			d.step = 2
			d.qty.Focus()
			return m, textinput.Blink
		}
	case 2:
		if key == "enter" {
			d.step = 3
			d.qty.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		d.qty, cmd = d.qty.Update(k)
		return m, cmd
	case 3:
		switch key {
		case "left", "h":
			if d.dial > events.ShipSlow {
				d.dial--
			}
		case "right", "l":
			if d.dial < events.ShipFast {
				d.dial++
			}
		case "1":
			d.dial = events.ShipSlow
		case "2":
			d.dial = events.ShipNormal
		case "3":
			d.dial = events.ShipFast
		case "enter":
			return m.confirmShip()
		}
	}
	return m, nil
}

func (m *Model) confirmShip() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	from := m.shipFrom().ID
	r := m.shipRoute()
	qty, err := parseQtyInput(m.shp.qty.Value(), m.maxShip(id))
	if err != nil {
		m.shp.err = err.Error()
		m.shp.step = 2
		m.shp.qty.Focus()
		return m, nil
	}
	s, err := m.set.Logistics.Ship(m.w, r.ID, from, r.Other(from), id, qty, m.shp.dial)
	if err != nil {
		m.shp.err = err.Error()
		m.shp.step = 2
		m.shp.qty.Focus()
		return m, nil
	}
	m.mode = modePlay
	m.status = fmt.Sprintf("%d %s left %s for %s by %s, %s: %d day(s), %s.", s.Units, m.w.ProductName(id), m.w.CityName(s.From), m.w.CityName(s.To), s.Mode, s.Dial, s.Arrives-s.Sent, money(s.Cost))
	return m, nil
}

func (m *Model) viewShip() string {
	w := m.w
	d := m.shp
	from := m.shipFrom()
	id := w.Products[m.cursor]
	lg := m.set.Logistics
	var b strings.Builder

	// Step 0: what to send, out of the stash here; once picked, one line.
	if d.step == 0 {
		for i, pid := range w.Products {
			line := fmt.Sprintf("%-8s  have %d", w.ProductName(pid), w.Stock(from.ID, pid))
			if i == m.cursor {
				b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(line) + "\n")
			} else {
				b.WriteString("  " + theme.Subtle.Render(line) + "\n")
			}
		}
		b.WriteString("\n" + theme.Subtle.Render("Pick a product, then enter.") + "\n")
	} else {
		b.WriteString(fmt.Sprintf("Product   %s   %s\n", theme.Selected.Render(" "+w.ProductName(id)+" "), theme.Subtle.Render(fmt.Sprintf("have %d in %s", w.Stock(from.ID, id), from.Name))))
	}

	// Step 1: the route, and what the product fetches at the far end;
	// once picked, one line.
	routes := m.shipRoutes()
	r := m.shipRoute()
	to := r.Other(from.ID)
	switch {
	case d.step == 1:
		b.WriteString("\n")
		for i, rt := range routes {
			line := fmt.Sprintf("%-11s %-5s %-8s %dd %4d units %s/u ~%.0f%%", fit(rt.Name, 11), rt.Mode, fit(w.CityName(rt.Other(from.ID)), 8), lg.Days(rt, events.ShipNormal), rt.Capacity, money(rt.Cost), lg.Risk(rt, events.ShipNormal)*100)
			if i == d.route {
				b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(line) + "\n")
			} else {
				b.WriteString("  " + theme.Subtle.Render(line) + "\n")
			}
		}
		if hp, tp := w.Product(from.ID, id), w.Product(to, id); hp != nil && tp != nil {
			b.WriteString(theme.Subtle.Render(fmt.Sprintf("%s fetches %s here, %s in %s.", w.ProductName(id), price(hp.Price), price(tp.Price), w.CityName(to))) + "\n")
		}
	case d.step > 1:
		b.WriteString(fmt.Sprintf("Route     %s   %s\n", theme.Selected.Render(" "+r.Name+" "), theme.Subtle.Render(fmt.Sprintf("%s to %s, up to %d units, %s/unit", r.Mode, w.CityName(to), r.Capacity, money(r.Cost)))))
	}

	// Step 2: quantity.
	if d.step >= 2 {
		mx := m.maxShip(id)
		b.WriteString(fmt.Sprintf("\nQuantity  %s   %s\n", d.qty.View(), theme.Subtle.Render(fmt.Sprintf("max %d", mx))))
		if qty, err := parseQtyInput(d.qty.Value(), mx); err == nil {
			cost := qty * r.Cost
			style := theme.Gold
			if cost > w.Player.DirtyCash {
				style = theme.Bad
			}
			b.WriteString(fmt.Sprintf("Cost      %s   %s\n", style.Render(money(cost)), theme.Subtle.Render("dirty cash "+cash(w.Player.DirtyCash))))
		}
	}

	// Step 3: the dial.
	if d.step >= 3 {
		names := []string{"slow", "normal", "fast"}
		var cells []string
		for i, n := range names {
			if events.Ship(i) == d.dial {
				cells = append(cells, theme.Selected.Render(" "+n+" "))
			} else {
				cells = append(cells, theme.Subtle.Render(" "+n+" "))
			}
		}
		b.WriteString("\nDial      " + strings.Join(cells, " ") + "\n")
		risk := lg.Risk(r, d.dial)
		b.WriteString(fmt.Sprintf("Expect    %d day(s), seized ~%s   %s\n", lg.Days(r, d.dial), heatStyle(risk*200).Render(fmt.Sprintf("%.0f%%", risk*100)), theme.Subtle.Render(shipBlurb(d.dial))))
	}

	if d.err != "" {
		b.WriteString("\n" + theme.Bad.Render(d.err) + "\n")
	}
	return m.modal("SHIP FROM "+strings.ToUpper(from.Name), strings.TrimRight(b.String(), "\n"))
}

func shipBlurb(d events.Ship) string {
	switch d {
	case events.ShipSlow:
		return "slow, and keeps its head down"
	case events.ShipFast:
		return "half the days; a seizure is evidence"
	default:
		return "the route as it is"
	}
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
	}
	b.WriteString(theme.Subtle.Render("Stock stays where it is; only a shipment moves it (t).\nThe supplier sells to you where you stand.") + "\n\n")
	b.WriteString(theme.Key.Render("y") + " go   " + theme.Key.Render("any other key") + " stay")
	return m.modal("GO TO "+strings.ToUpper(m.w.CityName(to))+"?", b.String())
}
