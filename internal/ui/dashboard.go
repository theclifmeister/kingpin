package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Panel heights on the dashboard, borders included: the title sits in
// the top border, so a panel is its lines plus two.
const (
	dashLawH   = 5
	dashCashH  = 6
	dashRivalH = 5
)

// streetLines is the STREET panel's content for a panel innerW cells
// wide: the product table with sparklines, then the state of the
// street. Fixed columns take 42 cells; whatever is left goes to the
// sparkline. withRoad adds the line on what is stashed elsewhere and on
// the road, for a layout without the CITIES panel.
func (m *Model) streetLines(innerW int, withRoad bool) string {
	w := m.w
	here := w.Here()
	var street strings.Builder
	sparkW := max(4, min(24, innerW-42))
	street.WriteString(theme.Subtle.Render(fmt.Sprintf("  %-8s %8s %5s %-*s %4s %-5s", "", "price", "Δ", sparkW, "30d", "stock", "order")) + "\n")
	for i, id := range w.Products {
		p := here.Market[id]
		if p == nil {
			continue
		}
		delta := 0.0
		if n := len(p.History); n >= 2 {
			delta = pct(p.History[n-2], p.History[n-1])
		}
		ds := theme.Subtle.Render(fmt.Sprintf("%+4.0f%%", delta))
		if delta > 1 {
			ds = theme.Good.Render(fmt.Sprintf("%+4.0f%%", delta))
		} else if delta < -1 {
			ds = theme.Bad.Render(fmt.Sprintf("%+4.0f%%", delta))
		}
		order := theme.Subtle.Render("-    ")
		if o, ok := w.Order(here.ID, id); ok {
			order = theme.Gold.Render(fit(fmt.Sprintf("%d%s", o.Qty, o.Dial.String()[:1]), 5))
		}
		cur := "  "
		if i == m.cursor {
			cur = theme.Gold.Render("▸ ")
		}
		row := fmt.Sprintf("%s%-8s %8s %s %s %4d %s",
			cur, truncate(p.Name, 8), price(p.Price), ds,
			theme.Good.Render(fit(sparkline.Render(p.History, sparkW), sparkW)),
			w.Stock(here.ID, id), order)
		if p.ShockDays > 0 {
			if p.ShockSlump {
				row += theme.Warning.Render(" ▼")
			} else {
				row += theme.Good.Render(" ▲")
			}
		}
		street.WriteString(row + "\n")
	}
	street.WriteString(theme.Subtle.Render(fmt.Sprintf("stash here %d/%d units · supplier sells at ~%.0f%% of street",
		w.Player.StockIn(here.ID), w.Capacity(here.ID), m.set.Market.SupplierRatio(w)*100)) + "\n")
	if line := m.elsewhereLine(); line != "" && withRoad {
		street.WriteString(lipgloss.NewStyle().Foreground(theme.Logistics).Render(line) + "\n")
	}
	if w.Worked() == 0 {
		street.WriteString(theme.Bad.Render("You hold no corner, so nothing sells. Claim one on the map (5).") + "\n")
	} else {
		held := 0
		for _, c := range here.Corners {
			if c.Held() {
				held++
			}
		}
		corners := fmt.Sprintf("corners %d worked, %d held of %d", w.WorkedIn(here.ID), held, len(here.Corners))
		if n := w.RivalHeld(); n > 0 && here == w.Home() {
			corners += fmt.Sprintf(", %d theirs", n)
		}
		if n := w.Worked() - w.WorkedIn(here.ID); n > 0 {
			corners += fmt.Sprintf(", %d worked elsewhere", n)
		}
		street.WriteString(theme.Rival.Render(corners) + "\n")
	}
	if n := len(w.Crew.Members); n > 0 {
		crew := fmt.Sprintf("crew %d · %s pay %s/day", n, w.Crew.Pay, money(m.set.Crew.Wages(w, w.Crew.Pay)))
		switch {
		case w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < m.set.Crew.Tuning().SuspectDays:
			street.WriteString(lipgloss.NewStyle().Foreground(theme.Crew).Render(crew) + theme.Bad.Render(" · skimming suspected") + "\n")
		default:
			street.WriteString(lipgloss.NewStyle().Foreground(theme.Crew).Render(crew) + "\n")
		}
	}
	if line := m.runsLine(); line != "" {
		street.WriteString(truncate(lipgloss.NewStyle().Foreground(theme.Crew).Render(line), innerW) + "\n")
	}
	if line := m.contractsLine(); line != "" {
		street.WriteString(truncate(line, innerW) + "\n")
	}
	street.WriteString(theme.Subtle.Render(m.ownedLine()) + "\n")
	if m.talking() {
		street.WriteString(theme.Bad.Render("Somebody is talking. Investigate (4, i).") + "\n")
	}
	if w.LieLow {
		street.WriteString(theme.Warning.Render("Lying low today. No sales, heat fades faster.") + "\n")
	} else if len(w.Orders) == 0 {
		if lt := w.Crew.Lieutenant(here.ID); lt != nil && m.standingHere() > 0 {
			street.WriteString(theme.Gold.Render(fmt.Sprintf("%s sells the stash here at %s. Press s to override, n to end the day.", lt.Name, m.set.Crew.Dial(*lt))) + "\n")
		} else {
			street.WriteString(theme.Subtle.Render("No sales queued. Press s to sell, n to end the day.") + "\n")
		}
	} else {
		street.WriteString(theme.Gold.Render("Orders queued. Press n to end the day.") + "\n")
	}
	if w.Heat.SellCapDays > 0 {
		street.WriteString(theme.Bad.Render(fmt.Sprintf("Patrols: sales capped at %.0f%% of demand for %d more day(s).", w.Heat.SellCap*100, w.Heat.SellCapDays)) + "\n")
	}
	if s := w.Strike; s != nil {
		if c := w.Corner(s.Corner); c != nil {
			street.WriteString(theme.Rival.Render(fmt.Sprintf("Enforcers go to %s tonight: %s.", c.Name, s.Force)) + "\n")
		}
	}
	return street.String()
}

