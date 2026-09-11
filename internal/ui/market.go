package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// cityTabs is the city selector on the market and map screens: the
// cities in order, the one shown highlighted, with a mark on the one you
// are in.
func (m *Model) cityTabs() string {
	var parts []string
	for _, id := range m.w.CityOrder {
		c := m.w.Cities[id]
		label := c.Name
		if id == m.w.Player.Location {
			label = "◉ " + label
		}
		if id == m.city {
			parts = append(parts, theme.Selected.Render(" "+label+" "))
		} else {
			parts = append(parts, theme.Subtle.Render(" "+label+" "))
		}
	}
	return strings.Join(parts, "")
}

// stashElsewhere is one line on what you hold of a product outside the
// city shown, and what is on the road.
func (m *Model) stashElsewhere(id string) string {
	var parts []string
	for _, cid := range m.w.CityOrder {
		if cid == m.city {
			continue
		}
		if q := m.w.Stock(cid, id); q > 0 {
			parts = append(parts, fmt.Sprintf("%d in %s", q, m.w.CityName(cid)))
		}
	}
	for _, s := range m.w.Shipments {
		if s.Product == id {
			parts = append(parts, fmt.Sprintf("%d on the road to %s (%dd)", s.Units, m.w.CityName(s.To), s.DaysLeft(m.w.Day)))
		}
	}
	return strings.Join(parts, " · ")
}

// productRows is the product table the dashboard and the market share:
// the price, its change on the day, the sparkline (sized by sparkCol
// once the caller knows what the width leaves), the stock in the city
// and the order queued there, which is the lieutenant's standing order
// where you placed none; the market adds the supplier's price and the
// demand your corners there serve. The cursor is the row of the
// product selected, or -1 when it is not in the city.
func (m *Model) productRows(city string, selected int, market bool) (cols []col, rows [][]any, cursor int) {
	w := m.w
	c := w.City(city)
	cols = []col{{"product", kText, 0}, {"price", kPrice, 0}, {"Δ", kPct, 0}, {"", kBar, 0}}
	if market {
		cols = append(cols, col{"supplier", kPrice, 0})
	}
	cols = append(cols, col{"stock", kInt, 0})
	if market {
		cols = append(cols, col{"demand/day", kInt, 0})
	}
	cols = append(cols, col{"order", kDial, 0})
	cursor = -1
	for i, id := range w.Products {
		p := c.Market[id]
		if p == nil {
			continue
		}
		if i == selected {
			cursor = len(rows)
		}
		delta := 0.0
		if n := len(p.History); n >= 2 {
			delta = pct(p.History[n-2], p.History[n-1])
		}
		var ds any = styled{theme.Subtle, signed{delta}}
		if delta > 1 {
			ds = styled{theme.Good, signed{delta}}
		} else if delta < -1 {
			ds = styled{theme.Bad, signed{delta}}
		}
		sp := spark{vs: p.History}
		if p.ShockDays > 0 {
			if p.ShockSlump {
				sp.mark = theme.Warning.Render("▼")
			} else {
				sp.mark = theme.Good.Render("▲")
			}
		}
		var ord any
		if o, ok := w.Order(city, id); ok {
			ord = styled{theme.Gold, order{o.Qty, dialShort(o.Dial), false}}
		} else if o, ok := w.StandingOrder(city, id); ok {
			ord = styled{lipgloss.NewStyle().Foreground(theme.Crew), order{o.Qty, dialShort(o.Dial), true}}
		}
		row := []any{p.Name, p.Price, ds, styled{theme.Good, sp}}
		if market {
			var supplier any = p.SupplierPrice
			if p.NoSupply {
				supplier = nil
			}
			row = append(row, supplier)
		}
		row = append(row, w.Stock(city, id))
		if market {
			row = append(row, approx{w.Demand(city, id)})
		}
		rows = append(rows, append(row, ord))
	}
	return cols, rows, cursor
}

// sparkCol sizes the product table's sparkline to width cells, or the
// days of history there are to draw, and titles it by those days.
func sparkCol(cols []col, rows [][]any, width int) {
	days := 0
	for i := range cols {
		if cols[i].kind != kBar {
			continue
		}
		for _, r := range rows {
			if sp, ok := r[i].(styled); ok {
				if x, ok := sp.v.(spark); ok {
					days = max(days, len(x.vs))
				}
			}
		}
		// No wider than the history it has to draw, so the mark
		// after a short series sits by it.
		cols[i].width = max(3, min(days, width))
		cols[i].title = fmt.Sprintf("%dd", min(days, width))
	}
}

