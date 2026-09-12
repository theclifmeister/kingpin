package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// frontRows lists the fronts on offer that the player does not own yet,
// cheapest first, locked ones included so the ladder is visible.
func (m *Model) frontRows() []game.FrontOffer {
	var rows []game.FrontOffer
	for _, o := range m.set.Laundering.Offers() {
		if m.w.Front(o.ID) == nil {
			rows = append(rows, o)
		}
	}
	return rows
}

// askFront opens the buy picker on its first page, the kind (#73: a
// front or a house).
func (m *Model) askFront() {
	if m.w.Over != nil {
		return
	}
	if len(m.frontRows()) == 0 && len(m.houseRows()) == 0 {
		m.refuse("Nothing to buy: you own every front and every house there is.")
		return
	}
	m.frontCursor, m.frontStep = 0, 0
	m.mode = modeFront
}

// confirmFront buys the offer under the cursor: a front, or a house on
// the picker's house page.
func (m *Model) confirmFront() {
	if m.frontKind == pickHouse {
		m.confirmHouse()
		return
	}
	rows := m.frontRows()
	m.mode = modePlay
	if len(rows) == 0 {
		return
	}
	o := rows[max(0, min(m.frontCursor, len(rows)-1))]
	f, err := m.w.BuyFront(o)
	if err != nil {
		m.refuse("Can't buy: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Bought %s for %s. It opens tomorrow, washing up to %s/day.", f.Name, money(o.Cost), money(m.set.Laundering.Throughput(m.w, f))))
}

func (m *Model) cycleLaunder() {
	d := (m.w.Laundering.Dial + 1) % 3
	m.w.SetLaunderDial(d)
	if len(m.w.Fronts) == 0 {
		m.say(fmt.Sprintf("Launder dial %s. %s Buy a front %s to use it.", d, launderBlurb(d), screenPointer(screenLedger)))
		return
	}
	m.say(fmt.Sprintf("Launder dial %s: washing up to %s/day, audit risk %.1f%%/day. %s", d, money(m.set.Laundering.Capacity(m.w)), m.set.Laundering.AnyAuditRisk(m.w)*100, launderBlurb(d)))
}

func launderBlurb(d events.Launder) string {
	switch d {
	case events.LaunderCareful:
		return "Slow and quiet."
	case events.LaunderGreedy:
		return "Fast, and an audit is evidence."
	default:
		return "Steady."
	}
}

// frontStatus is a front's state for the ledger's status column, in
// the one lowercase vocabulary: open, opens tomorrow, audit, back in
// 14d, shut, back in 2d.
func (m *Model) frontStatus(f game.Front) any {
	switch {
	case f.Frozen(m.w.Day+1) && f.Audited > 0 && f.FrozenUntil == f.Audited+m.set.Laundering.Tuning().AuditFreezeDays:
		return styled{theme.Bad, fmt.Sprintf("audit, back in %dd", f.FrozenUntil-m.w.Day)}
	case f.Frozen(m.w.Day + 1):
		return styled{theme.Warning, fmt.Sprintf("shut, back in %dd", f.FrozenUntil-m.w.Day)}
	case f.Bought == m.w.Day:
		return styled{theme.Subtle, "opens tomorrow"}
	default:
		return styled{theme.Good, "open"}
	}
}

// offerCols and offerRows are the fronts on offer, on the ledger and in
// the picker: what one costs, washes and keeps, its audit risk, and
// whether it is open to you, locked until you have moved enough, or
// short of what is in the till.
var offerCols = []col{{"front", kText, 0}, {"cost", kMoney, 0}, {"washes/day", kMoney, 0}, {"upkeep/day", kMoney, 0}, {"audit", kPct, 0}, {"status", kText, 0}}

// A locked offer's status is the distance to its line (#148), `locked
// · $18K to go`, or `$18K to go` alone where the table has no room for
// the word (80 columns, or beside the pane).
func (m *Model) offerRows(rows []game.FrontOffer, width int) [][]any {
	out := m.offerRowsWith(rows, true)
	if tableWidth(offerCols, out) > width {
		out = m.offerRowsWith(rows, false)
	}
	return out
}

