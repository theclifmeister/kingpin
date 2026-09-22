// The morning report (modeReport): the night's day report, read before
// the day starts (#275: out of model.go).

package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// viewReport is the report: the modal titled `MORNING REPORT · DAY 42`
// over the day's sections. While the morning's scene runs (#159) the
// title row is its frame's, in the same box, the body as it is; while
// the bust's runs (#155) the title row and a line at the head of the
// body are its; while the incident's runs (#203) the INCIDENT
// section's first line is its. Each holds until the report closes
// (#203), so "runs" is the whole of the report's time up.
func (m *Model) viewReport() string {
	if m.w.Report == nil {
		return m.modal("MORNING REPORT", []string{"Nothing happened yet."}, m.modalFooter())
	}
	if frame := m.morningFrame(); frame != nil {
		return m.modalTitled(frame[1], m.reportLines(), m.modalFooter())
	}
	if frame := m.bustFrame(); frame != nil {
		// The bust's line heads the body while it plays (#155), its own
		// section over the report's.
		return m.modalTitled(frame[0], append([]string{frame[1], ""}, m.reportLines()...), m.modalFooter())
	}
	if frame := m.incidentFrame(); frame != nil {
		body := m.reportLines()
		if i := incidentRow(body, m.w.Report); i >= 0 {
			body[i] = frame[0]
		}
		return m.modal(m.reportTitle(), body, m.modalFooter())
	}
	return m.modal(m.reportTitle(), m.reportLines(), m.modalFooter())
}

// reportTitle is the report's title row: `MORNING REPORT · DAY 42`.
func (m *Model) reportTitle() string {
	return fmt.Sprintf("MORNING REPORT · DAY %d", m.w.Report.Day)
}

// reportLines is the report's body: a fast-forward's stop line first
// (#116), then the day's sections, each a heading in its sim's colour
// over its lines.
func (m *Model) reportLines() []string {
	r := m.w.Report
	var body []string // the modal cuts a long line to its width, never wraps it
	if stop := m.stopLine(); stop != "" {
		// A fast-forward's report opens with why it stopped (#116).
		body = append(body, theme.Warning.Render(stop), "")
	}
	section := func(title string, ls []string, style lipgloss.Style) {
		if len(ls) == 0 {
			return
		}
		body = append(body, style.Bold(true).Render(title))
		for _, l := range ls {
			body = append(body, "  "+l)
		}
		body = append(body, "")
	}
	section("INCIDENT", r.Incident, theme.Fg(theme.World)) // the world's incident this morning (#44): first, the day is about it
	section("TIER", r.Tier, theme.Warning)                 // the tier entered this morning (#147)
	section("UNLOCKED", r.Unlocked, theme.Gold)            // a gate crossed (#148): next, it is what the morning is about
	section("PRICES", r.Prices, theme.Good)
	section("SALES", r.Sales, theme.Gold)
	section("SHIPMENTS", r.Shipments, theme.RoadText)
	section("HEAT", r.Heat, theme.Bad)
	section("LAW", r.Law, lawReportStyle)
	section("INTEL", r.Intel, theme.IntelText) // what was learnt tonight (#45)
	section("CREW", r.Crew, theme.CrewText)
	section("TERRITORY", r.Territory, theme.RivalText)
	section("MONEY", append(r.Money, fmt.Sprintf("Cash %s %s %s", cash(r.CashBefore), format.Arrow, cash(r.CashAfter))), theme.Gold)
	section("UPGRADES", r.Upgrades, theme.Gold)
	section("NEWS", r.News, theme.Subtle)
	for len(body) > 0 && body[len(body)-1] == "" {
		body = body[:len(body)-1]
	}
	return body
}
