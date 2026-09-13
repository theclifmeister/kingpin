package anim

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// A hold (#203) is what a report's scene does once it has resolved:
// its last frame stays up with a quiet loop on it until the report is
// closed, so what it said (STING, what was lost, the day, the weather)
// can be read at the player's own pace. The loop is built from two
// shapes below, each a painter that composes region by region as the
// scenes do, and slow enough not to make the report harder to read:
// Replay plays an effect again every HoldRest and rests on its
// resolved frame between passes, Pulse turns a text a colour for a
// moment of every period. Neither is ever done in the sense a scene
// is: they are the resting state of one that has, and their Done is a
// Held's, true from t = 0, so the scene's own Done stays at its
// length (Named.Length; TestSceneLengths) and the registry's Hold says
// the frames past it move.

// HoldRest is how long a held loop rests before each pass of its
// effect: the tape's jitter on the bust's line, the wipe over the
// morning's title, the beams over the incident's line.
const HoldRest = 3 * time.Second

// HoldPulse is the period of a held pulse and holdPulseOn how long of
// it the pulse colour shows: the strobe colour on the bust's title,
// once a second, for less than a third of it.
const (
	HoldPulse   = time.Second
	holdPulseOn = 300 * time.Millisecond
)

// Replay plays a scene over again: it rests on the scene's frame at
// its length for `rest`, plays the scene from its start over `over`,
// and rests again, so the first pass begins one rest in. Frame at t is
// a pure function of t, as every scene's is.
func Replay(s Scene, over, rest time.Duration) Scene {
	return &replay{s: s, over: length(over), rest: max(0, rest)}
}

type replay struct {
	s          Scene
	over, rest time.Duration
}

func (r *replay) Done(time.Duration) bool                  { return true }
func (r *replay) Frame(t time.Duration, w, h int) []string { return frame(r, t, w, h) }

func (r *replay) paint(cv *Canvas, t time.Duration) {
	if at := t % (r.rest + r.over); at >= r.rest {
		paint(r.s, cv, at-r.rest)
		return
	}
	paint(r.s, cv, r.over)
}

// Pulse is a text in the accent that turns the pulse colour for `on`
// at the end of every period: the bust's strobe, slowed to a
// heartbeat. No motion in the characters: the text stands, so it
// reads as a still under a colour profile with no colour.
func Pulse(text Text, accent, pulse lipgloss.Color, on, every time.Duration) Scene {
	return &pulsed{text: text, accent: accent, pulse: pulse, on: on, every: max(length(every), length(on))}
}

type pulsed struct {
	text          Text
	accent, pulse lipgloss.Color
	on, every     time.Duration
}

func (p *pulsed) Done(time.Duration) bool                  { return true }
func (p *pulsed) Frame(t time.Duration, w, h int) []string { return frame(p, t, w, h) }

func (p *pulsed) paint(cv *Canvas, t time.Duration) {
	col := p.accent
	if t%p.every >= p.every-p.on {
		col = p.pulse
	}
	drawText(cv, p.text, col)
}
