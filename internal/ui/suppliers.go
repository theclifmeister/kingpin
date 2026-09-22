package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The connects on the market screen (#72): a SUPPLIERS block under the
// buyers for the city shown, one row a connect with their price for the
// product under the cursor, their lot, what they have left today, the
// relationship, the credit they give and what you owe them, with its
// own cursor under the buyers' (supplierCursor, onSuppliers) and the
// connect's detail in the pane while it is there; the dashboard's debt
// alerts; the words the panes use for a connect.

// supplierRows are the connects the market screen lists for the city
// shown: every one seeded there, in order, locked ones included once
// the peak cash they want is moved (they say what they are waiting for).
func (m *Model) supplierRows() []*game.Supplier {
	var out []*game.Supplier
	for _, sup := range m.w.SuppliersIn(m.city) {
		if m.w.Stats.PeakCash < sup.UnlockCash {
			continue
		}
		out = append(out, sup)
	}
	return out
}

// selectedSupplier is the connect under the suppliers cursor, or nil
// when the cursor is on the product table or the buyers.
func (m *Model) selectedSupplier() *game.Supplier {
	rows := m.supplierRows()
	if !m.onSuppliers || len(rows) == 0 {
		return nil
	}
	return rows[clamp(&m.supplierCursor, len(rows))]
}

// suppliersLines is the SUPPLIERS block under the market's buyers: the
// title with the city, then one line a connect (`▸ Cass  $11.00  lot 50
// 1,500 left  rel ▮▮▮▯▯ 55  credit $3K  owe $1.2K by d9`), the
// selected one marked, and a warning in the row where they are frozen
// or have warned you.
func (m *Model) suppliersLines() []string {
	w := m.w
	rows := m.supplierRows()
	title := sectionTitle("SUPPLIERS", m.accent()) + theme.Subtle.Render(" · "+m.shown().Name)
	if len(rows) == 0 {
		return []string{title, theme.Subtle.Render("Nobody sells here. Yet.")}
	}
	id := ""
	if m.cursor < len(w.Products) {
		id = w.Products[m.cursor]
	}
	// The long form carries what they have left today and the
	// relationship as a bar; where MAIN is too narrow for every row
	// whole (64 columns beside the pane at 100), the short form drops
	// both, the number standing for the bar (firstFit).
	width := m.mainWidth()
	var long, short []string
	for i, sup := range rows {
		cur := "  "
		name := fit(sup.Name, 14)
		if m.onSuppliers && i == m.supplierCursor {
			cur = theme.Gold.Render("▸ ")
			name = theme.Selected.Render(name)
		}
		unit := theme.Subtle.Render(fmt.Sprintf("%8s", "-"))
		if id != "" && w.Available(sup, id) {
			unit = fmt.Sprintf("%8s", price(sup.Price[id]))
		}
		style := relStyle(m.rules.Market.Band(sup.Rel), m.rules.Market.Bands())
		bar := barText(sup.Rel/100, 5, nil, fmt.Sprintf(" %3.0f", sup.Rel), style)
		note := m.supplierNote(sup)
		long = append(long, fmt.Sprintf("%s%s %s  lot %-4d %5d left  rel %s  %s", cur, name, unit, sup.Lot, sup.Left(), bar, note))
		short = append(short, fmt.Sprintf("%s%s %s  lot %-4d rel %s  %s", cur, name, unit, sup.Lot, style.Render(fmt.Sprintf("%3.0f", sup.Rel)), note))
	}
	out := []string{title}
	for _, form := range [][]string{long, short} {
		fits := true
		for _, l := range form {
			fits = fits && lipgloss.Width(l) <= width
		}
		if fits {
			return append(out, form...)
		}
	}
	for _, l := range short {
		out = append(out, truncate(l, width))
	}
	return out
}

// supplierNote is the tail of a connect's row: what needs saying first,
// frozen, waiting for a word, out for the day, what you owe, a warning
// today, or the credit they give.
func (m *Model) supplierNote(sup *game.Supplier) string {
	w := m.w
	switch {
	case sup.Frozen(w.Day):
		return theme.Bad.Render(fmt.Sprintf("not taking calls, %dd", sup.FrozenUntil-w.Day))
	case sup.Locked(w):
		return theme.Subtle.Render("won't deal yet")
	case sup.Debt > 0:
		style := theme.Warning
		if sup.DebtDue <= w.Day+1 {
			style = theme.Bad
		}
		return style.Render(fmt.Sprintf("owe %s by d%d", cash(sup.Debt), sup.DebtDue))
	case sup.Warned == w.Day && w.Day > 0:
		return theme.Good.Render("has a tip: see the report")
	case sup.Left() == 0:
		return theme.Warning.Render("nothing left today")
	case sup.CreditDays > 0:
		return theme.Subtle.Render("credit " + cash(sup.Credit()))
	}
	return theme.Subtle.Render("cash only")
}

// supplierSections is the market's pane while the cursor is on a
// connect: who they are and where they stand with you, what they sell
// you today, their credit and your debt, what the relationship buys,
// and the keys.
func (m *Model) supplierSections(sup *game.Supplier) []section {
	w := m.w
	mk := m.rules.Market
	here := sup.City == w.Player.Location
	band, bands := mk.Band(sup.Rel), mk.Bands()
	sel := []string{
		row("temper", sup.Temper),
		row("", theme.Subtle.Render(temperShort(sup.Temper))),
		row("rel", barText(sup.Rel/100, 6, nil, fmt.Sprintf(" %.0f", sup.Rel), relStyle(band, bands))+sep+bandWord(band, bands)),
		row("price", fmt.Sprintf("~%.0f%% of street", mk.SupplierRatio(w, sup)*100)),
	}
	if band < bands-1 {
		sel = append(sel, row("", theme.Subtle.Render(fmt.Sprintf("~%.0f%% at rel %.0f", mk.RatioAt(w, sup, band+1)*100, float64(band+1)*mk.SuppliersTuning().BandWidth))))
	}
	sel = append(sel, row("lot", fmt.Sprintf("%d", sup.Lot)))
	if sup.SmallLot > 1 {
		sel = append(sel, row("", theme.Subtle.Render(fmt.Sprintf("under it ×%.2g a unit", sup.SmallLot))))
	}
	sel = append(sel, row("today", fmt.Sprintf("%d of %d left", sup.Left(), sup.Cap)))
	// What they sell is as good as the file says (#47): named where it
	// is not the default for a product they deal in.
	var graded []string
	for _, id := range w.Products {
		if q := sup.QualityOf(w, id); sup.Sells(id) && q != w.StreetQuality() {
			graded = append(graded, fmt.Sprintf("%s %.0f", w.ProductName(id), q))
		}
	}
	if len(graded) > 0 {
		sel = append(sel, row("quality", strings.Join(graded, ", ")))
	}
	if len(sup.Products) > 0 {
		var names []string
		for _, id := range sup.Products {
			names = append(names, w.ProductName(id))
		}
		sel = append(sel, row("deals in", strings.Join(names, ", ")))
	}
	switch {
	case sup.CreditDays <= 0:
		sel = append(sel, row("credit", theme.Subtle.Render("none")))
	default:
		sel = append(sel, row("credit", fmt.Sprintf("%s of %s", cash(sup.Credit()), cash(sup.Limit))),
			row("", theme.Subtle.Render(fmt.Sprintf("×%.2f/u, %dd to pay", sup.CreditRatio, sup.CreditDays))))
	}
	if sup.Debt > 0 {
		style := theme.Warning
		if sup.DebtDue <= w.Day+1 {
			style = theme.Bad
		}
		sel = append(sel, row("debt", style.Render(fmt.Sprintf("%s due d%d, %s", money(sup.Debt), sup.DebtDue, dueWord(sup.DebtDue-w.Day)))))
		if sup.Extended {
			sel = append(sel, row("", theme.Warning.Render("extended once already")))
		}
	}
	switch {
	case sup.Frozen(w.Day):
		sel = append(sel, row("status", theme.Bad.Render(fmt.Sprintf("no calls for %s", plural(sup.FrozenUntil-w.Day, "day")))))
	case sup.Locked(w) && w.Stats.PeakCash < sup.UnlockCash:
		sel = append(sel, row("status", theme.Subtle.Render("wants "+cash(sup.UnlockCash)+" moved")))
	case sup.Locked(w):
		if st := w.StreetSupplier(sup.City); st != nil {
			sel = append(sel, row("status", theme.Subtle.Render(fmt.Sprintf("needs %s at %.0f (%.0f)", st.Name, sup.UnlockRel, st.Rel))))
		}
	case sup.Warned == w.Day && w.Day > 0:
		sel = append(sel, row("status", theme.Good.Render("tipped you off today")))
	}
	if sup.Late > 0 {
		sel = append(sel, row("late", plural(sup.Late, "payment")+" missed"))
	}
	lt := w.Crew.Lieutenant(sup.City)
	switch {
	case here && sup.Open(w):
		sel = append(sel, keyRow("b", "buy from them"))
	case !here && lt != nil && sup.Open(w):
		sel = append(sel, keyRow("b", "buy through "+lt.Name+" at ×"+fmt.Sprintf("%.2f", mk.Markup()))) // the lieutenant runs the city (#174)
	case !here:
		sel = append(sel, keyRow("g", "go to "+w.CityName(sup.City)+" to buy"))
	}
	secs := []section{{strings.ToUpper(sup.Name) + " · " + strings.ToUpper(w.CityName(sup.City)), sel}}
	tun := mk.SuppliersTuning()
	rules := wrapped(theme.Subtle, fmt.Sprintf("Every lot bought is +%.2g rel; a debt cleared on its day +%.0f; a late one -%.0f, and %s. A bust that takes their product costs rel; under %.0f they stop taking calls. Left alone %s, they forget you.",
		tun.RelPerLot, tun.RelPaid, tun.RelLate, temperWords(sup.Temper), tun.FreezeRel, plural(tun.QuietDays, "day")))
	return append(secs, section{"RULES", rules})
}

// temperShort is a temper in three or four words, for the pane.
func temperShort(temper string) string {
	switch temper {
	case "patient":
		return "extends once"
	case "sharp":
		return "freezes, fees"
	case "connected":
		return "sends somebody"
	}
	return ""
}

// bandWord is where a relationship band stands, in a word.
func bandWord(band, bands int) string {
	n := bands / 2
	switch {
	case band == 0:
		return "the floor"
	case band < n:
		return "cool"
	case band == n:
		return "neutral"
	case band == bands-1:
		return "the top"
	}
	return "warm"
}

// dueWord is a due day in words: today, tomorrow, in N days, N days
// late.
func dueWord(days int) string {
	switch {
	case days < 0:
		return plural(-days, "day") + " late"
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	}
	return "in " + plural(days, "day")
}

// suppliersMove moves the suppliers cursor by d, or hands the arrows
// back up to the buyers, or the product table, off the top.
func (m *Model) suppliersMove(d int) {
	rows := m.supplierRows()
	if len(rows) == 0 {
		m.onSuppliers = false
		return
	}
	next := m.supplierCursor + d
	switch {
	case next < 0:
		m.onSuppliers, m.supplierCursor = false, 0
		if len(m.buyerRows()) > 0 {
			m.onBuyers, m.buyerCursor = true, len(m.buyerRows())-1
		}
	case next >= len(rows):
		m.supplierCursor = len(rows) - 1
	default:
		m.supplierCursor = next
	}
}

// debtAlerts are the connects' lines for the dashboard's ALERTS: a debt
// due tomorrow, the last morning to raise the cash (the market sim
// collects it at the top of the day it is due, so the morning it is
// due it is paid, or late and re-dated), keyed by the connect and the
// day, so a fast-forward (#116) stops on it once.
func (m *Model) debtAlerts() []alert {
	w := m.w
	var out []alert
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		if sup.Debt <= 0 || sup.DebtDue > w.Day+1 {
			continue
		}
		style := theme.Warning
		if w.Cash() < sup.Debt {
			style = theme.Bad
		}
		out = append(out, alert{
			text: style.Render(fmt.Sprintf("%s: %s due tomorrow, %s in hand.", sup.Name, money(sup.Debt), cash(w.Cash()))),
			why:  "debt due tomorrow",
			key:  fmt.Sprintf("debt %s due %d", sup.ID, sup.DebtDue),
		})
	}
	return out
}

