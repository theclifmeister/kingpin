package anim

import "time"

// Sequence plays scenes one after another (#156; #154 needs the same):
// each runs on a clock of its own from the moment the one before is
// done, and the sequence is done when the last is. A step's length is
// read off its Done a frame at a time when the sequence is made, so a
// step is what the effects are, a scene done at a length and holding
// after; a step never done runs for sequenceCap. Frame is the running
// step's frame, and after the end the last step's last frame, so a
// sequence holds the way an effect does.
func Sequence(scenes ...Scene) Scene {
	s := &sequence{}
	at := time.Duration(0)
	for _, sc := range scenes {
		s.steps = append(s.steps, seqStep{Scene: sc, start: at})
		at += lengthOf(sc)
	}
	s.over = at
	return s
}

// sequenceCap is the longest a step is allowed to run before the next
// starts; the game's scenes are all under two seconds.
const sequenceCap = 30 * time.Second

// lengthOf is a scene's length: the moment it is done from, found a
// frame at a time and then to the nanosecond between the last frame
// it was not and the first it was (Done never goes back), so a step of
// a second is a second and not the frame after; or the cap.
func lengthOf(s Scene) time.Duration {
	end := time.Duration(0)
	for !s.Done(end) && end < sequenceCap {
		end += Frame
	}
	if end == 0 || end >= sequenceCap {
		return end
	}
	lo, hi := end-Frame, end
	for hi-lo > 1 {
		if mid := lo + (hi-lo)/2; s.Done(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

type seqStep struct {
	Scene
	start time.Duration
}

type sequence struct {
	steps []seqStep
	over  time.Duration
}

func (s *sequence) Done(t time.Duration) bool                { return t >= s.over }
func (s *sequence) Frame(t time.Duration, w, h int) []string { return frame(s, t, w, h) }

// current is the step running at t and its own clock: the last one
// past the end, holding.
func (s *sequence) current(t time.Duration) (Scene, time.Duration) {
	if len(s.steps) == 0 {
		return nil, 0
	}
	i := len(s.steps) - 1
	for j := range s.steps {
		if j+1 < len(s.steps) && t < s.steps[j+1].start {
			i = j
			break
		}
	}
	return s.steps[i].Scene, t - s.steps[i].start
}

func (s *sequence) paint(cv *Canvas, t time.Duration) {
	if sc, at := s.current(t); sc != nil {
		paint(sc, cv, at)
	}
}

// Layer draws scenes over one another on one canvas: the first is the
// ground and each after it is painted on top, a cell it sets covering
// the one under it, so a word plays over a curtain and a stamp lands
// across a printed page (#156). Every layer runs on the same clock,
// and the layer is done when all of them are.
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
