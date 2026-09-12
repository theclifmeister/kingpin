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
// the card modal's, as Stage's is the stage's: row 0 the title, row 1
// blank, the prose from row 2, each region on a canvas of its own
// text's width so both sit flush left as the modal draws them; the
// model draws row 0 in the modal's title bar and the rest as its body,
// and the last frame is the finished card less the choices. The prose
// is wrapped at the frame's width (Wrap, what the modal's wrapLines
// does), so the scene needs no width to be made and a resize mid-scene
// rewraps. The title settles in theme.Warn, the dilemma's accent; the
// prose is plain, as the card's is, the wipe's edge in theme.Text. The
// dice are the title's; the wipe throws none.
func Card(title, text string, rng *rand.Rand) Scene {
	t := NewText(title)
	return &card{title: t, text: text, head: Decrypt(t, theme.Warn, CardTitleLength, rng)}
}

// CardTitleLength is how long the card's title takes to decrypt, and
// CardProseLength how long its prose takes to wipe in after it.
const (
	CardTitleLength = 600 * time.Millisecond
	CardProseLength = 400 * time.Millisecond
)

type card struct {
	title Text
	text  string
	head  Scene // the title's decrypt
}

func (c *card) Done(t time.Duration) bool { return t >= CardTitleLength+CardProseLength }

func (c *card) Frame(t time.Duration, w, h int) []string {
	out := make([]string, 0, h)
	if h > 0 {
		out = append(out, c.head.Frame(t, min(w, c.title.Width()), 1)...)
	}
	out = append(out, "")
	if t >= CardTitleLength {
		p := NewText(Wrap(c.text, w))
		out = append(out, Wipe(p, "", CardProseLength, nil).Frame(t-CardTitleLength, min(w, p.Width()), p.Height())...)
	}
	for len(out) < h {
		out = append(out, "")
	}
	return out[:h]
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

// sampleTitle and sampleText are the card the registry makes the scene
// over, for TestScenesFit and a listing.
const sampleTitle, sampleText = "A MESSAGE FROM THE PORT", "A man you have never met stops you outside the front and says the boat is late, the buyer is nervous and somebody is asking after you by name. He wants an answer before the tide turns."
