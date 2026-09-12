package anim

import (
	"math/rand/v2"
	"strings"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Card is the dilemma card dealt (#154), the first interstitial: the
// title decrypts from noise over CardTitleLength, then the prose wipes
// in under it over CardProseLength, and the frame holds. The layout is
// the card modal's: row 0 the title, row 1 blank, the prose from row 2
// wrapped to the frame's width (the modal's inner width, as viewCard
// wraps it, so a resize mid-scene rewraps and the scene needs no width
// to be made), every line flush left (each part's box is its text's
// size); the model draws row 0 in the modal's title bar and the rest
// as its body, and the last frame is the finished card less the
// choices. The title settles in theme.Warn, the dilemma's accent; the
// prose is plain, as the card's is, the wipe's edge in theme.Text. The
// dice are the title's; the wipe throws none.
func Card(title, text string, rng *rand.Rand) Scene {
	t := NewText(title)
	return &card{
		title: Part{Scene: Decrypt(t, theme.Warn, CardTitleLength, rng), Over: CardTitleLength, W: t.Width(), H: 1},
		text:  text,
	}
}

// CardTitleLength is how long the card's title takes to decrypt, and
// CardProseLength how long its prose takes to wipe in after it.
const (
	CardTitleLength = 600 * time.Millisecond
	CardProseLength = 400 * time.Millisecond
)

type card struct {
	title Part
	text  string
}

func (c *card) Done(t time.Duration) bool { return t >= CardTitleLength+CardProseLength }

func (c *card) Frame(t time.Duration, w, h int) []string {
	p := NewText(Wrap(c.text, w))
	return Sequence(c.title, Part{
		Scene: Wipe(p, "", CardProseLength, nil), Over: CardProseLength,
		Y: 2, W: p.Width(), H: p.Height(),
	}).Frame(t, w, h)
}

// Wrap wraps prose to w cells the way the modal does (ui's wrapLines:
// theme.Plain at the width), each line's padding trimmed so a text
// made of it is the words' width.
func Wrap(text string, w int) string {
	ls := strings.Split(theme.Plain.Width(max(1, w)).Render(text), "\n")
	for i, l := range ls {
		ls[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(ls, "\n")
}

// sampleCard is the card the registry makes the scene over, for
// TestScenesFit and a listing.
const sampleTitle, sampleText = "A MESSAGE FROM THE PORT", "A man you have never met stops you outside the front and says the boat is late, the buyer is nervous and somebody is asking after you by name. He wants an answer before the tide turns."
