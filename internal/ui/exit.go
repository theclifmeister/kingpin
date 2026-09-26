package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The exits (#49, docs/endings.md): w on the dashboard opens the
// walk-away dialog, one modal with two pages that asks twice. The first
// page is the three ways out with their terms and whether each is open:
// retiring on the offshore account (#195: retire_cash in the account and
// retire_days quiet in a row), vanishing on a new identity (the tree's
// identity node), taking the crown (#227: the reign on, World.Reign) and
// going straight (#398: the fronts out-earning the street legit_days
// nights running, World.LegitDays).
// enter on an open one turns to the second page, the confirmation, and
// y ends the run there and then: the ending is written through
// World.Retire, World.Vanish, World.Crown or World.GoStraight, the run saves, and modeOver
// opens on the ending's scene as a morning would.

// exitDialog is the walk-away dialog's state: the page and the row.
type exitDialog struct {
	stepper
	cursor int // 0 retire, 1 vanish, 2 the crown, 3 going straight (#398)
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

// exitRows lists the four ways out with their terms as they stand,
// every one closed while last night's lump is still to be read (#494).
func (m *Model) exitRows() []exitRow {
	w := m.w
	off := m.rules.Laundering.Offshore()
	fx := game.FoldEffects(w, m.cfg.Upgrades)
	retire := exitRow{cause: content.CauseRetired, name: "Retire", open: m.rules.Laundering.CanRetire(w)}
	retire.terms = fmt.Sprintf("%s offshore and %s quiet", money(off.RetireCash), plural(off.RetireDays, "day"))
	var parts []string
	if s := off.RetireCash - w.Offshore; s > 0 {
		parts = append(parts, money(s)+" short")
	}
	if d := off.RetireDays - w.QuietDays; d > 0 {
		parts = append(parts, plural(d, "more quiet day"))
	}
	retire.short = strings.Join(parts, ", ")
	// The terms are the plans' own (#498, docs/ambitions.md): the
	// ambitions panel and the walk away name the same conditions.
	vanish := exitRow{cause: content.CauseVanished, name: "Vanish", terms: "a new identity from the tree, after the lawyer on call and on retainer", open: w.CanVanish(fx)}
	if !vanish.open {
		vanish.short = "no new identity"
	}
	end := m.cfg.Rivals.Endings
	crown := exitRow{cause: content.CauseKingpin, name: "Take the crown", open: w.CanCrown()}
	crown.terms = fmt.Sprintf("more than %s of home's corners and every crew gone or paying, %s running", format.Pct(end.KingpinShare, 0), plural(end.DominantDays, "day"))
	if crown.open {
		crown.terms = fmt.Sprintf("day %d of the reign", w.ReignDay())
	} else {
		crown.short = m.crownShort()
	}
	straight := exitRow{cause: content.CauseBusinessman, name: "Go straight", open: m.rules.Laundering.CanGoStraight(w)}
	if days := m.cfg.Laundering.Businessman.LegitDays; straight.open {
		straight.terms = fmt.Sprintf("the fronts at %s a day", money(m.rules.Laundering.LegitIncome(w)))
	} else {
		straight.terms = fmt.Sprintf("the fronts out-earn the street and goodwill tops pressure at home, %s running", plural(days, "night"))
		straight.short = m.straightShort(days)
	}
	rows := []exitRow{retire, vanish, crown, straight}
	// Last night's lump offshore is read tonight (#494): every way out
	// waits on its pages, which the session refuses too.
	if pages := m.sess.PagesDue(); pages > 0 {
		for i := range rows {
			if rows[i].open {
				rows[i].open = false
				rows[i].short = fmt.Sprintf("%s from last night's transfer go in the DA's file tonight: end the day first", plural(pages, "page"))
			}
		}
	}
	return rows
}

// straightShort is what going straight waits on, in the legit plan's
// own steps (#498): the nights so far and whichever of the fronts over
// the street and goodwill over pressure fails this morning.
func (m *Model) straightShort(days int) string {
	parts := []string{fmt.Sprintf("%d of %s so far", m.w.LegitDays, plural(days, "night"))}
	for _, a := range m.sess.Ambitions() {
		if a.ID != content.AmbitionLegit {
			continue
		}
		for _, s := range a.Steps {
			switch {
			case s.Done:
			case s.ID == "income":
				parts = append(parts, "the fronts under the street")
			case s.ID == "goodwill":
				parts = append(parts, "goodwill under pressure")
			}
		}
	}
	return strings.Join(parts, ", ")
}

// crownShort is what the crown waits on (#399), in the kingpin plan's
// own steps (the ambition reads the detector's terms): the corners
// short of the share, the crews still standing, the days of the streak
// to go; or that the reign is slipping under the share.
func (m *Model) crownShort() string {
	w := m.w
	if w.Reign > 0 && w.ReignSlip > 0 {
		return "the city is slipping under the share: hold more corners"
	}
	var parts []string
	for _, a := range m.sess.Ambitions() {
		if a.ID != content.AmbitionCity {
			continue
		}
		for _, s := range a.Steps {
			if s.Done {
				continue
			}
			switch s.ID {
			case "share":
				parts = append(parts, fmt.Sprintf("%d of %d corners held", int(s.Have), int(s.Need)))
			case "factions":
				// The crews still to arrive apart (#478): a seat in the
				// wings blocks the crown as a crew on the street does,
				// and read as one it looked like a crew you could fight.
				coming := 0
				for _, r := range w.Rivals {
					if r != nil && r.Arrived == 0 && !m.rules.Rivals.Down(w, r).Counts {
						coming++
					}
				}
				if standing := int(s.Need-s.Have) - coming; standing > 0 {
					parts = append(parts, plural(standing, "crew")+" still standing")
				}
				if coming > 0 {
					parts = append(parts, plural(coming, "crew")+" yet to arrive")
				}
			case "streak":
				if s.Have > 0 || len(parts) == 0 {
					parts = append(parts, fmt.Sprintf("day %d of %d", int(s.Have), int(s.Need)))
				}
			}
		}
	}
	if len(parts) == 0 {
		return "the city is not yours"
	}
	return strings.Join(parts, ", ")
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
// what is short), a to the ambitions panel (#347), shift+tab back, y on
// the confirmation, esc closes.
func (m *Model) keyExit(key string) {
	rows := m.exitRows()
	m.exit.err = ""
	if closes(key) {
		m.mode = modePlay
		return
	}
	switch key {
	case "shift+tab":
		m.exit.back(noField)
	case "up", "k":
		if m.exit.step == 0 {
			stepCursor(&m.exit.cursor, -1, len(rows))
		}
	case "down", "j":
		if m.exit.step == 0 {
			stepCursor(&m.exit.cursor, 1, len(rows))
		}
	case "enter", "tab":
		if m.exit.step == 0 {
			m.openExit(rows)
		}
	case "a":
		if m.exit.step == 0 {
			m.openAmbitions(false) // the plans toward the ways out (#347)
		} else {
			m.mode = modePlay // the confirmation declines on any other key (#241)
		}
	case "y", "Y":
		if m.exit.step == 1 {
			m.confirmExit()
		}
	default:
		switch {
		case m.exit.step == 1 && key != "up" && key != "down" && key != "j" && key != "k":
			m.mode = modePlay // the confirmation declines on any other key, as every confirmation does (#241)
		case m.exit.step == 0:
			if i, ok := digit(key); ok && i < len(rows) {
				m.exit.cursor = i
				m.openExit(rows) // a digit selects and commits, as in every picker (#241)
			}
		}
	}
}

// openExit turns to the confirmation for the way out under the cursor,
// or refuses a closed one with what is short, on the dialog's error
// line (#468: the status bar is under the modal, so a refusal there
// read as enter doing nothing).
func (m *Model) openExit(rows []exitRow) {
	r := rows[max(0, min(m.exit.cursor, len(rows)-1))]
	if !r.open {
		m.exit.err = fmt.Sprintf("%s is not open: %s.", r.name, r.short)
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
		err = m.sess.Retire()
	case content.CauseKingpin:
		err = m.sess.Crown()
	case content.CauseBusinessman:
		err = m.sess.GoStraight()
	default:
		err = m.sess.Vanish()
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
		// Each way out is its row, then its terms and what is short on
		// lines of their own, wrapped rather than cut (#463): a column
		// of terms beside a column of shortfalls was cut at 80 columns.
		// Every way out scores the same (#478): the account over one
		// plus the bodies, whatever the ending; the row says so, so no
		// exit reads as worth more than another.
		score := "the account: " + theme.Gold.Render(money(w.Score()))
		if w.Stats.Bodies > 0 {
			score = fmt.Sprintf("the account over 1 + %s: %s", plural(w.Stats.Bodies, "body"), theme.Gold.Render(money(w.Score())))
		}
		var cells [][]any
		for _, r := range rows {
			var open any = styled{theme.Good, "open"}
			if !r.open {
				open = styled{theme.Subtle, "not yet"}
			}
			cells = append(cells, []any{r.name, open})
		}
		lines := table([]col{{"way out", kText, 0}, {"", kText, 0}}, cells, m.exit.cursor, m.modalInner())
		// What this morning would leave unsettled leads the page (#494):
		// the pages due tonight close every way out.
		body := m.pendingLines()
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, lines[0])
		for i, r := range rows {
			at := len(body)
			body = append(body, lines[1+i], "    "+row("terms", r.terms))
			if !r.open {
				body = append(body, "    "+row("short", theme.Warning.Render(r.short)))
			}
			body = append(body, "    "+row("scores", score))
			if i == m.exit.cursor {
				// A refusal under the row it refuses (#468), in view
				// with the row.
				if m.exit.err != "" {
					for _, l := range wrap(m.exit.err, m.modalInner()-4) {
						body = append(body, "    "+theme.Bad.Render(l))
					}
				}
				// The cursor's way out in view, its terms under it.
				m.modalFollow(len(body) - 1)
				m.modalFollow(at)
			}
		}
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
	case content.CauseBusinessman:
		// Its own copy (#498): it read as the vanish's.
		body = m.wrapLines(fmt.Sprintf("Go straight on %s: the fronts at %s a day, the street given up, %s offshore. The DA's file goes to the archive; the run ends now, on day %d.", plural(len(w.Fronts), "front"), money(m.rules.Laundering.LegitIncome(w)), money(w.Offshore), w.Day))
	default:
		body = m.wrapLines(fmt.Sprintf("Vanish on the new identity with %s offshore. The DA keeps looking; the papers are good. The run ends now, on day %d.", money(w.Offshore), w.Day))
	}
	if pending := m.pendingLines(); len(pending) > 0 {
		body = append(append(body, ""), pending...)
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
func exitStraight(m *Model) bool  { return exitConfirming(m) && m.exit.cursor == 3 }

// pendingLines are what the walk away would leave unsettled this
// morning (#494): the pages last night's lump files tonight, which
// close every way out until the day is ended, and a reserve made today,
// out of the pile already and not in the account until tonight, so a
// walk away this morning neither scores it nor leaves it behind.
func (m *Model) pendingLines() []string {
	var out []string
	if pages := m.sess.PagesDue(); pages > 0 {
		for _, l := range wrap(fmt.Sprintf("Last night's transfer offshore puts %s in the DA's file tonight: end the day first.", plural(pages, "page")), m.modalInner()) {
			out = append(out, theme.Bad.Render(l))
		}
	}
	if r := m.w.ReservedToday(); r > 0 {
		for _, l := range wrap(fmt.Sprintf("%s lands offshore tonight: end the day first, or it is neither scored nor left behind.", money(r)), m.modalInner()) {
			out = append(out, theme.Warning.Render(l))
		}
	}
	return out
}
