package ui

import (
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The bust's scene (#155): a sting, a raid or an arrest hits. The
// morning the report opens with an Enforcement past a patrol in the
// tick's events (Model.flash collects them off the bus), the report's
// title strobes red twice, the level (STING, RAID, ARRESTED) glitches
// in on a bad tape at the top of the report and the stock and cash
// lost burn along their line, anim.BustLength (1.2 s) in all, and the
// report is what it resolves to. It replaces the morning's scene
// (#159) rather than stacking on it, in openReport (card.go), and like
// it yields to a card, a stage or an ending: an arrest that ends the
// run is modeOver's morning, the ending's scene (#156), and plays none
// of its own. One scene a morning whatever fired: an arrest outranks a
// raid outranks a sting. A raid that went to the stash (Enforcement.
// Stash: somebody talked) glitches longer, with one frame in the
// rival's purple. Fast-forward stops on an Enforcement past a patrol
// already (stopEvent), and the stopping morning plays it, the stop
// line and all; nothing plays mid-loop. Any key skips it and is
// consumed; enter then closes the report and never ends the day. The
// heat gauge is not animated: the dashboard is play mode.

// bustRank orders the levels a bust's scene plays: the highest wins
// the morning.
var bustRank = map[string]int{"sting": 1, "raid": 2, "arrest": 3}

// bust is the enforcement the morning's scene is for: the highest
// level past a patrol among the tick's events, the first of a tie, or
// none.
func (m *Model) bust() (events.Enforcement, bool) {
	var top events.Enforcement
	found := false
	for _, ev := range m.flash {
		if bustRank[ev.Level] > bustRank[top.Level] || (bustRank[ev.Level] > 0 && !found) {
			top, found = ev, true
		}
	}
	return top, found
}

// bustLevel is the word the tape glitches in: the level in caps, an
// arrest as the report writes it.
func bustLevel(level string) string {
	if level == "arrest" {
		return "ARRESTED"
	}
	return strings.ToUpper(level)
}

// bustLoss is the rest of the report's line for the level, after the
// word: `: lost 40 Weed and $2,000 in Eastside`, ` at Ma's: lost …`
// for a house; "" when the report has no such line (an arrest's line
// is the word alone).
func (m *Model) bustLoss(level string) string {
	for _, l := range m.w.Report.Heat {
		if strings.HasPrefix(l, level) {
			return strings.TrimSuffix(strings.TrimPrefix(l, level), ".")
		}
	}
	return ""
}

// bustScene starts the scene for the morning's bust: animation on, the
// terminal at least 80x24, on anim.Seed(seed, day, "bust").
func (m *Model) bustScene(ev events.Enforcement) {
	if !m.opts.Anim || !m.titleFits() || m.w.Report == nil {
		return
	}
	level := bustLevel(ev.Level)
	m.reportScene = reportBust
	m.play(&anim.Player{
		Scene:  anim.Bust(m.reportTitle(), level, m.bustLoss(level), ev.Stash, anim.Seed(m.w.Seed, m.w.Day, "bust")),
		Accent: theme.Heat,
	})
}

// bustFrame is the scene's frame while it runs: row 0 the title row,
// row 1 the bust's line, at the modal's inner width; nil once it is
// over (or never was).
func (m *Model) bustFrame() []string {
	if !m.onReportScene(reportBust) {
		return nil
	}
	return m.scene.Frame(m.modalInner(), 2)
}
