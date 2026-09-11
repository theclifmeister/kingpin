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
		m.status = "Enforcers go against a corner the rival holds (purple)."
		return
	}
	if m.w.Crew.Role("enforcer") == 0 {
		m.status = "No enforcers to send. Hire one " + screenPointer(screenCrew) + "."
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
		m.status = "Called off. The enforcers stay home tonight."
		return
	}
	if err := m.w.SendEnforcers(c.ID, forces[i]); err != nil {
		m.status = "Can't send them: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Enforcers go to %s tonight: %s. Odds ~%.0f%%, heat +%.0f.", c.Name, forces[i], m.set.Rivals.Odds(m.w, forces[i])*100, m.set.Rivals.StrikeHeat(c, forces[i]))
}

func (m *Model) viewStrike() string {
	c := m.mapSelected()
	if c == nil {
		return m.modal("SEND ENFORCERS", []string{"Nowhere to send them."}, m.modalFooter())
	}
	rows := m.strikeRows()
	m.strikeCursor = max(0, min(m.strikeCursor, len(rows)-1))
	body := []string{theme.Subtle.Render(fmt.Sprintf("%d enforcer(s) vs %s on %s, muscle ~%.1f", m.w.Crew.Role("enforcer"), m.rivalName(), c.Name, m.set.Rivals.Defence(m.w))), ""}
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
// the rivals screen's RULES section.
var dealRules = []string{
	"Truce or tribute: they stay off your corners. Split: off your side of the line.",
	"A push or a hit under a deal breaks it: trust hits the floor and they make a call.",
	"So does a missed tribute, or walking off a split corner. A warning does not.",
}

// rivalsDetails is the rivals screen's pane: the offer under the cursor
// (or, with none on the table, the rival and where you stand with
// them), the rules of the table, and the keys.
func (m *Model) rivalsDetails() []section {
	w := m.w
	r := w.Rival
	if r.Arrived == 0 {
		return []section{{"NO RIVAL", wrapped(theme.Subtle, "Nobody is contesting the city. Yet. When somebody does, this is where you talk to them.")}}
	}
	var secs []section
	if n := len(w.Offers); n > 0 {
		o := w.Offers[max(0, min(m.dealCursor, n-1))]
		lines := []string{
			row("terms", w.Describe(o.Deal)),
			row("to answer", fmt.Sprintf("%d days", o.Expires-w.Day+1)),
			row("offered by", m.rivalName()),
			keyRow("y", "accept it"),
			keyRow("x", "turn it down"),
		}
		secs = append(secs, section{strings.ToUpper(o.Deal.Kind) + " · OFFERED", lines})
	}
	tun := m.set.Rivals.Tuning()
	corners := fmt.Sprintf("%d corners", w.RivalHeld())
	if w.RivalHeld() == 0 {
		corners = "run out of town"
	}
	war := theme.Subtle.Render("none")
	switch {
	case r.War >= tun.WarThreshold:
		war = theme.Bad.Render(fmt.Sprintf("%.0f/%.0f loud", r.War, tun.CrackdownThreshold))
	case r.War > 0:
		war = theme.Warning.Render(fmt.Sprintf("%.0f/%.0f", r.War, tun.CrackdownThreshold))
	}
	lines := []string{
		row("temper", m.personalityWord()),
		row("holds", corners),
		row("muscle", fmt.Sprintf("%d", r.Muscle)),
		row("trust", fmt.Sprintf("%.0f", r.Trust)),
		row("war", war),
	}
	for _, d := range r.Deals {
		term := "until broken"
		if d.Until > 0 {
			term = fmt.Sprintf("%d days left", d.Left(w.Day))
		}
		lines = append(lines, row(d.Kind, term))
	}
	if p := w.Proposal; p != nil {
		lines = append(lines, row("tonight", theme.Gold.Render(fmt.Sprintf("you propose, ~%.0f%%", m.set.Rivals.Chance(w, *p)*100))))
	}
	if m.set.Rivals.Distrusted(w, w.Day+1) {
		lines = append(lines, wrapped(theme.Bad, fmt.Sprintf("You broke a deal. They take nothing for %d more days.", r.Betrayed+m.set.Rivals.Diplomacy().DistrustDays-w.Day-1))...)
	}
	secs = append(secs, section{strings.ToUpper(m.rivalName()), lines})
	var rules []string
	for _, l := range dealRules {
		rules = append(rules, wrapped(theme.Subtle, l)...)
	}
	return append(secs, section{"RULES", rules})
}