func (m *Model) offerRowsWith(rows []game.FrontOffer, long bool) [][]any {
	var out [][]any
	for _, o := range rows {
		var status any
		switch {
		case o.Locked(m.w):
			status = styled{theme.Subtle, lockedStatus(m.w, o, long)}
		case o.Cost > m.w.Player.DirtyCash:
			status = styled{theme.Bad, "short " + money(o.Cost-m.w.Player.DirtyCash)}
		default:
			status = "open to you"
		}
		out = append(out, []any{o.Name, o.Cost, o.Throughput, o.Upkeep, o.AuditRisk * 100, status})
	}
	return out
}

func (m *Model) viewFront() string {
	if m.frontStep == 0 {
		return m.viewKind()
	}
	if m.frontKind == pickHouse {
		return m.viewHouses()
	}
	rows := m.frontRows()
	if len(rows) == 0 {
		return m.modal("BUY A FRONT", []string{"Nothing for sale."}, m.modalFooter())
	}
	m.frontCursor = max(0, min(m.frontCursor, len(rows)-1))
	m.modalFollow(1 + m.frontCursor) // under the header
	body := table(offerCols, m.offerRows(rows, m.modalInner()), m.frontCursor, m.modalInner())
	body = append(body, "", theme.Subtle.Render(fmt.Sprintf("Dirty cash %s. It opens tomorrow.", cash(m.w.Player.DirtyCash))))
	return m.modal("BUY A FRONT", body, m.modalFooter())
}

// The ledger (#87) is the till, the fronts, the houses (#73), the road
// and what is on offer, under one cursor: ledgerRows lists every row of
// the four tables in order, ledgerCursor walks them with the arrows, and the
// pane shows the selected front, route or offer. A ledger taller than
// MAIN scrolls with the cursor, its table kept in view, and never
// clamps.

// The kinds of row the ledger's cursor walks, in table order.
const (
	ledgerFront = iota
	ledgerHouse
	ledgerRoute
	ledgerOffer
)

// ledgerRow is one row of the ledger: which table and the index in it.
type ledgerRow struct{ kind, i int }

// ledgerRoutes are every route in the file, in city order: the
// LOGISTICS table's rows.
func (m *Model) ledgerRoutes() []content.RouteConfig {
	var routes []content.RouteConfig
	for _, cid := range m.w.CityOrder {
		for _, r := range m.set.Logistics.Routes(cid) {
			if r.From == cid {
				routes = append(routes, r)
			}
		}
	}
	return routes
}

// ledgerRows are the rows the cursor walks: the fronts, the houses
// (#73), the routes, then the offers.
func (m *Model) ledgerRows() []ledgerRow {
	var rows []ledgerRow
	for i := range m.w.Fronts {
		rows = append(rows, ledgerRow{ledgerFront, i})
	}
	for i := range m.w.Houses {
		rows = append(rows, ledgerRow{ledgerHouse, i})
	}
	for i := range m.ledgerRoutes() {
		rows = append(rows, ledgerRow{ledgerRoute, i})
	}
	for i := range m.frontRows() {
		rows = append(rows, ledgerRow{ledgerOffer, i})
	}
	return rows
}

// ledgerSelected is the row under the cursor, the cursor clamped to the
// rows there are; kind -1 when the ledger has none.
func (m *Model) ledgerSelected() ledgerRow {
	rows := m.ledgerRows()
	if len(rows) == 0 {
		return ledgerRow{-1, 0}
	}
	m.ledgerCursor = max(0, min(m.ledgerCursor, len(rows)-1))
	return rows[m.ledgerCursor]
}

// ledgerMove is the arrows on the ledger: the cursor down the fronts,
// the routes and the offers as one list.
func (m *Model) ledgerMove(dy int) {
	n := len(m.ledgerRows())
	if dy < 0 && m.ledgerCursor > 0 {
		m.ledgerCursor--
	} else if dy > 0 && m.ledgerCursor < n-1 {
		m.ledgerCursor++
	}
}

