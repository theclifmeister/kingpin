// The start menu (modeStart): the save slots, continuing or starting a
// run in one, and the confirmations for a new run and a deleted slot
// (#275: out of model.go).

package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// keyStart is the start menu: the slots and Quit under one cursor, enter
// takes the row, D asks before emptying a slot, q quits.
func (m *Model) keyStart(key string) (tea.Model, tea.Cmd) {
	rows := game.SlotCount + 1
	switch key {
	case "up", "k":
		wrapCursor(&m.startChoice, -1, rows)
	case "down", "j":
		wrapCursor(&m.startChoice, 1, rows)
	case "enter":
		return m.pickStart()
	case "D":
		if m.startChoice >= game.SlotCount || game.Slots()[m.startChoice].Empty {
			m.refuse("Nothing to delete.")
			return m, nil
		}
		m.cfm = confirm{verb: "delete", view: (*Model).deleteConfirm, act: (*Model).confirmDelete, back: modeStart} // no returns to the menu, not the run
		m.mode = modeConfirm
	case "q":
		return m.quit()
	}
	return m, nil
}

// pickStart takes the start menu's row: a full slot continues its run,
// an empty one starts a new run in it, Quit quits. A run that does not
// load leaves the menu up with the error under the rows.
func (m *Model) pickStart() (tea.Model, tea.Cmd) {
	if m.startChoice >= game.SlotCount {
		return m.quit()
	}
	slot := m.startChoice + 1
	if game.Slots()[m.startChoice].Empty {
		m.openNewRun(slot)
		return m, nil
	}
	if err := m.continueRun(slot); err != nil {
		m.alarm(fmt.Sprintf("Could not load slot %d: %s", slot, err.Error()))
	}
	return m, nil
}

// confirmDelete empties the slot under the start menu's cursor.
func (m *Model) confirmDelete() {
	slot := m.startChoice + 1
	m.mode = modeStart
	if err := game.DeleteSave(slot); err != nil {
		m.alarm(fmt.Sprintf("Could not delete slot %d: %s", slot, err.Error()))
		return
	}
	m.say(fmt.Sprintf("Slot %d deleted.", slot))
}

// viewStart is the start menu: the three slots and Quit under one
// cursor, and the delete confirmation over it. There is no run behind
// it, so no frame: the box sits where it does on every other screen,
// or under the title's art while its loop runs (#152: from 80x24 with
// animation on; otherwise the menu is as it always was).
func (m *Model) viewStart() string {
	var box string
	switch {
	case m.mode == modeConfirm: // the slot deletion, the one confirmation over the menu
		box = m.cfm.view(m)
	case m.mode == modeNewRun:
		box = m.viewNewRun()
	default:
		body := []string{theme.Subtle.Render("a drug empire, one day at a time"), ""}
		for i, o := range m.startRows() {
			if i == m.startChoice {
				body = append(body, theme.Gold.Render("▸ ")+theme.Selected.Render(" "+o+" "))
			} else {
				body = append(body, "   "+o)
			}
		}
		if h := m.historyLine(); h != "" {
			body = append(body, "", theme.Subtle.Render(h))
		}
		if m.profileErr != "" {
			body = append(body, "")
			for _, l := range m.wrapLines(m.profileErr) {
				body = append(body, theme.Warning.Render(l))
			}
		}
		if m.status != "" {
			// Load errors can be long; wrap inside the box instead of past it.
			body = append(body, "")
			for _, l := range m.wrapLines(m.status) {
				body = append(body, m.statusStyle().Render(l))
			}
		}
		box = m.modal("KINGPIN", body, m.modalFooter())
	}
	if art := m.titleArt(); art != nil {
		box = strings.Join(art, "\n") + "\n" + box
	}
	return theme.Plain.Width(m.width).Height(m.height).MaxHeight(m.height).Render("\n" + box)
}

// startRows is the start menu's rows: one a slot, then Quit.
func (m *Model) startRows() []string {
	rows := make([]string, 0, game.SlotCount+1)
	for _, s := range game.Slots() {
		rows = append(rows, slotLine(s, m.now(), m.cfg.Endings.Title))
	}
	return append(rows, "Quit")
}

// slotLine is what the start menu says of a slot: `Slot 1 · day 42 ·
// $1.2M · Eastside · saved 2h ago`, or `Slot 2 · empty`. A run that is
// over carries its ending's title, the one its summary opens with
// (`Slot 1 · INDICTED · day 21 · …`), so it does not read as a run to go
// back to (#441), and its score where a run going on has its cash
// (#498: `score $756K`, not the $45K left behind); title is the
// endings' Title.
func slotLine(s game.SlotInfo, now time.Time, title func(string) string) string {
	if s.Empty {
		return fmt.Sprintf("Slot %d · empty", s.Slot)
	}
	parts := []string{fmt.Sprintf("Slot %d", s.Slot)}
	if s.Ended != "" {
		parts = append(parts, title(s.Ended))
	}
	parts = append(parts, fmt.Sprintf("day %d", s.Day))
	if s.Ended != "" {
		parts = append(parts, "score "+cash(s.Score)) // #498: the cash left behind read as what the run was worth
	} else {
		parts = append(parts, cash(s.Cash))
	}
	if s.City != "" {
		parts = append(parts, s.City)
	}
	parts = append(parts, "saved "+format.Ago(now.Sub(s.Saved)))
	return strings.Join(parts, " · ")
}

// newConfirm asks before the run is abandoned for a new one.
func (m *Model) newConfirm() string {
	return m.modal("NEW RUN?", []string{"Abandon the current run and start over?"}, m.modalFooter())
}

// deleteConfirm asks before a slot is emptied, naming the run in it.
func (m *Model) deleteConfirm() string {
	s := game.Slots()[m.startChoice]
	line := slotLine(s, m.now(), m.cfg.Endings.Title)
	if i := strings.Index(line, " · "); i >= 0 {
		line = line[i+len(" · "):]
	}
	line = strings.ToUpper(line[:1]) + line[1:]
	return m.modal(fmt.Sprintf("DELETE SLOT %d?", s.Slot), []string{line, "The run is gone for good."}, m.modalFooter())
}
