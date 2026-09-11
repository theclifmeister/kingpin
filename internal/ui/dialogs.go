package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// dialog is the state of the buy or sell modal. A buy is from the
// supplier where you are, into the stash there; a sale is in the city
// shown, out of the stash there, by whoever works corners there.
type dialog struct {
	step int // 0 product, 1 quantity, 2 dial (sell only)
	qty  numberField
	dial events.Dial
	err  string
}

// dialogCity is the city a buy or sell dialog is about: a buy is where
// you are; a sale is in the city the market or map is turned to, and
// where you are from any other screen.
func (m *Model) dialogCity() string {
	if m.mode == modeBuy {
		return m.w.Player.Location
	}
	return m.actionCity()
}

// actionCity is the city a sale or a cancelled order is about: the one
// shown on the market and map screens, where you are everywhere else.
func (m *Model) actionCity() string {
	if m.screen == screenMarket || m.screen == screenMap {
		return m.shown().ID
	}
	return m.w.Player.Location
}

func (m *Model) openDialog(mode mode) {
	if m.w.Over != nil {
		return
	}
	if m.w.LieLow && mode == modeSell {
		m.refuse("Can't sell: you are lying low today, nothing sells.")
		return
	}
	m.dlg = dialog{qty: newNumberField("blank = max"), dial: events.DialNormal}
	if mode == modeSell {
		city := m.actionCity()
		if m.w.Player.StockIn(city) == 0 {
			if m.w.Player.TotalStock() == 0 {
				m.refuse("Nothing to sell: buy from the supplier first.")
			} else {
				m.refuse(fmt.Sprintf("Nothing to sell in %s: turn to the other city, or run a route into it %s.", m.w.CityName(city), screenPointer(screenMap)))
			}
			return
		}
		// Land on something you actually hold there.
		if m.w.Stock(city, m.w.Products[m.cursor]) == 0 {
			for i, id := range m.w.Products {
				if m.w.Stock(city, id) > 0 {
					m.cursor = i
					break
				}
			}
		}
		if o, ok := m.w.Order(city, m.w.Products[m.cursor]); ok {
			m.dlg.dial = o.Dial
		}
	}
	m.mode = mode
}

// keyDialog is the buy and sell dialogs' key handler. Back is one key
// and close is one key (#110): esc closes the dialog from any step,
// shift+tab goes back a step keeping what the earlier steps hold (the
// product stays under the cursor; the quantity is cleared on leaving
// its step and kept on coming back to it from the dial) and is silent
// on the first, tab goes forward once the step is complete (a product
// you can buy or sell, a quantity that reads) and is silent otherwise;
// enter is as it was. The quantity step is a numberField (#112): m, h,
// ↑↓ and pgup pgdn move the number within what the field can take, the
// stash here for a sale and maxBuy for a buy.
func (m *Model) keyDialog(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.dlg
	d.err = ""
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "shift+tab":
		return m.dialogBack()
	case "tab":
		return m.dialogForward()
	case "q":
		if d.step != 1 {
			m.mode = modePlay
			return m, nil
		}
	}
	switch d.step {
	case 0:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.w.Products)-1 {
				m.cursor++
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if i := int(key[0] - '1'); i < len(m.w.Products) {
				m.cursor = i
			}
		case "enter", "right", "l":
			if err := m.productErr(); err != "" {
				d.err = err
				return m, nil
			}
			return m.dialogForward()
		}
		return m, nil
	case 1:
		switch key {
		case "enter":
			if m.mode == modeBuy {
				return m.confirmBuy()
			}
			d.step = 2
			d.qty.Blur()
			return m, nil
		}
		d.qty.max = m.qtyMax()
		return m, d.qty.Update(k)
	case 2:
		switch key {
		case "left", "h":
			if d.dial > events.DialQuiet {
				d.dial--
			}
		case "right", "l":
			if d.dial < events.DialAggressive {
				d.dial++
			}
		case "1":
			d.dial = events.DialQuiet
		case "2":
			d.dial = events.DialNormal
		case "3":
			d.dial = events.DialAggressive
		case "enter":
			return m.confirmSell()
		}
	}
	return m, nil
}

// qtyMax is what the quantity step can take: the stash of the product
// in the dialog's city for a sale, what the supplier will sell you for
// a buy (maxBuy). Blank has always meant it; m fills it in.
func (m *Model) qtyMax() int {
	id := m.w.Products[m.cursor]
	if m.mode == modeBuy {
		return m.maxBuy(id)
	}
	return m.w.Stock(m.dialogCity(), id)
}

