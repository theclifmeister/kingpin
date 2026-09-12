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

// Named is a scene in the registry (#161): its name, what starts it in
// the game, the effects it draws with, how long it runs and how to
// make it on a seed, so a review tool (cmd/anim) and the guards
// (TestScenesFit, TestSceneIsDeterministic, TestSceneLengths,
// TestEveryModeWithASceneIsListed) can walk every scene the game has
// without knowing what each needs. Name is the scene's stream, the
// name its site in ui passes anim.Seed(seed, day, name), with a
// `:variant` after it where one stream plays several scenes (the
// ending's causes); Starts is the mode or the event that plays it;
// Effects the names in Effects it is built from, in the order they
// play; Length the moment Done turns true; Dice says the scene throws
// them, so another seed renders other frames (TestSceneIsDeterministic
// wants it of every scene that does; the morning's slide and wipe
// throw none, #159); With makes the scene on one of its effects by
// name, for the one scene that takes a choice (the title: its loop
// cycles the set and KINGPIN_ANIM_EFFECT pins one), nil for the rest.
type Named struct {
	Name    string
	Starts  string
	Effects []string
	Length  time.Duration
	New     func(seed uint64) Scene
	With    func(seed uint64, effect string) Scene
	Dice    bool
}

// Scenes is the registry, every scene the game plays: the title, its
// first pass on the seed, whichever effect the pass picks, and the
// interstitials as they landed: the stage (#157), the card (#154) over
// a sample card, the ending's three (#156), one a cause, the morning
// (#159) and the bust (#155) over a sample raid, and the strike (#158)
// over two sample cells.
func Scenes() []Named {
	return []Named{
		{
			Name:    "title",
			Starts:  "the start menu, on a loop (modeStart)",
			Effects: TitleEffects(),
			Length:  TitleLength,
			New:     func(seed uint64) Scene { s, _ := TitlePass(seed, 0, "", ""); return s },
			With:    func(seed uint64, effect string) Scene { s, _ := TitlePass(seed, 0, "", effect); return s },
			Dice:    true,
		},
		{
			Name:    "stage",
			Starts:  "the morning a tier is entered (modeStage)",
			Effects: []string{"print", "beams", "wipe"},
			Length:  StageLength,
			New:     stageScene,
			Dice:    true,
		},
		{
			Name:    "card",
			Starts:  "the morning a card is dealt (modeCard)",
			Effects: []string{"decrypt", "wipe"},
			Length:  CardTitleLength + CardProseLength,
			New:     func(seed uint64) Scene { return Card(sampleTitle, sampleText, Seed(seed, 0, "card")) },
			Dice:    true,
		},
		// The ending's three (#156), over the samples: seven pages and
		// a headline, the word, the figures.
		{
			Name:    "over:indicted",
			Starts:  "the morning the DA indicts (modeOver)",
			Effects: []string{"print", "slide"},
			Length:  OverLength,
			New:     func(seed uint64) Scene { return Indicted(7, sampleHeadline, Seed(seed, 1, "over")) },
			Dice:    true,
		},
		{
			Name:    "over:arrested",
			Starts:  "the morning of the arrest (modeOver)",
			Effects: []string{"curtain", "vhstape"},
			Length:  OverLength,
			New:     func(seed uint64) Scene { return Arrested(Seed(seed, 1, "over")) },
			Dice:    true,
		},
		{
			Name:    "over:broke",
			Starts:  "the morning the till is empty (modeOver)",
			Effects: []string{"pour", "rain"},
			Length:  OverLength,
			New:     func(seed uint64) Scene { return Broke(sampleFigures, Seed(seed, 1, "over")) },
			Dice:    true,
		},
		{
			Name:    "morning",
			Starts:  "the report on a plain morning (modeReport)",
			Effects: []string{"slide", "wipe"},
			Length:  MorningLength,
			New:     morningScene,
		},
		{
			Name:    "bust",
			Starts:  "the report on a sting, a raid or an arrest (modeReport)",
			Effects: []string{"vhstape", "burn"},
			Length:  BustLength,
			New:     bustScene,
			Dice:    true,
		},
		{
			Name:    "strike",
			Starts:  "the map, the morning after a corner changed hands (screenMap)",
			Effects: []string{"burn", "slide"},
			Length:  StrikeLength,
			New:     strikeScene,
			Dice:    true, // the burns' fronts; the slide throws none
		},
	}
}
