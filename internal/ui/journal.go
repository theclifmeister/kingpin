package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The journal (#84) is the headline list, newest first, with a cursor:
// MAIN is the list with the day in front of each headline, cut to the
// width with an ellipsis; the pane is the headline under the cursor
// (its day and source, the text whole), the legend of the sources'
// colours, and the keys.

// journalSources are the sources a headline can have, in the order the
// journal's legend lists them.
var journalSources = []string{"market", "buyers", "heat", "law", "crew", "territory", "rivals", "laundering", "logistics", "reputation", "dilemma", "flavour"}

// sourceName is a headline's source as the legend names it: the news
// sim's own colour is the flavour of the city.
func sourceName(source string) string {
	if source == "news" || source == "" {
		return "flavour"
	}
	return source
}

// headlines is the journal newest first: the order the list draws it in
// and the cursor counts through.
func (m *Model) headlines() []game.Headline {
	j := m.w.Journal
	out := make([]game.Headline, 0, len(j))
	for i := len(j) - 1; i >= 0; i-- {
		out = append(out, j[i])
	}
	return out
}

// journalRows is how many headlines MAIN shows at once: its rows less
// the title.
func (m *Model) journalRows() int {
	return max(1, m.mainHeight()-1)
}

// refreshJournal puts the cursor back on the newest headline: a new
// morning, a new run or a continued save starts at the top.
func (m *Model) refreshJournal() {
	if m.w == nil {
		return
	}
	m.journalCursor, m.journalTop = 0, 0
	// A new run's journal is shorter than the last one's: nothing in it
	// has been read past its end.
	m.journalSeen = min(m.journalSeen, len(m.w.Journal))
}

// journalUnread is how many headlines have landed since the journal
// screen was last shown.
func (m *Model) journalUnread() int {
	return max(0, len(m.w.Journal)-m.journalSeen)
}

// journalMove moves the cursor by d headlines, held inside the list.
func (m *Model) journalMove(d int) {
	n := len(m.w.Journal)
	if n == 0 {
		m.journalCursor, m.journalTop = 0, 0
		return
	}
	m.journalCursor = min(max(m.journalCursor+d, 0), n-1)
	m.journalFollow()
}

// journalPage moves the cursor a screenful of headlines up (d < 0) or
// down.
func (m *Model) journalPage(d int) {
	if d < 0 {
		m.journalMove(-m.journalRows())
	} else {
		m.journalMove(m.journalRows())
	}
}

// journalFollow scrolls the list so the cursor is on screen, and no
// further than the list runs.
func (m *Model) journalFollow() {
	if m.w == nil {
		return
	}
	n, rows := len(m.w.Journal), m.journalRows()
	m.journalCursor = min(max(m.journalCursor, 0), max(n-1, 0))
	m.journalTop = min(m.journalTop, max(n-rows, 0))
	if m.journalCursor < m.journalTop {
		m.journalTop = m.journalCursor
	}
	if m.journalCursor >= m.journalTop+rows {
		m.journalTop = m.journalCursor - rows + 1
	}
}

// selectedHeadline is the headline under the cursor, or nil when the
// paper has nothing to say.
func (m *Model) selectedHeadline() *game.Headline {
	hs := m.headlines()
	if len(hs) == 0 {
		return nil
	}
	m.journalFollow()
	h := hs[m.journalCursor]
	return &h
}

// viewJournal is the journal's MAIN: the title with the count, then the
// headlines from journalTop, each as its day and its text in the
// source's colour, cut to the width, the cursor's row selected across.
func (m *Model) viewJournal() string {
	m.journalSeen = len(m.w.Journal) // shown is read
	width := m.mainWidth()
	hs := m.headlines()
	var b strings.Builder
	b.WriteString(truncate(theme.PanelTitle.Render("JOURNAL")+theme.Subtle.Render(fmt.Sprintf(" · %s, newest first", plural(len(hs), "headline"))), width) + "\n")
	if len(hs) == 0 {
		b.WriteString(theme.Subtle.Render("The paper has nothing to say about you. Yet.") + "\n")
		return b.String()
	}
	m.journalFollow()
	dayW := 2
	for _, h := range hs {
		dayW = max(dayW, len(fmt.Sprintf("d%d", h.Day)))
	}
	textW := max(3, width-2-dayW-2)
	end := min(len(hs), m.journalTop+m.journalRows())
	for i := m.journalTop; i < end; i++ {
		h := hs[i]
		day := fit(fmt.Sprintf("d%d", h.Day), dayW)
		text := truncate(h.Text, textW)
		if i == m.journalCursor {
			b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(day+"  "+text) + "\n")
			continue
		}
		b.WriteString("  " + theme.Subtle.Render(day) + "  " + theme.SourceText(h.Source).Render(text) + "\n")
	}
	return b.String()
}

// journalDetails is the journal's pane: the headline under the cursor
// (its day and source in the title, the text whole) and the LEGEND of
// what the colours mean, one source a line. Both are laid out for
// where they are drawn: the pane beside MAIN, or else the overlay,
// which is wider and shorter, so the text wraps to its width and the
// legend packs its sources in cells the way KEYS does.
func (m *Model) journalDetails() []section {
	textW, cols := paneTextW, 1
	if !m.paneShown() {
		textW = m.modalInner()
		cols = max(1, textW/keyCellW)
	}
	var secs []section
	if h := m.selectedHeadline(); h != nil {
		style := theme.SourceText(h.Source)
		var lines []string
		for _, l := range wrap(h.Text, textW) {
			lines = append(lines, style.Render(l))
		}
		secs = append(secs, section{fmt.Sprintf("D%d · %s", h.Day, strings.ToUpper(sourceName(h.Source))), lines})
	} else {
		secs = append(secs, section{"JOURNAL", []string{theme.Subtle.Render("Nothing yet.")}})
	}
	var legend []string
	for i := 0; i < len(journalSources); i += cols {
		var line string
		for _, s := range journalSources[i:min(i+cols, len(journalSources))] {
			line += fit(theme.SourceText(s).Render(s), keyCellW)
		}
		legend = append(legend, strings.TrimRight(line, " "))
	}
	return append(secs, section{"LEGEND", legend})
}