// productErr is why the product under the cursor cannot go to the
// quantity step: none of it here to sell, or none affordable to buy;
// empty when it can.
func (m *Model) productErr() string {
	id := m.w.Products[m.cursor]
	if m.mode == modeSell && m.w.Stock(m.dialogCity(), id) == 0 {
		return "You have none of that here."
	}
	if m.mode == modeBuy && m.maxBuy(id) == 0 {
		return "Can't afford or hold any."
	}
	return ""
}

// dialogForward is tab on the buy or sell dialog: the next step once
// this one is complete, silent otherwise, and silent on the last (enter
// is what buys or sells).
func (m *Model) dialogForward() (tea.Model, tea.Cmd) {
	d := &m.dlg
	switch d.step {
	case 0:
		if m.productErr() != "" {
			return m, nil
		}
		d.step = 1
		return m, d.qty.Focus()
	case 1:
		if m.mode == modeBuy {
			return m, nil
		}
		if _, err := m.parseQty(m.w.Stock(m.dialogCity(), m.w.Products[m.cursor])); err != nil {
			return m, nil
		}
		d.step = 2
		d.qty.Blur()
	}
	return m, nil
}

// dialogBack is shift+tab on the buy or sell dialog: the step before,
// silent on the first.
func (m *Model) dialogBack() (tea.Model, tea.Cmd) {
	d := &m.dlg
	switch d.step {
	case 1:
		d.step = 0
		d.qty.SetValue("")
		d.qty.Blur()
	case 2:
		d.step = 1
		return m, d.qty.Focus()
	}
	return m, nil
}

func (m *Model) parseQty(maxQty int) (int, error) { return parseQtyInput(m.dlg.qty.Value(), maxQty) }

// dialogError is a game error as a dialog shows it: in the register of
// the status bar's refusals, sentence case with a full stop (`Enter a
// whole number above zero.`, `Only 3 Weed in Eastside.`).
func dialogError(err error) string { return sentence(capitalize(err.Error())) }

// parseQtyInput reads a quantity field: blank means the most allowed.
func parseQtyInput(v string, maxQty int) (int, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		if maxQty <= 0 {
			return 0, fmt.Errorf("nothing to do")
		}
		return maxQty, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("enter a whole number above zero")
	}
	return n, nil
}

// maxBuy is the most of a product the supplier where you are will sell
// you: what you can pay for and what the stash there can hold.
func (m *Model) maxBuy(id string) int {
	city := m.w.Player.Location
	p := m.w.Product(city, id)
	if p == nil || p.SupplierPrice <= 0 {
		return 0
	}
	afford := int(math.Floor(float64(m.w.Player.DirtyCash) / p.SupplierPrice))
	return max(0, min(afford, m.w.Free(city)))
}

func (m *Model) confirmBuy() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	qty, err := m.parseQty(m.maxBuy(id))
	if err != nil {
		m.dlg.err = dialogError(err)
		return m, nil
	}
	p, err := m.w.Buy(id, qty, m.set.Market.BuyPressure(m.w))
	if err != nil {
		m.dlg.err = dialogError(err)
		return m, nil
	}
	m.say(fmt.Sprintf("Bought %d %s for %s.", p.Qty, m.w.ProductName(id), money(p.Cost)))
	m.nextLine()
	return m, nil
}

// nextLine is the dialog after a buy or an order (#103): back on the
// product step with the cart under the table, so the next product can
// be picked without reopening; esc there closes it.
func (m *Model) nextLine() {
	m.dlg.step = 0
	m.dlg.qty.SetValue("")
	m.dlg.qty.Blur()
}

func (m *Model) confirmSell() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.w.Stock(city, id))
	if err != nil {
		m.dlg.err = dialogError(err)
		m.dlg.step = 1
		m.dlg.qty.Focus()
		return m, nil
	}
	if err := m.w.PlaceSell(city, id, qty, m.dlg.dial); err != nil {
		m.dlg.err = dialogError(err)
		m.dlg.step = 1
		m.dlg.qty.Focus()
		return m, nil
	}
	m.say(fmt.Sprintf("Queued %d %s in %s, %s. It sells at the end of the day.", qty, m.w.ProductName(id), m.w.CityName(city), m.dlg.dial))
	m.nextLine()
	return m, nil
}

// estHeat is what the heat sim will charge for this order in a city, plus
// the sloppy crew premium on the units it expects to move.
func (m *Model) estHeat(city, id string, qty int, dial events.Dial) float64 {
	if m.w.Product(city, id) == nil {
		return 0
	}
	moved := min(qty, m.set.Market.Capacity(m.w, city, id, dial))
	return m.set.Heat.SaleHeat(m.w, city, id, qty, dial) + m.set.Heat.SloppyHeat(m.w, city, moved)
}

