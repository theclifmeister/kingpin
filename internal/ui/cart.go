package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The cart (#103) is the day's shopping in one place: every buy made
// today and every sell order queued, with the totals, readable in the
// buy and sell dialogs and the dashboard's and the market's pane and
// editable in the cart modal (modeCart, one modal like every other):
// a line's quantity, its dial, or the line itself. Removing or
// shrinking a buy is a return (World.Return, the exact inverse of Buy);
// removing an order is CancelSell. The cart is UI over the world's
// per-day scratch (w.Buys, w.Orders): nothing new is saved.

// cartLine is one line of the cart: a buy made today, merged per city
// and product, or a sell order queued.
type cartLine struct {
	buy     bool
	city    string
	product string
	qty     int
	unit    float64     // a buy: the price paid a unit
	cost    int         // a buy: what it cost
	dial    events.Dial // an order: its dial
	take    int         // an order: the take expected
	heat    float64     // an order: the heat expected
}

// cartDialog is the state of the cart modal: the line under the cursor
// and, on its second step, the quantity being typed for it.
type cartDialog struct {
	cursor int
	step   int // 0 the lines, 1 a quantity for the selected one
	qty    textinput.Model
	err    string
}

// cartLines is the cart: the buys in the order they were made, then the
// orders in city and ladder order.
func (m *Model) cartLines() []cartLine {
	w := m.w
	var lines []cartLine
	at := map[string]int{}
	for _, b := range w.Buys {
		k := game.OrderKey(b.City, b.Product)
		if i, ok := at[k]; ok {
			lines[i].qty += b.Qty
			lines[i].cost += b.Cost
			continue
		}
		at[k] = len(lines)
		lines = append(lines, cartLine{buy: true, city: b.City, product: b.Product, qty: b.Qty, cost: b.Cost})
	}
	for i := range lines {
		if lines[i].qty > 0 {
			lines[i].unit = float64(lines[i].cost) / float64(lines[i].qty)
		}
	}
	for _, cid := range w.CityOrder {
		for _, pid := range w.Products {
			o, ok := w.Order(cid, pid)
			if !ok {
				continue
			}
			_, take, heat := m.orderEstimate(o)
			lines = append(lines, cartLine{city: cid, product: pid, qty: o.Qty, dial: o.Dial, take: take, heat: heat})
		}
	}
	return lines
}

// orderEstimate is what an order is expected to move, take and cost in
// heat: the sell dialog's expect and heat rows for it.
func (m *Model) orderEstimate(o game.SellOrder) (units, take int, heat float64) {
	p := m.w.Product(o.City, o.Product)
	if p == nil {
		return 0, 0, 0
	}
	units = min(o.Qty, m.set.Market.Capacity(m.w, o.City, o.Product, o.Dial))
	take = int(float64(units) * p.Price * m.set.Market.Dial(o.Dial).Price)
	return units, take, m.estHeat(o.City, o.Product, o.Qty, o.Dial)
}

// cartTotals is the bottom line: the lines bought and what they cost,
// the lines queued, what they are expected to take and the heat.
type cartTotals struct {
	buys, sells int
	spent, take int
	heat        float64
}

func totals(lines []cartLine) cartTotals {
	var t cartTotals
	for _, l := range lines {
		if l.buy {
			t.buys++
			t.spent += l.cost
		} else {
			t.sells++
			t.take += l.take
			t.heat += l.heat
		}
	}
	return t
}

// cartSummary is the cart in a sentence, the END THE DAY? modal's:
// `Buying 3 lines for $12K, selling 2 lines, ~$30K, +4.2 heat.`; empty
// for an empty cart.
func (m *Model) cartSummary() string {
	t := totals(m.cartLines())
	var parts []string
	if t.buys > 0 {
		parts = append(parts, fmt.Sprintf("buying %s for %s", plural(t.buys, "line"), cash(t.spent)))
	}
	if t.sells > 0 {
		parts = append(parts, fmt.Sprintf("selling %s, ~%s, %+.1f heat", plural(t.sells, "line"), cash(t.take), t.heat))
	}
	if len(parts) == 0 {
		return ""
	}
	return capitalize(strings.Join(parts, ", ")) + "."
}

// cartCols is the CART table: what the line is, the product, the city,
// the units, a buy's price a unit, an order's dial, the cash (a buy's
// cost, an order's expected take) and an order's heat.
var cartCols = []col{{"line", kText, 0}, {"product", kText, 0}, {"city", kText, 0}, {"units", kInt, 0}, {"price", kPrice, 0}, {"dial", kDial, 0}, {"cash", kMoney, 0}, {"heat", kText, 0}}

// cartRows are the cart's lines as CART table rows.
func (m *Model) cartRows(lines []cartLine) [][]any {
	var rows [][]any
	for _, l := range lines {
		name, city := m.w.ProductName(l.product), m.w.CityName(l.city)
		if l.buy {
			rows = append(rows, []any{"buy", name, city, l.qty, l.unit, nil, styled{theme.Bad, -l.cost}, nil})
		} else {
			rows = append(rows, []any{"sell", name, city, l.qty, nil, dialShort(l.dial), styled{theme.Gold, l.take}, styled{heatStyle(m.w.City(l.city).Heat + l.heat*4), fmt.Sprintf("%+.1f", l.heat)}})
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
		ls = append(ls, row("buying", fmt.Sprintf("%s, %s", plural(t.buys, "line"), cash(t.spent))))
	}
	if t.sells > 0 {
		ls = append(ls, row("selling", fmt.Sprintf("%s, ~%s", plural(t.sells, "line"), cash(t.take))), row("heat", fmt.Sprintf("%+.1f expected", t.heat)))
	}
	for _, l := range lines {
		what := fmt.Sprintf("%d %s", l.qty, m.w.ProductName(l.product))
		if l.buy {
			ls = append(ls, row("buy", what+" "+money(l.cost)))
		} else {
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
	ti := textinput.New()
	ti.Placeholder = "blank = max"
	ti.CharLimit = 6
	ti.Width = 14
	ti.Prompt = "> "
	m.crt = cartDialog{qty: ti}
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
	switch key {
	case "esc":
		if d.step == 0 {
			m.mode = modePlay
		} else {
			d.step = 0
			d.qty.Blur()
		}
		return m, nil
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
		var cmd tea.Cmd
		d.qty, cmd = d.qty.Update(k)
		return m, cmd
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
	case "enter":
		if l == nil {
			return m, nil
		}
		d.step = 1
		d.qty.SetValue(fmt.Sprint(l.qty))
		d.qty.CursorEnd()
		d.qty.Focus()
		return m, textinput.Blink
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
			if err := m.w.PlaceSell(l.city, l.product, l.qty, dial); err != nil {
				d.err = dialogError(err)
			}
		}
	}
	return m, nil
}

// setCartQty is enter on the cart's quantity step: an order is placed
// again at the new quantity; a buy is returned down to it, or added to
// from the supplier where you stand.
func (m *Model) setCartQty() {
	d := &m.crt
	l := m.cartSelected()
	if l == nil {
		d.step = 0
		return
	}
	if l.buy {
		here := l.city == m.w.Player.Location
		most := l.qty
		if here {
			most += m.maxBuy(l.product)
		}
		qty, err := parseQtyInput(d.qty.Value(), most)
		if err != nil {
			d.err = dialogError(err)
			return
		}
		switch {
		case qty < l.qty:
			refund, err := m.w.Return(l.city, l.product, l.qty-qty)
			if err != nil {
				d.err = dialogError(err)
				m.refuse("Can't return: " + err.Error())
				return
			}
			m.say(fmt.Sprintf("Returned %d %s, %s back.", l.qty-qty, m.w.ProductName(l.product), money(refund)))
		case qty > l.qty && !here:
			d.err = fmt.Sprintf("You are in %s: it was bought in %s.", m.w.CityName(m.w.Player.Location), m.w.CityName(l.city))
			return
		case qty > l.qty:
			p, err := m.w.Buy(l.product, qty-l.qty, m.set.Market.BuyPressure(m.w))
			if err != nil {
				d.err = dialogError(err)
				return
			}
			m.say(fmt.Sprintf("Bought %d more %s for %s.", p.Qty, m.w.ProductName(l.product), money(p.Cost)))
		}
	} else {
		qty, err := parseQtyInput(d.qty.Value(), m.w.Stock(l.city, l.product))
		if err != nil {
			d.err = dialogError(err)
			return
		}
		if err := m.w.PlaceSell(l.city, l.product, qty, l.dial); err != nil {
			d.err = dialogError(err)
			return
		}
		m.say(fmt.Sprintf("Queued %d %s in %s, %s.", qty, m.w.ProductName(l.product), m.w.CityName(l.city), l.dial))
	}
	d.step = 0
	d.qty.Blur()
}

// removeCartLine is x on a line: an order is cancelled, a buy returned
// whole. A buy whose units have left the stash stays, with the refusal
// in the modal and the status bar.
func (m *Model) removeCartLine(l cartLine) {
	if !l.buy {
		m.w.CancelSell(l.city, l.product)
		m.say("Order cancelled.")
		return
	}
	refund, err := m.w.Return(l.city, l.product, l.qty)
	if err != nil {
		m.crt.err = dialogError(err)
		m.refuse("Can't return: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Returned %d %s, %s back.", l.qty, m.w.ProductName(l.product), money(refund)))
}

// viewCart is the cart modal: the CART table under the cursor, the
// totals, and on the second step the quantity for the selected line.
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
	body = append(body, "", m.cartTotalLine(totals(lines)))
	if d.step == 1 {
		l := lines[cursor]
		var note string
		if l.buy {
			note = fmt.Sprintf("bought %d", l.qty)
			if l.city == m.w.Player.Location {
				note += fmt.Sprintf(", up to %d more", m.maxBuy(l.product))
			}
		} else {
			note = fmt.Sprintf("have %d in %s", m.w.Stock(l.city, l.product), m.w.CityName(l.city))
		}
		body = append(body, "", fmt.Sprintf("quantity   %s   %s", d.qty.View(), theme.Subtle.Render(note)))
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal("CART", body, m.modalFooter())
}

// endDayLine is the END THE DAY? modal's first sentence: lying low, the
// cart in a sentence, or nothing queued.
func (m *Model) endDayLine() string {
	switch {
	case m.w.LieLow:
		return "Lying low today."
	case len(m.cartLines()) > 0:
		return m.cartSummary()
	}
	return "No sales queued."
}
