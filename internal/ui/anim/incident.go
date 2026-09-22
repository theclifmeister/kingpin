package anim

import (
	"math/rand/v2"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Incident is the weather (#203; the INCIDENT section of #44): the
// morning the report opens with an incident and no bust, its line
// prints in on the report's own row in theme.World over
// IncidentLength, and holds: a slow pass of beams over the line every
// HoldRest, until the report closes. The frame is one row, the line at
// the report's indent at the width given, painted on a canvas of its
// own with the beams' margin either side (a beam runs in through the
// indent and out at the frame's edge) and blitted into place. The held
// pass is the beams layered over the line standing (Beams alone lights
// the text as it crosses it, from nothing; here the line stays and the
// beams dim what they cross until the sheen restores it). The dice are
// the beams'; the print throws none.
func Incident(line string, rng *rand.Rand) Scene {
	text := NewText(line)
	return &incident{
		print: Print(text, theme.World, IncidentLength, rng),
		beams: Replay(Layer(Still(text, theme.World), Play(Effects["beams"], text, theme.World, incidentBeams, rng)), incidentBeams, HoldRest),
		width: text.Width(),
	}
}

// IncidentLength is how long the incident's scene runs before it
// holds; incidentBeams how long each held pass of the beams takes.
const (
	IncidentLength = 800 * time.Millisecond
	incidentBeams  = 1500 * time.Millisecond
)

type incident struct {
	print Scene // the line printing in
	beams Scene // the line while held: the beams every HoldRest
	width int   // the line's width
}

func (s *incident) Done(t time.Duration) bool { return t >= IncidentLength }

func (s *incident) Frame(t time.Duration, w, h int) []string { return frame(s, t, w, h) }

func (s *incident) paint(cv *Canvas, t time.Duration) {
	line := NewCanvas(s.width+2*beamMargin, 1)
	if t < IncidentLength {
		paint(s.print, line, t)
	} else {
		paint(s.beams, line, t-IncidentLength)
	}
	blit(cv, line, bustIndent-beamMargin, 0)
}

// incidentScene is the registry's incident: a hurricane on the sample
// line.
func incidentScene(seed uint64) Scene {
	return Incident("A hurricane shuts the boat routes out of Bayport for 4 nights.", Seed(seed, 0, "incident"))
}
