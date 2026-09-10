package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

func (m *Model) viewMarket() string {
	w := m.w
	sparkW := max(8, min(30, m.width-72))
	var b strings.Builder
	b.WriteString(theme.PanelTitle.Render("MARKET · "+w.City) + theme.Subtle.Render("   ↑↓ pick · b buy · s sell · x cancel") + "\n\n")
	b.WriteString(theme.Subtle.Render(fmt.Sprintf("  %-8s %9s %6s  %-*s %8s %6s %9s  %s",
		"", "price", "Δ", sparkW, "last 30 days", "supplier", "stock", "demand", "order")) + "\n")
	for i, id := range w.Products {
		p := w.Market[id]
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
		if o, ok := w.Orders[id]; ok {
			order = theme.Gold.Render(fmt.Sprintf("%d %s", o.Qty, o.Dial))
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
			price(p.SupplierPrice), w.Player.Stock[id],
			fmt.Sprintf("~%.0f/day", p.Demand), order)
		b.WriteString(row + "\n")
	}
	b.WriteString("\n")
	id := w.Products[m.cursor]
	p := w.Market[id]
	lo, hi := p.Price, p.Price
	for _, v := range p.History {
		lo = min(lo, v)
		hi = min(max(hi, v), 1e9)
	}
	b.WriteString(theme.PanelTitle.Render(p.Name) + "\n")
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
	return b.String()
}
