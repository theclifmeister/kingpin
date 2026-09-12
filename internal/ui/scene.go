package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// A scene (#152) is a short, skippable animation drawn through View and
// driven by a tea.Tick that exists only while one is on screen: the one
// change to the rule that the UI redraws on a key or a resize. Model.scene
// is the anim.Player of the scene up now, nil otherwise; the chain of
// ticks starts with the command of the Update that put it there and
// ends by itself once it is nil, whether the scene ran out (Done) or a
// key ended it. Init still returns nil: the start menu's loop begins on
// the first WindowSizeMsg, inside Update. Never a tick in play mode,
// never inside a fast-forward's loop, never in the harness (it does not
// import ui), never in the test fixtures and the README's captures
// (they construct with Options{Anim: false}).

// Options is how the front end is set up: Anim plays the scenes;
// cmd/kingpin turns it off for -no-anim and KINGPIN_NO_ANIM, and every
// test fixture constructs with it off. Effect pins the title loop's
// effect by name (#153; KINGPIN_ANIM_EFFECT, for review); empty cycles
// the set.
type Options struct {
	Anim   bool
	Effect string
}

// frameMsg is a frame's tick: when it fired and which scene it was
// issued for, so a tick outstanding when its scene ended is dropped.
type frameMsg struct {
	at  time.Time
	gen int
}

// play puts a scene on screen: the tick chain starts with the command
// of the Update this is called in.
func (m *Model) play(p *anim.Player) {
	m.scene = p
	m.sceneGen++
	m.ticking = false
}

// stop takes the scene off: a tick on its way lands on the old
// generation and is dropped, and no new one is issued.
func (m *Model) stop() { m.scene = nil }

// tick is the command every Update ends with: the next frame's tick
// while a scene is up and none is on its way, else nil.
func (m *Model) tick() tea.Cmd {
	if m.scene == nil || m.ticking {
		return nil
	}
	m.ticking = true
	gen := m.sceneGen
	return tea.Tick(anim.Frame, func(t time.Time) tea.Msg { return frameMsg{at: t, gen: gen} })
}

// onFrame is a tick landing: the scene's clock moves, an interstitial
// that has run out comes off, and the next tick is issued.
func (m *Model) onFrame(f frameMsg) tea.Cmd {
	if m.scene == nil || f.gen != m.sceneGen {
		return nil
	}
	m.ticking = false
	if m.scene.Tick(f.at) {
		m.stop()
		return nil
	}
	return m.tick()
}

// skip is any key while an interstitial is up: the scene ends and the
// key is consumed, so the mode's own modal is what the next key acts
// on. An idle loop (the title's) plays on and the key falls through.
func (m *Model) skip() bool {
	if m.scene == nil || m.scene.Idle {
		return false
	}
	m.stop()
	return true
}

// titleLoop starts or stops the start menu's idle loop for the frame:
// on while animation is on, the menu is up and the terminal is 80x24
// or more, the art resolved over anim.TitleLength by an effect of the
// set picked off the pass's seed, never the one before's (#153;
// Options.Effect pins one), resting anim.TitleRest, then the next
// pass; off otherwise, so under 80x24 or with animation off the menu
// draws as it always did.
func (m *Model) titleLoop() {
	on := m.opts.Anim && m.onStart() && m.titleFits()
	switch {
	case on && m.scene == nil:
		// The first pass avoids the effect a stopped loop last played
		// (a resize across the floor restarts it), so no effect plays
		// twice running even then.
		var first anim.Scene
		first, m.titleEffect = anim.TitlePass(0, 0, m.titleEffect, m.opts.Effect)
		m.play(&anim.Player{
			Scene:  first,
			Accent: theme.Money,
			Idle:   true,
			Rest:   anim.TitleRest,
			Next: func(pass int) anim.Scene {
				var s anim.Scene
				s, m.titleEffect = anim.TitlePass(0, pass, m.titleEffect, m.opts.Effect)
				return s
			},
		})
	case !on && m.scene != nil && m.scene.Idle:
		m.stop()
	}
}

// onStart reports whether the start menu is what View draws.
func (m *Model) onStart() bool {
	return m.mode == modeStart || m.mode == modeConfirmDelete || m.w == nil
}

// titleFits reports whether the terminal has room for the art over the
// box: 80x24, the game's floor.
func (m *Model) titleFits() bool { return m.width >= 80 && m.height >= 24 }

// titleArt is the art's rows above the start menu's box while the loop
// runs, the scene's frame at the terminal's width, or nothing.
func (m *Model) titleArt() []string {
	if m.scene == nil || !m.scene.Idle || !m.onStart() {
		return nil
	}
	return m.scene.Frame(m.width, anim.NewText(anim.Kingpin).Height())
}
