package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The route dial (#61). Every route has a dial, off / slow / normal /
// fast, and a target stock for its far end per product, in units or in
// days of the far end's demand (#115); the logistics sim runs it every
// day. The map lists the routes out of the city shown under its grid,
// with a cursor the arrows reach past the bottom row: r turns the
// selected route's dial, R opens the target dialog. The ledger lists
// the same routes with their books.

// targetDialog is the state of the target modal: product, then whether
// the target is units or days of the far end's demand (←→, #115), then
// the number.
type targetDialog struct {
	stepper
	route string // route id it sets
	days  bool   // the number is days of the far end's demand, not units
	units numberField
}

func (d *targetDialog) fieldAt(step int) *numberField {
	if step == 2 {
		return &d.units
	}
	return nil
}

func (d *targetDialog) field() *numberField { return d.fieldAt(d.step) }

// mapRoutes are the routes touching the city the map shows, in file
// order: what the routes cursor walks.
func (m *Model) mapRoutes() []content.RouteConfig {
	return m.rules.Logistics.RoutesOpen(m.w, m.shown().ID) // the plane and the tunnel once their asset stands (#48)
}

// selectedRoute is the route under the routes cursor, or nil when the
// city shown has none.
func (m *Model) selectedRoute() *content.RouteConfig {
	routes := m.mapRoutes()
	if len(routes) == 0 {
		return nil
	}
	return &routes[clamp(&m.routeCursor, len(routes))]
}

// cycleRoute turns the selected route's dial a notch: off, slow, normal,
// fast, off.
func (m *Model) cycleRoute() {
	if m.w.Over != nil {
		return
	}
	r := m.selectedRoute()
	if r == nil {
		m.refuse("Nothing to turn: no route out of " + m.shown().Name + ".")
		return
	}
	d := (m.w.Route(r.ID).Dial + 1) % (events.RouteFast + 1)
	if err := m.sess.SetRoute(r.ID, d); err != nil {
		m.refuse("Can't turn the dial: " + err.Error())
		return
	}
	m.sayRouteDial(*r, d)
}

// sayRouteDial is the status after a route's dial is turned, on the
// map or the ledger: what the road does at the notch.
func (m *Model) sayRouteDial(r content.RouteConfig, d events.RouteDial) {
	if !d.On() {
		m.say(fmt.Sprintf("%s off: nothing moves on it. Its targets are kept.", r.Name))
		return
	}
	lg := m.rules.Logistics
	line := fmt.Sprintf("%s %s: %s %s to %s, seized %s.", r.Name, d, plural(lg.Days(m.w, r, d.Ship()), "day"), r.Mode, m.w.CityName(r.To), m.seizedWord(r, d.Ship()))
	if !m.w.Route(r.ID).HasTargets() {
		line += " It sends nothing without a target."
	}
	m.say(line)
}

// openTarget opens the target dialog for the selected route.
func (m *Model) openTarget() {
	if m.w.Over != nil {
		return
	}
	r := m.selectedRoute()
	if r == nil {
		m.refuse("Nothing to target: no route out of " + m.shown().Name + ".")
		return
	}
	m.tgt = targetDialog{route: r.ID, units: newNumberField("blank = none")}
	m.mode = modeTarget
}

// targetRoute is the route the target dialog is about.
func (m *Model) targetRoute() *content.RouteConfig {
	if r := m.rules.Logistics.Route(m.tgt.route); r != nil {
		return r
	}
	return m.selectedRoute()
}

func (m *Model) keyTarget(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.tgt
	d.err = ""
	r := m.targetRoute()
	if r == nil {
		m.mode = modePlay
		return m, nil
	}
	// Back is one key and close is one key (#110): esc closes from any
	// step, shift+tab goes back one (the product stays under the cursor
	// and the kind stays turned; the number is cleared on leaving its
	// step) and is silent on the first, tab goes forward (a product and
	// a kind are always chosen) and is silent on the last. The number is
	// a numberField (#112) whose max is targetMax.
	if closes(key) {
		m.mode = modePlay
		return m, nil
	}
	switch key {
	case "shift+tab":
		return m, d.back(d.fieldAt)
	case "tab":
		if d.step == 2 {
			return m, nil
		}
	}
	switch d.step {
	case 0:
		switch key {
		case "up", "k":
			stepCursor(&m.cursor, -1, len(m.w.Products))
		case "down", "j":
			stepCursor(&m.cursor, 1, len(m.w.Products))
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(key[0] - '1'); i < len(m.w.Products) {
				m.cursor = i
			}
		case "enter", "right", "l", "tab":
			// The kind opens on what the product has: days if it keeps
			// days, else units.
			d.step = 1
			d.days = m.w.Route(r.ID).Days[m.w.Products[m.cursor]] > 0
		}
	case 1:
		switch key {
		case "left", "right", "h", "l":
			d.days = !d.days
		case "enter", "tab":
			d.step = 2
			d.units.SetValue("")
			rs := m.w.Route(r.ID)
			if t := rs.Days[m.w.Products[m.cursor]]; d.days && t > 0 {
				d.units.Set(t)
			} else if t := rs.Target[m.w.Products[m.cursor]]; !d.days && t > 0 {
				d.units.Set(t)
			}
			return m, d.units.Focus()
		}
	case 2:
		if key == "enter" {
			return m.confirmTarget()
		}
		d.units.max = m.targetMax(r)
		return m, d.units.Update(k)
	}
	return m, nil
}

// targetMax is what the target's shortcuts fill to: what the stash at
// the route's far end can hold (World.Capacity there: the runners
// posted there, and you and the idle runners when you stand there), or
// for a days target the days of the far end's demand that holds. A
// target is stock to keep, not a shipment, so the route's capacity is
// not the line; and a typed target over it is set as it always was,
// the wholesaler selling into the stash whatever its room.
func (m *Model) targetMax(r *content.RouteConfig) int {
	room := m.w.Capacity(r.To)
	if !m.tgt.days {
		return room
	}
	if demand := m.w.Demand(r.To, m.w.Products[m.cursor]); demand > 0 {
		return int(float64(room) / demand)
	}
	return 0
}

// targetToday is the units a route's target means this morning, the
// logistics sim's read of it (Sim.Target) so the dialog and the pane
// show the number the road sends against.
func (m *Model) targetToday(r content.RouteConfig, product string) int {
	return m.rules.Logistics.Target(m.w, r, product)
}

func (m *Model) confirmTarget() (tea.Model, tea.Cmd) {
	r := m.targetRoute()
	id := m.w.Products[m.cursor]
	n := 0
	if s := strings.TrimSpace(m.tgt.units.Value()); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			m.tgt.err = "enter a whole number, or nothing for none"
			return m, nil
		}
		n = v
	}
	set, kept := m.sess.SetRouteTarget, fmt.Sprintf("%d %s", n, m.w.ProductName(id))
	if m.tgt.days {
		set = m.sess.SetRouteDays
		kept = fmt.Sprintf("%s of %s's demand", plural(n, "day"), m.w.ProductName(id))
	}
	if err := set(r.ID, id, n); err != nil {
		m.tgt.err = err.Error()
		return m, nil
	}
	if m.tgt.days && n > 0 {
		kept += fmt.Sprintf(" (≈%d today)", m.targetToday(*r, id))
	}
	m.mode = modePlay
	switch {
	case n == 0:
		m.say(fmt.Sprintf("%s: no target for %s; the route leaves it alone.", r.Name, m.w.ProductName(id)))
	case !m.w.Route(r.ID).Dial.On():
		m.say(fmt.Sprintf("%s keeps %s at %s once its dial is on.", r.Name, m.w.CityName(r.To), kept))
	default:
		m.say(fmt.Sprintf("%s keeps %s at %s: it sends the shortfall every day.", r.Name, m.w.CityName(r.To), kept))
	}
	return m, nil
}

