package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Quality (#47): the cut and the cook are the market screen's two lab
// dialogs, one shape (labDialog): a product page, then a number page
// through numberField. The cut (`t`, modeCut) steps on what the stash
// where you stand holds of a product: the number is the percent added,
// up to the product's cut_max, and the page says what the lot becomes
// (the units, the quality, the price multiplier, the chemist's hand)
// and what it costs. The cook (`o`, modeCook) is a chemist's order:
// the number is the units, up to their batch, the room and the till,
// and the page says the quality the lot lands at and when. Both are
// for where you stand: a cut is done in the stash you can reach, and
// the chemist works where you are.

// labDialog is the cut and the cook dialog's state.
type labDialog struct {
	step    int
	city    string
	product string
	cursor  int
	qty     numberField
	err     string
}

// labList is a lab dialog being on its product page.
func labList(m *Model) bool { return m.lab.step == 0 }

// labNumber is a lab dialog being on its number page.
func labNumber(m *Model) bool { return m.lab.step == 1 }

// cutProducts are the products the stash where you stand holds that the
// file lets you cut.
func (m *Model) cutProducts() []string {
	var out []string
	for _, id := range m.w.Products {
		if m.w.Stock(m.lab.city, id) > 0 && m.set.Market.CutMax(id) > 0 {
			out = append(out, id)
		}
	}
	return out
}

// cookProducts are the products a chemist cooks that are on the ladder
// in the city.
func (m *Model) cookProducts() []string {
	var out []string
	for _, id := range m.w.Products {
		if m.w.Product(m.lab.city, id) != nil && m.set.Market.Cooks(id) {
			out = append(out, id)
		}
	}
	return out
}

// labProducts is the product page's list for the open dialog.
func (m *Model) labProducts() []string {
	if m.mode == modeCook {
		return m.cookProducts()
	}
	return m.cutProducts()
}

func (m *Model) askCut() {
	if m.w.Over != nil {
		return
	}
	if m.w.Today.LieLow {
		m.refuse("Lying low today: nobody is working.")
		return
	}
	m.lab = labDialog{city: m.w.Player.Location, qty: newNumberField("blank = the most")}
	if len(m.cutProducts()) == 0 {
		m.refuse("Nothing here to cut: the stash in " + m.w.Here().Name + " is empty.")
		return
	}
	m.lab.qty.Focus()
	m.mode = modeCut
}

func (m *Model) askCook() {
	if m.w.Over != nil {
		return
	}
	if m.w.Today.LieLow {
		m.refuse("Lying low today: nobody is working.")
		return
	}
	if m.w.Crew.Chemist() == nil {
		m.refuse("Nobody on the payroll can cook. Hire a chemist on the crew screen (4).")
		return
	}
	m.lab = labDialog{city: m.w.Player.Location, qty: newNumberField("blank = a batch")}
	if len(m.cookProducts()) == 0 {
		m.refuse("Nothing to cook: no product a chemist makes is on the ladder yet.")
		return
	}
	m.lab.qty.Focus()
	m.mode = modeCook
}

// cutMax is the most percent a cut of the product can add: cut_max,
// held to the room where you stand and the till.
func (m *Model) cutMax() int {
	d := &m.lab
	units := m.w.Stock(d.city, d.product)
	most := int(m.set.Market.CutMax(d.product) * 100)
	if units <= 0 {
		return 0
	}
	if room := m.w.Free(d.city); room < units*most/100 {
		most = min(most, room*100/units)
	}
	if cost := m.set.Market.CutCost(d.product); cost > 0 {
		affordable := m.w.Player.DirtyCash / cost
		if affordable < units*most/100 {
			most = min(most, affordable*100/units)
		}
	}
	return max(0, most)
}

// cookMax is the most units a cook order can be for: the chemist's
// batch, the room where you stand with what is on its way counted, and
// the till.
func (m *Model) cookMax() int {
	d := &m.lab
	most := m.set.Crew.Batch(m.w)
	most = min(most, m.w.Free(d.city)-m.w.Crew.Cooking(d.city, d.product))
	if cost := m.set.Market.CookCost(d.product); cost > 0 {
		most = min(most, m.w.Player.DirtyCash/cost)
	}
	return max(0, most)
}

// labMax is the number page's max for the open dialog.
func (m *Model) labMax() int {
	if m.mode == modeCook {
		return m.cookMax()
	}
	return m.cutMax()
}

func (m *Model) keyLab(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	d := &m.lab
	d.err = ""
	switch key {
	case "esc":
		m.mode = modePlay
		return m, nil
	case "shift+tab":
		if d.step > 0 {
			d.step--
			d.qty.SetValue("")
		}
		return m, nil
	case "tab":
		if d.step == 0 {
			m.labNext()
		}
		return m, nil
	case "enter":
		if d.step == 0 {
			m.labNext()
		} else if m.mode == modeCook {
			m.confirmCook()
		} else {
			m.confirmCut()
		}
		return m, nil
	}
	if d.step == 0 {
		ids := m.labProducts()
		switch key {
		case "up", "k":
			if d.cursor > 0 {
				d.cursor--
			}
		case "down", "j":
			if d.cursor < len(ids)-1 {
				d.cursor++
			}
		default:
			if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				if i := int(key[0] - '1'); i < len(ids) {
					d.cursor = i
					m.labNext()
				}
			}
		}
		return m, nil
	}
	d.qty.max = m.labMax()
	return m, d.qty.Update(k)
}

