package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// panel renders a titled bordered box of exactly w x h cells. Lines are
// truncated, never wrapped, so the box never grows past its height.
func panel(title, content string, w, h int, accent lipgloss.Color) string {
	// lipgloss Width/Height include padding but not the border.
	blockW := max(6, w-2)
	blockH := max(1, h-2)
	textW := blockW - 2
	var ls []string
	for _, l := range strings.Split(clampLines(content, blockH-1), "\n") {
		ls = append(ls, truncate(l, textW))
	}
	head := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(truncate(title, textW))
	return theme.Panel.
		BorderForeground(accent).
		Width(blockW).
		Height(blockH).
		MaxHeight(h).
		Render(head + "\n" + strings.Join(ls, "\n"))
}

func (m *Model) viewDashboard() string {
	w := m.w
	here := w.Here()
	h := m.bodyHeight()
	leftW := m.width * 3 / 5
	rightW := m.width - leftW
	if m.width < 70 {
		leftW, rightW = m.width, 0
	}

	// Street panel: product table with sparklines. Fixed columns take 42
	// cells; whatever is left goes to the sparkline.
	var street strings.Builder
	innerW := leftW - 4
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
	street.WriteString("\n")
	street.WriteString(theme.Subtle.Render(fmt.Sprintf("stash here %d/%d units · supplier sells at ~%.0f%% of street",
		w.Player.StockIn(here.ID), w.Capacity(here.ID), m.set.Market.SupplierRatio(w)*100)) + "\n")
	if line := m.elsewhereLine(); line != "" {
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
	street.WriteString(theme.Subtle.Render(m.ownedLine()) + "\n")
	if m.talking() {
		street.WriteString(theme.Bad.Render("Somebody is talking. Investigate (4, i).") + "\n")
	}
	if w.LieLow {
		street.WriteString(theme.Warning.Render("Lying low today. No sales, heat fades faster.") + "\n")
	} else if len(w.Orders) == 0 {
		street.WriteString(theme.Subtle.Render("No sales queued. Press s to sell, n to end the day.") + "\n")
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

	leftH := h
	left := panel("STREET · "+here.Name, street.String(), leftW, leftH, theme.Market)
	if rightW == 0 {
		return left
	}

	// Heat panel with gauge and thresholds.
	var heat strings.Builder
	gaugeW := max(8, rightW-8)
	var marks []float64
	var thr []string
	for _, r := range m.set.Heat.Thresholds() {
		marks = append(marks, r.Threshold/100)
		thr = append(thr, fmt.Sprintf("%.0f %s", r.Threshold, r.Level))
	}
	heat.WriteString(heatStyle(here.Heat).Render(sparkline.Bar(here.Heat/100, gaugeW, marks)) + "\n")
	line := heatStyle(here.Heat).Render(fmt.Sprintf("%.0f", here.Heat)) + theme.Subtle.Render(fmt.Sprintf(" / 100  peak %.0f", w.Heat.Peak))
	if ev := m.set.Heat.EvidenceArrest(w); ev > 0 {
		style := theme.Subtle
		if w.Heat.Evidence >= ev-2 {
			style = theme.Bad
		}
		line += style.Render(fmt.Sprintf("  file %d/%d", w.Heat.Evidence, ev))
	}
	heat.WriteString(line + "\n")
	// Thresholds two per line so they fit narrow panels.
	for i := 0; i < len(thr); i += 2 {
		heat.WriteString(theme.Subtle.Render(strings.Join(thr[i:min(i+2, len(thr))], " · ")) + "\n")
	}
	heat.WriteString(m.reputationLine(rightW-4) + "\n")
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c != here {
			elsewhere = append(elsewhere, heatStyle(c.Heat).Render(fmt.Sprintf("%s %.0f", c.Name, c.Heat)))
		}
	}
	if len(elsewhere) > 0 {
		heat.WriteString(theme.Subtle.Render("elsewhere ") + strings.Join(elsewhere, theme.Subtle.Render(" · ")) + "\n")
	}
	heatH := 8
	if len(elsewhere) > 0 {
		heatH = 9
	}
	heatPanel := panel("HEAT", heat.String(), rightW, heatH, theme.Heat)

	// Cash panel.
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
	cashH := 7
	cashPanel := panel("CASH", till.String(), rightW, cashH, theme.Money)

	// The rival: who, what they are like, how much they hold, how loud
	// the war is, and where the enforcers go tonight.
	rivalH := 5
	rivalPanel := panel("RIVALS", m.rivalLines(), rightW, rivalH, theme.Rivals)

	// Alerts: recent heat-sourced headlines.
	alertsH := h - heatH - cashH - rivalH
	var alerts strings.Builder
	n := 0
	if m.talking() {
		alerts.WriteString(theme.Bad.Bold(true).Render("Somebody is talking.") + theme.Bad.Render(" Investigate (4, i).") + "\n")
		n++
	}
	for i := len(w.Journal) - 1; i >= 0 && n < max(1, alertsH-2); i-- {
		if src := w.Journal[i].Source; src != "heat" && src != "rivals" {
			continue
		}
		alerts.WriteString(theme.Subtle.Render(fmt.Sprintf("d%-3d ", w.Journal[i].Day)) + truncate(w.Journal[i].Text, max(10, rightW-11)) + "\n")
		n++
	}
	if n == 0 {
		alerts.WriteString(theme.Subtle.Render("Nobody is looking at you. Yet."))
	}
	right := lipgloss.JoinVertical(lipgloss.Left, heatPanel, cashPanel, rivalPanel)
	if alertsH >= 3 { // a border with nothing inside is not a panel
		right = lipgloss.JoinVertical(lipgloss.Left, right, panel("ALERTS", alerts.String(), rightW, alertsH, theme.Heat))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// elsewhereLine is what you hold outside the city you are in and what is
// on the road, or "" when there is nothing.
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