func (m *Model) viewTarget() string {
	w := m.w
	d := m.tgt
	r := m.targetRoute()
	if r == nil {
		return m.modal("TARGET", []string{"No route."}, m.modalFooter())
	}
	rs := w.Route(r.ID)
	// What the route does (#496): it ships out of the stash at the
	// source, and buys only whole lots off the wholesaler there, only
	// while their door is open; a playtest read "buying by the lot" as
	// a route that buys and saw it idle on an empty stash.
	body := m.subtle(fmt.Sprintf("%s keeps %s stocked: every day it ships what is short of the target, up to %d units, out of the %s stash, fares in dirty cash. %s", r.Name, w.CityName(r.To), m.rules.Logistics.Capacity(w, *r), w.CityName(r.From), m.routeBuys(*r)))
	body = append(body, "")
	var rows [][]any
	for _, pid := range w.Products {
		var target any
		switch {
		case rs.Days[pid] > 0:
			target = fmt.Sprintf("%dd (≈%d)", rs.Days[pid], m.targetToday(*r, pid))
		case rs.Target[pid] > 0:
			target = strconv.Itoa(rs.Target[pid])
		}
		rows = append(rows, []any{w.ProductName(pid), target, w.Stock(r.To, pid), w.Bound(r.To, pid), approx{w.Demand(r.To, pid)}})
	}
	m.modalFollow(len(body) + 1 + m.cursor) // under the header
	body = append(body, table([]col{{"product", kText, 0}, {"target", kText, 0}, {"there", kInt, 0}, {"road", kInt, 0}, {"sells/day", kInt, 0}}, rows, m.cursor, m.modalInner())...)
	id := w.Products[m.cursor]
	on := 0
	if d.days {
		on = 1
	}
	switch d.step {
	case 0:
		body = append(body, "", theme.Subtle.Render("Pick a product."))
	case 1:
		// The kind: a count of units, or days of what the corners at
		// the far end sell, which the road sizes again every morning.
		body = append(body, "", "Kept as   "+dialCells([]string{"units", "days"}, on))
		if d.days {
			body = append(body, theme.Subtle.Render(fmt.Sprintf("          days of %s's demand in %s, ~%.0f/day today: the", w.ProductName(id), w.CityName(r.To), w.Demand(r.To, id))),
				theme.Subtle.Render("          target follows the corners you hold there."))
		} else {
			body = append(body, row("", theme.Subtle.Render(fmt.Sprintf("units of %s kept in %s, whatever sells there.", w.ProductName(id), w.CityName(r.To)))))
		}
	default:
		d.units.max = m.targetMax(r)
		what := fmt.Sprintf("units of %s kept in %s", w.ProductName(id), w.CityName(r.To))
		if d.days {
			what = fmt.Sprintf("days of %s's demand in %s", w.ProductName(id), w.CityName(r.To))
		}
		body = append(body, "", row("target", d.units.View()+"   "+theme.Subtle.Render(what)))
		if s := strings.TrimSpace(d.units.Value()); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				if d.days {
					// What the days mean this morning: the number the
					// road sends against, and where it comes from.
					n = m.rules.Logistics.DaysTarget(w, *r, id, n)
					body = append(body, row("today", theme.Subtle.Render(fmt.Sprintf("%sd ≈ %s: ~%.0f/day on your corners in %s", s, plural(n, "unit"), w.Demand(r.To, id), w.CityName(r.To)))))
				}
				if src := w.Product(r.From, id); src != nil {
					short := max(0, n-w.Stock(r.To, id)-w.Bound(r.To, id))
					stashed := min(short, w.Stock(r.From, id))
					how := fmt.Sprintf("%d from the %s stash", stashed, w.CityName(r.From))
					if rest := short - stashed; rest > 0 {
						if sup := w.WholesaleSupplier(r.From); sup != nil && sup.Open(w) && sup.Price[id] > 0 {
							how += fmt.Sprintf(", %d by the lot from %s ~%s", rest, sup.Name, money(int(float64(rest)*sup.Price[id])))
						} else {
							how += fmt.Sprintf(", %d with nothing to ship", rest)
						}
					}
					body = append(body, row("short", theme.Subtle.Render(fmt.Sprintf("%d: %s + %s fares", short, how, money(int(math.Ceil(float64(short)*m.rules.Logistics.Fare(w, *r))))))))
				}
			}
		}
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal("TARGET · "+r.Name, body, m.modalFooter())
}