func (m *Model) viewMarket() string {
	w := m.w
	city := m.shown()
	here := city.ID == w.Player.Location
	var b strings.Builder
	b.WriteString(truncate(theme.PanelTitle.Render("MARKET · ")+m.cityTabs()+theme.Subtle.Render("   ←→ city · b buy · s sell · g go · routes on the map"), m.width) + "\n\n")
	cols, rows, cursor := m.productRows(city.ID, m.cursor, true)
	sparkCol(cols, rows, max(3, min(30, m.width-tableWidth(cols, rows))))
	for _, l := range table(cols, rows, cursor, m.width) {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
	// The buyers (#71): the reason to visit. Their rows sit under the
	// table, and while the cursor is on one the detail below is the
	// contract's, not the product's.
	for _, l := range m.buyersLines() {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
	if c := m.selectedContract(); c != nil {
		b.WriteString(m.contractDetail(*c))
		return b.String()
	}
	id := w.Products[m.cursor]
	p := city.Market[id]
	if p == nil {
		return b.String()
	}
	lo, hi := p.Price, p.Price
	for _, v := range p.History {
		lo = min(lo, v)
		hi = min(max(hi, v), 1e9)
	}
	b.WriteString(theme.PanelTitle.Render(p.Name) + theme.Subtle.Render(" in "+city.Name) + "\n")
	b.WriteString(fmt.Sprintf("  30-day range  %s – %s\n", price(lo), price(hi)))
	b.WriteString(fmt.Sprintf("  glut          %.0f%%  %s\n", p.Glut*100, theme.Subtle.Render("(recent oversupply, pushes price down)")))
	if p.ShockDays > 0 {
		if p.ShockSlump {
			b.WriteString(theme.Warning.Render(fmt.Sprintf("  demand slump  x%.2f for %d more day(s)\n", p.ShockFactor, p.ShockDays)))
		} else {
			b.WriteString(theme.Good.Render(fmt.Sprintf("  supply shock  x%.2f for %d more day(s)\n", p.ShockFactor, p.ShockDays)))
		}
	}
	margin := 0.0
	if p.SupplierPrice > 0 {
		margin = (p.Price - p.SupplierPrice) / p.SupplierPrice * 100
	}
	if p.NoSupply {
		b.WriteString(truncate(theme.Warning.Render("  supplier      not sold here: it comes in by the road (the map's routes) or in your pockets"), m.width) + "\n")
	} else {
		b.WriteString(fmt.Sprintf("  margin        %.0f%% over supplier\n", margin))
	}
	b.WriteString(truncate(fmt.Sprintf("  demand        ~%.0f/day on your %d corner(s) here  %s", w.Demand(city.ID, id), w.WorkedIn(city.ID), theme.Subtle.Render(fmt.Sprintf("(~%.0f per standard corner; the map shows the rest)", p.Demand))), m.width) + "\n")
	// The other city's price is what a route is worth.
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if cid == city.ID {
			continue
		}
		if o := w.Product(cid, id); o != nil {
			elsewhere = append(elsewhere, fmt.Sprintf("%s %s street, %s supplier", w.CityName(cid), price(o.Price), price(o.SupplierPrice)))
		}
	}
	if len(elsewhere) > 0 {
		b.WriteString(truncate("  elsewhere     "+strings.Join(elsewhere, " · "), m.width) + "\n")
	}
	if s := m.stashElsewhere(id); s != "" {
		b.WriteString(truncate("  stash         "+s, m.width) + "\n")
	}
	if !here {
		b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  You are in %s: the supplier here sells to you there (g). Runners sell what is stashed here.", w.Here().Name)), m.width) + "\n")
	}
	if city.Wholesale {
		o := m.set.Logistics.Wholesale()
		if o.Locked(w) {
			b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  The supplier here sells lots of %d at %.0f%% to the routes once you have moved %s.", o.Lot, o.Mul*100, cash(o.UnlockCash))), m.width) + "\n")
		} else {
			b.WriteString(truncate(theme.Good.Render(fmt.Sprintf("  Wholesale: lots of %d at %.0f%% of the supplier price feed the routes out of here (map, r).", o.Lot, o.Mul*100)), m.width) + "\n")
		}
	}
	return b.String()
}
