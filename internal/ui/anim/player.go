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
// An interstitial (the card, the ending, the strike) has no Next: it
// ends on Done or on any key. An idle loop (the title's) is Idle, with
// a Rest and a Next: it plays behind the menu, keys act on the menu,
// and when the scene is done and has rested Next gives the next pass.
// A hold (#203: the report's scenes) is Hold: past Done the player
// ticks on and draws the scene's frames past its end, its quiet loop
// (Holding), and keys act on the modal it holds; a key before Done
// resolves it (Skip) rather than ending it, and the loop ends with the
// modal, which stops the player.
type Player struct {
	Scene  Scene
	Accent lipgloss.Color
	Idle   bool                 // runs behind a menu: keys act on the menu and the loop plays on
	Hold   bool                 // holds a modal: past Done the loop plays on and keys act on the modal (#203)
	Rest   time.Duration        // for a loop: how long the finished scene holds before the next pass
	Next   func(pass int) Scene // for a loop: the scene of the next pass (1 the second); nil ends the player after the rest

	start  time.Time
	t      time.Duration
	offset time.Duration // what Skip added to the clock
	done   time.Duration // the clock at the first tick that found the scene done
	pass   int
}

// Tick advances the clock to the time a frame's tick fired and reports
// whether the player is over: an interstitial once its scene is done, a
// loop once Next gives no scene, a hold never (the modal ends it). A
// loop past its scene's end and its rest moves to the next pass, the
// clock restarting from now.
func (p *Player) Tick(now time.Time) (over bool) {
	if p.start.IsZero() {
		p.start = now
	}
	p.t = now.Sub(p.start) + p.offset
	if !p.Scene.Done(p.t) {
		return false
	}
	if p.Next == nil {
		return !p.Hold
	}
	if p.done == 0 {
		p.done = p.t
	}
	if p.t < p.done+p.Rest {
		return false
	}
	p.pass++
	p.Scene = p.Next(p.pass)
	p.start, p.t, p.offset, p.done = now, 0, 0, 0
	return p.Scene == nil
}

// Holding reports whether a hold is past its scene's end: the loop is
// what is drawn, and a key is the modal's.
func (p *Player) Holding() bool { return p.Hold && p.Scene.Done(p.t) }

// Skip resolves the scene: the clock jumps to the first frame the
// scene is done at, so a hold draws its loop from the next tick (a
// key mid-scene on the report skips to the resolved report, and the
// loop plays on it), and an interstitial is over on its next tick. A
// scene never done within skipLimit is left where it is.
func (p *Player) Skip() {
	at := p.t
	for !p.Scene.Done(at) && at < p.t+skipLimit {
		at += Frame
	}
	p.offset += at - p.t
	p.t = at
}

// skipLimit is how far Skip looks for a scene's end.
const skipLimit = 30 * time.Second

// T is the clock: how far into the scene the last tick was.
func (p *Player) T() time.Duration { return p.t }

// Pass is how many times a loop has restarted.
func (p *Player) Pass() int { return p.pass }

// Frame is the scene's picture at the clock, h lines of at most w cells.
func (p *Player) Frame(w, h int) []string { return p.Scene.Frame(p.t, w, h) }
