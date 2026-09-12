package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The books (#70): the rival's machine made legible, for a price, and
// the three moves against it that are not a strike. The rivals screen
// carries the BOOKS block (what a scout last read, how old it is, and
// the police's attention on them) and takes i (scout) and $ (buy off);
// the map's strike picker gains a boost row per force and t tips the
// police on the corner under the cursor. Every move confirms first, with
// the numbers the dice use.

// booksCols are the BOOKS table's columns: what was read, the figure,
// and how many days ago.
var booksCols = []col{{"books", kText, 0}, {"as read", kText, 0}, {"age", kDays, 0}}

// booksAge is how old the snapshot is in words: `read 3 days ago`,
// `read today`, with `stale` once it is.
func (m *Model) booksAge() string {
	k := m.w.Rival.Known
	if !k.Read() {
		return "never read"
	}
	age := k.Age(m.w.Day)
	s := "read today"
	if age > 0 {
		s = "read " + plural(age, "day") + " ago"
	}
	if m.set.Rivals.Stale(m.w, m.w.Day) {
		s += " · stale"
	}
	return s
}

// booksRows are the BOOKS table's rows: the chest, the take, the
// muscle and the wage bill as last read, `?` while never read.
func (m *Model) booksRows() [][]any {
	k := m.w.Rival.Known
	if !k.Read() {
		return [][]any{{"cash", "?", nil}, {"income", "?", nil}, {"muscle", "?", nil}, {"wages", "?", nil}}
	}
	age := k.Age(m.w.Day)
	return [][]any{
		{"cash", cash(k.Cash), age},
		{"income", cash(k.Income) + "/day", age},
		{"muscle", plural(k.Muscle, "head"), age},
		{"wages", cash(k.Wages) + "/day", age},
	}
}

// policeBar is the police's attention on the rival against the line
// they act at: `police ████░░░░ 45/60`, `raided 3d ago` while they will
// not be back, or `none` at zero.
func (m *Model) policeBar(width int) string {
	r := m.w.Rival
	tp := m.set.Rivals.TipTuning()
	if r.Heat <= 0 && r.LastRaid == 0 {
		return theme.Subtle.Render("none")
	}
	style := theme.Warning
	if r.Heat >= tp.PoliceNotice {
		style = theme.Bad
	}
	s := style.Render(sparkline.Bar(r.Heat/tp.PoliceNotice, width, nil)) + style.Render(fmt.Sprintf(" %.0f/%.0f", r.Heat, tp.PoliceNotice))
	if !m.set.Rivals.RaidReady(m.w, m.w.Day+1) {
		s += theme.Subtle.Render(fmt.Sprintf(" · raided %s ago", plural(m.w.Day-r.LastRaid, "day")))
	}
	return s
}

// booksLines is the BOOKS block of the rivals screen's MAIN: the title
// with the snapshot's age, the table, and the police's attention.
func (m *Model) booksLines(width int) []string {
	ls := []string{sectionTitle("BOOKS · "+m.booksAge(), theme.Rivals)}
	ls = append(ls, table(booksCols, m.booksRows(), -1, width)...)
	barW := max(6, min(12, width/6))
	ls = append(ls, truncate(theme.Subtle.Render("police ")+m.policeBar(barW), width))
	return ls
}

// booksSection is the rivals pane's BOOKS section: what the two moves
// here cost, with the numbers the dice use. The snapshot's age is MAIN's
// title; the pane at 120x40 has no room for prose beside the rules.
func (m *Model) booksSection() section {
	w := m.w
	return section{"BOOKS", []string{
		keyRow("i", fmt.Sprintf("scout for %s, ~%.0f%%", money(m.set.Rivals.ScoutCost()), m.set.Rivals.ScoutOdds(w)*100)),
		keyRow("$", fmt.Sprintf("buy off a head, %s", cash(m.set.Rivals.MusclePrice(w)))),
	}}
}

// askScout opens the scout confirmation, or says why there is nothing to
// read.
func (m *Model) askScout() {
	if m.w.Rival.Arrived == 0 {
		m.refuse("Nothing to scout: nobody is contesting the city yet.")
		return
	}
	if m.w.Today.Scouting != nil {
		m.refuse("Can't scout twice: somebody is already reading their books tonight.")
		return
	}
	m.mode = modeConfirmScout
}

func (m *Model) confirmScout() {
	m.mode = modePlay
	if err := m.w.Scout(m.set.Rivals.ScoutCost()); err != nil {
		m.refuse("Can't scout: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Somebody reads %s's books tonight. Odds ~%.0f%%.", m.rivalName(), m.set.Rivals.ScoutOdds(m.w)*100))
}

// scoutConfirm is the confirmation's body: the cost, who does the
// reading, the odds and what a read gives.
func (m *Model) scoutConfirm() string {
	w := m.w
	odds := m.set.Rivals.ScoutOdds(w)
	best := 0
	for _, c := range w.Crew.Members {
		if c.Role == "enforcer" && c.Skill > best {
			best = c.Skill
		}
	}
	who := "With no enforcer on the payroll you are asking around yourself."
	if best > 0 {
		who = fmt.Sprintf("Your best enforcer (skill %d) does the asking.", best)
	}
	body := m.wrapLines(fmt.Sprintf("Somebody goes through %s's books tonight for %s.", m.rivalName(), money(m.set.Rivals.ScoutCost())))
	body = append(body, who, fmt.Sprintf("~%.0f%% it reads them: the chest, the take, the muscle, the wages.", odds*100))
	if w.Rival.Scouted > 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Every empty night so far (%d) makes the next likelier.", w.Rival.Scouted)))
	}
	body = append(body, theme.Subtle.Render(fmt.Sprintf("A snapshot: it goes stale after %s. Failing, the money is gone.", plural(m.set.Rivals.Books().StaleDays, "day"))))
	return m.modal("SCOUT THEIR BOOKS?", body, m.modalFooter())
}

