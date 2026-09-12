package anim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Matrix is TTE's matrix (NOTICE; ttfx's src/effects/matrix.rs is the
// reference): digital rain over the whole canvas, streams of the
// original's glyphs (digits, punctuation and half-width katakana)
// falling down every column, a bright head in theme.Text over a body
// in theme.Market green and a tail in theme.Dim; then every column
// fills with rain to the bottom; then the rain resolves, cell by cell
// in a random order, to the text in the accent where the text is and
// to nothing where it is not. The rain takes half the length, the
// fill to seven tenths, the resolve the rest.
//
// The dice: the seed of every column's streams and of the resolve's
// order; the streams themselves and their glyphs are hashed off it per
// column, so the same seed rains the same on a canvas of any width and
// Frame throws nothing.
func Matrix(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	return &matrix{text: text, accent: accent, over: length(over), seed: int(rng.Uint32())}
}

// matrixGlyphs are the original's rain symbols.
var matrixGlyphs = []rune("2598Z*):.\"=+-¦|_ｦｱｳｴｵｶｷｹｺｻｼｽｾｿﾀﾂﾃﾅﾆﾇﾈﾊﾋﾎﾏﾐﾑﾒﾓﾔﾕﾗﾘﾜ")

// matrixStreams is how many streams a column has over the rain, and
// matrixTail the longest tail.
const (
	matrixStreams = 3
	matrixTail    = 12
)

type matrix struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	seed   int
}

func (m *matrix) Done(t time.Duration) bool { return t >= m.over }

// stream is one drop down a column: when its head leaves the top, how
// many rows a second it falls and how long its tail is.
type stream struct {
	start time.Duration
	rows  float64 // a second
	tail  int
}

// streams are column x's over the rain and the fill: each hashed off
// the seed and the column, the rain's spread over the first half and
// the fill's, the last, an endless tail starting in the sixth tenth
// and reaching the bottom by the seventh.
func (m *matrix) streams(x, h int) [matrixStreams + 1]stream {
	var out [matrixStreams + 1]stream
	rain := m.over / 2
	for i := 0; i < matrixStreams; i++ {
		hs := hash(m.seed, x, i)
		out[i] = stream{
			start: time.Duration(float64(rain) * float64(hs%1000) / 1000),
			rows:  8 + float64((hs>>10)%1000)/1000*12,
			tail:  4 + int((hs>>20)%uint64(matrixTail-3)),
		}
	}
	hs := hash(m.seed, x, matrixStreams)
	start := m.over/2 + time.Duration(float64(m.over)/10*float64(hs%1000)/1000)
	end := m.over * 7 / 10
	out[matrixStreams] = stream{start: start, rows: float64(h+1) / math.Max(0.001, (end-start).Seconds()), tail: 1 << 20}
	return out
}

// glyph is the rain glyph a cell shows: swapped every few frames.
func (m *matrix) glyph(x, y, f int) rune { return pick(matrixGlyphs, hash(m.seed, x, y, f/6)) }

func (m *matrix) Frame(t time.Duration, w, h int) []string {
	cv := NewCanvas(w, h)
	if t >= m.over {
		return drawText(cv, m.text, m.accent).Lines()
	}
	f := frames(t)
	ox, oy := m.text.Origin(w, h)
	resolveFrom := m.over * 7 / 10
	resolving := t >= resolveFrom
	for x := 0; x < w; x++ {
		ss := m.streams(x, h)
		for y := 0; y < h; y++ {
			// The nearest head above or on the cell, and how far behind
			// it the cell is: the colour.
			behind, tail := math.MaxInt, 0
			for _, s := range ss {
				if t < s.start {
					continue
				}
				head := int(math.Floor((t - s.start).Seconds() * s.rows))
				if d := head - y; d >= 0 && d < s.tail && d < behind {
					behind, tail = d, s.tail
				}
			}
			var col lipgloss.Color
			switch {
			case behind == math.MaxInt:
				continue
			case behind == 0:
				col = theme.Text
			case behind >= tail-3:
				col = theme.Dim
			default:
				col = theme.Market
			}
			if resolving {
				// Each cell resolves at its own moment of the last three
				// tenths, to the text or to nothing.
				when := resolveFrom + time.Duration(float64(m.over-resolveFrom)*0.95*float64(hash(m.seed, x, y, -1)%1000)/1000)
				if t >= when {
					continue
				}
			}
			cv.Set(x, y, m.glyph(x, y, f), col)
		}
	}
	// The text: what has resolved.
	if resolving {
		for _, c := range m.text.Cells() {
			x, y := ox+c.X, oy+c.Y
			when := resolveFrom + time.Duration(float64(m.over-resolveFrom)*0.95*float64(hash(m.seed, x, y, -1)%1000)/1000)
			if t >= when {
				cv.Set(x, y, c.R, m.accent)
			}
		}
	}
	return cv.Lines()
}
