package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The dilemma card (#15) is the modal shown before the morning report when
// the news sim dealt one overnight. It opens with no choice highlighted
// (#461): up/down or 1-3 pick one, enter takes the one picked and shows
// the outcome, a second enter opens the report. Enter never ends the
// day here, and a key typed ahead of the card (the enter that ran the
// fast-forward it stopped, a digit meant for a screen) never answers it.
// esc closes it unanswered (#500): the report opens, and the card is
// back when the report closes, still wanting an answer.

// noChoice is the card's cursor before a choice is picked.
const noChoice = -1

// showCard opens the pending card, if there is one, else the report.
func (m *Model) showCard() {
	m.cardCursor = noChoice
	m.cardDone = false
	m.cardLater = false
	if c := m.w.Dilemmas.Pending; c != nil {
		if m.mode != modeCard {
			m.cardScene(c) // #154: the card is dealt (once: a refused answer reopens it still)
		}
		m.mode = modeCard
		return
	}
	m.openReport()
}

// openReport opens the morning report, or the play screen when there
// is none: the one way a morning reaches the report (showCard with no
// card pending, the card's outcome closed), so the report's scene
// starts here and nowhere else. A scene plays when the report is the
// first thing the morning shows: the mode it opens from is play or the
// end-day confirmation, never the stage's or the card's, whose scenes
// outrank it. The bust's (#155) when the tick's events hold an
// enforcement past a patrol, a fast-forward's stopping morning
// included (the enforcement is what stopped it); else the incident's
// (#203) when the report opens on one, after a fast-forward too (the
// line is the report's own); else the morning's (#159), never after a
// fast-forward (fastStop). Each holds until the report closes (#203).
// r's reopen (keys.go) sets the mode itself: a reopen is no morning.
func (m *Model) openReport() {
	if m.w.Report == nil {
		m.mode = modePlay
		return
	}
	first := m.mode != modeStage && m.mode != modeCard
	m.mode = modeReport
	if !first {
		return
	}
	switch ev, ok := m.bust(); {
	case ok:
		m.bustScene(ev)
	case len(m.w.Report.Incident) > 0:
		m.incidentScene()
	case m.fastStop == "":
		m.morningScene()
	}
}

func (m *Model) keyCard(key string) (tea.Model, tea.Cmd) {
	c := m.w.Dilemmas.Pending
	if m.cardDone || c == nil {
		switch key {
		case "enter", "esc", " ", "q":
			m.cardDone = false
			m.openReport()
		default:
			m.scrollModal(key)
		}
		return m, nil
	}
	switch key {
	case "up", "k":
		if m.cardCursor == noChoice {
			m.cardCursor = len(c.Choices)
		}
		stepCursor(&m.cardCursor, -1, len(c.Choices))
	case "down", "j":
		stepCursor(&m.cardCursor, 1, len(c.Choices))
	case "esc":
		// Closed without an answer (#500): esc took no choice and
		// said nothing, and read as the do-nothing one. The card
		// steps aside for the report and is back when it closes.
		if cardCloses(m) {
			m.cardLater = true
			m.openReport()
		}
	case "enter":
		if m.cardCursor == noChoice {
			m.say("Pick a choice first: ↑↓ or " + cardDigits(len(c.Choices)) + ", then enter.")
			return m, nil
		}
		m.answerCard()
	default:
		// A digit picks, never decides (#461): the same digits switch
		// screens everywhere else.
		if i, ok := digit(key); ok && i < len(c.Choices) {
			m.cardCursor = i
		}
	}
	return m, nil
}

// cardCloses is a card open on its choices with a report behind it:
// where esc sets it aside, unanswered, until the report closes (#500).
func cardCloses(m *Model) bool {
	return m.modalStep() == 0 && !m.cardDone && m.w.Dilemmas.Pending != nil && m.w.Report != nil
}

// cardWaits is a card esc set aside, still pending: the report's close
// brings it back, and its jumps wait for the answer.
func (m *Model) cardWaits() bool {
	return m.cardLater && m.w != nil && m.w.Dilemmas.Pending != nil
}

// cardBack reopens the card esc set aside, as it was dealt, with no
// choice picked and no second scene.
func (m *Model) cardBack() {
	m.cardLater = false
	m.mode = modeCard // showCard deals the scene only to a card not already up
	m.showCard()
}

// cardDigits is the digits a card of n choices takes: 1-2 or 1-3.
func cardDigits(n int) string { return "1-" + string(rune('0'+n)) }

// cardPicked is a card open on its choices with one picked: where
// enter decides (#461).
func cardPicked(m *Model) bool {
	return m.modalStep() == 0 && !m.cardDone && m.w.Dilemmas.Pending != nil && m.cardCursor != noChoice
}

// answerCard takes the highlighted choice: the effects land at once, the
// outcome goes in the journal and stays on screen until the next enter.
func (m *Model) answerCard() {
	a, err := m.sess.Choose(m.cardCursor)
	if err != nil {
		m.refuse("Can't answer that: " + err.Error())
		m.showCard()
		return
	}
	m.outcome = a.Outcome
	m.cardDone = true
	m.modalScroll = 0
	m.save()
	m.refreshJournal()
}

// viewCard is the card, its prose wrapped to the modal's width (the one
// thing that wraps), then the choices, each with what it does under it
// (#358); after the answer, the outcome. The cursor keeps the whole of
// its choice in view, the label first.
func (m *Model) viewCard() string {
	if m.cardDone {
		return m.modal("WHAT HAPPENED", m.wrapLines(m.outcome), m.modalFooter())
	}
	c := m.w.Dilemmas.Pending
	if c == nil {
		return m.modal("DILEMMA", []string{"Nothing to decide."}, m.modalFooter())
	}
	if m.cardOnScene() {
		return m.viewCardScene(c)
	}
	m.cardCursor = max(noChoice, min(m.cardCursor, len(c.Choices)-1))
	body := append(m.wrapLines(c.Text), "")
	choices, at := m.cardChoices(c)
	if m.cardCursor != noChoice {
		end := len(choices)
		if m.cardCursor+1 < len(at) {
			end = at[m.cardCursor+1]
		}
		m.modalFollow(len(body) + end - 1)
		m.modalFollow(len(body) + at[m.cardCursor])
	}
	return m.modal(c.Title, append(body, choices...), m.modalFooter())
}

// cardChoices is the card's choices as the modal draws them: each label,
// the cursor's highlighted, and under it its chips (engine.ChoiceChips)
// in their tones, folded at " · " to the modal's width. at is the line
// each choice starts on.
func (m *Model) cardChoices(c *game.Card) (lines []string, at []int) {
	chips := m.cardChips(c)
	for i, ch := range c.Choices {
		at = append(at, len(lines))
		label := string(rune('1'+i)) + " " + truncate(ch.Label, m.modalInner()-6)
		if i == m.cardCursor {
			lines = append(lines, theme.Gold.Render("▸ ")+theme.Selected.Render(" "+label+" "))
		} else {
			lines = append(lines, "    "+label)
		}
		if i < len(chips) {
			lines = append(lines, foldChips(chips[i], m.modalInner()-6, "      ")...)
		}
	}
	return lines, at
}

// cardChips is what each choice on the pending card does, worked out
// once a card: ChoiceChips copies the world for every choice, and the
// world stands still while the card is up.
func (m *Model) cardChips(c *game.Card) [][]engine.Chip {
	if m.chipsFor != c {
		m.chipsFor, m.chips = c, engine.ChoiceChips(m.cfg, m.rules, m.w, c)
	}
	return m.chips
}

// chipStyle is a chip's tone in the theme's colours: a gain the good's,
// a cost the danger's, a line crossed the warning's, a note subtle.
func chipStyle(tone string) lipgloss.Style {
	switch tone {
	case engine.ToneGain:
		return theme.Good
	case engine.ToneCost:
		return theme.Bad
	case engine.ToneLine:
		return theme.Warning
	}
	return theme.Subtle
}

// foldChips joins chips with " · " into lines of at most width, each
// behind the indent; a chip wider than a line has one to itself and is
// cut there.
func foldChips(chips []engine.Chip, width int, indent string) []string {
	var out []string
	var line strings.Builder
	w := 0
	for _, c := range chips {
		cw := min(width, lipgloss.Width(c.Text))
		if w > 0 && w+3+cw > width {
			out = append(out, indent+line.String())
			line.Reset()
			w = 0
		}
		if w > 0 {
			line.WriteString(theme.Subtle.Render(" · "))
			w += 3
		}
		line.WriteString(chipStyle(c.Tone).Render(truncate(c.Text, width)))
		w += cw
	}
	if w > 0 {
		out = append(out, indent+line.String())
	}
	return out
}
