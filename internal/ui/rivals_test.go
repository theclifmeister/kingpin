package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
)

// paneText is the details pane's text read off a view at a width from
// the pane's: every body row's last paneWidth cells, borders trimmed.
func paneText(m *Model) string {
	var out []string
	for _, l := range strings.Split(stripANSI(m.View()), "\n")[1 : m.height-1] {
		rs := []rune(l)
		if len(rs) < paneWidth {
			continue
		}
		out = append(out, strings.TrimSpace(strings.Trim(string(rs[len(rs)-paneWidth:]), "│╭╮╰╯─ ")))
	}
	return strings.Join(out, "\n")
}

// mainText is MAIN's text read off a view: every body row's first
// mainWidth cells.
func mainText(m *Model) string {
	var out []string
	for _, l := range strings.Split(stripANSI(m.View()), "\n")[1 : m.height-1] {
		rs := []rune(l)
		out = append(out, strings.TrimRight(string(rs[:min(len(rs), m.mainWidth())]), " "))
	}
	return strings.Join(out, "\n")
}

// The rivals screen (#87): MAIN is the leader line, the trust and war
// bars, DEALS and OFFERS as tables with the offer cursor and tonight's
// proposal; the pane is the selected offer (its terms, who and when,
// what it does and what breaking it costs), RULES whole and LIFETIME.
// The key text beside the offer and the rules under it are gone from
// MAIN.
func TestRivalsPane(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	// The fixture's deal and offer, set again: a chaotic rival can break
	// the split on the day the fixture plays, and seeds are wall-clock.
	w.Rival.Deals = []game.Deal{{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{w.Home().Corners[1].ID}}, Since: w.Day}}
	w.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Offered: true}, Expires: w.Day + 3}}
	m.Update(key("8"))
	assertFrame(t, m, "rivals at 120x40")
	main := mainText(m)
	for _, want := range []string{"RIVALS · " + w.Home().Name, "trust ", "war ", "DEALS", "kind", "terms", "days", "who", "OFFERS", "answer by", "▸ truce", "30 days"} {
		if !strings.Contains(main, want) {
			t.Errorf("MAIN lacks %q:\n%s", want, main)
		}
	}
	for _, stale := range []string{"y accept x decline", dealRules[0], "Struck", "Trust grows", "day(s)"} {
		if strings.Contains(main, stale) {
			t.Errorf("MAIN still carries %q:\n%s", stale, main)
		}
	}
	pane := paneText(m)
	for _, want := range []string{"TRUCE · 30 DAYS", "theirs · 4 days to answer", "y  accept it", "x  turn it down", "RULES", "LIFETIME", "struck", "broken", "tribute", "KEYS"} {
		if !strings.Contains(pane, want) {
			t.Errorf("the pane lacks %q:\n%s", want, pane)
		}
	}
	// The rules are whole in the pane at 120: every word of every rule,
	// in order, and no line cut.
	flat := strings.Join(strings.Fields(pane), " ")
	for _, rule := range dealRules {
		if !strings.Contains(flat, rule) {
			t.Errorf("the pane cuts the rule %q:\n%s", rule, pane)
		}
	}
	if strings.Contains(pane, "…") {
		t.Errorf("a pane line is cut:\n%s", pane)
	}
	// A proposal for tonight goes under DEALS.
	if err := w.Propose(game.DealTribute, game.Terms{PerDay: 500}); err != nil {
		t.Fatal(err)
	}
	if main := mainText(m); !strings.Contains(main, "Tonight  you propose tribute of $500 a day") {
		t.Errorf("MAIN lacks the proposal:\n%s", main)
	}
	w.Proposal = nil
	// With no offer the pane shows the deal that holds; with none of
	// either, the mood.
	w.Offers = nil
	if secs := m.details(); secs[0].title != "SPLIT · 1 CORNER" || !strings.Contains(stripANSI(strings.Join(secs[0].lines, "\n")), "until broken") {
		t.Errorf("with no offer the first section is %q: %v", secs[0].title, secs[0].lines)
	}
	w.Rival.Deals = nil
	if secs := m.details(); secs[0].title != strings.ToUpper(m.rivalName()) || !strings.Contains(strings.Join(secs[0].lines, " "), "Trust grows") {
		t.Errorf("with nothing on the table the first section is %q: %v", secs[0].title, secs[0].lines)
	}
	assertFrame(t, m, "rivals with nothing on the table")
}

// At 80x24 the overlay carries the rivals' sections whole: the last
// RULES line is reached by scrolling, never cut (#87 made the overlay
// scroll rather than trim).
func TestRivalsOverlayScrolls(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	w.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Offered: true}, Expires: w.Day + 3}}
	m.Update(key("8"))
	assertFrame(t, m, "rivals at 80x24")
	if got := stripLine(m); !strings.HasPrefix(got, "▸ TRUCE · 30 DAYS") {
		t.Fatalf("the strip does not name the offer: %q", got)
	}
	m.Update(key(" "))
	if m.mode != modeDetails {
		t.Fatalf("space: mode %v", m.mode)
	}
	last := "warning does not."
	seen := strings.Contains(stripANSI(m.View()), last)
	for i := 0; i < 40 && !seen; i++ {
		assertFits(t, m.View(), 80, 24, "rivals overlay")
		m.Update(key("down"))
		seen = strings.Contains(stripANSI(m.View()), last)
	}
	if !seen {
		t.Fatalf("the overlay never reaches the last RULES line:\n%s", stripANSI(m.View()))
	}
	view := stripANSI(m.View())
	if strings.Contains(view, "…") {
		t.Errorf("an overlay line is cut:\n%s", view)
	}
	for _, want := range []string{"LIFETIME", "KEYS"} {
		for !strings.Contains(stripANSI(m.View()), want) {
			before := m.modalScroll
			m.Update(key("down"))
			if m.modalScroll == before {
				t.Fatalf("the overlay never reaches %s:\n%s", want, stripANSI(m.View()))
			}
		}
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc: mode %v", m.mode)
	}
}

// The rivals screen's empty states are one sentence in the one
// register: no deals, nothing on the table, nobody in town.
func TestRivalsEmptyStates(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	m.Update(key("8"))
	if view := stripANSI(m.View()); !strings.Contains(view, "Nobody is contesting the city yet.") {
		t.Errorf("no rival:\n%s", view)
	}
	w.Rival.Arrived, w.Rival.Muscle, w.Rival.War = 1, 3, 0
	view := stripANSI(m.View())
	for _, want := range []string{"No deals. Press d to propose one.", "Nothing on the table.", "no war", "run out of town"} {
		if !strings.Contains(view, want) {
			t.Errorf("the rivals screen lacks %q:\n%s", want, view)
		}
	}
	for _, stale := range []string{"none. d proposes one.", "nothing on the table."} {
		if strings.Contains(view, stale) {
			t.Errorf("the rivals screen still says %q:\n%s", stale, view)
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	assertFrame(t, m, "rivals with the empty states")
}
