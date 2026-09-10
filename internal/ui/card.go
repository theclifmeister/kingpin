package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		m.status = "Can't answer that: " + err.Error()
		m.showCard()
		return
	}
	m.outcome = a.Outcome
	m.cardDone = true
	m.save()
	m.refreshJournal()
}

func (m *Model) viewCard() string {
	width := max(30, min(72, m.width-10))
	wrap := lipgloss.NewStyle().Width(width)
	if m.cardDone {
		return m.modal("WHAT HAPPENED", wrap.Render(m.outcome)+"\n\n"+theme.Key.Render("enter")+" the morning report")
	}
	c := m.w.Dilemmas.Pending
	if c == nil {
		return m.modal("DILEMMA", "Nothing to decide.")
	}
	m.cardCursor = max(0, min(m.cardCursor, len(c.Choices)-1))
	var b strings.Builder
	b.WriteString(wrap.Render(c.Text) + "\n\n")
	for i, ch := range c.Choices {
		label := string(rune('1'+i)) + " " + truncate(ch.Label, width-6)
		if i == m.cardCursor {
			b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(" "+label+" ") + "\n")
		} else {
			b.WriteString("    " + label + "\n")
		}
	}
	b.WriteString("\n" + theme.Subtle.Render("Your call. ") + theme.Key.Render("enter") + " decide")
	return m.modal(strings.ToUpper(c.Title), strings.TrimRight(b.String(), "\n"))
}
