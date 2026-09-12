package anim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Slide is TTE's slide (NOTICE; ttfx's src/effects/slide.rs is the
// reference): the characters slide in from an edge of the canvas to
// their places, a row at a time, the far end of each row released
// first so it travels the farthest and the row arrives together, in
// theme.Text while they move and the accent once they land. The
// original moves at a speed with in-out-quad easing and releases a
// character a frame; the port gives every character one travel time,
// eased out (a fast start that settles, so the first frame moves),
// and spreads the releases over the rest of the length. From the left
// by default (Right: the travel's direction); SlideFrom for the others,
// Up for a counter rolling in from below (#159).
//
// No dice: the order is the text's.
func Slide(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	return SlideFrom(Right)(text, accent, over, rng)
}

// SlideFrom is a slide travelling in a direction: Right and Left slide
// the rows in from the sides, Down and Up the columns from the top and
// the bottom; Diagonal is Right.
func SlideFrom(dir Direction) Maker {
	return func(text Text, accent lipgloss.Color, over time.Duration, _ *rand.Rand) Scene {
		over = length(over)
		s := &slide{text: text, accent: accent, over: over, dir: dir}
		if dir == Diagonal {
			s.dir = Right
		}
		// The groups: rows for a sideways slide, columns for a vertical
		// one; within a group the cell farthest from the edge is
		// released first (the original reverses the group).
		byGroup := map[int][]Cell{}
		var keys []int
		for _, c := range text.Cells() {
			k := c.Y
			if s.vertical() {
				k = c.X
			}
			if _, ok := byGroup[k]; !ok {
				keys = append(keys, k)
			}
			byGroup[k] = append(byGroup[k], c)
		}
		s.travel = over * 2 / 5
		longest := 0
		for _, k := range keys {
			longest = max(longest, len(byGroup[k]))
		}
		// Releases: group i's cells one step apart from 2i steps in
		// (the original's gap of two frames between rows), the last
		// release a travel before the end.
		steps := math.Max(1, float64(2*(len(keys)-1)+longest))
		step := float64(over-s.travel) / steps
		for i, k := range keys {
			cells := byGroup[k]
			for j := range cells {
				c := cells[len(cells)-1-j] // the far end first
				if s.dir == Left || s.dir == Up {
					c = cells[j]
				}
				s.cells = append(s.cells, slider{Cell: c, release: time.Duration(float64(2*i+j) * step)})
			}
		}
		return s
	}
}

type slider struct {
	Cell
	release time.Duration
}

type slide struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	dir    Direction
	travel time.Duration
	cells  []slider
}

func (s *slide) vertical() bool            { return s.dir == Down || s.dir == Up }
func (s *slide) Done(t time.Duration) bool { return t >= s.over }

func (s *slide) Frame(t time.Duration, w, h int) []string { return frame(s, t, w, h) }

func (s *slide) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if t >= s.over {
		drawText(cv, s.text, s.accent)
		return
	}
	ox, oy := s.text.Origin(w, h)
	for _, c := range s.cells {
		if t < c.release {
			continue
		}
		p := OutQuad(at(t, c.release, s.travel))
		hx, hy := ox+c.X, oy+c.Y
		if p >= 1 {
			cv.Set(hx, hy, c.R, s.accent)
			continue
		}
		// From one cell outside the edge to home.
		x, y := float64(hx), float64(hy)
		switch s.dir {
		case Right:
			x = -1 + (float64(hx)+1)*p
		case Left:
			x = float64(w) - (float64(w-hx))*p
		case Down:
			y = -1 + (float64(hy)+1)*p
		case Up:
			y = float64(h) - (float64(h-hy))*p
		}
		cv.Set(int(math.Round(x)), int(math.Round(y)), c.R, theme.Text)
	}
}
