// The alerts: what needs you this morning, one line each, on the
// dashboard's ALERTS and, when new, a fast-forward's stop (#116); with
// them the retirement line and when the float matters (#275: out of
// dashboard.go).

package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// alert is one of the engine's alerts (#298, engine.Alerts) as the
// TUI words it: the line as the dashboard's ALERTS draws it, the reason
// as the report's stop line names it (`contract due today`) and the
// engine's key, the alert's identity morning to morning that a
// fast-forward compares.
type alert struct {
	kind engine.AlertKind
	text string
	why  string
	key  string
}

// alerts is what needs you this morning, loudest first, in the engine's
// order (engine.Alerts: somebody talking, a contract or a debt due, the
// heat over the patrol line, a task force forming, an investigation
// (#343), the float, the wages, a member near a line, the skim, a
// corner nobody works, the gate within reach, a house the police know,
// the DA race,
// retirement, the favour, the reign). The dashboard's ALERTS carry them
// and a fast-forward stops on one the morning before did not have.
func (m *Model) alerts() []alert {
	var out []alert
	for _, a := range m.sess.Alerts() {
		out = append(out, m.alertOf(a))
	}
	return out
}

// alertOf words an engine alert.
func (m *Model) alertOf(a engine.Alert) alert {
	w := m.w
	text, why := "", a.Key
	switch a.Kind {
	case engine.AlertTalking:
		text = theme.Bad.Bold(true).Render("Somebody is talking.") + theme.Bad.Render(" Investigate "+screenPointer(screenCrew)+".")
	case engine.AlertContractDue:
		when, style := "tomorrow", theme.Warning
		if a.Due <= w.Day {
			when, style = "today", theme.Bad
		}
		why = "contract due " + when
		if c := w.Contract(a.Contract); c != nil {
			text = style.Render(fmt.Sprintf("%s: %d %s due %s in %s", c.Name, c.Owed(), w.ProductName(c.Product), when, w.CityName(c.City)))
		}
	case engine.AlertDebtDue:
		why = "debt due tomorrow"
		style := theme.Warning
		if a.Have < a.Amount {
			style = theme.Bad
		}
		if sup := w.Supplier(a.Supplier); sup != nil {
			text = style.Render(fmt.Sprintf("%s: %s due tomorrow, %s in hand.", sup.Name, money(a.Amount), cash(a.Have)))
		}
	case engine.AlertHeat:
		text = theme.Bad.Render(fmt.Sprintf("Heat %.0f in %s is over the patrol line (%.0f).", a.Heat, w.CityName(a.City), a.Line))
	case engine.AlertTaskForce:
		text = theme.Bad.Bold(true).Render("A task force formed this morning.") + theme.Bad.Render(" It comes tonight: lie low.")
	case engine.AlertInvestigation:
		text, why = m.investigationAlert(a)
	case engine.AlertFloat:
		text = theme.Warning.Render(fmt.Sprintf("Dirty cash %s is under the float (%s): the wash and the road wait.", cash(a.Have), cash(a.Amount)))
	case engine.AlertWages:
		text = theme.Warning.Render(fmt.Sprintf("Wages %s due tonight, %s dirty in hand.", money(a.Amount), money(a.Have)))
	case engine.AlertCrewLine:
		text, why = m.crewLineAlert(a)
	case engine.AlertSkim:
		text = theme.Bad.Render(fmt.Sprintf("Skimming suspected: money went missing on day %d. Watch the loyalty %s.", a.Day, screenPointer(screenCrew)))
		why = "skimming suspected"
	case engine.AlertIdleCorner:
		text, why = m.idleCornerAlert(a)
	case engine.AlertGate:
		text, why = theme.Gold.Render(gateText(w, *a.Gate)), gateThe(*a.Gate)+" within reach"
	case engine.AlertHouseKnown:
		if h := w.House(a.House); h != nil {
			text = theme.Bad.Render(fmt.Sprintf("The police know about %s: move the stock out and drop it %s.", h.Name, screenPointer(screenLedger)))
			why = "the police know about " + h.Name
		}
	case engine.AlertDARace:
		text = theme.Warning.Render(fmt.Sprintf("The DA race is %s off and the tickets are taking money %s.", plural(a.Days, "day"), screenPointer(screenLedger)))
	case engine.AlertRetire:
		text = m.retireLine()
	case engine.AlertFavour:
		text = theme.Warning.Render(fmt.Sprintf("Chief %s owes you one and the %s comes tonight: call it in %s.", w.Law.Chief.Name, favourWord(a.Level), screenPointer(screenLedger)))
	case engine.AlertReign:
		who := "every crew gone"
		if a.Count > 0 {
			who = fmt.Sprintf("%s paying %s a night", plural(a.Count, "crew"), money(a.Amount))
		}
		text = theme.Gold.Render(fmt.Sprintf("The city is yours: day %d of the reign, %s. Take the crown or play on.", a.Days, who))
	}
	return alert{kind: a.Kind, text: text, why: why, key: a.Key}
}

