package anim

import "time"

// Layer draws scenes over one another on one canvas (#156): the first
// is the ground and each after it is painted on top, a cell it sets
// covering the one under it, so a word plays over a curtain and a
// stamp lands across a printed page. Every layer runs on the same
// clock, and the layer is done when all of them are. It composes at
// the cell level because every effect is a painter underneath
// (effects.go); a scene from outside the package is drawn by its rows.
func Layer(scenes ...Scene) Scene { return layer(scenes) }

type layer []Scene

func (l layer) Done(t time.Duration) bool {
	for _, s := range l {
		if !s.Done(t) {
			return false
		}
	}
	return true
}

func (l layer) Frame(t time.Duration, w, h int) []string { return frame(l, t, w, h) }

func (l layer) paint(cv *Canvas, t time.Duration) {
	for _, s := range l {
		paint(s, cv, t)
	}
}

// Held is a scene's frame at one moment, held: done from t = 0, the way
// a Still is, so a curtain that has fallen stays down under the next
// step of a sequence.
func Held(s Scene, at time.Duration) Scene { return held{s, at} }

type held struct {
	s  Scene
	at time.Duration
}

func (h held) Done(time.Duration) bool                   { return true }
func (h held) Frame(t time.Duration, w, hh int) []string { return frame(h, t, w, hh) }
func (h held) paint(cv *Canvas, _ time.Duration)         { paint(h.s, cv, h.at) }
