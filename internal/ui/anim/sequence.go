package anim

import "time"

// Sequence plays scenes one after another (#157; #154's card composes
// the same way): each Step is a scene and how long it runs, the next
// starting where the last ended, so a title can Print and then Beams
// over the same text. Frame at t is the frame of the step whose span
// holds t, on that step's own clock; past the last step it is the last
// step's frame at its end, held. Done at the sum of the steps.
type Step struct {
	Scene Scene
	Over  time.Duration
}

// Sequence is the steps in order; none is a Still of nothing.
func Sequence(steps ...Step) Scene {
	s := &sequence{steps: steps}
	for _, st := range steps {
		s.over += st.Over
	}
	return s
}

type sequence struct {
	steps []Step
	over  time.Duration
}

func (s *sequence) Done(t time.Duration) bool { return t >= s.over }

func (s *sequence) Frame(t time.Duration, w, h int) []string {
	if st, at := s.step(t); st != nil {
		return st.Scene.Frame(at, w, h)
	}
	return NewCanvas(w, h).Lines()
}

// paint draws the step's scene at its own clock onto the canvas given,
// so a sequence composes under a Layer or a blit (#155's strobe) as an
// effect does.
func (s *sequence) paint(cv *Canvas, t time.Duration) {
	if st, at := s.step(t); st != nil {
		paint(st.Scene, cv, at)
	}
}

// step is the step whose span holds t, on its own clock, the last one
// held past its end; nil for no steps.
func (s *sequence) step(t time.Duration) (*Step, time.Duration) {
	at := time.Duration(0)
	for i := range s.steps {
		st := &s.steps[i]
		if t < at+st.Over || i == len(s.steps)-1 {
			return st, min(t-at, st.Over)
		}
		at += st.Over
	}
	return nil, 0
}
