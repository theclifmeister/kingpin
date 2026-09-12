package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The cart (#103) is the day's shopping in one place: every buy made
// today, what the supply contracts bought this morning (#113, lines
// marked contract, returnable like a buy), every sell order queued and
// every standing order that sells tonight (#114, lines marked standing,
// edited like an order), with the totals, readable in the
// buy and sell dialogs and the dashboard's and the market's pane and
// editable in the cart modal (modeCart, one modal like every other):
// a line's quantity, its dial, or the line itself. Removing or
// shrinking a buy is a return (World.Return, the exact inverse of Buy);
// removing an order is CancelSell, a standing one CancelStanding. The
// cart is UI over the world's per-day scratch (w.Buys, w.Orders) and
// its standing orders (w.Standing): nothing new is saved.

// cartLine is one line of the cart: a buy made today, merged per city
// and product, what a supply contract bought this morning (#113: a buy
// marked contract, merged the same way and kept apart from the buys by
// hand), a sell order queued, or a standing order that sells tonight
// (#114: one you placed no order of the day over).
type cartLine struct {
	buy      bool
	contract bool // a buy the supply contract made this morning
	credit   bool // a buy on a connect's book (#72), kept apart from the cash ones
	standing bool // a standing order of yours, selling tonight
	city     string
	product  string
	qty      int
	unit     float64     // a buy: the price paid a unit
	cost     int         // a buy: what it cost
	dial     events.Dial // an order: its dial
	take     int         // an order: the take expected
	heat     float64     // an order: the heat expected
}

// cartDialog is the state of the cart modal: the line under the cursor
// and, on its second step, the quantity being typed for it.
type cartDialog struct {
	cursor int
	step   int // 0 the lines, 1 a quantity for the selected one
	qty    numberField
	err    string
}

// cartLines is the cart: the buys in the order they were made, the
// contracts' first since they were made first, then the orders in city
// and ladder order, a standing order standing in for the order of the
// day where you placed none (and never on a lie-low day: nothing sells).
func (m *Model) cartLines() []cartLine {
	w := m.w
	var lines []cartLine
	at := map[string]int{}
	for _, b := range w.Today.Buys {
		k := game.OrderKey(b.City, b.Product)
		switch {
		case b.Contract:
			k = "contract " + k
		case b.Credit:
			k = "credit " + k
		}
		if i, ok := at[k]; ok {
			lines[i].qty += b.Qty
			lines[i].cost += b.Cost
			continue
		}
		at[k] = len(lines)
		lines = append(lines, cartLine{buy: true, contract: b.Contract, credit: b.Credit, city: b.City, product: b.Product, qty: b.Qty, cost: b.Cost})
	}
	for i := range lines {
		if lines[i].qty > 0 {
			lines[i].unit = float64(lines[i].cost) / float64(lines[i].qty)
		}
	}
	for _, cid := range w.CityOrder {
		for _, pid := range w.Products {
			o, ok := w.Order(cid, pid)
			standing := false
			if !ok && !w.Today.LieLow {
				o, ok = w.YourStanding(cid, pid)
				standing = true
			}
			if !ok {
				continue
			}
			_, take, heat := m.orderEstimate(o, standing)
			lines = append(lines, cartLine{standing: standing, city: cid, product: pid, qty: o.Qty, dial: o.Dial, take: take, heat: heat})
		}
	}
	return lines
}

// orderEstimate is what an order is expected to move, take and cost in
// heat: the sell dialog's expect and heat rows for it. A standing order
// (#114) sells at most what is stashed (and what the contract brings),
// and its take is after the crew's cut.
func (m *Model) orderEstimate(o game.SellOrder, standing bool) (units, take int, heat float64) {
	p := m.w.Product(o.City, o.Product)
	if p == nil {
		return 0, 0, 0
	}
	qty := o.Qty
	cut := 0.0
	if standing {
		qty = min(qty, m.sellable(o.City, o.Product))
		cut = m.set.Market.Cut()
	}
	units = min(qty, m.set.Market.Capacity(m.w, o.City, o.Product, o.Dial))
	take = int(float64(units) * p.Price * m.set.Market.Dial(o.Dial).Price * (1 - cut))
	return units, take, m.estHeat(o.City, o.Product, qty, o.Dial)
}

// cartTotals is the bottom line: the lines bought and what they cost
// (the contracts' among them, counted again on their own), the lines
// queued (the standing among them, counted again), what they are
// expected to take and the heat.
type cartTotals struct {
	buys, sells         int
	spent, take         int
	contracts, supplied int // the contract lines and what they cost, part of buys and spent
	credits, booked     int // the credit lines (#72) and what went on the book, part of buys and not of spent
	standing            int // the standing lines, part of sells
	heat                float64
}

func totals(lines []cartLine) cartTotals {
	var t cartTotals
	for _, l := range lines {
		if l.buy {
			t.buys++
			switch {
			case l.credit:
				t.credits++
				t.booked += l.cost
			case l.contract:
				t.spent += l.cost
				t.contracts++
				t.supplied += l.cost
			default:
				t.spent += l.cost
			}
		} else {
			t.sells++
			t.take += l.take
			t.heat += l.heat
			if l.standing {
				t.standing++
			}
		}
	}
	return t
}

// cartSummary is the cart in a sentence, the END THE DAY? modal's:
// `Buying 3 lines for $12K (2 by contract), selling 2 lines (1
// standing), ~$30K, +4.2 heat.`; empty for an empty cart.
func (m *Model) cartSummary() string {
	t := totals(m.cartLines())
	var parts []string
	if t.buys > 0 {
		line := fmt.Sprintf("buying %s for %s", plural(t.buys, "line"), cash(t.spent+t.booked))
		switch {
		case t.contracts > 0 && t.credits > 0:
			line += fmt.Sprintf(" (%d by contract, %d on credit)", t.contracts, t.credits)
		case t.contracts > 0:
			line += fmt.Sprintf(" (%d by contract)", t.contracts)
		case t.credits > 0:
			line += fmt.Sprintf(" (%d on credit)", t.credits)
		}
		parts = append(parts, line)
	}
	if t.sells > 0 {
		line := fmt.Sprintf("selling %s", plural(t.sells, "line"))
		if t.standing > 0 {
			line += fmt.Sprintf(" (%d standing)", t.standing)
		}
		parts = append(parts, line+fmt.Sprintf(", ~%s, %+.1f heat", cash(t.take), t.heat))
	}
	if len(parts) == 0 {
		return ""
	}
	return capitalize(strings.Join(parts, ", ")) + "."
}

// cartCols is the CART table: what the line is, the product, the city,
// the units, a buy's price a unit, an order's street price change on
// the day (#138, the market table's Δ; a buy's is blank, as its dial
// and heat are), an order's dial, the cash (a buy's cost, an order's
// expected take) and an order's heat.
var cartCols = []col{{"line", kText, 0}, {"product", kText, 0}, {"city", kText, 0}, {"units", kInt, 0}, {"price", kPrice, 0}, {"Δ", kPct, 0}, {"dial", kDial, 0}, {"cash", kMoney, 0}, {"heat", kText, 0}}

// cartRows are the cart's lines as CART table rows.
func (m *Model) cartRows(lines []cartLine) [][]any {
	var rows [][]any
	for _, l := range lines {
		name, city := m.w.ProductName(l.product), m.w.CityName(l.city)
		switch {
		case l.contract:
			rows = append(rows, []any{"contract", name, city, l.qty, l.unit, nil, nil, styled{theme.Bad, -l.cost}, nil})
		case l.credit:
			rows = append(rows, []any{"credit", name, city, l.qty, l.unit, nil, nil, styled{theme.Warning, -l.cost}, nil})
		case l.buy:
			rows = append(rows, []any{"buy", name, city, l.qty, l.unit, nil, nil, styled{theme.Bad, -l.cost}, nil})
		default:
			kind := "sell"
			if l.standing {
				kind = "standing"
			}
			var delta any
			if f := m.priceFacts(l.city, l.product); f != nil {
				delta = f.deltaCell()
			}
			rows = append(rows, []any{kind, name, city, l.qty, nil, delta, dialShort(l.dial), styled{theme.Gold, l.take}, styled{heatStyle(m.w.City(l.city).Heat + l.heat*4), fmt.Sprintf("%+.1f", l.heat)}})
		}
	}
	return rows
}

// cartTotalLine is the totals under the CART table: `spent $785 ·
// expect ~$1,200 · heat +4.2 · dirty cash $12,345`.
func (m *Model) cartTotalLine(t cartTotals) string {
	sub := theme.Subtle.Render
	var parts []string
	if t.buys > 0 {
		parts = append(parts, sub("spent ")+money(t.spent))
	}
	if t.contracts > 0 {
		parts = append(parts, sub("by contract ")+money(t.supplied))
	}
	if t.credits > 0 {
		parts = append(parts, sub("on credit ")+money(t.booked))
	}
	if t.sells > 0 {
		parts = append(parts, sub("expect ")+theme.Gold.Render("~"+money(t.take)), sub("heat ")+fmt.Sprintf("%+.1f", t.heat))
	}
	parts = append(parts, sub("dirty cash ")+money(m.w.Player.DirtyCash))
	return strings.Join(parts, sub(" · "))
}

// cartBlock is the CART block under a dialog's product table: the
// heading, the table with no cursor and the totals; nothing for an
// empty cart.
func (m *Model) cartBlock() []string {
	lines := m.cartLines()
	if len(lines) == 0 {
		return nil
	}
	body := []string{theme.Gold.Bold(true).Render("CART")}
	body = append(body, table(cartCols, m.cartRows(lines), -1, m.modalInner())...)
	return append(body, m.cartTotalLine(totals(lines)))
}

// cartSection is the pane's CART section for the dashboard and the
// market: the totals first, so the strip carries them, then one line a
// cart line, the city named where it is not the one the screen is
// about, and what c does; nil for an empty cart.
func (m *Model) cartSection(city string) []section {
	lines := m.cartLines()
	if len(lines) == 0 {
		return nil
	}
	t := totals(lines)
	var ls []string
	if t.buys > 0 {
		line := fmt.Sprintf("%s, %s", plural(t.buys, "line"), cash(t.spent))
		if t.contracts > 0 {
			line += sep + fmt.Sprintf("%d by contract", t.contracts)
		}
		ls = append(ls, row("buying", line))
	}
	if t.sells > 0 {
		line := fmt.Sprintf("%s, ~%s", plural(t.sells, "line"), cash(t.take))
		if t.standing > 0 {
			line += sep + fmt.Sprintf("%d standing", t.standing)
		}
		ls = append(ls, row("selling", line), row("heat", fmt.Sprintf("%+.1f expected", t.heat)))
	}
	for _, l := range lines {
		what := fmt.Sprintf("%d %s", l.qty, m.w.ProductName(l.product))
		switch {
		case l.contract:
			ls = append(ls, row("contract", what+" "+money(l.cost)))
		case l.buy:
			ls = append(ls, row("buy", what+" "+money(l.cost)))
		case l.standing:
			ls = append(ls, row("standing", what+" "+dialShort(l.dial)+" ~"+cash(l.take)))
		default:
			ls = append(ls, row("sell", what+" "+dialShort(l.dial)+" ~"+cash(l.take)))
		}
		if l.city != city {
			ls = append(ls, row("", theme.Subtle.Render("in "+m.w.CityName(l.city))))
		}
	}
	ls = append(ls, keyRow("c", "edit: quantity, dial, remove"))
	return []section{{"CART", ls}}
}

// openCart opens the cart modal on its first line.
func (m *Model) openCart() {
	if m.w.Over != nil {
		return
	}
	m.crt = cartDialog{qty: newNumberField("blank = max")}
	m.mode = modeCart
}

// cartSelected is the line under the cart's cursor, clamped to the
// lines there are; nil for an empty cart.
func (m *Model) cartSelected() *cartLine {
	lines := m.cartLines()
	if len(lines) == 0 {
		return nil
	}
	m.crt.cursor = max(0, min(m.crt.cursor, len(lines)-1))
	return &lines[m.crt.cursor]
}

// cartHasLines and cartOnSell are the cart footer's conditions: a line
// to edit, and an order under the cursor for the dial keys.
func cartHasLines(m *Model) bool { return m.modalStep() == 0 && m.cartSelected() != nil }

func cartOnSell(m *Model) bool {
	l := m.cartSelected()
	return m.modalStep() == 0 && l != nil && !l.buy
}

func (m *Model) keyCart(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.crt
	d.err = ""
	// Back is one key and close is one key (#110): esc closes from
	// either step, shift+tab returns the quantity step to the line list
	// (the line stays under the cursor) and is silent there, tab opens
	// the quantity for the line under the cursor and is silent on it.
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "shift+tab":
		if d.step == 1 {
			d.step = 0
			d.qty.Blur()
		}
		return m, nil
	case "tab":
		if d.step == 1 {
			return m, nil
		}
	case "q":
		if d.step == 0 {
			m.mode = modePlay
			return m, nil
		}
	}
	if d.step == 1 {
		if key == "enter" {
			m.setCartQty()
			return m, nil
		}
		d.qty.max = m.cartMax()
		return m, d.qty.Update(k)
	}
	l := m.cartSelected()
	switch key {
	case "up", "k":
		if d.cursor > 0 {
			d.cursor--
		}
	case "down", "j":
		if l != nil && d.cursor < len(m.cartLines())-1 {
			d.cursor++
		}
	case "enter", "tab":
		if l == nil {
			return m, nil
		}
		d.step = 1
		d.qty.Set(l.qty)
		return m, d.qty.Focus()
	case "x":
		if l != nil {
			m.removeCartLine(*l)
		}
	case "left", "h", "right", "l", "1", "2", "3":
		if l == nil || l.buy {
			return m, nil
		}
		dial := l.dial
		switch key {
		case "left", "h":
			if dial > events.DialQuiet {
				dial--
			}
		case "right", "l":
			if dial < events.DialAggressive {
				dial++
			}
		default:
			dial = events.Dial(key[0] - '1')
		}
		if dial != l.dial {
			if err := m.placeLine(*l, l.qty, dial); err != nil {
				d.err = dialogError(err)
			}
		}
	}
	return m, nil
}

