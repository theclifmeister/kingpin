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

// dialog is the state of the buy or sell modal.
type dialog struct {
	step int // 0 product, 1 quantity, 2 dial (sell only)
	qty  textinput.Model
	dial events.Dial
	err  string
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
		if m.w.Player.TotalStock() == 0 {
			m.status = "Nothing to sell. Press b to buy from the supplier."
			return
		}
		// Land on something you actually hold.
		if m.w.Player.Stock[m.w.Products[m.cursor]] == 0 {
			for i, id := range m.w.Products {
				if m.w.Player.Stock[id] > 0 {
					m.cursor = i
					break
				}
			}
		}
		if o, ok := m.w.Orders[m.w.Products[m.cursor]]; ok {
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
			if m.mode == modeSell && m.w.Player.Stock[m.w.Products[m.cursor]] == 0 {
				d.err = "you have none of that"
				return m, nil
			}
			if m.mode == modeBuy && m.maxBuy(m.w.Products[m.cursor]) == 0 {
				d.err = "you can't afford or carry any"
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

func (m *Model) parseQty(maxQty int) (int, error) {
	s := strings.TrimSpace(m.dlg.qty.Value())
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

func (m *Model) maxBuy(id string) int {
	p := m.w.Market[id]
	if p == nil || p.SupplierPrice <= 0 {
		return 0
	}
	afford := int(math.Floor(float64(m.w.Player.DirtyCash) / p.SupplierPrice))
	room := m.w.Player.CarryLimit - m.w.Player.TotalStock()
	return max(0, min(afford, room))
}

func (m *Model) confirmBuy() (tea.Model, tea.Cmd) {
	id := m.w.Products[m.cursor]
	qty, err := m.parseQty(m.maxBuy(id))
	if err != nil {
		m.dlg.err = err.Error()
		return m, nil
	}
	p, err := m.w.Buy(id, qty, m.cfg.Market.Market.BuyPricePressure)
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
	qty, err := m.parseQty(m.w.Player.Stock[id])
	if err != nil {
		m.dlg.err = err.Error()
		m.dlg.step = 1
		m.dlg.qty.Focus()
		return m, nil
	}
	if err := m.w.PlaceSell(id, qty, m.dlg.dial); err != nil {
		m.dlg.err = err.Error()
		m.dlg.step = 1
		m.dlg.qty.Focus()
		return m, nil
	}
	m.mode = modePlay
	m.status = fmt.Sprintf("Queued %d %s, %s. Ends at end of day.", qty, m.w.ProductName(id), m.dlg.dial)
	return m, nil
}

// estHeat mirrors the heat sim's formula so the dial preview is honest.
func (m *Model) estHeat(id string, qty int, dial events.Dial) float64 {
	p := m.w.Market[id]
	pc := m.cfg.Market.Product(id)
	if p == nil || pc == nil || p.Demand <= 0 {
		return 0
	}
	dc := m.set.Market.Dial(dial)
	sold := math.Min(float64(qty), math.Round(p.Demand*dc.Fill))
	return m.cfg.Heat.Heat.SaleHeat * sold / p.Demand * dc.Heat * pc.Heat
}

func (m *Model) viewDialog() string {
	w := m.w
	d := m.dlg
	id := w.Products[m.cursor]
	p := w.Market[id]
	buy := m.mode == modeBuy
	var b strings.Builder

	// Step 0: product list.
	for i, pid := range w.Products {
		pm := w.Market[pid]
		var line string
		if buy {
			line = fmt.Sprintf("%-6s  %8s/unit   have %d", pm.Name, fmt.Sprintf("$%.2f", pm.SupplierPrice), w.Player.Stock[pid])
		} else {
			line = fmt.Sprintf("%-6s  %8s/unit   have %d   demand ~%.0f", pm.Name, fmt.Sprintf("$%.2f", pm.Price), w.Player.Stock[pid], pm.Demand)
		}
		if i == m.cursor {
			b.WriteString(theme.Gold.Render("▸ ") + theme.Selected.Render(line) + "\n")
		} else {
			b.WriteString("  " + theme.Subtle.Render(line) + "\n")
		}
	}
	b.WriteString("\n")

	// Step 1: quantity.
	if d.step >= 1 {
		if buy {
			mx := m.maxBuy(id)
			b.WriteString(fmt.Sprintf("Quantity  %s   %s\n", d.qty.View(), theme.Subtle.Render(fmt.Sprintf("max %d", mx))))
			qty, err := m.parseQty(mx)
			if err == nil {
				cost, _ := w.SupplierQuote(id, qty)
				style := theme.Gold
				if cost > w.Player.DirtyCash {
					style = theme.Bad
				}
				b.WriteString(fmt.Sprintf("Total     %s   %s\n", style.Render(money(cost)), theme.Subtle.Render("dirty cash "+money(w.Player.DirtyCash))))
			}
		} else {
			b.WriteString(fmt.Sprintf("Quantity  %s   %s\n", d.qty.View(), theme.Subtle.Render(fmt.Sprintf("have %d", w.Player.Stock[id]))))
		}
	} else {
		b.WriteString(theme.Subtle.Render("Pick a product, then enter.") + "\n")
	}

	// Step 2: dial preview.
	if !buy && d.step >= 2 {
		b.WriteString("\n")
		qty, _ := m.parseQty(w.Player.Stock[id])
		names := []string{"quiet", "normal", "aggressive"}
		var cells []string
		for i, n := range names {
			if events.Dial(i) == d.dial {
				cells = append(cells, theme.Selected.Render(" "+n+" "))
			} else {
				cells = append(cells, theme.Subtle.Render(" "+n+" "))
			}
		}
		b.WriteString("Dial      " + strings.Join(cells, " ") + "\n")
		dc := m.set.Market.Dial(d.dial)
		capacity := int(math.Round(p.Demand * dc.Fill))
		if w.Heat.SellCapDays > 0 && w.Heat.SellCap > 0 && w.Heat.SellCap < dc.Fill {
			capacity = int(math.Round(p.Demand * w.Heat.SellCap))
		}
		est := min(qty, capacity)
		b.WriteString(fmt.Sprintf("Expect    ~%d of %d sold at ~$%.2f  =  ~%s\n", est, qty, p.Price*dc.Price, theme.Gold.Render(money(int(float64(est)*p.Price*dc.Price)))))
		h := m.estHeat(id, qty, d.dial)
		b.WriteString(fmt.Sprintf("Heat      %s   %s\n", heatStyle(w.Heat.Value+h*4).Render(fmt.Sprintf("+%.1f", h)), theme.Subtle.Render(dialBlurb(d.dial))))
	}

	if d.err != "" {
		b.WriteString("\n" + theme.Bad.Render(d.err) + "\n")
	}
	title := "SELL ON THE STREET"
	if buy {
		title = "BUY FROM SUPPLIER"
	}
	return m.modal(title, strings.TrimRight(b.String(), "\n"))
}

func dialBlurb(d events.Dial) string {
	switch d {
	case events.DialQuiet:
		return "half the volume, small discount, barely a ripple"
	case events.DialAggressive:
		return "push past demand, premium at first, then the price crashes"
	default:
		return "sell to demand at market price"
	}
}
