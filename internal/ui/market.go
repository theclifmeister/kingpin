package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
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
// keeps the stash at (#113), and calls the stock the stash it is,
// with the lot's quality beside it (#47, `qual`: blank with nothing
// held, Warning under the default)
// (`product price Δ Nd supplier stash qual demand/day order keep`). The
// cursor is the row of the product selected, or -1 when it is not in
// the city.
func (m *Model) productRows(city string, selected int, market bool) (cols []col, rows [][]any, cursor int) {
	w := m.w
	c := w.City(city)
	cols = []col{{"product", kText, 0}, {"price", kPrice, 0}, {"Δ", kPct, 0}, {"", kBar, 0}}
	if market {
		cols = append(cols, col{"supplier", kPrice, 0}, col{"stash", kInt, 0}, col{"qual", kInt, 0}, col{"demand/day", kInt, 0})
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
		f := facts(p)
		var ord any
		if o, ok := w.Order(city, id); ok {
			ord = styled{theme.Gold, order{qty: o.Qty, dial: dialShort(o.Dial)}}
		} else if o, ok := w.YourStanding(city, id); ok {
			ord = styled{theme.Gold, order{qty: o.Qty, dial: dialShort(o.Dial), standing: true}}
		} else if o, ok := w.DelegatedOrder(city, id); ok {
			ord = styled{theme.CrewText, order{qty: o.Qty, dial: dialShort(o.Dial), lt: true}}
		}
		row := []any{p.Name, p.Price, f.deltaCell(), f.sparkCell()}
		if market {
			var supplier any = p.SupplierPrice
			if p.NoSupply {
				supplier = nil
			}
			row = append(row, supplier)
		}
		row = append(row, w.Stock(city, id))
		if market {
			row = append(row, qualityCell(w, city, id), approx{w.Demand(city, id)})
		}
		row = append(row, ord)
		if market {
			// The supply contract's level (#113), `-` with none; the
			// lieutenant's where you set none (#174), `120 (lt)` in
			// CrewText as the order column reads theirs.
			var keep any
			if c, ok := w.Supplied(city, id); ok {
				keep = styled{theme.Gold, level{units: c.Units}}
			} else if c, ok := w.DelegatedSupplied(city, id); ok {
				keep = styled{theme.CrewText, level{units: c.Units, lt: true}}
			}
			row = append(row, keep)
		}
		rows = append(rows, row)
	}
	return cols, rows, cursor
}

// dropCol takes the named column out of a table's columns and rows.
func dropCol(cols []col, rows [][]any, title string) ([]col, [][]any) {
	at := -1
	for i, c := range cols {
		if c.title == title {
			at = i
		}
	}
	if at < 0 {
		return cols, rows
	}
	cols = append(append([]col(nil), cols[:at]...), cols[at+1:]...)
	out := make([][]any, len(rows))
	for r, row := range rows {
		if at < len(row) {
			out[r] = append(append([]any(nil), row[:at]...), row[at+1:]...)
		} else {
			out[r] = row
		}
	}
	return cols, out
}

// priceFacts is what a product's price is doing in a city, the numbers
// the market table, the dashboard's and the market's pane, the buy and
// sell dialogs and the cart all read (#138), so the decision made in a
// dialog is made on the number the market shows and the two cannot
// disagree: the price, its change on the day (yesterday's close to
// today's off History), the range of the history, the supplier's price
// and the margin over it, and the shock or slump while one runs.
type priceFacts struct {
	p       *game.ProductMarket
	unit    float64 // the supplier price the facts are read against: the market's (the best available connect's), or the chosen connect's (#72); 0 where nobody sells it
	delta   float64 // yesterday → today, in percent
	lo, hi  float64 // the range of the history
	margin  float64 // the street over the supplier, in percent
	base    float64 // the connect's own price where unit carries a markup (#174); 0 otherwise
	markup  float64 // the markup a buy through a lieutenant pays (#174); 0 where none
	through string  // the lieutenant the buy goes through (#174); empty where none
}

// facts reads a product market's price facts against the market's
// supplier price.
func facts(p *game.ProductMarket) priceFacts { return factsAt(p, p.SupplierPrice) }

// factsAt reads a product market's price facts against a supplier
// price of the caller's: the buy dialog's connect (#72), whose price
// and margin are what the buy pays and makes, or 0 where they do not
// deal in it.
func factsAt(p *game.ProductMarket, unit float64) priceFacts {
	f := priceFacts{p: p, unit: unit, lo: p.Price, hi: p.Price}
	if n := len(p.History); n >= 2 {
		f.delta = pct(p.History[n-2], p.History[n-1])
	}
	for _, v := range p.History {
		f.lo = min(f.lo, v)
		f.hi = min(max(f.hi, v), 1e9)
	}
	if p.NoSupply {
		f.unit = 0
	}
	if f.unit > 0 {
		f.margin = (p.Price - f.unit) / f.unit * 100
	}
	return f
}

