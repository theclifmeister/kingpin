package anim

import (
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Burn is TTE's burn (NOTICE; ttfx's src/effects/burn.rs is the
// reference): the text stands in theme.Dim and burns, each cell
// through the original's glyphs, `' . ▖ ▙ █ ▜ ▀ ▝ .`, white-hot in
// theme.Text, then theme.Warn, then theme.Heat as it dies down, a wisp
// of smoke above it in theme.Dim, and is left as its character in the
// accent. The original spreads the fire along a random spanning tree
// of the text; the port burns from the bottom up, a flame front with a
// jitter a column and a cell, so a corner flipping or stock lost reads
// as a thing consumed. The front takes seven tenths of the length and
// a cell burns for a quarter of it.
//
// The dice: the front's jitter; Frame reads it.
func Burn(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	over = length(over)
	b := &burn{text: text, accent: accent, over: over, each: over / 4}
	W, H := text.Width(), text.Height()
	front := over * 7 / 10
	cols := make([]float64, W)
	for x := range cols {
		cols[x] = rng.Float64() * 1.5
	}
	// A cell ignites by its rows from the bottom plus its column's and
	// its own jitter, the whole spread over the front's time.
	span := float64(H) + 1.5 + 0.5
	for _, c := range text.Cells() {
		rows := float64(H-1-c.Y) + cols[c.X] + rng.Float64()*0.5
		b.cells = append(b.cells, ember{Cell: c, ignite: time.Duration(rows / span * float64(front))})
	}
	return b
}

// burnGlyphs are the original's, in order, with the colour each burns
// in: white-hot, then orange, then red.
var burnGlyphs = []rune{'\'', '.', '▖', '▙', '█', '▜', '▀', '▝', '.'}

func burnColour(step int) lipgloss.Color {
	switch {
	case step < 3:
		return theme.Text
	case step < 6:
		return theme.Warn
	}
	return theme.Heat
}

// smoke is what rises off a burning cell: the original's symbols.
var smoke = []rune{'.', ',', '\'', '`', '*'}

type ember struct {
	Cell
	ignite time.Duration
}

type burn struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	each   time.Duration // how long a cell burns
	cells  []ember
}

func (b *burn) Done(t time.Duration) bool { return t >= b.over }

func (b *burn) Frame(t time.Duration, w, h int) []string {
	cv := NewCanvas(w, h)
	if t >= b.over {
		return drawText(cv, b.text, b.accent).Lines()
	}
	ox, oy := b.text.Origin(w, h)
	f := frames(t)
	// The smoke first, so a cell of the text drawn after it wins.
	for _, c := range b.cells {
		if t < c.ignite || t >= c.ignite+b.each {
			continue
		}
		step := int(float64(len(burnGlyphs)) * at(t, c.ignite, b.each))
		if step >= 2 && step <= 6 && hash(c.X, c.Y, f/2)%3 == 0 {
			cv.Set(ox+c.X, oy+c.Y-1, pick(smoke, hash(c.X, c.Y, f)), theme.Dim)
		}
	}
	for _, c := range b.cells {
		x, y := ox+c.X, oy+c.Y
		switch {
		case t < c.ignite:
			cv.Set(x, y, c.R, theme.Dim)
		case t >= c.ignite+b.each:
			cv.Set(x, y, c.R, b.accent)
		default:
			step := int(float64(len(burnGlyphs)) * at(t, c.ignite, b.each))
			step = min(step, len(burnGlyphs)-1)
			cv.Set(x, y, burnGlyphs[step], burnColour(step))
		}
	}
	return cv.Lines()
}
