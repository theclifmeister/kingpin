package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The card's scene (#154), the first interstitial and the template the
// others copy: the morning a card is pending its title decrypts from
// noise in the modal's title bar (anim.CardTitleLength), the prose
// wipes in under it (anim.CardProseLength), and then the choices and
// the footer appear with the keys live. Any key skips to the finished
// card and is consumed (skip, at the top of handleKey). The day has
// stepped and saved before the first frame (stepDay, then morning);
// fast-forward's stopping morning plays it as the n key's does, and so
// does continuing a save on a card; the card's outcome is a result,
// read at leisure, and is not animated. Under 80x24 or with animation
// off there is no scene and the card is exactly the one before #154.

// cardScene starts the card's scene: animation on and the terminal at
// least 80x24, or its size not yet known (kingpin -slot N continuing a
// save on a card runs before the first WindowSizeMsg, whose Update
// issues the first tick; the scene wraps at the frame's width, so it
// needs none to be made), the dice anim.Seed(seed, day, "card"). The
// title is in caps, as the modal's bar draws it.
func (m *Model) cardScene(c *game.Card) {
	if !m.opts.Anim || (m.width != 0 && !m.titleFits()) {
		return
	}
	m.play(&anim.Player{
		Scene:  anim.Card(strings.ToUpper(c.Title), c.Text, anim.Seed(m.w.Seed, m.w.Day, "card")),
		Accent: theme.Warn,
	})
}

// cardOnScene reports whether viewCard draws the scene: one is up and
// the terminal is 80x24 or more (a scene started before the size was
// known runs out unseen under the floor).
func (m *Model) cardOnScene() bool {
	return m.scene != nil && !m.scene.Idle && m.titleFits()
}

// viewCardScene is the card mid-scene: the scene's frame at the modal's
// inner width, as many rows as the title and the wrapped prose take;
// row 0 in the title bar (the colours stripped, so the bar draws it in
// theme.Title as it always does; the churn glyphs are the noise), the
// rest the body, with the rows the choices will take held blank so the
// box is its finished size from the first frame, and no footer until
// Done.
func (m *Model) viewCardScene(c *game.Card) string {
	frame := m.scene.Frame(m.modalInner(), 2+len(m.wrapLines(c.Text)))
	body := append(frame[2:], make([]string, 1+len(c.Choices))...)
	return m.modal(ansi.Strip(frame[0]), body, nil)
}
