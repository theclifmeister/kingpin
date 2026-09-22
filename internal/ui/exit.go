package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The exits (#49, docs/endings.md): w on the dashboard opens the
// walk-away dialog, one modal with two pages that asks twice. The first
// page is the three ways out with their terms and whether each is open:
// retiring on the offshore account (#195: retire_cash in the account and
// retire_days quiet in a row), vanishing on a new identity (the tree's
// identity node) and taking the crown (#227: the reign on, World.Reign).
// enter on an open one turns to the second page, the confirmation, and
// y ends the run there and then: the ending is written through
// World.Retire, World.Vanish or World.Crown, the run saves, and modeOver
// opens on the ending's scene as a morning would.

// exitDialog is the walk-away dialog's state: the page and the row.
type exitDialog struct {
	stepper
	cursor int // 0 retire, 1 vanish, 2 the crown
}

func (d *exitDialog) field() *numberField { return nil }

// exitRow is one way out on the first page.
type exitRow struct {
	cause string
	name  string
	terms string
	open  bool
	short string // what is short, for the refusal
}

// exitRows lists the two ways out with their terms as they stand.
func (m *Model) exitRows() []exitRow {
	w := m.w
	off := m.set.Laundering.Offshore()
	fx := game.FoldEffects(w, m.cfg.Upgrades)
	retire := exitRow{cause: content.CauseRetired, name: "Retire", open: m.set.Laundering.CanRetire(w)}
	retire.terms = fmt.Sprintf("%s offshore and %s quiet", money(off.RetireCash), plural(off.RetireDays, "day"))
	var parts []string
	if s := off.RetireCash - w.Offshore; s > 0 {
		parts = append(parts, money(s)+" short")
	}
	if d := off.RetireDays - w.QuietDays; d > 0 {
		parts = append(parts, plural(d, "more quiet day"))
	}
	retire.short = strings.Join(parts, ", ")
	vanish := exitRow{cause: content.CauseVanished, name: "Vanish", terms: "a new identity from the tree", open: w.CanVanish(fx)}
	if !vanish.open {
		vanish.short = "no new identity"
	}
	crown := exitRow{cause: content.CauseKingpin, name: "Take the crown", terms: "the city yours: every crew gone or paying", open: w.CanCrown()}
	if crown.open {
		crown.terms = fmt.Sprintf("day %d of the reign", w.ReignDay())
	} else {
		crown.short = "the city is not yours"
	}
	return []exitRow{retire, vanish, crown}
}

// askExit opens the dialog on its first page.
func (m *Model) askExit() {
	if m.w.Over != nil {
		return
	}
	m.exit = exitDialog{}
	rows := m.exitRows()
	for i, r := range rows {
		if r.open {
			m.exit.cursor = i
			break
		}
	}
	m.mode = modeExit
}

// keyExit is the dialog's keys: the cursor on the first page, enter to
// the confirmation on an open way out (a closed one is refused with
// what is short), shift+tab back, y on the confirmation, esc closes.
func (m *Model) keyExit(key string) {
	rows := m.exitRows()
	if closes(key) {
		m.mode = modePlay
		return
	}
	switch key {
	case "shift+tab":
		m.exit.back(noField)
	case "up", "k":
		if m.exit.step == 0 && m.exit.cursor > 0 {
			m.exit.cursor--
		}
	case "down", "j":
		if m.exit.step == 0 && m.exit.cursor < len(rows)-1 {
			m.exit.cursor++
		}
	case "enter", "tab":
		if m.exit.step == 0 {
			m.openExit(rows)
		}
	case "y", "Y":
		if m.exit.step == 1 {
			m.confirmExit()
		}
	default:
		switch {
		case m.exit.step == 1 && key != "up" && key != "down" && key != "j" && key != "k":
			m.mode = modePlay // the confirmation declines on any other key, as every confirmation does (#241)
		case m.exit.step == 0 && len(key) == 1 && key[0] >= '1' && key[0] <= '9':
			if i := int(key[0] - '1'); i < len(rows) {
				m.exit.cursor = i
				m.openExit(rows) // a digit selects and commits, as in every picker (#241)
			}
		}
	}
}

// openExit turns to the confirmation for the way out under the cursor,
// or refuses a closed one.
func (m *Model) openExit(rows []exitRow) {
	r := rows[max(0, min(m.exit.cursor, len(rows)-1))]
	if !r.open {
		m.refuse(fmt.Sprintf("%s is not open: %s.", r.name, r.short))
		return
	}
	m.exit.step = 1
}

// confirmExit ends the run the way the cursor says, saves, and opens
// the ending's scene; a refusal from the world is the status bar's.
func (m *Model) confirmExit() {
	rows := m.exitRows()
	r := rows[max(0, min(m.exit.cursor, len(rows)-1))]
	m.mode = modePlay
	var err error
	switch r.cause {
	case content.CauseRetired:
		err = m.set.Laundering.Retire(m.w)
	case content.CauseKingpin:
		err = m.w.Crown()
	default:
		err = m.w.Vanish(game.FoldEffects(m.w, m.cfg.Upgrades))
	}
	if err != nil {
		m.refuse("Can't: " + err.Error() + ".")
		return
	}
	m.save()
	m.finish(true)
}

// viewExit is the dialog: the ways out, then the confirmation.
func (m *Model) viewExit() string {
	w := m.w
	rows := m.exitRows()
	m.exit.cursor = max(0, min(m.exit.cursor, len(rows)-1))
	if m.exit.step == 0 {
		var cells [][]any
		for _, r := range rows {
			var open any = styled{theme.Good, "open"}
			if !r.open {
				open = styled{theme.Subtle, r.short}
			}
			cells = append(cells, []any{r.name, r.terms, open})
		}
		body := table([]col{{"way out", kText, 0}, {"terms", kText, 0}, {"", kText, 0}}, cells, m.exit.cursor, m.modalInner())
		body = append(body, "")
		body = append(body, m.subtle(fmt.Sprintf("The account holds %s and %s quiet. Whatever you leave with, the run ends this morning: the pile, the stock, the crew and the fronts stay behind, and the account over one plus the bodies is the score.", money(w.Offshore), plural(w.QuietDays, "day")))...)
		return m.modal("WALK AWAY", body, m.modalFooter())
	}
	r := rows[m.exit.cursor]
	var body []string
	switch r.cause {
	case content.CauseRetired:
		body = m.wrapLines(fmt.Sprintf("Retire on %s offshore, %s quiet. Nobody comes looking. The run ends now, on day %d.", money(w.Offshore), plural(w.QuietDays, "day"), w.Day))
	case content.CauseKingpin:
		crews, homage := w.HomageDeals()
		body = m.wrapLines(fmt.Sprintf("Take the crown on day %d of the reign: %s paying homage, %s a night, %s offshore. The city stays yours in the epilogue; the run ends now, on day %d.", w.ReignDay(), plural(crews, "crew"), money(homage), money(w.Offshore), w.Day))
	default:
		body = m.wrapLines(fmt.Sprintf("Vanish on the new identity with %s offshore. The DA keeps looking; the papers are good. The run ends now, on day %d.", money(w.Offshore), w.Day))
	}
	body = append(body, "", theme.Gold.Render(fmt.Sprintf("Score %s: %s over 1 + %s.", cash(w.Score()), cash(w.Offshore), plural(w.Stats.Bodies, "body"))))
	body = append(body, theme.Subtle.Render(fmt.Sprintf("Left behind: %s dirty, %s clean, %s in stock, %s.", cash(w.Player.DirtyCash), cash(w.Player.CleanCash), plural(w.TotalStock(), "unit"), plural(len(w.Crew.Members), "member"))))
	return m.modal(strings.ToUpper(r.name)+"?", body, m.modalFooter())
}

// exitConfirming is the dialog being on its confirmation page.
func exitConfirming(m *Model) bool { return m.mode == modeExit && m.exit.step == 1 }

// exitRetiring is the confirmation being retirement's; exitVanishing
// the identity's; exitCrowning the crown's (#227).
func exitRetiring(m *Model) bool  { return exitConfirming(m) && m.exit.cursor == 0 }
func exitVanishing(m *Model) bool { return exitConfirming(m) && m.exit.cursor == 1 }
func exitCrowning(m *Model) bool  { return exitConfirming(m) && m.exit.cursor == 2 }
