package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The dilemma card (#15) is the modal shown before the morning report when
// the news sim dealt one overnight. Up/down or 1-3 pick a choice, enter
// takes it and shows the outcome, a second enter opens the report. Enter
// never ends the day here.

// showCard opens the pending card, if there is one, else the report.
func (m *Model) showCard() {
	m.cardCursor = 0
	m.cardDone = false
	if m.w.Dilemmas.Pending != nil {
		m.mode = modeCard
		return
	}
	if m.w.Report != nil {
		m.mode = modeReport
		return
	}
	m.mode = modePlay
}

func (m *Model) keyCard(key string) (tea.Model, tea.Cmd) {
	c := m.w.Dilemmas.Pending
	if m.cardDone || c == nil {
		switch key {
		case "enter", "esc", " ", "q":
			m.cardDone = false
			if m.w.Report != nil {
				m.mode = modeReport
			} else {
				m.mode = modePlay
			}
		default:
			m.scrollModal(key)
		}
		return m, nil
	}
	switch key {
	case "up", "k":
		if m.cardCursor > 0 {
			m.cardCursor--
		}
	case "down", "j":
		if m.cardCursor < len(c.Choices)-1 {
			m.cardCursor++
		}
	case "enter":
		m.answerCard()
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if i := int(key[0] - '1'); i < len(c.Choices) {
				m.cardCursor = i
				m.answerCard()
			}
		}
	}
	return m, nil
}

// answerCard takes the highlighted choice: the effects land at once, the
// outcome goes in the journal and stays on screen until the next enter.
func (m *Model) answerCard() {
	a, err := m.w.Choose(m.cardCursor)
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
// thing that wraps), then the choices; after the answer, the outcome.
func (m *Model) viewCard() string {
	if m.cardDone {
		return m.modal("WHAT HAPPENED", m.wrapLines(m.outcome), m.modalFooter())
	}
	c := m.w.Dilemmas.Pending
	if c == nil {
		return m.modal("DILEMMA", []string{"Nothing to decide."}, m.modalFooter())
	}
	m.cardCursor = max(0, min(m.cardCursor, len(c.Choices)-1))
	body := append(m.wrapLines(c.Text), "")
	for i, ch := range c.Choices {
		label := string(rune('1'+i)) + " " + truncate(ch.Label, m.modalInner()-6)
		if i == m.cardCursor {
			m.modalFollow(len(body))
			body = append(body, theme.Gold.Render("▸ ")+theme.Selected.Render(" "+label+" "))
		} else {
			body = append(body, "    "+label)
		}
	}
	return m.modal(c.Title, body, m.modalFooter())
}
