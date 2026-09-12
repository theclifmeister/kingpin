package anim

import (
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Strike is the scene of a corner changing hands (#158), the one scene
// on a play screen: the morning after a strike you won or a corner
// taken off you, the first time the map is shown, the corner's cell
// burns from the colour it was to the colour it is (BurnFrom: blue to
// purple when taken from you, purple to blue when won) and its name
// slides into the pane's inspector, StrikeLength in all. The frame is
// the rows the map replaces: one a flip, the corner's name cell as the
// map draws it, then the inspector's title, each on a canvas of its
// own text's width so it sits flush left in its cell; the rest of the
// map, the pane and the frame draw as they are. The dice are the
// burns' fronts; the slide throws none.
func Strike(flips []Flip, name string, accent lipgloss.Color, rng *rand.Rand) Scene {
	s := &strike{}
	for _, f := range flips {
		t := NewText(f.Cell)
		s.rows = append(s.rows, region{t, BurnFrom(f.From)(t, f.To, StrikeLength, rng)})
	}
	n := NewText(name)
	s.rows = append(s.rows, region{n, Slide(n, accent, StrikeLength, rng)})
	return s
}

// Flip is one corner in the scene: its name cell, plain, as the map
// draws it (`▴ THE YARDS`), the colour it stood in and the colour it
// burns to, the owner's now.
type Flip struct {
	Cell     string
	From, To lipgloss.Color
}

// StrikeLength is how long the cells burn and the name slides.
const StrikeLength = 800 * time.Millisecond

// region is one row of the frame: a text and the effect resolving it.
type region struct {
	text  Text
	scene Scene
}

type strike struct {
	rows []region // the flips' cells, then the name
}

func (s *strike) Done(t time.Duration) bool { return t >= StrikeLength }

func (s *strike) Frame(t time.Duration, w, h int) []string {
	out := make([]string, 0, h)
	for _, r := range s.rows {
		out = append(out, r.scene.Frame(t, min(w, r.text.Width()), 1)...)
	}
	for len(out) < h {
		out = append(out, "")
	}
	return out[:h]
}

// strikeScene is the registry's strike: two cells, one won and one
// taken, on the seed's dice.
func strikeScene(seed uint64) Scene {
	return Strike([]Flip{
		{Cell: "▪ THE YARDS", From: theme.Rivals, To: theme.Crew},
		{Cell: "▴ DOCKSIDE", From: theme.Crew, To: theme.Rivals},
	}, "THE YARDS", theme.Rivals, Seed(seed, 0, "strike"))
}
