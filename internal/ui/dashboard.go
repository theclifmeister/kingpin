package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The dashboard (#83) is the overview: MAIN holds the state, the pane
// what happened (ALERTS), what is selected (the product under the
// cursor and what s would do to it) and the keys. Two layouts: from
// paneMinWidth columns STREET runs the width, sized to its lines, then
// HEAT beside CASH, LAW beside RIVALS and CITIES under them; narrower
// than that STREET, then HEAT, CASH and LAW side by side with the
// rival's line folded into LAW and the other city into the street.
// Panels are sized to what they hold, so no panel carries dead space,
// and the room a layout leaves goes to ALERTS where no pane carries
// them.
const (
	streetMaxH = 15 // STREET's most rows at the wide layout, border included
	dashPanelH = 6  // HEAT, CASH, LAW and RIVALS: four lines and the border
	dashBarW   = 8  // the war, trust and pressure bars
)

// sep joins the facts of a street line.
var sep = theme.Subtle.Render(" · ")

// fact is one thing the street says, with how much it matters when
// the panel is out of room: the lowest go first.
type fact struct {
	s   string
	pri int
}

// What a street fact is worth when the panel is out of room, lowest
// dropped first.
const (
	priTier = iota // the tier the run is in (#147): the first to go
	priSupplier
	priUpgrades
	priStage // the tier while its stage is new (#149): news, so it outranks the counts
	priStash
	priSupply
	priRuns
	priRoad
	priContracts
	priDebt
	priCrew
	priStrike
	priPatrol
	priTalking
	priCorners
)

// pack joins facts into lines of at most width cells, ` · ` between
// them, a fact that does not fit starting the next line.
func pack(facts []fact, width int) []string {
	var out []string
	var line string
	for _, f := range facts {
		switch {
		case f.s == "":
		case line == "":
			line = f.s
		case lipgloss.Width(line)+3+lipgloss.Width(f.s) <= width:
			line += sep + f.s
		default:
			out = append(out, line)
			line = f.s
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

// firstFit is the first of the candidates that fits width cells, or
// the last of them.
func firstFit(width int, candidates ...string) string {
	for _, c := range candidates {
		if lipgloss.Width(c) <= width {
			return c
		}
	}
	return candidates[len(candidates)-1]
}

// streetLines is the STREET panel's content for a panel innerW cells
// wide, at most maxLines of it: the product table with sparklines (the
// sparkline takes what the fixed columns leave, up to 24 days), then
// the state of the street as facts, and last the line on tonight's
// sales, which is never cut. The facts are topics, one a line where
// they fit (the stash and the supplier, the corners and the road, the
// crew and the skim); narrow packs them into as few lines as the width
// allows, and so does a wide panel out of room, which then drops the
// facts that matter least until the rest fit. withRoad adds the stash
// elsewhere and the road, for a layout without the CITIES panel.
func (m *Model) streetLines(innerW, maxLines int, narrow, withRoad bool) []string {
	w := m.w
	here := w.Here()
	cols, rows, cursor := m.productRows(here.ID, m.cursor, false)
	sparkW := max(3, min(24, innerW-tableWidth(cols, rows)))
	sparkCol(cols, rows, sparkW)
	lines := table(cols, rows, cursor, innerW)

	var topics [][]fact
	topic := func(facts ...fact) { topics = append(topics, facts) }
	stash := fact{theme.Subtle.Render(m.stashedLine(here.ID, narrow)), priStash}
	if narrow {
		topic(stash)
	} else {
		topic(stash, fact{theme.Subtle.Render(m.supplierLine()), priSupplier})
	}
	corners := fact{theme.CrewText.Render(m.cornersLine(here.ID)), priCorners}
	if w.Worked() == 0 {
		corners.s = theme.Bad.Render("You hold no corner, so nothing sells. Claim one " + screenPointer(screenMap) + ".")
	}
	tier := fact{theme.Subtle.Render("tier " + w.TierName(m.cfg.Progression)), priTier}
	if w.StagePending() > 0 {
		// The stage not yet seen (#149) is marked the way the Journal tab
		// counts the unread, and the fact is worth a line while it is.
		tier.s += theme.NewsText.Render(" · new")
		tier.pri = priStage
	}
	if withRoad {
		topic(append([]fact{corners, tier}, m.elsewhereFacts()...)...)
	} else {
		topic(corners, tier)
	}
	if n := len(w.Crew.Members); n > 0 {
		crew := fact{theme.CrewText.Render(fmt.Sprintf("crew %d · %s pay %s/day", n, w.Crew.Pay, money(m.set.Crew.Wages(w, w.Crew.Pay)))), priCrew}
		if w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < m.set.Crew.Tuning().SuspectDays {
			topic(crew, fact{theme.Bad.Render("skimming suspected"), priCrew})
		} else {
			topic(crew)
		}
	}
	if line := m.supplyLine(); line != "" {
		topic(fact{line, priSupply})
	}
	if line := m.debtLine(); line != "" {
		topic(fact{line, priDebt})
	}
	if line := m.runsLine(); line != "" {
		topic(fact{theme.CrewText.Render(line), priRuns})
	}
	if line := m.contractsLine(); line != "" {
		topic(fact{line, priContracts})
	}
	topic(fact{theme.Subtle.Render(m.ownedLine()), priUpgrades})
	if m.talking() {
		topic(fact{theme.Bad.Render("Somebody is talking. Investigate " + screenPointer(screenCrew) + "."), priTalking})
	}
	if w.Heat.SellCapDays > 0 {
		topic(fact{theme.Bad.Render(fmt.Sprintf("Patrols: sales capped at %.0f%% of demand for %s more.", w.Heat.SellCap*100, plural(w.Heat.SellCapDays, "day"))), priPatrol})
	}
	if s := w.Strike; s != nil {
		if c := w.Corner(s.Corner); c != nil {
			topic(fact{theme.Warning.Render(fmt.Sprintf("Enforcers go to %s tonight: %s.", c.Name, s.Force)), priStrike})
		}
	}

	var last string
	switch {
	case w.LieLow:
		last = theme.Warning.Render("Lying low today. No sales, heat fades faster.")
	case len(w.Orders) > 0:
		last = theme.Gold.Render("Orders queued for tonight.")
	case len(w.Standing) > 0:
		last = theme.Gold.Render(fmt.Sprintf("Standing orders sell tonight; the crew keep %.0f%%.", m.set.Market.Cut()*100))
	default:
		if lt := w.Crew.Lieutenant(here.ID); lt != nil && m.standingHere() > 0 {
			last = theme.Gold.Render(fmt.Sprintf("%s sells the stash here at %s; an order of yours overrides it.", lt.Name, m.set.Crew.Dial(*lt)))
		} else {
			last = tutorialLine()
		}
	}

	room := max(0, maxLines-len(lines)-1)
	var packed []string
	if !narrow {
		for _, t := range topics {
			packed = append(packed, pack(t, innerW)...)
		}
	}
	if narrow || len(packed) > room {
		// Out of room, or narrow: every fact packed as tight as the
		// width allows, whatever its topic, and the facts that matter
		// least dropped until the rest fit.
		var facts []fact
		for _, t := range topics {
			facts = append(facts, t...)
		}
		for packed = pack(facts, innerW); len(packed) > room && len(facts) > 0; packed = pack(facts, innerW) {
			least := 0
			for i, f := range facts {
				if f.pri < facts[least].pri {
					least = i
				}
			}
			facts = append(facts[:least], facts[least+1:]...)
		}
	}
	lines = append(lines, packed...)
	return append(lines, last)
}

// cornersLine is the street's corner count in the map's vocabulary:
// `corners 1 worked, 1 held of 10, 1 theirs`, and what is worked in
// the other city.
func (m *Model) cornersLine(city string) string {
	w := m.w
	held := 0
	for _, c := range w.City(city).Corners {
		if c.Held() {
			held++
		}
	}
	line := fmt.Sprintf("corners %d worked, %d held of %d", w.WorkedIn(city), held, len(w.City(city).Corners))
	if n := w.RivalHeld(); n > 0 && w.City(city) == w.Home() {
		line += fmt.Sprintf(", %d theirs", n)
	}
	if n := w.Worked() - w.WorkedIn(city); n > 0 {
		line += fmt.Sprintf(", %d worked elsewhere", n)
	}
	return line
}

// elsewhereFacts are what you hold outside the city you are in and
// what is on the road, as street facts, for a layout without the
// CITIES panel.
func (m *Model) elsewhereFacts() []fact {
	w := m.w
	var facts []fact
	for _, cid := range w.CityOrder {
		if cid == w.Player.Location {
			continue
		}
		if n := w.StockIn(cid); n > 0 {
			facts = append(facts, fact{theme.RoadText.Render(fmt.Sprintf("%s in %s", plural(n, "unit"), w.CityName(cid))), priRoad})
		}
	}
	if road, soonest := m.roadUnits(""); road > 0 {
		facts = append(facts, fact{theme.RoadText.Render(fmt.Sprintf("%s on the road, next in %dd", plural(road, "unit"), soonest)), priRoad})
	}
	return facts
}

// otherHeat is the other cities' heat, `Bayport heat 1`, joined, or "".
func (m *Model) otherHeat() string {
	w := m.w
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c != w.Here() {
			elsewhere = append(elsewhere, theme.Subtle.Render(c.Name+" heat ")+heatStyle(c.Heat).Render(fmt.Sprintf("%.0f", c.Heat)))
		}
	}
	return strings.Join(elsewhere, sep)
}

// heatLines is the HEAT panel's content for a panel innerW cells wide:
// the gauge with the thresholds marked, the numbers, the thresholds on
// one line where they fit and two a line where they do not, and, wide,
// the three reputation bars on one line (narrow has four lines with
// the thresholds and leaves the bars to the wide layout).
func (m *Model) heatLines(innerW int, narrow bool) []string {
	w := m.w
	here := w.Here()
	var marks []float64
	var thr []string
	for _, r := range m.set.Heat.ThresholdsIn(w, here) {
		marks = append(marks, r.Threshold/100)
		thr = append(thr, fmt.Sprintf("%s %.0f", r.Level, r.Threshold))
	}
	lines := []string{heatStyle(here.Heat).Render(sparkline.Bar(here.Heat/100, innerW, marks))}
	numbers := []string{heatStyle(here.Heat).Render(fmt.Sprintf("%.0f", here.Heat)) + theme.Subtle.Render("/100"), theme.Subtle.Render(fmt.Sprintf("peak %.0f", w.Heat.Peak))}
	if ev := m.set.Heat.EvidenceArrest(w); ev > 0 {
		style := theme.Subtle
		if w.Heat.Evidence >= ev-2 {
			style = theme.Bad
		}
		numbers = append(numbers, style.Render(fmt.Sprintf("file %d/%d", w.Heat.Evidence, ev)))
	}
	if line := strings.Join(numbers, sep); lipgloss.Width(line) <= innerW {
		lines = append(lines, line)
	} else {
		lines = append(lines, strings.Join(numbers, " "))
	}
	if all := strings.Join(thr, " · "); lipgloss.Width(all) <= innerW {
		lines = append(lines, theme.Subtle.Render(all))
	} else {
		for i := 0; i < len(thr); i += 2 {
			lines = append(lines, theme.Subtle.Render(strings.Join(thr[i:min(i+2, len(thr))], " · ")))
		}
	}
	if !narrow {
		lines = append(lines, m.reputationLine(innerW))
	}
	return lines
}

// cashLines is the CASH panel's content, four lines: the dirty pile
// with the warning on its row once it is past what the fronts cover,
// the clean cash with the day's wash, the peak, and the other city's
// heat (the layout keeps HEAT to four lines by putting it here).
// Narrow has the pools alone, the wash as a total, and the warning as
// the last line, where the other city's heat goes without it.
func (m *Model) cashLines(innerW int, narrow bool) []string {
	w := m.w
	over := ""
	if line := m.set.Heat.DirtyCashThreshold(w); line > 0 {
		if line += m.set.Heat.Cover(w); w.Player.DirtyCash > line {
			over = theme.Warning.Render(fmt.Sprintf("over %s: heat", cash(line)))
		}
	}
	dirty := theme.Gold.Render("dirty  " + cash(w.Player.DirtyCash))
	clean := theme.Subtle.Render("clean  " + cash(w.Player.CleanCash))
	if len(w.Fronts) > 0 {
		wash := cash(m.set.Laundering.Capacity(w))
		clean = firstFit(innerW, clean+theme.Subtle.Render(fmt.Sprintf("  +%s/day %s", wash, w.Laundering.Dial)), clean+theme.Subtle.Render(" +"+wash))
	}
	peak := theme.Subtle.Render("peak   " + cash(w.Stats.PeakCash))
	last := m.otherHeat()
	// The warning rides the dirty row where the row has the room, and
	// is the last line where it does not (narrow, or a middling width).
	if over != "" && !narrow && lipgloss.Width(dirty)+2+lipgloss.Width(over) <= innerW {
		dirty = fit(dirty, lipgloss.Width(dirty)+2) + over
		over = ""
	}
	if over != "" {
		last = over
	}
	return []string{dirty, clean, peak, last}
}

// bar is a labelled gauge: `war ████░░░░ 58/80 loud`.
func bar(label string, frac float64, after string) string {
	return label + " " + sparkline.Bar(frac, dashBarW, nil) + " " + after
}

// lawLines is the LAW panel's content, four lines: the chief and what
// they are like once you have seen them work with the days they have
// left, the DA and their ticket with the days to the election, the
// pressure where you are with what you have bought the city, and the
// other city's pressure. Narrow drops the days, shortens the ticket
// and folds the rival's line into the last row.
func (m *Model) lawLines(innerW int, narrow bool) []string {
	w := m.w
	l := w.Law
	here := w.Here()
	chief := "Chief " + l.Chief.Name + sep
	if l.Chief.Observed {
		chief += theme.Subtle.Render(l.Chief.Personality)
	} else {
		chief += theme.Subtle.Render("new")
	}
	if end := m.set.Law.ChiefTermEnds(w); end > 0 && !narrow {
		chief = firstFit(innerW, chief+sep+theme.Subtle.Render(fmt.Sprintf("%dd left", max(0, end-w.Day))), chief)
	}
	// The ticket is spelt out where the line has the room for it and
	// the election, and law-order where it does not; the election goes
	// before the ticket does.
	da := "DA " + l.DA.Name + sep
	long, short := stanceWord(l.DA.Stance), stanceWord(l.DA.Stance)
	if l.DA.Stance == "law_and_order" {
		short = "law-order"
	}
	election, tight := "", ""
	if next := m.set.Law.NextElection(w); next > 0 && !narrow {
		election = sep + theme.Subtle.Render(fmt.Sprintf("election in %dd", max(0, next-w.Day)))
		tight = sep + theme.Subtle.Render(fmt.Sprintf("election %dd", max(0, next-w.Day)))
	}
	da = firstFit(innerW,
		da+theme.Subtle.Render(long)+election,
		da+theme.Subtle.Render(short)+election,
		da+theme.Subtle.Render(short)+tight,
		da+theme.Subtle.Render(long),
		da+theme.Subtle.Render(short))
	// The pressure bar, in heat's red, with the goodwill after it: what
	// you have bought the city is always shown once you have bought
	// some, the bar giving way to the number where the room is short.
	pressure := theme.Bad.Render(bar("pressure", here.Pressure/100, fmt.Sprintf("%.0f", here.Pressure)))
	switch goodwill := theme.Good.Render(fmt.Sprintf("goodwill %.0f", here.Goodwill)); {
	case here.Goodwill > 0:
		pressure = firstFit(innerW, pressure+sep+goodwill, theme.Bad.Render(fmt.Sprintf("pressure %.0f", here.Pressure))+sep+goodwill, pressure)
	case !narrow:
		pressure = firstFit(innerW, pressure+sep+theme.Subtle.Render("goodwill 0"), pressure)
	}
	last := ""
	if narrow {
		last = m.rivalShort(innerW)
	} else {
		var others []string
		for _, cid := range w.CityOrder {
			if c := w.Cities[cid]; c != here {
				others = append(others, theme.Subtle.Render(fmt.Sprintf("%s pressure %.0f", c.Name, c.Pressure)))
			}
		}
		last = strings.Join(others, sep)
	}
	return []string{chief, da, pressure, last}
}

// rivalShort is the rival in one line for the narrow layout's LAW
// panel: who they are and what they hold.
func (m *Model) rivalShort(innerW int) string {
	w := m.w
	if w.Rival.Arrived == 0 {
		return theme.Subtle.Render("no rival yet")
	}
	leader := theme.RivalText.Render(w.Rival.Leader)
	short := leader + sep + theme.Subtle.Render(plural(w.RivalHeld(), "corner"))
	if eye := m.eyeingWord(); eye != "" {
		return firstFit(innerW, short+sep+eye, leader+sep+eye, eye, short)
	}
	return short
}

// rivalLines is the RIVALS panel's content, four lines: who they are,
// what they hold and what they are like, the war as a bar against the
// line the police crack down at, the trust as a bar, and whatever
// holds or waits at the table.
func (m *Model) rivalLines(innerW int) []string {
	w := m.w
	r := w.Rival
	tun := m.set.Rivals.Tuning()
	if r.Arrived == 0 {
		var ls []string
		for _, l := range wrap("Nobody is contesting the city. Yet.", innerW) {
			ls = append(ls, theme.Subtle.Render(l))
		}
		return ls
	}
	leader := theme.RivalText.Render(r.Leader)
	who := leader + sep + theme.Subtle.Render(plural(w.RivalHeld(), "corner"))
	temper := who + sep + theme.Subtle.Render(m.personalityWord())
	if eye := m.eyeingWord(); eye != "" {
		// The tell (#69) outranks the temper, the count and the name
		// where the line has room for one of them: it needs you.
		who = firstFit(innerW, temper+sep+eye, who+sep+eye, leader+sep+eye, eye, temper, who)
	} else {
		who = firstFit(innerW, temper, who)
	}
	war := bar("war", r.War/tun.CrackdownThreshold, fmt.Sprintf("%.0f/%.0f", r.War, tun.CrackdownThreshold))
	switch {
	case r.War >= tun.WarThreshold:
		war = theme.Bad.Render(war + " loud")
	case r.War > 0:
		war = theme.Warning.Render(war)
	default:
		war = theme.Subtle.Render(war)
	}
	trust := theme.Subtle.Render(bar("trust", r.Trust/100, fmt.Sprintf("%.0f", r.Trust)))
	var table string
	switch {
	case len(w.Offers) > 0:
		offers := plural(len(w.Offers), "offer")
		table = theme.Gold.Render(firstFit(innerW, offers+" "+screenPointer(screenRivals), offers+" waits"))
	case len(r.Deals) > 0:
		var ds []string
		for _, d := range r.Deals {
			if d.Until > 0 {
				ds = append(ds, fmt.Sprintf("%s %dd", d.Kind, d.Left(w.Day)))
			} else {
				ds = append(ds, string(d.Kind))
			}
		}
		table = theme.Good.Render(strings.Join(ds, ", "))
	case w.Proposal != nil:
		table = theme.Gold.Render("proposal tonight")
	default:
		table = theme.Subtle.Render("Nothing on the table.")
	}
	return []string{who, war, trust, table}
}

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
	for _, r := range m.set.Heat.ThresholdsIn(w, here) {
		if r.Level == "patrol" && here.Heat >= r.Threshold {
			out = append(out, newAlert(theme.Bad.Render(fmt.Sprintf("Heat %.0f in %s is over the patrol line (%.0f).", here.Heat, here.Name, r.Threshold)), "heat in "+here.Name+" over the patrol line"))
		}
	}
	if fl := m.set.Laundering.Float(w); w.Player.DirtyCash < fl && m.floatMatters() {
		out = append(out, newAlert(theme.Warning.Render(fmt.Sprintf("Dirty cash %s is under the float (%s): the wash and the road wait.", cash(w.Player.DirtyCash), cash(fl))), "dirty cash under the float"))
	}
	if wages := m.set.Crew.Wages(w, w.Crew.Pay); wages > w.Player.DirtyCash {
		out = append(out, newAlert(theme.Warning.Render(fmt.Sprintf("Wages %s due tonight, %s dirty in hand.", money(wages), money(w.Player.DirtyCash))), "wages short"))
	}
	out = append(out, m.unlockAlerts()...)
	out = append(out, m.houseAlerts()...)
	return out
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
		out = append(out, theme.Subtle.Render("Nobody is looking at you. Yet."))
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

// viewDashboard is the overview, in the layout the width calls for.
func (m *Model) viewDashboard() string {
	if m.width >= paneMinWidth {
		return m.dashboardWide(m.mainWidth(), m.mainHeight())
	}
	return m.dashboardNarrow(m.mainWidth(), m.mainHeight())
}

// dashboardNarrow is the layout under paneMinWidth columns: STREET
// over HEAT | CASH | LAW at 28 | 23 | 29 of 80 (the law has the two
// columns the heat's four lines can spare, for the goodwill), the
// rival's line in LAW and the other city in the street, and what the
// street leaves going to ALERTS.
func (m *Model) dashboardNarrow(width, h int) string {
	here := m.w.Here()
	heatW := width * 28 / 80
	cashW := width * 23 / 80
	lawW := width - heatW - cashW
	street := m.streetLines(width-4, h-dashPanelH-2, true, true)
	streetH := min(len(street)+2, h-dashPanelH)
	alerts := m.alertsPanel(width, h-dashPanelH-streetH)
	if alerts == "" {
		streetH = h - dashPanelH
	}
	rows := []string{
		panel("STREET · "+here.Name, strings.Join(street, "\n"), width, streetH, theme.Market),
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("HEAT", strings.Join(m.heatLines(heatW-4, true), "\n"), heatW, dashPanelH, theme.Heat),
			panel("CASH", strings.Join(m.cashLines(cashW-4, true), "\n"), cashW, dashPanelH, theme.Money),
			panel("LAW", strings.Join(m.lawLines(lawW-4, true), "\n"), lawW, dashPanelH, theme.Heat)),
	}
	if alerts != "" {
		rows = append(rows, alerts)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// dashboardWide is the layout for the pane's regime: STREET the width,
// sized to its lines up to streetMaxH; HEAT | CASH at 46 | 38 of 84;
// LAW | RIVALS the same; CITIES, one row a city, where the height
// leaves the street its lines (where it does not, the street carries
// the road in a line); and what is left to ALERTS where no pane sits
// beside MAIN.
func (m *Model) dashboardWide(width, h int) string {
	w := m.w
	here := w.Here()
	leftW := width * 46 / 84
	rightW := width - leftW
	heat := m.heatLines(leftW-4, false)
	rowH := max(len(heat), 4) + 2
	room := h - rowH - dashPanelH
	citiesH := 0
	street := m.streetLines(width-4, streetMaxH-2, false, false)
	if n := len(w.CityOrder); n > 1 && room-(n+2) >= min(len(street)+2, streetMaxH) {
		citiesH = n + 2
	} else {
		street = m.streetLines(width-4, streetMaxH-2, false, true)
	}
	streetH := min(len(street)+2, streetMaxH, room-citiesH)
	rows := []string{
		panel("STREET · "+here.Name, strings.Join(street, "\n"), width, streetH, theme.Market),
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("HEAT", strings.Join(heat, "\n"), leftW, rowH, theme.Heat),
			panel("CASH", strings.Join(m.cashLines(rightW-4, false), "\n"), rightW, rowH, theme.Money)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("LAW", strings.Join(m.lawLines(leftW-4, false), "\n"), leftW, dashPanelH, theme.Heat),
			panel("RIVALS", strings.Join(m.rivalLines(rightW-4), "\n"), rightW, dashPanelH, theme.Rivals)),
	}
	if citiesH > 0 {
		rows = append(rows, panel("CITIES", strings.Join(m.citiesLines(width-4), "\n"), width, citiesH, theme.Logistics))
	}
	if alerts := m.alertsPanel(width, room-citiesH-streetH); alerts != "" {
		rows = append(rows, alerts)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// sellEstimate is what s would sell of a product here at a dial, and
// what it would bring: the sell dialog's expect row for the whole stash.
func (m *Model) sellEstimate(city, id string, dial events.Dial) (units int, take int) {
	w := m.w
	p := w.Product(city, id)
	if p == nil {
		return 0, 0
	}
	units = min(w.Stock(city, id), m.set.Market.Capacity(w, city, id, dial))
	return units, int(float64(units) * p.Price * m.set.Market.Dial(dial).Price)
}

// dashboardDetails is the dashboard's pane: the product under the
// cursor with what s would do to it, the alerts, and the keys.
func (m *Model) dashboardDetails() []section {
	w := m.w
	here := w.Here()
	secs := m.cartSection(here.ID) // the day's cart first, so the strip carries its totals (#103)
	if m.cursor < len(w.Products) {
		id := w.Products[m.cursor]
		if p := here.Market[id]; p != nil {
			f := facts(p)
			lines := []string{row("price", price(p.Price)+"  "+f.deltaText())}
			if p.NoSupply {
				lines = append(lines, row("supplier", theme.Subtle.Render("not sold here")))
			} else {
				lines = append(lines, row("supplier", price(p.SupplierPrice)+sep+"margin "+f.marginText()))
			}
			stock := []string{fmt.Sprintf("%d here", w.Stock(here.ID, id))}
			for _, cid := range w.CityOrder {
				if q := w.Stock(cid, id); q > 0 && cid != here.ID {
					stock = append(stock, fmt.Sprintf("%d in %s", q, w.CityName(cid)))
				}
			}
			// One row where the cities fit on it, one a city where not.
			if one := strings.Join(stock, sep); lipgloss.Width(one) <= paneTextW-paneLabelW-1 {
				lines = append(lines, row("stock", one))
			} else {
				label := "stock"
				for _, st := range stock {
					lines = append(lines, row(label, st))
					label = ""
				}
			}
			if road, soonest := m.roadUnits(id); road > 0 {
				lines = append(lines, row("road", theme.RoadText.Render(fmt.Sprintf("%s, %dd", plural(road, "unit"), soonest))))
			}
			lines = append(lines, row("demand", fmt.Sprintf("~%.0f/day on %s", w.Demand(here.ID, id), plural(w.WorkedIn(here.ID), "corner"))))
			dial := events.DialNormal
			switch o, ok := w.Order(here.ID, id); {
			case ok:
				dial = o.Dial
				lines = append(lines, row("order", theme.Gold.Render(fmt.Sprintf("%d %s", o.Qty, o.Dial))))
			default:
				if so, ok := w.YourStanding(here.ID, id); ok {
					dial = so.Dial
				} else if so, ok := w.DelegatedOrder(here.ID, id); ok {
					lines = append(lines, row("order", theme.CrewText.Render(fmt.Sprintf("%d %s (lt)", so.Qty, so.Dial))))
				} else {
					lines = append(lines, row("order", theme.Subtle.Render("none")))
				}
			}
			switch units, take := m.sellEstimate(here.ID, id, dial); {
			case w.LieLow:
				lines = append(lines, keyRow("s", theme.Subtle.Render("nothing sells today: lying low")))
			case w.Stock(here.ID, id) == 0:
				lines = append(lines, keyRow("s", theme.Subtle.Render("nothing stashed here to sell")))
			default:
				lines = append(lines, keyRow("s", fmt.Sprintf("sell ~%d at %s = ~%s", units, dialShort(dial), theme.Gold.Render(money(take)))))
			}
			if o, ok := w.Order(here.ID, id); ok {
				lines = append(lines, keyRow("x", fmt.Sprintf("cancel the %d %s", o.Qty, o.Dial)))
			}
			lines = append(lines, m.standingRows(here.ID, id)...)
			lines = append(lines, m.contractRows(here.ID, id)...)
			secs = append(secs, section{strings.ToUpper(p.Name) + " · " + strings.ToUpper(here.Name), lines})
		}
	}
	secs = append(secs, section{"ALERTS", m.alertLines(paneTextW, 8)})
	return secs
}

// supplyLine is the dashboard's one line on the supply contracts
// (#113): how many stand and what they bought this morning (`supply 2
// contracts · $3,400 this morning`). Empty with none.
func (m *Model) supplyLine() string {
	n := len(m.w.Supply)
	if n == 0 {
		return ""
	}
	line := theme.Gold.Render("supply " + plural(n, "contract"))
	if _, cost := m.w.SuppliedToday(); cost > 0 {
		line += sep + money(cost) + " this morning"
	}
	return line
}

// citiesLines is the CITIES panel: one line per city with the value of
// the stash there, the corners held, who runs it (you where you stand,
// the lieutenant where one does), its heat and what is on the road to
// it, the columns aligned, in width cells.
func (m *Model) citiesLines(width int) []string {
	w := m.w
	type cityRow struct {
		cells []string
		style []lipgloss.Style
	}
	var rows []cityRow
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		name, nameStyle := "  "+c.Name, theme.Subtle
		if cid == w.Player.Location {
			name, nameStyle = "◉ "+c.Name, theme.Gold
		}
		value := 0
		for id, q := range w.StashOf(cid) {
			if p := c.Market[id]; p != nil {
				value += int(float64(q) * p.SupplierPrice)
			}
		}
		held := 0
		for _, k := range c.Corners {
			if k.Held() {
				held++
			}
		}
		runs, runsStyle := "nobody", theme.Subtle
		switch lt := w.Crew.Lieutenant(cid); {
		case lt != nil:
			runs, runsStyle = lt.Name, theme.CrewText
		case cid == w.Player.Location:
			runs, runsStyle = "you", theme.Body
		}
		road, soonest := 0, 0
		for _, sh := range w.Shipments {
			if sh.To != cid {
				continue
			}
			road += sh.Units
			if d := sh.DaysLeft(w.Day); soonest == 0 || d < soonest {
				soonest = d
			}
		}
		onRoad := ""
		if road > 0 {
			onRoad = fmt.Sprintf("◂ %s in %dd", plural(road, "unit"), soonest)
		}
		rows = append(rows, cityRow{
			cells: []string{name, "stash " + cash(value), fmt.Sprintf("%d/%d corners", held, len(c.Corners)), runs, fmt.Sprintf("heat %.0f", c.Heat), onRoad},
			style: []lipgloss.Style{nameStyle, theme.Subtle, theme.Subtle, runsStyle, heatStyle(c.Heat), theme.RoadText},
		})
	}
	var widths []int
	for _, r := range rows {
		for i, c := range r.cells {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	var out []string
	for _, r := range rows {
		var parts []string
		for i, c := range r.cells {
			parts = append(parts, r.style[i].Render(fit(c, widths[i])))
		}
		out = append(out, truncate(strings.TrimRight(strings.Join(parts, "  "), " "), width))
	}
	return out
}

// reputationAxes are the dashboard's three bars: the axis, its label at
// full and at narrow width, and its accent.
var reputationAxes = []struct {
	axis, long, short string
	colour            lipgloss.Color
}{
	{"fear", "fear", "F", theme.Rivals},
	{"respect", "respect", "R", theme.Crew},
	{"notoriety", "notoriety", "N", theme.Warn},
}

// reputationLine is the three reputation bars under the heat gauge, side
// by side so they fit in one line of a panel width cells wide.
func (m *Model) reputationLine(width int) string {
	rep := m.w.Player.Reputation
	labels := len("fear ") + len("respect ") + len("notoriety ")
	long := width >= labels+2+3*3 // the words, with bars of three
	if !long {
		labels = 3 * 2 // "F "
	}
	barW := max(3, (width-labels-2)/3)
	var parts []string
	for _, a := range reputationAxes {
		label := a.short
		if long {
			label = a.long
		}
		v := *rep.Axis(a.axis)
		parts = append(parts, theme.Fg(a.colour).Render(label+" "+sparkline.Bar(v/100, barW, nil)))
	}
	return strings.Join(parts, " ")
}

// roadUnits is how many units of a product (every product for "") are
// on the road, and the soonest any of them land.
func (m *Model) roadUnits(id string) (units, soonest int) {
	for _, sh := range m.w.Shipments {
		if id != "" && sh.Product != id {
			continue
		}
		units += sh.Units
		if d := sh.DaysLeft(m.w.Day); soonest == 0 || d < soonest {
			soonest = d
		}
	}
	return units, soonest
}

// supplierLine is the street's fact on the supply side (#72): the best
// available connect where you stand and their price as a share of
// street, `Cass sells at ~55% of street`, or that nobody is selling to
// you today.
func (m *Model) supplierLine() string {
	w := m.w
	here := w.Player.Location
	var best *game.Supplier
	ratio := 0.0
	for _, sup := range w.SuppliersIn(here) {
		for _, id := range w.Products {
			p := w.Product(here, id)
			if p == nil || p.Price <= 0 || !w.Available(sup, id) {
				continue
			}
			if r := sup.Price[id] / p.Price; best == nil || r < ratio {
				best, ratio = sup, r
			}
		}
	}
	if best == nil {
		if len(w.SuppliersIn(here)) == 0 {
			return fmt.Sprintf("supplier at ~%.0f%% of street", m.set.Market.BaseRatio(w)*100)
		}
		return "nobody is selling to you today"
	}
	return fmt.Sprintf("%s sells at ~%.0f%% of street", best.Name, ratio*100)
}
