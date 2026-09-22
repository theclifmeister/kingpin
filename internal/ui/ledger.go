package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// frontRows lists the fronts on offer that the player does not own yet,
// cheapest first, locked ones included so the ladder is visible.
func (m *Model) frontRows() []game.FrontOffer { return m.sess.FrontOffers() }

// askFront opens the buy picker on its first page, the kind (#73: a
// front or a house).
func (m *Model) askFront() {
	if m.w.Over != nil {
		return
	}
	if len(m.frontRows()) == 0 && len(m.houseRows()) == 0 && len(m.assetRows()) == 0 {
		m.refuse("Nothing to buy: you own every front, every house and every asset there is.")
		return
	}
	m.front.cursor, m.front.step = 0, 0 // the kind page first, wherever the cursor sits (#241: enter's shortcut onto an offer row went with enter)
	m.mode = modeFront
}

// confirmFront buys the offer under the cursor: a front, or a house on
// the picker's house page.
func (m *Model) confirmFront() {
	switch m.front.kind {
	case pickHouse:
		m.confirmHouse()
		return
	case pickAsset:
		m.confirmAsset()
		return
	}
	rows := m.frontRows()
	m.mode = modePlay
	if len(rows) == 0 {
		return
	}
	o := rows[max(0, min(m.front.cursor, len(rows)-1))]
	f, err := m.sess.BuyFront(o.ID)
	if err != nil {
		m.refuse("Can't buy: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Bought %s for %s. It opens tomorrow, washing up to %s/day.", f.Name, money(o.Cost), money(m.rules.Laundering.Throughput(m.w, f))))
}

func (m *Model) cycleLaunder() {
	d := (m.w.Laundering.Dial + 1) % 3
	if err := m.sess.SetLaunderDial(d); err != nil {
		m.refuse("Can't set the dial: " + err.Error())
		return
	}
	if len(m.w.Fronts) == 0 {
		m.say(fmt.Sprintf("Launder dial %s. %s Buy a front %s to use it.", d, launderBlurb(d), screenPointer(screenLedger)))
		return
	}
	m.say(fmt.Sprintf("Launder dial %s: washing up to %s/day, audit risk %s/day. %s", d, money(m.rules.Laundering.Capacity(m.w)), format.Pct(m.rules.Laundering.AnyAuditRisk(m.w), 1), launderBlurb(d)))
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
	case f.Frozen(m.w.Day+1) && f.Audited > 0 && f.FrozenUntil == f.Audited+m.rules.Laundering.Tuning().AuditFreezeDays:
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
	if m.front.step == 0 {
		return m.viewKind()
	}
	switch m.front.kind {
	case pickHouse:
		return m.viewHouses()
	case pickAsset:
		return m.viewAssets()
	}
	rows := m.frontRows()
	if len(rows) == 0 {
		return m.modal("BUY A FRONT", []string{"Nothing for sale."}, m.modalFooter())
	}
	clamp(&m.front.cursor, len(rows))
	return m.pickerModal("BUY A FRONT", nil, offerCols, m.offerRows(rows, m.modalInner()), m.front.cursor, m.inHand(), theme.Subtle.Render("It opens tomorrow."))
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
	ledgerDeed       // the property (#194)
	ledgerAsset      // an asset owned (#48)
	ledgerAssetOffer // one on offer
	ledgerPayoff     // the bought law (#42)
	ledgerOffer
)

// ledgerRow is one row of the ledger: which table and the index in it.
type ledgerRow struct{ kind, i int }

// ledgerRoutes are every open route in the file (#48: the plane and
// the tunnel once their asset stands), in city order: the LOGISTICS
// table's rows.
func (m *Model) ledgerRoutes() []content.RouteConfig {
	var routes []content.RouteConfig
	for _, cid := range m.w.CityOrder {
		for _, r := range m.rules.Logistics.RoutesOpen(m.w, cid) {
			if r.From == cid {
				routes = append(routes, r)
			}
		}
	}
	return routes
}

