package anim

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Text is what a scene resolves to: the block art or the words, as
// rows of runes, drawn centred on the canvas. A cell is a rune at a row
// and column of the text; a blank is no cell.
type Text struct {
	rows [][]rune
	w    int
}

// NewText splits the art on newlines. Tabs are not expanded; every rune
// is one cell, so the art is plain and single-width.
func NewText(s string) Text {
	var t Text
	for _, l := range strings.Split(strings.Trim(s, "\n"), "\n") {
		r := []rune(l)
		t.rows = append(t.rows, r)
		t.w = max(t.w, lipgloss.Width(l))
	}
	return t
}

// Width is the widest row; Height the number of rows.
func (t Text) Width() int  { return t.w }
func (t Text) Height() int { return len(t.rows) }

// Origin is the top-left cell of the text centred on a w by h canvas.
func (t Text) Origin(w, h int) (x, y int) {
	return (w - t.w) / 2, (h - t.Height()) / 2
}

// Cell is one rune of the text at its row and column.
type Cell struct {
	X, Y int
	R    rune
}

// Cells is every non-blank cell in reading order: top to bottom, left
// to right, the order TTE sorts characters in.
func (t Text) Cells() []Cell {
	var cs []Cell
	for y, r := range t.rows {
		for x, c := range r {
			if c != ' ' {
				cs = append(cs, Cell{X: x, Y: y, R: c})
			}
		}
	}
	return cs
}
