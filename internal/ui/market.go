package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
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

func (m *Model) viewMarket() string {
	w := m.w
	city := m.shown()
	width := m.mainWidth()
	sparkW := max(8, min(30, width-72))
	var b strings.Builder
	b.WriteString(truncate(theme.PanelTitle.Render("MARKET · ")+m.cityTabs(), width) + "\n\n")
	b.WriteString(theme.Subtle.Render(fmt.Sprintf("  %-8s %9s %6s  %-*s %8s %6s %9s  %s",
		"", "price", "Δ", sparkW, "last 30 days", "supplier", "stash", "demand", "order")) + "\n")
	for i, id := range w.Products {
		p := city.Market[id]
		if p == nil {
			continue
		}
		delta := 0.0
		if n := len(p.History); n >= 2 {
			delta = pct(p.History[n-2], p.History[n-1])
		}
		ds := theme.Subtle.Render(fmt.Sprintf("%+5.0f%%", delta))
		if delta > 1 {
			ds = theme.Good.Render(fmt.Sprintf("%+5.0f%%", delta))
		} else if delta < -1 {
			ds = theme.Bad.Render(fmt.Sprintf("%+5.0f%%", delta))
		}
		order := theme.Subtle.Render("-")
		if o, ok := w.Order(city.ID, id); ok {
			order = theme.Gold.Render(fmt.Sprintf("%d %s", o.Qty, o.Dial))
		} else if o, ok := w.StandingOrder(city.ID, id); ok {
			order = lipgloss.NewStyle().Foreground(theme.Crew).Render(fmt.Sprintf("%d %s (lt)", o.Qty, o.Dial))
		}
		name := fit(p.Name, 8)
		cur := "  "
		if i == m.cursor {
			cur = theme.Gold.Render("▸ ")
			name = theme.Selected.Render(name)
		}
		supplier := price(p.SupplierPrice)
		if p.NoSupply {
			supplier = theme.Subtle.Render("not sold")
		}
		row := fmt.Sprintf("%s%s %9s %s  %s %8s %6d %9s  %s",
			cur, name, price(p.Price), ds,
			theme.Good.Render(fit(sparkline.Render(p.History, sparkW), sparkW)),
			supplier, w.Stock(city.ID, id),
			fmt.Sprintf("~%.0f/day", w.Demand(city.ID, id)), order)
		b.WriteString(truncate(row, width) + "\n")
	}
	b.WriteString("\n")
	// The buyers (#71): the reason to visit. Their rows sit under the
	// table; the detail of the product or the contract under the cursor
	// is the pane's.
	for _, l := range m.buyersLines() {
		b.WriteString(l + "\n")
	}
	return b.String()
}

// marketDetails is the market's pane: the product under the cursor in
// the city shown (its range, glut, margin and demand), what it is worth
// elsewhere, the notes on where you are and the wholesaler, and the
// keys; the contract's terms instead while the cursor is on the buyers.
func (m *Model) marketDetails() ([]section, []binding) {
	w := m.w
	city := m.shown()
	here := city.ID == w.Player.Location
	if c := m.selectedContract(); c != nil {
		return m.contractSections(*c), m.screenKeys()
	}
	if m.cursor >= len(w.Products) {
		return nil, m.screenKeys()
	}
	id := w.Products[m.cursor]
	p := city.Market[id]
	if p == nil {
		return nil, m.screenKeys()
	}
	lo, hi := p.Price, p.Price
	for _, v := range p.History {
		lo = min(lo, v)
		hi = min(max(hi, v), 1e9)
	}
	sel := []string{
		row("range 30d", fmt.Sprintf("%s – %s", price(lo), price(hi))),
		row("glut", fmt.Sprintf("%.0f%%", p.Glut*100)),
	}
	if p.ShockDays > 0 {
		if p.ShockSlump {
			sel = append(sel, row("slump", theme.Warning.Render(fmt.Sprintf("×%.2f, %d more days", p.ShockFactor, p.ShockDays))))
		} else {
			sel = append(sel, row("shock", theme.Good.Render(fmt.Sprintf("×%.2f, %d more days", p.ShockFactor, p.ShockDays))))
		}
	}
	margin := 0.0
	if p.SupplierPrice > 0 {
		margin = (p.Price - p.SupplierPrice) / p.SupplierPrice * 100
	}
	if p.NoSupply {
		sel = append(sel, row("supplier", theme.Warning.Render("not sold here")))
	} else {
		sel = append(sel, row("margin", fmt.Sprintf("%.0f%% over supplier", margin)))
	}
	corners := "corners"
	if w.WorkedIn(city.ID) == 1 {
		corners = "corner"
	}
	sel = append(sel,
		row("demand", fmt.Sprintf("~%.0f/day, %d %s", w.Demand(city.ID, id), w.WorkedIn(city.ID), corners)),
		row("", theme.Subtle.Render(fmt.Sprintf("~%.0f/standard corner", p.Demand))))
	secs := []section{{strings.ToUpper(p.Name) + " · " + strings.ToUpper(city.Name), sel}}
	// The other city's price is what a route is worth.
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if cid == city.ID {
			continue
		}
		if o := w.Product(cid, id); o != nil {
			elsewhere = append(elsewhere, row(w.CityName(cid), fmt.Sprintf("%s · sup %s", price(o.Price), price(o.SupplierPrice))))
		}
		if q := w.Stock(cid, id); q > 0 {
			elsewhere = append(elsewhere, row("stash", fmt.Sprintf("%d in %s", q, w.CityName(cid))))
		}
	}
	for _, sh := range w.Shipments {
		if sh.Product == id {
			elsewhere = append(elsewhere, row("road", lipgloss.NewStyle().Foreground(theme.Logistics).Render(fmt.Sprintf("%d → %s, %dd", sh.Units, w.CityName(sh.To), sh.DaysLeft(w.Day)))))
		}
	}
	if len(elsewhere) > 0 {
		secs = append(secs, section{"ELSEWHERE", elsewhere})
	}
	var notes []string
	if p.NoSupply {
		notes = append(notes, wrapped(theme.Warning, "Not sold here: it comes in by the road (the map's routes) or in your pockets.")...)
	}
	if !here {
		notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("You are in %s: the supplier here sells to you there (g). Runners sell what is stashed here.", w.Here().Name))...)
	}
	if city.Wholesale {
		o := m.set.Logistics.Wholesale()
		if o.Locked(w) {
			notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("The supplier here sells lots of %d at %.0f%% to the routes once you have moved %s.", o.Lot, o.Mul*100, cash(o.UnlockCash)))...)
		} else {
			notes = append(notes, wrapped(theme.Good, fmt.Sprintf("Wholesale: lots of %d at %.0f%% of the supplier price feed the routes out of here (map, r).", o.Lot, o.Mul*100))...)
		}
	}
	if len(notes) > 0 {
		secs = append(secs, section{"NOTES", notes})
	}
	return secs, m.screenKeys()
}
