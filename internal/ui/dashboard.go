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
		p := w.Market[id]
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
		if o, ok := w.Orders[id]; ok {
			order = theme.Gold.Render(fit(fmt.Sprintf("%d%s", o.Qty, o.Dial.String()[:1]), 5))
		}
		cur := "  "
		if i == m.cursor {
			cur = theme.Gold.Render("▸ ")
		}
		row := fmt.Sprintf("%s%-8s %8s %s %s %4d %s",
			cur, truncate(p.Name, 8), price(p.Price), ds,
			theme.Good.Render(fit(sparkline.Render(p.History, sparkW), sparkW)),
			w.Player.Stock[id], order)
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
	street.WriteString(theme.Subtle.Render(fmt.Sprintf("carrying %d/%d units · supplier sells at ~%.0f%% of street",
		w.Player.TotalStock(), w.Capacity(), m.cfg.Market.Market.SupplierRatio*100)) + "\n")
	if w.Worked() == 0 {
		street.WriteString(theme.Bad.Render("You hold no corner, so nothing sells. Claim one on the map (5).") + "\n")
	} else {
		corners := fmt.Sprintf("corners %d worked, %d held of %d", w.Worked(), w.Held(), len(w.Territory.Corners))
		if n := w.RivalHeld(); n > 0 {
			corners += fmt.Sprintf(", %d theirs", n)
		}
		street.WriteString(theme.Rival.Render(corners) + "\n")
	}
	if n := len(w.Crew.Members); n > 0 {
		crew := fmt.Sprintf("crew %d · %s pay %s/day", n, w.Crew.Pay, money(m.set.Crew.Wages(w, w.Crew.Pay)))
		if w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < m.set.Crew.Tuning().SuspectDays {
			street.WriteString(lipgloss.NewStyle().Foreground(theme.Crew).Render(crew) + theme.Bad.Render(" · skimming suspected") + "\n")
		} else {
			street.WriteString(lipgloss.NewStyle().Foreground(theme.Crew).Render(crew) + "\n")
		}
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
	left := panel("STREET · "+w.City, street.String(), leftW, leftH, theme.Market)
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
	heat.WriteString(heatStyle(w.Heat.Value).Render(sparkline.Bar(w.Heat.Value/100, gaugeW, marks)) + "\n")
	line := heatStyle(w.Heat.Value).Render(fmt.Sprintf("%.0f", w.Heat.Value)) + theme.Subtle.Render(fmt.Sprintf(" / 100  peak %.0f", w.Heat.Peak))
	if ev := m.cfg.Heat.Heat.EvidenceArrest; ev > 0 {
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
	heatH := 7
	heatPanel := panel("HEAT", heat.String(), rightW, heatH, theme.Heat)

	// Cash panel.
	var till strings.Builder
	till.WriteString(theme.Gold.Render("dirty  "+cash(w.Player.DirtyCash)) + "\n")
	till.WriteString(theme.Subtle.Render("clean  "+cash(w.Player.CleanCash)) + "\n")
	till.WriteString(theme.Subtle.Render(fmt.Sprintf("peak   %s", cash(w.Stats.PeakCash))) + "\n")
	if thr := m.cfg.Heat.Heat.DirtyCashThreshold; thr > 0 && w.Player.DirtyCash > thr {
		till.WriteString(theme.Warning.Render(fmt.Sprintf("dirty cash over %s draws heat", cash(thr))) + "\n")
	}
	cashH := 6
	cashPanel := panel("CASH", till.String(), rightW, cashH, theme.Money)

	// The rival: who, what they are like, how much they hold, how loud
	// the war is, and where the enforcers go tonight.
	rivalH := 5
	rivalPanel := panel("RIVALS", m.rivalLines(), rightW, rivalH, theme.Rivals)

	// Alerts: recent heat-sourced headlines.
	alertsH := h - heatH - cashH - rivalH
	var alerts strings.Builder
	n := 0
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
	alertsPanel := panel("ALERTS", alerts.String(), rightW, alertsH, theme.Heat)

	right := lipgloss.JoinVertical(lipgloss.Left, heatPanel, cashPanel, rivalPanel, alertsPanel)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}
