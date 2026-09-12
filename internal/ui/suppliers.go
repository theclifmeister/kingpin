package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
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
	if m.supplierCursor >= len(rows) {
		m.supplierCursor = len(rows) - 1
	}
	return rows[m.supplierCursor]
}

// suppliersLines is the SUPPLIERS block under the market's buyers: the
// title with the city, then one line a connect (`▸ Cass  $11.00  lot 50
// 1,500 left  rel ▮▮▮▯▯ 55  credit $3K  owe $1.2K by d9`), the
// selected one marked, and a warning in the row where they are frozen
// or have warned you.
func (m *Model) suppliersLines() []string {
	w := m.w
	rows := m.supplierRows()
	title := sectionTitle("SUPPLIERS", theme.Market) + theme.Subtle.Render(" · "+m.shown().Name)
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
		style := relStyle(m.set.Market.Band(sup.Rel), m.set.Market.Bands())
		bar := style.Render(sparkline.Bar(sup.Rel/100, 5, nil) + fmt.Sprintf(" %3.0f", sup.Rel))
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
	mk := m.set.Market
	here := sup.City == w.Player.Location
	band, bands := mk.Band(sup.Rel), mk.Bands()
	sel := []string{
		row("temper", sup.Temper),
		row("", theme.Subtle.Render(temperShort(sup.Temper))),
		row("rel", relStyle(band, bands).Render(sparkline.Bar(sup.Rel/100, 6, nil)+fmt.Sprintf(" %.0f", sup.Rel))+sep+bandWord(band, bands)),
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
	if here && sup.Open(w) {
		sel = append(sel, keyRow("b", "buy from them"))
	} else if !here {
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
