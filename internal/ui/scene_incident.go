package ui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The incident's scene (#203; the INCIDENT section of #44): the
// morning the report opens on an incident and no bust, the section's
// first line prints in on its own row, anim.IncidentLength, and
// holds: a slow pass of beams over it every anim.HoldRest until the
// report closes, so the weather reads too. The hook is openReport
// (card.go), between the bust's and the morning's: a bust outranks it
// (the alarm over the weather), it outranks the morning's, and like
// them it yields to a card, a stage or an ending. It plays on a
// fast-forward's stopping morning too, under the stop line: the line
// is the report's own and nothing in it counts days. Any key skips it
// to the resolved line, consumed, and the hold plays on; enter then
// closes the report and never ends the day.

// incidentScene starts the scene for the morning's incident: animation
// on, the terminal at least 80x24, a line to print, on
// anim.Seed(seed, day, "incident"), held until the report closes. The
// line is cut to the modal's row as the report cuts it, so the scene
// resolves to the row it replaces.
func (m *Model) incidentScene() {
	if !m.opts.Anim || !m.titleFits() || m.w.Report == nil || len(m.w.Report.Incident) == 0 {
		return
	}
	line := ansi.Truncate(m.w.Report.Incident[0], m.modalInner()-2, "…")
	m.reportScene = reportIncident
	m.play(&anim.Player{
		Scene:  anim.Incident(line, anim.Seed(m.w.Seed, m.w.Day, "incident")),
		Accent: theme.World,
		Hold:   true,
	})
}

// incidentFrame is the scene's frame while it runs or holds: one row,
// the incident's line at the modal's inner width; nil once the report
// has closed (or it never was).
func (m *Model) incidentFrame() []string {
	if !m.onReportScene(reportIncident) {
		return nil
	}
	return m.scene.Frame(m.modalInner(), 1)
}

// incidentRow is the row of the report's body the scene draws on: the
// INCIDENT section's first line, as reportLines writes it, wherever
// the section sits (a fast-forward's stop line comes first); -1 for
// none.
func incidentRow(body []string, r *game.DayReport) int {
	if len(r.Incident) == 0 {
		return -1
	}
	for i, l := range body {
		if l == "  "+r.Incident[0] {
			return i
		}
	}
	return -1
}