// crossWords are what crossing each of the crew's loyalty lines is
// called in an alert: the skim, a lieutenant's flip, the walk.
var crossWords = map[string]string{"skim": "skimming", "flip": "turning", "walk": "walking"}

// crewLineAlert words a member near their next line (#345): `Deshawn
// is 4 from walking (2 days). Pay them, or pay them off on the crew
// screen (4).`, the days at tonight's drift when it is falling; red
// for the walk and a lieutenant's flip, amber for the skim.
func (m *Model) crewLineAlert(a engine.Alert) (text, why string) {
	name := "Somebody"
	if c := m.w.Crew.Member(a.Member); c != nil {
		name = c.Name
	}
	gap := max(1, int(math.Ceil(a.Gap)))
	why = fmt.Sprintf("%s %d from %s", name, gap, crossWords[a.Cross])
	line := fmt.Sprintf("%s is %d from %s", name, gap, crossWords[a.Cross])
	if a.Days > 0 {
		line += " (" + plural(a.Days, "day") + ")"
	}
	line += ". Pay them, or pay them off " + screenPointer(screenCrew) + "."
	style := theme.Bad
	if a.Cross == "skim" {
		style = theme.Warning
	}
	return style.Render(line), why
}

// idleCornerAlert words a corner you hold that nobody works (#345):
// `Nobody works Rail Yard: back to the street in 2 days. Post a runner
// on the map screen (5).`, the city named when it is not where you
// stand; red the night it goes.
func (m *Model) idleCornerAlert(a engine.Alert) (text, why string) {
	w := m.w
	name := a.Corner
	if c := w.Corner(a.Corner); c != nil {
		name = c.Name
	}
	if a.City != w.Here().ID {
		name += " in " + w.CityName(a.City)
	}
	when, style := "in "+plural(a.Days, "day"), theme.Warning
	if a.Days <= 1 {
		when, style = "tonight", theme.Bad
	}
	return style.Render(fmt.Sprintf("Nobody works %s: back to the street %s. Post a runner %s.", name, when, screenPointer(screenMap))), "nobody works " + name
}

// investigationAlert words an open investigation (#343): what the police
// named, where, the nights to the hit and the answer, `Police are
// working Rail Yard: they hit in 3 days. Work another corner on the map
// screen (5), or let them have it.`; red the night it lands.
func (m *Model) investigationAlert(a engine.Alert) (text, why string) {
	w := m.w
	name, answer := "", ""
	switch a.Target {
	case game.LeadCorner:
		name = w.LeadName(a.Target, a.Corner)
		answer = "Work another corner " + screenPointer(screenMap) + ", or let them have it."
	case game.LeadProduct:
		name = "the " + w.ProductName(a.Product) + " trade"
		answer = "Stop selling it there and move it out " + screenPointer(screenMarket) + ", or let them have it."
	case game.LeadHouse:
		name = w.LeadName(a.Target, a.House)
		answer = "Move the stock out " + screenPointer(screenLedger) + ", or let them have it."
	}
	if a.City != w.Here().ID {
		name += " in " + w.CityName(a.City)
	}
	when, style := "in "+plural(a.Days, "day"), theme.Warning
	if a.Days <= 1 {
		when, style = "tonight", theme.Bad
	}
	return style.Render(fmt.Sprintf("Police are working %s: they hit %s. %s", name, when, answer)), "police working " + name
}

