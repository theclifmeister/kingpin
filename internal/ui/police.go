// The POLICE section (#355): the police risk in a city in one place,
// heat, the file, pressure and the cash exposure, each said in a clause.
// The dashboard's pane leads with it while the arrows are past the
// product table on HEAT, CASH and LAW (onPolice), and the map's carries
// it for the city shown, under the corner.

package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// dashboardMove is the dashboard's arrows: down the product table and,
// past its last row, onto the police (HEAT, CASH and LAW), whose
// section then leads the pane; up from there is the table again.
func (m *Model) dashboardMove(dy int) {
	switch {
	case m.onPolice && dy < 0:
		m.onPolice = false
	case !m.onPolice && dy > 0 && m.cursor >= len(m.w.Products)-1:
		m.onPolice = true
	case !m.onPolice:
		m.productMove(dy)
	}
}

// policeTitle is a dashboard panel's title, marked while the arrows are
// on the police.
func (m *Model) policeTitle(title string) string {
	if m.onPolice && m.screen == screenDashboard {
		return "▸ " + title
	}
	return title
}

// rungName is a rung as the pane's label column holds it: the task
// force is the feds.
func rungName(level string) string {
	if level == content.TaskForce {
		return "feds"
	}
	return level
}

// rungWords is what a rung does, as a row's value: the share of the
// stock and the dirty cash it takes, a patrol's cap and its days, the
// feds' asset, the arrest's end. Two lines for the feds.
func rungWords(r content.ResponseConfig) []string {
	take := fmt.Sprintf("%s stock, %s cash", format.Pct(r.StockLoss, 0), format.Pct(r.CashLoss, 0))
	switch r.Level {
	case content.Patrol:
		return []string{fmt.Sprintf("sales ~%s for %dd", format.Pct(r.Cap, 0), r.CapDays)}
	case content.Arrest:
		return []string{"a warrant, then a cell"} // #475: served the next night on a sale or the heat held
	case content.TaskForce:
		return []string{"an asset, and", take}
	}
	return []string{take}
}

// policeSection is the police risk in a city: the heat and how far the
// next rung is, each rung's line and what it takes, the file and what
// fills it, the pressure and the goodwill and what they move, the dirty
// pile against the line with the fronts' cover, and the cop's word as an
// estimate. The lines are heat.Sim's (Rungs, EvidenceArrest,
// ExposureLine, Cover), so the pane and the dice agree; the forecast is
// the file's (game.Known), never the truth.
func (m *Model) policeSection(city *game.City) section {
	w := m.w
	rungs := m.rules.Heat.Rungs(w, city)
	var lines []string

	// The heat, and the first rung it is under.
	heat := fmt.Sprintf("%.0f", city.Heat)
	for _, r := range rungs {
		if city.Heat < r.Threshold {
			heat += fmt.Sprintf(" · %s in %.0f", rungName(r.Level), r.Threshold-city.Heat)
			break
		}
	}
	lines = append(lines, row("heat", heatStyle(city.Heat).Render(heat)))
	next := true
	for _, r := range rungs {
		style := theme.Subtle
		switch {
		case city.Heat >= r.Threshold:
			style = theme.Bad
		case next:
			style, next = theme.Warning, false
		}
		label := fmt.Sprintf("%s %.0f", rungName(r.Level), r.Threshold)
		for _, v := range rungWords(r) {
			lines = append(lines, row(label, style.Render(v)))
			label = ""
		}
	}

	// The file: how thick, how far from the line, and what adds a page.
	if arrest := m.rules.Heat.EvidenceArrest(w); arrest > 0 {
		left := max(0, arrest-w.Heat.Evidence)
		style := theme.Subtle
		if left <= 2 {
			style = theme.Bad
		}
		lines = append(lines, row("file", style.Render(fmt.Sprintf("%d/%d · %s to go", w.Heat.Evidence, arrest, plural(left, "page")))))
		var pages []string
		for _, r := range rungs {
			if r.Evidence > 0 {
				name := strings.ToUpper(r.Level[:1]) + r.Level[1:]
				if len(pages) > 0 {
					name = r.Level
				}
				if r.Level == content.TaskForce {
					name = "feds"
				}
				pages = append(pages, fmt.Sprintf("%s %d", name, r.Evidence))
			}
		}
		note := "A quiet day files nothing."
		if len(pages) > 0 {
			note = strings.Join(pages, ", ") + " pages, if you sold; a quiet day files nothing."
		}
		lines = append(lines, m.wrapped(theme.Subtle, note)...)
	} else {
		lines = append(lines, row("file", theme.Subtle.Render("no case can be made")))
	}

	// Pressure and goodwill: what the one moves and the other takes off.
	pg := fmt.Sprintf("%.0f", city.Pressure)
	if city.Goodwill > 0 {
		pg += theme.Good.Render(fmt.Sprintf(" · goodwill %.0f", city.Goodwill))
	}
	lines = append(lines, row("pressure", theme.Bad.Render(pg)))
	fx := m.cfg.Law.Effects
	lt := m.rules.Law.Tuning()
	note := fmt.Sprintf("Lowers the police lines %s and a patrol's cap %s; fades to %.0f.",
		format.Pct(1-content.Cut(city.Pressure, fx.PressureThresholdCut), 0), format.Pct(1-content.Cut(city.Pressure, fx.PressureCapCut), 0), lt.Baseline)
	if city.Goodwill > 0 {
		note += fmt.Sprintf(" Goodwill takes %.1f a day.", lt.GoodwillCut*city.Goodwill/100)
	}
	lines = append(lines, m.wrapped(theme.Subtle, note)...)

	// The dirty pile against the line: the threshold and the cover.
	if line := m.rules.Heat.ExposureLine(w); line > 0 {
		dirty := w.Player.DirtyCash
		style := theme.Gold
		if dirty > line {
			style = theme.Warning
		}
		lines = append(lines, row("dirty", style.Render(fmt.Sprintf("%s of %s", cash(dirty), cash(line)))))
		if cover := m.rules.Heat.Cover(w); cover > 0 {
			lines = append(lines, row("", theme.Subtle.Render(fmt.Sprintf("%s + %s fronts", cash(line-cover), cash(cover)))))
		}
		if dirty > line {
			lines = append(lines, row("", theme.Warning.Render("over: heat every day")))
		} else {
			lines = append(lines, row("", theme.Subtle.Render("under: draws nothing")))
		}
	} else {
		lines = append(lines, row("dirty", theme.Subtle.Render("pile draws nothing")))
	}

	// The forecast: the cop's word, an estimate, or none.
	k := m.known()
	if f, ok := k.Fact(city.ID, game.FactResponse); ok {
		level, day, _ := k.Response(city.ID)
		lines = append(lines,
			row("estimate", theme.Warning.Render(fmt.Sprintf("%s from day %d", rungName(level), day))),
			row("", theme.Subtle.Render(fmt.Sprintf("a cop · %s sure", format.Pct(f.Now(w.Day), 0)))))
	} else {
		lines = append(lines, row("estimate", theme.Subtle.Render("no word from inside")))
	}
	return section{"POLICE · " + strings.ToUpper(city.Name), lines}
}
