package ui

import (
	"regexp"
	"strings"
	"testing"
)

// tableLine finds the row of a rendered table that starts with name
// (after the gutter) in the last table drawn whose columns include
// title, split into its cells by the widths it was drawn at; the
// index of the titled column comes with it.
func tableLine(t *testing.T, m *Model, title, name string) (cell []string, at int) {
	t.Helper()
	var found []string
	at = -1
	hook := tableHook
	tableHook = func(cols []col, lines []string) {
		i := -1
		for j, c := range cols {
			if c.title == title {
				i = j
			}
		}
		if i < 0 {
			return
		}
		for _, l := range lines[1:] {
			if cs := cells(cols, l); cs[0] == name {
				found, at = cs, i
			}
		}
	}
	m.View()
	tableHook = hook
	if found == nil {
		t.Fatalf("no row %q in a table with a %q column:\n%s", name, title, stripANSI(m.View()))
	}
	return found, at
}

// TestDialogShowsTheDelta (#138): the sell dialog on a product whose
// price moved yesterday shows the market table's Δ cell on the product
// step and the price line on the quantity and dial steps; the buy
// dialog shows the supplier price, the street price and the margin the
// pane shows; the cart shows Δ on a sell line and blank on a buy line.
func TestDialogShowsTheDelta(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	weed := w.Products[0]
	p := w.Product(home, weed)
	// Yesterday's close 20, today's 22: +10% on the day.
	p.History = []float64{18, 19, 20, 22}
	p.Price = 22
	p.SupplierPrice = 12
	for _, sup := range w.SuppliersIn(home) {
		sup.Price[weed] = 12 // the connects' price is what the market's supplier price reads (#72)
	}
	name := w.ProductName(weed)

	m.Update(key("2"))
	market, at := tableLine(t, m, "Δ", name)
	if market[at] != "+10%" {
		t.Fatalf("the market table's Δ for %s is %q, want +10%%", name, market[at])
	}
	pane := stripANSI(m.View())
	if !strings.Contains(pane, "range 30d   $18.00 – $22.00") || !strings.Contains(pane, "margin      +83% over supplier") {
		t.Fatalf("the market's pane does not show the range and the margin:\n%s", pane)
	}

	// The sell dialog: the same Δ cell on the product step, the price
	// line on the quantity and dial steps.
	m.Update(key("s"))
	if m.mode != modeSell {
		t.Fatalf("s: mode %v, %q", m.mode, m.status)
	}
	sell, at := tableLine(t, m, "Δ", name)
	if sell[at] != market[at] {
		t.Errorf("the sell dialog's Δ for %s is %q, the market's %q", name, sell[at], market[at])
	}
	if sell[1] != "$22.00" {
		t.Errorf("the sell dialog's price for %s is %q, want $22.00", name, sell[1])
	}
	line := "price      $22.00 · +10% today · range 30d $18.00 – $22.00"
	if strings.Contains(stripANSI(m.View()), line) {
		t.Errorf("the price line is on the product step:\n%s", stripANSI(m.View()))
	}
	for _, step := range []string{"quantity", "dial"} {
		m.Update(key("enter"))
		if v := stripANSI(m.View()); !strings.Contains(v, line) {
			t.Errorf("the sell dialog's %s step lacks %q:\n%s", step, line, v)
		}
	}
	m.Update(key("esc"))

	// The buy dialog: the supplier price, the street price and the
	// margin the pane shows, then the supplier line.
	m.Update(key("1"))
	dash := stripANSI(m.View())
	if !strings.Contains(dash, "supplier    $12.00 · margin +83%") {
		t.Fatalf("the dashboard's pane does not show the supplier and the margin:\n%s", dash)
	}
	m.Update(key("b"))
	if m.mode != modeBuy {
		t.Fatalf("b: mode %v, %q", m.mode, m.status)
	}
	buy, at := tableLine(t, m, "margin", name)
	if buy[1] != "$12.00" || buy[2] != "$22.00" || buy[at] != "+83%" {
		t.Errorf("the buy dialog's row for %s reads %q, want $12.00 $22.00 +83%%", name, buy)
	}
	if d, _ := tableLine(t, m, "Δ", name); d[len(d)-3] != "+10%" {
		t.Errorf("the buy dialog's Δ for %s is %q, want +10%%", name, d[len(d)-3])
	}
	m.Update(key("enter"))
	if v := stripANSI(m.View()); !strings.Contains(v, "supplier   $12.00 · street $22.00 · margin +83%") {
		t.Errorf("the buy dialog's quantity step lacks the supplier line:\n%s", v)
	}
	m.Update(key("esc"))

	// The cart: Δ on a sell line, blank on a buy line, and the selected
	// line's price sentence.
	fillCart(t, m)
	m.Update(key("c"))
	if m.mode != modeCart {
		t.Fatalf("c: mode %v, %q", m.mode, m.status)
	}
	var rows [][]string
	var delta int
	hook := tableHook
	tableHook = func(cols []col, lines []string) {
		if cols[0].title != "line" {
			return
		}
		for i, c := range cols {
			if c.title == "Δ" {
				delta = i
			}
		}
		rows = nil
		for _, l := range lines[1:] {
			rows = append(rows, cells(cols, l))
		}
	}
	m.View()
	tableHook = hook
	if len(rows) == 0 {
		t.Fatalf("no cart table:\n%s", stripANSI(m.View()))
	}
	for _, r := range rows {
		switch {
		case r[0] == "sell" && r[1] == name:
			if r[delta] != "+10%" {
				t.Errorf("the cart's Δ on the %s sell line is %q, want +10%%", name, r[delta])
			}
		case r[0] == "buy" || r[0] == "contract":
			if r[delta] != "-" {
				t.Errorf("the cart's Δ on a %s line is %q, want blank", r[0], r[delta])
			}
		}
	}
	// The first line is a buy: the supplier sentence; the first sell
	// line the price sentence, on both steps.
	if v := stripANSI(m.View()); !regexp.MustCompile(`supplier   \$[\d.,]+ · street \$[\d.,]+ · margin [+-]\d+%`).MatchString(v) {
		t.Errorf("the cart on a buy line lacks the supplier sentence:\n%s", v)
	}
	for i, r := range rows {
		if r[0] == "sell" && r[1] == name {
			for j := 0; j < i; j++ {
				m.Update(key("j"))
			}
			break
		}
	}
	for _, step := range []string{"lines", "quantity"} {
		if v := stripANSI(m.View()); !strings.Contains(v, line) {
			t.Errorf("the cart's %s step on the %s sell line lacks %q:\n%s", step, name, line, v)
		}
		m.Update(key("enter"))
	}
}

