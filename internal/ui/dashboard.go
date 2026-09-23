package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
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
	sel := m.cursor
	if m.onPolice {
		sel = -1 // the arrows are on the police (#355): no product is picked
	}
	cols, rows, cursor := m.productRows(here.ID, sel, false)
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
	if w.Reign > 0 {
		// The city is yours (#227): on the corners' fact, the one the
		// panel never drops, since the tier's is the first to go.
		corners.s += theme.Gold.Render(fmt.Sprintf(" · reign d%d", w.ReignDay()))
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
		crew := fact{theme.CrewText.Render(fmt.Sprintf("crew %d · %s pay %s/day", n, w.Crew.Pay, money(m.rules.Crew.Wages(w, w.Crew.Pay)))), priCrew}
		if w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < m.rules.Crew.Tuning().SuspectDays {
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
		topic(fact{theme.Bad.Render(fmt.Sprintf("Patrols: sales capped at %s of demand for %s more.", format.Pct(w.Heat.SellCap, 0), plural(w.Heat.SellCapDays, "day"))), priPatrol})
	}
	if s := w.Today.Strike; s != nil {
		if c := w.Corner(s.Corner); c != nil {
			topic(fact{theme.Warning.Render(fmt.Sprintf("Enforcers go to %s tonight: %s.", c.Name, s.Force)), priStrike})
		}
	} else if r := w.Faction(w.War); w.War != "" && r != nil {
		// The war order (#229): where the enforcers go tonight on it.
		line := fmt.Sprintf("War on %s: nowhere to go tonight.", m.rivalName(r))
		if c := m.rules.Rivals.WarTarget(w, r); c != nil {
			force, _ := m.cfg.Rivals.War.Force()
			line = fmt.Sprintf("War on %s: enforcers go to %s tonight, %s.", m.rivalName(r), c.Name, force)
		}
		topic(fact{theme.Warning.Render(line), priStrike})
	}

	var last string
	switch {
	case w.Today.LieLow:
		last = theme.Warning.Render("Lying low today. No sales, heat fades faster.")
	case len(w.Today.Orders) > 0:
		last = theme.Gold.Render("Orders queued for tonight.")
	case len(w.Standing) > 0:
		last = theme.Gold.Render("Standing orders sell tonight; the crew keep " + format.Pct(m.rules.Market.Cut(), 0) + ".")
	default:
		if lt := w.Crew.Lieutenant(here.ID); lt != nil && m.standingHere() > 0 {
			last = theme.Gold.Render(fmt.Sprintf("%s sells the stash here at %s; an order of yours overrides it.", lt.Name, m.rules.Crew.Dial(*lt)))
		} else if line := m.retireLine(); line != "" {
			last = line // how far off the exit is (#195)
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
			panel(m.policeTitle("HEAT"), strings.Join(m.heatLines(heatW-4, true), "\n"), heatW, dashPanelH, theme.Heat),
			panel(m.policeTitle("CASH"), strings.Join(m.cashLines(cashW-4, true), "\n"), cashW, dashPanelH, theme.Money),
			panel(m.policeTitle("LAW"), strings.Join(m.lawLines(lawW-4, true), "\n"), lawW, dashPanelH, theme.Heat)),
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
			panel(m.policeTitle("HEAT"), strings.Join(heat, "\n"), leftW, rowH, theme.Heat),
			panel(m.policeTitle("CASH"), strings.Join(m.cashLines(rightW-4, false), "\n"), rightW, rowH, theme.Money)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel(m.policeTitle("LAW"), strings.Join(m.lawLines(leftW-4, false), "\n"), leftW, dashPanelH, theme.Heat),
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
	units = min(w.Stock(city, id), m.rules.Market.Capacity(w, city, id, dial))
	return units, int(float64(units) * p.Price * m.rules.Market.Dial(dial).Price)
}

// dashboardDetails is the dashboard's pane: the product under the
// cursor with what s would do to it, or POLICE with the arrows past the
// table (#355), the alerts, and the keys.
func (m *Model) dashboardDetails() []section {
	w := m.w
	here := w.Here()
	if m.onPolice {
		// Past the product table (#355): the police where you stand,
		// first even over the cart, since the arrows asked for it.
		return append(append([]section{m.policeSection(here)}, m.cartSection(here.ID)...), m.dashboardAfter()...)
	}
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
			case w.Today.LieLow:
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
	return append(secs, m.dashboardAfter()...)
}

// dashboardAfter is the dashboard pane past its selection: the alerts
// and what your name buys (#233), after what needs you.
func (m *Model) dashboardAfter() []section {
	return append([]section{{"ALERTS", m.alertLines(paneTextW, 8)}}, m.nameSection()...)
}
