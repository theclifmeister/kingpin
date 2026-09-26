// The status bar: one message at a time, its kind (a confirmation, a
// warning, a refusal) its colour, one sentence each, and the footer
// that draws it (#275: out of model.go).

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// statusKind is what a status message is, and so how the status bar
// colours it: neutral for a confirmation of what you did, a warning for
// a refusal, bad for a danger.
type statusKind int

const (
	statusBody statusKind = iota
	statusWarning
	statusBad
)

// say sets the status to a confirmation of what you did, in the body
// colour; refuse to a refusal, in the warning colour, ended with a full
// stop where the site left it off (`Can't hire: the crew is as big as
// you can manage.`); alarm to a danger, in red. Every site sets the
// kind through one of the three (#88, TestStatusKinds).
func (m *Model) say(s string) {
	m.status, m.statusKind = s, statusBody
}

func (m *Model) refuse(s string) {
	m.status, m.statusKind = sentence(s), statusWarning
}

func (m *Model) alarm(s string) {
	m.status, m.statusKind = sentence(s), statusBad
}

// sentence ends s with a full stop where it has no end punctuation, so
// a refusal built on a game error (`Can't sell: only 3 Weed in
// Eastside`) reads as one; a dialog error is capitalized too
// (dialogError).
func sentence(s string) string {
	if s == "" || strings.ContainsRune(".!?", rune(s[len(s)-1])) {
		return s
	}
	return s + "."
}

// statusStyle is the colour of the status message by its kind: a
// confirmation in the body colour, a refusal in the warning colour, a
// danger in red. Every site sets the kind (say, refuse, alarm).
func (m *Model) statusStyle() lipgloss.Style { return kindStyle(m.statusKind) }

// kindStyle is a status kind's colour: the bar's and the overlay's
// STATUS (#535).
func kindStyle(k statusKind) lipgloss.Style {
	switch k {
	case statusWarning:
		return theme.Warning
	case statusBad:
		return theme.Bad
	}
	return theme.Body
}

// viewFooter is the status bar: the status message at the left, in its
// kind's colour, and `? help` pinned at the right; no legend (#109: the
// keys are listed where they are used, in the pane). The message wins:
// when the two cannot share the row it shows alone, and it is cut only
// when it alone does not fit. Inside a modal the bar repeats the
// modal's footer and nothing else.
func (m *Model) viewFooter() string {
	if f := m.modalFooter(); f != nil {
		return fit(legend(f), m.width)
	}
	help := m.helpPair()
	if m.status == "" {
		return fit(strings.Repeat(" ", max(0, m.width-lipgloss.Width(help)))+help, m.width)
	}
	msg := " " + m.statusStyle().Render(m.status)
	if gap := m.width - lipgloss.Width(msg) - lipgloss.Width(help); gap >= 0 {
		return msg + strings.Repeat(" ", gap) + help
	}
	if !m.statusCut() {
		return fit(msg, m.width)
	}
	// Too long for the bar alone (#535: "… P…" hid a campaign's
	// confirmation): cut, and space opens it whole, as the strip's
	// `␣ more` says.
	more := theme.Key.Render("␣") + theme.Subtle.Render(" more")
	room := max(1, m.width-lipgloss.Width(more)-2)
	return fit(truncate(msg, room), room) + "  " + more
}

// statusCut is whether the status message is too long for the bar even
// alone.
func (m *Model) statusCut() bool {
	return m.status != "" && 1+lipgloss.Width(m.status) > m.width
}

// keepSaid keeps the message the bar cut for the key pressed on it, so
// space there shows it whole (#535); any other message, or none, keeps
// nothing. Every key in play mode calls it before the bar is cleared.
func (m *Model) keepSaid() {
	m.said, m.saidKind = "", statusBody
	if m.statusCut() {
		m.said, m.saidKind = m.status, m.statusKind
	}
}

// saidSection is the overlay's first section while it holds a message
// the bar cut (#535): STATUS, the message whole in its colour, which the
// modal wraps.
func (m *Model) saidSection() []section {
	if m.said == "" {
		return nil
	}
	return []section{{title: "STATUS", lines: []string{kindStyle(m.saidKind).Render(m.said)}}}
}
