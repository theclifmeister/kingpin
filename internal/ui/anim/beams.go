package anim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Beams is TTE's beams (NOTICE; ttfx's src/effects/beams.rs is the
// reference) finished with Omarchy's About sheen: beams of light run
// along the text's rows (`▂▁_`) and down its columns (`▌▍▎▏`), a bright
// head in theme.Text fading through theme.Logistics to theme.Dim, and
// leave the characters they cross faintly lit in theme.Dim; then one
// band of bright cells leans across the text from left to right, `█`
// and `▓` in theme.Text on the cells under it, and the text it has
// crossed rests in the accent. The original's final wipe is a diagonal
// brightening; the port's is the sheen from
// bin/omarchy-branding-about-animation, which already runs in three
// colours. The beams take the first half of the length, the sheen the
// second.
//
// The dice: which columns carry a beam and when each beam starts and
// how fast it runs; Frame reads them.
func Beams(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	over = length(over)
	b := &beams{text: text, accent: accent, over: over}
	W, H := text.Width(), text.Height()
	// Every row and two columns in five, released over the first third
	// and crossing the box (with a margin either side) in a sixth to a
	// third of the length.
	phase := over / 2
	for y := 0; y < H; y++ {
		b.beams = append(b.beams, beam{row: true, at: y,
			start: time.Duration(rng.Float64() * float64(phase) / 2),
			run:   time.Duration(float64(over) * (1 + rng.Float64()) / 6)})
	}
	for x := 0; x < W; x++ {
		if rng.IntN(5) < 2 {
			b.beams = append(b.beams, beam{at: x,
				start: time.Duration(rng.Float64() * float64(phase) / 2),
				run:   time.Duration(float64(over) * (1 + rng.Float64()) / 6)})
		}
	}
	return b
}

// beamMargin is how far outside the text's box a beam starts and ends.
const beamMargin = 4

var (
	rowBeam = []rune{'▂', '▁', '_'}
	colBeam = []rune{'▌', '▍', '▎', '▏'}
	sheen   = []rune{'▓', '█', '█', '▓'}
)

type beam struct {
	row   bool // along a row, else down a column
	at    int  // the row or column, in the text's frame
	start time.Duration
	run   time.Duration // to cross the box and its margins
}

type beams struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	beams  []beam
}

func (b *beams) Done(t time.Duration) bool { return t >= b.over }

// head is how far along its span a beam is at t, in cells from the
// span's start, or -1 before it starts; span is the length.
func (b *beam) head(t time.Duration, span int) float64 {
	if t < b.start {
		return -1
	}
	return float64(span+len(rowBeam)) * at(t, b.start, b.run)
}

func (b *beams) Frame(t time.Duration, w, h int) []string { return frame(b, t, w, h) }

func (b *beams) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if t >= b.over {
		drawText(cv, b.text, b.accent)
		return
	}
	W, H := b.text.Width(), b.text.Height()
	ox, oy := b.text.Origin(w, h)
	// The band: its left edge leans one cell a row, `/`, and crosses
	// from beyond the box's left to beyond its right over the second
	// half, eased in and out; -1 before it starts.
	band := -1.0
	if t >= b.over/2 {
		band = -float64(H+len(sheen)) + float64(W+2*H+2*len(sheen))*InOutSine(at(t, b.over/2, b.over/2))
	}
	lit := map[[2]int]bool{}
	// The beams, on every cell of the box and its margins; a cell a
	// beam's head has passed shows the beam's tail, then nothing.
	for i := range b.beams {
		bm := &b.beams[i]
		span := W + 2*beamMargin
		glyphs := rowBeam
		if !bm.row {
			span = H + 2*beamMargin
			glyphs = colBeam
		}
		hd := bm.head(t, span)
		if hd < 0 {
			continue
		}
		for k := 0; k < span; k++ {
			behind := int(hd) - k
			x, y := ox-beamMargin+k, oy+bm.at
			if !bm.row {
				x, y = ox+bm.at, oy-beamMargin+k
			}
			if behind >= 0 {
				lit[[2]int{x, y}] = true
			}
			if behind < 0 || behind >= len(glyphs) {
				continue
			}
			col := theme.Dim
			switch behind {
			case 0:
				col = theme.Text
			case 1:
				col = theme.Logistics
			}
			cv.Set(x, y, glyphs[behind], col)
		}
	}
	// The text: under the band the sheen, behind it the accent, ahead
	// of it faintly lit where a beam has been.
	for _, c := range b.text.Cells() {
		x, y := ox+c.X, oy+c.Y
		d := float64(c.X) - (band + float64(H-1-c.Y))
		switch {
		case band < 0 || d >= float64(len(sheen)):
			if lit[[2]int{x, y}] {
				cv.Set(x, y, c.R, theme.Dim)
			}
		case d < 0:
			cv.Set(x, y, c.R, b.accent)
		default:
			cv.Set(x, y, sheen[int(math.Floor(d))], theme.Text)
		}
	}
}
