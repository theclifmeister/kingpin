package ui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
// footer line of k() pairs. A body line wider than the box wraps under
// itself (#463, wrapLine), never cut, so a report's line, a dialog's
// terms and a note are read to their end. A body taller than the room
// scrolls, and the footer says so.

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

// stepper is the state every dialog with pages embeds (#243): the page
// it is on and the error under its body. back is the one back rule: a
// step back blurs and clears the number field the step leaves and
// focuses the one it lands on, keeping what the earlier steps chose
// (the product, the line, the kind, the city); fieldAt is the dialog's
// number field on a step, nil off one (noField for a dialog with none).
type stepper struct {
	step int
	err  string
}

func (s *stepper) back(fieldAt func(int) *numberField) tea.Cmd {
	if s.step == 0 {
		return nil
	}
	if f := fieldAt(s.step); f != nil {
		f.SetValue("")
		f.Blur()
	}
	s.step--
	if f := fieldAt(s.step); f != nil {
		return f.Focus()
	}
	return nil
}

func (s *stepper) page() int { return s.step }

// noField is fieldAt for a dialog with no number field.
func noField(int) *numberField { return nil }

// closes reports whether a key closes a modal whole: esc, and q, from
// any step (#243).
func closes(key string) bool { return key == "esc" || key == "q" }

// paged is what a mode's state tells the key table (#243): the page it
// is on and the number field on it, nil off one. The footer's generic
// rows follow from it through openPaged: the field's four while there
// is a field, `⇧tab back` past the first page and `esc close`.
type paged interface {
	page() int
	field() *numberField
}

// picker is the one-page picker's state (#243): the cursor the arrows
// and the digits walk, and the post picker's role. One is open at a
// time, so the six (post, strike, undercut, assign, guard, driver) share
// it; each askX sets the cursor as it opens.
type picker struct {
	cursor int
	role   string // the post picker's: runner or enforcer
	budget int    // the captain picker's (#346): the index of the budget in crew.toml [captain] budgets
}

func (p *picker) page() int           { return 0 }
func (p *picker) field() *numberField { return nil }

// pickerKey is every one-page picker's keys (#243): ↑↓ and j k move the
// cursor within rows, 1-9 move it to that row (#500: a digit never
// acts), enter commits, esc and q close.
func (m *Model) pickerKey(key string, rows int, pick func()) {
	switch key {
	case "esc", "q":
		m.mode = modePlay
	case "up", "k":
		stepCursor(&m.pick.cursor, -1, rows)
	case "down", "j":
		stepCursor(&m.pick.cursor, 1, rows)
	case "enter":
		pick()
	default:
		if i, ok := digit(key); ok && i < rows {
			m.pick.cursor = i // the row; enter acts (#500)
		}
	}
}

// pickerModal is every picker's modal (#243): the head lines, the table
// with the cursor's row kept in view, then a blank and the notes where
// there are any.
func (m *Model) pickerModal(title string, head []string, cols []col, cells [][]any, cursor int, notes ...string) string {
	m.modalFollow(len(head) + 1 + cursor) // under the header
	body := append(append([]string{}, head...), table(cols, cells, cursor, m.modalInner())...)
	if len(notes) > 0 {
		body = append(append(body, ""), notes...)
	}
	return m.modal(title, body, m.modalFooter())
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
	// Every line wider than the box wraps under itself (#463); the
	// lines a picker asked to keep in view (modalFollow) are found in
	// the wrapped body.
	at := make([]int, len(body)+1) // body line i's first wrapped line
	var lines []string
	for i, l := range body {
		at[i] = len(lines)
		lines = append(lines, wrapLine(l, inner)...)
	}
	at[len(body)] = len(lines)
	for _, f := range m.follow {
		if f < 0 || f >= len(body) {
			continue
		}
		if at[f] < m.modalScroll {
			m.modalScroll = at[f]
		}
		if end := at[f+1] - 1; end >= m.modalScroll+room {
			m.modalScroll = end - room + 1
		}
	}
	m.follow = nil
	body = lines
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

// modalFollow scrolls the body so the line a picker's cursor is on
// shows, all of it where it wraps: the line is the body's, the index
// the view passes, and the next modal drawn finds it among the wrapped
// lines (#463), in the order asked.
func (m *Model) modalFollow(line int) { m.follow = append(m.follow, line) }

// gluedPointer is a pointer to a screen, `on the market screen (2)`, which
// wrapLine keeps on one row.
var gluedPointer = regexp.MustCompile(`on the \w+ screen \(\d\)`)

// wrapLine is a body line cut into lines of at most width cells at its
// spaces (#463), the one wrap of the modal: a word longer than the
// width is cut through, the styles carry over, and the lines after the
// first hang under the value (the end of the first run of two spaces,
// a row's label or a table's first cell) or two cells in from the
// line's own indent. A line that fits is itself.
func wrapLine(l string, width int) []string {
	if width <= 0 || lipgloss.Width(l) <= width {
		return []string{l}
	}
	plain := []rune(ansi.Strip(l))
	cols := make([]int, len(plain)+1) // the cell each rune starts on
	for i, r := range plain {
		cols[i+1] = cols[i] + ansi.StringWidth(string(r))
	}
	lead := 0
	for lead < len(plain) && plain[lead] == ' ' {
		lead++
	}
	hang := cols[lead] + 2
	if i := strings.Index(string(plain[lead:]), "  "); i >= 0 {
		j := lead + len([]rune(string(plain[lead:])[:i]))
		for j < len(plain) && plain[j] == ' ' {
			j++
		}
		if cols[j] <= width/2 {
			hang = cols[j]
		}
	}
	hang = min(hang, width/2)
	glued := map[int]bool{} // a pointer to a screen is never broken (#463)
	for _, g := range gluedPointer.FindAllStringIndex(string(plain), -1) {
		for j := len([]rune(string(plain)[:g[0]])); j < len([]rune(string(plain)[:g[1]])); j++ {
			glued[j] = true
		}
	}
	var out []string
	start, lim := 0, width
	for start < len(plain) {
		end := len(plain)
		if cols[end]-cols[start] > lim {
			end = -1
			hard := start + 1
			for j := start + 1; j < len(plain) && cols[j]-cols[start] <= lim; j++ {
				hard = j
				if plain[j] == ' ' && j > lead && plain[j-1] != ' ' && !glued[j] {
					end = j
				}
			}
			if end < 0 {
				end = hard
			}
		}
		piece := ansi.Cut(l, cols[start], cols[end])
		if len(out) > 0 {
			piece = strings.Repeat(" ", hang) + piece
		}
		if strings.Contains(piece, "\x1b") {
			piece += "\x1b[0m"
		}
		out = append(out, piece)
		for start = end; start < len(plain) && plain[start] == ' '; start++ {
		}
		lim = width - hang
	}
	return out
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

// wrapLines wraps prose to the modal's width ahead of the box, for a
// caller that counts the rows (the card's scene); the box wraps any
// line still wider itself (wrapLine, #463).
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