// TestDialogPriceMatchesPane (#138): the dialogs and the cart read the
// pane's numbers through one helper, so for every product in the city
// the sell dialog's range, the buy dialog's margin and the shock or
// slump while one runs are the market pane's, character for character.
func TestDialogPriceMatchesPane(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	// A shock on the first product and a slump on the second, so both
	// rows are read.
	p0, p1 := w.Product(home, w.Products[0]), w.Product(home, w.Products[1])
	p0.ShockDays, p0.ShockFactor, p0.ShockSlump = 3, 1.4, false
	p1.ShockDays, p1.ShockFactor, p1.ShockSlump = 1, 0.6, true
	rng := regexp.MustCompile(`range 30d +(\$[\d.,]+ – \$[\d.,]+)`)
	margin := regexp.MustCompile(`margin +([+-]\d+%) over supplier`)
	shock := regexp.MustCompile(`(shock|slump) +(×[\d.]+, \d+ days? more)`)
	m.Update(key("2"))
	for i, id := range w.Products {
		if w.Stock(home, id) == 0 {
			w.Stash(home)[id] = 10
		}
		m.cursor = i
		pane := stripANSI(m.View())
		r := rng.FindStringSubmatch(pane)
		mg := margin.FindStringSubmatch(pane)
		sh := shock.FindStringSubmatch(pane)
		if r == nil {
			t.Fatalf("no range in the pane for %s:\n%s", w.ProductName(id), pane)
		}
		m.Update(key("s"))
		m.Update(key("enter"))
		if m.mode != modeSell || m.dlg.step != 1 {
			t.Fatalf("%s: the sell dialog is on mode %v step %d: %q %q", w.ProductName(id), m.mode, m.dlg.step, m.dlg.err, m.status)
		}
		v := stripANSI(m.View())
		if !strings.Contains(v, "range 30d "+r[1]) {
			t.Errorf("%s: the sell dialog's range is not the pane's %q:\n%s", w.ProductName(id), r[1], v)
		}
		if sh != nil && !strings.Contains(v, sh[1]+"      "+sh[2]) {
			t.Errorf("%s: the sell dialog does not name the %s as the pane does (%q):\n%s", w.ProductName(id), sh[1], sh[2], v)
		}
		m.Update(key("esc"))
		if mg == nil {
			continue // not sold here: nothing to buy
		}
		m.Update(key("b"))
		m.Update(key("enter"))
		if m.mode != modeBuy || m.dlg.step != 1 {
			t.Fatalf("%s: the buy dialog is on mode %v step %d: %q %q", w.ProductName(id), m.mode, m.dlg.step, m.dlg.err, m.status)
		}
		v = stripANSI(m.View())
		if !strings.Contains(v, "margin "+mg[1]) {
			t.Errorf("%s: the buy dialog's margin is not the pane's %q:\n%s", w.ProductName(id), mg[1], v)
		}
		if sh != nil && !strings.Contains(v, sh[1]+"      "+sh[2]) {
			t.Errorf("%s: the buy dialog does not name the %s as the pane does (%q):\n%s", w.ProductName(id), sh[1], sh[2], v)
		}
		m.Update(key("esc"))
	}
}
