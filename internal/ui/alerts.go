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
// heat over the patrol line, a task force forming, the float, the
// wages, a member near a line, the skim, a member with no post, a
// corner nobody works, a full stash, a faction on its way to a city
// where you earn (#341), the gate within reach, a house the police
// know, the DA race,
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
	case engine.AlertFloat:
		text = theme.Warning.Render(fmt.Sprintf("Dirty cash %s is under the float (%s): the wash and the road wait.", cash(a.Have), cash(a.Amount)))
	case engine.AlertWages:
		text = theme.Warning.Render(fmt.Sprintf("Wages %s due tonight, %s dirty in hand.", money(a.Amount), money(a.Have)))
	case engine.AlertCrewLine:
		text, why = m.crewLineAlert(a)
	case engine.AlertSkim:
		text = theme.Bad.Render(fmt.Sprintf("Skimming suspected: money went missing on day %d. Watch the loyalty %s.", a.Day, screenPointer(screenCrew)))
		why = "skimming suspected"
	case engine.AlertUnposted:
		text, why = m.unpostedAlert(a)
	case engine.AlertIdleCorner:
		text, why = m.idleCornerAlert(a)
	case engine.AlertStashFull:
		name := w.CityName(a.City)
		text = theme.Warning.Render(fmt.Sprintf("The stash in %s is full: %d of %d. Rent a house or move stock %s.", name, a.Count, a.Amount, screenPointer(screenLedger)))
		why = "the stash in " + name + " is full"
	case engine.AlertScouts:
		text, why = m.scoutsAlert(a)
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

