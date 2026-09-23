package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestEndDayShowsThePreview (#353): the END THE DAY? modal opens on
// what needs you (an idle runner, the alerts), then the cart and
// tonight's money, estimated, closing where the engine's preview does,
// and says what it cannot know. The scroll keys move it without
// closing it, and it ends the day only on y or enter.
func TestEndDayShowsThePreview(t *testing.T) {
	m := richModel(t, 80, 24)
	fillCart(t, m)
	// A runner nobody posted.
	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 9001, Name: "Idle Ike", Role: game.RoleRunner, Loyalty: 80})
	day := m.w.Day
	m.Update(key("enter"))
	if m.mode != modeConfirmEnd {
		t.Fatalf("enter: mode %v", m.mode)
	}
	body := stripANSI(strings.Join(m.previewLines(), "\n"))
	p := m.sess.Preview()
	for _, want := range []string{"Idle tonight: Idle Ike (runner)", "Buying 2 lines for", "tonight ~", "Now", "Sales, net of cuts", "Closing", money(p.Flow.Closing.Dirty + p.Flow.Closing.Clean), "Estimates before the dice", "robberies", "the police"} {
		if !strings.Contains(body, want) {
			t.Errorf("the preview lacks %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "Idle tonight") > strings.Index(body, "Buying") || strings.Index(body, "Buying") > strings.Index(body, "tonight ~") {
		t.Errorf("the attention lines do not come first, then the cart, then the money:\n%s", body)
	}
	assertFits(t, m.View(), 80, 24, "the day's preview")
	for _, k := range []string{"pgdown", "down", "j", "up"} {
		m.Update(key(k))
		if m.mode != modeConfirmEnd || m.w.Day != day {
			t.Fatalf("%s on the preview: mode %v, day %d -> %d", k, m.mode, day, m.w.Day)
		}
	}
	m.Update(key("enter"))
	if m.w.Day != day+1 {
		t.Fatalf("enter on the preview: day %d -> %d", day, m.w.Day)
	}
}
