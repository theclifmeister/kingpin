package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// forces are the strike picker's rows, in dial order.
var forces = []events.Force{events.ForceWarn, events.ForcePush, events.ForceHit}

// rivalName is how the faction is referred to everywhere.
func (m *Model) rivalName() string {
	if m.w.Rival.Leader == "" {
		return "the rival"
	}
	return m.w.Rival.Leader + "'s crew"
}

// personalityWord is what the player knows about the rival's temperament.
func (m *Model) personalityWord() string {
	if !m.w.Rival.Observed {
		return "unknown"
	}
	return m.w.Rival.Personality
}

// eyeingWord is the tell (#69) for a panel line, `eyeing Riverside` in
// the rival's colour, or "" while no corner is spoken for.
func (m *Model) eyeingWord() string {
	c := m.set.Rivals.Eyeing(m.w)
	if c == nil || c.Owner != game.OwnerNone {
		return ""
	}
	return theme.RivalText.Render("eyeing " + c.Name)
}

// strikeRows are the picker's choices: the three forces for the corner,
// the three for its till (a boost, #70), plus calling off a strike or a
// boost already queued.
func (m *Model) strikeRows() []string {
	rows := []string{"warn", "push", "hit", "boost warn", "boost push", "boost hit"}
	if m.w.Today.Strike != nil {
		rows = append(rows, "stop")
	}
	return rows
}

// askStrike opens the force picker for the selected corner.
func (m *Model) askStrike() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Owner != game.OwnerRival {
		m.refuse("Can't send them there: enforcers go against a corner the rival holds.")
		return
	}
	if m.w.Crew.Role("enforcer") == 0 {
		m.refuse("Nothing to send: no enforcers. Hire one " + screenPointer(screenCrew) + ".")
		return
	}
	m.strikeCursor = 1
	m.mode = modeStrike
}

func (m *Model) confirmStrike() {
	c := m.mapSelected()
	rows := m.strikeRows()
	m.mode = modePlay
	if c == nil {
		return
	}
	i := max(0, min(m.strikeCursor, len(rows)-1))
	if i >= 2*len(forces) {
		m.w.CallOff()
		m.say("Called off. The enforcers stay home tonight.")
		return
	}
	if i >= len(forces) {
		// A boost confirms first (#70): the picker's cursor carries the
		// force into the confirmation.
		m.mode = modeConfirmBoost
		return
	}
	if err := m.w.SendEnforcers(c.ID, forces[i]); err != nil {
		m.refuse("Can't send them: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Enforcers go to %s tonight: %s. Odds ~%.0f%%, heat +%.0f.", c.Name, forces[i], m.set.Rivals.Odds(m.w, forces[i])*100, m.set.Rivals.StrikeHeat(c, forces[i])))
}

func (m *Model) viewStrike() string {
	c := m.mapSelected()
	if c == nil {
		return m.modal("SEND ENFORCERS", []string{"Nowhere to send them."}, m.modalFooter())
	}
	rows := m.strikeRows()
	m.strikeCursor = max(0, min(m.strikeCursor, len(rows)-1))
	body := []string{theme.Subtle.Render(fmt.Sprintf("%s vs %s on %s, muscle ~%.1f", plural(m.w.Crew.Role("enforcer"), "enforcer"), m.rivalName(), c.Name, m.set.Rivals.Defence(m.w))), ""}
	var cells [][]any
	b := m.set.Rivals.BoostTuning()
	for i, r := range rows {
		switch {
		case i < len(forces):
			f := forces[i]
			cells = append(cells, []any{r, approx{m.set.Rivals.Odds(m.w, f) * 100}, signed{m.set.Rivals.StrikeHeat(c, f)}, signed{m.cfg.Rivals.ForceFor(f).War}, "the corner"})
		case i < 2*len(forces):
			// A boost (#70): the same odds at the force, for the till.
			f := forces[i-len(forces)]
			cells = append(cells, []any{r, approx{m.set.Rivals.Odds(m.w, f) * 100}, signed{m.set.Rivals.BoostHeat(c)}, signed{b.War}, "~" + cash(m.set.Rivals.BoostTake(m.w, *c))})
		default:
			cells = append(cells, []any{r, nil, nil, nil, nil})
		}
	}
	m.modalFollow(len(body) + 1 + m.strikeCursor) // under the header
	body = append(body, table([]col{{"force", kText, 0}, {"lands", kPct, 0}, {"heat", kInt, 0}, {"war", kInt, 0}, {"for", kText, 0}}, cells, m.strikeCursor, m.modalInner())...)
	body = append(body, "")
	for _, l := range []string{
		"Harder flips faster, draws more heat on you, adds to the war",
		"and costs the enforcers' nerve. A loud enough war brings a",
		"crackdown on both sides. A boost takes the till, not the corner.",
	} {
		body = append(body, theme.Subtle.Render(l))
	}
	return m.modal("SEND ENFORCERS", body, m.modalFooter())
}