// debtLine is the dashboard's fact on what you owe the connects: the
// total and the nearest day, or nothing with no debt.
func (m *Model) debtLine() string {
	w := m.w
	owed := w.Owed()
	if owed == 0 {
		return ""
	}
	due := 0
	for _, sup := range w.Suppliers {
		if sup.Debt > 0 && (due == 0 || sup.DebtDue < due) {
			due = sup.DebtDue
		}
	}
	style := theme.Warning
	if due <= w.Day+1 {
		style = theme.Bad
	}
	return style.Render(fmt.Sprintf("owe %s, due %s", cash(owed), dueWord(due-w.Day)))
}

// connectsHere is every connect in the buy's city (buyCity: where you
// stand, or the city a lieutenant runs for you, #174), in the order
// seeded: the buy dialog's connect step and what the picked index
// counts into.
func (m *Model) connectsHere() []*game.Supplier { return m.w.SuppliersIn(m.buyCity()) }

// dealing reports whether a connect is open for business with you
// today: unlocked, taking calls, with something left and a product to
// sell here.
func (m *Model) dealing(sup *game.Supplier) bool {
	for _, id := range m.w.Products {
		if m.w.Available(sup, id) {
			return true
		}
	}
	return false
}

// sellsYou reports whether a connect sells you anything today: some
// product is available from them and you could take at least a unit of
// it, for cash or on their book.
func (m *Model) sellsYou(sup *game.Supplier) bool {
	for _, id := range m.w.Products {
		if m.maxBuyFrom(sup, id, false) > 0 || m.maxBuyFrom(sup, id, true) > 0 {
			return true
		}
	}
	return false
}

