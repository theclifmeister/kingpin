package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The rivals screen (#32) is the table: who the rival is, what it thinks
// of you, the deals that hold, the offers waiting and tonight's
// proposal. d proposes, y and x answer the selected offer.

// proposeKinds are the propose dialog's first page: the three deals that
// can be struck, and the joint shipment, listed but waiting on routes.
var proposeKinds = []string{game.DealTruce, game.DealTribute, game.DealSplit, game.DealShipment}

// termRows are the dialog's second page: the three asks for a kind, with
// the deal each one makes and a word on it.
func (m *Model) termRows(kind string) ([]game.Deal, []string) {
	dip := m.set.Rivals.Diplomacy()
	w := m.w
	var deals []game.Deal
	var words []string
	switch kind {
	case game.DealTruce:
		for i, d := range dip.TruceDays {
			deals = append(deals, game.Deal{Kind: kind, Terms: game.Terms{Days: d}})
			words = append(words, []string{"short", "standard", "long"}[i])
		}
	case game.DealTribute:
		for i, c := range dip.TributeCuts {
			deals = append(deals, game.Deal{Kind: kind, Terms: game.Terms{PerDay: m.set.Rivals.Cut(w, c)}})
			words = append(words, fmt.Sprintf("%.0f%% of your street", c*100)+[]string{" (thin)", "", " (fat)"}[i])
		}
	case game.DealSplit:
		for i, line := range w.SplitLines() {
			deals = append(deals, game.Deal{Kind: kind, Terms: game.Terms{Corners: line}})
			words = append(words, []string{"what you hold", "plus the free corners on your side", "plus every free corner"}[i])
		}
	}
	return deals, words
}

// askPropose opens the dialog, or explains why there is nobody to talk to.
func (m *Model) askPropose() {
	if m.w.Rival.Arrived == 0 {
		m.refuse("Nothing to propose: nobody is contesting the city yet.")
		return
	}
	if m.w.Over != nil {
		return
	}
	m.proposeStep, m.proposeKind, m.proposeCursor = 0, 0, 0
	m.mode = modePropose
}

// proposeRows are the rows of the current page.
func (m *Model) proposeRows() int {
	if m.proposeStep == 0 {
		n := len(proposeKinds)
		if m.w.Today.Proposal != nil {
			n++ // withdraw
		}
		return n
	}
	deals, _ := m.termRows(proposeKinds[m.proposeKind])
	return len(deals)
}

// pickPropose is enter on the dialog: the kind page opens the terms
// page, the terms page sends the proposal.
func (m *Model) pickPropose() {
	if m.proposeStep == 0 {
		if m.proposeCursor >= len(proposeKinds) {
			m.w.Withdraw()
			m.mode = modePlay
			m.say("Proposal withdrawn.")
			return
		}
		kind := proposeKinds[m.proposeCursor]
		if kind == game.DealShipment {
			m.refuse("Can't propose a shipment: joint shipments need routes, and there are none yet.")
			return
		}
		if d := m.w.Deal(kind); d != nil {
			m.refuse(fmt.Sprintf("Can't propose that: you already have %s.", m.w.Describe(*d)))
			return
		}
		m.proposeKind, m.proposeStep, m.proposeCursor = m.proposeCursor, 1, 1
		return
	}
	deals, _ := m.termRows(proposeKinds[m.proposeKind])
	if len(deals) == 0 {
		m.mode = modePlay
		return
	}
	d := deals[max(0, min(m.proposeCursor, len(deals)-1))]
	m.mode = modePlay
	if err := m.w.Propose(d.Kind, d.Terms); err != nil {
		m.refuse("Can't propose: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Proposed %s to %s. They answer in the morning; odds ~%.0f%%.", m.w.Describe(d), m.rivalName(), m.set.Rivals.Chance(m.w, d)*100))
}

func (m *Model) viewPropose() string {
	w := m.w
	var body []string
	m.proposeCursor = max(0, min(m.proposeCursor, m.proposeRows()-1))
	if m.proposeStep == 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("%s · %s · trust %.0f", m.rivalName(), m.personalityWord(), w.Rival.Trust)), "")
		for i, kind := range proposeKinds {
			var line, note string
			switch kind {
			case game.DealTruce:
				line, note = "truce   ", "neither side pushes; no undercutting, no tips, for a term"
			case game.DealTribute:
				line, note = "tribute ", "you pay a cut a day; they leave your corners alone, until you stop"
			case game.DealSplit:
				line, note = "split   ", "a line through the city; each side keeps to its own"
			case game.DealShipment:
				line, note = "shipment", theme.Subtle.Render("half the cost of a run, half the loss (needs routes)")
			}
			if d := w.Deal(kind); d != nil {
				note = theme.Good.Render("live: " + m.dealTerms(*d))
			}
			body = m.proposeLine(body, i, line, note)
		}
		if w.Today.Proposal != nil {
			body = m.proposeLine(body, len(proposeKinds), "withdraw", "take back tonight's proposal, "+w.Describe(*w.Today.Proposal))
		}
	} else {
		kind := proposeKinds[m.proposeKind]
		deals, words := m.termRows(kind)
		body = append(body, theme.Subtle.Render(fmt.Sprintf("%s to %s. Odds are what the dice use.", capitalize(kind), m.rivalName())), "")
		for i, d := range deals {
			odds := m.set.Rivals.Chance(w, d)
			var line string
			switch kind {
			case game.DealTruce:
				line = fmt.Sprintf("%3d days  %-9s", d.Terms.Days, words[i])
			case game.DealTribute:
				line = fmt.Sprintf("%9s/day  %-24s", money(d.Terms.PerDay), words[i])
			case game.DealSplit:
				line = fmt.Sprintf("%-10s %-36s", plural(len(d.Terms.Corners), "corner"), words[i])
			}
			note := fmt.Sprintf("~%.0f%%", odds*100)
			if odds == 0 {
				note = theme.Bad.Render("refused")
			}
			body = m.proposeLine(body, i, line, note)
		}
		switch kind {
		case game.DealTribute:
			body = append(body, "", theme.Subtle.Render(m.tributeBaseLine()))
		case game.DealSplit:
			body = append(body, "", theme.Subtle.Render("Your side: "+w.Side(deals[m.proposeCursor])))
		}
		if m.set.Rivals.Distrusted(w, w.Day+1) {
			body = append(body, "", theme.Bad.Render("They are not taking your calls. You broke a deal."))
		}
	}
	return m.modal("PROPOSE A DEAL", body, m.modalFooter())
}

