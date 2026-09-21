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

// A modal's rows are the pane's rows (#236): every labelled line in a
// dialog body is row(label, value), lowercase, the label at paneLabelW,
// and every dialog that spends opens its rows with the cash in hand, so
// the player reads the same row in the same place before every
// purchase.
func (m *Model) inHand() string { return inHand(m.w.Player.DirtyCash, m.w.Player.CleanCash) }

// inHand is the row for a dirty and a clean pile: `in hand     $452K
// dirty · $50K clean`.
func inHand(dirty, clean int) string {
	return row("in hand", cash(dirty)+" dirty · "+cash(clean)+" clean")
}

// confirm is the one confirmation's payload (#242): the fourteen
// yes-or-no questions (a new run, a slot deleted, a fire, an upgrade,
// asking around, a pay-off, a trip, a house dropped, a scout, a boost, a
// tip, a checkpoint, bail, a deed) are one modeConfirm, each askX
// setting what y does, what the modal shows and where no goes.
type confirm struct {
	verb string              // the footer's `y <verb>`
	view func(*Model) string // the rendered modal
	act  func(*Model)        // what yes does; it sets the mode it leaves in
	back mode                // where any other key goes: modeStart for the slot deletion, modePlay otherwise
}

// ask opens a confirmation over the play screen.
func (m *Model) ask(verb string, view func(*Model) string, act func(*Model)) {
	m.cfm = confirm{verb: verb, view: view, act: act, back: modePlay}
	m.mode = modeConfirm
}

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
	return m.modalTitled(theme.Title.Render(ansi.Truncate(strings.ToUpper(title), m.modalInner(), "…")), body, footer)
}

// modalTitled is the modal with its title row already rendered: a
// scene's animated title (#157's stage prints it in) where modal's is
// the caps in theme.Title. Everything else is modal's.
func (m *Model) modalTitled(title string, body []string, footer []binding) string {
	inner := m.modalInner()
	room := m.modalRoom()
	last := max(0, len(body)-room)
	m.modalScroll = max(0, min(m.modalScroll, last))
	var b strings.Builder
	b.WriteString(ansi.Truncate(title, inner, "…") + "\n\n")
	for _, l := range body[m.modalScroll:min(len(body), m.modalScroll+room)] {
		b.WriteString(ansi.Truncate(l, inner, "…") + "\n")
	}
	foot := strings.TrimPrefix(legend(footer), " ") // flush with the body
	more := ""
	if m.modalScroll > 0 {
		more += k("↑", "more")
	}
	if m.modalScroll < last {
		more += k("↓", "more")
	}
	// The scroll marks are never cut: a long footer (a number field's)
	// gives way to them, as the status bar carries the footer whole.
	if more != "" && lipgloss.Width(foot)+lipgloss.Width(more) > inner {
		foot = ansi.Truncate(foot, inner-lipgloss.Width(more), "…")
	}
	b.WriteString("\n" + ansi.Truncate(foot+more, inner, "…"))
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
	return strings.Split(theme.Plain.Width(m.modalInner()).Render(s), "\n")
}

// modalFooter is the footer of the modal open now, and what the status bar
// shows while it is: the key table's list for the mode (modeKeys), nil
// when no modal is open.
func (m *Model) modalFooter() []binding {
	if m.mode == modePlay {
		return nil
	}
	out := m.modeKeys(m.mode)
	for i := range out {
		out[i].label = m.labelOf(out[i]) // the confirmation's verb off its payload (#242)
	}
	return out
}
