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
		m.status = "No enforcers to send. Hire one on the crew screen (4)."
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
		return m.modal("SEND ENFORCERS", "Nowhere to send them.")
	}
	rows := m.strikeRows()
	m.strikeCursor = max(0, min(m.strikeCursor, len(rows)-1))
	width := max(30, m.width-10)
	var b strings.Builder
	b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("%d enforcer(s) vs %s on %s, muscle ~%.1f", m.w.Crew.Role("enforcer"), m.rivalName(), c.Name, m.set.Rivals.Defence(m.w))), width) + "\n\n")
	var cells [][]any
	for i, r := range rows {
		if i < len(forces) {
			f := forces[i]
			cells = append(cells, []any{r, approx{m.set.Rivals.Odds(m.w, f) * 100}, signed{m.set.Rivals.StrikeHeat(c, f)}, signed{m.cfg.Rivals.ForceFor(f).War}})
		} else {
			cells = append(cells, []any{r, nil, nil, nil})
		}
	}
	for _, l := range table([]col{{"force", kText, 0}, {"takes it", kPct, 0}, {"heat", kInt, 0}, {"war", kInt, 0}}, cells, m.strikeCursor, width) {
		b.WriteString(l + "\n")
	}
	for _, l := range []string{
		"Harder flips faster, draws more heat on you, adds to the war",
		"and costs the enforcers' nerve. A loud enough war brings a",
		"crackdown on both sides.",
	} {
		b.WriteString(truncate(theme.Subtle.Render(l), width) + "\n")
	}
	b.WriteString("\n" + theme.Key.Render("enter") + " send  " + theme.Key.Render("esc") + " back")
	return m.modal("SEND ENFORCERS", strings.TrimRight(b.String(), "\n"))
}

// rivalLines is the dashboard panel's content: who and how much they
// hold, then what they are like and how loud the war is.
func (m *Model) rivalLines() string {
	w := m.w
	r := w.Rival
	tun := m.set.Rivals.Tuning()
	if r.Arrived == 0 {
		return theme.Subtle.Render("Nobody is contesting the city. Yet.")
	}
	corners := fmt.Sprintf("%d corners", w.RivalHeld())
	switch w.RivalHeld() {
	case 0:
		corners = "run out of town"
	case 1:
		corners = "1 corner"
	}
	// The war meter against the line the police crack down at.
	war := fmt.Sprintf("war %.0f/%.0f", r.War, tun.CrackdownThreshold)
	switch {
	case r.War >= tun.WarThreshold:
		war = theme.Bad.Render(war + " loud")
	case r.War > 0:
		war = theme.Warning.Render(war)
	default:
		war = theme.Subtle.Render("no war")
	}
	// The table: trust, and whatever holds or waits.
	table := theme.Subtle.Render(fmt.Sprintf("trust %.0f", r.Trust))
	switch {
	case len(w.Offers) > 0:
		table += theme.Gold.Render(fmt.Sprintf(" · %d offer(s) (8)", len(w.Offers)))
	case len(r.Deals) > 0:
		var ds []string
		for _, d := range r.Deals {
			if d.Until > 0 {
				ds = append(ds, fmt.Sprintf("%s %dd", d.Kind, d.Left(w.Day)))
			} else {
				ds = append(ds, d.Kind)
			}
		}
		table += theme.Good.Render(" · " + strings.Join(ds, ", "))
	case w.Proposal != nil:
		table += theme.Gold.Render(" · proposal tonight")
	}
	return theme.Rival.Render(r.Leader) + theme.Subtle.Render(" · "+corners) + "\n" +
		theme.Subtle.Render(m.personalityWord()+" · ") + war + "\n" + table + "\n"
}
