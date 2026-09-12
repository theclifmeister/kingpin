package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The ending's scene (#156): the morning a run ends, modeOver opens on
// a scene for the cause (anim.Indicted, Arrested, Broke), drawn inside
// the GAME OVER modal, and the summary follows on the same modal when
// it is done or a key ends it. The day has stepped and saved before
// the first frame (stepDay, then morning); the scene reads the world
// and the summary is read off the world after, never off the scene
// (viewOver's table is what it was, byte for byte, once the scene is
// down). A finished run continued from its save shows the summary
// alone: the scene has been seen.

// playOver puts the ending's scene up, with animation on: the hook in
// morning, where modeOver is set.
func (m *Model) playOver() {
	if !m.opts.Anim || m.w.Over == nil {
		return
	}
	m.play(&anim.Player{Scene: m.causeScene(m.w.Over.Cause), Accent: theme.Heat})
}

// causeScene is the switch: one scene a cause of game.Ending, on dice
// of the ending's own (anim.Seed on the run's seed, the day and "over",
// never the sims' stream), and the arrested scene for a cause without
// one, so #49's exits plug in a case each and play the default until
// they do.
func (m *Model) causeScene(cause string) anim.Scene {
	rng := anim.Seed(m.w.Seed, m.w.Day, "over")
	switch cause {
	case "indicted":
		return anim.Indicted(m.w.Heat.Evidence, m.overHeadline(), rng)
	case "broke":
		return anim.Broke(m.overFigures(), rng)
	}
	return anim.Arrested(rng)
}

// overFrame is the scene's frame at the modal's inner size: the body
// viewOver draws while the scene is up.
func (m *Model) overFrame() []string {
	return m.scene.Frame(m.modalInner(), m.modalRoom())
}

// overHeadline is the headline the indicted scene stamps across the
// file: the last the police made on the day the run ended (the heat
// source's: the arrest), else the journal's last, else the cause; cut
// to the modal's width, as the canvas cuts at both ends.
func (m *Model) overHeadline() string {
	j := m.w.Journal
	text := strings.ToUpper(m.w.Over.Cause)
	if n := len(j); n > 0 {
		text = j[n-1].Text
	}
	for i := len(j) - 1; i >= 0 && j[i].Day == m.w.Over.Day; i-- {
		if j[i].Source == "heat" {
			text = j[i].Text
			break
		}
	}
	return truncate(text, m.modalInner())
}

// overFigures is what the broke scene drops off the screen: the run's
// cash lines as the summary writes them, a label and its figure a row,
// and under them the crew's names, the roster on one row cut to the
// modal's width.
func (m *Model) overFigures() string {
	w := m.w
	rows := [][2]string{
		{"peak cash", cash(w.Stats.PeakCash)},
		{"total revenue", cash(w.Stats.TotalRevenue)},
		{"wages", cash(w.Stats.Wages)},
		{"skimmed", cash(w.Stats.Skimmed)},
		{"robbed", cash(w.Stats.Robbed)},
		{"washed", cash(w.Stats.Laundered)},
		{"seized", cash(w.Stats.Seized)},
		{"clean cash", cash(w.Player.CleanCash)},
	}
	width := 0
	for _, r := range rows {
		width = max(width, len(r[1]))
	}
	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-14s %*s", r[0], width, r[1]))
	}
	var names []string
	for _, c := range w.Crew.Members {
		names = append(names, c.Name)
	}
	if len(names) > 0 {
		lines = append(lines, " ", truncate(strings.Join(names, "  "), m.modalInner()))
	}
	return strings.Join(lines, "\n")
}