// whyNobodySells is the refusal when no connect where you stand is
// dealing today: nobody here, frozen, out of stock for the day, or
// locked.
func (m *Model) whyNobodySells() string {
	w := m.w
	city := m.buyCity()
	cs := m.connectsHere()
	if len(cs) == 0 {
		return "nobody sells in " + w.CityName(city)
	}
	for _, sup := range cs {
		if sup.Frozen(w.Day) {
			return sup.Name + " is not taking your calls for " + plural(sup.FrozenUntil-w.Day, "day")
		}
	}
	for _, sup := range cs {
		if sup.Open(w) && sup.Left() == 0 {
			return sup.Name + " has nothing left today"
		}
	}
	return "nobody in " + w.CityName(city) + " will deal with you yet"
}

// buySupplier is the connect the buy dialog buys a product from: the
// one picked on the connect step where there was one, else the one
// connect that sells you something, else the cheapest that sells the
// product today.
func (m *Model) buySupplier(id string) *game.Supplier {
	if cs := m.connectsHere(); m.mode == modeBuy && m.dlg.supplier >= 0 && m.dlg.supplier < len(cs) {
		return cs[m.dlg.supplier]
	}
	return m.w.BestSupplier(m.buyCity(), id)
}

// creditOffered reports whether the buy dialog's connect will run you
// anything today.
func (m *Model) creditOffered() bool {
	sup := m.buySupplier(m.w.Products[m.cursor])
	return sup != nil && sup.Credit() > 0
}