// scoutsAlert words a faction moving on a city where you earn (#341):
// `Sal's crew has scouts in Bayport: in on day 59. See the rivals
// screen (6).`, amber while it scouts, red once it recruits (the take
// no longer sends it home).
func (m *Model) scoutsAlert(a engine.Alert) (text, why string) {
	who := "Somebody"
	for _, r := range m.w.Rivals {
		if r != nil && r.Scouting() && r.ScoutingCity == a.City {
			who = m.rivalName(r)
		}
	}
	city := m.w.CityName(a.City)
	style, what := theme.Warning, "has scouts in "+city
	if a.Level == "recruiting" {
		style, what = theme.Bad, "is recruiting in "+city
	}
	why = who + " " + what
	when := "in " + plural(a.Days, "day")
	if a.Days <= 0 {
		when = "due now"
	}
	return style.Render(fmt.Sprintf("%s %s: %s. Answer them %s.", who, what, when, screenPointer(screenRivals))), why
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

// unpostedAlert words a runner or an enforcer with no post (#352):
// `Vee has no post: put them on Rail Yard on the map screen (5).`,
// the corner the engine names (its city named when it is not where you
// stand), or, with none, what would give them one.
func (m *Model) unpostedAlert(a engine.Alert) (text, why string) {
	w := m.w
	name, role := "Somebody", game.RoleRunner
	if c := w.Crew.Member(a.Member); c != nil {
		name, role = c.Name, c.Role
	}
	why = name + " has no post"
	c := w.Corner(a.Corner)
	switch {
	case c != nil:
		where := c.Name
		if c.City != w.Here().ID {
			where += " in " + w.CityName(c.City)
		}
		verb := "put them on"
		if role == game.RoleEnforcer {
			verb = "have them guard"
		}
		text = fmt.Sprintf("%s has no post: %s %s %s.", name, verb, where, screenPointer(screenMap))
	case role == game.RoleEnforcer:
		text = fmt.Sprintf("%s has no post: an enforcer needs a corner of yours to guard.", name)
	default:
		text = fmt.Sprintf("%s has no post and no corner free: take one back %s.", name, screenPointer(screenMap))
	}
	return theme.Warning.Render(text), why
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

// alertLines are the ALERTS: what needs you this morning (alerts), the
// selected one marked (#352: the one the dashboard's o opens), then the
// recent heat, rival and law headlines, at most n lines width cells
// wide.
func (m *Model) alertLines(width, n int) []string {
	w := m.w
	var out []string
	alerts := m.alerts()
	if len(alerts) > 0 {
		clamp(&m.alertCursor, len(alerts))
	}
	for i, a := range alerts {
		cur := "  "
		if i == m.alertCursor {
			cur = theme.Gold.Render("▸ ")
		}
		out = append(out, cur+truncate(a.text, max(10, width-2)))
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

// ---- the jump (#352): every alert is one key from what answers it

// alertScreen is an engine screen name (engine.Act.Screen) as the
// TUI's tab: every tab by its word, so a screen the engine names that
// the TUI lacks is not found (TestEveryAlertHasATarget).
func alertScreen(name string) (screen, bool) {
	for s := range screenCount {
		if screens[s].word == name {
			return s, true
		}
	}
	return 0, false
}

// alertSubjects put the cursor of the screen landed on on the alert's
// subject (engine.Act.Subject): the member's row, the corner on its
// city's map, the buyer, the connect, the house, a city's first house.
var alertSubjects = map[string]func(*Model, engine.Alert){
	engine.SubjectMember:   func(m *Model, a engine.Alert) { m.selectMember(a.Member) },
	engine.SubjectCorner:   func(m *Model, a engine.Alert) { m.selectCorner(a.Corner) },
	engine.SubjectContract: func(m *Model, a engine.Alert) { m.selectContract(a.Contract) },
	engine.SubjectSupplier: func(m *Model, a engine.Alert) { m.selectSupplier(a.Supplier) },
	engine.SubjectHouse:    func(m *Model, a engine.Alert) { m.selectHouse(func(h game.House) bool { return h.ID == a.House }) },
	engine.SubjectCity:     func(m *Model, a engine.Alert) { m.selectHouse(func(h game.House) bool { return h.City == a.City }) },
}

// alertModes open the dialog an act names (engine.Act.Mode) once the
// screen and the subject are set: the post picker on the alert's corner,
// the member it names under the cursor.
var alertModes = map[string]func(*Model, engine.Alert){
	engine.ModePost: (*Model).postFromAlert,
}

// openAlert goes where the alert is answered (engine.Alert.Act): the
// screen, the subject under its cursor, the dialog open. It closes the
// modal it is pressed in (the report's stop line) and opens nothing
// that spends.
func (m *Model) openAlert(a engine.Alert) {
	s, ok := alertScreen(a.Act.Screen)
	if !ok {
		return
	}
	m.mode = modePlay
	m.switchScreen(s)
	if f := alertSubjects[a.Act.Subject]; f != nil {
		f(m, a)
	}
	if f := alertModes[a.Act.Mode]; f != nil {
		f(m, a)
	}
}

// openSelectedAlert is the dashboard's o: the alert under the ALERTS
// cursor.
func (m *Model) openSelectedAlert() {
	as := m.sess.Alerts()
	if len(as) == 0 {
		return
	}
	m.openAlert(as[clamp(&m.alertCursor, len(as))])
}

// cycleAlert moves the ALERTS cursor, round the list.
func (m *Model) cycleAlert(d int) {
	if n := len(m.sess.Alerts()); n > 0 {
		m.alertCursor = ((m.alertCursor+d)%n + n) % n
	}
}

// hasAlerts is the dashboard having an alert to pick and open, its
// arrows on the product table: with them on HEAT, CASH and LAW (#355)
// the pane leads with POLICE and the keys wear that region, so the
// alert keys are neither listed nor live there.
func hasAlerts(m *Model) bool { return !m.onPolice && len(m.sess.Alerts()) > 0 }

// stoppedOnAlert is the report open on a fast-forward that stopped on
// an alert: its o opens it.
func stoppedOnAlert(m *Model) bool { return m.fastAlert != nil }

// selectMember puts the crew cursor on the member.
func (m *Model) selectMember(id int) {
	for i, c := range m.w.Crew.Members {
		if c.ID == id {
			m.crewCursor = i
		}
	}
}

// selectCorner turns the map to the corner's city with the cursor on
// it.
func (m *Model) selectCorner(id string) {
	c := m.w.Corner(id)
	if c == nil {
		return
	}
	m.city = c.City
	m.routeCursor, m.onRoutes = 0, false
	for i, x := range m.shown().Corners {
		if x.ID == id {
			m.mapCursor = i
		}
	}
}

// selectContract turns the market to the contract's city with the
// cursor on its buyer.
func (m *Model) selectContract(id int) {
	c := m.w.Contract(id)
	if c == nil {
		return
	}
	m.city = c.City
	m.onSuppliers, m.supplierCursor = false, 0
	for i, r := range m.buyerRows() {
		if r.ID == id {
			m.onBuyers, m.buyerCursor = true, i
		}
	}
}

// selectSupplier turns the market to the connect's city with the cursor
// on them.
func (m *Model) selectSupplier(id string) {
	sup := m.w.Supplier(id)
	if sup == nil {
		return
	}
	m.city = sup.City
	m.onBuyers, m.buyerCursor = false, 0
	for i, r := range m.supplierRows() {
		if r.ID == id {
			m.onSuppliers, m.supplierCursor = true, i
		}
	}
}

// selectHouse puts the ledger's cursor on the first house that matches.
func (m *Model) selectHouse(match func(game.House) bool) {
	for i, r := range m.ledgerRows() {
		if r.kind == ledgerHouse && match(m.w.Houses[r.i]) {
			m.ledgerCursor = i
			return
		}
	}
}

// postFromAlert opens the post picker on the alert's corner for the
// role it wants: the member's own, else a runner; the cursor on the
// member, else on the first runner with nowhere to be.
func (m *Model) postFromAlert(a engine.Alert) {
	w := m.w
	m.selectCorner(a.Corner)
	role := game.RoleRunner
	if c := w.Crew.Member(a.Member); c != nil && c.Role == game.RoleEnforcer {
		role = game.RoleEnforcer
	}
	m.askPost(role)
	if m.mode != modePost {
		return
	}
	for i, r := range m.postRows(role) {
		if (a.Member != 0 && r.ID == a.Member) || (a.Member == 0 && r.ID != game.You && w.PostOf(r.ID) == nil && r.Fit(w.Day)) {
			m.pick.cursor = i
			return
		}
	}
}