// dials are the undercut picker's rows, in dial order (#68).
var dials = []events.Dial{events.DialQuiet, events.DialNormal, events.DialAggressive}

// undercutRows are the picker's choices: the three dials, plus calling
// off an undercut already queued on the corner.
func (m *Model) undercutRows() []string {
	rows := []string{"quiet", "normal", "aggressive"}
	if c := m.mapSelected(); c != nil {
		if _, ok := m.w.Undercutting(c.ID); ok {
			rows = append(rows, "stop")
		}
	}
	return rows
}

// askUndercut opens the dial picker for a price war on the selected
// corner, or says why there is none to fight there.
func (m *Model) askUndercut() {
	c := m.mapSelected()
	if c == nil {
		return
	}
	if c.Owner != game.OwnerRival {
		m.refuse("Can't undercut there: a price war is fought on a corner the rival holds.")
		return
	}
	if err := m.w.CanUndercut(c.ID); err != nil {
		m.refuse("Can't undercut: " + err.Error())
		return
	}
	m.undercutCursor = 1
	if d, ok := m.w.Undercutting(c.ID); ok {
		m.undercutCursor = int(d)
	}
	m.mode = modeUndercut
}

func (m *Model) confirmUndercut() {
	c := m.mapSelected()
	rows := m.undercutRows()
	m.mode = modePlay
	if c == nil {
		return
	}
	i := max(0, min(m.undercutCursor, len(rows)-1))
	if i >= len(dials) {
		m.w.CancelUndercut(c.ID)
		m.say("Called off. " + c.Name + " sells at their price tonight.")
		return
	}
	d := dials[i]
	if err := m.w.Undercut(c.ID, d); err != nil {
		m.refuse("Can't undercut: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Undercutting %s tonight at %s: ~%.0f units a day at %.0f%% off, %s loses ~%s a day.",
		c.Name, d, m.set.Market.UndercutUnits(m.w, *c, d), m.set.Market.PriceCut()*100, m.w.Rival.Leader, cash(m.undercutLoss(*c, d))))
}

// undercutLoss is what a price war on a corner at the dial costs the
// rival a day: the share taken of what the corner earns it.
func (m *Model) undercutLoss(c game.Corner, d events.Dial) int {
	return int(math.Round(m.set.Market.Steal(m.w, c, d) * float64(m.set.Rivals.CornerIncome(m.w, c))))
}

// undercutPriceCut is what a unit moved off the rival's corner at the
// dial makes against the street price, as a signed share: the dial's
// price less price_cut.
func (m *Model) undercutPriceCut(d events.Dial) float64 {
	return (m.set.Market.Dial(d).Price*(1-m.set.Market.PriceCut()) - 1) * 100
}

func (m *Model) viewUndercut() string {
	c := m.mapSelected()
	if c == nil {
		return m.modal("UNDERCUT", []string{"Nothing to undercut."}, m.modalFooter())
	}
	rows := m.undercutRows()
	m.undercutCursor = max(0, min(m.undercutCursor, len(rows)-1))
	body := []string{
		theme.Subtle.Render(fmt.Sprintf("%s on %s: worth ~%s a day to them", m.rivalName(), c.Name, cash(m.set.Rivals.CornerIncome(m.w, *c)))),
		theme.Subtle.Render(fmt.Sprintf("your corners next door ×%.1f: the share taken scales with them", m.w.NextDoor(*c))),
		"",
	}
	var cells [][]any
	for i, r := range rows {
		if i < len(dials) {
			d := dials[i]
			cells = append(cells, []any{r, approx{m.set.Market.Steal(m.w, *c, d) * 100}, approx{m.set.Market.UndercutUnits(m.w, *c, d)}, signed{m.undercutPriceCut(d)}, m.undercutLoss(*c, d)})
		} else {
			cells = append(cells, []any{r, nil, nil, nil, nil})
		}
	}
	m.modalFollow(len(body) + 1 + m.undercutCursor) // under the header
	body = append(body, table([]col{{"dial", kText, 0}, {"takes", kPct, 0}, {"units/day", kInt, 0}, {"price", kPct, 0}, {"they lose", kCash, 0}}, cells, m.undercutCursor, m.modalInner())...)
	body = append(body, "")
	for _, l := range []string{
		"Tonight's orders here serve their customers too, cheap, on top",
		"of your own corners; the extra volume gluts the product. No",
		"heat, a little war and a grudge; starve a corner long enough",
		"and they push back or give it up, by temper.",
	} {
		body = append(body, theme.Subtle.Render(l))
	}
	return m.modal("UNDERCUT", body, m.modalFooter())
}