// routeBuys is the target dialog's word on what a route buys (#496): whole
// lots off the wholesaler at the source while their door is open, out of
// the dirty cash over the till, or nothing, the stash being yours to
// stock.
func (m *Model) routeBuys(r content.RouteConfig) string {
	w := m.w
	sup := w.WholesaleSupplier(r.From)
	switch {
	case sup == nil:
		return fmt.Sprintf("It buys nothing: stock the %s stash yourself.", w.CityName(r.From))
	case sup.Open(w):
		return fmt.Sprintf("What the stash lacks it buys by the lot from %s, out of the dirty cash over the %s till.", sup.Name, money(m.till()))
	case sup.Locked(w):
		return fmt.Sprintf("It buys nothing until %s deals with you (%s moved): stock the %s stash yourself.", sup.Name, money(sup.UnlockCash), w.CityName(r.From))
	}
	return fmt.Sprintf("It buys by the lot from %s while they deal; today they do not: stock the %s stash yourself.", sup.Name, w.CityName(r.From))
}

// targetLine is a route's targets in one line, `3d (≈180) Weed · 400
// Coke`, a days target with the units it means today, or "" when it
// has none.
func (m *Model) targetLine(route string) string {
	r := m.rules.Logistics.Route(route)
	rs := m.w.Route(route)
	var parts []string
	for _, id := range m.w.Products {
		switch {
		case rs.Days[id] > 0 && r != nil:
			parts = append(parts, fmt.Sprintf("%dd (≈%d) %s", rs.Days[id], m.targetToday(*r, id), m.w.ProductName(id)))
		case rs.Target[id] > 0:
			parts = append(parts, fmt.Sprintf("%d %s", rs.Target[id], m.w.ProductName(id)))
		}
	}
	return strings.Join(parts, " · ")
}

