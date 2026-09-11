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
// shown, out of the stash there, by whoever works corners there. A
// buy's last step is whether it is for today or stands (#113: `once` /
// `keep at`, a supply contract at the quantity); a sale's is the dial.
type dialog struct {
	step   int // 0 product, 1 quantity, 2 the buy's repeat or the sale's dial
	qty    numberField
	dial   events.Dial
	repeat repeat
	err    string
}

// repeat is the last step of the buy dialog (#113), and the shape the
// sell dialog's standing orders (#114) mirror: whether the line is for
// today (once) or stands (keep at: a supply contract at the quantity).
type repeat int

const (
	repeatOnce repeat = iota
	repeatKeep
)

// repeatNames are the notches as the dialog draws them, through the
// dial convention (dialCells).
var repeatNames = []string{"once", "keep at"}

// stepRepeat is the key handling of a repeat step: ←→ (h, l) turn it, a
// digit picks a notch. It reports whether the key was one of those.
func stepRepeat(key string, on *repeat, n int) bool {
	switch key {
	case "left", "h":
		if *on > 0 {
			*on--
		}
	case "right", "l":
		if int(*on) < n-1 {
			*on++
		}
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := int(key[0] - '1'); i < n {
			*on = repeat(i)
		}
	default:
		return false
	}
	return true
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
		if m.sellableIn(city) == 0 {
			if m.w.Player.TotalStock() == 0 {
				m.refuse("Nothing to sell: buy from the supplier first.")
			} else {
				m.refuse(fmt.Sprintf("Nothing to sell in %s: turn to the other city, or run a route into it %s.", m.w.CityName(city), screenPointer(screenMap)))
			}
			return
		}
		// Land on something you actually hold there, or that the
		// contract brings.
		if m.sellable(city, m.w.Products[m.cursor]) == 0 {
			for i, id := range m.w.Products {
				if m.sellable(city, id) > 0 {
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
// its step and kept on coming back to it from the last step) and is
// silent on the first, tab goes forward once the step is complete (a
// product you can buy or sell, a quantity that reads) and is silent
// otherwise; enter is next until the last step, where it commits: a
// buy's `once` / `keep at` (#113, ←→ or 1-2; enter buys, or sets the
// contract), a sale's dial. The quantity step is a numberField (#112):
// m, h, ↑↓ and pgup pgdn move the number within what the field can
// take, the stash here (and what the contract brings) for a sale and
// maxBuy for a buy.
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
			d.step = 2
			d.qty.Blur()
			return m, nil
		}
		d.qty.max = m.qtyMax()
		return m, d.qty.Update(k)
	case 2:
		if m.mode == modeBuy {
			if key == "enter" {
				if d.repeat == repeatKeep {
					return m.confirmKeep()
				}
				return m.confirmBuy()
			}
			stepRepeat(key, &d.repeat, len(repeatNames))
			return m, nil
		}
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
// in the dialog's city plus what the supply contract there brings in
// the morning (#113: the contract fills before the orders resolve) for
// a sale, what the supplier will sell you for a buy (maxBuy). Blank has
// always meant it; m fills it in.
func (m *Model) qtyMax() int {
	id := m.w.Products[m.cursor]
	if m.mode == modeBuy {
		return m.maxBuy(id)
	}
	return m.sellable(m.dialogCity(), id)
}

// sellable is what an order for a product in a city may be for: the
// stash there and what its supply contract will buy in the morning by
// the market sim's plan.
func (m *Model) sellable(city, id string) int {
	return m.w.Stock(city, id) + m.set.Market.Due(m.w, city, id)
}

// sellableIn is sellable over every product in a city: whether the sell
// dialog has anything to open on there.
func (m *Model) sellableIn(city string) int {
	n := 0
	for _, id := range m.w.Products {
		n += m.sellable(city, id)
	}
	return n
}

// productErr is why the product under the cursor cannot go to the
// quantity step: none of it here to sell, or none affordable to buy;
// empty when it can.
func (m *Model) productErr() string {
	id := m.w.Products[m.cursor]
	if m.mode == modeSell && m.sellable(m.dialogCity(), id) == 0 {
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
		// A product kept by contract opens on its level, at keep at,
		// the way the target dialog opens on the target (#113).
		if c, ok := m.w.Supplied(m.dialogCity(), m.w.Products[m.cursor]); ok && m.mode == modeBuy {
			d.qty.Set(c.Units)
			d.repeat = repeatKeep
		}
		return m, d.qty.Focus()
	case 1:
		if _, err := m.parseQty(m.qtyMax()); err != nil {
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

// confirmBuy is enter on the buy dialog's last step at once: the buy
// itself. A quantity that does not read, or that the supplier refuses,
// puts the dialog back on the quantity step with the reason.
func (m *Model) confirmBuy() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	qty, err := m.parseQty(m.maxBuy(id))
	if err != nil {
		return m.quantityAgain(err)
	}
	p, err := m.w.Buy(id, qty, m.set.Market.BuyPressure(m.w))
	if err != nil {
		return m.quantityAgain(err)
	}
	m.say(fmt.Sprintf("Bought %d %s for %s.", p.Qty, m.w.ProductName(id), money(p.Cost)))
	m.nextLine()
	return m, nil
}

// confirmKeep is enter on the buy dialog's last step at keep at (#113):
// a supply contract for the product where you stand at the quantity,
// bought each morning at the contract price. The quantity may be over
// what you can buy today: the contract fills as far as the room and
// the cash allow.
func (m *Model) confirmKeep() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.maxBuy(id))
	if err != nil {
		return m.quantityAgain(err)
	}
	if err := m.w.SetSupply(city, id, qty); err != nil {
		return m.quantityAgain(err)
	}
	m.say(fmt.Sprintf("Keeping %d %s in %s: bought each morning at ×%.2f the supplier's price.", qty, m.w.ProductName(id), m.w.CityName(city), m.set.Market.Markup()))
	m.nextLine()
	return m, nil
}

// quantityAgain is a buy or sell dialog refused on its last step: back
// on the quantity step, the reason shown, the field focused.
func (m *Model) quantityAgain(err error) (tea.Model, tea.Cmd) {
	m.dlg.err = dialogError(err)
	m.dlg.step = 1
	return m, m.dlg.qty.Focus()
}

// nextLine is the dialog after a buy or an order (#103): back on the
// product step with the cart under the table, so the next product can
// be picked without reopening; esc there closes it.
func (m *Model) nextLine() {
	m.dlg.step = 0
	m.dlg.qty.SetValue("")
	m.dlg.qty.Blur()
	m.dlg.repeat = repeatOnce
}

func (m *Model) confirmSell() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.sellable(city, id))
	if err != nil {
		return m.quantityAgain(err)
	}
	if err := m.w.PlaceSell(city, id, qty, m.dlg.dial); err != nil {
		return m.quantityAgain(err)
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
		} else if due := m.set.Market.Due(w, city, id); due > 0 {
			body = append(body, theme.Subtle.Render(fmt.Sprintf("%d stashed and %d the contract brings in the morning.", w.Stock(city, id), due)))
		}
	} else if len(w.Buys)+len(w.Orders) == 0 {
		body = append(body, theme.Subtle.Render("Pick a product."))
	}

	// Step 2 of a buy: once or keep at (#113), in the dial convention,
	// and what keeping the quantity means.
	if buy && d.step >= 2 {
		qty, _ := m.parseQty(m.maxBuy(id))
		body = append(body, "", "repeat     "+dialCells(repeatNames, int(d.repeat)))
		if d.repeat == repeatKeep {
			body = append(body, fmt.Sprintf("contract   keep %d here, the shortfall bought each morning at %s (×%.2f)", qty, price(m.set.Market.SupplyPrice(w, city, id)), m.set.Market.Markup()))
			if c, ok := w.Supplied(city, id); ok {
				body = append(body, theme.Subtle.Render(fmt.Sprintf("Kept at %d since day %d; this replaces it.", c.Units, c.Since)))
			}
		} else {
			body = append(body, theme.Subtle.Render("Bought now, once. Keep at is a supply contract: the same each morning."))
		}
	}

	// Step 2 of a sale: dial preview. The dial row is the dial
	// convention: the chosen notch in brackets and the accent.
	if !buy && d.step >= 2 {
		qty, _ := m.parseQty(m.sellable(city, id))
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