// crewTrouble is the morning's crew trouble in a line (#345), `2 near
// the line, 1 corner unworked`, or "" when there is none: the report's
// CREW section and the dashboard's crew fact, counted off the alerts.
func (m *Model) crewTrouble() string {
	near, idle := 0, 0
	for _, a := range m.sess.Alerts() {
		switch a.Kind {
		case engine.AlertCrewLine:
			near++
		case engine.AlertIdleCorner:
			idle++
		}
	}
	var parts []string
	if near > 0 {
		parts = append(parts, fmt.Sprintf("%d near the line", near))
	}
	if idle > 0 {
		parts = append(parts, plural(idle, "corner")+" unworked")
	}
	return strings.Join(parts, ", ")
}

// alertsOf is this morning's alerts of one kind.
func (m *Model) alertsOf(kind engine.AlertKind) []alert {
	var out []alert
	for _, a := range m.alerts() {
		if a.kind == kind {
			out = append(out, a)
		}
	}
	return out
}

// retireLine is how far off retiring is (#195), once the account has
// something in it: `retire in 12 quiet days · $2.4M short`, or that
// it is open. Blank before the first dollar goes offshore, and with no
// exit in the file.
func (m *Model) retireLine() string {
	w := m.w
	off := m.rules.Laundering.Offshore()
	if w.Offshore <= 0 || off.RetireCash <= 0 {
		return ""
	}
	if m.rules.Laundering.CanRetire(w) {
		return theme.Good.Render(fmt.Sprintf("You could retire: %s offshore, %s quiet.", cash(w.Offshore), plural(w.QuietDays, "day")))
	}
	var parts []string
	if d := off.RetireDays - w.QuietDays; d > 0 {
		parts = append(parts, fmt.Sprintf("retire in %s", plural(d, "quiet day")))
	}
	if s := off.RetireCash - w.Offshore; s > 0 {
		parts = append(parts, cash(s)+" short")
	}
	return theme.Subtle.Render(strings.Join(parts, " · "))
}

// alertLines are the ALERTS: what needs you this morning (alerts), then
// the recent heat, rival and law headlines, at most n lines width cells
// wide.
func (m *Model) alertLines(width, n int) []string {
	w := m.w
	var out []string
	for _, a := range m.alerts() {
		out = append(out, truncate(a.text, max(10, width)))
	}
	for i := len(w.Journal) - 1; i >= 0 && len(out) < max(1, n); i-- {
		if src := w.Journal[i].Source; src != "heat" && src != "rivals" && src != "law" {
			continue
		}
		day := fmt.Sprintf("d%-2d ", w.Journal[i].Day)
		out = append(out, theme.Subtle.Render(day)+truncate(w.Journal[i].Text, max(10, width-lipgloss.Width(day))))
	}
	if len(out) == 0 {
		out = append(out, emptyState("Nobody is looking at you. Yet."))
	}
	return out
}

// alertsPanel is the ALERTS panel for a layout with room rows left and
// no pane beside MAIN, sized to its lines, or "" where there is no
// room for one (a border with nothing inside is not a panel).
func (m *Model) alertsPanel(width, room int) string {
	if room < 3 || m.paneShown() {
		return ""
	}
	alerts := m.alertLines(width-4, room-2)
	return panel("ALERTS", strings.Join(alerts, "\n"), width, len(alerts)+2, theme.Heat)
}
