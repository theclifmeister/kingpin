package ui

import (
	"fmt"
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

// strikeRows are the picker's choices: the three forces, plus calling off
// a strike already queued.
func (m *Model) strikeRows() []string {
	rows := []string{"warn", "push", "hit"}
	if m.w.Strike != nil {
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
	if i >= len(forces) {
		m.w.CallOff()
		m.say("Called off. The enforcers stay home tonight.")
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
	for i, r := range rows {
		if i < len(forces) {
			f := forces[i]
			cells = append(cells, []any{r, approx{m.set.Rivals.Odds(m.w, f) * 100}, signed{m.set.Rivals.StrikeHeat(c, f)}, signed{m.cfg.Rivals.ForceFor(f).War}})
		} else {
			cells = append(cells, []any{r, nil, nil, nil})
		}
	}
	m.modalFollow(len(body) + 1 + m.strikeCursor) // under the header
	body = append(body, table([]col{{"force", kText, 0}, {"takes it", kPct, 0}, {"heat", kInt, 0}, {"war", kInt, 0}}, cells, m.strikeCursor, m.modalInner())...)
	body = append(body, "")
	for _, l := range []string{
		"Harder flips faster, draws more heat on you, adds to the war",
		"and costs the enforcers' nerve. A loud enough war brings a",
		"crackdown on both sides.",
	} {
		body = append(body, theme.Subtle.Render(l))
	}
	return m.modal("SEND ENFORCERS", body, m.modalFooter())
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
	return []section{sel, {"RULES", rules}, {"LIFETIME", life}}
}