// routeIdle is why a route on its dial would send nothing (#459,
// logistics.Sim.Idle), as the map's row words it where it fits
// (long), as it words it where it does not (short), and its colour:
// a warning where the route is short of its target, subtle where it
// is simply at it. All "" for a route that would send, or is off.
func (m *Model) routeIdle(r content.RouteConfig) (long, short string, style lipgloss.Style) {
	style = theme.Warning
	switch m.rules.Logistics.Idle(m.w, r) {
	case events.IdleTill:
		if m.rules.Logistics.Budget(m.w) > 0 {
			// Over the till, but not by a lot and its fare.
			return "idle: too little over the " + cash(m.till()) + " till for a lot", "idle: till", style
		}
		return "idle: no dirty cash over the " + cash(m.till()) + " till", "idle: till", style
	case events.IdleStock:
		return "idle: nothing in the " + m.w.CityName(r.From) + " stash", "idle: empty", style
	case events.IdleNoTarget:
		return "idle: no target", "no target", style
	case events.IdleClosed:
		return "idle: shut", "shut", style
	case events.IdleMet:
		return "idle: target met", "idle: met", theme.Subtle
	}
	return "", "", style
}

// dialStyle is the colour a route dial is drawn in: the dial's accent
// when it is on, red at fast (the risk is the road's danger), Subtle
// off. The pane's row is one bracketed notch, `[slow]`, because four
// notches do not fit its value column.
func dialStyle(d events.RouteDial) lipgloss.Style {
	switch d {
	case events.RouteFast:
		return theme.Bad
	default:
		return theme.Dial(d.On())
	}
}

