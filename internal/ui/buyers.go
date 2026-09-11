package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// buyerRows are the contracts the market screen lists for the city
// shown: open offers and the ones you hold, oldest first.
func (m *Model) buyerRows() []game.Contract { return m.w.ContractsIn(m.city) }

// selectedContract is the contract under the buyers cursor, or nil when
// the cursor is on the product table or there is nothing to select.
func (m *Model) selectedContract() *game.Contract {
	rows := m.buyerRows()
	if !m.onBuyers || len(rows) == 0 {
		return nil
	}
	if m.buyerCursor >= len(rows) {
		m.buyerCursor = len(rows) - 1
	}
	c := rows[m.buyerCursor]
	return &c
}

// urgency colours a contract by the days it has left.
func urgency(days int) lipgloss.Style {
	switch {
	case days <= 1:
		return theme.Bad
	case days <= 3:
		return theme.Warning
	default:
		return theme.Good
	}
}

// buyersLines is the BUYERS panel under the market's product table: every
// offer and contract in the city shown with its terms, what it pays at
// today's street and the days it has left, the selected one marked.
func (m *Model) buyersLines() []string {
	w := m.w
	rows := m.buyerRows()
	title := sectionTitle("BUYERS", theme.Market) + theme.Subtle.Render(" · "+m.shown().Name)
	if len(rows) == 0 {
		return []string{title, theme.Subtle.Render("Nobody is asking. Offers come here and lapse in a few days.")}
	}
	out := []string{title}
	for i, c := range rows {
		cur := "  "
		name := fit(c.Name, 22)
		if m.onBuyers && i == m.buyerCursor {
			cur = theme.Gold.Render("▸ ")
			name = theme.Selected.Render(name)
		}
		left := c.DaysLeft(w.Day)
		days := urgency(left).Render(fmt.Sprintf("%dd left", left))
		if left <= 1 {
			days = urgency(left).Render("last day")
		}
		pays := fmt.Sprintf("%s (×%.3g)", price(m.set.Market.ContractPrice(w, c)), c.Premium)
		var state string
		switch c.Status {
		case game.ContractOffered:
			state = theme.Gold.Render("offer")
		default:
			state = fmt.Sprintf("%d/%d delivered", c.Delivered, c.Units)
			if q := w.QueuedDelivery(c.ID); q > 0 {
				state += theme.Gold.Render(fmt.Sprintf(", %d tonight", q))
			}
		}
		out = append(out, truncate(fmt.Sprintf("%s%s %4d %-8s %-12s %s  %s", cur, name, c.Units, fit(w.ProductName(c.Product), 8), pays, days, state), m.mainWidth()))
	}
	return out
}

// contractSections is the market's pane when the cursor is on a
// contract instead of a product: the pitch, the terms, what a handoff
// would pay and draw tonight.
func (m *Model) contractSections(c game.Contract) []section {
	w := m.w
	pays := m.set.Market.ContractPrice(w, c)
	sel := []string{
		row("status", c.Status.String()),
		row("wants", fmt.Sprintf("%d %s", c.Units, w.ProductName(c.Product))),
		row("pays", fmt.Sprintf("%s a unit today", price(pays))),
		row("", theme.Subtle.Render(fmt.Sprintf("×%.3g the street (%s)", c.Premium, price(pays/c.Premium)))),
		row("", theme.Subtle.Render(fmt.Sprintf("%s when they asked", price(c.Street)))),
	}
	switch c.Status {
	case game.ContractOffered:
		sel = append(sel,
			row("by", fmt.Sprintf("day %d (%s)", c.Due, plural(c.Due-w.Day, "day"))),
			row("lapses", fmt.Sprintf("after day %d", c.Expires)))
	default:
		sel = append(sel,
			row("owed", fmt.Sprintf("%d of %d by day %d", c.Owed(), c.Units, c.Due)),
			row("stash", fmt.Sprintf("%d %s here", w.Stock(c.City, c.Product), w.ProductName(c.Product))))
		n := w.Deliverable(c)
		if q := w.QueuedDelivery(c.ID); q > 0 {
			n = q
		}
		if n > 0 {
			heat := m.set.Heat.ContractHeat(w, c.City, c.Product, n, c.HeatMul)
			sel = append(sel,
				row("tonight", fmt.Sprintf("%d for ~%s", n, money(int(pays*float64(n))))),
				row("heat", fmt.Sprintf("+%.1f in %s", heat, w.CityName(c.City))))
		}
	}
	secs := []section{{strings.ToUpper(c.Name), sel}}
	var notes []string
	notes = append(notes, wrapped(theme.Subtle, c.Pitch)...)
	if c.Status == game.ContractOffered {
		notes = append(notes, wrapped(theme.Warning, fmt.Sprintf("If you fail: respect -%.0f, notoriety +%.0f, and they collect %.0f%% of what is short; they stay away %s.",
			c.Penalty, m.set.Market.BuyersTuning().NotorietyPenalty, c.PenaltyCash*100, plural(m.set.Market.BuyersTuning().BlacklistDays, "day")))...)
	} else {
		notes = append(notes, wrapped(theme.Subtle, "A handoff needs no corner and no dial.")...)
	}
	if c.City != w.Player.Location {
		notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("The handoff is in %s: you have to be there, with the stock in the stash there.", w.CityName(c.City)))...)
	}
	return append(secs, section{"NOTES", notes})
}

