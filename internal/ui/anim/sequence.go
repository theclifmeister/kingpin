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
	at := time.Duration(0)
	for i, st := range s.steps {
		if t < at+st.Over || i == len(s.steps)-1 {
			return st.Scene.Frame(min(t-at, st.Over), w, h)
		}
		at += st.Over
	}
	return NewCanvas(w, h).Lines()
}
