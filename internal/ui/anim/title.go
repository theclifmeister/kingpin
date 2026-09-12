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

// Title is one pass of the title: the art decrypted in gold on the dice
// given (Seed(seed, pass, "title") for the menu's loop, a new stream a
// pass).
func Title(rng *rand.Rand) Scene {
	return Decrypt(NewText(Kingpin), theme.Money, TitleLength, rng)
}