// proposeLine appends a row of the propose dialog; the modal cuts it to
// its width.
func (m *Model) proposeLine(body []string, i int, line, note string) []string {
	if i == m.proposeCursor {
		m.modalFollow(len(body))
		return append(body, theme.Gold.Render("▸ ")+theme.Selected.Render(line)+"  "+note)
	}
	return append(body, "  "+line+"  "+note)
}

// answerOffer is y or x on the rivals screen: the selected offer taken
// or turned down.
func (m *Model) answerOffer(accept bool) {
	w := m.w
	if len(w.Offers) == 0 {
		m.refuse("Nothing to answer: no offer on the table.")
		return
	}
	o := w.Offers[max(0, min(m.dealCursor, len(w.Offers)-1))]
	if accept {
		if _, err := w.Accept(o.ID); err != nil {
			m.refuse("Can't accept: " + err.Error())
			return
		}
		m.say(fmt.Sprintf("Accepted %s. It holds from tonight.", w.Describe(o.Deal)))
		return
	}
	if _, err := w.Decline(o.ID); err != nil {
		m.refuse("Can't decline: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Turned down %s.", w.Describe(o.Deal)))
}

// trustBar is the rival's trust in you, as a bar.
func (m *Model) trustBar(width int) string {
	t := m.w.Rival.Trust
	style := theme.RivalText
	switch {
	case t < 20:
		style = theme.Bad
	case t >= 60:
		style = theme.Good
	}
	return style.Render(sparkline.Bar(t/100, width, nil)) + style.Render(fmt.Sprintf(" %.0f", t))
}

// warBar is the war against the line the police crack down at, as a
// bar: `war ████░░░░ 53/80 loud`, or `no war`.
func (m *Model) warBar(width int) string {
	r := m.w.Rival
	tun := m.set.Rivals.Tuning()
	if r.War <= 0 {
		return theme.Subtle.Render("no war")
	}
	style := theme.Warning
	word := ""
	if r.War >= tun.WarThreshold {
		style, word = theme.Bad, " loud"
	}
	return style.Render(sparkline.Bar(r.War/tun.CrackdownThreshold, width, nil)) + style.Render(fmt.Sprintf(" %.0f/%.0f%s", r.War, tun.CrackdownThreshold, word))
}

// rivalCorners is how much of the city the rival holds, in words.
func (m *Model) rivalCorners() string {
	if n := m.w.RivalHeld(); n > 0 {
		return plural(n, "corner")
	}
	return "run out of town"
}

// tributeBaseLine is what a tribute is a cut of today, the number the
// dice use (#162): the street value the corners you work at home move
// in the products the rival deals in, the port's product left out.
func (m *Model) tributeBaseLine() string {
	return fmt.Sprintf("Your street: %s a day on your corners here in what they sell.", cash(int(math.Round(m.set.Rivals.TributeBase(m.w)))))
}