// otherHeat is the other cities' heat, `Bayport 1`, joined, or "".
func (m *Model) otherHeat() string {
	w := m.w
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c != w.Here() {
			elsewhere = append(elsewhere, heatStyle(c.Heat).Render(fmt.Sprintf("%s %.0f", c.Name, c.Heat)))
		}
	}
	return strings.Join(elsewhere, theme.Subtle.Render(" · "))
}

// heatLines is the HEAT panel's content for a panel width cells wide:
// the gauge with the thresholds marked, the numbers, the thresholds,
// the reputation bars and, if elsewhere, the other city's heat. It
// returns the lines and how many there are.
func (m *Model) heatLines(width int, elsewhere bool) (string, int) {
	w := m.w
	here := w.Here()
	var heat strings.Builder
	n := 0
	gaugeW := max(8, width-4)
	var marks []float64
	var thr []string
	for _, r := range m.set.Heat.ThresholdsIn(w, here) {
		marks = append(marks, r.Threshold/100)
		thr = append(thr, fmt.Sprintf("%.0f %s", r.Threshold, r.Level))
	}
	heat.WriteString(heatStyle(here.Heat).Render(sparkline.Bar(here.Heat/100, gaugeW, marks)) + "\n")
	n++
	line := heatStyle(here.Heat).Render(fmt.Sprintf("%.0f", here.Heat)) + theme.Subtle.Render(fmt.Sprintf(" / 100  peak %.0f", w.Heat.Peak))
	if ev := m.set.Heat.EvidenceArrest(w); ev > 0 {
		style := theme.Subtle
		if w.Heat.Evidence >= ev-2 {
			style = theme.Bad
		}
		line += style.Render(fmt.Sprintf("  file %d/%d", w.Heat.Evidence, ev))
	}
	heat.WriteString(line + "\n")
	n++
	// Thresholds on one line where they fit, else two per line.
	if all := strings.Join(thr, " · "); lipgloss.Width(all) <= width-4 {
		heat.WriteString(theme.Subtle.Render(all) + "\n")
		n++
	} else {
		for i := 0; i < len(thr); i += 2 {
			heat.WriteString(theme.Subtle.Render(strings.Join(thr[i:min(i+2, len(thr))], " · ")) + "\n")
			n++
		}
	}
	heat.WriteString(m.reputationLine(width-4) + "\n")
	n++
	if other := m.otherHeat(); other != "" && elsewhere {
		heat.WriteString(theme.Subtle.Render("elsewhere ") + other + "\n")
		n++
	}
	return heat.String(), n
}

