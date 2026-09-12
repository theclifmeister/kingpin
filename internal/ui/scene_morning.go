package ui

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The morning's scene (#159): the day rolls over. When the report
// opens on a plain morning (n or enter, the report the first thing the
// morning shows: no stage, no card, no ending, whose scenes replace it
// rather than stack), the title bar's day slides from yesterday's to
// today's and the report's title row wipes in, anim.MorningLength in
// all; the frame is the report's modal with its body as it is. The
// hook is openReport (card.go), the one way a morning reaches the
// report. Never after F: a fast-forward's stopping report opens still,
// with its `Stopped after 3 days: …` line, and no scene. Any key skips
// it and is consumed; enter then closes the report and never ends the
// day. KINGPIN_NO_MORNING_ANIM turns this one scene off on its own
// (Options.MorningAnim), a player who likes the card's reveal not
// wanting one every morning; with it off, with animation off or under
// 80x24 the report is today's, byte for byte.

// morningScene starts the scene: animation and the morning's on, the
// terminal at least 80x24, on anim.Seed(seed, day, "morning"), which
// the slide and the wipe leave unthrown. reportScene names it, so the
// views know whose frame the report's scene is.
func (m *Model) morningScene() {
	if !m.opts.Anim || !m.opts.MorningAnim || !m.titleFits() || m.w.Report == nil {
		return
	}
	m.reportScene = reportMorning
	m.play(&anim.Player{
		Scene:  anim.Morning(m.w.Day-1, m.w.Day, m.reportTitle(), anim.Seed(m.w.Seed, m.w.Day, "morning")),
		Accent: theme.Money,
	})
}

// reportScene is which scene the report opened on, while m.scene is up
// in modeReport: the morning's (#159) or the bust's (#155).
type reportSceneKind int

const (
	reportNone    reportSceneKind = iota
	reportMorning                 // the day rolls over (#159)
	reportBust                    // a sting, a raid or an arrest hits (#155)
)

// onReportScene reports whether the report's scene of the kind is up:
// one is, the mode is the report's and it is the kind's.
func (m *Model) onReportScene(kind reportSceneKind) bool {
	return m.scene != nil && !m.scene.Idle && m.mode == modeReport && m.reportScene == kind
}

// morningFrame is the scene's frame while it runs: row 0 the day
// counter, row 1 the report's title row at the modal's inner width;
// nil once it is over (or never was).
func (m *Model) morningFrame() []string {
	if !m.onReportScene(reportMorning) {
		return nil
	}
	return m.scene.Frame(m.modalInner(), 2)
}

// dayLabel is the title bar's `Day 42`: while the morning's scene runs
// the number is the counter's row, yesterday's digits rolling out and
// today's in, at today's width so the bar holds still.
func (m *Model) dayLabel() string {
	if frame := m.morningFrame(); frame != nil {
		return "Day " + fit(frame[0], len(fmt.Sprint(m.w.Day)))
	}
	return fmt.Sprintf("Day %d", m.w.Day)
}
