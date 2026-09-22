// The dashboard's smaller readings: the supply contracts, the cities,
// reputation, what is on the road and the connect where you stand
// (#275: out of dashboard.go).

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// supplyLine is the dashboard's one line on the supply contracts
// (#113): how many stand and what they bought this morning (`supply 2
// contracts · $3,400 this morning`). Empty with none.
func (m *Model) supplyLine() string {
	n := len(m.w.Supply)
	lt := 0 // the lieutenants' contracts (#174), counted apart
	for _, c := range m.w.DelegatedSupply {
		if _, own := m.w.Supplied(c.City, c.Product); !own && m.w.Crew.Lieutenant(c.City) != nil {
			lt++
		}
	}
	if n+lt == 0 {
		return ""
	}
	line := theme.Gold.Render("supply " + plural(n, "contract"))
	if n == 0 {
		line = theme.CrewText.Render("supply " + plural(lt, "contract") + " (lt)")
	} else if lt > 0 {
		line += theme.CrewText.Render(fmt.Sprintf(" +%d (lt)", lt))
	}
	if _, cost := m.w.SuppliedToday(); cost > 0 {
		line += sep + money(cost) + " this morning"
	}
	return line
}

// citiesLines is the CITIES panel: one line per city with the value of
// the stash there, the corners held, who runs it (you where you stand,
// the lieutenant where one does), its heat and what is on the road to
// it, the columns aligned, in width cells.
func (m *Model) citiesLines(width int) []string {
	w := m.w
	type cityRow struct {
		cells []string
		style []lipgloss.Style
	}
	var rows []cityRow
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		name, nameStyle := "  "+c.Name, theme.Subtle
		if cid == w.Player.Location {
			name, nameStyle = "◉ "+c.Name, theme.Gold
		}
		value := 0
		for id, q := range w.StashOf(cid) {
			if p := c.Market[id]; p != nil {
				value += int(float64(q) * p.SupplierPrice)
			}
		}
		held := 0
		for _, k := range c.Corners {
			if k.Held() {
				held++
			}
		}
		runs, runsStyle := "nobody", theme.Subtle
		switch lt := w.Crew.Lieutenant(cid); {
		case lt != nil:
			runs, runsStyle = lt.Name, theme.CrewText
		case cid == w.Player.Location:
			runs, runsStyle = "you", theme.Body
		}
		road, soonest := 0, 0
		for _, sh := range w.Shipments {
			if sh.To != cid {
				continue
			}
			road += sh.Units
			if d := sh.DaysLeft(w.Day); soonest == 0 || d < soonest {
				soonest = d
			}
		}
		onRoad := ""
		if road > 0 {
			onRoad = fmt.Sprintf("◂ %s in %dd", plural(road, "unit"), soonest)
		}
		rows = append(rows, cityRow{
			cells: []string{name, "stash " + cash(value), fmt.Sprintf("%d/%d corners", held, len(c.Corners)), runs, fmt.Sprintf("heat %.0f", c.Heat), onRoad},
			style: []lipgloss.Style{nameStyle, theme.Subtle, theme.Subtle, runsStyle, heatStyle(c.Heat), theme.RoadText},
		})
	}
	var widths []int
	for _, r := range rows {
		for i, c := range r.cells {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	var out []string
	for _, r := range rows {
		var parts []string
		for i, c := range r.cells {
			parts = append(parts, r.style[i].Render(fit(c, widths[i])))
		}
		out = append(out, truncate(strings.TrimRight(strings.Join(parts, "  "), " "), width))
	}
	return out
}

// reputationAxes are the dashboard's three bars: the axis, its label at
// full and at narrow width, and its accent.
var reputationAxes = []struct {
	axis, long, short string
	colour            lipgloss.Color
}{
	{"fear", "fear", "F", theme.Rivals},
	{"respect", "respect", "R", theme.Crew},
	{"notoriety", "notoriety", "N", theme.Warn},
}

// reputationLine is the three reputation bars under the heat gauge, side
// by side so they fit in one line of a panel width cells wide.
func (m *Model) reputationLine(width int) string {
	rep := m.w.Player.Reputation
	labels := len("fear ") + len("respect ") + len("notoriety ")
	long := width >= labels+2+3*3 // the words, with bars of three
	if !long {
		labels = 3 * 2 // "F "
	}
	barW := max(3, (width-labels-2)/3)
	var parts []string
	for _, a := range reputationAxes {
		label := a.short
		if long {
			label = a.long
		}
		v := *rep.Axis(a.axis)
		parts = append(parts, theme.Fg(a.colour).Render(label+" "+sparkline.Bar(v/100, barW, nil)))
	}
	return strings.Join(parts, " ")
}

// roadUnits is how many units of a product (every product for "") are
// on the road, and the soonest any of them land.
func (m *Model) roadUnits(id string) (units, soonest int) {
	for _, sh := range m.w.Shipments {
		if id != "" && sh.Product != id {
			continue
		}
		units += sh.Units
		if d := sh.DaysLeft(m.w.Day); soonest == 0 || d < soonest {
			soonest = d
		}
	}
	return units, soonest
}

// supplierLine is the street's fact on the supply side (#72): the best
// available connect where you stand and their price as a share of
// street, `Cass sells at ~55% of street`, or that nobody is selling to
// you today.
func (m *Model) supplierLine() string {
	w := m.w
	here := w.Player.Location
	var best *game.Supplier
	ratio := 0.0
	for _, sup := range w.SuppliersIn(here) {
		for _, id := range w.Products {
			p := w.Product(here, id)
			if p == nil || p.Price <= 0 || !w.Available(sup, id) {
				continue
			}
			if r := sup.Price[id] / p.Price; best == nil || r < ratio {
				best, ratio = sup, r
			}
		}
	}
	if best == nil {
		if len(w.SuppliersIn(here)) == 0 {
			return fmt.Sprintf("supplier at ~%.0f%% of street", m.set.Market.BaseRatio(w)*100)
		}
		return "nobody is selling to you today"
	}
	return fmt.Sprintf("%s sells at ~%.0f%% of street", best.Name, ratio*100)
}