// dealRules are the three lines on what a deal does and what breaks it,
// the rivals pane's RULES section.
var dealRules = []string{
	"Truce or tribute: they stay off your corners. Split: off your side of the line.",
	"A push or a hit under a deal breaks it: trust hits the floor and they make a call.",
	"So does a missed tribute, or walking off a split corner. A warning does not.",
}

// dealDoes is what a deal of the kind does while it holds.
func dealDoes(kind string) string {
	switch kind {
	case game.DealTruce:
		return "Neither side pushes; no undercutting, no tips, for the term."
	case game.DealTribute:
		return "You pay the cut each night; they leave every corner of yours alone until you stop."
	case game.DealSplit:
		return "They neither claim nor push on your side of the line, and you post nobody past it."
	}
	return "Half the cost of a run, half the loss."
}

// dealBreaks is what breaks a deal of the kind and what that costs.
func dealBreaks(kind string) string {
	how := "A push or a hit under it"
	switch kind {
	case game.DealTribute:
		how = "A missed night, a push or a hit"
	case game.DealSplit:
		how = "Walking off a split corner, a push or a hit"
	}
	return how + " breaks it: trust hits the floor and they call the police."
}

// moodLine is where you stand with the rival: what trust buys you, or,
// after a betrayal, how long they are not taking your calls (bad).
func (m *Model) moodLine() (line string, bad bool) {
	w := m.w
	r := w.Rival
	switch {
	case m.set.Rivals.Distrusted(w, w.Day+1):
		return fmt.Sprintf("You broke a deal. They take nothing for %s.", plural(r.Betrayed+m.set.Rivals.Diplomacy().DistrustDays-w.Day-1, "more day")), true
	case r.Trust >= 60:
		return "They take you at your word. A deal is cheap to strike.", false
	case r.Trust < 20:
		return "They do not trust you. Keep a deal a while and that changes.", false
	default:
		return "Trust grows a little every day a deal holds and falls with every strike.", false
	}
}

// rivalsDetails is the rivals screen's pane: the offer under the cursor
// or, with none, the deal that holds or where you stand with the rival;
// then RULES, LIFETIME and the keys.
func (m *Model) rivalsDetails() []section {
	w := m.w
	r := w.Rival
	if r.Arrived == 0 {
		return []section{{"NO RIVAL", wrapped(theme.Subtle, "Nobody is contesting the city yet. When somebody does, this is where you talk to them.")}}
	}
	mood, bad := m.moodLine()
	moodStyle := theme.Subtle
	if bad {
		moodStyle = theme.Bad
	}
	var sel section
	switch {
	case len(w.Offers) > 0:
		o := w.Offers[max(0, min(m.dealCursor, len(w.Offers)-1))]
		lines := []string{theme.Subtle.Render(fmt.Sprintf("theirs · %s to answer", plural(o.Expires-w.Day+1, "day")))}
		if o.Deal.Kind == game.DealTribute {
			lines = append(lines, m.tributeRows(o.Deal)...)
		}
		lines = append(lines, wrapped(theme.Body, dealDoes(o.Deal.Kind))...)
		lines = append(lines, wrapped(theme.Subtle, dealBreaks(o.Deal.Kind))...)
		lines = append(lines, keyRow("y", "accept it"), keyRow("x", "turn it down"))
		sel = section{m.dealTitle(o.Deal), lines}
	case len(r.Deals) > 0:
		d := r.Deals[0]
		who, term := "yours", "until broken"
		if d.Offered {
			who = "theirs"
		}
		if d.Until > 0 {
			term = plural(d.Left(w.Day), "day") + " left"
		}
		lines := []string{row("who", fmt.Sprintf("%s, since day %d", who, d.Since)), row("holds", term)}
		if d.Kind == game.DealTribute {
			lines = append(lines, m.tributeRows(d)...)
		}
		lines = append(lines, wrapped(theme.Body, dealDoes(d.Kind))...)
		lines = append(lines, wrapped(theme.Subtle, dealBreaks(d.Kind))...)
		sel = section{m.dealTitle(d), lines}
	default:
		sel = section{strings.ToUpper(m.rivalName()), wrapped(moodStyle, mood)}
	}
	// A betrayal's clock is worth a line whatever is selected.
	if bad && len(w.Offers)+len(r.Deals) > 0 {
		sel.lines = append(sel.lines, wrapped(moodStyle, mood)...)
	}
	var rules []string
	for _, l := range dealRules {
		rules = append(rules, wrapped(theme.Subtle, l)...)
	}
	s := w.Stats
	life := []string{
		row("struck", fmt.Sprintf("%d · refused %d", s.Deals, s.DealsRefused)),
		row("broken", fmt.Sprintf("by you %d, by them %d", s.Betrayals, s.BetrayedBy)),
		row("tribute", cash(s.Tribute)+" paid"),
	}
	return []section{sel, {"RULES", rules}, m.booksSection(), {"LIFETIME", life}}
}
