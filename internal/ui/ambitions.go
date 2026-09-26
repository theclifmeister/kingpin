package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The ambitions (#347, docs/ambitions.md): the endings surfaced early
// as plans. a on the walk-away dialog (w on the dashboard) or on the
// stage modal opens the panel:
// every plan the file does not box, a bar each (full exactly when the
// ending's own condition holds), the next step, and the selected plan's
// steps under the table. enter pins the plan under the cursor (or
// unpins it): the pinned plan shows on the dashboard with its next
// step, its milestones are alerts, and the report carries a PLAN line.
// Nothing here writes more than the pin (World.Ambition), which no sim
// reads.

// ambitionsDialog is the panel's state: the row, and whether it was
// opened from the stage, so closing it goes on to the card and the
// report as closing the stage would.
type ambitionsDialog struct {
	cursor    int
	fromStage bool
}

// ambitions is every plan as the view words it.
func (m *Model) ambitions() []engine.AmbitionView {
	var out []engine.AmbitionView
	for _, a := range m.sess.Ambitions() {
		out = append(out, engine.AmbitionViewOf(m.cfg.Ambitions, a, m.w.Ambition == a.ID))
	}
	return out
}

// plan is the pinned plan as the view words it, false with none.
func (m *Model) plan() (engine.AmbitionView, bool) {
	a, ok := m.sess.Plan()
	if !ok {
		return engine.AmbitionView{}, false
	}
	return engine.AmbitionViewOf(m.cfg.Ambitions, a, true), true
}

// openAmbitions opens the panel on the pinned plan, else the first.
func (m *Model) openAmbitions(fromStage bool) {
	m.amb = ambitionsDialog{fromStage: fromStage}
	for i, a := range m.ambitions() {
		if a.Pinned {
			m.amb.cursor = i
		}
	}
	m.modalScroll = 0
	m.mode = modeAmbitions
}

// keyAmbitions is the panel's keys: the cursor, enter to pin or unpin
// the plan under it, a digit to move to a row (#500), esc to close.
func (m *Model) keyAmbitions(key string) {
	rows := m.ambitions()
	if closes(key) {
		m.closeAmbitions()
		return
	}
	switch key {
	case "up", "k":
		stepCursor(&m.amb.cursor, -1, len(rows))
	case "down", "j":
		stepCursor(&m.amb.cursor, 1, len(rows))
	case "enter":
		m.pinAmbition(rows)
	default:
		if i, ok := digit(key); ok && i < len(rows) {
			m.amb.cursor = i // a digit moves, enter pins, as in every picker (#500)
		}
	}
}

// closeAmbitions closes the panel: back to play, or on to the card and
// the report when the stage opened it.
func (m *Model) closeAmbitions() {
	if m.amb.fromStage {
		m.amb.fromStage = false
		m.showCard()
		return
	}
	m.mode = modePlay
}

// pinAmbition pins the plan under the cursor, or unpins it when it is
// the plan already, and says so on the status bar.
func (m *Model) pinAmbition(rows []engine.AmbitionView) {
	if len(rows) == 0 {
		return
	}
	a := rows[max(0, min(m.amb.cursor, len(rows)-1))]
	id, say := a.ID, "The plan: "+a.Name+"."
	if a.Pinned {
		id, say = "", "No plan pinned."
	}
	if err := m.sess.PinAmbition(id); err != nil {
		m.refuse("Can't: " + err.Error() + ".")
		return
	}
	m.say(say)
}

// viewAmbitions is the panel: the plans with their bars and next steps,
// then the selected plan's blurb and steps.
func (m *Model) viewAmbitions() string {
	rows := m.ambitions()
	if len(rows) == 0 {
		return m.modal("AMBITIONS", []string{"No ambition is open in this run."}, m.modalFooter())
	}
	m.amb.cursor = max(0, min(m.amb.cursor, len(rows)-1))
	var cells [][]any
	for _, a := range rows {
		next := any(styled{theme.Subtle, nextLabel(a)})
		if a.Done {
			next = styled{theme.Good, doneWord(a)}
		}
		var mark any = ""
		if a.Pinned {
			mark = styled{theme.Gold, "plan"}
		}
		cells = append(cells, []any{a.Name, gauge{frac: a.Progress, n: a.Progress * 100}, next, mark})
	}
	body := table([]col{{"ambition", kText, 0}, {"done %", kBar, dashBarW}, {"next", kText, 0}, {"", kText, 0}}, cells, m.amb.cursor, m.modalInner())
	sel := rows[m.amb.cursor]
	body = append(body, "")
	if row := m.cfg.Ambitions.Ambition(sel.ID); row != nil {
		body = append(body, m.subtle(row.Blurb)...)
	}
	for _, st := range sel.Steps {
		mark, style := "·", theme.Plain
		if st.Done {
			mark, style = "✓", theme.Good
		}
		body = append(body, style.Render(fmt.Sprintf("  %s %s: %s", mark, st.Label, stepWords(st))))
		if sel.ID == content.AmbitionCity && st.ID == "factions" && !st.Done {
			body = append(body, m.downLines()...)
		}
	}
	if sel.Done && sel.Ending != "" {
		// Where a ready ending is taken (#498): the panel said ready and
		// the retiree never found the walk away. The one key this body
		// names, drawn the pane's way as the tutorial line's are.
		body = append(body, "", emptyState("Ready to take: ", "w", " on the dashboard walks away on it."))
	}
	return m.modal("AMBITIONS", body, m.modalFooter())
}

// downLines are the crown's "crews down" step spelled out (#472): a
// line a faction that does not count yet, with what keeps it off the
// count and for how long (downWords).
func (m *Model) downLines() []string {
	var out []string
	for _, r := range m.w.Rivals {
		if r == nil {
			continue
		}
		words := m.downWords(r)
		if words == "" {
			continue
		}
		name := r.Leader
		if name == "" {
			name = "a crew to come"
		}
		for i, l := range wrap(name+": "+words, m.modalInner()-6) {
			if i == 0 {
				l = m.factionStyle(r.Faction()).Render(name) + theme.Subtle.Render(strings.TrimPrefix(l, name))
			} else {
				l = theme.Subtle.Render(l)
			}
			out = append(out, "      "+l)
		}
	}
	return out
}

// nextLabel is the plan's next step as the table's cell: the label
// alone.
func nextLabel(a engine.AmbitionView) string {
	for _, st := range a.Steps {
		if st.ID == a.Next {
			return st.Label
		}
	}
	return ""
}

// doneWord is what a plan met says: an ending is ready to take, the
// milestone made.
func doneWord(a engine.AmbitionView) string {
	if a.Ending == "" {
		return "made"
	}
	return "ready"
}

// stepWords is a step's reading in words, by its unit: `$412,000 of
// $750,000`, `3 of 14 days`, `40 against 62`.
func stepWords(st engine.AmbitionStepView) string {
	have, need := int(st.Have), int(st.Need)
	switch st.Unit {
	case game.UnitCash:
		return money(have) + " of " + money(need)
	case game.UnitClean, game.UnitDirty:
		if st.Done {
			return "owned"
		}
		return fmt.Sprintf("%s %s, %s in hand", money(need), st.Unit, money(have))
	case game.UnitDays:
		return fmt.Sprintf("%d of %s", have, plural(need, "day"))
	case game.UnitPoints:
		return fmt.Sprintf("%.0f against %.0f", st.Have, st.Need)
	case game.UnitIncome:
		return fmt.Sprintf("%s/day against %s last night", money(have), money(need))
	}
	return fmt.Sprintf("%d of %d", have, need)
}

// stepPart is a step as one part of the plan's line (#465): a count
// as `14/14`, anything else as how far along it is, `0%`.
func stepPart(st engine.AmbitionStepView) string {
	switch st.Unit {
	case game.UnitDays, game.UnitCorners, game.UnitCount:
		return fmt.Sprintf("%s %d/%d", st.Label, int(st.Have), int(st.Need))
	}
	frac := game.AmbitionStep{Have: st.Have, Need: st.Need, Done: st.Done}.Frac()
	return st.Label + " " + format.Pct(frac, 0)
}

// planParts is the plan's steps, each its part, joined: `the account
// 0% · quiet days 14/14`. The one bar was their mean, and "Retire clean
// 50%" with nothing offshore read as half the money (#465).
func planParts(a engine.AmbitionView) string {
	var parts []string
	for _, st := range a.Steps {
		parts = append(parts, stepPart(st))
	}
	return strings.Join(parts, " · ")
}

// planFact is the dashboard's line on the pinned plan: `plan Retire
// clean · the account 0% · quiet days 3/14`, or `plan Retire clean
// ready`; "" with none pinned.
func (m *Model) planFact() string {
	a, ok := m.plan()
	if !ok {
		return ""
	}
	if a.Done {
		return theme.Good.Render("plan " + a.Name + " " + doneWord(a))
	}
	line := "plan " + a.Name + " · " + planParts(a)
	if why := m.quietReset(a); why != "" {
		line += " · reset by " + why
	}
	return theme.Gold.Render(line)
}

// quietReset is what last broke the quiet streak retiring counts
// (#465), in words, while the plan pinned is Retire clean and its
// quiet days are short: `a sting in Eastside on day 41`; "" otherwise.
// A save loaded forgets it, as the status bar does.
func (m *Model) quietReset(a engine.AmbitionView) string {
	ev := m.quietBroke
	if ev == nil || a.ID != content.AmbitionRetire {
		return ""
	}
	for _, st := range a.Steps {
		if st.ID == "quiet" && st.Done {
			return ""
		}
	}
	where := ""
	if c := m.w.Cities[ev.City]; c != nil {
		where = " in " + c.Name
	}
	var what string
	switch ev.Cause {
	case events.QuietHeat:
		what = "heat at the retire line" + where
	case events.QuietPolice:
		what = "a " + bustLevelWord(ev.Level) + where
	case events.QuietStrike:
		what = "your strike on a corner"
	case events.QuietPush:
		what = "a push on your corners"
	case events.QuietWar:
		what = "the war getting loud"
	case events.QuietContract:
		what = "a buyer's contract still open" + where
	default:
		return ""
	}
	return fmt.Sprintf("%s on day %d", what, ev.Day)
}

// bustLevelWord is a police level as a line names it: `sting`, `task
// force`.
func bustLevelWord(level string) string {
	if level == content.TaskForce {
		return "task force"
	}
	return level
}

// planReport is the report's PLAN section: the pinned plan's steps and
// its next one in words, one line; nil with none pinned, the report
// before it.
func (m *Model) planReport() []string {
	a, ok := m.plan()
	if !ok {
		return nil
	}
	if a.Done {
		if a.Ending != "" {
			return []string{fmt.Sprintf("%s: ready. Walk away on the dashboard to take it, or play on.", a.Name)} // #498: "ready." alone was a quiet change
		}
		return []string{fmt.Sprintf("%s: %s.", a.Name, doneWord(a))}
	}
	line := fmt.Sprintf("%s: %s", a.Name, planParts(a))
	if why := m.quietReset(a); why != "" {
		line += ", the quiet days reset by " + why
	}
	for _, st := range a.Steps {
		if st.ID == a.Next {
			line += fmt.Sprintf(". Next, %s: %s", st.Label, stepWords(st))
		}
	}
	return []string{line + "."}
}

// planAlert words the pinned plan's milestone (#347): `The plan, Retire
// clean: 1 of 2 steps met.`, or ready.
func (m *Model) planAlert(a engine.Alert) (text, why string) {
	name := a.Ambition
	if row := m.cfg.Ambitions.Ambition(a.Ambition); row != nil {
		name = row.Name
	}
	if a.Ready {
		word := "ready"
		if content.AmbitionEnding[a.Ambition] == "" {
			word = "made" // the milestone ends nothing
		}
		return theme.Good.Render(fmt.Sprintf("The plan, %s: %s.", name, word)), "the plan is " + word
	}
	return theme.Gold.Render(fmt.Sprintf("The plan, %s: %d of %d steps met.", name, a.Count, a.Steps)), "the plan moved on"
}

// ambitionPinned is the panel open on the plan pinned, ambitionUnpinned
// on another: the footer's enter pin or unpin.
func ambitionPinned(m *Model) bool {
	rows := m.ambitions()
	return m.mode == modeAmbitions && len(rows) > 0 && rows[max(0, min(m.amb.cursor, len(rows)-1))].Pinned
}

func ambitionUnpinned(m *Model) bool { return m.mode == modeAmbitions && !ambitionPinned(m) }
