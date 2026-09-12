package anim

import (
	"math/rand/v2"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Bust is a sting, a raid or an arrest hitting (#155): the morning the
// report opens with an Enforcement past a patrol, the report's title
// strobes theme.Heat twice, the level (STING, RAID, ARRESTED) glitches
// into place on a bad tape at the top of the report, and the stock and
// cash lost burn along their line, BustLength in all, before the report
// reads as usual. A raid that went to the stash (somebody talked)
// glitches longer, and for one frame the level is in the rival's
// purple: the tell, for a player who reads it. The frame is two rows,
// laid out as the report's modal draws them: row 0 the title row, at
// the title's width; row 1 the bust's line at the width given, the
// level at the report's indent and the loss after it, each painted on
// a canvas of its own (the tape shoves its line into a margin either
// side of the word, cut at the frame's edge) and blitted into place.
// The dice are the tape's and the burn's; the strobe is a Sequence of
// Stills, each Held its step.
func Bust(title, level, loss string, stash bool, rng *rand.Rand) Scene {
	glitch, burnAt := bustGlitch, bustBurnAt
	if stash {
		glitch, burnAt = bustGlitchStash, bustBurnAtStash
	}
	head, word, rest := NewText(title), NewText(level), NewText(loss)
	b := &bust{
		title: Sequence(
			Step{Still(head, theme.Heat), bustStrobe},
			Step{Still(head, theme.Money), bustStrobe},
			Step{Still(head, theme.Heat), bustStrobe},
			Step{Still(head, theme.Money), bustStrobe},
		),
		titleW: head.Width(),
		level:  Vhstape(word, theme.Heat, glitch, rng),
		levelW: word.Width(),
		loss:   Burn(rest, theme.Heat, BustLength-burnAt, rng),
		lossW:  rest.Width(),
		burnAt: burnAt,
		purple: -1,
	}
	if stash {
		b.purple = frames(bustGlitchStash / 3)
	}
	return b
}

// BustLength is how long the bust's scene runs.
const BustLength = 1200 * time.Millisecond

// The steps' shares of the length.
const (
	bustStrobe      = 75 * time.Millisecond  // a strobe step: red, gold, red, gold, and the gold held
	bustGlitch      = 600 * time.Millisecond // the level's tape
	bustGlitchStash = 900 * time.Millisecond // longer when somebody talked
	bustBurnAt      = 500 * time.Millisecond // when the loss starts burning
	bustBurnAtStash = 800 * time.Millisecond
)

// bustIndent is the report's indent, `  `, the level's column; the
// tape's margin is what a shove and its snow can take either side of
// the word.
const (
	bustIndent = 2
	bustMargin = 6
)

type bust struct {
	title  Scene // the strobe
	titleW int
	level  Scene // the word on the tape
	levelW int
	loss   Scene // the line's rest, burning
	lossW  int
	burnAt time.Duration
	purple int // the frame the level shows in theme.Rivals (a stash raid, a third into its tape), or -1
}

func (b *bust) Done(t time.Duration) bool { return t >= BustLength }

func (b *bust) Frame(t time.Duration, w, h int) []string { return frame(b, t, w, h) }

func (b *bust) paint(cv *Canvas, t time.Duration) {
	title := NewCanvas(min(cv.W, b.titleW), 1)
	paint(b.title, title, t)
	blit(cv, title, 0, 0)
	level := NewCanvas(b.levelW+2*bustMargin, 1)
	paint(b.level, level, t)
	if frames(t) == b.purple {
		for i := range level.cols {
			if level.runes[i] != 0 {
				level.cols[i] = theme.Rivals
			}
		}
	}
	blit(cv, level, bustIndent-bustMargin, 1)
	if t >= b.burnAt {
		loss := NewCanvas(b.lossW, 1)
		paint(b.loss, loss, t-b.burnAt)
		blit(cv, loss, bustIndent+b.levelW, 1)
	}
}

// blit copies a canvas's set cells onto another at an offset; a cell
// landing outside the destination is dropped, as Set drops it.
func blit(dst, src *Canvas, dx, dy int) {
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			if r := src.runes[y*src.W+x]; r != 0 {
				dst.Set(dx+x, dy+y, r, src.cols[y*src.W+x])
			}
		}
	}
}

// bustScene is the registry's bust: a raid on the sample line.
func bustScene(seed uint64) Scene {
	return Bust("MORNING REPORT · DAY 42", "RAID", ": lost 40 Weed and $2,000 in Eastside", false, Seed(seed, 0, "bust"))
}