// ledgerActable reports whether enter is the ledger's on the selected
// row: it buys an offer or turns a route's dial; on a front it is the
// frame's, and asks to end the day as it does everywhere.
func ledgerActable(m *Model) bool {
	kind := m.ledgerSelected().kind
	return m.screen == screenLedger && kind != ledgerFront && kind != ledgerHouse
}

// ledgerEnter is enter on the ledger: the selected offer goes to the
// buy confirmation (the picker, on that row), the selected route's dial
// turns a notch.
func (m *Model) ledgerEnter() {
	if m.w.Over != nil {
		return
	}
	sel := m.ledgerSelected()
	switch sel.kind {
	case ledgerOffer:
		m.frontKind, m.frontStep, m.frontCursor = pickFront, 1, sel.i
		m.mode = modeFront
	case ledgerRoute:
		m.cycleLedgerRoute(m.ledgerRoutes()[sel.i])
	}
}

// cycleLedgerRoute turns a route's dial a notch, as the map's r does,
// and says what the road does at it.
func (m *Model) cycleLedgerRoute(r content.RouteConfig) {
	d := (m.w.Route(r.ID).Dial + 1) % (events.RouteFast + 1)
	if err := m.w.SetRoute(r.ID, d); err != nil {
		m.refuse("Can't turn the dial: " + err.Error())
		return
	}
	m.sayRouteDial(r, d)
}

// launderRow draws the launder dial as `careful  [normal]  greedy`.
func launderRow(d events.Launder) string {
	var notches []string
	for x := events.LaunderCareful; x <= events.LaunderGreedy; x++ {
		notches = append(notches, x.String())
	}
	return dialCells(notches, int(d-events.LaunderCareful))
}

var frontCols = []col{{"front", kText, 0}, {"washes/day", kMoney, 0}, {"today", kMoney, 0}, {"lifetime", kMoney, 0}, {"audit", kPct, 0}, {"status", kText, 0}}

var routeCols = []col{{"route", kText, 0}, {"mode", kText, 0}, {"dial", kDial, 0}, {"target", kText, 0}, {"on the road", kText, 0}, {"lots/wk", kCash, 0}, {"fares/wk", kCash, 0}, {"lost", kInt, 0}}

// routeRow is a route's LOGISTICS row: its dial, what it keeps where,
// what is on it, and the week's books.
func (m *Model) routeRow(r content.RouteConfig) []any {
	w := m.w
	rs := w.Route(r.ID)
	target, road := m.targetLine(r.ID), m.roadOn(r.ID)
	if target == "" {
		target = "none"
	}
	if road == "" {
		road = "none"
	}
	lots, fares := w.Logistics.RouteSpend(r.ID, w.Day, 7)
	return []any{r.Name, r.Mode, styled{dialStyle(rs.Dial), rs.Dial}, target, road, lots, fares, w.Logistics.Lost[r.ID]}
}