// placeLine places an order line again at a quantity and a dial: a
// standing line through PlaceStanding, an order of the day through
// PlaceSell.
func (m *Model) placeLine(l cartLine, qty int, dial events.Dial) error {
	if l.standing {
		return m.w.PlaceStanding(l.city, l.product, qty, dial)
	}
	return m.w.PlaceSell(l.city, l.product, qty, dial)
}

// cartMax is what the cart's quantity can take for the line under the
// cursor: an order, standing or of the day, the stash of the product in its city; a buy, what
// was bought today plus what the supplier would sell you more where you
// stand (a return down to nothing is x). Blank has always meant it.
func (m *Model) cartMax() int {
	l := m.cartSelected()
	switch {
	case l == nil:
		return 0
	case !l.buy:
		return m.sellable(l.city, l.product)
	case m.w.CanBuyIn(l.city) && !l.contract:
		// More where you stand, or through the lieutenant who runs
		// the city (#174), from the cheapest connect selling it.
		return l.qty + m.maxBuyFrom(m.w.BestSupplier(l.city, l.product), l.product, false)
	}
	return l.qty
}

// giveBack is the cart returning units of a buy line: a contract's
// through ReturnSupplied, a buy by hand's through Return.
func (m *Model) giveBack(l cartLine, qty int) (int, error) {
	switch {
	case l.contract:
		return m.w.ReturnSupplied(l.city, l.product, qty)
	case l.credit:
		return m.w.ReturnCredit(l.city, l.product, qty)
	}
	return m.w.Return(l.city, l.product, qty)
}

