package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The stage (#149) is the modal shown the morning a tier is entered,
// before the card and the report: it names the stage, says what just
// opened and what the next one takes, in the voice of progression.toml
// (the tier's blurb and text, its opens lines, its next line or, on the
// last tier, the file's closing line). Once per tier per run: what has
// been shown is Progression.Seen on the world, so a save on the modal
// reopens it, as a save on a card does, and a continued save past it
// does not. An old save with tiers reached and none seen is shown the
// highest once (World.StagePending; SeeStage marks the lower ones seen
// with it). Enter closes it and never ends the day.

// showStage opens the stage when a tier reached has not been seen, else
// falls through to showCard: the stage, then the card, then the report.
func (m *Model) showStage() {
	if n := m.w.StagePending(); n > 0 {
		m.stage = n
		m.modalScroll = 0
		m.mode = modeStage
		m.stageScene(n) // #157: the scene plays over the modal's rows
		return
	}
	m.showCard()
}

// keyStage closes the stage on enter or esc (space and q too, as the
// card's outcome takes them), marking it seen and saving (quietly: the
// morning's status, the tell that somebody is talking over the save
// line, stays), and opens what the morning has next: the card, else the
// report. Any other key scrolls, where the modal is short of room.
func (m *Model) keyStage(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc", " ", "q":
		m.w.SeeStage(m.stage)
		_ = game.Save(m.slot, m.w)
		m.showCard()
	default:
		m.scrollModal(key)
	}
	return m, nil
}

// viewStage is the modal: `STAGE 3 · TERRITORY`, the blurb, the text
// wrapped as one paragraph, OPENED with the file's lines on what the
// stage opens, and NEXT with what the next stage takes, or the closing
// line at the top of the ladder. The copy is the file's (TestStageFits
// holds every tier's to 80x24 whole). While the scene runs (#157) the
// title row and the body are its frame, in the same box.
func (m *Model) viewStage() string {
	if frame := m.stageFrame(); frame != nil {
		return m.modalTitled(frame[0], frame[1:], m.modalFooter())
	}
	return m.modal(m.stageTitle(m.stage), m.stageLines(m.stage), m.modalFooter())
}

// stageTitle is the modal's title for tier n: `STAGE 3 · TERRITORY`.
func (m *Model) stageTitle(n int) string {
	tier := m.cfg.Progression.Tier(n)
	if tier == nil {
		return fmt.Sprintf("STAGE %d", n)
	}
	return fmt.Sprintf("STAGE %d · %s", n, strings.ToUpper(tier.Name))
}

// stageLines is the modal's body for tier n.
func (m *Model) stageLines(n int) []string {
	tier := m.cfg.Progression.Tier(n)
	if tier == nil {
		return []string{"A stage with no name in the file."}
	}
	body := []string{theme.Gold.Render(tier.Blurb)}
	body = append(body, m.wrapLines(strings.Join(tier.Text, " "))...)
	body = append(body, "", theme.PanelTitle.Render("OPENED"))
	for _, o := range tier.Opens {
		body = append(body, "  "+o)
	}
	next := tier.Next
	if n == len(m.cfg.Progression.Tiers) && tier.Closing != "" {
		next = tier.Closing
	}
	body = append(body, "", theme.PanelTitle.Render("NEXT"))
	body = append(body, m.wrapLines(next)...)
	return body
}