// viewLedger is the ledger's MAIN: the till lines, then FRONTS,
// LOGISTICS and ON OFFER as tables under the one cursor, scrolled so
// the cursor's table stays in view.
func (m *Model) viewLedger() string {
	w := m.w
	l := m.set.Laundering
	width := m.mainWidth()
	sel := m.ledgerSelected()
	var ls []string
	line := func(s string) { ls = append(ls, truncate(s, width)) }
	sub := theme.Subtle.Render

	line(theme.PanelTitle.Render("LEDGER"))
	line(theme.Gold.Render("dirty "+cash(w.Player.DirtyCash)) + sub(" · ") + theme.Good.Render("clean "+cash(w.Player.CleanCash)) + sub(fmt.Sprintf(" · seized %s lifetime", cash(w.Stats.Seized))))
	line(sub("launder  ") + launderRow(w.Laundering.Dial) + sub(fmt.Sprintf("   audit %.1f%%/day · up to %s/day", l.AnyAuditRisk(w)*100, money(l.Capacity(w)))))
	if thr := m.set.Heat.DirtyCashThreshold(w); thr > 0 && w.Player.DirtyCash > thr {
		line(theme.Warning.Render(fmt.Sprintf("▲ Dirty cash over %s draws heat every day it sits there.", cash(thr))))
	}

	// Each table's heading and the line the cursor is on, for the
	// scroll: the cursor's table is kept in view from its heading, and
	// the till lines with the first table.
	top, at, first := -1, -1, -1
	cursorIn := func(kind int) int {
		if sel.kind == kind {
			return sel.i
		}
		return -1
	}
	heading := func(title, note string) {
		if first < 0 {
			first = len(ls)
		}
		line(sectionTitle(title, theme.Money) + sub(note))
	}
	tableLines := func(kind int, cols []col, rows [][]any) {
		c := cursorIn(kind)
		if c >= 0 {
			top, at = len(ls)-1, len(ls)+1+c
		}
		ls = append(ls, table(cols, rows, c, width)...)
	}

	heading("FRONTS", fmt.Sprintf(" · %d owned · washed %s lifetime", len(w.Fronts), cash(w.Stats.Laundered)))
	if len(w.Fronts) == 0 {
		line(emptyState("No fronts yet. A front washes dirty cash clean; press ", "b", " to buy one."))
	} else {
		var rows [][]any
		for _, f := range w.Fronts {
			rows = append(rows, []any{f.Name, l.Throughput(w, f), f.WashedToday, f.Washed, l.AuditRisk(w, f) * 100, m.frontStatus(f)})
		}
		tableLines(ledgerFront, frontCols, rows)
	}

	if len(w.Houses) > 0 {
		housed := 0
		for _, h := range w.Houses {
			housed += h.Units()
		}
		heading("STASH", fmt.Sprintf(" · %s · %s stashed", plural(len(w.Houses), "house"), plural(housed, "unit")))
		var rows [][]any
		for _, h := range w.Houses {
			rows = append(rows, m.houseRow(h))
		}
		cols := append([]col(nil), houseCols...)
		// Where MAIN is too narrow for the row whole the pane's columns
		// go: the guard, then the block.
		for _, drop := range []int{5, 2} {
			if tableWidth(cols, rows) <= width {
				break
			}
			cols = append(cols[:drop:drop], cols[drop+1:]...)
			for i := range rows {
				rows[i] = append(rows[i][:drop:drop], rows[i][drop+1:]...)
			}
		}
		tableLines(ledgerHouse, cols, rows)
	}

	routes := m.ledgerRoutes()
	if len(routes) > 0 {
		lost := 0
		for _, n := range w.Logistics.Lost {
			lost += n
		}
		heading("LOGISTICS", fmt.Sprintf(" · shipped %s in %s · seized %d", plural(w.Stats.Shipped, "unit"), plural(w.Stats.Shipments, "run"), lost))
		var rows [][]any
		for _, r := range routes {
			rows = append(rows, m.routeRow(r))
		}
		cols := append([]col(nil), routeCols...)
		// Where MAIN is too narrow for the targets to read whole (64
		// columns beside the pane at 100, with a route keeping two
		// products), the columns the pane carries go first: what the
		// route has lost, then its mode.
		for _, drop := range []int{7, 1} {
			if tableWidth(cols, rows) <= width {
				break
			}
			cols = append(cols[:drop:drop], cols[drop+1:]...)
			for i := range rows {
				rows[i] = append(rows[i][:drop:drop], rows[i][drop+1:]...)
			}
		}
		tableLines(ledgerRoute, cols, rows)
	}

	offers := m.frontRows()
	heading("ON OFFER", "")
	if len(offers) == 0 {
		line(sub("You own every front there is."))
	} else {
		tableLines(ledgerOffer, offerCols, m.offerRows(offers, width))
	}

	if top == first {
		top = 0
	}
	return strings.Join(m.ledgerWindow(ls, top, at), "\n")
}