// returned is the status after a return: what came back to the till,
// or, for a credit line, what came off the book (#72).
func (m *Model) returned(l cartLine, qty, refund int) string {
	if l.credit {
		return fmt.Sprintf("Returned %d %s: off the book.", qty, m.w.ProductName(l.product))
	}
	return fmt.Sprintf("Returned %d %s, %s back.", qty, m.w.ProductName(l.product), money(refund))
}

// setCartQty is enter on the cart's quantity step: an order, standing
// or of the day, is placed again at the new quantity; a buy is returned
// down to it, or added to from the supplier where you stand.
func (m *Model) setCartQty() {
	d := &m.crt
	l := m.cartSelected()
	if l == nil {
		d.step = 0
		return
	}
	if l.buy {
		here := m.w.CanBuyIn(l.city) // where you stand, or through a lieutenant (#174)
		qty, err := parseQtyInput(d.qty.Value(), m.cartMax())
		if err != nil {
			d.err = dialogError(err)
			return
		}
		switch {
		case qty < l.qty:
			refund, err := m.giveBack(*l, l.qty-qty)
			if err != nil {
				d.err = dialogError(err)
				m.refuse("Can't return: " + err.Error())
				return
			}
			m.say(m.returned(*l, l.qty-qty, refund))
		case qty > l.qty && l.contract:
			d.err = "A contract's line only goes back: the supplier sells you more by hand."
			return
		case qty > l.qty && l.credit:
			d.err = "A credit line only goes back: more on the book is a new buy."
			return
		case qty > l.qty && !here:
			d.err = fmt.Sprintf("You are in %s: it was bought in %s, and nobody runs it for you.", m.w.CityName(m.w.Player.Location), m.w.CityName(l.city))
			return
		case qty > l.qty:
			// More by hand: from the cheapest connect that sells it
			// today, for cash (#72).
			sup := m.w.BestSupplier(l.city, l.product)
			if sup == nil {
				d.err = fmt.Sprintf("Nobody in %s sells %s today.", m.w.CityName(l.city), m.w.ProductName(l.product))
				return
			}
			p, err := m.w.Buy(sup.ID, l.product, qty-l.qty, false, m.set.Market.BuyPressure(m.w))
			if err != nil {
				d.err = dialogError(err)
				return
			}
			from := sup.Name
			if p.Lieutenant != "" {
				from += " through " + p.Lieutenant
			}
			m.say(fmt.Sprintf("Bought %d more %s from %s for %s.", p.Qty, m.w.ProductName(l.product), from, money(p.Cost)))
		}
	} else {
		qty, err := parseQtyInput(d.qty.Value(), m.cartMax())
		if err != nil {
			d.err = dialogError(err)
			return
		}
		if err := m.placeLine(*l, qty, l.dial); err != nil {
			d.err = dialogError(err)
			return
		}
		if l.standing {
			m.say(fmt.Sprintf("Standing: %d %s in %s, %s, every night.", qty, m.w.ProductName(l.product), m.w.CityName(l.city), l.dial))
		} else {
			m.say(fmt.Sprintf("Queued %d %s in %s, %s.", qty, m.w.ProductName(l.product), m.w.CityName(l.city), l.dial))
		}
	}
	d.step = 0
	d.qty.Blur()
}

