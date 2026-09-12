package ui

import (
	"fmt"
	"strings"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// cityTabs is the city selector on the market and map screens: the
// cities in order, the one shown in brackets and Selected, with a mark
// on the one you are in (`[ ◉ Eastside ]  Bayport`).
func (m *Model) cityTabs() string {
	var parts []string
	for _, id := range m.w.CityOrder {
		c := m.w.Cities[id]
		label := c.Name
		if id == m.w.Player.Location {
			label = "◉ " + label
		}
		if id == m.city {
			parts = append(parts, theme.Selected.Render("[ "+label+" ]"))
		} else {
			parts = append(parts, theme.Subtle.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

// screenTitle is a screen's title line: the name and the city shown
// (`MARKET · Eastside`), then the city tabs.
func (m *Model) screenTitle(name string) string {
	return theme.PanelTitle.Render(name+" · "+m.shown().Name) + "   " + m.cityTabs()
}

// productRows is the product table the dashboard and the market share:
// the price, its change on the day, the sparkline (sized by sparkCol
// once the caller knows what the width leaves), the stock in the city
// and the order queued there, which is your standing order (#114, ↻)
// or the lieutenant's where you placed none; the market adds the supplier's price, the
// demand your corners there serve and the level a supply contract
// keeps the stash at (#113), and calls the stock the stash it is
// (`product price Δ Nd supplier stash demand/day order keep`). The
// cursor is the row of the product selected, or -1 when it is not in
// the city.
func (m *Model) productRows(city string, selected int, market bool) (cols []col, rows [][]any, cursor int) {
	w := m.w
	c := w.City(city)
	cols = []col{{"product", kText, 0}, {"price", kPrice, 0}, {"Δ", kPct, 0}, {"", kBar, 0}}
	if market {
		cols = append(cols, col{"supplier", kPrice, 0}, col{"stash", kInt, 0}, col{"demand/day", kInt, 0})
	} else {
		cols = append(cols, col{"stock", kInt, 0})
	}
	cols = append(cols, col{"order", kDial, 0})
	if market {
		cols = append(cols, col{"keep", kInt, 0})
	}
	cursor = -1
	for i, id := range w.Products {
		p := c.Market[id]
		if p == nil {
			continue
		}
		if i == selected {
			cursor = len(rows)
		}
		delta := 0.0
		if n := len(p.History); n >= 2 {
			delta = pct(p.History[n-2], p.History[n-1])
		}
		var ds any = styled{theme.Subtle, signed{delta}}
		if delta > 1 {
			ds = styled{theme.Good, signed{delta}}
		} else if delta < -1 {
			ds = styled{theme.Bad, signed{delta}}
		}
		sp := spark{vs: p.History}
		if p.ShockDays > 0 {
			if p.ShockSlump {
				sp.mark = theme.Warning.Render("▼")
			} else {
				sp.mark = theme.Good.Render("▲")
			}
		}
		var ord any
		if o, ok := w.Order(city, id); ok {
			ord = styled{theme.Gold, order{qty: o.Qty, dial: dialShort(o.Dial)}}
		} else if o, ok := w.YourStanding(city, id); ok {
			ord = styled{theme.Gold, order{qty: o.Qty, dial: dialShort(o.Dial), standing: true}}
		} else if o, ok := w.DelegatedOrder(city, id); ok {
			ord = styled{theme.CrewText, order{qty: o.Qty, dial: dialShort(o.Dial), lt: true}}
		}
		row := []any{p.Name, p.Price, ds, styled{theme.Good, sp}}
		if market {
			var supplier any = p.SupplierPrice
			if p.NoSupply {
				supplier = nil
			}
			row = append(row, supplier)
		}
		row = append(row, w.Stock(city, id))
		if market {
			row = append(row, approx{w.Demand(city, id)})
		}
		row = append(row, ord)
		if market {
			// The supply contract's level (#113), `-` with none.
			var keep any
			if c, ok := w.Supplied(city, id); ok {
				keep = styled{theme.Gold, c.Units}
			}
			row = append(row, keep)
		}
		rows = append(rows, row)
	}
	return cols, rows, cursor
}

// sparkCol sizes the product table's sparkline to width cells, or the
// days of history there are to draw, and titles it by those days.
func sparkCol(cols []col, rows [][]any, width int) {
	days := 0
	for i := range cols {
		if cols[i].kind != kBar {
			continue
		}
		for _, r := range rows {
			if sp, ok := r[i].(styled); ok {
				if x, ok := sp.v.(spark); ok {
					days = max(days, len(x.vs))
				}
			}
		}
		// No wider than the history it has to draw, so the mark
		// after a short series sits by it.
		cols[i].width = max(3, min(days, width))
		cols[i].title = fmt.Sprintf("%dd", min(days, width))
	}
}

// viewMarket is the market's MAIN (#84): the title with the city tabs,
// the product table for the city shown, a blank, and the buyers there.
// Nothing else: the product's detail and the notes are the pane's.
func (m *Model) viewMarket() string {
	city := m.shown()
	width := m.mainWidth()
	var b strings.Builder
	b.WriteString(truncate(m.screenTitle("MARKET"), width) + "\n\n")
	cols, rows, cursor := m.productRows(city.ID, m.cursor, true)
	sparkCol(cols, rows, max(3, min(30, width-tableWidth(cols, rows))))
	for _, l := range table(cols, rows, cursor, width) {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
	// The buyers (#71): the reason to visit. Their rows sit under the
	// table; the detail of the product or the contract under the cursor
	// is the pane's.
	for _, l := range m.buyersLines() {
		b.WriteString(l + "\n")
	}
	return b.String()
}

// marketDetails is the market's pane (#84): the product under the
// cursor in the city shown (`WEED · EASTSIDE`: its range, glut, margin,
// the demand your corners there serve and a standard corner's, and a
// shock while one runs), ELSEWHERE (the other city's street and
// supplier price, your stash there, what is on the road to it), the
// NOTES on where you are and the wholesaler, and the keys; the
// contract's terms instead while the cursor is on the buyers.
func (m *Model) marketDetails() []section {
	w := m.w
	city := m.shown()
	here := city.ID == w.Player.Location
	if c := m.selectedContract(); c != nil {
		return m.contractSections(*c)
	}
	if m.cursor >= len(w.Products) {
		return nil
	}
	id := w.Products[m.cursor]
	p := city.Market[id]
	if p == nil {
		return nil
	}
	lo, hi := p.Price, p.Price
	for _, v := range p.History {
		lo = min(lo, v)
		hi = min(max(hi, v), 1e9)
	}
	sel := []string{
		row("range 30d", fmt.Sprintf("%s – %s", price(lo), price(hi))),
		row("glut", fmt.Sprintf("%.0f%%", p.Glut*100)),
	}
	margin := 0.0
	if p.SupplierPrice > 0 {
		margin = (p.Price - p.SupplierPrice) / p.SupplierPrice * 100
	}
	if p.NoSupply {
		sel = append(sel, row("supplier", theme.Warning.Render("not sold here")))
	} else {
		sel = append(sel, row("margin", fmt.Sprintf("%.0f%% over supplier", margin)))
	}
	sel = append(sel,
		row("demand", fmt.Sprintf("~%.0f/day on %s", w.Demand(city.ID, id), plural(w.WorkedIn(city.ID), "corner"))),
		row("", theme.Subtle.Render(fmt.Sprintf("~%.0f per standard", p.Demand))))
	if p.ShockDays > 0 {
		if p.ShockSlump {
			sel = append(sel, row("slump", theme.Warning.Render(fmt.Sprintf("×%.2f, %s more", p.ShockFactor, plural(p.ShockDays, "day")))))
		} else {
			sel = append(sel, row("shock", theme.Good.Render(fmt.Sprintf("×%.2f, %s more", p.ShockFactor, plural(p.ShockDays, "day")))))
		}
	}
	sel = append(sel, m.standingRows(city.ID, id)...)
	sel = append(sel, m.contractRows(city.ID, id)...)
	if len(m.buyerRows()) > 0 {
		sel = append(sel, keyRow("↓", "past the table reaches the buyers"))
	}
	secs := append(m.cartSection(city.ID), section{strings.ToUpper(p.Name) + " · " + strings.ToUpper(city.Name), sel}) // the cart first, so the strip carries its totals (#103)
	// The other city's price is what a route is worth.
	var elsewhere []string
	for _, cid := range w.CityOrder {
		if cid == city.ID {
			continue
		}
		if o := w.Product(cid, id); o != nil {
			elsewhere = append(elsewhere, row(w.CityName(cid), fmt.Sprintf("%s · sup %s", price(o.Price), price(o.SupplierPrice))))
		}
		if q := w.Stock(cid, id); q > 0 {
			elsewhere = append(elsewhere, row("stash", fmt.Sprintf("%d in %s", q, w.CityName(cid))))
		}
	}
	for _, sh := range w.Shipments {
		if sh.Product == id {
			elsewhere = append(elsewhere, row("road", theme.RoadText.Render(fmt.Sprintf("%d → %s, %dd", sh.Units, w.CityName(sh.To), sh.DaysLeft(w.Day)))))
		}
	}
	if len(elsewhere) > 0 {
		secs = append(secs, section{"ELSEWHERE", elsewhere})
	}
	var notes []string
	if p.NoSupply {
		notes = append(notes, wrapped(theme.Warning, "Not sold here: it comes in by the road (the map's routes) or in your pockets.")...)
	}
	if !here {
		notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("You are in %s: the supplier here sells to you there, not here. Runners sell what is stashed here.", w.Here().Name))...)
	}
	if city.Wholesale {
		o := m.set.Logistics.Wholesale(w)
		if o.Locked(w) {
			notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("The supplier here sells lots of %d at %.0f%% to the routes once you have moved %s.", o.Lot, o.Mul*100, cash(o.UnlockCash)))...)
		} else {
			notes = append(notes, wrapped(theme.Good, fmt.Sprintf("Wholesale: lots of %d at %.0f%% of the supplier price feed the routes out of here, run %s.", o.Lot, o.Mul*100, screenPointer(screenMap)))...)
		}
	}
	if len(notes) > 0 {
		secs = append(secs, section{"NOTES", notes})
	}
	return secs
}

// contractRows is the pane's rows on a product's supply contract
// (#113): `contract  keep at 120`, what it bought this morning (`bought
// 40 today for $1,900`), what it will bring in the morning and what x
// does to it; nothing with none.
func (m *Model) contractRows(city, id string) []string {
	w := m.w
	c, ok := w.Supplied(city, id)
	if !ok {
		return nil
	}
	rows := []string{row("contract", theme.Gold.Render(fmt.Sprintf("keep at %d", c.Units)))}
	bought, cost := 0, 0
	for _, b := range w.Buys {
		if b.Contract && b.City == city && b.Product == id {
			bought += b.Qty
			cost += b.Cost
		}
	}
	if bought > 0 {
		rows = append(rows, row("", fmt.Sprintf("bought %d today for %s", bought, money(cost))))
	}
	if due := m.set.Market.Due(w, city, id); due > 0 {
		rows = append(rows, row("", theme.Subtle.Render(fmt.Sprintf("brings %d in the morning at %s", due, price(m.set.Market.SupplyPrice(w, city, id))))))
	}
	if _, ok := w.Order(city, id); ok {
		return rows
	}
	if _, ok := w.YourStanding(city, id); !ok {
		rows = append(rows, keyRow("x", "clear the contract"))
	}
	return rows
}

// standingRows is the pane's rows on a product's standing order (#114):
// `standing  120 aggr. · cut 5%` (the dial short, so the row fits the
// pane) and, with no order of the day to cancel first, what x does to
// it; nothing with none.
func (m *Model) standingRows(city, id string) []string {
	w := m.w
	o, ok := w.YourStanding(city, id)
	if !ok {
		return nil
	}
	rows := []string{row("standing", theme.Gold.Render(fmt.Sprintf("%d %s", o.Qty, dialShort(o.Dial)))+sep+fmt.Sprintf("cut %.0f%%", m.set.Market.Cut()*100))}
	if _, ok := w.Order(city, id); !ok {
		rows = append(rows, keyRow("x", "cancel the standing order"))
	}
	return rows
}