// tributeRows are the pane's lines on a tribute: the cut it is of your
// street today, and what your street is, so the pane and the dice agree.
func (m *Model) tributeRows(d game.Deal) []string {
	base := m.set.Rivals.TributeBase(m.w)
	cut := "-"
	if base > 0 {
		cut = fmt.Sprintf("~%.0f%% of your street", 100*float64(d.Terms.PerDay)/base)
	}
	lines := []string{row("cut", cut)}
	return append(lines, wrapped(theme.Subtle, fmt.Sprintf("Your street is %s a day: what your corners here move in what they sell.", cash(int(math.Round(base)))))...)
}

// dealTerms is a deal's terms for a table cell: the description less
// its article, so the kind column carries the kind.
func (m *Model) dealTerms(d game.Deal) string {
	switch d.Kind {
	case game.DealTruce:
		return plural(d.Terms.Days, "day")
	case game.DealTribute:
		return money(d.Terms.PerDay) + " a day"
	case game.DealSplit:
		return "yours " + m.w.Side(d)
	}
	return m.w.Describe(d)
}

// dealTitle is a deal's name in caps for the pane: `TRUCE · 30 DAYS`.
func (m *Model) dealTitle(d game.Deal) string {
	switch d.Kind {
	case game.DealTruce:
		return fmt.Sprintf("TRUCE · %d DAYS", d.Terms.Days)
	case game.DealTribute:
		return "TRIBUTE · " + strings.ToUpper(money(d.Terms.PerDay)) + "/DAY"
	case game.DealSplit:
		return "SPLIT · " + strings.ToUpper(plural(len(d.Terms.Corners), "corner"))
	}
	return strings.ToUpper(d.Kind)
}

var (
	dealCols       = []col{{"kind", kText, 0}, {"terms", kText, 0}, {"days", kDays, 0}, {"who", kText, 0}}
	rivalOfferCols = []col{{"kind", kText, 0}, {"terms", kText, 0}, {"answer by", kDays, 0}}
)

// viewRivals is the rivals screen's MAIN: who the rival is, trust and
// the war, the deals that hold, tonight's proposal and the offers
// waiting under the cursor. The mood, the rules and the lifetime
// figures are the pane's.
func (m *Model) viewRivals() string {
	w := m.w
	r := w.Rival
	width := m.mainWidth()
	var ls []string
	line := func(s string) { ls = append(ls, truncate(s, width)) }
	sub := theme.Subtle.Render
	line(theme.PanelTitle.Render("RIVALS · " + w.Home().Name))
	if r.Arrived == 0 {
		line(sub("Nobody is contesting the city yet."))
		return strings.Join(ls, "\n")
	}
	leader := theme.RivalText.Render(m.rivalName()) + sub(fmt.Sprintf(" · %s · %s · muscle %d", m.personalityWord(), m.rivalCorners(), r.Muscle))
	if eye := m.eyeingWord(); eye != "" {
		leader += sub(" · ") + eye
	}
	line(leader)
	barW := max(6, min(12, width/6))
	line(sub("trust ") + m.trustBar(barW) + "   " + sub("war ") + m.warBar(barW))
	ls = append(ls, "")

	line(sectionTitle("DEALS", theme.Rivals))
	if len(r.Deals) == 0 {
		line(emptyState("No deals. Press ", "d", " to propose one."))
	} else {
		var rows [][]any
		for _, d := range r.Deals {
			var left any
			if d.Until > 0 {
				left = d.Left(w.Day)
			}
			who := "yours"
			if d.Offered {
				who = "theirs"
			}
			rows = append(rows, []any{d.Kind, m.dealTerms(d), left, who})
		}
		ls = append(ls, table(dealCols, rows, -1, width)...)
	}
	if p := w.Today.Proposal; p != nil {
		line(theme.Gold.Render(fmt.Sprintf("Tonight  you propose %s; they answer in the morning, ~%.0f%%", w.Describe(*p), m.set.Rivals.Chance(w, *p)*100)))
	}
	ls = append(ls, "")

	line(sectionTitle("OFFERS", theme.Rivals))
	if len(w.Offers) == 0 {
		line(sub("Nothing on the table."))
	} else {
		m.dealCursor = max(0, min(m.dealCursor, len(w.Offers)-1))
		var rows [][]any
		for _, o := range w.Offers {
			rows = append(rows, []any{o.Deal.Kind, m.dealTerms(o.Deal), day(o.Expires)})
		}
		ls = append(ls, table(rivalOfferCols, rows, m.dealCursor, width)...)
	}
	ls = append(ls, "")

	// The books (#70): what a scout last read, and the police's
	// attention on them.
	ls = append(ls, m.booksLines(width)...)
	return strings.Join(ls, "\n")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