// cashLines is the CASH panel's content: the two pools, the peak, a
// word on what the dirty pile draws and, if elsewhere, the other
// city's heat (the layout that keeps HEAT to four lines puts it here).
func (m *Model) cashLines(elsewhere bool) string {
	w := m.w
	var till strings.Builder
	till.WriteString(theme.Gold.Render("dirty  "+cash(w.Player.DirtyCash)) + "\n")
	clean := theme.Subtle.Render("clean  " + cash(w.Player.CleanCash))
	if len(w.Fronts) > 0 {
		clean += theme.Subtle.Render(fmt.Sprintf("  +%s/day %s", cash(m.set.Laundering.Capacity(w)), w.Laundering.Dial))
	}
	till.WriteString(clean + "\n")
	till.WriteString(theme.Subtle.Render(fmt.Sprintf("peak   %s", cash(w.Stats.PeakCash))) + "\n")
	switch rows := m.frontRows(); {
	case m.cfg.Heat.Heat.DirtyCashThreshold > 0 && w.Player.DirtyCash > m.cfg.Heat.Heat.DirtyCashThreshold:
		till.WriteString(theme.Warning.Render(fmt.Sprintf("dirty cash over %s draws heat", cash(m.cfg.Heat.Heat.DirtyCashThreshold))) + "\n")
	case len(w.Fronts) == 0 && len(rows) > 0 && !rows[0].Locked(w):
		till.WriteString(theme.Subtle.Render("a front is on offer (7)") + "\n")
	}
	if other := m.otherHeat(); other != "" && elsewhere {
		till.WriteString(theme.Subtle.Render("heat elsewhere ") + other + "\n")
	}
	return till.String()
}

// alertLines are the ALERTS: somebody talking, a contract due, then the
// recent heat, rival and law headlines, at most n lines width cells
// wide.
func (m *Model) alertLines(width, n int) []string {
	w := m.w
	var out []string
	if m.talking() {
		out = append(out, theme.Bad.Bold(true).Render("Somebody is talking.")+theme.Bad.Render(" Investigate (4, i)."))
	}
	for _, a := range m.contractAlerts() {
		out = append(out, truncate(a, max(10, width)))
	}
	for i := len(w.Journal) - 1; i >= 0 && len(out) < max(1, n); i-- {
		if src := w.Journal[i].Source; src != "heat" && src != "rivals" && src != "law" {
			continue
		}
		out = append(out, theme.Subtle.Render(fmt.Sprintf("d%-3d ", w.Journal[i].Day))+truncate(w.Journal[i].Text, max(10, width-5)))
	}
	if len(out) == 0 {
		out = append(out, theme.Subtle.Render("Nobody is looking at you. Yet."))
	}
	return out
}

