// The DA race on the ledger (#534): while the tickets take money the
// ledger carries a DA RACE block, a row a city with what you have put
// behind a ticket there, and the pane's section for the city under the
// cursor says who is running, the odds, the day and what it costs. The
// da_race alert and its stop open it: a playtest's `o` landed on a
// ledger that said nothing of the race.

package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// raceShown is whether the ledger carries the DA RACE block: the
// tickets take money today (the law sim's campaign window) and there is
// an election to come.
func (m *Model) raceShown() bool {
	return m.w.Law.CampaignOpen && m.rules.Law.NextElection(m.w) > 0
}

// raceRows are the block's rows: a city each, in city order.
func (m *Model) raceRows() []string {
	if !m.raceShown() {
		return nil
	}
	return m.w.CityOrder
}

// raceCols are the block's columns: the city, the ticket your money is
// behind there, what the campaign holds and the points of the vote it
// moves.
var raceCols = []col{{"city", kText, 0}, {"ticket", kText, 0}, {"backed", kMoney, 0}, {"points", kText, 0}}

// raceTable is the block's rows as the table draws them.
func (m *Model) raceTable() [][]any {
	cmp := m.rules.Law.Campaign()
	var out [][]any
	for _, cid := range m.raceRows() {
		camp := m.w.Campaigning(cid)
		ticket, points := "-", "-"
		switch {
		case camp.Hedged:
			ticket = "both"
		case camp.Cash > 0:
			ticket, points = stanceWord(camp.Ticket), swingWord(cmp.Swing(camp.Cash))
		}
		out = append(out, []any{m.w.CityName(cid), ticket, camp.Cash, points})
	}
	return out
}

// raceNote is the block's heading note: the day of the vote.
func (m *Model) raceNote() string {
	next := m.rules.Law.NextElection(m.w)
	return fmt.Sprintf(" · the vote on day %d, %s", next, raceWhen(next-m.w.Day))
}

// raceWhen is how far off the vote is: `tonight`, `in 12 days`.
func raceWhen(days int) string {
	if days <= 0 {
		return "tonight"
	}
	return "in " + plural(days, "day")
}

// raceSection is the pane's section for the city under the cursor: who
// is running, the odds as the count would read them this morning, the
// day, what your money there holds and buys, and what it costs.
func (m *Model) raceSection(city string) section {
	w := m.w
	law := m.rules.Law
	cmp := law.Campaign()
	next := law.NextElection(w)
	lo, reform, moderate := law.Odds(w)
	lines := []string{
		row("vote", fmt.Sprintf("day %d, %s", next, raceWhen(next-w.Day))),
		row("sitting", fmt.Sprintf("DA %s, %s", w.Law.DA.Name, stanceWord(w.Law.DA.Stance))),
		row("odds", "law-and-order "+format.Pct(lo, 0)),
		row("", "reform "+format.Pct(reform, 0)),
		row("", "a moderate "+format.Pct(moderate, 0)),
	}
	camp := w.Campaigning(city)
	switch {
	case camp.Hedged:
		lines = append(lines, row("yours", theme.Bad.Render(money(camp.Cash)+" on both")))
	case camp.Cash > 0:
		lines = append(lines, row("yours", theme.Gold.Render(money(camp.Cash))+" "+stanceWord(camp.Ticket)), row("", theme.Good.Render(swingWord(cmp.Swing(camp.Cash)))+" of the vote"))
	default:
		lines = append(lines, row("yours", theme.Subtle.Render("nothing yet")))
	}
	lines = append(lines, row("price", money(cmp.Cash)+" a point"), row("", fmt.Sprintf("%.0f points at most", cmp.SwingMax*100)))
	lines = append(lines, keyRow("f", "fund; back a ticket on page 2"))
	// What it costs, after the key so the pane cuts the prose first.
	costs := fmt.Sprintf("Clean cash, spent at the count. A campaign adds %.1f pressure a day; a losing ticket adds %.0f here", cmp.Pressure, cmp.LoserPressure)
	if cmp.LoserChief {
		costs += ", and a zealous chief if law-and-order wins"
	}
	lines = append(lines, m.wrapped(theme.Subtle, costs+". A winner you backed owes you: the sting line sits higher.")...)
	return section{title: "DA RACE · " + strings.ToUpper(w.CityName(city)), lines: lines}
}

// selectRace puts the ledger's cursor on the city's DA race row, where
// the da_race alert opens (#534), or on the first if the city has none.
func (m *Model) selectRace(city string) {
	first := -1
	for i, r := range m.ledgerRows() {
		if r.kind != ledgerRace {
			continue
		}
		if first < 0 {
			first = i
		}
		if m.raceRows()[r.i] == city {
			m.ledgerCursor = i
			return
		}
	}
	if first >= 0 {
		m.ledgerCursor = first
	}
}

// raceCity is the city of the DA race row i.
func (m *Model) raceCity(i int) string {
	if rows := m.raceRows(); i < len(rows) {
		return rows[i]
	}
	return m.w.Player.Location
}
