package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// dialog is the state of the buy or sell modal. A buy is from the
// supplier where you are, into the stash there; a sale is in the city
// shown, out of the stash there, by whoever works corners there.
type dialog struct {
	step int // 0 product, 1 quantity, 2 dial (sell only)
	qty  textinput.Model
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
		m.status = "You are lying low today. Press l to get back on the corner."
		return
	}
	ti := textinput.New()
	ti.Placeholder = "blank = max"
	ti.CharLimit = 6
	ti.Width = 14
	ti.Prompt = "> "
	m.dlg = dialog{qty: ti, dial: events.DialNormal}
	if mode == modeSell {
		city := m.actionCity()
		if m.w.Player.StockIn(city) == 0 {
			if m.w.Player.TotalStock() == 0 {
				m.status = "Nothing to sell. Press b to buy from the supplier."
			} else {
				m.status = fmt.Sprintf("Nothing stashed in %s to sell. Turn to the other city (←→), or run a route into it (5, r).", m.w.CityName(city))
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

func (m *Model) keyDialog(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.dlg
	d.err = ""
	switch key {
	case "esc":
		if d.step == 0 {
			m.mode = modePlay
		} else {
			d.step--
			d.qty.Blur()
		}
		return m, nil
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
			if m.mode == modeSell && m.w.Stock(m.dialogCity(), m.w.Products[m.cursor]) == 0 {
				d.err = "you have none of that here"
				return m, nil
			}
			if m.mode == modeBuy && m.maxBuy(m.w.Products[m.cursor]) == 0 {
				d.err = "you can't afford or hold any"
				return m, nil
			}
			d.step = 1
			d.qty.Focus()
			return m, textinput.Blink
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
		var cmd tea.Cmd
		d.qty, cmd = d.qty.Update(k)
		return m, cmd
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

func (m *Model) parseQty(maxQty int) (int, error) { return parseQtyInput(m.dlg.qty.Value(), maxQty) }

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
		m.dlg.err = err.Error()
		return m, nil
	}
	p, err := m.w.Buy(id, qty, m.set.Market.BuyPressure(m.w))
	if err != nil {
		m.dlg.err = err.Error()
		return m, nil
	}
	m.mode = modePlay
	m.status = fmt.Sprintf("Bought %d %s for %s.", p.Qty, m.w.ProductName(id), money(p.Cost))
	return m, nil
}

func (m *Model) confirmSell() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	city := m.dialogCity()
	qty, err := m.parseQty(m.w.Stock(city, id))
	if err != nil {
		m.dlg.err = err.Error()
		m.dlg.step = 1
		m.dlg.qty.Focus()
		return m, nil
	}
	if err := m.w.PlaceSell(city, id, qty, m.dlg.dial); err != nil {
		m.dlg.err = err.Error()
		m.dlg.step = 1
		m.dlg.qty.Focus()
		return m, nil
	}
	m.mode = modePlay
	m.status = fmt.Sprintf("Queued %d %s in %s, %s. Ends at end of day.", qty, m.w.ProductName(id), m.w.CityName(city), m.dlg.dial)
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
	for i, pid := range w.Products {
		pm := w.Product(city, pid)
		if pm == nil {
			continue
		}
		var line string
		if buy {
			line = fmt.Sprintf("%-8s  %8s/unit   have %3d", pm.Name, price(pm.SupplierPrice), w.Stock(city, pid))
		} else {
			line = fmt.Sprintf("%-8s  %8s/unit   have %3d   demand ~%.0f", pm.Name, price(pm.Price), w.Stock(city, pid), w.Demand(city, pid))
		}
		if i == m.cursor {
			m.modalFollow(len(body))
			body = append(body, theme.Gold.Render("▸ ")+theme.Selected.Render(line))
		} else {
			body = append(body, "  "+theme.Subtle.Render(line))
		}
	}
	body = append(body, "")

	// Step 1: quantity.
	if d.step >= 1 {
		if buy {
			mx := m.maxBuy(id)
			body = append(body, fmt.Sprintf("quantity   %s   %s", d.qty.View(), theme.Subtle.Render(fmt.Sprintf("max %d", mx))))
			if qty, err := m.parseQty(mx); err == nil {
				cost, _ := w.SupplierQuote(id, qty)
				style := theme.Gold
				if cost > w.Player.DirtyCash {
					style = theme.Bad
				}
				body = append(body, fmt.Sprintf("total      %s   %s", style.Render(money(cost)), theme.Subtle.Render("dirty cash "+cash(w.Player.DirtyCash))))
			}
			if o := m.set.Logistics.Wholesale(); w.Here().Wholesale && !o.Locked(w) {
				body = append(body, theme.Subtle.Render(fmt.Sprintf("The wholesaler here sells lots of %d at %s/unit to the routes (map, r).", o.Lot, price(p.SupplierPrice*o.Mul))))
			}
		} else {
			body = append(body, fmt.Sprintf("quantity   %s   %s", d.qty.View(), theme.Subtle.Render(fmt.Sprintf("have %d in %s", w.Stock(city, id), w.CityName(city)))))
		}
	} else {
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
			body = append(body, theme.Bad.Render(fmt.Sprintf("You work no corner in %s: nothing will sell. Post somebody (map, 5).", w.CityName(city))))
		}
	}

	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	title := "SELL · " + w.CityName(city)
	if buy {
		title = "BUY · " + w.CityName(city)
	}
	return m.modal(title, body, m.modalFooter())
}

// dialRow draws the sell dial as `quiet  normal  [aggressive]`.
func dialRow(d events.Dial) string {
	var cells []string
	for i, n := range []string{"quiet", "normal", "aggressive"} {
		if events.Dial(i) == d {
			cells = append(cells, theme.Gold.Render("["+n+"]"))
		} else {
			cells = append(cells, theme.Subtle.Render(n))
		}
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
