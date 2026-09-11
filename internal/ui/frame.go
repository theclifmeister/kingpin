package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The frame (#79, #95) is what every play-mode screen renders into: the
// title bar on row 0, the body on rows 1..h-2 and the status bar on row
// h-1; nothing in it moves without you. The body is MAIN beside the
// details pane from paneMinWidth columns, MAIN over the one-line details
// strip (row h-2) under that. Modals keep the whole body (bodyHeight); a
// screen's MAIN gets mainWidth by mainHeight.

// paneShown reports whether the details pane sits beside MAIN.
func (m *Model) paneShown() bool {
	return m.width >= paneMinWidth && !m.paneHidden
}

// mainWidth is the width a screen's MAIN is drawn to.
func (m *Model) mainWidth() int {
	if m.paneShown() {
		return max(20, m.width-paneWidth)
	}
	return m.width
}

// bodyHeight is the body between the title bar and the status bar: the
// rows a modal has.
func (m *Model) bodyHeight() int {
	// title bar + status bar
	return max(5, m.height-2)
}

// mainHeight is the rows a screen's MAIN has: the body, less the
// details strip where the pane cannot sit beside it.
func (m *Model) mainHeight() int {
	h := m.bodyHeight()
	if m.width < paneMinWidth {
		h--
	}
	return max(4, h)
}

// resize sizes what keeps its own dimensions to the frame.
func (m *Model) resize() {
	m.journalFollow()
}

// block cuts and pads text to exactly w by h cells: every line
// truncated, never wrapped, short lines and missing rows padded.
func block(s string, w, h int) []string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(ls) > h {
		ls = ls[:h]
	}
	for len(ls) < h {
		ls = append(ls, "")
	}
	for i, l := range ls {
		ls[i] = fit(cut(l, w), w)
	}
	return ls
}

// frame lays a screen out: the title bar, MAIN with the pane beside it
// or the strip under it, and the status bar.
func (m *Model) frame(main string, sections []section, keys []binding, accent lipgloss.Color) string {
	h := m.mainHeight()
	body := block(main, m.mainWidth(), h)
	switch {
	case m.paneShown():
		side := strings.Split(m.pane(sections, keys, paneWidth, h, accent), "\n")
		for i := range body {
			if i < len(side) {
				body[i] += side[i]
			}
		}
	case m.width < paneMinWidth:
		body = append(body, strip(sections, m.width, accent))
	}
	// Every row is drawn, an empty status bar included, so the rows
	// keep their places.
	return strings.Join([]string{m.viewTitle(), strings.Join(body, "\n"), m.viewFooter()}, "\n")
}

// panel renders a titled bordered box of exactly w by h cells, the
// title in the top border (`╭─ STREET · Eastside ───╮`) in the accent.
// Lines are truncated, never wrapped, so the box never grows past its
// height.
func panel(title, content string, w, h int, accent lipgloss.Color) string {
	w, h = max(8, w), max(2, h)
	textW := w - 4
	border := theme.Fg(accent)
	head := truncate(title, w-6)
	top := border.Render("╭─ ") + theme.Heading(accent).Render(head) + border.Render(" "+strings.Repeat("─", w-5-lipgloss.Width(head))+"╮")
	ls := []string{top}
	for _, l := range block(content, textW, h-2) {
		ls = append(ls, border.Render("│ ")+l+border.Render(" │"))
	}
	ls = append(ls, border.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(ls, "\n")
}
