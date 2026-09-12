package anim

import (
	"math/rand/v2"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Stage is the scene of a stage reached (#157): the interstitial's
// title, `STAGE 3 · TERRITORY`, printed in the tier's colour and then
// swept once by the beams' sheen, and the modal's body, the blurb, the
// prose, OPENED and NEXT, wiped in under it, over StageLength in all.
// The frame is the modal's rows: line 0 the title row, the rest the
// body, each region drawn on a canvas of its own text's width so both
// sit flush left as the modal draws them. The title and the body are
// the modal's own lines, handed in by the model (the body stripped of
// its styles: the wipe resolves it plain, and the modal's blurb and
// headings take their colour when the scene ends). The dice are the
// beams'; print and wipe throw none.
func Stage(title string, body []string, accent lipgloss.Color, rng *rand.Rand) Scene {
	third := StageLength / 3
	t, b := NewText(title), NewText(strings.Join(body, "\n"))
	return &stage{
		title: t,
		body:  b,
		rows:  len(body),
		head: Sequence(
			Step{Print(t, accent, third, rng), third},
			Step{Beams(t, accent, third, rng), third},
		),
		wipe: Wipe(b, "", third, rng),
		at:   2 * third,
	}
}

// StageLength is how long the stage's scene runs: a third each for the
// title's print, its sheen and the body's wipe.
const StageLength = 1200 * time.Millisecond

type stage struct {
	title Text
	body  Text
	rows  int           // the body's lines, blank ones included
	head  Scene         // the title: print, then beams
	wipe  Scene         // the body
	at    time.Duration // when the body's wipe starts
}

func (s *stage) Done(t time.Duration) bool { return t >= StageLength }

func (s *stage) Frame(t time.Duration, w, h int) []string {
	out := make([]string, 0, h)
	if h > 0 {
		out = append(out, s.head.Frame(t, min(w, s.title.Width()), 1)...)
	}
	body := make([]string, s.rows)
	if t >= s.at {
		lines := s.wipe.Frame(t-s.at, min(w, s.body.Width()), s.body.Height())
		copy(body, lines)
	}
	out = append(out, body...)
	for len(out) < h {
		out = append(out, "")
	}
	return out[:h]
}

// stageSample is what the registry's entry plays, for a review tool and
// the fit test: the shape of a stage's modal, not the file's copy (the
// game reads progression.toml's through the model).
var stageSample = []string{
	"Corners held, a crew on them and a rival on the way.",
	"The block is yours now, or near enough. The connect knows your",
	"name and the runners know the corners; what they do not know is",
	"how long you mean to keep them.",
	"",
	"OPENED",
	"  the second connect, if the street connect vouches",
	"  the laundromat, and the fronts behind it",
	"",
	"NEXT",
	"Move $500K: the wholesaler's lots and the port's product.",
}

// stageScene is the registry's stage: the sample on the seed's dice.
func stageScene(seed uint64) Scene {
	return Stage("STAGE 3 · TERRITORY", stageSample, theme.Money, Seed(seed, 0, "stage"))
}