// routeLines are the routes out of the city shown, one line each, drawn
// as an edge between the cities with the dial, the route's terms at that
// dial (`3d · 60 units · $8/u · ~3%`; `60u` where the long form would
// not fit the width, and no terms at all where that would not either:
// the pane has them) and, on the edge, where each shipment in flight is
// (#160: `Bayport ─truck──▪───▶ Eastside`, the track's cells being the
// days in transit; the track goes last, where even the bare line would
// not fit, and the pane keeps the marker whatever the width); the
// routes cursor's row is marked ▸ and drawn Selected across while the
// cursor is on the routes, and ▹ while it is on the grid (#462). A route
// on its dial that would send nothing says why after the dial (#459,
// routeIdle: `idle: no dirty cash over the $50K till`, shortened to
// `idle: till` before the track goes). The selected route's targets
// are the pane's (routeSection).
func (m *Model) routeLines(width int) []string {
	w := m.w
	lg := m.rules.Logistics
	routes := m.mapRoutes()
	nameW, cityW, modeW := 0, 0, edgeW
	for _, r := range routes {
		nameW = max(nameW, lipgloss.Width(r.Name))
		cityW = max(cityW, lipgloss.Width(w.CityName(r.From)), lipgloss.Width(w.CityName(r.To)))
		modeW = max(modeW, lipgloss.Width(r.Mode))
	}
	// Why a route sends nothing (#459), once a route: the long words
	// where they fit, the short where they do not.
	type why struct {
		long, short string
		style       lipgloss.Style
	}
	idle := make([]why, len(routes))
	for i, r := range routes {
		idle[i].long, idle[i].short, idle[i].style = m.routeIdle(r)
	}
	draw := func(units string, track, long bool) (lines []string, widest int) {
		for i, r := range routes {
			d := w.Route(r.ID).Dial
			name := fit(r.Name, nameW)
			road := ""
			if track {
				road = m.track(r.ID)
			}
			from := fit(w.CityName(r.From), cityW) + " " + modeEdgeW(r.Mode, modeW)
			to := "▶ " + fit(w.CityName(r.To), cityW)
			dial := fit(d.String(), 6)
			terms := ""
			if units != "" {
				terms = fmt.Sprintf("  %dd · %d%s · %s/u · %s", lg.Days(w, r, d.Ship()), lg.Capacity(w, r), units, fare(lg.Fare(w, r)), m.seizedWord(r, d.Ship()))
			}
			why := idle[i].short
			if long {
				why = idle[i].long
			}
			if why != "" {
				why = "  " + why // before the terms, which go first where the row is narrow
			}
			plain := name + "  " + from + road + to + "  " + dial + why + terms
			widest = max(widest, 2+lipgloss.Width(plain))
			mark := "  "
			if i == m.routeCursor {
				// One ▸ on the screen (#462): the route's while the
				// arrows are on the routes, ▹ while they walk the grid.
				mark = theme.Subtle.Render(unfocusedMark + " ")
				if m.onRoutes {
					lines = append(lines, theme.Gold.Render("▸ ")+theme.Selected.Render(plain))
					continue
				}
			}
			lines = append(lines, mark+name+"  "+theme.Subtle.Render(from)+markTrack(road)+theme.Subtle.Render(to)+"  "+dialStyle(d).Render(dial)+idle[i].style.Render(why)+theme.Subtle.Render(terms))
		}
		return lines, widest
	}
	lines, widest := draw(" units", true, true)
	if widest > width {
		lines, widest = draw("u", true, true)
	}
	if widest > width {
		lines, widest = draw("", true, true)
	}
	if widest > width {
		lines, widest = draw("", true, false)
	}
	if widest > width {
		lines, _ = draw("", false, false)
	}
	for i, l := range lines {
		lines[i] = truncate(l, width)
	}
	return lines
}

// trackW is the cells of road between the mode and the arrowhead on the
// map's route line, where a shipment's marker sits by the days it has
// been on the road (#160): `─truck──▪───▶`.
const trackW = 5

// track is the road a route's shipments are on: trackW cells of `─`
// with a `▪` on each cell a shipment in flight has reached (the days
// elapsed over the days in transit, floored onto the track and clamped
// inside it, so one landing today sits at the arrowhead's end) and the
// count after it where two share a cell (`▪2`), plain, for the map's
// route line. Read off World.Shipments and the day like every other
// cell: no tick, redrawn on key.
func (m *Model) track(route string) string {
	counts := make([]int, trackW)
	for _, sh := range m.w.Shipments {
		if sh.Route == route {
			counts[trackCell(sh, m.w.Day)]++
		}
	}
	cells := []rune(strings.Repeat("─", trackW))
	for i, n := range counts {
		if n == 0 {
			continue
		}
		label := []rune("▪")
		if n > 1 {
			label = []rune("▪" + strconv.Itoa(n))
		}
		label = label[:min(len(label), trackW)]
		at := min(i, trackW-len(label))
		copy(cells[at:], label)
	}
	return string(cells)
}

// trackCell is the cell of the track a shipment has reached on day:
// floor(elapsed / days × trackW), clamped inside the track.
func trackCell(sh game.Shipment, day int) int {
	days := max(1, sh.Arrives-sh.Sent)
	elapsed := max(0, day-sh.Sent)
	return min(trackW-1, elapsed*trackW/days)
}