// ledgerWindow is the slice of the ledger's lines MAIN shows: the
// scroll follows the cursor, keeping its row and, where they fit, the
// lines from top (its table's heading, or the title for the first
// table) in view; a ledger that fits scrolls nowhere.
func (m *Model) ledgerWindow(ls []string, top, at int) []string {
	h := m.mainHeight()
	if len(ls) <= h {
		m.ledgerScroll = 0
		return ls
	}
	s := max(0, min(m.ledgerScroll, len(ls)-h))
	if at >= 0 {
		if at >= s+h {
			s = at - h + 1
		}
		if at < s {
			s = at
		}
		if top >= 0 && at-top < h {
			s = min(s, top)
		}
		s = max(0, min(s, len(ls)-h))
	}
	m.ledgerScroll = s
	return ls[s : s+h]
}

// emptyState is an empty state that names its key the legend's way, as
// the tutorial line does: `No deals. Press d to propose one.`
func emptyState(before, key, after string) string {
	return theme.Subtle.Render(before) + theme.Key.Render(key) + theme.Subtle.Render(after)
}

// ledgerDetails is the ledger's pane: the selected front, house, route
// or offer, then WASH (the float, what the fronts wash and cost, the
// accountants) and the keys.
func (m *Model) ledgerDetails() []section {
	var secs []section
	sel := m.ledgerSelected()
	switch sel.kind {
	case ledgerFront:
		secs = append(secs, m.frontSection(m.w.Fronts[sel.i]))
	case ledgerHouse:
		secs = append(secs, m.houseSection(m.w.Houses[sel.i]))
	case ledgerRoute:
		secs = append(secs, m.ledgerRouteSection(m.ledgerRoutes()[sel.i]))
	case ledgerOffer:
		secs = append(secs, m.offerSection(m.frontRows()[sel.i]))
	}
	return append(secs, m.washSection())
}

// frontSection is a front's detail: its state, what it washes and has
// washed, its audit risk at the dial, its upkeep and the day it was
// bought.
func (m *Model) frontSection(f game.Front) section {
	w := m.w
	l := m.set.Laundering
	status, _ := cellText(kText, 0, m.frontStatus(f))
	st := m.frontStatus(f).(styled).st
	washes := money(l.Throughput(w, f)) + "/day"
	if acct, ok := m.accountantBonus(f); ok {
		washes += fmt.Sprintf(" (+%s accountants)", money(acct))
	}
	lines := []string{
		st.Render(status),
		row("washes", washes),
		row("today", money(f.WashedToday)+" · lifetime "+money(f.Washed)),
		row("audit", fmt.Sprintf("%.1f%%/day at %s", l.AuditRisk(w, f)*100, w.Laundering.Dial)),
	}
	if m.cfg.Laundering.Front(f.ID) != nil {
		lines = append(lines, row("upkeep", money(l.FrontUpkeep(w, f))+"/day clean"))
	}
	lines = append(lines, row("bought", fmt.Sprintf("day %d · %s", f.Bought, money(f.Cost))))
	return section{strings.ToUpper(f.Name), lines}
}

// accountantBonus is what the accountants add to a front's daily wash
// at the dial, when there are any.
func (m *Model) accountantBonus(f game.Front) (int, bool) {
	fc := m.cfg.Laundering.Front(f.ID)
	if fc == nil || m.w.Crew.Role("accountant") == 0 {
		return 0, false
	}
	base := int(float64(fc.Throughput)*m.set.Laundering.Dial(m.w.Laundering.Dial).Mul + 0.5)
	return m.set.Laundering.Throughput(m.w, f) - base, true
}

// ledgerRouteSection is a route's detail on the ledger: the edge, its
// dial and terms, the targets, what is on the road, the week's lots and
// fares, what it has lost, and where the road's money comes from.
func (m *Model) ledgerRouteSection(r content.RouteConfig) section {
	w := m.w
	sec := m.routeSection(r)
	// The map's key rows come off: the ledger's keys are its own.
	lines := sec.lines[:len(sec.lines)-2]
	lots, fares := w.Logistics.RouteSpend(r.ID, w.Day, 7)
	lines = append(lines, m.buysFromRow(r)...)
	lines = append(lines,
		row("this week", fmt.Sprintf("lots %s · fares %s", cash(lots), cash(fares))),
		row("lost", plural(w.Logistics.Lost[r.ID], "unit")+" on the road"))
	lines = append(lines, wrapped(theme.Subtle, fmt.Sprintf("The road spends what is over %s dirty.", cash(m.set.Laundering.Float(w))))...)
	lines = append(lines, keyRow("enter", "turn the dial"))
	return section{sec.title, lines}
}