// boostForce is the force the boost row picked carries into the
// confirmation.
func (m *Model) boostForce() (events.Force, bool) {
	i := m.strikeCursor - len(forces)
	if i < 0 || i >= len(forces) {
		return 0, false
	}
	return forces[i], true
}

// confirmBoost is y on the boost confirmation: the enforcers go for the
// till tonight.
func (m *Model) confirmBoost() {
	c := m.mapSelected()
	f, ok := m.boostForce()
	m.mode = modePlay
	if c == nil || !ok {
		return
	}
	if err := m.w.Boost(c.ID, f); err != nil {
		m.refuse("Can't send them: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Enforcers go for the till on %s tonight at %s: ~%s, odds ~%.0f%%, heat +%.0f.", c.Name, f, cash(m.set.Rivals.BoostTake(m.w, *c)), m.set.Rivals.Odds(m.w, f)*100, m.set.Rivals.BoostHeat(c)))
}

// boostConfirm is the confirmation's body: the till, the odds, the heat
// and the war, the toll, and the table.
func (m *Model) boostConfirm() string {
	c := m.mapSelected()
	f, ok := m.boostForce()
	if c == nil || !ok {
		return m.modal("BOOST?", []string{"Nowhere to send them."}, m.modalFooter())
	}
	b := m.set.Rivals.BoostTuning()
	body := m.wrapLines(fmt.Sprintf("The enforcers go in at %s for the till on %s, not the corner.", f, c.Name))
	body = append(body, m.wrapLines(fmt.Sprintf("~%.0f%% they come back with ~%s of %s's takings.", m.set.Rivals.Odds(m.w, f)*100, cash(m.set.Rivals.BoostTake(m.w, *c)), m.w.Rival.Leader))...)
	body = append(body, fmt.Sprintf("Heat +%.0f and war +%.0f either way; trust -%.0f.", m.set.Rivals.BoostHeat(c), b.War, m.cfg.Rivals.ForceFor(f).Trust))
	fail := fmt.Sprintf("Failing, the enforcers lose %.0f nerve-weighted loyalty", b.FailLoss)
	if m.w.Rival.Muscle > b.FailMuscle {
		fail += " and one gets hurt"
	}
	body = append(body, theme.Warning.Render(fail+"."))
	if f != events.ForceWarn && (m.w.AtPeace() || m.w.Deal(game.DealSplit) != nil) {
		body = append(body, theme.Bad.Render(fmt.Sprintf("A boost at %s under a deal breaks it: trust hits the floor.", f)))
	}
	return m.modal("BOOST?", body, m.modalFooter())
}

