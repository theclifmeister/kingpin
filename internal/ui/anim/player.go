package anim

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Player is the scene on screen: the scene, the accent the mode draws
// it in, the clock it runs on and what happens when it ends. The model
// holds one while a scene is up (Model.scene) and none otherwise, which
// is what the tick chain runs on. The clock is the ticks' own: Tick is
// given the time each frame's tick fired and the first sets the start,
// so a frame is never read off the wall clock and a test drives the
// player with times of its choosing.
//
// An interstitial (the card, the bust, the ending) has no Next: it ends
// on Done or on any key. An idle loop (the title's) is Idle, with a
// Rest and a Next: it plays behind the menu, keys act on the menu, and
// when the scene is done and has rested Next gives the next pass.
type Player struct {
	Scene  Scene
	Accent lipgloss.Color
	Idle   bool                 // runs behind a menu: keys act on the menu and the loop plays on
	Rest   time.Duration        // for a loop: how long the finished scene holds before the next pass
	Next   func(pass int) Scene // for a loop: the scene of the next pass (1 the second); nil ends the player after the rest

	start time.Time
	t     time.Duration
	done  time.Duration // the clock at the first tick that found the scene done
	pass  int
}

// Tick advances the clock to the time a frame's tick fired and reports
// whether the player is over: an interstitial once its scene is done, a
// loop once Next gives no scene. A loop past its scene's end and its
// rest moves to the next pass, the clock restarting from now.
func (p *Player) Tick(now time.Time) (over bool) {
	if p.start.IsZero() {
		p.start = now
	}
	p.t = now.Sub(p.start)
	if !p.Scene.Done(p.t) {
		return false
	}
	if p.Next == nil {
		return true
	}
	if p.done == 0 {
		p.done = p.t
	}
	if p.t < p.done+p.Rest {
		return false
	}
	p.pass++
	p.Scene = p.Next(p.pass)
	p.start, p.t, p.done = now, 0, 0
	return p.Scene == nil
}

// T is the clock: how far into the scene the last tick was.
func (p *Player) T() time.Duration { return p.t }

// Pass is how many times a loop has restarted.
func (p *Player) Pass() int { return p.pass }

// Frame is the scene's picture at the clock, h lines of at most w cells.
func (p *Player) Frame(w, h int) []string { return p.Scene.Frame(p.t, w, h) }
