package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The details pane (#79) is the one place a screen's details live: what
// is selected, the further sections the screen needs, and KEYS, last and
// never cut. Each screen feeds it as sections through its details()
// method; the frame draws it beside MAIN at paneMinWidth columns and up,
// collapses it to the one-line strip under that, and space opens the
// same sections as a modal overlay (modeDetails) where the strip is.

const (
	paneWidth    = 36  // columns, border included
	paneMinWidth = 100 // the terminal width from which the pane sits beside MAIN
	paneTextW    = paneWidth - 4
	paneLabelW   = 11 // a section row's label column; the value has the rest
)

// section is a titled block of the pane: the selection's name in caps
// with its facts one per line, or a further block like ALERTS.
type section struct {
	title string
	lines []string
}

// row is a section line with the label left and the value after it.
func row(label, value string) string {
	return theme.Subtle.Render(fit(label, paneLabelW)) + " " + value
}

// cut truncates s to w cells. A styled string cut short loses its
// closing sequence, so a cut one gets a reset after the ellipsis.
func cut(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return truncate(s, w) + "\x1b[0m"
}

// keyRow is a section line saying what a key would do to the selection.
func keyRow(key, what string) string {
	return theme.Key.Render(fit(key, max(2, lipgloss.Width(key)))) + " " + what
}

// wrap breaks plain prose into lines of at most w cells at the spaces,
// for a note the pane carries whole rather than cut.
func wrap(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	var line string
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= w:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	if line != "" {
		out = append(out, line)
	}
	for i, l := range out {
		out[i] = truncate(l, w)
	}
	return out
}

// wrapped is a section's lines for a note, wrapped and styled.
func wrapped(style lipgloss.Style, s string) []string {
	var out []string
	for _, l := range wrap(s, paneTextW) {
		out = append(out, style.Render(l))
	}
	return out
}

// sectionTitle is how a section is headed in the pane and the overlay.
func sectionTitle(title string, accent lipgloss.Color) string {
	return lipgloss.NewStyle().Bold(true).Foreground(accent).Render(truncate(title, paneTextW))
}

// keyCellW is a KEYS cell: `key  label` in the pane's half-width.
const keyCellW = paneTextW / 2

// keyLines is the KEYS section: the title, then the bindings as
// `key  label` in cells of keyCellW, as many a line as textW holds (two
// in the pane, four in the overlay), the labels as the legend draws
// them; a pair too wide for one cell takes two rather than a cut.
func (m *Model) keyLines(keys []binding, textW int, accent lipgloss.Color) []string {
	ls := []string{sectionTitle("KEYS", accent)}
	cols := max(1, textW/keyCellW)
	var line string
	used := 0
	for _, b := range keys {
		cell := theme.Key.Render(fit(b.key, max(2, lipgloss.Width(b.key)))) + " " + theme.Subtle.Render(m.labelOf(b))
		need := min(cols, (lipgloss.Width(cell)+1+keyCellW-1)/keyCellW)
		if used+need > cols {
			ls = append(ls, strings.TrimRight(line, " "))
			line, used = "", 0
		}
		line += fit(cut(cell, need*keyCellW-1), need*keyCellW)
		used += need
	}
	if line != "" {
		ls = append(ls, strings.TrimRight(line, " "))
	}
	return ls
}

// trimSections cuts the sections to room lines (a title, its lines, and
// a blank between sections each count one): from the bottom of the
// lowest section first, each cut section ending in `…`, a section with
// nothing left but its title going whole.
func trimSections(secs []section, room int) []section {
	out := make([]section, len(secs))
	for i, s := range secs {
		out[i] = section{s.title, append([]string(nil), s.lines...)}
	}
	height := func() int {
		n := 0
		for i, s := range out {
			if i > 0 {
				n++
			}
			n += 1 + len(s.lines)
		}
		return n
	}
	cut := map[int]bool{}
	for len(out) > 0 && height() > room {
		i := len(out) - 1
		s := &out[i]
		switch {
		case !cut[i] && len(s.lines) > 0:
			cut[i] = true
			s.lines[len(s.lines)-1] = theme.Subtle.Render("…")
		case len(s.lines) > 1:
			s.lines = append(s.lines[:len(s.lines)-2], theme.Subtle.Render("…"))
		default:
			out = out[:i]
		}
	}
	return out
}

// sectionLines renders sections as text lines textW cells wide, a blank
// between sections, cut to room lines; a room under zero cuts nothing.
func sectionLines(secs []section, textW, room int, accent lipgloss.Color) []string {
	if room >= 0 {
		secs = trimSections(secs, room)
	}
	var ls []string
	for i, s := range secs {
		if i > 0 {
			ls = append(ls, "")
		}
		ls = append(ls, sectionTitle(s.title, accent))
		for _, l := range s.lines {
			ls = append(ls, cut(l, textW))
		}
	}
	return ls
}

// pane renders the bordered details pane, w by h cells: the sections
// from the top, KEYS anchored at the bottom, whatever is between them
// blank.
func (m *Model) pane(sections []section, keys []binding, w, h int, accent lipgloss.Color) string {
	textW := w - 4
	rows := h - 2
	kl := m.keyLines(keys, textW, accent)
	room := rows - len(kl) - 1 // a blank over KEYS
	ls := sectionLines(sections, textW, room, accent)
	for len(ls) < rows-len(kl) {
		ls = append(ls, "")
	}
	ls = append(ls, kl...)
	return panel("DETAILS", strings.Join(ls, "\n"), w, h, accent)
}

var spaces = regexp.MustCompile(`  +`)

// strip is the pane collapsed to one line for a terminal too narrow to
// hold it beside MAIN: the first section's title and its first lines
// joined with ` · `, cut to the width, `␣ more` at the right.
func strip(sections []section, w int, accent lipgloss.Color) string {
	more := theme.Key.Render("␣") + theme.Subtle.Render(" more")
	moreW := lipgloss.Width(more) + 2
	var text string
	if len(sections) > 0 {
		s := sections[0]
		parts := []string{lipgloss.NewStyle().Bold(true).Foreground(accent).Render(s.title)}
		for _, l := range s.lines {
			if l = strings.TrimSpace(spaces.ReplaceAllString(l, " ")); l != "" {
				parts = append(parts, l)
			}
		}
		text = strings.Join(parts, theme.Subtle.Render(" · "))
	}
	text = theme.Gold.Render("▸ ") + text
	return fit(cut(text, max(1, w-moreW)), max(1, w-moreW)) + "  " + more
}

// overlay renders the pane's sections as a modal over MAIN, for a
// terminal too narrow to hold the pane beside it: the same sections in
// the one modal (#81), every one whole, KEYS last. The overlay is the
// one place the 80-column player can read what the strip could not
// show, so nothing is cut there (#87): a body taller than the room
// scrolls, as every modal's does, and the footer says so.
func (m *Model) overlay(sections []section, keys []binding, accent lipgloss.Color) string {
	kl := m.keyLines(keys, m.modalInner(), accent)
	ls := sectionLines(sections, m.modalInner(), -1, accent)
	ls = append(ls, "")
	ls = append(ls, kl...)
	return m.modal("DETAILS", ls, m.modalFooter())
}
