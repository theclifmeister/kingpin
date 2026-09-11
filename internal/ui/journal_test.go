package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The journal is a list with a cursor (#84): a headline wider than the
// screen is one line ending in an ellipsis at 80 columns, the arrows
// and the paging keys move the cursor through it, newest first, and the
// pane names the selected headline's day and source and carries the
// legend.
func TestJournalTruncatesWithEllipsis(t *testing.T) {
	m := newTestModel(t, 80, 24)
	long := ("Somebody is moving a lot of Weed in Eastside, " + strings.Repeat("and then some more, ", 10))[:200]
	m.w.Journal = append(m.w.Journal,
		game.Headline{Day: 3, Source: "crew", Text: "Eastside crews hiring, say people who would know"},
		game.Headline{Day: 8, Source: "heat", Text: long},
	)
	m.Update(key("3"))
	assertFrame(t, m, "journal at 80x24")
	main, _ := bodyRows(m)
	if !strings.HasPrefix(main[0], "JOURNAL · ") || !strings.Contains(main[0], "headlines, newest first") {
		t.Errorf("the title is %q", main[0])
	}
	first := strings.TrimRight(main[1], " ")
	if !strings.HasPrefix(first, "▸ d8  Somebody") || !strings.HasSuffix(first, "…") || lipgloss.Width(first) > 80 {
		t.Errorf("the newest headline is not one line ending in …: %q", first)
	}
	if n := strings.Count(strings.Join(main, "\n"), "Somebody"); n != 1 {
		t.Errorf("the long headline is on %d lines", n)
	}
	if !strings.HasPrefix(strings.TrimRight(main[2], " "), "  d3  Eastside crews hiring") {
		t.Errorf("the older headline is not second: %q", main[2])
	}
	// The strip names the day and source; the overlay carries the text
	// whole and the legend.
	ls := strings.Split(stripANSI(m.View()), "\n")
	if strip := ls[len(ls)-2]; !strings.HasPrefix(strip, "▸ D8 · HEAT · Somebody") {
		t.Errorf("the strip is %q", strip)
	}
	m.Update(key(" "))
	overlay := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "journal overlay")
	if strings.Count(overlay, "more") < strings.Count(long, "more") || !strings.Contains(overlay, "LEGEND") || !strings.Contains(overlay, "laundering") {
		t.Errorf("the overlay lacks the headline whole or the legend:\n%s", overlay)
	}
	m.Update(key("esc"))
	// The cursor moves, and the pane at 120 follows it.
	m.Update(key("down"))
	if m.journalCursor != 1 {
		t.Fatalf("down: cursor %d", m.journalCursor)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_, pane := bodyRows(m)
	paneText := strings.Join(pane, "\n")
	if !strings.Contains(paneText, "D3 · CREW") || !strings.Contains(paneText, "Eastside crews hiring") {
		t.Errorf("the pane does not name the selected headline's day and source:\n%s", paneText)
	}
	for _, s := range journalSources {
		if !strings.Contains(paneText, s) {
			t.Errorf("the legend lacks %q:\n%s", s, paneText)
		}
	}
	main, _ = bodyRows(m)
	if !strings.HasPrefix(main[2], "▸ d3  ") || strings.HasPrefix(main[1], "▸") {
		t.Errorf("the cursor row is not the second headline:\n%s", strings.Join(main[:3], "\n"))
	}
	m.Update(key("down"))
	if m.journalCursor != 1 {
		t.Errorf("down past the end: cursor %d", m.journalCursor)
	}
	m.Update(key("up"))
	m.Update(key("up"))
	if m.journalCursor != 0 {
		t.Errorf("up past the top: cursor %d", m.journalCursor)
	}
}

// The paging keys move the cursor a screenful and the list scrolls to
// keep it in view; the morning puts it back on the newest headline.
func TestJournalPages(t *testing.T) {
	m := newTestModel(t, 80, 24)
	for i := 0; i < 60; i++ {
		m.w.Journal = append(m.w.Journal, game.Headline{Day: i / 5, Source: "market", Text: "headline"})
	}
	m.Update(key("3"))
	rows := m.journalRows()
	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.journalCursor != rows || m.journalTop != 1 {
		t.Fatalf("pgdn: cursor %d top %d rows %d", m.journalCursor, m.journalTop, rows)
	}
	main, _ := bodyRows(m)
	if !strings.HasPrefix(main[len(main)-1], "▸ ") {
		t.Errorf("the cursor is not on the last row shown:\n%s", strings.Join(main, "\n"))
	}
	for i := 0; i < 10; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if m.journalCursor != 59 || m.journalTop != 60-rows {
		t.Errorf("pgdn to the end: cursor %d top %d", m.journalCursor, m.journalTop)
	}
	assertFrame(t, m, "journal at the end")
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.journalCursor != 59-rows {
		t.Errorf("pgup: cursor %d", m.journalCursor)
	}
	// A narrower, shorter terminal keeps the cursor in view.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	assertFrame(t, m, "journal at 80x12")
	if m.journalCursor < m.journalTop || m.journalCursor >= m.journalTop+m.journalRows() {
		t.Errorf("after a resize the cursor %d is off the window at %d", m.journalCursor, m.journalTop)
	}
	endDay(t, m)
	m.Update(key("enter"))
	if m.journalCursor != 0 || m.journalTop != 0 {
		t.Errorf("the morning did not put the cursor on the newest headline: %d at %d", m.journalCursor, m.journalTop)
	}
	// An empty journal says so and takes the arrows in its stride.
	m.w.Journal = nil
	m.refreshJournal()
	m.Update(key("down"))
	if got := stripANSI(m.View()); !strings.Contains(got, "The paper has nothing to say about you. Yet.") {
		t.Errorf("the empty journal:\n%s", got)
	}
	if secs := m.details(); len(secs) == 0 || secs[len(secs)-1].title != "LEGEND" {
		t.Errorf("the empty journal's pane: %+v", secs)
	}
}
