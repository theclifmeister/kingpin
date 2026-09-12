package anim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Pour and Rain are TTE's pour and rain (NOTICE; ttfx's
// src/effects/pour.rs and rain.rs are the reference), one engine: the
// characters fall from above the canvas into place, the bottom row
// first so the text stacks up. Pour releases them in order, a row at a
// time, alternating the way along each row (the original's zigzag),
// each falling in theme.Text with a trail of `▒░` in theme.Dim behind
// it, eased in like a drop. Rain releases them in a random order a row
// at a time, each a raindrop (`o . , * |`, the original's) in theme.Dim
// at a speed of its own, flashing theme.Text as it lands. Both leave
// the text in the accent. The timing is the length's: the falls take a
// share of it and the releases spread over the rest. PourFrom pours
// from another edge, and Reverse(PourFrom(Up)) is the text falling off
// the bottom of the canvas (the ending's cash, #156).
//
// The dice: pour throws none but the rain's order and speeds; Frame
// reads them.
func Pour(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	return PourFrom(Down)(text, accent, over, rng)
}

// PourFrom pours in a direction: Down from the top edge (the default),
// Up from the bottom, Right and Left from the sides; Diagonal is Down.
func PourFrom(dir Direction) Maker {
	return func(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
		f := newFall(text, accent, over, dir)
		f.trail = true
		f.ease = InQuad
		// Rows from the far edge back (the bottom first for a pour
		// down), the cells of every other row reversed.
		rows := f.groups()
		for i, cells := range rows {
			if i%2 == 1 {
				for l, r := 0, len(cells)-1; l < r; l, r = l+1, r-1 {
					cells[l], cells[r] = cells[r], cells[l]
				}
			}
			for _, c := range cells {
				f.drops = append(f.drops, drop{Cell: c, glyph: c.R, speed: 1})
			}
		}
		f.schedule()
		return f
	}
}

// Rain is the rain: the characters land as raindrops from the top, in
// a random order a row at a time from the bottom, at speeds of their
// own.
func Rain(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	f := newFall(text, accent, over, Down)
	f.ease = InQuad
	for _, cells := range f.groups() {
		rng.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })
		for _, c := range cells {
			f.drops = append(f.drops, drop{
				Cell:  c,
				glyph: raindrops[rng.IntN(len(raindrops))],
				speed: 0.7 + rng.Float64()*0.6,
			})
		}
	}
	f.schedule()
	return f
}

// raindrops are the original's rain symbols.
var raindrops = []rune{'o', '.', ',', '*', '|'}

// fallFlash is how long a landed raindrop shows in theme.Text.
const fallFlash = 3 * Frame

type drop struct {
	Cell
	glyph   rune          // what falls: the character, or a raindrop
	speed   float64       // a fall takes the travel over this
	release time.Duration // when it starts
}

type fall struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	dir    Direction
	trail  bool
	ease   Easing
	travel time.Duration
	drops  []drop
}

func newFall(text Text, accent lipgloss.Color, over time.Duration, dir Direction) *fall {
	if dir == Diagonal {
		dir = Down
	}
	over = length(over)
	return &fall{text: text, accent: accent, over: over, dir: dir, travel: over * 7 / 20}
}

// groups are the text's rows (or columns for a sideways fall), the
// one nearest the edge the characters come from last: what lands
// first is what is farthest from it, so the text stacks.
func (f *fall) groups() [][]Cell {
	byKey := map[int][]Cell{}
	var keys []int
	for _, c := range f.text.Cells() {
		k := c.Y
		if f.dir == Left || f.dir == Right {
			k = c.X
		}
		if _, ok := byKey[k]; !ok {
			keys = append(keys, k)
		}
		byKey[k] = append(byKey[k], c)
	}
	// Reading order gives the keys ascending (rows) or, for columns,
	// in the order first met; sort them, then take them from the far
	// end for a fall from the top or the left.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	if f.dir == Down || f.dir == Right {
		for l, r := 0, len(keys)-1; l < r; l, r = l+1, r-1 {
			keys[l], keys[r] = keys[r], keys[l]
		}
	}
	out := make([][]Cell, len(keys))
	for i, k := range keys {
		out[i] = byKey[k]
	}
	return out
}

// schedule spreads the releases over the length less the slowest
// fall, in the order the drops were listed, so the last lands by the
// end.
func (f *fall) schedule() {
	slowest := f.travel
	for _, d := range f.drops {
		slowest = max(slowest, time.Duration(float64(f.travel)/d.speed))
	}
	n := math.Max(1, float64(len(f.drops)-1))
	step := float64(f.over-slowest) / n
	for i := range f.drops {
		f.drops[i].release = time.Duration(float64(i) * step)
	}
}

func (f *fall) Done(t time.Duration) bool { return t >= f.over }

func (f *fall) Frame(t time.Duration, w, h int) []string { return frame(f, t, w, h) }

func (f *fall) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if t >= f.over {
		drawText(cv, f.text, f.accent)
		return
	}
	ox, oy := f.text.Origin(w, h)
	for _, d := range f.drops {
		// A drop shows once it has moved: at its release it is still off
		// the edge, so a fall reversed ends on an empty canvas.
		if t <= d.release {
			continue
		}
		hx, hy := ox+d.X, oy+d.Y
		dur := time.Duration(float64(f.travel) / d.speed)
		p := at(t, d.release, dur)
		if p >= 1 {
			col := f.accent
			if !f.trail && t < d.release+dur+fallFlash {
				col = theme.Text
			}
			cv.Set(hx, hy, d.R, col)
			continue
		}
		// From the edge's own row or column to home (the original starts
		// on the canvas's top row, so a drop shows from its first
		// frame); the trail is the two cells behind, back toward the
		// edge.
		e := f.ease(p)
		x, y := float64(hx), float64(hy)
		dx, dy := 0, 0
		switch f.dir {
		case Down:
			y, dy = float64(hy)*e, -1
		case Up:
			y, dy = float64(h-1)-float64(h-1-hy)*e, 1
		case Right:
			x, dx = float64(hx)*e, -1
		case Left:
			x, dx = float64(w-1)-float64(w-1-hx)*e, 1
		}
		cx, cy := int(math.Round(x)), int(math.Round(y))
		col := theme.Dim
		if f.trail {
			col = theme.Text
			cv.Set(cx+dx, cy+dy, '▒', theme.Dim)
			cv.Set(cx+2*dx, cy+2*dy, '░', theme.Dim)
		}
		cv.Set(cx, cy, d.glyph, col)
	}
}