// labNext takes the product under the cursor and opens the number page.
func (m *Model) labNext() {
	d := &m.lab
	ids := m.labProducts()
	if len(ids) == 0 {
		return
	}
	d.product = ids[max(0, min(d.cursor, len(ids)-1))]
	d.step = 1
}

// cutPreview is what a cut at a percent makes of the lot: the units
// after, the quality after with the chemist's hand, and the cost.
func (m *Model) cutPreview(pct int) (units int, quality float64, cost int) {
	d := &m.lab
	have := m.w.Stock(d.city, d.product)
	added := int(float64(have)*float64(pct)/100 + 0.5)
	from := m.w.Quality(d.city, d.product)
	quality = from
	if have+added > 0 {
		quality = min(from, from*float64(have)/float64(have+added)+m.set.Crew.CutBonus(m.w))
	}
	return have + added, quality, added * m.set.Market.CutCost(d.product)
}

func (m *Model) confirmCut() {
	d := &m.lab
	pct, err := parseQtyInput(d.qty.Value(), m.cutMax())
	if err != nil {
		d.err = dialogError(err)
		return
	}
	rec, err := m.w.Cut(d.city, d.product, float64(pct)/100, m.set.Market.CutMax(d.product), m.set.Market.CutCost(d.product), m.set.Crew.CutBonus(m.w), m.set.Crew.ChemistName(m.w))
	if err != nil {
		d.err = dialogError(err)
		return
	}
	m.mode = modePlay
	hand := ""
	if rec.Chemist != "" {
		hand = fmt.Sprintf(" %s kept it at that.", rec.Chemist)
	}
	m.say(fmt.Sprintf("Cut %d %s into %d: quality %.0f → %.0f, sells at ×%.2f.%s Cost %s.", rec.Units, m.w.ProductName(d.product), rec.Units+rec.Added, rec.From, rec.To, m.set.Market.QualityMul(rec.To), hand, cash(rec.Cost)))
}

func (m *Model) confirmCook() {
	d := &m.lab
	units, err := parseQtyInput(d.qty.Value(), m.cookMax())
	if err != nil {
		d.err = dialogError(err)
		return
	}
	k, err := m.w.CookOrder(d.city, d.product, units, m.set.Market.CookCost(d.product), m.set.Crew.CookDays(), m.set.Crew.ChemistQuality(m.w), m.set.Crew.Batch(m.w), m.set.Crew.ChemistName(m.w))
	if err != nil {
		d.err = dialogError(err)
		return
	}
	m.mode = modePlay
	m.say(fmt.Sprintf("%s is cooking %d %s at quality %.0f, ready in %s. Cost %s.", k.Chemist, k.Units, m.w.ProductName(d.product), k.Quality, plural(k.Ready-k.Ordered, "day"), cash(k.Cost)))
}