// buysFromRow names the connect a route buys its lots from (#72): the
// wholesaler at its source, with the lot and where they stand, or that
// nobody there sells by the lot.
func (m *Model) buysFromRow(r content.RouteConfig) []string {
	w := m.w
	sup := w.WholesaleSupplier(r.From)
	if sup == nil {
		return []string{row("buys from", theme.Subtle.Render("nobody: the stash there"))}
	}
	state := fmt.Sprintf("lots of %d", sup.Lot)
	switch {
	case sup.Frozen(w.Day):
		state = theme.Bad.Render(fmt.Sprintf("not taking calls, %dd", sup.FrozenUntil-w.Day))
	case sup.Locked(w):
		state = theme.Subtle.Render("once " + cash(sup.UnlockCash) + " is moved")
	case sup.Left() == 0:
		state = theme.Warning.Render("nothing left today")
	}
	return []string{row("buys from", sup.Name+sep+state)}
}

// offerSection is an offer's detail: what it costs, washes and keeps,
// its audit risk, and whether it is locked, short or open to you.
func (m *Model) offerSection(o game.FrontOffer) section {
	w := m.w
	lines := []string{
		row("cost", money(o.Cost)+" dirty"),
		row("washes", money(o.Throughput)+"/day"),
		row("upkeep", money(o.Upkeep)+"/day clean"),
		row("audit", fmt.Sprintf("%.1f%%/day", o.AuditRisk*100)),
	}
	switch {
	case o.Locked(w):
		lines = append(lines, theme.Subtle.Render("locked until peak cash "+cash(o.UnlockCash)), theme.Subtle.Render(cash(o.UnlockCash-w.Stats.PeakCash)+" to go"))
	case o.Cost > w.Player.DirtyCash:
		lines = append(lines, theme.Bad.Render("short "+money(o.Cost-w.Player.DirtyCash)))
	default:
		lines = append(lines, keyRow("enter", "buy it"))
	}
	return section{strings.ToUpper(o.Name), lines}
}

// washSection is the wash as it stands: the dial, what the fronts wash
// and cost between them, the float the till keeps, the accountants and
// the dirty-cash warning.
func (m *Model) washSection() section {
	w := m.w
	l := m.set.Laundering
	tun := l.Tuning()
	lines := []string{
		row("dial", w.Laundering.Dial.String()),
		row("washing", fmt.Sprintf("up to %s/day", money(l.Capacity(w)))),
		row("upkeep", fmt.Sprintf("%s/day", money(l.Upkeep(w)))),
		row("audit", fmt.Sprintf("%.1f%%/day", l.AnyAuditRisk(w)*100)),
		row("fronts", fmt.Sprintf("%d · washed %s", len(w.Fronts), cash(w.Stats.Laundered))),
	}
	lines = append(lines, wrapped(theme.Subtle, fmt.Sprintf("The till keeps %s dirty for the street; the wash and the road spend only what is over it.", cash(tun.Float)))...)
	if n := w.Crew.Role("accountant"); n > 0 {
		lines = append(lines, wrapped(theme.Subtle, fmt.Sprintf("%s on the payroll: more through every front, fewer audits.", plural(n, "accountant")))...)
	} else if len(w.Fronts) > 0 {
		lines = append(lines, wrapped(theme.Subtle, "An accountant, hired "+screenPointer(screenCrew)+", adds to every front and cuts audit risk. Keep them loyal: they skim the wash.")...)
	}
	if thr := m.set.Heat.DirtyCashThreshold(w); thr > 0 && w.Player.DirtyCash > thr {
		lines = append(lines, wrapped(theme.Warning, fmt.Sprintf("Dirty cash over %s draws heat every day it sits there.", cash(thr)))...)
	}
	return section{"WASH", lines}
}