// ledgerRows are the rows the cursor walks: the fronts, the houses
// (#73), the deeds (#194), the routes, then the offers.
func (m *Model) ledgerRows() []ledgerRow {
	var rows []ledgerRow
	for i := range m.w.Fronts {
		rows = append(rows, ledgerRow{ledgerFront, i})
	}
	for i := range m.w.Houses {
		rows = append(rows, ledgerRow{ledgerHouse, i})
	}
	for i := range m.w.Deeds() {
		rows = append(rows, ledgerRow{ledgerDeed, i})
	}
	if m.assetsShown() {
		for i := range m.w.Assets {
			rows = append(rows, ledgerRow{ledgerAsset, i})
		}
		for i := range m.assetRows() {
			rows = append(rows, ledgerRow{ledgerAssetOffer, i})
		}
	}
	for i := range m.payoffRows() {
		rows = append(rows, ledgerRow{ledgerPayoff, i})
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
	return rows[clamp(&m.ledgerCursor, len(rows))]
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

// launderRow draws the launder dial as `careful  [normal]  greedy`.
func launderRow(d events.Launder) string {
	return dialCells(events.LaunderNames(), int(d-events.LaunderCareful))
}

// frontCols are the FRONTS table's columns: the level and what the
// front earns a day on its own (#192) beside what it washes; where MAIN
// is too narrow for the row whole the lifetime wash goes, then today's.
var frontCols = []col{{"front", kText, 0}, {"lvl", kInt, 0}, {"earns/day", kMoney, 0}, {"washes/day", kMoney, 0}, {"today", kMoney, 0}, {"lifetime", kMoney, 0}, {"audit", kPct, 0}, {"status", kText, 0}}

// viewLedger is the ledger's MAIN: the till lines, then FRONTS,
// LOGISTICS and ON OFFER as tables under the one cursor, scrolled so
// the cursor's table stays in view.
func (m *Model) viewLedger() string {
	w := m.w
	l := m.rules.Laundering
	width := m.mainWidth()
	sel := m.ledgerSelected()
	var ls []string
	line := func(s string) { ls = append(ls, truncate(s, width)) }
	sub := theme.Subtle.Render

	line(theme.PanelTitle.Render("LEDGER"))
	line(theme.Gold.Render("dirty "+cash(w.Player.DirtyCash)) + sub(" · ") + theme.Good.Render("clean "+cash(w.Player.CleanCash)) + sub(" · ") + theme.Gold.Render("offshore "+cash(w.Offshore)) + sub(fmt.Sprintf(" · seized %s lifetime", cash(w.Stats.Seized))))
	line(sub("launder  ") + launderRow(w.Laundering.Dial) + sub(fmt.Sprintf("   audit %s/day · up to %s/day · legit %s/day", format.Pct(l.AnyAuditRisk(w), 1), money(l.Capacity(w)), money(l.LegitIncome(w)))))
	if thr := m.rules.Heat.DirtyCashThreshold(w); thr > 0 && w.Player.DirtyCash > thr {
		line(theme.Warning.Render(fmt.Sprintf("▲ Dirty cash over %s draws heat every day it sits there.", cash(thr))))
	}
	// The tax (#231): what the free corners of a city you hold pay a
	// night, city by city where it holds.
	for _, cid := range w.CityOrder {
		if corners, amount := m.rules.Territory.TaxDue(w, cid); corners > 0 {
			line(sub("tax      ") + theme.Gold.Render(fmt.Sprintf("%s in %s pay ~%s/night", plural(corners, "free corner"), w.CityName(cid), money(amount))) + sub(fmt.Sprintf(" · %s so far", cash(w.Stats.Taxed))))
		}
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
		line(sectionTitle(title, m.accent()) + sub(note))
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
			rows = append(rows, []any{f.Name, f.Level, l.Income(f), l.Throughput(w, f), f.WashedToday, f.Washed, l.AuditRisk(w, f) * 100, m.frontStatus(f)})
		}
		cols := append([]col(nil), frontCols...)
		for _, drop := range []int{5, 4} {
			if tableWidth(cols, rows) <= width {
				break
			}
			cols = append(cols[:drop:drop], cols[drop+1:]...)
			for i := range rows {
				rows[i] = append(rows[i][:drop:drop], rows[i][drop+1:]...)
			}
		}
		tableLines(ledgerFront, cols, rows)
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

	// The property (#194): every block whose deed is yours, and the
	// DA's line in the heading; a ledger with no deed has no table, the
	// STASH rule, so the ledger fits as it did (the block is for sale on
	// the map, where the corner's inspector prices it).
	if deeds := w.Deeds(); len(deeds) > 0 {
		heading("PROPERTY", m.deedNote())
		var rows [][]any
		for _, c := range deeds {
			rows = append(rows, m.deedRow(c))
		}
		cols := append([]col(nil), deedCols...)
		// Where MAIN is too narrow for the row whole the pane's columns
		// go: since, then whose corner it is.
		for _, drop := range []int{5, 2} {
			if tableWidth(cols, rows) <= width {
				break
			}
			cols = append(cols[:drop:drop], cols[drop+1:]...)
			for i := range rows {
				rows[i] = append(rows[i][:drop:drop], rows[i][drop+1:]...)
			}
		}
		tableLines(ledgerDeed, cols, rows)
	}

	// The assets (#48): owned, then on offer, once the cartel is in
	// view; the offers are bought on the picker's asset page.
	if m.assetsShown() {
		owned, offers := w.Assets, m.assetRows()
		note := fmt.Sprintf(" · %s owned · %s/day clean", plural(len(owned), "asset"), money(m.rules.Laundering.AssetUpkeep(w)))
		if len(w.AssetsLost) > 0 {
			note += fmt.Sprintf(" · %d lost", len(w.AssetsLost))
		}
		heading("ASSETS", note)
		if m.rules.Heat.TaskForceForming(w) {
			line(theme.Bad.Render("▲ A task force formed this morning and comes tonight. Lie low."))
		}
		if len(owned)+len(offers) == 0 {
			line(sub("Every asset there was is gone."))
		} else {
			cols := append([]col(nil), assetCols...)
			rows := m.assetTable(owned, offers, true)
			if tableWidth(cols, rows) > width {
				rows = m.assetTable(owned, offers, false)
			}
			for _, drop := range []int{2, 1} {
				if tableWidth(cols, rows) <= width {
					break
				}
				cols = append(cols[:drop:drop], cols[drop+1:]...)
				for i := range rows {
					rows[i] = append(rows[i][:drop:drop], rows[i][drop+1:]...)
				}
			}
			c := -1
			if sel.kind == ledgerAsset {
				c = sel.i
			} else if sel.kind == ledgerAssetOffer {
				c = len(owned) + sel.i
			}
			if c >= 0 {
				top, at = len(ls)-1, len(ls)+1+c
			}
			ls = append(ls, table(cols, rows, c, width)...)
		}
	}

	// The road is the map's (#245): the ledger reads its money and
	// points at the map for the dials, the targets and what is on it.
	lost := 0
	for _, n := range w.Logistics.Lost {
		lost += n
	}
	heading("LOGISTICS", fmt.Sprintf(" · shipped %s in %s · seized %d", plural(w.Stats.Shipped, "unit"), plural(w.Stats.Shipments, "run"), lost))
	line(theme.Subtle.Render(fmt.Sprintf("%s open; the road is %s.", plural(len(m.ledgerRoutes()), "route"), screenPointer(screenMap))))

	// The bought law (#42): every live deal, and what the DA has heard
	// if somebody on the payroll knows.
	heading("PAYOFFS", m.payoffNote())
	if payoffs := m.payoffRows(); len(payoffs) == 0 {
		line(emptyState("Nobody at city hall is on the payroll; checkpoints are on the map."))
	} else {
		tableLines(ledgerPayoff, payoffCols, m.payoffTable(payoffs))
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
	case ledgerDeed:
		if deeds := m.w.Deeds(); sel.i < len(deeds) {
			secs = append(secs, m.deedSection(deeds[sel.i]))
		}
	case ledgerAsset:
		secs = append(secs, m.assetSection(m.w.Assets[sel.i]))
	case ledgerAssetOffer:
		secs = append(secs, m.assetOfferSection(m.assetRows()[sel.i]))
	case ledgerPayoff:
		if rows := m.payoffRows(); sel.i < len(rows) {
			secs = append(secs, m.payoffSection(rows[sel.i]))
		}
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
	l := m.rules.Laundering
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
		row("audit", pctText(l.AuditRisk(w, f)*100)+"/day at "+w.Laundering.Dial.String()),
	}
	if m.cfg.Laundering.Front(f.ID) != nil {
		lines = append(lines, row("upkeep", money(l.FrontUpkeep(w, f))+"/day clean"))
	}
	lines = append(lines, row("bought", fmt.Sprintf("day %d · %s", f.Bought, money(f.Cost))))
	// The levels (#192): where the front stands, what it earns on its
	// own, and what the next level costs and adds.
	if top := l.MaxLevel(f); top > 0 {
		lines = append(lines, row("level", fmt.Sprintf("%d of %d · earns %s/day", f.Level, top, money(l.Income(f)))))
		if f.Level < top {
			next := f
			next.Level++
			lines = append(lines, row("next", fmt.Sprintf("%s clean → +%s/day", money(l.LevelCost(f, 1)), money(l.Income(next)-l.Income(f)))))
			lines = append(lines, keyRow("u", "invest"))
		}
	}
	return section{strings.ToUpper(f.Name), lines}
}

// accountantBonus is what the accountants add to a front's daily wash
// at the dial, when there are any.
func (m *Model) accountantBonus(f game.Front) (int, bool) {
	fc := m.cfg.Laundering.Front(f.ID)
	if fc == nil || m.w.Crew.Role(game.RoleAccountant) == 0 {
		return 0, false
	}
	base := int(float64(fc.Throughput)*m.rules.Laundering.Dial(m.w.Laundering.Dial).Mul + 0.5)
	return m.rules.Laundering.Throughput(m.w, f) - base, true
}

// offerSection is an offer's detail: what it costs, washes and keeps,
// its audit risk, and whether it is locked, short or open to you.
func (m *Model) offerSection(o game.FrontOffer) section {
	w := m.w
	lines := []string{
		row("cost", money(o.Cost)+" dirty"),
		row("washes", money(o.Throughput)+"/day"),
		row("upkeep", money(o.Upkeep)+"/day clean"),
		row("audit", pctText(o.AuditRisk*100)+"/day"),
	}
	switch {
	case o.Locked(w):
		lines = append(lines, theme.Subtle.Render("locked until peak cash "+cash(o.UnlockCash)), theme.Subtle.Render(cash(o.UnlockCash-w.Stats.PeakCash)+" to go"))
	case o.Cost > w.Player.DirtyCash:
		lines = append(lines, theme.Bad.Render("short "+money(o.Cost-w.Player.DirtyCash)))
	default:
		lines = append(lines, keyRow("b", "buy it through the picker"))
	}
	return section{strings.ToUpper(o.Name), lines}
}

// washSection is the wash as it stands: the dial, what the fronts wash
// and cost between them, the float the till keeps, the accountants and
// the dirty-cash warning.
func (m *Model) washSection() section {
	w := m.w
	l := m.rules.Laundering
	tun := l.Tuning()
	lines := []string{
		row("dial", w.Laundering.Dial.String()),
		row("washing", fmt.Sprintf("up to %s/day", money(l.Capacity(w)))),
		row("upkeep", fmt.Sprintf("%s/day", money(l.Upkeep(w)))),
		row("audit", pctText(l.AnyAuditRisk(w)*100)+"/day"),
		row("legit", fmt.Sprintf("%s/day net of upkeep", money(l.LegitIncome(w)))),
		row("fronts", fmt.Sprintf("%d · washed %s", len(w.Fronts), cash(w.Stats.Laundered))),
	}
	lines = append(lines, wrapped(theme.Subtle, fmt.Sprintf("The till keeps %s dirty for the street; the wash and the road spend only what is over it.", cash(tun.Float)))...)
	if n := w.Crew.Role(game.RoleAccountant); n > 0 {
		lines = append(lines, wrapped(theme.Subtle, fmt.Sprintf("%s on the payroll: more through every front, fewer audits.", plural(n, "accountant")))...)
	} else if len(w.Fronts) > 0 {
		// The pointer on a line of its own: wrapped mid-phrase it read
		// `screen (4)` at a line's start, a key hint to the grammar's eye.
		lines = append(lines, wrapped(theme.Subtle, "An accountant adds to every front and cuts audit risk. Keep them loyal: they skim the wash.")...)
		lines = append(lines, theme.Subtle.Render("Hire one "+screenPointer(screenCrew)+"."))
	}
	if thr := m.rules.Heat.DirtyCashThreshold(w); thr > 0 && w.Player.DirtyCash > thr {
		lines = append(lines, wrapped(theme.Warning, fmt.Sprintf("Dirty cash over %s draws heat every day it sits there.", cash(thr)))...)
	}
	return section{"WASH", lines}
}