// buyersMove moves the buyers cursor by d, or hands the arrows back to
// the product table off the top.
func (m *Model) buyersMove(d int) {
	rows := m.buyerRows()
	if len(rows) == 0 {
		m.onBuyers = false
		return
	}
	next := m.buyerCursor + d
	switch {
	case next < 0:
		m.onBuyers = false
		m.buyerCursor = 0
	case next >= len(rows):
		m.buyerCursor = len(rows) - 1
	default:
		m.buyerCursor = next
	}
}

// answerContract accepts or declines the selected offer.
func (m *Model) answerContract(accept bool) {
	c := m.selectedContract()
	if c == nil {
		return
	}
	if c.Status != game.ContractOffered {
		m.refuse("Can't answer that: it is yours already, deliver it from the stash.")
		return
	}
	var err error
	if accept {
		err = m.w.AcceptContract(c.ID)
	} else {
		err = m.w.DeclineContract(c.ID)
	}
	if err != nil {
		m.refuse("Can't answer that: " + err.Error())
		return
	}
	if accept {
		m.say(fmt.Sprintf("Taken: %d %s to %s by day %d. Deliver from the stash in %s.", c.Units, m.w.ProductName(c.Product), c.Name, c.Due, m.w.CityName(c.City)))
	} else {
		m.say(fmt.Sprintf("Declined %s's offer.", c.Name))
	}
}

// deliverSelected queues everything in the stash against the selected
// contract for tonight, or explains why not.
func (m *Model) deliverSelected() {
	c := m.selectedContract()
	if c == nil {
		return
	}
	if c.Status == game.ContractOffered {
		m.refuse("Nothing to deliver: take the offer first.")
		return
	}
	n := m.w.Deliverable(*c)
	if n <= 0 {
		m.refuse(fmt.Sprintf("Nothing to hand over: no %s in the stash in %s.", m.w.ProductName(c.Product), m.w.CityName(c.City)))
		return
	}
	if err := m.w.Deliver(c.ID, n); err != nil {
		if err == game.ErrElsewhere {
			m.refuse(fmt.Sprintf("Can't deliver from here: the handoff is in %s, go there.", m.w.CityName(c.City)))
		} else {
			m.refuse("Can't deliver: " + err.Error())
		}
		return
	}
	m.say(fmt.Sprintf("%d %s go to %s tonight for about %s.", n, m.w.ProductName(c.Product), c.Name, money(int(m.set.Market.ContractPrice(m.w, *c)*float64(n)))))
}

// contractsLine is the dashboard's one line on the buyers: how many
// contracts you hold and which are due, or an offer waiting. Empty when
// there is nothing to say.
func (m *Model) contractsLine() string {
	w := m.w
	live, today, tomorrow := w.ContractsDue()
	offers := 0
	for _, c := range w.Contracts {
		if c.Open(w.Day) {
			offers++
		}
	}
	if live == 0 && offers == 0 {
		return ""
	}
	var parts []string
	if live > 0 {
		s := plural(live, "contract")
		switch {
		case today > 0:
			s += theme.Bad.Render(fmt.Sprintf(", %d due today", today))
		case tomorrow > 0:
			s += theme.Warning.Render(fmt.Sprintf(", %d due tomorrow", tomorrow))
		}
		parts = append(parts, s)
	}
	if offers > 0 {
		parts = append(parts, theme.Gold.Render(plural(offers, "offer")+" "+screenPointer(screenMarket)))
	}
	return strings.Join(parts, " · ")
}

// contractAlerts are the buyers' lines for the dashboard's ALERTS panel:
// a contract due today or tomorrow.
func (m *Model) contractAlerts() []string {
	w := m.w
	var out []string
	for _, c := range w.Contracts {
		if c.Status != game.ContractAccepted || c.Due > w.Day+1 {
			continue
		}
		when := "tomorrow"
		style := theme.Warning
		if c.Due <= w.Day {
			when, style = "today", theme.Bad
		}
		out = append(out, style.Render(fmt.Sprintf("%s: %d %s due %s in %s", c.Name, c.Owed(), w.ProductName(c.Product), when, w.CityName(c.City))))
	}
	return out
}