// askTip opens the tip confirmation for the corner under the cursor, or
// says why there is nobody to tip on.
func (m *Model) askTip() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Owner != game.OwnerRival {
		m.refuse("Can't tip the police there: a tip is on a corner the rival holds.")
		return
	}
	if m.w.Today.Tipoff != nil {
		m.refuse("Can't tip twice: you have already tipped the police tonight.")
		return
	}
	m.mode = modeConfirmTip
}

func (m *Model) confirmTip() {
	c := m.mapSelected()
	m.mode = modePlay
	if c == nil {
		return
	}
	if err := m.w.Tip(c.ID); err != nil {
		m.refuse("Can't tip: " + err.Error())
		return
	}
	tp := m.set.Rivals.TipTuning()
	m.say(fmt.Sprintf("The police hear about %s tonight. Their attention on %s: %.0f → %.0f of %.0f.", c.Name, m.w.Rival.Leader, m.w.Rival.Heat, min(100, m.w.Rival.Heat+tp.Heat), tp.PoliceNotice))
}

// tipConfirm is the confirmation's body: where the police's attention
// stands, what the tip does to it and when they act, the trust, the
// page and the peace.
func (m *Model) tipConfirm() string {
	c := m.mapSelected()
	if c == nil {
		return m.modal("TIP THE POLICE?", []string{"Nobody to tip on."}, m.modalFooter())
	}
	w := m.w
	tp := m.set.Rivals.TipTuning()
	after := min(100, w.Rival.Heat+tp.Heat)
	body := []string{fmt.Sprintf("A word to the police about %s's people on %s. Free.", w.Rival.Leader, c.Name)}
	body = append(body, m.wrapLines(fmt.Sprintf("Their attention on %s goes %.0f → %.0f; at %.0f they raid the corner.", m.rivalName(), w.Rival.Heat, after, tp.PoliceNotice))...)
	switch {
	case !m.set.Rivals.RaidReady(w, w.Day+1):
		for _, l := range m.wrapLines(fmt.Sprintf("They raided %s ago and will not be back for %s; the attention builds meanwhile.", plural(w.Day-w.Rival.LastRaid, "day"), plural(w.Rival.LastRaid+tp.RaidDays-w.Day-1, "day"))) {
			body = append(body, theme.Subtle.Render(l))
		}
	case after >= tp.PoliceNotice:
		for _, l := range m.wrapLines(fmt.Sprintf("That is the line: a raid tonight takes the corner and %.0f%% of their muscle.", tp.RaidMuscle*100)) {
			body = append(body, theme.Good.Render(l))
		}
	}
	body = append(body, theme.Warning.Render(fmt.Sprintf("Trust -%.0f, and ~%.0f%% the DA's file on you gains a page.", tp.Trust, m.cfg.Heat.Heat.TipEvidence*100)))
	if w.AtPeace() {
		for _, l := range m.wrapLines("Under a truce or a tribute a tip breaks the peace: trust hits the floor.") {
			body = append(body, theme.Bad.Render(l))
		}
	}
	return m.modal("TIP THE POLICE?", body, m.modalFooter())
}

// buyOffDialog is the state of the buy-off confirmation: the heads and
// the error under them.
type buyOffDialog struct {
	units numberField
	err   string
}

