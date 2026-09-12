package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// stanceWord is a DA's ticket as the screen prints it.
func stanceWord(stance string) string {
	if stance == "law_and_order" {
		return "law-and-order"
	}
	return stance
}

// fundDialog is the state of the fund-a-city modal: which city and how
// much clean cash for goodwill on its first page, and while a campaign
// is open (#193) a second page, the campaign: which ticket and how much
// clean cash behind it. Enter on the last page gives both.
type fundDialog struct {
	city   int // index into CityOrder
	amt    numberField
	step   int // 0 the city and the goodwill, 1 the campaign
	ticket int // index into game.Tickets
	camp   numberField
	err    string
}

// askFund opens the fund dialog on the city you are in.
func (m *Model) askFund() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.CleanCash <= 0 {
		m.refuse("Can't fund a city: goodwill is bought with clean cash, and you have none.")
		return
	}
	m.fnd = fundDialog{amt: newNumberField("blank = up to 100"), camp: newNumberField("blank = nothing")}
	m.fnd.amt.money = true
	m.fnd.camp.money = true
	m.fnd.amt.Focus()
	for i, id := range m.w.CityOrder {
		if id == m.w.Player.Location {
			m.fnd.city = i
		}
	}
	m.mode = modeFund
}

// campaignOpen is whether the tickets are taking money today (#193):
// the fund dialog's second page exists while they are.
func (m *Model) campaignOpen() bool { return m.w.Law.CampaignOpen }

// fundTicket is the ticket the campaign page is turned to.
func (m *Model) fundTicket() string {
	m.fnd.ticket = max(0, min(m.fnd.ticket, len(game.Tickets)-1))
	return game.Tickets[m.fnd.ticket]
}

// fundAmount is what the first page gives for goodwill: the typed
// amount, or blank's most; an error if it does not read.
func (m *Model) fundAmount(c *game.City) (int, error) {
	return parseQtyInput(m.fnd.amt.Value(), m.maxFund(c))
}

// maxBack is what the campaign field's m fills in (#193): the clean cash
// left after the goodwill, up to what buys the city's campaign the
// whole swing over what it already holds. Blank is nothing.
func (m *Model) maxBack(c *game.City) int {
	left := m.w.Player.CleanCash
	if amt, err := m.fundAmount(c); err == nil {
		left -= amt
	}
	room := m.set.Law.Campaign().Fill() - m.w.Campaigning(c.ID).Cash
	return max(0, min(left, room))
}

// backAmount is what the campaign page puts behind the ticket: the typed
// amount, blank being nothing.
func (m *Model) backAmount() (int, error) {
	n, ok := m.fnd.camp.Number()
	if !ok {
		return 0, fmt.Errorf("enter a whole number")
	}
	return n, nil
}

// fundCity is the city the fund dialog is turned to.
func (m *Model) fundCity() *game.City {
	if len(m.w.CityOrder) == 0 {
		return m.w.Here()
	}
	m.fnd.city = max(0, min(m.fnd.city, len(m.w.CityOrder)-1))
	return m.w.Cities[m.w.CityOrder[m.fnd.city]]
}

// maxFund is what a blank amount means and what the field's m fills in
// (#112): enough clean cash to take the city's goodwill to 100, or all
// of it if that is less. Goodwill stops at 100, so more would be given
// for nothing.
func (m *Model) maxFund(c *game.City) int {
	need := int((100 - c.Goodwill) * float64(m.set.Law.Tuning().GoodwillCash))
	return max(0, min(need, m.w.Player.CleanCash))
}