// priceFacts is facts for a product in a city; nil where the city has
// no market for it.
func (m *Model) priceFacts(city, id string) *priceFacts {
	p := m.w.Product(city, id)
	if p == nil {
		return nil
	}
	f := facts(p)
	return &f
}

// deltaStyle is the colour of the day's change: Good past +1%, Bad
// past -1%, Subtle between.
func (f priceFacts) deltaStyle() lipgloss.Style {
	switch {
	case f.delta > 1:
		return theme.Good
	case f.delta < -1:
		return theme.Bad
	}
	return theme.Subtle
}

// deltaCell is the Δ cell of a table: the change signed, in its colour.
func (f priceFacts) deltaCell() any { return styled{f.deltaStyle(), signed{f.delta}} }

// deltaText is the change as prose, `+6%`, in its colour.
func (f priceFacts) deltaText() string {
	return f.deltaStyle().Render(fmt.Sprintf("%+.0f%%", f.delta))
}

// sparkCell is the sparkline cell of a table over the history, marked
// ▲ or ▼ while a shock or a slump runs.
func (f priceFacts) sparkCell() any {
	sp := spark{vs: f.p.History}
	if f.p.ShockDays > 0 {
		if f.p.ShockSlump {
			sp.mark = theme.Warning.Render("▼")
		} else {
			sp.mark = theme.Good.Render("▲")
		}
	}
	return styled{theme.Good, sp}
}

// rangeText is the range of the history, `$30 – $45`.
func (f priceFacts) rangeText() string { return price(f.lo) + " – " + price(f.hi) }

// marginCell is the margin cell of a table: the street over the
// supplier, signed; nil where the supplier does not sell it.
func (f priceFacts) marginCell() any {
	if f.unit <= 0 {
		return nil
	}
	return signed{f.margin}
}

// marginText is the margin as prose, `+82%`.
func (f priceFacts) marginText() string { return fmt.Sprintf("%+.0f%%", f.margin) }

// shockRow is the shock or slump while one runs, as the pane names it:
// the label (`shock` / `slump`) and the value (`×1.40, 3 days more`) in
// its colour; empty with none.
func (f priceFacts) shockRow() (label, value string) {
	if f.p.ShockDays <= 0 {
		return "", ""
	}
	v := fmt.Sprintf("×%.2f, %s more", f.p.ShockFactor, plural(f.p.ShockDays, "day"))
	if f.p.ShockSlump {
		return "slump", theme.Warning.Render(v)
	}
	return "shock", theme.Good.Render(v)
}

