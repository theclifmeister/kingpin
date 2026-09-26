package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The trophies (#392, docs/trophies.md) on the ledger. The TROPHIES
// block sits under EXPORTS once a trophy is owned or lost or the first
// line is within reach (engine.GateNear of it to go on peak clean
// cash): the trophies owned, then the ones on offer, under the ledger's
// one cursor. `t` on an offer buys it, after asking.

// trophyRows are the trophies on offer you do not own, cheapest first.
func (m *Model) trophyRows() []game.TrophyOffer { return m.sess.TrophyOffers() }

// trophiesShown reports whether the ledger carries the TROPHIES block,
// so a run nowhere near the money reads the ledger it always did.
func (m *Model) trophiesShown() bool {
	w := m.w
	if len(w.Trophies) > 0 || len(w.TrophiesLost) > 0 {
		return true
	}
	for _, o := range m.trophyRows() {
		if float64(o.UnlockCash-w.Stats.PeakClean) < float64(o.UnlockCash)*engine.GateNear {
			return true
		}
	}
	return false
}

// ledgerOnTrophyOffer reports whether the ledger's cursor is on a
// trophy for sale.
func ledgerOnTrophyOffer(m *Model) bool {
	return m.screen == screenLedger && m.ledgerSelected().kind == ledgerTrophyOffer
}

// trophyCols are the TROPHIES table's columns.
var trophyCols = []col{{"name", kText, 0}, {"cost", kMoney, 0}, {"status", kText, 0}}

// trophyTable is the TROPHIES block's rows: the trophies owned, then
// the offers, each with its status.
func (m *Model) trophyTable(owned []game.Trophy, offers []game.TrophyOffer, long bool) [][]any {
	w := m.w
	var out [][]any
	for _, t := range owned {
		out = append(out, []any{t.Name, t.Cost, styled{theme.Good, fmt.Sprintf("yours since d%d", t.Bought)}})
	}
	for _, o := range offers {
		var status any
		switch {
		case o.Locked(w):
			toGo := cash(o.UnlockCash-w.Stats.PeakClean) + " clean to go"
			if long {
				toGo = "locked · " + toGo
			}
			status = styled{theme.Subtle, toGo}
		case o.Cost > w.Player.CleanCash:
			status = styled{theme.Bad, "short " + money(o.Cost-w.Player.CleanCash) + " clean"}
		default:
			status = "open to you · t to buy"
		}
		out = append(out, []any{o.Name, o.Cost, status})
	}
	return out
}

// trophyNote is the TROPHIES heading's note.
func (m *Model) trophyNote() string {
	w := m.w
	note := fmt.Sprintf(" · %s owned", plural(len(w.Trophies), "trophy"))
	if len(w.TrophiesLost) > 0 {
		note += fmt.Sprintf(" · %d taken", len(w.TrophiesLost))
	}
	return note
}

// trophyTalk is what a trophy adds to the street's opinion a day, in
// words: `+0.25 respect, +0.15 notoriety a day`.
func (m *Model) trophyTalk(id string) string {
	tc := m.cfg.Trophies.Trophy(id)
	if tc == nil {
		return ""
	}
	var parts []string
	for _, a := range []struct {
		v    float64
		name string
	}{{tc.Fear, "fear"}, {tc.Respect, "respect"}, {tc.Notoriety, "notoriety"}} {
		if a.v > 0 {
			parts = append(parts, fmt.Sprintf("+%.2g %s", a.v, a.name))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ") + " a day"
}

// trophySection is a trophy in the pane, owned or on offer.
func (m *Model) trophySection(id, name string, cost int, owned *game.Trophy, offer *game.TrophyOffer) section {
	w := m.w
	var lines []string
	if tc := m.cfg.Trophies.Trophy(id); tc != nil {
		lines = append(lines, m.wrapped(theme.Subtle, tc.Blurb)...)
	}
	lines = append(lines, row("cost", money(cost)+" clean"))
	if talk := m.trophyTalk(id); talk != "" {
		lines = append(lines, row("talk", talk))
	}
	switch {
	case owned != nil:
		lines = append(lines, row("bought", fmt.Sprintf("day %d", owned.Bought)), theme.Subtle.Render("The task force takes the costliest thing you own."))
	case offer.Locked(w):
		lines = append(lines, theme.Subtle.Render("locked until peak clean cash "+cash(offer.UnlockCash)), theme.Subtle.Render(cash(offer.UnlockCash-w.Stats.PeakClean)+" to go"))
	case offer.Cost > w.Player.CleanCash:
		lines = append(lines, theme.Bad.Render("short "+money(offer.Cost-w.Player.CleanCash)+" clean"))
	default:
		lines = append(lines, keyRow("t", "buy it"))
	}
	for _, l := range w.TrophiesLost {
		if l.ID == id {
			lines = append(lines, m.wrapped(theme.Warning, fmt.Sprintf("The feds took it on day %d.", l.Lost))...)
		}
	}
	return section{strings.ToUpper(name), lines}
}

// ledgerTrophyOfferSelected is the trophy offer under the cursor, or nil.
func (m *Model) ledgerTrophyOfferSelected() *game.TrophyOffer {
	sel := m.ledgerSelected()
	rows := m.trophyRows()
	if sel.kind != ledgerTrophyOffer || sel.i >= len(rows) {
		return nil
	}
	return &rows[sel.i]
}

// askTrophy asks before the offer under the cursor is bought.
func (m *Model) askTrophy() {
	o := m.ledgerTrophyOfferSelected()
	if o == nil || m.w.Over != nil {
		return
	}
	if o.Locked(m.w) {
		m.refuse(fmt.Sprintf("Can't buy %s: it wants %s peak clean cash, %s to go.", o.Name, cash(o.UnlockCash), cash(o.UnlockCash-m.w.Stats.PeakClean)))
		return
	}
	m.ask("buy", (*Model).trophyConfirm, (*Model).confirmTrophy)
}

func (m *Model) trophyConfirm() string {
	o := m.ledgerTrophyOfferSelected()
	if o == nil {
		return m.modal("BUY?", []string{"Nothing to buy."}, m.modalFooter())
	}
	body := []string{fmt.Sprintf("Buy %s for %s clean?", o.Name, money(o.Cost))}
	if talk := m.trophyTalk(o.ID); talk != "" {
		body = append(body, m.subtle("The street talks: "+talk+" while it is yours.")...)
	}
	body = append(body, m.subtle("It counts in net worth at cost. The task force takes the costliest thing you own.")...)
	return m.modal("BUY "+strings.ToUpper(o.Name)+"?", body, m.modalFooter())
}

func (m *Model) confirmTrophy() {
	o := m.ledgerTrophyOfferSelected()
	m.mode = modePlay
	if o == nil {
		return
	}
	t, err := m.sess.BuyTrophy(o.ID)
	if err != nil {
		m.refuse("Can't buy: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s is yours for %s clean. The paper will have it by morning.", t.Name, money(t.Cost)))
}

// pileLine is the ledger's word on a dirty pile big enough to be a
// storage problem (#392): its weight in hundreds, and the rot a night
// once it is over the line. Empty under $10M.
func (m *Model) pileLine() string {
	w := m.w
	if w.Player.DirtyCash < 10_000_000 {
		return ""
	}
	line := fmt.Sprintf("the pile weighs %s in hundreds", format.CashWeight(w.Player.DirtyCash))
	if rot := m.rules.Laundering.Rot(w.Player.DirtyCash); rot > 0 {
		line += fmt.Sprintf(" · rats and damp take ~%s a night over %s", money(rot), cash(m.rules.Laundering.RotLine()))
	}
	return line
}
