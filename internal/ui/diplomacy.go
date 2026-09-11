package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

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
			words = append(words, fmt.Sprintf("%.0f%% of your take", c*100)+[]string{" (thin)", "", " (fat)"}[i])
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
		m.status = "Nobody is contesting the city yet. There is nobody to deal with."
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
		if m.w.Proposal != nil {
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
			m.status = "Proposal withdrawn."
			return
		}
		kind := proposeKinds[m.proposeCursor]
		if kind == game.DealShipment {
			m.status = "Joint shipments need routes. Not yet."
			return
		}
		if d := m.w.Deal(kind); d != nil {
			m.status = fmt.Sprintf("You already have %s.", m.w.Describe(*d))
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
		m.status = "Can't propose: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Proposed %s to %s. They answer in the morning; odds ~%.0f%%.", m.w.Describe(d), m.rivalName(), m.set.Rivals.Chance(m.w, d)*100)
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
				note = theme.Good.Render("live: " + w.Describe(*d))
			}
			body = m.proposeLine(body, i, line, note)
		}
		if w.Proposal != nil {
			body = m.proposeLine(body, len(proposeKinds), "withdraw", "take back tonight's proposal: "+w.Describe(*w.Proposal))
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
				line = fmt.Sprintf("%9s/day  %-22s", money(d.Terms.PerDay), words[i])
			case game.DealSplit:
				line = fmt.Sprintf("%d corners  %-36s", len(d.Terms.Corners), words[i])
			}
			note := fmt.Sprintf("~%.0f%%", odds*100)
			if odds == 0 {
				note = theme.Bad.Render("refused")
			}
			body = m.proposeLine(body, i, line, note)
		}
		if kind == game.DealSplit {
			body = append(body, "", theme.Subtle.Render("Your side: "+w.Describe(deals[m.proposeCursor])[len("a split: yours "):]))
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
		m.status = "No offer on the table."
		return
	}
	o := w.Offers[max(0, min(m.dealCursor, len(w.Offers)-1))]
	if accept {
		if _, err := w.Accept(o.ID); err != nil {
			m.status = "Can't accept: " + err.Error()
			return
		}
		m.status = fmt.Sprintf("Accepted %s. It holds from tonight.", w.Describe(o.Deal))
		return
	}
	if _, err := w.Decline(o.ID); err != nil {
		m.status = "Can't decline: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Turned down %s.", w.Describe(o.Deal))
}

// trustBar is the rival's trust in you, as a bar.
func (m *Model) trustBar(width int) string {
	t := m.w.Rival.Trust
	style := theme.Rival
	switch {
	case t < 20:
		style = theme.Bad
	case t >= 60:
		style = theme.Good
	}
	return style.Render(sparkline.Bar(t/100, width, nil)) + style.Render(fmt.Sprintf(" %.0f", t))
}

func (m *Model) viewRivals() string {
	w := m.w
	r := w.Rival
	var b strings.Builder
	title := theme.PanelTitle.Render("RIVALS · " + w.Home().Name)
	if r.Arrived == 0 {
		b.WriteString(title + "\n\n" + theme.Subtle.Render("Nobody is contesting the city. Yet. When somebody does, this is where you talk to them.") + "\n")
		return b.String()
	}
	tun := m.set.Rivals.Tuning()
	corners := fmt.Sprintf("%d corners", w.RivalHeld())
	if w.RivalHeld() == 0 {
		corners = "run out of town"
	}
	b.WriteString(truncate(title+theme.Subtle.Render(fmt.Sprintf("  %s · %s · %s · muscle %d", m.rivalName(), m.personalityWord(), corners, r.Muscle)), m.width) + "\n\n")

	// Trust and the war, side by side.
	war := fmt.Sprintf("war %.0f/%.0f", r.War, tun.CrackdownThreshold)
	switch {
	case r.War >= tun.WarThreshold:
		war = theme.Bad.Render(war + " loud")
	case r.War > 0:
		war = theme.Warning.Render(war)
	default:
		war = theme.Subtle.Render("no war")
	}
	b.WriteString(truncate("  trust "+m.trustBar(max(6, min(20, m.width/4)))+"   "+war, m.width) + "\n")
	var mood string
	switch {
	case m.set.Rivals.Distrusted(w, w.Day+1):
		mood = theme.Bad.Render(fmt.Sprintf("  You broke a deal. They take nothing for %d more day(s).", r.Betrayed+m.set.Rivals.Diplomacy().DistrustDays-w.Day-1))
	case r.Trust >= 60:
		mood = theme.Subtle.Render("  They take you at your word. A deal is cheap to strike.")
	case r.Trust < 20:
		mood = theme.Subtle.Render("  They do not trust you. Keep a deal a while and that changes.")
	default:
		mood = theme.Subtle.Render("  Trust grows a little every day a deal holds and falls with every strike.")
	}
	b.WriteString(truncate(mood, m.width) + "\n\n")

	// Live deals.
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.Rivals).Render("DEALS") + "\n")
	if len(r.Deals) == 0 {
		b.WriteString(theme.Subtle.Render("  none. d proposes one.") + "\n")
	}
	for _, d := range r.Deals {
		term := "holds until broken"
		if d.Until > 0 {
			term = fmt.Sprintf("%d day(s) left", d.Left(w.Day))
		}
		who := "yours"
		if d.Offered {
			who = "theirs"
		}
		b.WriteString(truncate(fmt.Sprintf("  %-10s %s · %s · since day %d, %s", capitalize(d.Kind), w.Describe(d), term, d.Since, who), m.width) + "\n")
	}
	if p := w.Proposal; p != nil {
		b.WriteString(truncate(theme.Gold.Render(fmt.Sprintf("  Tonight    you propose %s; they answer in the morning, ~%.0f%%", w.Describe(*p), m.set.Rivals.Chance(w, *p)*100)), m.width) + "\n")
	}
	b.WriteString("\n")

	// Offers, with the cursor.
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.Rivals).Render("OFFERS") + "\n")
	if len(w.Offers) == 0 {
		b.WriteString(theme.Subtle.Render("  nothing on the table.") + "\n")
	}
	m.dealCursor = max(0, min(m.dealCursor, max(0, len(w.Offers)-1)))
	for i, o := range w.Offers {
		line := fmt.Sprintf("%-10s %s · %d day(s) to answer", capitalize(o.Deal.Kind), w.Describe(o.Deal), o.Expires-w.Day+1)
		if i == m.dealCursor {
			b.WriteString(truncate(theme.Gold.Render("▸ ")+theme.Selected.Render(line)+"  "+theme.Key.Render("y")+" accept "+theme.Key.Render("x")+" decline", m.width) + "\n")
		} else {
			b.WriteString(truncate("  "+line, m.width) + "\n")
		}
	}
	b.WriteString("\n")
	for _, l := range []string{
		"Truce or tribute: they stay off your corners. Split: off your side of the line.",
		"A push or a hit under a deal breaks it: trust hits the floor and they make a call.",
		"So does a missed tribute, or walking off a split corner. A warning does not.",
	} {
		b.WriteString(truncate(theme.Subtle.Render(l), m.width) + "\n")
	}
	if s := w.Stats; s.Deals+s.Betrayals+s.BetrayedBy > 0 {
		b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("Struck %d · refused %d · broken by you %d, by them %d · tribute paid %s", s.Deals, s.DealsRefused, s.Betrayals, s.BetrayedBy, cash(s.Tribute))), m.width) + "\n")
	}
	return b.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