// viewLab draws the cut and the cook dialog.
func (m *Model) viewLab() string {
	d := &m.lab
	w := m.w
	cook := m.mode == modeCook
	title := "CUT · " + w.CityName(d.city)
	if cook {
		title = "COOK · " + w.CityName(d.city)
	}
	var body []string
	ids := m.labProducts()
	switch d.step {
	case 0:
		d.cursor = max(0, min(d.cursor, max(0, len(ids)-1)))
		if len(ids) == 0 {
			body = []string{theme.Subtle.Render("Nothing to do.")}
			break
		}
		m.modalFollow(1 + d.cursor)
		var rows [][]any
		if cook {
			for _, id := range ids {
				rows = append(rows, []any{w.ProductName(id), m.set.Market.CookCost(id), w.Product(d.city, id).SupplierPrice, w.Stock(d.city, id)})
			}
			body = table([]col{{"product", kText, 0}, {"cook/unit", kPrice, 0}, {"buy/unit", kPrice, 0}, {"have", kInt, 0}}, rows, d.cursor, m.modalInner())
			body = append(body, "", theme.Subtle.Render(fmt.Sprintf("%s cooks at quality %.0f, up to %d a batch, ready in %s.", m.set.Crew.ChemistName(w), m.set.Crew.ChemistQuality(w), m.set.Crew.Batch(w), plural(m.set.Crew.CookDays(), "day"))))
		} else {
			for _, id := range ids {
				l := w.Lot(d.city, id)
				rows = append(rows, []any{w.ProductName(id), l.Units, l.Quality, fmt.Sprintf("+%.0f%%", m.set.Market.CutMax(id)*100), m.set.Market.CutCost(id)})
			}
			body = table([]col{{"product", kText, 0}, {"have", kInt, 0}, {"quality", kInt, 0}, {"most", kText, 0}, {"$/unit", kPrice, 0}}, rows, d.cursor, m.modalInner())
			body = append(body, "", theme.Subtle.Render("Cut what?"))
		}
	default:
		qty := d.qty
		qty.max = m.labMax()
		if cook {
			chem := m.set.Crew.ChemistName(w)
			body = []string{
				fmt.Sprintf("%s cooks %s", chem, w.ProductName(d.product)),
				"",
				"Units     " + qty.View(),
				"",
			}
			body = append(body, m.subtle(fmt.Sprintf("Precursors are %s a unit, dirty, paid now; the lot lands in %s in %s at quality %.0f, %s's. A batch is %d.", price(float64(m.set.Market.CookCost(d.product))), w.CityName(d.city), plural(m.set.Crew.CookDays(), "day"), m.set.Crew.ChemistQuality(w), chem, m.set.Crew.Batch(w)))...)
			if n, err := parseQtyInput(d.qty.Value(), m.cookMax()); err == nil && n > 0 {
				body = append(body, theme.Subtle.Render(fmt.Sprintf("Cost      %s for %d, against %s from a connect", cash(n*m.set.Market.CookCost(d.product)), n, cash(int(float64(n)*w.Product(d.city, d.product).SupplierPrice)))))
			}
		} else {
			l := w.Lot(d.city, d.product)
			body = []string{
				fmt.Sprintf("%s: %d at quality %.0f", w.ProductName(d.product), l.Units, l.Quality),
				"",
				"Percent   " + qty.View(),
				"",
			}
			note := fmt.Sprintf("The cut adds that share of the units at nothing, so the quality falls by the same share; %s a unit added, dirty. The street pays full at quality %.0f and less under it; a corner sold under %.0f stops coming back.", price(float64(m.set.Market.CutCost(d.product))), w.StreetQuality(), m.set.Market.Tuning().RepeatFloor)
			if chem := w.Crew.Chemist(); chem != nil {
				note += fmt.Sprintf(" %s's hand keeps %.0f points of it.", chem.Name, m.set.Crew.CutBonus(w))
			}
			body = append(body, m.subtle(note)...)
			if pct, err := parseQtyInput(d.qty.Value(), m.cutMax()); err == nil && pct > 0 {
				units, quality, cost := m.cutPreview(pct)
				body = append(body, theme.Subtle.Render(fmt.Sprintf("After     %d units at quality %.0f, sells at ×%.2f, for %s", units, quality, m.set.Market.QualityMul(quality), cash(cost))))
			}
		}
	}
	if d.err != "" {
		body = append(body, "", theme.Bad.Render(d.err))
	}
	return m.modal(title, body, m.modalFooter())
}

// qualityCell is the market table's quality cell (#47): the lot's
// figure, blank with nothing held.
func qualityCell(w *game.World, city, id string) any {
	if w.Stock(city, id) <= 0 {
		return nil
	}
	q := w.Quality(city, id)
	if q < w.StreetQuality() {
		return styled{theme.Warning, int(q + 0.5)}
	}
	return int(q + 0.5)
}

// repeatText is a corner's repeat business as the map's pane reads it
// (#47): `all of them` at 1, else the share.
func repeatText(c game.Corner) string {
	r := c.Repeats()
	if r >= 1 {
		return "all of them come back"
	}
	return strings.TrimSpace(fmt.Sprintf("%.0f%% come back", r*100))
}