// keyFund is the fund dialog's keys: left and right turn the city on
// the first page and the ticket on the campaign page (#193), enter
// gives on the last page and goes forward otherwise, tab goes forward
// while the page is complete and shift+tab back (#110), esc closes.
func (m *Model) keyFund(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.fnd
	d.err = ""
	last := !m.campaignOpen() || d.step == 1
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "left", "[", "right", "]":
		n, at := len(m.w.CityOrder), &d.city
		if d.step == 1 {
			n, at = len(game.Tickets), &d.ticket
		}
		if n > 1 {
			if key == "left" || key == "[" {
				*at = (*at + n - 1) % n
			} else {
				*at = (*at + 1) % n
			}
		}
		return m, nil
	case "shift+tab":
		if d.step == 1 {
			d.step = 0
			d.camp.Blur()
			return m, d.amt.Focus()
		}
		return m, nil
	case "tab":
		if last {
			return m, nil
		}
		return m.nextFund()
	case "enter":
		if last {
			return m.confirmFund()
		}
		return m.nextFund()
	}
	c := m.fundCity()
	if d.step == 1 {
		d.camp.max = m.maxBack(c)
		return m, d.camp.Update(k)
	}
	d.amt.max = m.maxFund(c)
	return m, d.amt.Update(k)
}

// nextFund turns to the campaign page once the goodwill amount reads.
func (m *Model) nextFund() (tea.Model, tea.Cmd) {
	c := m.fundCity()
	if _, err := m.fundAmount(c); err != nil && strings.TrimSpace(m.fnd.amt.Value()) != "" {
		m.fnd.err = dialogError(err)
		return m, nil
	}
	m.fnd.step = 1
	m.fnd.amt.Blur()
	return m, m.fnd.camp.Focus()
}

// confirmFund gives what the pages hold: the goodwill amount, and on
// the campaign page whatever is behind the ticket. Blank goodwill is
// the most; blank campaign money is none; both nothing is refused.
func (m *Model) confirmFund() (tea.Model, tea.Cmd) {
	c := m.fundCity()
	amt, err := m.fundAmount(c)
	if err != nil && strings.TrimSpace(m.fnd.amt.Value()) != "" {
		m.fnd.err = dialogError(err)
		return m, nil
	}
	back := 0
	if m.fnd.step == 1 {
		if back, err = m.backAmount(); err != nil {
			m.fnd.err = dialogError(err)
			return m, nil
		}
	}
	if amt <= 0 && back <= 0 {
		m.fnd.err = "Nothing to give."
		return m, nil
	}
	if amt > 0 {
		if err := m.w.Fund(c.ID, amt); err != nil {
			m.fnd.err = dialogError(err)
			return m, nil
		}
	}
	if back > 0 {
		if err := m.w.Back(c.ID, m.fundTicket(), back); err != nil {
			m.fnd.err = dialogError(err)
			return m, nil
		}
	}
	m.mode = modePlay
	var said []string
	if amt > 0 {
		said = append(said, fmt.Sprintf("Gave %s %s clean. Goodwill +%.0f tonight; it takes the pressure off a little every day.", c.Name, money(amt), m.set.Law.Goodwill(amt)))
	}
	if back > 0 {
		camp := m.w.Campaigning(c.ID)
		said = append(said, fmt.Sprintf("Put %s clean behind the %s ticket in %s: the campaign holds %s, %s of the city's vote.", money(back), stanceWord(m.fundTicket()), c.Name, money(camp.Cash), swingWord(m.set.Law.Campaign().Swing(camp.Cash))))
	}
	m.say(strings.Join(said, " "))
	return m, nil
}

// swingWord is a campaign's pull on a city's vote, in points.
func swingWord(swing float64) string { return fmt.Sprintf("%.1f points", swing*100) }