// markTrack colours a track: the road in Subtle, the markers and their
// counts in the road's colour.
func markTrack(track string) string {
	var b strings.Builder
	for _, run := range splitRuns(track, '─') {
		if strings.HasPrefix(run, "─") {
			b.WriteString(theme.Subtle.Render(run))
		} else {
			b.WriteString(theme.RoadText.Render(run))
		}
	}
	return b.String()
}

// splitRuns breaks s into runs of sep and runs of everything else, in
// order, so each can be styled once.
func splitRuns(s string, sep rune) []string {
	var runs []string
	var run []rune
	for _, r := range s {
		if len(run) > 0 && (run[0] == sep) != (r == sep) {
			runs = append(runs, string(run))
			run = run[:0]
		}
		run = append(run, r)
	}
	if len(run) > 0 {
		runs = append(runs, string(run))
	}
	return runs
}

// shipmentLines are the pane's `on the road` rows, one per shipment in
// flight on the route in the order sent, each in two parts the row
// joins with ` · `: `▪ day 2 of 3`, the day it is on over the days in
// transit, the marker the map's, and `400 Weed`.
func (m *Model) shipmentLines(route string) [][]string {
	var lines [][]string
	for _, sh := range m.w.Shipments {
		if sh.Route != route {
			continue
		}
		days := max(1, sh.Arrives-sh.Sent)
		day := min(days, max(0, m.w.Day-sh.Sent)+1)
		lines = append(lines, []string{fmt.Sprintf("▪ day %d of %d", day, days), fmt.Sprintf("%d %s", sh.Units, m.w.ProductName(sh.Product))})
	}
	return lines
}

// routeSection is the route's detail for the map's pane: the facts
// (routeFacts) and what r, R and v do to it.
func (m *Model) routeSection(r content.RouteConfig) section {
	title, lines := m.routeFacts(r)
	return section{title, append(lines, keyRow("r", "turn the dial"), keyRow("R", "set a target"), keyRow("v", "put a driver on it"))}
}

// routeFacts is the route's detail without its keys (#240: the ledger
// takes the facts and adds keys of its own): the title, then the edge,
// the dial and the terms at it, the targets, what is on the road and
// the driver.
func (m *Model) routeFacts(r content.RouteConfig) (string, []string) {
	w := m.w
	lg := m.rules.Logistics
	d := w.Route(r.ID).Dial
	lines := []string{
		theme.Subtle.Render(fmt.Sprintf("%s %s %s", w.CityName(r.From), edge(r.Mode), w.CityName(r.To))),
		row("dial", dialStyle(d).Render("["+d.String()+"]")),
	}
	if rs := w.Route(r.ID); w.RouteClosed(r.ID) {
		// Shut by an incident (#44): tonight and the nights after it before it reopens.
		lines = append(lines, row("closed", theme.Warning.Render(plural(rs.ClosedUntil-w.Day-1, "night")+" to go")))
	}
	if long, _, st := m.routeIdle(r); long != "" && !w.RouteClosed(r.ID) && w.Route(r.ID).HasTargets() {
		// Why it sends nothing (#459), wrapped: the closed row above says a shut one's, the target row one with none.
		label := "idle"
		for _, l := range wrap(strings.TrimPrefix(long, "idle: "), m.valueW()) {
			lines = append(lines, row(label, st.Render(l)))
			label = ""
		}
	}
	if r.Mode == "plane" {
		// The plane's risk is the task force's alone (#48): the file's
		// while the feds watch the skies, nothing otherwise.
		if m.rules.Logistics.Watched(w, w.Day+1) {
			lines = append(lines, row("watched", theme.Warning.Render(fmt.Sprintf("the feds, %s to go", plural(w.Heat.WatchUntil-w.Day-1, "night")))))
		} else {
			lines = append(lines, row("watched", theme.Subtle.Render("nobody: the sky is clear")))
		}
	}
	lines = append(lines,
		row("days", fmt.Sprintf("%d · capacity %d", lg.Days(w, r, d.Ship()), lg.Capacity(w, r))),
		row("fare", fmt.Sprintf("%s/u · seized %s", fare(lg.Fare(w, r)), m.seizedWord(r, d.Ship()))),
	)
	switch t := m.targetLine(r.ID); {
	case t == "" && d.On():
		lines = append(lines, row("target", theme.Warning.Render("none: it sends nothing")))
	case t == "":
		lines = append(lines, row("target", theme.Subtle.Render("none")))
	default:
		label := "target"
		for _, l := range wrap(t, m.valueW()) {
			lines = append(lines, row(label, l))
			label = ""
		}
	}
	label := "on the road"
	for _, sh := range m.shipmentLines(r.ID) {
		// One row a shipment where the value column holds it, else the
		// day on one row and the units under it.
		rows := []string{strings.Join(sh, " · ")}
		if lipgloss.Width(rows[0]) > m.valueW() {
			rows = sh
		}
		for _, l := range rows {
			lines = append(lines, row(label, theme.RoadText.Render(l)))
			label = ""
		}
	}
	lines = append(lines, row("driver", m.driverLine(r.ID)))
	return strings.ToUpper(r.Name), lines
}

