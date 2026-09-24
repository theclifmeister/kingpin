package ui

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
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
// the plan under it, a digit to pick and pin, esc to close.
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
			m.amb.cursor = i
			m.pinAmbition(rows) // a digit selects and commits, as in every picker (#241)
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
	}
	return m.modal("AMBITIONS", body, m.modalFooter())
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
	case game.UnitClean:
		if st.Done {
			return "owned"
		}
		return fmt.Sprintf("%s clean, %s in hand", money(need), money(have))
	case game.UnitDays:
		return fmt.Sprintf("%d of %s", have, plural(need, "day"))
	case game.UnitPoints:
		return fmt.Sprintf("%.0f against %.0f", st.Have, st.Need)
	case game.UnitIncome:
		return fmt.Sprintf("%s/day against %s last night", money(have), money(need))
	}
	return fmt.Sprintf("%d of %d", have, need)
}

// planFact is the dashboard's line on the pinned plan: `plan Retire
// clean 42% · next quiet days: 3 of 14 days`, or `plan Retire clean
// ready`; "" with none pinned.
func (m *Model) planFact() string {
	a, ok := m.plan()
	if !ok {
		return ""
	}
	if a.Done {
		return theme.Good.Render("plan " + a.Name + " " + doneWord(a))
	}
	line := fmt.Sprintf("plan %s %s", a.Name, format.Pct(a.Progress, 0))
	for _, st := range a.Steps {
		if st.ID == a.Next {
			line += " · next " + st.Label + ": " + stepWords(st)
		}
	}
	return theme.Gold.Render(line)
}

// planReport is the report's PLAN section: the pinned plan's bar and
// its next step, one line; nil with none pinned, the report before it.
func (m *Model) planReport() []string {
	a, ok := m.plan()
	if !ok {
		return nil
	}
	if a.Done {
		return []string{fmt.Sprintf("%s: %s.", a.Name, doneWord(a))}
	}
	line := fmt.Sprintf("%s: %s", a.Name, format.Pct(a.Progress, 0))
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
