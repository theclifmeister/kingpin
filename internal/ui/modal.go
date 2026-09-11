package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The one modal (#81, the epic's "Modal" convention): every dialog,
// picker, confirmation, the card, the report and help are the same box.
// It is min(width-4, 76) wide, border included, whatever it holds and
// whichever step of a dialog it is on, so the title never moves; it sits
// on body row 2 (one blank row under the title bar), centred; a double
// border in gold; the title in caps, a blank, the body, a blank and a
// footer line of k() pairs. Body lines are cut to the width, never
// wrapped (the card wraps its prose before it gets here). A body taller
// than the room scrolls, and the footer says so.

// binding is one key and what it does, the way k() draws it. #80's key
// table is the same shape; until it lands this is the modal's own.
type binding struct{ key, label string }

// modalMax is the widest a modal gets. Under it the modal is the terminal
// less a two-column margin each side.
const modalMax = 76

// modalTop is the body row the modal's top border sits on.
const modalTop = 1

// modalWidth is the width of every modal, border included.
func (m *Model) modalWidth() int { return max(12, min(m.width-4, modalMax)) }

// modalInner is what a body line has between the border's padding.
func (m *Model) modalInner() int { return m.modalWidth() - 4 }

// modalRoom is how many body lines fit: the body less the row above the
// box, its two border rows, the title and its blank, and the blank and
// the footer.
func (m *Model) modalRoom() int { return max(1, m.bodyHeight()-modalTop-6) }

// modal draws the box. The body is shown from m.modalScroll, clamped so
// the last line is never scrolled past the room.
func (m *Model) modal(title string, body []string, footer []binding) string {
	inner := m.modalInner()
	room := m.modalRoom()
	last := max(0, len(body)-room)
	m.modalScroll = max(0, min(m.modalScroll, last))
	var b strings.Builder
	b.WriteString(theme.Title.Render(ansi.Truncate(strings.ToUpper(title), inner, "…")) + "\n\n")
	for _, l := range body[m.modalScroll:min(len(body), m.modalScroll+room)] {
		b.WriteString(ansi.Truncate(l, inner, "…") + "\n")
	}
	foot := strings.TrimPrefix(legend(footer), " ") // flush with the body
	if m.modalScroll > 0 {
		foot += k("↑", "more")
	}
	if m.modalScroll < last {
		foot += k("↓", "more")
	}
	b.WriteString("\n" + ansi.Truncate(foot, inner, "…"))
	box := theme.Modal.Width(m.modalWidth() - 2).Render(b.String())
	return strings.Repeat("\n", modalTop) + lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box)
}

// legend renders bindings the way the status bar does.
func legend(bs []binding) string {
	var b strings.Builder
	for _, x := range bs {
		b.WriteString(k(x.key, x.label))
	}
	return b.String()
}

// modalFollow scrolls the body so the line a picker's cursor is on shows.
func (m *Model) modalFollow(line int) {
	room := m.modalRoom()
	if line < m.modalScroll {
		m.modalScroll = line
	}
	if line >= m.modalScroll+room {
		m.modalScroll = line - room + 1
	}
}

// scrollModal moves a modal that has no cursor of its own; it reports
// whether the key was one of the scroll keys. The view clamps the offset.
func (m *Model) scrollModal(key string) bool {
	switch key {
	case "up", "k":
		m.modalScroll--
	case "down", "j":
		m.modalScroll++
	case "pgup":
		m.modalScroll -= m.modalRoom()
	case "pgdown":
		m.modalScroll += m.modalRoom()
	default:
		return false
	}
	m.modalScroll = max(0, m.modalScroll)
	return true
}

// wrapLines wraps prose to the modal's width, the one thing that wraps.
func (m *Model) wrapLines(s string) []string {
	return strings.Split(lipgloss.NewStyle().Width(m.modalInner()).Render(s), "\n")
}

// modalFooter is the footer of the modal open now, and what the status bar
// shows while it is: nil when no modal is open. Confirmations are
// y <verb> / esc back (any other key still declines); pickers and dialogs
// enter <verb> / esc back (q still closes, silently); the end of the day
// is the one that also takes enter and says so; the report, the card's
// outcome and help close on enter or esc.
func (m *Model) modalFooter() []binding {
	pick := binding{"↑↓", "pick"}
	next := binding{"enter", "next"}
	back := binding{"esc", "back"}
	closes := []binding{{"enter esc", "close"}}
	switch m.mode {
	case modeStart:
		return []binding{pick, {"enter", "select"}, {"c", "continue"}, {"n", "new run"}, {"q", "quit"}}
	case modeReport, modeHelp:
		return closes
	case modeOver:
		return []binding{{"enter", "new run"}, {"q", "quit"}}
	case modeBuy:
		if m.dlg.step == 0 {
			return []binding{pick, next, back}
		}
		return []binding{{"enter", "buy"}, back}
	case modeSell:
		switch m.dlg.step {
		case 0:
			return []binding{pick, next, back}
		case 1:
			return []binding{next, back}
		}
		return []binding{{"←→", "dial"}, {"1-3", "dial"}, {"enter", "sell"}, back}
	case modeConfirmNew:
		return []binding{{"y", "new run"}, back}
	case modeConfirmFire:
		return []binding{{"y", "fire"}, back}
	case modeConfirmEnd:
		return []binding{{"y enter", "end day"}, back}
	case modeConfirmUpgrade:
		return []binding{{"y", "buy"}, back}
	case modeConfirmInvestigate:
		return []binding{{"y", "ask"}, back}
	case modeConfirmPayOff:
		return []binding{{"y", "pay"}, back}
	case modeConfirmTravel:
		return []binding{{"y", "go"}, back}
	case modeCard:
		if m.cardDone {
			return closes
		}
		return []binding{pick, {"1-3", "choose"}, {"enter", "decide"}}
	case modePost:
		return []binding{pick, {"enter", "post"}, back}
	case modeStrike:
		return []binding{pick, {"enter", "send"}, back}
	case modeFront:
		return []binding{pick, {"enter", "buy"}, back}
	case modeTarget:
		if m.tgt.step == 0 {
			return []binding{pick, next, back}
		}
		return []binding{{"enter", "set"}, back}
	case modePropose:
		if m.proposeStep == 0 {
			return []binding{pick, next, back}
		}
		return []binding{pick, {"enter", "propose"}, back}
	case modeAssign:
		return []binding{pick, {"enter", "assign"}, back}
	case modeFund:
		return []binding{{"←→", "city"}, {"enter", "give"}, back}
	}
	return nil
}
