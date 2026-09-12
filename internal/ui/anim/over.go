package anim

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The ending's scenes (#156): one a cause of game.Ending, each
// OverLength long, played inside the GAME OVER modal before the run's
// summary. Each is a Sequence (#157's) of the set's effects over a text the
// model reads off the world (the file's page count, the last headline,
// the run's cash lines) and dice off Seed(seed, day, "over"); the
// summary that follows is read off the world, never off the scene. The
// model's causeScene is the switch, with Arrested the default for a
// cause without a scene of its own (#49's exits, as they land).

// OverLength is how long every ending's scene runs.
const OverLength = 1500 * time.Millisecond

// The steps' shares of the length.
const (
	overPrint = 900 * time.Millisecond  // indicted: the file's pages print
	overStamp = 600 * time.Millisecond  // indicted: the headline slides across it
	overFall  = 400 * time.Millisecond  // arrested: the bars fall, and later lift
	overWord  = 700 * time.Millisecond  // arrested: the word on the tape
	overLoss  = 1000 * time.Millisecond // broke: the figures fall off the bottom
	overZero  = 500 * time.Millisecond  // broke: the nought rains in
)

// OverPagesMax is the most pages the DA's file prints: the file is
// Heat.Evidence deep, and a canvas of fifteen rows (the modal's at
// 80x24) holds this many with the stamp across them.
const OverPagesMax = 12

// Indicted is the DA's file: pages printed one under another in
// theme.Dim by the print head, `page 1` to `page n` (n the evidence,
// at least one and at most OverPagesMax), then the headline slid in
// across the file in theme.Heat from one side or the other, on a row
// of the file's middle, both off the dice.
func Indicted(pages int, headline string, rng *rand.Rand) Scene {
	pages = max(1, min(pages, OverPagesMax))
	var file []string
	for i := 1; i <= pages; i++ {
		file = append(file, fmt.Sprintf("page %d", i))
	}
	text := NewText(strings.Join(file, "\n"))
	// The stamp: the headline on one row of a text the file's height,
	// blank rows (a space each, so NewText keeps them) above and below,
	// so it lands across the middle of the file rather than over the
	// last page printed.
	row := 0
	if pages >= 3 {
		row = 1 + rng.IntN(pages-2)
	}
	rows := make([]string, pages)
	for i := range rows {
		rows[i] = " "
	}
	rows[row] = headline
	side := Right
	if rng.IntN(2) == 1 {
		side = Left
	}
	stamp := SlideFrom(side)(NewText(strings.Join(rows, "\n")), theme.Heat, overStamp, rng)
	// The page under the stamp goes: a stamp covers what it lands on,
	// its spaces too, and a text's blanks are no cells.
	under := append([]string(nil), file...)
	under[row] = " "
	return Sequence(
		Step{Print(text, theme.Dim, overPrint, rng), overPrint},
		Step{Layer(Still(NewText(strings.Join(under, "\n")), theme.Dim), stamp), overStamp},
	)
}

// Arrested is the red bars: the curtain falls over the frame in
// theme.Heat, the word ARRESTED plays on a bad tape in theme.Text over
// the bars held, then the bars lift from the bottom with the word
// standing, and the summary follows.
func Arrested(rng *rand.Rand) Scene {
	bars := Curtain(Text{}, theme.Heat, overFall, rng)
	word := NewText(Arrest)
	return Sequence(
		Step{bars, overFall},
		Step{Layer(Held(bars, overFall), Vhstape(word, theme.Text, overWord, rng)), overWord},
		Step{Layer(Reverse(Curtain)(Text{}, theme.Heat, overFall, rng), Still(word, theme.Text)), overFall},
	)
}

// Broke is the till empty: the run's cash lines, and the crew's names
// under them, stand in theme.Money and fall off the bottom of the
// frame (a pour from below, reversed), then the nought rains in and
// settles in theme.Heat. figures is the lines, one a row.
func Broke(figures string, rng *rand.Rand) Scene {
	return Sequence(
		Step{Reverse(PourFrom(Up))(NewText(figures), theme.Money, overLoss, rng), overLoss},
		Step{Rain(NewText(Zero), theme.Heat, overZero, rng), overZero},
	)
}

// Arrest is the word's block art in the title's hand: six rows, sixty
// columns.
const Arrest = `
 ████   █████   █████   █████   █████  ██████  █████  █████
██  ██  ██  ██  ██  ██  ██     ██        ██    ██     ██  ██
██████  █████   █████   ████    ████     ██    ████   ██  ██
██  ██  ██ ██   ██ ██   ██         ██    ██    ██     ██  ██
██  ██  ██  ██  ██  ██  ██         ██    ██    ██     ██  ██
██  ██  ██  ██  ██  ██  █████  █████     ██    █████  █████
`

// Zero is the nought's block art: seven rows, fourteen columns.
const Zero = `
  ██     ████
 █████  ██  ██
██      ██  ██
 ████   ██  ██
    ██  ██  ██
█████   ██  ██
  ██     ████
`

// The registry's samples: what Scenes plays each ending over, for the
// review tool and the guards, where there is no world to read.
const (
	sampleHeadline = "Indictment lands in Ridgeport: 'we got our man', says DA"
	sampleFigures  = "peak cash         $84,930\n" +
		"total revenue    $212,400\n" +
		"wages             $61,200\n" +
		"washed            $18,000\n" +
		"clean cash             $0\n" +
		" \n" +
		"Marco  Dee  Little Ray  Tiny"
)