// priceLine is the sentence under the product in the buy and sell
// dialogs' later steps and the cart's for its selected line (#138): a
// sale's `$38 · +6% today · range 30d $30 – $45`, a buy's `$21 · street
// $38 · margin +82%` (`not sold here · street $38` where the supplier
// does not sell it), in Subtle but for the change, which keeps its
// colour; empty where the city has no market for the product. The
// shock or slump while one runs is a row of its own (priceRows): with
// it on the line a heroin range runs past the modal.
func (m *Model) priceLine(city, id string, buy bool) string {
	f := m.priceFacts(city, id)
	if f == nil {
		return ""
	}
	if buy && m.mode == modeBuy {
		// The buy dialog's line is its connect's (#72): what this buy
		// pays and makes, not the best price in town.
		cf := m.connectFacts(city, id)
		f = &cf
	}
	sub := theme.Subtle.Render
	var parts []string
	if buy {
		switch {
		case f.p.NoSupply:
			parts = append(parts, theme.Warning.Render("not sold here"), sub("street "+price(f.p.Price)))
		case f.unit <= 0:
			parts = append(parts, theme.Warning.Render("not from them"), sub("street "+price(f.p.Price)))
		case f.through != "":
			// Through a lieutenant (#174): the connect's price, the
			// markup in words, and the margin over what the buy pays.
			parts = append(parts, sub(price(f.base)), sub(fmt.Sprintf("+%.0f%% through %s", (f.markup-1)*100, f.through)), sub("street "+price(f.p.Price)), sub("margin "+f.marginText()))
		default:
			parts = append(parts, sub(price(f.unit)), sub("street "+price(f.p.Price)), sub("margin "+f.marginText()))
		}
		// The connect's quality (#47): what this buy blends into the lot.
		if sup := m.buySupplier(id); sup != nil && sup.Sells(id) && !f.p.NoSupply {
			parts = append(parts, sub(fmt.Sprintf("quality %.0f", sup.QualityOf(m.w, id))))
		}
	} else {
		parts = append(parts, sub(price(f.p.Price)), f.deltaText()+sub(" today"), sub("range 30d "+f.rangeText()))
	}
	return strings.Join(parts, sep)
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
// the product table for the city shown, a blank, the buyers there, a
// blank, and the connects there (#72). Nothing else: the product's
// detail and the notes are the pane's.
func (m *Model) viewMarket() string {
	city := m.shown()
	width := m.mainWidth()
	var b strings.Builder
	b.WriteString(truncate(m.screenTitle("MARKET"), width) + "\n\n")
	cols, rows, cursor := m.productRows(city.ID, m.cursor, true)
	// The quality column (#47) goes where MAIN is too narrow for the
	// table whole: the pane beside it carries the lot's quality then.
	if tableWidth(cols, rows) > width {
		cols, rows = dropCol(cols, rows, "qual")
	}
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
	b.WriteString("\n")
	// The connects (#72): who sells here, at what, and where you stand
	// with them; the detail is the pane's.
	for _, l := range m.suppliersLines() {
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
	if sup := m.selectedSupplier(); sup != nil {
		return m.supplierSections(sup)
	}
	if m.cursor >= len(w.Products) {
		return nil
	}
	id := w.Products[m.cursor]
	p := city.Market[id]
	if p == nil {
		return nil
	}
	f := facts(p)
	sel := []string{
		row("range 30d", f.rangeText()),
		row("glut", fmt.Sprintf("%.0f%%", p.Glut*100)),
	}
	if p.NoSupply {
		sel = append(sel, row("supplier", theme.Warning.Render("not sold here")))
	} else {
		sel = append(sel, row("margin", f.marginText()+" over supplier"))
	}
	sel = append(sel,
		row("demand", fmt.Sprintf("~%.0f/day on %s", w.Demand(city.ID, id), plural(w.WorkedIn(city.ID), "corner"))),
		row("", theme.Subtle.Render(fmt.Sprintf("~%.0f per standard", p.Demand))))
	// The lot's quality (#47) and what it does to the price, where it
	// is not the default (the table's column has the figure; the pane
	// keeps its rows for what has changed); the connect's likewise.
	if l := w.Lot(city.ID, id); l.Units > 0 && l.Quality != w.StreetQuality() {
		v := fmt.Sprintf("%.0f, sells at ×%.2f", l.Quality, m.set.Market.QualityMul(l.Quality))
		if l.Quality < w.StreetQuality() {
			v = theme.Warning.Render(v)
		}
		sel = append(sel, row("quality", v))
	}
	if sup := w.BestSupplier(city.ID, id); sup != nil && sup.QualityOf(w, id) != w.StreetQuality() {
		sel = append(sel, row("", theme.Subtle.Render(fmt.Sprintf("%s sells it at %.0f", sup.Name, sup.QualityOf(w, id)))))
	}
	if n := w.Crew.Cooking(city.ID, id); n > 0 {
		sel = append(sel, row("cooking", fmt.Sprintf("%d on the way", n)))
	}
	if label, v := f.shockRow(); label != "" {
		sel = append(sel, row(label, v))
	}
	sel = append(sel, m.standingRows(city.ID, id)...)
	sel = append(sel, m.contractRows(city.ID, id)...)
	switch {
	case len(m.buyerRows()) > 0:
		sel = append(sel, keyRow("↓", "past the table reaches the buyers"))
	case len(m.supplierRows()) > 0:
		sel = append(sel, keyRow("↓", "past the table reaches the connects"))
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
	if lt := w.Crew.Lieutenant(city.ID); !here && lt != nil {
		notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("You are in %s: %s buys here for you at ×%.2f the connect's price, and keeps the stash stocked where you set no contract. Runners sell what is stashed here.", w.Here().Name, lt.Name, m.set.Market.Markup()))...)
	} else if !here {
		notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("You are in %s: the supplier here sells to you there, not here. Runners sell what is stashed here.", w.Here().Name))...)
	}
	if o := w.WholesaleSupplier(city.ID); o != nil {
		if o.Locked(w) {
			notes = append(notes, wrapped(theme.Subtle, fmt.Sprintf("%s sells lots of %d to the routes once you have moved %s.", o.Name, o.Lot, cash(o.UnlockCash)))...)
		} else {
			notes = append(notes, wrapped(theme.Good, fmt.Sprintf("Wholesale: %s's lots of %d feed the routes out of here, run %s.", o.Name, o.Lot, screenPointer(screenMap)))...)
		}
	}
	// The next product on the ladder and what it takes (#148): the
	// line named before it fires.
	if next := m.nextProductNote(city.ID); next != "" {
		notes = append(notes, wrapped(theme.Subtle, next)...)
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
	own := ok
	if !ok {
		// The lieutenant's contract (#174) where you set none: theirs
		// to keep, so x does nothing to it.
		if c, ok = w.DelegatedSupplied(city, id); !ok {
			return nil
		}
	}
	rows := []string{row("contract", theme.Gold.Render(fmt.Sprintf("keep at %d", c.Units)))}
	if !own {
		rows = []string{row("contract", theme.CrewText.Render(fmt.Sprintf("keep at %d (lt)", c.Units)))}
	}
	bought, cost := 0, 0
	for _, b := range w.Today.Buys {
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
	if _, ok := w.Order(city, id); ok || !own {
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