// removeCartLine is x on a line: an order is cancelled (a standing one
// for good, #114), a buy returned whole. A buy whose units have left
// the stash stays, with the refusal in the modal and the status bar.
func (m *Model) removeCartLine(l cartLine) {
	if l.standing {
		m.w.CancelStanding(l.city, l.product)
		m.say("Standing order cancelled.")
		return
	}
	if !l.buy {
		m.w.CancelSell(l.city, l.product)
		m.say("Order cancelled.")
		return
	}
	refund, err := m.giveBack(l, l.qty)
	if err != nil {
		m.crt.err = dialogError(err)
		m.refuse("Can't return: " + err.Error())
		return
	}
	m.say(m.returned(l, l.qty, refund))
}

// viewCart is the cart modal: the CART table under the cursor, the
// totals, the selected line's price sentence (#138) and on the second
// step the quantity for it.
func (m *Model) viewCart() string {
	d := m.crt
	lines := m.cartLines()
	var body []string
	if len(lines) == 0 {
		body = append(body, theme.Subtle.Render("Nothing in the cart."))
		return m.modal("CART", body, m.modalFooter())
	}
	cursor := max(0, min(d.cursor, len(lines)-1))
	m.modalFollow(1 + cursor) // under the header
	body = append(body, table(cartCols, m.cartRows(lines), cursor, m.modalInner())...)
	body = append(body, "", m.cartTotalLine(totals(lines)), "")
	l := lines[cursor]
	if d.step == 1 {
		d.qty.max = m.cartMax()
		line := "quantity   " + d.qty.View()
		if l.buy {
			note := fmt.Sprintf("bought %d", l.qty)
			if l.contract {
				note += " by contract"
			} else if l.city == m.w.Player.Location {
				note += fmt.Sprintf(", up to %d more", m.maxBuy(l.product))
			}
			line += "   " + theme.Subtle.Render(note)
		}
		body = append(body, line)
	}
	// The selected line's price sentence, the dialog's (#138): the
	// number the line was decided on, under the eye while it is edited.
	body = append(body, m.priceRows(l.city, l.product, l.buy)...)
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal("CART", body, m.modalFooter())
}

// endDayLine is the END THE DAY? modal's first sentence: lying low, the
// cart in a sentence, or nothing queued.
func (m *Model) endDayLine() string {
	switch {
	case m.w.Today.LieLow:
		return "Lying low today."
	case len(m.cartLines()) > 0:
		return m.cartSummary()
	}
	return "No sales queued."
}