// maxBuyFrom is maxBuyBy from a named connect.
func (m *Model) maxBuyFrom(sup *game.Supplier, id string, credit bool) int {
	if sup == nil || !m.w.Available(sup, id) {
		return 0
	}
	return max(0, min(m.affordFrom(sup, id, credit), m.w.Free(sup.City), sup.Left()))
}

// affordFrom is how many units of a product the cash, or the connect's
// book, covers at their quote (World.Quote: the markup on it where the
// buy goes through a lieutenant, #174): the plain price first, then
// the small-lot premium once the buy is under the lot.
func (m *Model) affordFrom(sup *game.Supplier, id string, credit bool) int {
	unit := sup.Price[id] * m.w.BuyMarkup(sup.City)
	if unit <= 0 {
		return 0
	}
	cash := m.w.Player.DirtyCash
	if credit {
		cash = sup.Credit()
		unit *= sup.CreditRatio
	}
	n := int(math.Floor(float64(cash) / unit))
	if n < sup.Lot && sup.SmallLot > 1 {
		n = int(math.Floor(float64(cash) / (unit * sup.SmallLot)))
	}
	for n > 0 && m.w.Quote(sup, id, n, credit) > cash {
		n--
	}
	return n
}

// temperWords is what a connect's temper does about a missed payment,
// for the credit note and the pane.
func temperWords(temper string) string {
	switch temper {
	case "patient":
		return "they let it ride once, then stop taking your calls"
	case "sharp":
		return "they stop taking your calls and add a fee"
	case "connected":
		return "they send somebody for your muscle"
	}
	return "they remember"
}