// edge draws a route's mode as an arrow of fixed width, so the routes
// line up: ──car──▶, ─truck─▶; a mode longer than the width (#48, the
// tunnel) widens its own edge.
func edge(mode string) string { return modeEdge(mode) + "▶" }

// edgeW is the cells a mode's name has in the edge: five, `truck`.
const edgeW = 5

// modeEdge is the edge without its arrowhead, ──car──, so the map's
// route line can lay the track between the mode and the arrowhead.
func modeEdge(mode string) string { return modeEdgeW(mode, edgeW) }

// modeEdgeW is modeEdge at a width: the routes list's, the widest mode
// shown, so every edge in the list lines up.
func modeEdgeW(mode string, width int) string {
	width = max(width, lipgloss.Width(mode))
	mode = fit(mode, width)
	pad := width - lipgloss.Width(strings.TrimRight(mode, " "))
	mode = strings.TrimRight(mode, " ")
	return strings.Repeat("─", 1+pad/2) + mode + strings.Repeat("─", 1+pad-pad/2)
}

// askTravel asks before moving you to the other city.
func (m *Model) askTravel() {
	if m.w.Over != nil {
		return
	}
	if len(m.w.CityOrder) < 2 {
		m.refuse("Can't go: there is nowhere else.")
		return
	}
	m.ask("go", (*Model).travelConfirm, (*Model).confirmTravel)
}

// travelTo is the city g goes to: the one shown if you are not in it,
// else the next one.
func (m *Model) travelTo() string {
	if m.city != m.w.Player.Location {
		return m.city
	}
	order := m.w.CityOrder
	for i, id := range order {
		if id == m.w.Player.Location {
			return order[(i+1)%len(order)]
		}
	}
	return order[0]
}

func (m *Model) confirmTravel() {
	to := m.travelTo()
	m.mode = modePlay
	if err := m.sess.Travel(to); err != nil {
		m.refuse("Can't go: " + err.Error())
		return
	}
	m.city = to
	m.mapCursor = m.yourCorner()
	m.say(fmt.Sprintf("You are in %s. Your stock stayed where it was; post yourself on a corner %s.", m.w.CityName(to), screenPointer(screenMap)))
}

func (m *Model) travelConfirm() string {
	to := m.travelTo()
	body := []string{fmt.Sprintf("Leave %s for %s today?", m.w.Here().Name, m.w.CityName(to)), ""}
	if c := m.w.PostOf(game.You); c != nil {
		body = append(body, theme.Warning.Render(fmt.Sprintf("You step off %s: back to the street in %dd", c.Name, m.driftLeft(c))), theme.Warning.Render("unless a runner takes it."))
	} else {
		body = append(body, theme.Subtle.Render("You stand on no corner here to leave."))
	}
	for _, l := range []string{
		"Stock stays where it is; the routes " + screenPointer(screenMap) + " move it.",
		"The supplier sells to you where you stand. Go for what",
		"only you can do there: stand on a corner, hire, ask around.",
	} {
		body = append(body, theme.Subtle.Render(l))
	}
	return m.modal("GO TO "+m.w.CityName(to)+"?", body, m.modalFooter())
}