func (m *Model) viewFund() string {
	w := m.w
	c := m.fundCity()
	tun := m.set.Law.Tuning()
	if m.fnd.step == 1 {
		return m.viewCampaign(c)
	}
	// The city is turned with left and right, so it is drawn as a dial.
	var cities []string
	for _, id := range w.CityOrder {
		cities = append(cities, w.CityName(id))
	}
	amt := m.fnd.amt
	amt.max = m.maxFund(c)
	body := []string{
		"City      " + dialCells(cities, m.fnd.city),
		fmt.Sprintf("Now       pressure %s  goodwill %s", theme.Bad.Render(fmt.Sprintf("%.0f", c.Pressure)), theme.Good.Render(fmt.Sprintf("%.0f", c.Goodwill))),
		fmt.Sprintf("Amount    %s   %s", amt.View(), theme.Subtle.Render("clean "+cash(w.Player.CleanCash))),
	}
	if amt, err := m.fundAmount(c); err == nil {
		g := m.set.Law.Goodwill(amt)
		style := theme.Gold
		if amt > w.Player.CleanCash {
			style = theme.Bad
		}
		body = append(body, fmt.Sprintf("Buys      %s goodwill for %s   %s", theme.Good.Render(fmt.Sprintf("+%.0f", g)), style.Render(money(amt)), theme.Subtle.Render(fmt.Sprintf("(%s a point, 100 at most)", money(tun.GoodwillCash)))))
	}
	body = append(body, "",
		theme.Subtle.Render(fmt.Sprintf("Full goodwill takes %.1f pressure off the city a day; it fades %.0f%% a day.", tun.GoodwillCut, tun.GoodwillDecay*100)),
		theme.Subtle.Render("Community centres, campaigns, benevolent funds: clean money only."))
	if m.campaignOpen() {
		if next := m.set.Law.NextElection(w); next > 0 {
			body = append(body, theme.Gold.Render(fmt.Sprintf("DA race in %s: the tickets are taking money on the next page.", plural(max(0, next-w.Day), "day"))))
		}
	}
	if m.fnd.err != "" {
		body = append(body, "", theme.Bad.Render(m.fnd.err))
	}
	return m.modal("FUND · "+c.Name, body, m.modalFooter())
}

// viewCampaign is the fund dialog's second page (#193): the ticket as a
// dial, what the city's campaign holds, and the amount to add.
func (m *Model) viewCampaign(c *game.City) string {
	w := m.w
	cmp := m.set.Law.Campaign()
	var tickets []string
	for _, t := range game.Tickets {
		tickets = append(tickets, stanceWord(t))
	}
	camp := w.Campaigning(c.ID)
	holds := theme.Subtle.Render("nothing yet")
	switch {
	case camp.Hedged:
		holds = theme.Bad.Render(fmt.Sprintf("%s on both tickets: it buys nothing", money(camp.Cash)))
	case camp.Cash > 0:
		holds = fmt.Sprintf("%s behind %s, %s of the vote", theme.Gold.Render(money(camp.Cash)), stanceWord(camp.Ticket), theme.Good.Render(swingWord(cmp.Swing(camp.Cash))))
	}
	given := 0
	if amt, err := m.fundAmount(c); err == nil {
		given = amt
	}
	field := m.fnd.camp
	field.max = m.maxBack(c)
	body := []string{
		"Ticket    " + dialCells(tickets, m.fnd.ticket),
		"Campaign  " + holds,
		fmt.Sprintf("Amount    %s   %s", field.View(), theme.Subtle.Render("clean "+cash(w.Player.CleanCash-given))),
	}
	if back, err := m.backAmount(); err == nil && back > 0 {
		style := theme.Gold
		if back > w.Player.CleanCash-given {
			style = theme.Bad
		}
		total := camp.Cash + back
		body = append(body, fmt.Sprintf("Buys      %s of %s's vote for %s   %s", theme.Good.Render(swingWord(cmp.Swing(total))), c.Name, style.Render(money(back)), theme.Subtle.Render(fmt.Sprintf("(%s a point, %.0f at most)", money(cmp.Cash), cmp.SwingMax*100))))
	}
	body = append(body, "",
		theme.Subtle.Render("A winner you backed owes you: the sting line sits higher. A loser's rival knows who paid."),
		theme.Subtle.Render(fmt.Sprintf("Money on both tickets buys nothing; a campaign adds %.1f pressure a day.", cmp.Pressure)))
	if m.fnd.err != "" {
		body = append(body, "", theme.Bad.Render(m.fnd.err))
	}
	return m.modal("FUND · "+c.Name+" · campaign", body, m.modalFooter())
}

// lawReportStyle is the colour the report's LAW section is printed in.
var lawReportStyle = theme.LawText
