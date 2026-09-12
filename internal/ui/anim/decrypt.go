package anim

import (
	"math/rand/v2"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Decrypt is TTE's decrypt (NOTICE; ttfx's src/effects/decrypt.rs is
// the reference): the text is typed in as blocks, every cell then
// churns through cipher glyphs and settles, left to right, in the
// length given. The ciphertext is theme.Dim, a cell flashes theme.Text
// as it is discovered and the settled text is the accent. The shape is
// the original's, three phases a cell, typing, a fast churn and a slow
// one, then the discovery; the timing is scaled to the length rather
// than the original's frame counts, so a 1.5 s title and a 0.4 s
// morning are the same effect at two speeds.
//
// Every die is thrown here, once, so Frame is a pure function of t.
func Decrypt(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	d := &decrypt{text: text, accent: accent, over: over}
	w := max(1, text.Width())
	typing := over * 3 / 10               // the typing runs over the first three tenths
	start, spread := over*4/10, over*5/10 // the settling runs from the fourth tenth to the ninth
	cells := text.Cells()
	// TTE sorts top to bottom, left to right; the port types and settles
	// by column, so the art resolves left to right, with a cell's own
	// jitter inside its column so no column moves as one.
	sort.SliceStable(cells, func(i, j int) bool { return cells[i].X < cells[j].X })
	for _, c := range cells {
		appear := time.Duration(float64(typing) * (float64(c.X) + rng.Float64()) / float64(w))
		settle := start + time.Duration(float64(spread)*(float64(c.X)+rng.Float64())/float64(w))
		var steps []step
		at := appear
		// Typed in: the four blocks a frame each, as the original's
		// "typing" scene, then a cipher glyph.
		for _, r := range typingBlocks {
			steps = append(steps, step{at, r})
			at += Frame
		}
		// The fast churn: a glyph a frame for a few frames.
		for n := 4 + rng.IntN(7); n > 0 && at < settle; n-- {
			steps = append(steps, step{at, cipher(rng)})
			at += Frame
		}
		// The slow churn: a glyph held a few frames, three in ten held
		// longer (the original's 30% chance of a long hold, which
		// breaks the waves), until the cell settles.
		for at < settle {
			hold := 2 + rng.IntN(4)
			if rng.IntN(100) < 30 {
				hold = 6 + rng.IntN(5)
			}
			steps = append(steps, step{at, cipher(rng)})
			at += time.Duration(hold) * Frame
		}
		steps = append(steps, step{settle, c.R})
		d.cells = append(d.cells, cell{Cell: c, steps: steps, settle: settle})
	}
	return d
}

// typingBlocks are the glyphs a cell is typed in with, in order.
var typingBlocks = []rune{'▉', '▓', '▒', '░'}

// ciphers are the glyphs the ciphertext churns through: TTE's ranges,
// printable ASCII, the block elements, the box drawing set and the
// Latin supplement, every one a single cell wide.
var ciphers = func() []rune {
	var rs []rune
	for _, r := range [][2]rune{{33, 127}, {9608, 9632}, {9472, 9599}, {174, 452}} {
		for c := r[0]; c < r[1]; c++ {
			rs = append(rs, c)
		}
	}
	return rs
}()

func cipher(rng *rand.Rand) rune { return ciphers[rng.IntN(len(ciphers))] }

// flash is how long a discovered cell shows in theme.Text before the
// accent.
const flash = 3 * Frame

type step struct {
	at time.Duration
	r  rune
}

type cell struct {
	Cell
	steps  []step // what the cell shows from when, in order; the last is the text
	settle time.Duration
}

type decrypt struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	cells  []cell
}

func (d *decrypt) Done(t time.Duration) bool { return t >= d.over }

func (d *decrypt) Frame(t time.Duration, w, h int) []string { return frame(d, t, w, h) }

func (d *decrypt) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if t >= d.over {
		drawText(cv, d.text, d.accent)
		return
	}
	ox, oy := d.text.Origin(w, h)
	for _, c := range d.cells {
		if len(c.steps) == 0 || t < c.steps[0].at {
			continue
		}
		r := c.steps[0].r
		for _, s := range c.steps[1:] {
			if s.at > t {
				break
			}
			r = s.r
		}
		col := theme.Dim
		switch {
		case t >= c.settle+flash:
			col = d.accent
		case t >= c.settle:
			col = theme.Text
		}
		cv.Set(ox+c.X, oy+c.Y, r, col)
	}
}
