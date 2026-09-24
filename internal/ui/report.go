// The morning report (modeReport): the night's day report, read before
// the day starts (#275: out of model.go).

package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/engine"
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
// (#116), then TODAY, the night's lead (#354), then the day's sections
// in engine.ReportSections' order, each a heading in its sim's colour
// over its lines.
func (m *Model) reportLines() []string {
	r := m.w.Report
	var body []string // the modal cuts a long line to its width, never wraps it
	if stop := m.stopLine(); stop != "" {
		// A fast-forward's report opens with why it stopped (#116).
		body = append(body, theme.Warning.Render(stop), "")
	}
	if len(r.Lead) > 0 {
		// The biggest changes of the night (#354), each one key from
		// what answers it: 1, 2, 3.
		body = append(body, theme.Title.Render("TODAY"))
		for i, l := range r.Lead {
			body = append(body, "  "+theme.Key.Render(fmt.Sprint(i+1))+" "+theme.Bold.Render(l.Text))
		}
		body = append(body, "")
	}
	for _, sec := range engine.ReportSections(r) {
		ls := sec.Lines
		switch sec.ID {
		case "crew":
			if line := m.crewTrouble(); line != "" {
				// The crew trouble this morning (#345), the alerts' counts.
				ls = append(append([]string(nil), ls...), theme.Warning.Render(line))
			}
		case "money":
			ls = m.moneyLines(r)
		}
		if len(ls) == 0 {
			continue
		}
		body = append(body, reportStyles[sec.ID].Bold(true).Render(sec.Title))
		for _, l := range ls {
			body = append(body, "  "+l)
		}
		body = append(body, "")
	}
	for len(body) > 0 && body[len(body)-1] == "" {
		body = body[:len(body)-1]
	}
	return body
}

// reportStyles are the report's sections' colours, by
// engine.ReportSection id: each its sim's.
var reportStyles = map[string]lipgloss.Style{
	"incident":  theme.Fg(theme.World),
	"tier":      theme.Warning,
	"unlocked":  theme.Gold,
	"prices":    theme.Good,
	"sales":     theme.Gold,
	"shipments": theme.RoadText,
	"heat":      theme.Bad,
	"law":       lawReportStyle,
	"intel":     theme.IntelText,
	"crew":      theme.CrewText,
	"territory": theme.RivalText,
	"money":     theme.Gold,
	"upgrades":  theme.Gold,
	"news":      theme.Subtle,
}

// hasLead is the report open on a night with a lead: its 1, 2 and 3
// open the lines (#354).
func hasLead(m *Model) bool { return m.w != nil && m.w.Report != nil && len(m.w.Report.Lead) > 0 }

// openLead is the report's 1, 2 or 3: the lead line's act, the way the
// dashboard's o opens an alert's (#352).
func (m *Model) openLead(key string) {
	if !hasLead(m) {
		return
	}
	i := int(key[0] - '1')
	if lead := m.w.Report.Lead; i >= 0 && i < len(lead) {
		m.openAlert(engine.LeadAlert(lead[i]))
	}
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
