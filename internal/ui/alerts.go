// The alerts: what needs you this morning, one line each, on the
// dashboard's ALERTS and, when new, a fast-forward's stop (#116); with
// them the retirement line and when the float matters (#275: out of
// dashboard.go).

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// alert is one thing that needs you this morning, on the dashboard's
// ALERTS and, when it is new, a fast-forward's stop (#116): the line as
// the panel draws it, the reason as the report's stop line names it
// (`contract due today`) and the key a fast-forward compares morning
// to morning, the alert's identity: a contract's is the contract and
// the day, so each stops once when it is due tomorrow and once when it
// is due today whatever the others; the rest are the reason, which
// carries no number that moves.
type alert struct {
	text string
	why  string
	key  string
}

// newAlert is an alert whose key is its reason.
func newAlert(text, why string) alert { return alert{text, why, why} }

// alerts is what needs you this morning, loudest first: somebody
// talking, a contract due today or tomorrow, the heat where you are at
// or over the patrol line, the dirty cash under the float while a front
// or a route waits on it, the wages the dirty cash cannot pay tonight,
// and, last, the nearest gate ahead while it is within reach (#148,
// unlockAlerts). The dashboard's ALERTS carry them and a fast-forward
// stops on one the morning before did not have; the list is the one
// source for both.
func (m *Model) alerts() []alert {
	w := m.w
	here := w.Here()
	var out []alert
	if m.talking() {
		out = append(out, newAlert(theme.Bad.Bold(true).Render("Somebody is talking.")+theme.Bad.Render(" Investigate "+screenPointer(screenCrew)+"."), "somebody is talking"))
	}
	out = append(out, m.contractAlerts()...)
	out = append(out, m.debtAlerts()...)
	for _, r := range m.rules.Heat.ThresholdsIn(w, here) {
		if r.Level == content.Patrol && here.Heat >= r.Threshold {
			out = append(out, newAlert(theme.Bad.Render(fmt.Sprintf("Heat %.0f in %s is over the patrol line (%.0f).", here.Heat, here.Name, r.Threshold)), "heat in "+here.Name+" over the patrol line"))
		}
	}
	// A task force announced this morning (#48): it comes tonight and
	// takes an asset; a fast-forward stops on it.
	if m.rules.Heat.TaskForceForming(w) {
		out = append(out, newAlert(theme.Bad.Bold(true).Render("A task force formed this morning.")+theme.Bad.Render(" It comes tonight: lie low."), "a task force formed"))
	}
	if fl := m.rules.Laundering.Float(w); w.Player.DirtyCash < fl && m.floatMatters() {
		out = append(out, newAlert(theme.Warning.Render(fmt.Sprintf("Dirty cash %s is under the float (%s): the wash and the road wait.", cash(w.Player.DirtyCash), cash(fl))), "dirty cash under the float"))
	}
	if wages := m.rules.Crew.Wages(w, w.Crew.Pay); wages > w.Player.DirtyCash {
		out = append(out, newAlert(theme.Warning.Render(fmt.Sprintf("Wages %s due tonight, %s dirty in hand.", money(wages), money(w.Player.DirtyCash))), "wages short"))
	}
	out = append(out, m.unlockAlerts()...)
	out = append(out, m.houseAlerts()...)
	// A DA race taking money (#193), while you have clean cash to put in
	// and none in this city's campaign yet.
	if next := m.rules.Law.NextElection(w); w.Law.CampaignOpen && w.Player.CleanCash > 0 && w.Campaigning(here.ID).Cash == 0 && next > 0 {
		out = append(out, newAlert(theme.Warning.Render(fmt.Sprintf("The DA race is %s off and the tickets are taking money %s.", plural(max(0, next-w.Day), "day"), screenPointer(screenLedger))), "the DA race is taking money"))
	}
	if line := m.retireLine(); line != "" {
		out = append(out, newAlert(line, "retirement"))
	}
	// The favour (#228): the chief owes you one and the police come
	// tonight; keyed on the response due, so F stops once a night it
	// could be called.
	if due := m.favourDue(); w.CanCallFavour(due != "") {
		out = append(out, newAlert(theme.Warning.Render(fmt.Sprintf("Chief %s owes you one and the %s comes tonight: call it in %s.", w.Law.Chief.Name, favourWord(due), screenPointer(screenLedger))), "the favour on the "+due))
	}
	// The reign (#227): the city is yours and the crown is there to take;
	// keyed once, so a fast-forward stops the morning it begins.
	if w.Reign > 0 {
		crews, homage := w.HomageDeals()
		who := "every crew gone"
		if crews > 0 {
			who = fmt.Sprintf("%s paying %s a night", plural(crews, "crew"), money(homage))
		}
		out = append(out, newAlert(theme.Gold.Render(fmt.Sprintf("The city is yours: day %d of the reign, %s. Take the crown or play on.", w.ReignDay(), who)), "the city is yours"))
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

// floatMatters is whether anything reads the laundering float: a front
// to wash with or a route with its dial on. A new run's $500 against a
// $50,000 float is nobody's business until then.
func (m *Model) floatMatters() bool {
	if len(m.w.Fronts) > 0 {
		return true
	}
	for _, r := range m.w.Routes {
		if r.Dial != events.RouteOff {
			return true
		}
	}
	return false
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
