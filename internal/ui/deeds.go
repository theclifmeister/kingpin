package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The property (#194): d on the map with a corner under the cursor
// opens the confirmation for the block it is on (the price, the rent,
// what changes, the DA's line), y buys it with clean cash; the ledger's
// PROPERTY block lists the deeds under the one cursor and the pane
// shows the selected one.

// deedGlyph marks a block whose deed is yours on the map, in place of
// the owner's mark: the colour still says whose the corner is.
const deedGlyph = "⌂"

// askDeed opens the confirmation on the corner under the map cursor;
// silent on the routes, refused on a block already yours or where none
// is on sale.
func (m *Model) askDeed() {
	if m.w.Over != nil {
		return
	}
	if m.onRoutes {
		m.refuse("Pick a corner: the block is under it, not the road.")
		return
	}
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Deed != nil {
		m.refuse(fmt.Sprintf("The block %s is on is yours already, since day %d.", c.Name, c.Deed.Bought))
		return
	}
	if m.rules.Territory.DeedPrice(m.w, *c) <= 0 {
		m.refuse("Can't buy: " + game.ErrNoDeeds.Error() + ".")
		return
	}
	m.ask("buy", (*Model).deedModal, (*Model).confirmDeed)
}

// deedModal is the confirmation's modal over deedConfirm's body.
func (m *Model) deedModal() string {
	return m.modal("BUY THE BLOCK?", m.deedConfirm(), m.modalFooter())
}

// confirmDeed buys the block under the selected corner.
func (m *Model) confirmDeed() {
	c := m.mapSelected()
	m.mode = modePlay
	if c == nil {
		return
	}
	price := m.rules.Territory.DeedPrice(m.w, *c)
	if err := m.sess.BuyDeed(c.ID); err != nil {
		m.refuse("Can't buy the block: " + err.Error() + ".")
		return
	}
	line := fmt.Sprintf("The block %s is on is yours: %s clean. It pays %s/day.", c.Name, money(price), money(m.rules.Territory.DeedRent(m.w, *c, price)))
	if m.rules.Law.Forfeits(m.w) {
		m.alarm(line + " The DA will ask where the money came from.")
		return
	}
	m.say(line)
}

// deedConfirm is the confirmation's body: what the block costs and pays,
// what the deed changes on it by whose corner it is, what it adds to the
// city's pressure, and the DA's line: what the deeds held would cost
// against what the fronts have washed.
func (m *Model) deedConfirm() []string {
	c := m.mapSelected()
	if c == nil {
		return []string{"No corner."}
	}
	w := m.w
	tr := m.rules.Territory
	tun := tr.Deeds()
	price := tr.DeedPrice(w, *c)
	rent := tr.DeedRent(w, *c, price)
	body := m.wrapLines(fmt.Sprintf("Buy the block %s is on%s for %s, clean: %s of its street trade, at today's prices. It pays %s/day clean for as long as you hold it.", c.Name, m.inCity(c.City), money(price), plural(int(tun.Days), "day"), money(rent)))
	body = append(body, "")
	switch {
	case c.Held():
		body = append(body, row("robbery", fmt.Sprintf("×%s on this corner", times(tun.RobberyMul))), row("pushes", fmt.Sprintf("×%s on this corner (never 0)", times(tun.PushMul))))
	case c.Owner == game.OwnerRival:
		body = append(body, row("theirs", fmt.Sprintf("%s's defence of it ×%s: the businessman's way to a corner", m.factionOf(c).Leader, times(tun.PushMul))))
	default:
		body = append(body, row("free", fmt.Sprintf("robbery ×%s and pushes ×%s once you hold it", times(tun.RobberyMul), times(tun.PushMul))))
	}
	if n := m.housesOn(c.ID); n > 0 {
		body = append(body, row("houses", fmt.Sprintf("%s here weigh ×%s in the raid's roll", plural(n, "house"), times(tun.RaidMul))))
	}
	if tun.Pressure > 0 {
		body = append(body, row("pressure", fmt.Sprintf("+%s a day in %s: deeds are public record", times(tun.Pressure), w.CityName(c.City))))
	}
	body = append(body, "")
	held, limit := w.DeedValue(), m.rules.Law.DeedLimit(w)
	line := fmt.Sprintf("The DA's line: %s in deeds against %s washed lets you hold %s. With this one you would hold %s.", money(held), money(w.Stats.Laundered), money(limit), money(held+price))
	if held+price > limit {
		for _, l := range m.wrapLines(line + fmt.Sprintf(" Over it the DA seizes the newest deed tonight and files %s in the morning.", plural(m.rules.Heat.ForfeitEvidence(), "page"))) {
			body = append(body, theme.Bad.Render(l))
		}
	} else {
		body = append(body, m.wrapLines(line)...)
	}
	if n := w.DeedsIn(c.City) + 1; tun.HeadlineDeeds > 0 && n >= tun.HeadlineDeeds {
		body = append(body, theme.Warning.Render(fmt.Sprintf("Your %s in %s: this one makes the paper.", ordinal(n)+" block", w.CityName(c.City))))
	}
	body = append(body, "", m.inHand())
	return body
}

// inCity names a city for a line about somewhere other than where you
// stand, the report's way.
func (m *Model) inCity(city string) string {
	if city == m.w.Player.Location {
		return ""
	}
	return " in " + m.w.CityName(city)
}

// housesOn counts the stash houses on a block.
func (m *Model) housesOn(corner string) int {
	n := 0
	for _, h := range m.w.Houses {
		if h.Corner == corner {
			n++
		}
	}
	return n
}

// ordinal is 1st, 2nd, 3rd, 4th.
func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// deedCols is the ledger's PROPERTY table: the block, its city, whose
// corner it is, what it cost, what it pays a day and since when.
var deedCols = []col{{"block", kText, 0}, {"city", kText, 0}, {"corner", kText, 0}, {"cost", kMoney, 0}, {"rent/day", kMoney, 0}, {"since", kText, 0}}

// deedRow is a deed's PROPERTY row.
func (m *Model) deedRow(c game.Corner) []any {
	return []any{c.Name, m.w.CityName(c.City), m.cornerWhose(c), c.Deed.Price, m.rules.Territory.DeedRent(m.w, c, c.Deed.Price), fmt.Sprintf("day %d", c.Deed.Bought)}
}

// cornerWhose is whose a corner is, for a table cell: yours, the
// faction's leader, or free.
func (m *Model) cornerWhose(c game.Corner) any {
	switch c.Owner {
	case game.OwnerPlayer:
		return styled{theme.CrewText, "yours"}
	case game.OwnerRival:
		return styled{m.factionStyle(c.Faction), m.factionOf(&c).Leader + "'s"}
	}
	return styled{theme.Subtle, "free"}
}

// deedNote is the PROPERTY heading's note: the deeds held, what they
// cost, what they pay a day, and the DA's line.
func (m *Model) deedNote() string {
	w := m.w
	deeds := w.Deeds()
	if len(deeds) == 0 {
		return ""
	}
	rent := 0
	for _, c := range deeds {
		rent += m.rules.Territory.DeedRent(w, c, c.Deed.Price)
	}
	note := fmt.Sprintf(" · %s · %s · %s/day", plural(len(deeds), "block"), cash(w.DeedValue()), cash(rent))
	if m.rules.Law.Forfeits(w) {
		return note + " · " + theme.Bad.Render("over the DA's line")
	}
	return note + fmt.Sprintf(" · DA's line %s", cash(m.rules.Law.DeedLimit(w)))
}

// deedSection is a deed's detail in the pane: whose corner it is, what
// it cost and pays, what it does there, and the DA's line.
func (m *Model) deedSection(c game.Corner) section {
	w := m.w
	tr := m.rules.Territory
	tun := tr.Deeds()
	whose, _ := cellText(kText, 0, m.cornerWhose(c))
	lines := []string{
		m.cornerWhose(c).(styled).st.Render(whose),
		row("city", w.CityName(c.City)),
		row("since", fmt.Sprintf("day %d · %s", c.Deed.Bought, money(c.Deed.Price))),
		row("rent", money(tr.DeedRent(w, c, c.Deed.Price))+"/day clean"),
	}
	switch {
	case c.Held():
		lines = append(lines, row("robbery", fmt.Sprintf("%s/day (×%s)", pctText(tr.RobberyChance(w, &c)*100), times(tun.RobberyMul))), row("pushes", fmt.Sprintf("×%s on this corner", times(tun.PushMul))))
	case c.Owner == game.OwnerRival:
		lines = append(lines, row("defence", fmt.Sprintf("theirs ×%s: strike it", times(tun.PushMul))))
	}
	if n := m.housesOn(c.ID); n > 0 {
		lines = append(lines, row("houses", fmt.Sprintf("%d here · raid ×%s", n, times(tun.RaidMul))))
	}
	held, limit := w.DeedValue(), m.rules.Law.DeedLimit(w)
	if m.rules.Law.Forfeits(w) {
		lines = append(lines, m.wrapped(theme.Bad, fmt.Sprintf("Over the DA's line: %s in deeds against %s allowed. The newest goes tonight.", money(held), money(limit)))...)
	} else {
		lines = append(lines, row("DA's line", fmt.Sprintf("%s of %s", money(held), money(limit))))
	}
	return section{strings.ToUpper(c.Name), lines}
}
