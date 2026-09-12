package anim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Direction is which way a wipe, a slide or a pour runs: Right is left
// to right, Down top to bottom, and Diagonal from the top-left corner
// to the bottom-right (TTE's default wipe).
type Direction int

const (
	Diagonal Direction = iota
	Right
	Down
	Left
	Up
)

// Wipe is TTE's wipe (NOTICE; ttfx's src/effects/wipe.rs is the
// reference): the text is revealed a group of cells at a time, the
// groups the diagonals from the top-left corner, or the columns or
// the rows for the other directions (WipeFrom), the newest group in
// theme.Text for a moment and the rest in the accent. The original
// eases the reveal in and out over a hundred steps; the port eases it
// out alone, so the first frames move (a curve that starts slow
// reveals nothing for the first several frames of a short wipe).
//
// No dice: the groups are the text's.
func Wipe(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	return WipeFrom(Diagonal)(text, accent, over, rng)
}

// WipeFrom is a wipe in a direction: Right reveals column by column,
// Down row by row (the ending's red bars are Curtain, the colour-only
// mode), Diagonal from the corner.
func WipeFrom(dir Direction) Maker {
	return func(text Text, accent lipgloss.Color, over time.Duration, _ *rand.Rand) Scene {
		return &wipe{text: text, accent: accent, over: length(over), dir: dir}
	}
}

// Curtain is wipe's colour-only mode: no text, every cell of the
// canvas filled with `█` in the colour as the front passes, top to
// bottom (CurtainFrom for another direction), eased in like a thing
// that falls, and held. It is the ending's red bars (#156) and, played
// in Reverse, the bars lifting; the text is ignored, and Needs says so.
func Curtain(_ Text, colour lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	return CurtainFrom(Down)(Text{}, colour, over, rng)
}

// CurtainFrom is a curtain in a direction.
func CurtainFrom(dir Direction) Maker {
	return func(_ Text, colour lipgloss.Color, over time.Duration, _ *rand.Rand) Scene {
		return &wipe{accent: colour, over: length(over), dir: dir, curtain: true}
	}
}

// edgeHold is how long the newest group of a wipe shows in theme.Text.
const edgeHold = 2 * Frame

type wipe struct {
	text    Text
	accent  lipgloss.Color
	over    time.Duration
	dir     Direction
	curtain bool
}

func (s *wipe) Done(t time.Duration) bool { return t >= s.over }

// group is the group a cell of a w by h area belongs to in the
// direction, 0 the first revealed; groups is how many there are.
func (s *wipe) group(x, y, w, h int) int {
	switch s.dir {
	case Right:
		return x
	case Left:
		return w - 1 - x
	case Down:
		return y
	case Up:
		return h - 1 - y
	}
	return x + y
}

func (s *wipe) groups(w, h int) int {
	switch s.dir {
	case Right, Left:
		return w
	case Down, Up:
		return h
	}
	return w + h - 1
}

func (s *wipe) Frame(t time.Duration, w, h int) []string { return frame(s, t, w, h) }

func (s *wipe) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if s.curtain {
		s.curtainFrame(cv, t)
		return
	}
	if t >= s.over {
		drawText(cv, s.text, s.accent)
		return
	}
	W, H := s.text.Width(), s.text.Height()
	n := s.groups(W, H)
	// The front: how many groups are revealed at t, eased out, and when
	// each group was, so the newest wear theme.Text for edgeHold.
	front := OutSine(at(t, 0, s.over))
	shown := int(math.Floor(front*float64(n) + 1e-9))
	ox, oy := s.text.Origin(w, h)
	for _, c := range s.text.Cells() {
		g := s.group(c.X, c.Y, W, H)
		if g >= shown {
			continue
		}
		col := s.accent
		if s.revealedAt(g, n) > t-edgeHold {
			col = theme.Text
		}
		cv.Set(ox+c.X, oy+c.Y, c.R, col)
	}
}

// revealedAt is when group g of n was first shown: the inverse of the
// eased front, so the edge's hold reads off the clock rather than the
// frame count.
func (s *wipe) revealedAt(g, n int) time.Duration {
	// OutSine(p) = sin(p·π/2) = (g+1)/n  =>  p = asin((g+1)/n)·2/π,
	// the front crossing the whole of the group.
	p := math.Asin(math.Min(1, float64(g+1)/float64(n))) * 2 / math.Pi
	return time.Duration(p * float64(s.over))
}

// curtainFrame fills the canvas group by group in the colour: eased
// in, the fall of a curtain; every cell by the end, and held.
func (s *wipe) curtainFrame(cv *Canvas, t time.Duration) {
	n := s.groups(cv.W, cv.H)
	shown := n
	if t < s.over {
		shown = int(math.Floor(InQuad(at(t, 0, s.over))*float64(n) + 1e-9))
	}
	for y := 0; y < cv.H; y++ {
		for x := 0; x < cv.W; x++ {
			if s.group(x, y, cv.W, cv.H) < shown {
				cv.Set(x, y, '█', s.accent)
			}
		}
	}
}
