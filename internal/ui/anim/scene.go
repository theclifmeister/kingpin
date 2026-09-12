// Package anim is the game's animation: scenes, short and skippable,
// drawn through the model's own View and driven by a tea.Tick that
// exists only while one is on screen (#152, from the investigation in
// #150). A scene is a pure function of time over a seed of its own: it
// reads the world and never writes it, so a run with animation on and
// one with it off are the same run. The effects are ports of
// TerminalTextEffects (NOTICE), each a function of the text, the accent
// and the dice, coloured only through theme.
//
// The package imports theme and nothing else of ui; the model imports it.
package anim

import "time"

// Frame is the tick between two frames: 30 a second.
const Frame = time.Second / 30

// Scene is one animation: Frame is the picture at t, h lines of at most
// w cells each, and Done says when it is over. Both are pure functions
// of t, so a frame asked for twice is the same frame and a scene can be
// rendered at any size at any time (TestScenesFit walks t = 0, half and
// the end at every common size).
type Scene interface {
	Frame(t time.Duration, w, h int) []string
	Done(t time.Duration) bool
}

// Named is a scene in the registry: its name and how to make it on a
// seed, so a review tool and TestScenesFit can walk every scene the
// game has without knowing what each needs; Dice says the scene throws
// them, so another seed renders other frames (TestSceneIsDeterministic
// wants it of every scene that does; the morning's slide and wipe
// throw none, #159).
type Named struct {
	Name string
	New  func(seed uint64) Scene
	Dice bool
}

// Scenes is the registry, every scene the game plays: the title, its
// first pass on the seed, whichever effect the pass picks, and the
// interstitials as they land (#161): the stage (#157), the card (#154)
// over a sample card, the ending's three (#156), one a cause, and the
// morning (#159).
func Scenes() []Named {
	return []Named{
		{Name: "title", New: func(seed uint64) Scene { s, _ := TitlePass(seed, 0, "", ""); return s }, Dice: true},
		{Name: "stage", New: stageScene, Dice: true},
		{Name: "card", New: func(seed uint64) Scene { return Card(sampleTitle, sampleText, Seed(seed, 0, "card")) }, Dice: true},
		// The ending's three (#156), over the samples: seven pages and
		// a headline, the word, the figures.
		{Name: "over:indicted", New: func(seed uint64) Scene { return Indicted(7, sampleHeadline, Seed(seed, 1, "over")) }, Dice: true},
		{Name: "over:arrested", New: func(seed uint64) Scene { return Arrested(Seed(seed, 1, "over")) }, Dice: true},
		{Name: "over:broke", New: func(seed uint64) Scene { return Broke(sampleFigures, Seed(seed, 1, "over")) }, Dice: true},
		{Name: "morning", New: morningScene},
	}
}
