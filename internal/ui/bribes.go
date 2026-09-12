package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The bought law (#42): the bribe dialog ($ on the ledger: the target,
// then the amount), the checkpoint confirmation ($ on the map with the
// routes cursor on an edge) and the ledger's PAYOFFS block, under the
// one ledger cursor, with the pane's section for the selected deal.

// bribeDialog is the state of the bribe dialog: the target under the
// cursor on its first page, the amount on its second.
type bribeDialog struct {
	step   int // 0 the target, 1 the amount
	cursor int // index into bribeTargets
	amt    numberField
	err    string
}

// bribeTargets are the dialog's rows, in order.
var bribeTargets = []string{game.BribeChief, game.BribeDA}

// askBribe opens the bribe dialog on the chief.
func (m *Model) askBribe() {
	if m.w.Over != nil {
		return
	}
	if m.w.Player.DirtyCash <= 0 {
		m.refuse("Can't bribe anybody: an envelope is dirty cash, and you have none.")
		return
	}
	m.br = bribeDialog{amt: newNumberField("blank = the price")}
	m.br.amt.money = true
	m.mode = modeBribe
}

// bribeTarget is the target under the dialog's cursor.
func (m *Model) bribeTarget() string {
	m.br.cursor = max(0, min(m.br.cursor, len(bribeTargets)-1))
	return bribeTargets[m.br.cursor]
}

// bribePrice is what the target wants: the chief's price, or the DA's
// (halved under one you backed, less the fixer's cut).
func (m *Model) bribePrice(target string) int {
	if target == game.BribeDA {
		return m.set.Law.DAPrice(m.w)
	}
	return m.set.Law.Bribes().ChiefPrice
}

// bribeMax is what the amount field's m fills in: the price, up to the
// dirty cash in hand.
func (m *Model) bribeMax() int {
	return max(0, min(m.bribePrice(m.bribeTarget()), m.w.Player.DirtyCash))
}

// bribeAmount is what the field reads: the price for a blank.
func (m *Model) bribeAmount() (int, error) {
	if strings.TrimSpace(m.br.amt.Value()) == "" {
		return m.bribePrice(m.bribeTarget()), nil
	}
	n, ok := m.br.amt.Number()
	if !ok || n <= 0 {
		return 0, fmt.Errorf("enter a whole number above zero")
	}
	return n, nil
}

// officialName is the chief's or the DA's name with their title.
func (m *Model) officialName(target string) string {
	if target == game.BribeDA {
		return "DA " + m.w.Law.DA.Name
	}
	return "Chief " + m.w.Law.Chief.Name
}

// keyBribe is the dialog's keys: the cursor on the first page, the
// number field on the second, enter forward then pay, shift+tab back
// (#110), esc closes from either.
func (m *Model) keyBribe(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.br
	d.err = ""
	switch key {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "shift+tab":
		if d.step == 1 {
			d.step = 0
			d.amt.Blur()
		}
		return m, nil
	case "tab":
		if d.step == 0 {
			d.step = 1
			return m, d.amt.Focus()
		}
		return m, nil
	case "enter":
		if d.step == 0 {
			d.step = 1
			return m, d.amt.Focus()
		}
		m.confirmBribe()
		return m, nil
	}
	if d.step == 0 {
		switch key {
		case "up", "k":
			d.cursor = max(0, d.cursor-1)
		case "down", "j":
			d.cursor = min(len(bribeTargets)-1, d.cursor+1)
		case "1", "2":
			d.cursor = int(key[0] - '1')
		}
		return m, nil
	}
	d.amt.max = m.bribeMax()
	return m, d.amt.Update(k)
}

// confirmBribe pays the envelope, or shows why it cannot.
func (m *Model) confirmBribe() {
	target := m.bribeTarget()
	amt, err := m.bribeAmount()
	if err != nil {
		m.br.err = dialogError(err)
		return
	}
	if err := m.w.Bribe(target, amt); err != nil {
		m.br.err = dialogError(err)
		return
	}
	m.mode = modePlay
	m.say(fmt.Sprintf("%s in an envelope for %s. You hear tonight.", money(amt), m.officialName(target)))
}

// viewBribe is the dialog: the two targets with what is known of them
// and their price, then the amount with the odds.
func (m *Model) viewBribe() string {
	w := m.w
	lw := m.set.Law
	tun := lw.Bribes()
	if m.br.step == 1 {
		target := m.bribeTarget()
		price := m.bribePrice(target)
		field := m.br.amt
		field.max = m.bribeMax()
		amt, err := m.bribeAmount()
		if err != nil {
			amt = price
		}
		body := []string{
			"For       " + m.officialName(target) + theme.Subtle.Render("  · price "+money(price)),
			fmt.Sprintf("Amount    %s   %s", field.View(), theme.Subtle.Render("dirty "+cash(w.Player.DirtyCash))),
			"",
		}
		body = append(body, m.wrapLines(m.bribeOdds(target, amt))...)
		if m.br.err != "" {
			body = append(body, "", theme.Bad.Render(m.br.err))
		}
		return m.modal("BRIBE · "+m.officialName(target), body, m.modalFooter())
	}
	var body []string
	for i, target := range bribeTargets {
		cur := "  "
		if i == m.br.cursor {
			cur = "▸ "
		}
		name := m.officialName(target)
		known := m.officialWord(target)
		line := fmt.Sprintf("%s%d  %-26s %-14s %s", cur, i+1, name, known, theme.Subtle.Render(money(m.bribePrice(target))))
		if i == m.br.cursor {
			line = theme.Selected.Render(line)
		}
		body = append(body, line)
	}
	body = append(body, "")
	body = append(body, m.wrapLines(fmt.Sprintf("A corrupt chief takes it: heat fades %.1fx faster and stings and raids come %s later for %s. A lazy one takes half the good. A zealous one, or a law-and-order DA, files it: a page and heat %.0f in the morning.", m.set.Heat.BribeDecayMul(), plural(m.set.Heat.BribeCooldown(), "day"), plural(tun.BribeDays, "day"), tun.BackfireHeat))...)
	body = append(body, m.wrapLines(fmt.Sprintf("A DA who takes it needs a thicker file to indict for %s. Every envelope taken is a lead; at %d the DA opens a file. Dirty cash only.", plural(tun.BribeDays, "day"), tun.LeadsCase))...)
	if w.Cold() {
		body = append(body, "", theme.Bad.Render("A law-and-order DA sits: the chief is not taking calls."))
	}
	if m.br.err != "" {
		body = append(body, "", theme.Bad.Render(m.br.err))
	}
	return m.modal("BRIBE", body, m.modalFooter())
}

// officialWord is what you know of an official for the picker: the
// chief's personality once observed, the DA's ticket, and `bought` while
// they are.
func (m *Model) officialWord(target string) string {
	l := m.w.Law
	if target == game.BribeDA {
		word := stanceWord(l.DA.Stance)
		if l.DA.Backed {
			word += ", yours"
		}
		if l.DABoughtOn(m.w.Day) {
			word = "bought"
		}
		return word
	}
	word := "new"
	if l.Chief.Observed {
		word = l.Chief.Personality
	}
	if l.ChiefBoughtOn(m.w.Day) {
		word = "bought"
	}
	return word
}

// bribeOdds is the sentence under the amount: what the envelope is
// likely to do, as far as you know.
func (m *Model) bribeOdds(target string, amt int) string {
	l := m.w.Law
	lw := m.set.Law
	tun := lw.Bribes()
	if target == game.BribeDA {
		switch {
		case l.DA.Backed:
			return fmt.Sprintf("~%.0f%% they take it: they owe you the election. Refused, the money is gone and nothing else happens.", lw.DAOdds(m.w, amt)*100)
		case l.DA.Stance == "law_and_order":
			return "A law-and-order DA does not take envelopes. This one goes in an evidence bag."
		case l.DA.Stance == "reform":
			return "A reformer sends it back with no note. Nothing happens, and the money is gone."
		}
		return fmt.Sprintf("~%.0f%% they take it (%s is even odds). Refused, the money is gone and nothing else happens.", lw.DAOdds(m.w, amt)*100, money(lw.DAPrice(m.w)))
	}
	if amt < tun.ChiefPrice {
		return theme.Warning.Render(fmt.Sprintf("Under the price (%s): the chief's people pocket it and nothing changes.", money(tun.ChiefPrice)))
	}
	if l.Chief.Observed {
		switch l.Chief.Personality {
		case "zealous":
			return theme.Bad.Render("Chief " + l.Chief.Name + " is zealous: this goes in an evidence bag, a page in the file and heat in the morning.")
		case "lazy":
			return "A lazy chief takes it at half the good: heat fades a little faster and the stings come a little later."
		}
		return "A corrupt chief takes it: heat fades faster and stings and raids come later while it holds."
	}
	return "You have not seen this chief work: a corrupt one takes it, a lazy one takes half the good, a zealous one files it."
}

// askCheckpoint opens the checkpoint confirmation on the route under
// the map's routes cursor ($ is listed there and silent on the grid:
// mapOnRoutes).
func (m *Model) askCheckpoint() {
	if m.w.Over != nil {
		return
	}
	if m.selectedRoute() == nil {
		m.refuse("Nothing to buy: no route out of " + m.shown().Name + ".")
		return
	}
	m.mode = modeConfirmCheckpoint
}

// checkpointWord is the selected route's deal, for the confirmation's
// title.
func (m *Model) checkpointWord() string {
	if r := m.selectedRoute(); r != nil {
		return dealWord(*r)
	}
	return "checkpoint"
}

// dealWord is what a route's deal is called: a checkpoint on the road,
// a customs agent at the water.
func dealWord(r content.RouteConfig) string {
	if logistics.Customs(r) {
		return "customs agent"
	}
	return "checkpoint"
}

// dealPrice is what the route's deal costs.
func (m *Model) dealPrice(r content.RouteConfig) int {
	if logistics.Customs(r) {
		return m.set.Law.Bribes().CustomsPrice
	}
	return m.set.Law.Bribes().CheckpointPrice
}

// confirmCheckpoint buys the deal on the selected route.
func (m *Model) confirmCheckpoint() {
	r := m.selectedRoute()
	if r == nil {
		m.mode = modePlay
		return
	}
	tun := m.set.Law.Bribes()
	price := m.dealPrice(*r)
	if err := m.w.BuyCheckpoint(r.ID, price, tun.CheckpointDays); err != nil {
		m.mode = modePlay
		m.refuse("Can't buy the " + dealWord(*r) + ": " + err.Error() + ".")
		return
	}
	m.mode = modePlay
	until, _ := m.w.Checkpoint(r.ID)
	m.say(fmt.Sprintf("The %s on the %s is yours until day %d: %s. Risk on that edge cut %.0f%% while it holds.", dealWord(*r), r.Name, until, money(price), m.set.Logistics.DealCut(*r)*100))
}

// checkpointConfirm is the confirmation's body.
func (m *Model) checkpointConfirm() []string {
	r := m.selectedRoute()
	if r == nil {
		return []string{"No route."}
	}
	w := m.w
	lg := m.set.Logistics
	tun := m.set.Law.Bribes()
	d := w.Route(r.ID).Dial
	body := m.wrapLines(fmt.Sprintf("Buy the %s on the %s (%s %s %s) for %s, dirty: %s of the risk off every day on that edge for %s.", dealWord(*r), r.Name, w.CityName(r.From), edge(r.Mode), w.CityName(r.To), money(m.dealPrice(*r)), fmt.Sprintf("%.0f%%", lg.DealCut(*r)*100), plural(tun.CheckpointDays, "day")))
	if until, live := w.Checkpoint(r.ID); live {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Yours until day %d already; this adds to it.", until)))
	}
	if d.On() {
		body = append(body, "", row("seized now", fmt.Sprintf("~%.0f%% a run at %s", lg.Risk(w, *r, d.Ship())*100, d)))
	}
	body = append(body, "")
	for _, l := range m.wrapLines(fmt.Sprintf("A law-and-order DA taking office ends it within %s, and nothing is for sale while they sit.", plural(tun.CallsStopDays, "day"))) {
		body = append(body, theme.Warning.Render(l))
	}
	if w.Cold() {
		body = append(body, theme.Bad.Render("A law-and-order DA sits: nobody is taking calls."))
	}
	return body
}

// The ledger's PAYOFFS block: the live deals, one row each, under the
// ledger's cursor (kind ledgerPayoff). payoffRows lists them in a fixed
// order: the chief, the DA, then the routes in ledger order.

// payoff is one row of the block: who, what it does, and the day it
// runs out; Route is set for a road deal.
type payoff struct {
	Who   string
	What  string
	Until int
	Route *content.RouteConfig
}

// payoffRows are the live deals.
func (m *Model) payoffRows() []payoff {
	w := m.w
	var rows []payoff
	if w.Law.ChiefBoughtOn(w.Day) {
		what := "heat fades faster, stings and raids come later"
		if w.Law.ChiefShare < 1 {
			what = "half the good: a lazy chief"
		}
		rows = append(rows, payoff{Who: "Chief " + w.Law.Chief.Name, What: what, Until: w.Law.ChiefBought})
	}
	if w.Law.DABoughtOn(w.Day) {
		rows = append(rows, payoff{Who: "DA " + w.Law.DA.Name, What: "a thicker file to indict", Until: w.Law.DABought})
	}
	for _, r := range m.ledgerRoutes() {
		if until, live := w.Checkpoint(r.ID); live {
			rc := r
			rows = append(rows, payoff{Who: r.Name, What: dealWord(r) + fmt.Sprintf(", risk cut %.0f%%", m.set.Logistics.DealCut(r)*100), Until: until, Route: &rc})
		}
	}
	return rows
}

// payoffCols are the block's columns.
var payoffCols = []col{{"who", kText, 0}, {"what", kText, 0}, {"days left", kText, 0}}

// payoffTable is the block's rows as the table draws them.
func (m *Model) payoffTable(rows []payoff) [][]any {
	var out [][]any
	for _, p := range rows {
		out = append(out, []any{p.Who, p.What, plural(max(0, p.Until-m.w.Day), "day")})
	}
	return out
}

// payoffNote is the block's heading note: what the DA's office has
// heard, if a fixer is on the payroll to tell you.
func (m *Model) payoffNote() string {
	if f := m.w.Crew.Fixer(); f != nil {
		return fmt.Sprintf(" · %s says the DA has %s", f.Name, plural(m.w.Law.Leads, "lead"))
	}
	return ""
}

// payoffSection is the pane's section for the selected deal.
func (m *Model) payoffSection(p payoff) section {
	w := m.w
	lines := []string{
		row("what", p.What),
		row("until", fmt.Sprintf("day %d · %s left", p.Until, plural(max(0, p.Until-w.Day), "day"))),
	}
	if p.Route != nil {
		r := *p.Route
		lg := m.set.Logistics
		d := w.Route(r.ID).Dial
		lines = append(lines, row("edge", fmt.Sprintf("%s %s %s", w.CityName(r.From), edge(r.Mode), w.CityName(r.To))), row("seized", fmt.Sprintf("~%.0f%% a run at %s", lg.Risk(w, r, d.Ship())*100, d)))
	}
	if w.Cold() {
		lines = append(lines, theme.Bad.Render("A law-and-order DA sits: it ends within the week."))
	}
	return section{strings.ToUpper(p.Who), lines}
}
