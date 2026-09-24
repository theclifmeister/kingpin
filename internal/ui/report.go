// The morning report (modeReport): the night's day report, read before
// the day starts (#275: out of model.go).

package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
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
	section("PLAN", m.planReport(), theme.Gold)            // the plan pinned (#347): its bar and next step
	section("UNLOCKED", r.Unlocked, theme.Gold)            // a gate crossed (#148): next, it is what the morning is about
	section("PRICES", r.Prices, theme.Good)
	section("SALES", r.Sales, theme.Gold)
	section("SHIPMENTS", r.Shipments, theme.RoadText)
	section("HEAT", r.Heat, theme.Bad)
	section("LAW", r.Law, lawReportStyle)
	section("INTEL", r.Intel, theme.IntelText) // what was learnt tonight (#45)
	crew := r.Crew
	if line := m.crewTrouble(); line != "" {
		// The crew trouble this morning (#345), the alerts' counts.
		crew = append(append([]string(nil), crew...), theme.Warning.Render(line))
	}
	section("CREW", crew, theme.CrewText)
	section("TERRITORY", r.Territory, theme.RivalText)
	section("MONEY", m.moneyLines(r), theme.Gold)
	section("UPGRADES", r.Upgrades, theme.Gold)
	section("NEWS", r.News, theme.Subtle)
	for len(body) > 0 && body[len(body)-1] == "" {
		body = body[:len(body)-1]
	}
	return body
}

// flowCols are the waterfall's columns (#351): the category, then the
// two piles and both together.
var flowCols = []col{{"flow", kText, 0}, {"dirty", kMoney, 0}, {"clean", kMoney, 0}, {"total", kMoney, 0}}

// moneyLines is the report's MONEY section (#351): the night's cash
// flow first, the opening, a row a category that moved (signed, dirty
// and clean apart, one past [flow] big_share of the opening in red or
// green) and the closing, which is the opening and the rows summed pile
// by pile; then the itemised lines naming each cause, then the cash
// before and after. A night that moved nothing is the lines alone.
func (m *Model) moneyLines(r *game.DayReport) []string {
	cashLine := fmt.Sprintf("Cash %s %s %s", cash(r.CashBefore), format.Arrow, cash(r.CashAfter))
	f := r.Flow
	var rows [][]any
	for _, l := range f.Lines {
		if l.Dirty == 0 && l.Clean == 0 {
			continue
		}
		row := []any{game.FlowLabel(l.Cat), flowCell(l.Dirty), flowCell(l.Clean), signed{l.Total()}}
		if f.Big(l, m.cfg.Headlines.Flow.BigShare) {
			st := theme.Good
			if l.Total() < 0 {
				st = theme.Bad
			}
			for i := range row {
				row[i] = styled{st, row[i]}
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return append(append([]string(nil), r.Money...), cashLine)
	}
	ends := func(label string, p game.Pools) []any {
		return []any{styled{theme.Gold, label}, styled{theme.Gold, p.Dirty}, styled{theme.Gold, p.Clean}, styled{theme.Gold, p.Total()}}
	}
	rows = append([][]any{ends("Opening", f.Opening)}, rows...)
	rows = append(rows, ends("Closing", f.Closing))
	ls := table(flowCols, rows, -1, 0)
	if len(r.Money) > 0 {
		ls = append(ls, "")
	}
	return append(append(ls, r.Money...), cashLine)
}

// flowCell is one pile of a flow line: signed, and empty where the
// pile did not move.
func flowCell(n int) any {
	if n == 0 {
		return nil
	}
	return signed{n}
}