// buyOffMax is the most heads the dialog offers: the muscle as last
// read, or one while the books are unread (you do not know how many
// there are).
func (m *Model) buyOffMax() int {
	if k := m.w.Rival.Known; k.Read() {
		return k.Muscle
	}
	return 1
}

// askBuyOff opens the buy-off confirmation on one head.
func (m *Model) askBuyOff() {
	if m.w.Rival.Arrived == 0 {
		m.refuse("Nobody to buy off: nobody is contesting the city yet.")
		return
	}
	if m.w.Today.Poach != nil {
		m.refuse("Can't buy off twice: you are already paying their people tonight.")
		return
	}
	m.bo = buyOffDialog{units: newNumberField("blank = 1")}
	m.bo.units.max = m.buyOffMax()
	m.bo.units.Focus()
	m.mode = modeConfirmBuyOff
}

// buyOffUnits is the heads the field reads: one for a blank, an error
// for a number that does not read.
func (m *Model) buyOffUnits() (int, error) {
	s := strings.TrimSpace(m.bo.units.Value())
	if s == "" {
		return 1, nil
	}
	n, ok := m.bo.units.Number()
	if !ok || n <= 0 {
		return 0, fmt.Errorf("enter a whole number above zero")
	}
	return n, nil
}

// keyBuyOff is the confirmation: y or enter pays, esc closes, and the
// rest goes to the number field.
func (m *Model) keyBuyOff(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	m.bo.err = ""
	switch key {
	case "esc", "q":
		m.mode = modePlay
		return m, nil
	case "y", "Y", "enter":
		m.confirmBuyOff()
		return m, nil
	}
	m.bo.units.max = m.buyOffMax()
	return m, m.bo.units.Update(k)
}

// confirmBuyOff pays for the heads the field reads, or shows why it
// cannot.
func (m *Model) confirmBuyOff() {
	n, err := m.buyOffUnits()
	if err != nil {
		m.bo.err = dialogError(err)
		return
	}
	price := m.set.Rivals.MusclePrice(m.w)
	if err := m.w.BuyOff(n, n*price); err != nil {
		m.bo.err = dialogError(err)
		return
	}
	m.mode = modePlay
	m.say(fmt.Sprintf("%s of %s's muscle paid to go home tonight: %s. Odds ~%.0f%%.", plural(n, "head"), m.w.Rival.Leader, money(n*price), m.set.Rivals.PoachTuning().Odds*100))
}

// viewBuyOff is the confirmation: the price a head, the field, the odds
// and the price of failing.
func (m *Model) viewBuyOff() string {
	w := m.w
	p := m.set.Rivals.PoachTuning()
	price := m.set.Rivals.MusclePrice(w)
	n, err := m.buyOffUnits()
	if err != nil {
		n = 1
	}
	units := m.bo.units
	units.max = m.buyOffMax()
	known := "You have not read their books: buy blind, a head at a time."
	if k := w.Rival.Known; k.Read() {
		known = fmt.Sprintf("As last read (%s): %s.", strings.TrimPrefix(m.booksAge(), "read "), plural(k.Muscle, "head"))
	}
	body := m.wrapLines(fmt.Sprintf("Pay %s's people to go home: %s a head, %s for %s.", w.Rival.Leader, money(price), money(n*price), plural(n, "head")))
	body = append(body, theme.Subtle.Render(known), "", "Heads     "+units.View(), "")
	body = append(body, m.wrapLines(fmt.Sprintf("~%.0f%% it lands: they leave %s and never join you; what you paid for heads that were not there comes back. A well-paid crew costs more; respect cuts it.", p.Odds*100, m.rivalName()))...)
	body = append(body, theme.Warning.Render("Failing, the money is gone and they know you tried."))
	if m.bo.err != "" {
		body = append(body, "", theme.Bad.Render(m.bo.err))
	}
	return m.modal("BUY OFF THEIR MUSCLE?", body, m.modalFooter())
}