// connectTable is the buy dialog's connect step (#72): one row a
// connect where you stand, their price for the product, the lot, what
// they have left today, the relationship and the credit they give, or
// why they sell you nothing.
func (m *Model) connectTable(id string, cursor int) []string {
	w := m.w
	cols := []col{{"connect", kText, 0}, {"price", kPrice, 0}, {"lot", kInt, 0}, {"left", kInt, 0}, {"rel", kBar, 8}, {"credit", kCash, 0}, {"", kText, 0}}
	var rows [][]any
	for _, sup := range m.connectsHere() {
		var unit any
		if w.Available(sup, id) {
			unit = sup.Price[id]
		}
		rows = append(rows, []any{sup.Name, unit, sup.Lot, sup.Left(), styled{relStyle(m.rules.Market.Band(sup.Rel), m.rules.Market.Bands()), gauge{frac: sup.Rel / 100, n: sup.Rel}}, sup.Credit(), m.connectStatus(sup)})
	}
	return table(cols, rows, cursor, m.modalInner())
}

// connectStatus is a connect's state in a word or two: frozen, locked,
// sold out for the day, owing, or nothing.
func (m *Model) connectStatus(sup *game.Supplier) string {
	w := m.w
	switch {
	case sup.Frozen(w.Day):
		return theme.Bad.Render(fmt.Sprintf("frozen %dd", sup.FrozenUntil-w.Day))
	case sup.Locked(w):
		return theme.Subtle.Render("won't deal yet")
	case sup.Left() == 0:
		return theme.Warning.Render("nothing left today")
	case sup.Debt > 0:
		return theme.Warning.Render(fmt.Sprintf("owe %s by d%d", cash(sup.Debt), sup.DebtDue))
	}
	return ""
}

// connectBlurb is the connect step's note on the connect under the
// cursor: their temper, what they deal in, and the door if it is shut.
func (m *Model) connectBlurb(sup *game.Supplier, id string) []string {
	w := m.w
	deals := "everything sold here"
	if len(sup.Products) > 0 {
		var names []string
		for _, pid := range sup.Products {
			names = append(names, w.ProductName(pid))
		}
		deals = strings.Join(names, ", ")
	}
	lines := []string{theme.Subtle.Render(fmt.Sprintf("%s: %s, deals in %s by the %d.", sup.Name, sup.Temper, deals, sup.Lot))}
	switch {
	case sup.Locked(w) && w.Stats.PeakCash < sup.UnlockCash:
		lines = append(lines, theme.Warning.Render(fmt.Sprintf("They deal with people who have moved %s.", cash(sup.UnlockCash))))
	case sup.Locked(w):
		if st := w.StreetSupplier(sup.City); st != nil {
			lines = append(lines, theme.Warning.Render(fmt.Sprintf("They want a word from %s first: rel %.0f, they need %.0f.", st.Name, st.Rel, sup.UnlockRel)))
		}
	case sup.Frozen(w.Day):
		lines = append(lines, theme.Bad.Render(fmt.Sprintf("Not taking your calls for %s.", plural(sup.FrozenUntil-w.Day, "day"))))
	}
	return lines
}

// relStyle colours a relationship by its band: the floor red, under
// neutral a warning, neutral plain, over it good.
func relStyle(band, bands int) lipgloss.Style {
	n := bands / 2
	switch {
	case band == 0:
		return theme.Bad
	case band < n:
		return theme.Warning
	case band > n:
		return theme.Good
	}
	return theme.Subtle
}
