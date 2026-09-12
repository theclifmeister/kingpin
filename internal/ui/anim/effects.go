package anim

import (
	"math/rand/v2"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The effect set (#153): the ports of TerminalTextEffects a scene draws
// from, each a Maker of the one shape, so a scene takes an effect by
// name and the start menu cycles them. Every effect resolves the text
// in the accent in the length given and is a pure function of time
// from then on: the dice are thrown once, at construction, off the rng
// the scene passes (anim.Seed's, never the game's), and Frame reads
// what they decided. Colour is decoration on a shape that reads in
// sixteen colours: the motion is in the characters, which is what
// TestEffectsReadInSixteenColours holds every effect to.

// Maker makes an effect over a text: the text it resolves to, the
// accent it is left in, how long it takes and the dice. Decrypt is the
// first; every effect in the set is one, so a scene picks by name.
type Maker func(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene

// Needs is what an effect needs of a scene: a text (the curtain has
// none) and a canvas at least MinW by MinH. A scene that lacks it plays
// a Still instead.
type Needs struct {
	Text       bool
	MinW, MinH int
}

// Met reports whether the text and the canvas are enough for the
// effect: a text with at least one cell where one is needed, and the
// canvas at least the minimum.
func (n Needs) Met(text Text, w, h int) bool {
	if n.Text && len(text.Cells()) == 0 {
		return false
	}
	return w >= n.MinW && h >= n.MinH
}

// Effect is one of the set: its TTE name, the Maker, what it needs and
// whether it throws dice (two seeds render two runs; print, wipe, slide
// and pour draw none, so their frames are the same on every seed).
type Effect struct {
	Name  string
	New   Maker
	Needs Needs
	Dice  bool
}

// Effects is the set by name: decrypt and the eight of #153, and the
// curtain, wipe's colour-only mode, which resolves to no text and so is
// out of the title's cycle (Needs.Text is how a caller tells).
var Effects = map[string]Effect{
	"decrypt": {Name: "decrypt", New: Decrypt, Needs: Needs{Text: true, MinW: 1, MinH: 1}, Dice: true},
	"print":   {Name: "print", New: Print, Needs: Needs{Text: true, MinW: 1, MinH: 1}},
	"wipe":    {Name: "wipe", New: Wipe, Needs: Needs{Text: true, MinW: 1, MinH: 1}},
	"curtain": {Name: "curtain", New: Curtain, Needs: Needs{MinW: 1, MinH: 1}},
	"slide":   {Name: "slide", New: Slide, Needs: Needs{Text: true, MinW: 4, MinH: 1}},
	"pour":    {Name: "pour", New: Pour, Needs: Needs{Text: true, MinW: 1, MinH: 2}},
	"rain":    {Name: "rain", New: Rain, Needs: Needs{Text: true, MinW: 1, MinH: 2}, Dice: true},
	"beams":   {Name: "beams", New: Beams, Needs: Needs{Text: true, MinW: 4, MinH: 1}, Dice: true},
	"burn":    {Name: "burn", New: Burn, Needs: Needs{Text: true, MinW: 1, MinH: 1}, Dice: true},
	"vhstape": {Name: "vhstape", New: Vhstape, Needs: Needs{Text: true, MinW: 4, MinH: 1}, Dice: true},
	"matrix":  {Name: "matrix", New: Matrix, Needs: Needs{Text: true, MinW: 1, MinH: 2}, Dice: true},
}

// Names is the set's names in one order, for a cycle and a listing.
func Names() []string {
	names := make([]string, 0, len(Effects))
	for n := range Effects {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Still is the fallback: the text in the accent, done at once, for a
// scene whose effect's Needs are not met or that plays with animation
// off. Its frame is what every effect's last frame is.
func Still(text Text, accent lipgloss.Color) Scene { return still{text, accent} }

type still struct {
	text   Text
	accent lipgloss.Color
}

func (s still) Done(time.Duration) bool                  { return true }
func (s still) Frame(t time.Duration, w, h int) []string { return frame(s, t, w, h) }
func (s still) paint(cv *Canvas, _ time.Duration)        { drawText(cv, s.text, s.accent) }

// painter is what every effect is underneath: a scene that draws its
// frame at t onto a canvas it is given, so Layer (#156) can put one
// effect's cells over another's on one canvas. Frame is paint on a
// fresh canvas rendered to lines.
type painter interface {
	Scene
	paint(cv *Canvas, t time.Duration)
}

// frame is an effect's Frame: a fresh canvas painted and rendered.
func frame(p painter, t time.Duration, w, h int) []string {
	cv := NewCanvas(w, h)
	p.paint(cv, t)
	return cv.Lines()
}

// drawText puts the whole text on the canvas in one colour, centred.
func drawText(cv *Canvas, text Text, col lipgloss.Color) *Canvas {
	ox, oy := text.Origin(cv.W, cv.H)
	for _, c := range text.Cells() {
		cv.Set(ox+c.X, oy+c.Y, c.R, col)
	}
	return cv
}

// Reverse plays an effect backwards: the scene opens on the resolved
// text and runs the effect's frames from its end to its start, so a
// pour reversed is the text falling off the canvas (the ending's cash
// figures, #156) and a decrypt reversed is the text dissolving. Done at
// the same length.
func Reverse(m Maker) Maker {
	return func(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
		return reversed{s: m(text, accent, over, rng), over: over}
	}
}

type reversed struct {
	s    Scene
	over time.Duration
}

func (r reversed) Done(t time.Duration) bool                { return t >= r.over }
func (r reversed) Frame(t time.Duration, w, h int) []string { return frame(r, t, w, h) }
func (r reversed) paint(cv *Canvas, t time.Duration)        { paint(r.s, cv, max(0, r.over-t)) }

// paint draws a scene at t onto the canvas: a painter's cells, or, for
// a scene from outside the package, its rendered rows in plain text
// where they have any (its colours are its own lines', which a canvas
// cannot hold; every scene of the game's is a painter).
func paint(s Scene, cv *Canvas, t time.Duration) {
	if p, ok := s.(painter); ok {
		p.paint(cv, t)
		return
	}
	for y, l := range s.Frame(t, cv.W, cv.H) {
		for x, r := range []rune(ansi.Strip(l)) {
			if r != ' ' {
				cv.Set(x, y, r, "")
			}
		}
	}
}

// length is the effect's clock: never under a frame, so a scene asked
// for in no time is a still with one frame's motion rather than a
// division by zero.
func length(over time.Duration) time.Duration { return max(over, Frame) }

// at is progress through a span as a fraction, clamped.
func at(t, start, span time.Duration) float64 {
	if span <= 0 {
		if t >= start {
			return 1
		}
		return 0
	}
	return clamp01(float64(t-start) / float64(span))
}

// frames is the frame index of t: the churn glyphs are picked by it,
// so a frame asked for twice is the same frame.
func frames(t time.Duration) int { return int(t / Frame) }

// hash mixes a few integers into one, for the glyph a cell shows on a
// frame without a die: the same cell on the same frame is the same
// glyph, and a neighbour's differs.
func hash(vs ...int) uint64 {
	h := uint64(0x9E3779B97F4A7C15)
	for _, v := range vs {
		h ^= uint64(v) + 0x9E3779B97F4A7C15 + (h << 6) + (h >> 2)
		h *= 0xBF58476D1CE4E5B9
		h ^= h >> 31
	}
	return h
}

// pick is one of a set by a hash.
func pick(rs []rune, h uint64) rune { return rs[h%uint64(len(rs))] }
