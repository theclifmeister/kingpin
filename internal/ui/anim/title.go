package anim

import (
	"math/rand/v2"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Kingpin is the title's block art: six rows, fifty columns, drawn in
// gold above the start menu's box (#152) and resolved by an effect on a
// loop, the way Omarchy's screensaver loops effects over its logo.
const Kingpin = `
██  ██  ████  ██  ██   █████  █████   ████  ██  ██
██ ██    ██   ███ ██  ██      ██  ██   ██   ███ ██
████     ██   ██████  ██ ███  █████    ██   ██████
██ ██    ██   ██ ███  ██  ██  ██       ██   ██ ███
██  ██   ██   ██  ██  ██  ██  ██       ██   ██  ██
██  ██  ████  ██  ██   █████  ██      ████  ██  ██
`

// TitleLength is how long one pass of the title takes to resolve, and
// TitleRest how long the resolved art rests before the next pass.
const (
	TitleLength = 1500 * time.Millisecond
	TitleRest   = 4 * time.Second
)

// Title is one pass of the title: the art resolved in gold by the named
// effect on the dice given (an unknown name is decrypt, the first).
func Title(name string, rng *rand.Rand) Scene {
	e, ok := Effects[name]
	if !ok || !e.Needs.Text {
		e = Effects["decrypt"]
	}
	return e.New(NewText(Kingpin), theme.Money, TitleLength, rng)
}

// TitleEffects is the set the title cycles: every effect that resolves
// a text, in Names' order.
func TitleEffects() []string {
	var names []string
	for _, n := range Names() {
		if Effects[n].Needs.Text {
			names = append(names, n)
		}
	}
	return names
}

// TitlePass is one pass of the start menu's loop (#153): the effect
// picked off the pass's own dice, Seed(seed, pass, "title"), from
// TitleEffects less the pass before's, so no effect plays twice
// running; a pinned name (KINGPIN_ANIM_EFFECT) plays every pass. The
// scene runs on the same stream after the pick. It returns the scene
// and the name, for the next pass to avoid.
func TitlePass(seed uint64, pass int, prev, pinned string) (Scene, string) {
	rng := Seed(seed, pass, "title")
	name := pinned
	if _, ok := Effects[pinned]; !ok || !Effects[pinned].Needs.Text {
		var names []string
		for _, n := range TitleEffects() {
			if n != prev {
				names = append(names, n)
			}
		}
		name = names[rng.IntN(len(names))]
	}
	return Title(name, rng), name
}
