package anim

import (
	"strings"
	"time"
)

// Sequence plays scenes one after another on one canvas (#154): each
// Part runs for its Over from where the part before ended, drawn in a
// box of its own on the canvas, and holds its last frame once it is
// done, so the title decrypts, then the prose wipes in under it, and
// the frame at the end is every part settled. A part not yet started
// draws nothing. Done at the sum of the lengths. Like every scene, a
// pure function of t: the parts are, and the sequence only offsets
// their clocks.
//
// The card (#154), the ending (#156) and the stage (#157) are built
// from it; a part is any Scene, an effect of the set or another
// sequence.
func Sequence(parts ...Part) Scene {
	s := &sequence{parts: parts}
	for _, p := range parts {
		s.over += max(0, p.Over)
	}
	return s
}

// Part is one step of a Sequence: the scene, how long it runs before
// the next starts (the length it was made over; 0 is a still that the
// next part follows at once), and its box: X, Y the top-left cell it
// draws from, W, H its width and height, 0 the rest of the canvas. A
// box the size of the part's text puts the text's origin at the box's
// corner, which is how a line is drawn flush left.
type Part struct {
	Scene      Scene
	Over       time.Duration
	X, Y, W, H int
}

type sequence struct {
	parts []Part
	over  time.Duration
}

func (s *sequence) Done(t time.Duration) bool { return t >= s.over }

func (s *sequence) Frame(t time.Duration, w, h int) []string {
	out := make([]string, max(0, h))
	start := time.Duration(0)
	for _, p := range s.parts {
		if t < start {
			break // the parts after this one have not begun
		}
		// The box, its width cut to the canvas (a line past the right
		// edge cannot be drawn; a row past the bottom is dropped below).
		pw, ph := p.W, p.H
		if pw <= 0 || pw > w-p.X {
			pw = w - p.X
		}
		if ph <= 0 {
			ph = h - p.Y
		}
		indent := strings.Repeat(" ", max(0, p.X))
		for i, l := range p.Scene.Frame(t-start, pw, ph) {
			if y := p.Y + i; y >= 0 && y < h && l != "" {
				out[y] = indent + l
			}
		}
		start += max(0, p.Over)
	}
	return out
}
