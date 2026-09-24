package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The restock dialog (#356): the market's one action that fills the
// stash for the next few days. One page, a number field in days of the
// demand the corners you work there serve (blank is restockDays), and
// under it the plan it would buy (Session.RestockPlan, the stocked
// policy's sizing, World.StockLevels), redrawn as the number changes:
// a line a product, the total, and the cash and the room after. Enter
// buys the lines by hand, one Buy each, so they land in the cart,
// where each can be cut or returned before the day ends; nothing is
// bought before enter. It is an amountDialog (#275), its subject the
// city.

const (
	restockDays    = 2  // a blank field: tonight's sales and tomorrow's
	restockDaysMax = 30 // the field's max
)

// restockCols is the plan's table.
var restockCols = []col{{"product", kText, 0}, {"level", kInt, 0}, {"have", kInt, 0}, {"buy", kInt, 0}, {"cost", kMoney, 0}}

// askRestock opens the restock dialog on the city a buy on the market
// would be in (buyCity: where you stand, or the shown city a lieutenant
// runs for you, #174).
func (m *Model) askRestock() {
	if m.w.Over != nil {
		return
	}
	if why := m.buyPointer(); why != "" {
		m.refuse(why)
		return
	}
	if why := m.cannotOpen(modeBuy); why != "" {
		m.refuse(why)
		return
	}
	m.openAmount(modeRestock, fmt.Sprintf("blank = %d", restockDays), restockDaysMax, false, m.buyCity())
}

// restockDaysRead is the days the field reads: restockDays for a
// blank, an error for a number that does not read or is past the max.
func (m *Model) restockDaysRead() (int, error) {
	n, err := m.amt.Read(restockDays)
	if err != nil {
		return 0, err
	}
	if n > restockDaysMax {
		return 0, fmt.Errorf("up to %d days at a time", restockDaysMax)
	}
	return n, nil
}

func (m *Model) keyRestock(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.keyAmount(k, func() int { return restockDaysMax }, m.confirmRestock)
}

// confirmRestock buys the plan a line at a time, or shows why not.
func (m *Model) confirmRestock() {
	days, err := m.restockDaysRead()
	if err != nil {
		m.amt.err = dialogError(err)
		return
	}
	city := m.amt.subject
	plan := m.sess.RestockPlan(city, float64(days))
	if len(plan) == 0 {
		m.amt.err = "Nothing to buy: " + m.restockNothing(city) + "."
		return
	}
	lines, spent := 0, 0
	var refused error
	for _, l := range plan {
		p, err := m.sess.Buy(l.Supplier, l.Product, l.Units, false)
		if err != nil {
			refused = fmt.Errorf("%s: %w", m.w.ProductName(l.Product), err)
			continue
		}
		lines++
		spent += p.Cost
	}
	m.mode = modePlay
	say := fmt.Sprintf("Restocked %s in %s for %s, in the cart until the day ends.", plural(lines, "line"), m.w.CityName(city), money(spent))
	if refused != nil {
		m.alarm(say + " " + dialogError(refused))
		return
	}
	m.say(say)
}

// restockNothing is why a plan is empty, as the dialog says it.
func (m *Model) restockNothing(city string) string {
	w := m.w
	switch {
	case w.Free(city) <= 0:
		return "the stash in " + w.CityName(city) + " is full"
	case w.Player.DirtyCash <= 0:
		return "no dirty cash to spend"
	}
	return "the stash holds that much of what your corners sell already"
}

// viewRestock is the dialog: the field, the plan it would buy, the
// total, and the cash and the room after.
func (m *Model) viewRestock() string {
	w := m.w
	city := m.amt.subject
	days, err := m.restockDaysRead()
	if err != nil {
		days = restockDays
	}
	field := m.amountField(restockDaysMax)
	body := []string{
		fmt.Sprintf("Top the stash in %s up to %s of what your corners sell.", w.CityName(city), plural(days, "day")),
		"",
		m.inHand(),
		row("days", field.View()),
		"",
	}
	plan := m.sess.RestockPlan(city, float64(days))
	if len(plan) == 0 {
		body = append(body, theme.Subtle.Render("Nothing to buy: "+m.restockNothing(city)+"."))
	} else {
		var rows [][]any
		units, cost := 0, 0
		for _, l := range plan {
			rows = append(rows, []any{w.ProductName(l.Product), l.Level, l.Have, l.Units, styled{theme.Bad, -l.Cost}})
			units += l.Units
			cost += l.Cost
		}
		body = append(body, table(restockCols, rows, -1, m.modalInner())...)
		body = append(body, "", row("total", theme.Gold.Render(money(cost))+" for "+plural(units, "unit")))
		body = append(body, m.restockAfter(city, units, cost))
	}
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("RESTOCK", body, m.modalFooter())
}

// restockAfter is the plan's `after` row, the buy dialog's (afterRow)
// for the whole plan: never over, since the plan is cut to both.
func (m *Model) restockAfter(city string, units, cost int) string {
	w := m.w
	return row("after", fmt.Sprintf("%s dirty · stash %d/%d in %s", cash(w.Player.DirtyCash-cost), w.StockIn(city)+units, w.Capacity(city), w.CityName(city)))
}
