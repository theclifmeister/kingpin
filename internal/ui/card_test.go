package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestCardShowsWhatEachChoiceDoes (#358): under every label the card
// shows what the choice does, the chips folded to the modal; the
// longest card in the deck with its chips fits 80x24, 100x30 and
// 120x40, and the cursor on its last choice keeps that choice and its
// chips in view. A card with hide shows "costs you something" under
// every choice and no figure.
func TestCardShowsWhatEachChoiceDoes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		m.w.Player.DirtyCash = 40_000
		var long *game.Card
		size := 0
		for _, cc := range m.cfg.Dilemmas.Cards {
			c := &game.Card{ID: cc.ID, Day: m.w.Day, Title: cc.Title, Text: cc.Text, Amount: 4_000}
			if len(m.w.Crew.Members) > 0 {
				c.Member = m.w.Crew.Members[0].ID
			}
			n := len(cc.Text)
			for _, ch := range cc.Choices {
				c.Choices = append(c.Choices, game.Choice{Label: ch.Label, Outcome: ch.Outcome, Effects: ch.Effects})
				n += len(ch.Label) + 24*len(ch.Effects)
			}
			if n > size {
				long, size = c, n
			}
		}
		m.w.Dilemmas.Pending = long
		m.showCard()
		view := m.View()
		assertFits(t, view, sz[0], sz[1], long.ID)
		plain := stripANSI(view)
		if !strings.Contains(plain, " · ") && !strings.Contains(plain, "changes nothing") {
			t.Errorf("%dx%d: %s shows no chips:\n%s", sz[0], sz[1], long.ID, plain)
		}
		last := len(long.Choices) - 1
		for range last {
			m.Update(key("down"))
		}
		view = stripANSI(m.View())
		assertFits(t, view, sz[0], sz[1], long.ID+" on its last choice")
		if !strings.Contains(view, long.Choices[last].Label) {
			t.Errorf("%dx%d: the last choice is out of view:\n%s", sz[0], sz[1], view)
		}
		lines, at := m.cardChoices(long)
		if tail := strings.TrimSpace(stripANSI(lines[len(lines)-1])); at[last] < len(lines)-1 && !strings.Contains(view, tail) {
			t.Errorf("%dx%d: the last choice's chips %q are out of view:\n%s", sz[0], sz[1], tail, view)
		}
	}

	m := richModel(t, 80, 24)
	c := testCard(m.w.Day)
	m.w.Dilemmas.Pending = c
	m.showCard()
	view := stripANSI(m.View())
	for _, want := range []string{"dirty −$100", "heat +7", "changes nothing"} {
		if !strings.Contains(view, want) {
			t.Errorf("the test card does not show %q:\n%s", want, view)
		}
	}
	c = testCard(m.w.Day)
	c.Hide = true
	m.w.Dilemmas.Pending = c
	m.showCard()
	view = stripANSI(m.View())
	if strings.Count(view, "costs you something") != len(c.Choices) || strings.Contains(view, "$100") || strings.Contains(view, "heat +7") {
		t.Errorf("a hidden card shows:\n%s", view)
	}
}
