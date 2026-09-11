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

func (m *Model) viewMarket() string {
	w := m.w
	city := m.shown()
	here := city.ID == w.Player.Location
	sparkW := max(8, min(30, m.width-72))
	var b strings.Builder
	b.WriteString(truncate(theme.PanelTitle.Render("MARKET · ")+m.cityTabs()+theme.Subtle.Render("   ←→ city · b buy · s sell · g go · routes on the map"), m.width) + "\n\n")
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
		row := fmt.Sprintf("%s%s %9s %s  %s %8s %6d %9s  %s",
			cur, name, price(p.Price), ds,
			theme.Good.Render(fit(sparkline.Render(p.History, sparkW), sparkW)),
			price(p.SupplierPrice), w.Stock(city.ID, id),
			fmt.Sprintf("~%.0f/day", w.Demand(city.ID, id)), order)
		b.WriteString(truncate(row, m.width) + "\n")
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
	b.WriteString(fmt.Sprintf("  margin        %.0f%% over supplier\n", margin))
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