func (m *Model) viewDialog() string {
	w := m.w
	d := m.dlg
	id := w.Products[m.cursor]
	city := m.dialogCity()
	p := w.Product(city, id)
	buy := m.mode == modeBuy
	var body []string

	// Step 0: product list.
	cols := []col{{"product", kText, 0}, {"price/unit", kPrice, 0}, {"have", kInt, 0}}
	if !buy {
		cols = append(cols, col{"demand/day", kInt, 0})
	}
	var rows [][]any
	cursor := -1
	for i, pid := range w.Products {
		pm := w.Product(city, pid)
		if pm == nil {
			continue
		}
		if i == m.cursor {
			cursor = len(rows)
		}
		if buy {
			rows = append(rows, []any{pm.Name, pm.SupplierPrice, w.Stock(city, pid)})
		} else {
			rows = append(rows, []any{pm.Name, pm.Price, w.Stock(city, pid), approx{w.Demand(city, pid)}})
		}
	}
	if cursor >= 0 {
		m.modalFollow(len(body) + 1 + cursor) // under the header
	}
	body = append(body, table(cols, rows, cursor, m.modalInner())...)
	body = append(body, "")

	// Step 1: quantity, the number field with what it can take after it.
	if d.step >= 1 {
		d.qty.max = m.qtyMax()
		body = append(body, "quantity   "+d.qty.View())
		if buy {
			if qty, err := m.parseQty(d.qty.max); err == nil {
				cost, _ := w.SupplierQuote(id, qty)
				style := theme.Gold
				if cost > w.Player.DirtyCash {
					style = theme.Bad
				}
				body = append(body, fmt.Sprintf("total      %s   %s", style.Render(money(cost)), theme.Subtle.Render("dirty cash "+cash(w.Player.DirtyCash))))
			}
			if o := m.set.Logistics.Wholesale(w); w.Here().Wholesale && !o.Locked(w) {
				body = append(body, theme.Subtle.Render(fmt.Sprintf("The wholesaler's lots of %d at %s/unit go to the routes %s.", o.Lot, price(p.SupplierPrice*o.Mul), screenPointer(screenMap))))
			}
		}
	} else if len(w.Buys)+len(w.Orders) == 0 {
		body = append(body, theme.Subtle.Render("Pick a product."))
	}

	// Step 2: dial preview. The dial row is the dial convention: the
	// chosen notch in brackets and the accent.
	if !buy && d.step >= 2 {
		qty, _ := m.parseQty(w.Stock(city, id))
		body = append(body, "", "dial       "+dialRow(d.dial))
		dc := m.set.Market.Dial(d.dial)
		est := min(qty, m.set.Market.Capacity(w, city, id, d.dial))
		body = append(body, fmt.Sprintf("expect     ~%d of %d at ~%s = ~%s", est, qty, price(p.Price*dc.Price), theme.Gold.Render(money(int(float64(est)*p.Price*dc.Price)))))
		h := m.estHeat(city, id, qty, d.dial)
		body = append(body, fmt.Sprintf("heat       %s   %s", heatStyle(w.City(city).Heat+h*4).Render(fmt.Sprintf("+%.1f", h)), theme.Subtle.Render(dialBlurb(d.dial))))
		if w.WorkedIn(city) == 0 {
			body = append(body, theme.Bad.Render(fmt.Sprintf("You work no corner in %s: nothing will sell.", w.CityName(city))), theme.Bad.Render("Post somebody "+screenPointer(screenMap)+"."))
		}
	}

	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	if cb := m.cartBlock(); cb != nil {
		if body[len(body)-1] != "" {
			body = append(body, "")
		}
		body = append(body, cb...)
	}
	title := "SELL · " + w.CityName(city)
	if buy {
		title = "BUY · " + w.CityName(city)
	}
	return m.modal(title, body, m.modalFooter())
}

// dialRow draws the sell dial as `quiet  normal  [aggressive]`.
func dialRow(d events.Dial) string {
	return dialCells([]string{"quiet", "normal", "aggressive"}, int(d))
}

// dialCells is the dial convention (#88): every notch in a row two
// spaces apart, the chosen one bracketed in the accent (theme.Dial), as
// `quiet  [normal]  aggressive`. The sell, pay and launder dials draw
// through it.
func dialCells(notches []string, on int) string {
	cells := make([]string, len(notches))
	for i, n := range notches {
		if i == on {
			n = "[" + n + "]"
		}
		cells[i] = theme.Dial(i == on).Render(n)
	}
	return strings.Join(cells, "  ")
}

// dialBlurb is what the dial does, short enough for the modal's width
// after the heat figure.
func dialBlurb(d events.Dial) string {
	switch d {
	case events.DialQuiet:
		return "half the volume, small discount, barely a ripple"
	case events.DialAggressive:
		return "push past demand, premium first, then the crash"
	default:
		return "sell to demand at market price"
	}
}
