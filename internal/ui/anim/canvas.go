package anim

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Canvas is a frame being drawn: w by h cells, each a rune in a colour.
// Lines renders it as h lines, each row's cells joined into runs of one
// colour through theme.Fg, so the package builds no style of its own
// (TestThemeIsTheOnlyStylist greps it). An unset cell is a blank; a cell
// with no colour is plain text. A cell set outside the canvas is
// dropped, so a scene wider than its room is cut, never wrapped.
type Canvas struct {
	W, H  int
	runes []rune
	cols  []lipgloss.Color
}

// NewCanvas is a blank canvas of w by h.
func NewCanvas(w, h int) *Canvas {
	w, h = max(0, w), max(0, h)
	return &Canvas{W: w, H: h, runes: make([]rune, w*h), cols: make([]lipgloss.Color, w*h)}
}

// Set puts r at (x, y) in the colour c; "" is plain.
func (c *Canvas) Set(x, y int, r rune, col lipgloss.Color) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	c.runes[y*c.W+x], c.cols[y*c.W+x] = r, col
}

// Lines is the canvas as h lines of at most w cells, trailing blanks
// dropped, each run of one colour styled once.
func (c *Canvas) Lines() []string {
	out := make([]string, c.H)
	for y := 0; y < c.H; y++ {
		row, cols := c.runes[y*c.W:(y+1)*c.W], c.cols[y*c.W:(y+1)*c.W]
		end := c.W
		for end > 0 && row[end-1] == 0 {
			end--
		}
		var b strings.Builder
		var run []rune
		var col lipgloss.Color
		flush := func() {
			if len(run) == 0 {
				return
			}
			if col == "" {
				b.WriteString(string(run))
			} else {
				b.WriteString(theme.Fg(col).Render(string(run)))
			}
			run = run[:0]
		}
		for x := 0; x < end; x++ {
			r := row[x]
			if r == 0 {
				r = ' '
			}
			if cols[x] != col {
				flush()
				col = cols[x]
			}
			run = append(run, r)
		}
		flush()
		out[y] = b.String()
	}
	return out
}