// viewDashboard is the overview. Two layouts: at paneMinWidth columns
// and up (the pane's regime) STREET runs the width, then HEAT beside
// CASH, LAW beside RIVALS, and CITIES under them; narrower than that
// the street and the law are the left column and the heat, the cash
// and the rival the right, the way 80 columns has always read. The
// ALERTS are the pane's; MAIN carries them only where no pane sits
// beside it and there is room.
func (m *Model) viewDashboard() string {
	w := m.w
	here := w.Here()
	h := m.mainHeight()
	width := m.mainWidth()
	if m.width >= paneMinWidth {
		return m.dashboardWide(width, h)
	}
	leftW := width * 3 / 5
	rightW := width - leftW
	if width < 70 {
		leftW, rightW = width, 0
	}

	// The law sits under the street: who the chief and the DA are, and
	// how loud the city is; under that, where the terminal is tall enough
	// for the street to keep its lines, the cities side by side: what is
	// stashed in each, who runs it, its heat and what is on the road to
	// it. Where it is not, the street carries the road in a line.
	citiesH := 0
	if len(w.CityOrder) > 1 && h-dashLawH-(2+len(w.CityOrder)) >= 15 {
		citiesH = 2 + len(w.CityOrder)
	}
	left := lipgloss.JoinVertical(lipgloss.Left,
		panel("STREET · "+here.Name, m.streetLines(leftW-4, citiesH == 0), leftW, h-dashLawH-citiesH, theme.Market),
		panel("LAW", m.lawLines(leftW-4), leftW, dashLawH, theme.Heat))
	if citiesH > 0 {
		left = lipgloss.JoinVertical(lipgloss.Left, left, panel("CITIES", m.citiesLines(leftW-4), leftW, citiesH, theme.Logistics))
	}
	if rightW == 0 {
		return left
	}
	heat, heatN := m.heatLines(rightW, true)
	heatH := heatN + 2
	right := lipgloss.JoinVertical(lipgloss.Left,
		panel("HEAT", heat, rightW, heatH, theme.Heat),
		panel("CASH", m.cashLines(false), rightW, dashCashH, theme.Money),
		panel("RIVALS", m.rivalLines(), rightW, dashRivalH, theme.Rivals))
	if alertsH := h - heatH - dashCashH - dashRivalH; alertsH >= 3 && !m.paneShown() { // a border with nothing inside is not a panel
		right = lipgloss.JoinVertical(lipgloss.Left, right, panel("ALERTS", strings.Join(m.alertLines(rightW-4, alertsH-2), "\n"), rightW, alertsH, theme.Heat))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// dashboardWide is the layout for the pane's regime: STREET over HEAT |
// CASH over LAW | RIVALS over CITIES, each row of panels sized to its
// lines, the street taking what is left.
func (m *Model) dashboardWide(width, h int) string {
	w := m.w
	here := w.Here()
	leftW := width * 11 / 20
	rightW := width - leftW
	heat, heatN := m.heatLines(leftW, false)
	heatH := max(heatN, 5) + 2 // CASH beside it has five lines
	citiesH := 0
	if len(w.CityOrder) > 1 && h-heatH-dashLawH-(2+len(w.CityOrder)) >= 12 {
		citiesH = 2 + len(w.CityOrder)
	}
	streetH := h - heatH - dashLawH - citiesH
	// The street takes no more than its lines need; what it leaves goes
	// to ALERTS where no pane carries them.
	street := m.streetLines(width-4, citiesH == 0)
	if n := strings.Count(strings.TrimRight(street, "\n"), "\n") + 3; n < streetH {
		streetH = n
	}
	rows := []string{
		panel("STREET · "+here.Name, street, width, streetH, theme.Market),
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("HEAT", heat, leftW, heatH, theme.Heat),
			panel("CASH", m.cashLines(true), rightW, heatH, theme.Money)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("LAW", m.lawLines(leftW-4), leftW, dashLawH, theme.Heat),
			panel("RIVALS", m.rivalLines(), rightW, dashLawH, theme.Rivals)),
	}
	if citiesH > 0 {
		rows = append(rows, panel("CITIES", m.citiesLines(width-4), width, citiesH, theme.Logistics))
	}
	if alertsH := h - streetH - heatH - dashLawH - citiesH; alertsH >= 3 && !m.paneShown() {
		rows = append(rows, panel("ALERTS", strings.Join(m.alertLines(width-4, alertsH-2), "\n"), width, alertsH, theme.Heat))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// dashboardDetails is the dashboard's pane: the product under the
// cursor, the alerts, and the keys.
func (m *Model) dashboardDetails() ([]section, []binding) {
	w := m.w
	here := w.Here()
	var secs []section
	if m.cursor < len(w.Products) {
		id := w.Products[m.cursor]
		if p := here.Market[id]; p != nil {
			delta := 0.0
			if n := len(p.History); n >= 2 {
				delta = pct(p.History[n-2], p.History[n-1])
			}
			ds := theme.Subtle.Render(fmt.Sprintf("%+.0f%%", delta))
			if delta > 1 {
				ds = theme.Good.Render(fmt.Sprintf("%+.0f%%", delta))
			} else if delta < -1 {
				ds = theme.Bad.Render(fmt.Sprintf("%+.0f%%", delta))
			}
			lines := []string{row("price", price(p.Price)+" "+ds)}
			if p.NoSupply {
				lines = append(lines, row("supplier", theme.Subtle.Render("not sold here")))
			} else {
				margin := 0.0
				if p.SupplierPrice > 0 {
					margin = (p.Price - p.SupplierPrice) / p.SupplierPrice * 100
				}
				lines = append(lines, row("supplier", fmt.Sprintf("%s · margin %.0f%%", price(p.SupplierPrice), margin)))
			}
			stock := []string{fmt.Sprintf("%d here", w.Stock(here.ID, id))}
			for _, cid := range w.CityOrder {
				if q := w.Stock(cid, id); q > 0 && cid != here.ID {
					stock = append(stock, fmt.Sprintf("%d in %s", q, w.CityName(cid)))
				}
			}
			if road, soonest := m.roadUnits(id); road > 0 {
				stock = append(stock, fmt.Sprintf("%d on the road, %dd", road, soonest))
			}
			label := "stock"
			for _, l := range wrap(strings.Join(stock, " · "), paneTextW-paneLabelW-1) {
				lines = append(lines, row(label, l))
				label = ""
			}
			corners := "corners"
			if w.WorkedIn(here.ID) == 1 {
				corners = "corner"
			}
			lines = append(lines, row("demand", fmt.Sprintf("~%.0f/day on %d %s", w.Demand(here.ID, id), w.WorkedIn(here.ID), corners)))
			switch o, ok := w.Order(here.ID, id); {
			case ok:
				lines = append(lines, row("order", theme.Gold.Render(fmt.Sprintf("%d %s", o.Qty, o.Dial))))
			default:
				if so, ok := w.StandingOrder(here.ID, id); ok {
					lines = append(lines, row("order", lipgloss.NewStyle().Foreground(theme.Crew).Render(fmt.Sprintf("%d %s (lt)", so.Qty, so.Dial))))
				} else {
					lines = append(lines, row("order", theme.Subtle.Render("none")))
				}
			}
			secs = append(secs, section{strings.ToUpper(p.Name) + " · " + strings.ToUpper(here.Name), lines})
		}
	}
	secs = append(secs, section{"ALERTS", m.alertLines(paneTextW, 8)})
	return secs, m.screenKeys()
}

// elsewhereLine is what you hold outside the city you are in and what is
// on the road, for a terminal too short for the CITIES panel, or ""
// when there is nothing.
func (m *Model) elsewhereLine() string {
	w := m.w
	var parts []string
	for _, cid := range w.CityOrder {
		if cid == w.Player.Location {
			continue
		}
		if n := w.Player.StockIn(cid); n > 0 {
			parts = append(parts, fmt.Sprintf("%d units in %s", n, w.CityName(cid)))
		}
	}
	road, soonest := 0, 0
	for _, s := range w.Shipments {
		road += s.Units
		if d := s.DaysLeft(w.Day); soonest == 0 || d < soonest {
			soonest = d
		}
	}
	if road > 0 {
		parts = append(parts, fmt.Sprintf("%d units on the road, next in %dd", road, soonest))
	}
	return strings.Join(parts, " · ")
}

// citiesLines is the CITIES panel: one line per city with the value of
// the stash there, the corners held, who runs it (you where you stand,
// the lieutenant where one does), its heat and what is on the road to
// it, in width cells.
func (m *Model) citiesLines(width int) string {
	w := m.w
	var b strings.Builder
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		mark := "  "
		if cid == w.Player.Location {
			mark = theme.Gold.Render("◉ ")
		}
		value := 0
		for id, q := range w.Player.Stash[cid] {
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
		runsW := 6
		if width >= 60 {
			runsW = 10
		}
		runs := theme.Subtle.Render(fit("-", runsW))
		switch lt := w.Crew.Lieutenant(cid); {
		case lt != nil:
			runs = lipgloss.NewStyle().Foreground(theme.Crew).Render(fit(lt.Name, runsW))
		case cid == w.Player.Location:
			runs = fit("you", runsW)
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
		line := mark + fit(c.Name, 8) + " " + theme.Gold.Render(fit(cash(value), 5)) + theme.Subtle.Render(fit(fmt.Sprintf(" %d/%d", held, len(c.Corners)), 6)) + runs + " " + heatStyle(c.Heat).Render(fmt.Sprintf("heat %.0f", c.Heat))
		if road > 0 {
			line += lipgloss.NewStyle().Foreground(theme.Logistics).Render(fmt.Sprintf(" ◂%d %dd", road, soonest))
		}
		b.WriteString(truncate(line, width) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
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
	long := width >= 44
	labels := 3 * 2 // "F "
	if long {
		labels = len("fear ") + len("respect ") + len("notoriety ")
	}
	barW := max(3, (width-labels-2)/3)
	var parts []string
	for _, a := range reputationAxes {
		label := a.short
		if long {
			label = a.long
		}
		v := *rep.Axis(a.axis)
		parts = append(parts, lipgloss.NewStyle().Foreground(a.colour).Render(label+" "+sparkline.Bar(v/100, barW, nil)))
	}
	return strings.Join(parts, " ")
}

// roadUnits is how many units of a product are on the road, and the
// soonest any of them land.
func (m *Model) roadUnits(id string) (units, soonest int) {
	for _, sh := range m.w.Shipments {
		if sh.Product != id {
			continue
		}
		units += sh.Units
		if d := sh.DaysLeft(m.w.Day); soonest == 0 || d < soonest {
			soonest = d
		}
	}
	return units, soonest
}
