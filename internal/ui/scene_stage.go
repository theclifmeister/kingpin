package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The stage's scene (#157): the morning a tier is entered, #149's modal
// opens with its title printed in and swept once by the beams' sheen
// and its body wiped in under it, anim.StageLength in all, then rests
// as the modal it is. It plays wherever showStage opens the stage: the
// morning, a fast-forward's stop on a new stage (the stopping morning,
// never mid-loop) and continueRun on a save made on the modal. The
// day has stepped and saved before the first frame. Any key ends it
// and is consumed (skip); the next enter is the modal's, so SeeStage
// fires on the close, never on the skip. With animation off, or under
// 80x24, showStage opens exactly the modal it did.

// stageScene starts the scene for tier n: the title in the tier's
// colour (theme.Money: progression.toml gives none), the modal's own
// title and body lines, on anim.Seed(seed, day, "stage").
func (m *Model) stageScene(n int) {
	// Under 80x24 the modal opens as it did; so does a size not yet
	// known (-slot continues a save before the first resize, and the
	// body is wrapped to the modal's width).
	if !m.opts.Anim || m.width < 80 || m.height < 24 {
		return
	}
	body := m.stageLines(n)
	plain := make([]string, len(body))
	for i, l := range body {
		plain[i] = ansi.Strip(l)
	}
	m.play(&anim.Player{
		Scene:  anim.Stage(strings.ToUpper(m.stageTitle(n)), plain, theme.Money, anim.Seed(m.w.Seed, m.w.Day, "stage")),
		Accent: theme.Money,
	})
}

// stageFrame is the scene's frame while it runs, the title row and
// then the body's rows at the modal's inner width, or nil once it is
// over (or never was), when viewStage draws the modal itself.
func (m *Model) stageFrame() []string {
	if m.scene == nil || m.scene.Idle || m.mode != modeStage {
		return nil
	}
	return m.scene.Frame(m.modalInner(), 1+len(m.stageLines(m.stage)))
}
