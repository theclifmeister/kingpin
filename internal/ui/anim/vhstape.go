package anim

import (
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Vhstape is TTE's vhstape (NOTICE; ttfx's src/effects/vhstape.rs is
// the reference): the text is on the screen and the tape is bad. Lines
// glitch, a line or a band of three shoved left or right for a few
// frames and back, its colour rolling through the original's white,
// red, green, blue (theme.Text, theme.Heat, theme.Market, theme.Crew)
// and snow (`# * . :`, the original's) at the torn edges; whole frames
// of snow come and go; then the picture loses itself to snow entirely,
// and then the lines are redrawn one at a time from the bottom in the
// accent and hold. The glitching runs over the first fifty-five
// hundredths of the length, the snow to three quarters, the redraw to
// the end.
//
// The dice: which lines glitch when, how far and which way, and the
// snow frames; Frame reads them, the snow's glyphs by a hash of the
// cell and the frame.
func Vhstape(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	over = length(over)
	v := &vhstape{text: text, accent: accent, over: over}
	H := text.Height()
	glitching := frames(over * 55 / 100)
	// Two streams of glitches back to back over the glitching frames,
	// so a line is always torn: one of single lines, one of bands.
	for stream := 0; stream < 2; stream++ {
		for f := stream * rng.IntN(3); f < glitching; {
			g := glitch{from: f, frames: 2 + rng.IntN(5), line: rng.IntN(max(1, H)), lines: 1,
				shift: (2 + rng.IntN(max(1, text.Width()/6+3))) * (1 - 2*rng.IntN(2))}
			if stream == 1 && H >= 3 {
				g.lines = 3
				g.line = min(g.line, H-3)
			}
			v.glitches = append(v.glitches, g)
			f += g.frames + rng.IntN(2)
		}
	}
	// Whole frames of snow, one in fifteen, while glitching.
	for f := 0; f < glitching; f++ {
		if rng.IntN(15) == 0 {
			v.snow = append(v.snow, f)
		}
	}
	return v
}

var (
	snowGlyphs   = []rune{'#', '*', '.', ':'}
	snowColours  = []lipgloss.Color{theme.Dim, theme.Dim, theme.News, theme.Text}
	tapeColours  = []lipgloss.Color{theme.Text, theme.Heat, theme.Market, theme.Crew, theme.Text}
	tapeSnowEdge = 2 // the cells of snow at a shoved line's torn ends
)

type glitch struct {
	from, frames int // the frames it holds
	line, lines  int // the first line and how many
	shift        int // cells, right if positive
}

type vhstape struct {
	text     Text
	accent   lipgloss.Color
	over     time.Duration
	glitches []glitch
	snow     []int // the frames of whole-picture snow
}

func (v *vhstape) Done(t time.Duration) bool { return t >= v.over }

// snowCell is a cell of snow on a frame: the glyph and the grey by the
// hash of where and when.
func snowCell(cv *Canvas, x, y, f int) {
	h := hash(x, y, f)
	cv.Set(x, y, pick(snowGlyphs, h), snowColours[(h>>8)%uint64(len(snowColours))])
}

func (v *vhstape) Frame(t time.Duration, w, h int) []string { return frame(v, t, w, h) }

func (v *vhstape) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if t >= v.over {
		drawText(cv, v.text, v.accent)
		return
	}
	f := frames(t)
	ox, oy := v.text.Origin(w, h)
	H := v.text.Height()
	glitching, snowing := v.over*55/100, v.over*3/4
	switch {
	case t < glitching:
		// The lines as they stand, then the torn ones over them.
		shift := make([]int, H)
		colour := make([]lipgloss.Color, H)
		for i := range colour {
			colour[i] = v.accent
		}
		for _, g := range v.glitches {
			if f < g.from || f >= g.from+g.frames {
				continue
			}
			for l := g.line; l < g.line+g.lines && l < H; l++ {
				shift[l] = g.shift
				colour[l] = tapeColours[(f-g.from)%len(tapeColours)]
			}
		}
		for _, sf := range v.snow {
			if sf == f {
				for _, c := range v.text.Cells() {
					snowCell(cv, ox+c.X, oy+c.Y, f)
				}
				return
			}
		}
		for _, c := range v.text.Cells() {
			cv.Set(ox+c.X+shift[c.Y], oy+c.Y, c.R, colour[c.Y])
		}
		// The torn ends: snow where a shoved line came from and where
		// it went, a couple of cells each.
		for y := 0; y < H; y++ {
			if shift[y] == 0 {
				continue
			}
			row := v.text.rows[y]
			first, last := 0, len(row)-1
			for first < len(row) && row[first] == ' ' {
				first++
			}
			for last >= 0 && row[last] == ' ' {
				last--
			}
			if first > last {
				continue
			}
			for k := 0; k < tapeSnowEdge; k++ {
				if shift[y] > 0 {
					snowCell(cv, ox+first+k, oy+y, f)
					snowCell(cv, ox+last+shift[y]+1+k, oy+y, f)
				} else {
					snowCell(cv, ox+last-k, oy+y, f)
					snowCell(cv, ox+first+shift[y]-1-k, oy+y, f)
				}
			}
		}
	case t < snowing:
		for _, c := range v.text.Cells() {
			snowCell(cv, ox+c.X, oy+c.Y, f)
		}
	default:
		// Redrawn a line at a time from the bottom; the rest still snow.
		drawn := int(float64(H) * at(t, snowing, v.over-snowing))
		for _, c := range v.text.Cells() {
			if H-1-c.Y < drawn {
				cv.Set(ox+c.X, oy+c.Y, c.R, v.accent)
			} else {
				snowCell(cv, ox+c.X, oy+c.Y, f)
			}
		}
	}
}
