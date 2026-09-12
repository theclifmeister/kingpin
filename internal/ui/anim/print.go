package anim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Print is TTE's print (NOTICE; ttfx's src/effects/print_effect.rs is
// the reference): a print head writes the text a line at a time on the
// bottom line of the text's box, a cell typed in as the four blocks
// before its character, and between lines feeds the page up a row and
// returns the head, `█` in theme.Text, to the next line's first column
// with in-out-quad easing. The typed blocks are theme.Text and the
// characters the accent. The shape is the original's; the timing is
// scaled to the length: every column the head passes is a unit, a
// carriage return a third of a unit a cell, and the units share the
// length less a tail for the last cell's blocks to clear.
//
// No dice: the head does what the text says.
func Print(text Text, accent lipgloss.Color, over time.Duration, rng *rand.Rand) Scene {
	over = length(over)
	p := &print{text: text, accent: accent, over: over, rows: make([]printRow, text.Height())}
	for y := range p.rows {
		l, r := -1, -1
		for x, c := range text.rows[y] {
			if c != ' ' {
				if l < 0 {
					l = x
				}
				r = x
			}
		}
		p.rows[y] = printRow{left: l, right: r}
	}
	units := 0.0
	for y := range p.rows {
		if p.rows[y].left >= 0 {
			units += float64(p.rows[y].right - p.rows[y].left + 1)
		}
		if y+1 < len(p.rows) {
			p.rows[y].cr = p.crUnits(y)
			units += p.rows[y].cr
		}
	}
	// The typing runs over nine tenths of the length; the tail is for
	// the last cell's blocks (and is where a short length overflows,
	// which the frame's end clamp covers).
	p.unit = time.Duration(float64(over) * 0.9 / math.Max(1, units))
	p.hold = max(Frame, min(3*Frame, p.unit))
	at := time.Duration(0)
	for y := range p.rows {
		row := &p.rows[y]
		row.typeStart = at
		if row.left >= 0 {
			at += time.Duration(row.right-row.left+1) * p.unit
		}
		row.typeEnd = at
		at += time.Duration(row.cr * float64(p.unit))
		row.crEnd = at
	}
	return p
}

// crUnits is the cost of the carriage return after row y: a third of
// a unit a cell from one past the row's last column back to the next
// row's first, and two units at least, the line feed's pause.
func (p *print) crUnits(y int) float64 {
	from, to := p.rows[y].right+1, p.rows[y+1].left
	if p.rows[y].left < 0 {
		from = 0
	}
	if to < 0 {
		to = 0
	}
	return math.Max(2, math.Abs(float64(from-to))/3)
}

// printBlocks are the glyphs a cell is typed in with, in order, before
// its character: the original's typing scene.
var printBlocks = []rune{'█', '▓', '▒', '░'}

type printRow struct {
	left, right int // the first and last typed column; -1 for a blank row
	cr          float64
	typeStart   time.Duration // the head reaches the row's first column
	typeEnd     time.Duration // the last cell is typed; the line feed and the return start
	crEnd       time.Duration // the head is on the next row's first column
}

type print struct {
	text   Text
	accent lipgloss.Color
	over   time.Duration
	rows   []printRow
	unit   time.Duration // the time the head spends on one column
	hold   time.Duration // how long each typing block shows
}

func (p *print) Done(t time.Duration) bool { return t >= p.over }

// current is the row being typed at t, or, during a carriage return,
// the row about to be: the page has already fed up for it.
func (p *print) current(t time.Duration) int {
	cur := 0
	for i := range p.rows {
		if t >= p.rows[i].typeEnd && i+1 < len(p.rows) {
			cur = i + 1
		}
	}
	return cur
}

func (p *print) Frame(t time.Duration, w, h int) []string { return frame(p, t, w, h) }

func (p *print) paint(cv *Canvas, t time.Duration) {
	w, h := cv.W, cv.H
	if t >= p.over {
		drawText(cv, p.text, p.accent)
		return
	}
	ox, oy := p.text.Origin(w, h)
	H := len(p.rows)
	cur := p.current(t)
	bottom := oy + H - 1
	for y := 0; y <= cur && y < H; y++ {
		row := p.rows[y]
		if row.left < 0 {
			continue
		}
		line := bottom - (cur - y)
		for x := row.left; x <= row.right; x++ {
			r := p.text.rows[y][x]
			if r == ' ' {
				continue
			}
			typed := row.typeStart + time.Duration(x-row.left)*p.unit
			if t < typed {
				break
			}
			step := int((t - typed) / p.hold)
			if step < len(printBlocks) {
				cv.Set(ox+x, line, printBlocks[step], theme.Text)
			} else {
				cv.Set(ox+x, line, r, p.accent)
			}
		}
	}
	// The head, during a carriage return: from one past the finished
	// row's last column to the next row's first, on the bottom line.
	if cur > 0 {
		prev := p.rows[cur-1]
		if t >= prev.typeEnd && t < prev.crEnd {
			from, to := prev.right+1, p.rows[cur].left
			if prev.left < 0 {
				from = 0
			}
			if to < 0 {
				to = 0
			}
			q := InOutQuad(at(t, prev.typeEnd, prev.crEnd-prev.typeEnd))
			x := int(math.Round(float64(from) + float64(to-from)*q))
			cv.Set(ox+x, bottom, '█', theme.Text)
		}
	}
}
