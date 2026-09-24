package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The export lanes (#391, docs/exports.md) on the ledger. The EXPORTS
// block sits under ASSETS once the Dutchman's book stands or a load has
// gone: every lane in the file, the open ones with their standing
// order and the rate abroad tonight, the shut ones with the asset that
// opens them, under the ledger's one cursor. `t` on a lane opens the
// order dialog: the product (←→) and the units a night (a number
// field; blank or zero turns the lane off).

// exportsShown reports whether the ledger carries the EXPORTS block: a
// lane open, or a load out or on the record, so a run that never owns
// the book reads the ledger it always did.
func (m *Model) exportsShown() bool {
	w := m.w
	return len(m.rules.Logistics.LanesOpen(w)) > 0 || len(w.Exports.Loads) > 0 || len(w.Exports.Record) > 0
}

// exportLanes are the EXPORTS block's rows: every lane in the file.
func (m *Model) exportLanes() []content.LaneConfig { return m.rules.Logistics.Lanes() }

// ledgerOnLane reports whether the ledger's cursor is on a lane.
func ledgerOnLane(m *Model) bool {
	return m.screen == screenLedger && m.ledgerSelected().kind == ledgerLane
}

// ledgerLaneSelected is the lane under the ledger's cursor, or nil.
func (m *Model) ledgerLaneSelected() *content.LaneConfig {
	sel := m.ledgerSelected()
	lanes := m.exportLanes()
	if sel.kind != ledgerLane || sel.i >= len(lanes) {
		return nil
	}
	return &lanes[sel.i]
}

// exportCols are the EXPORTS table's columns: the lane, where it leaves
// from, the order (product and units a night), the rate a unit tonight,
// the days out and the status; where MAIN is too narrow the city goes,
// then the days (the pane carries both).
var exportCols = []col{{"line", kText, 0}, {"city", kText, 0}, {"product", kText, 0}, {"units", kInt, 0}, {"price", kPrice, 0}, {"days", kDays, 0}, {"status", kText, 0}}

// laneAsset is the name of the asset a shut lane waits on: its own, or
// the book when that is what is missing.
func (m *Model) laneAsset(l content.LaneConfig) string {
	w := m.w
	if b := m.cfg.Assets.ByEffect(content.AssetSupplier); b != nil && !w.AssetLive(b.ID) {
		return b.Name
	}
	if a := m.cfg.Assets.Asset(l.Asset); a != nil && !w.AssetLive(a.ID) {
		return a.Name
	}
	return ""
}

// laneStatus is a lane's state for the status column: shut with what
// opens it, off, or the loads out and the next one landing.
func (m *Model) laneStatus(l content.LaneConfig) any {
	w := m.w
	lg := m.rules.Logistics
	if !lg.LaneOpen(w, l) {
		return styled{theme.Subtle, "needs " + m.laneAsset(l)}
	}
	out := w.ExportsOut(l.ID)
	if len(out) == 0 {
		if !w.ExportOrder(l.ID).On() {
			return styled{theme.Subtle, "idle · t to order"}
		}
		return styled{theme.Good, "loading tonight"}
	}
	return styled{theme.Good, fmt.Sprintf("%d out · lands d%d", len(out), out[0].Lands)}
}

// exportTable is the EXPORTS block's rows.
func (m *Model) exportTable(lanes []content.LaneConfig) [][]any {
	w := m.w
	lg := m.rules.Logistics
	var out [][]any
	for _, l := range lanes {
		o := w.ExportOrder(l.ID)
		product, units, abroad := "off", 0, 0.0
		if o.On() {
			product, units, abroad = w.ProductName(o.Product), o.Units, lg.ExportPrice(w, l, o.Product)
		}
		out = append(out, []any{l.Name, w.CityName(l.City), product, units, abroad, l.Days, m.laneStatus(l)})
	}
	return out
}

// exportsNote is the EXPORTS heading's note: what the lanes have paid.
func (m *Model) exportsNote() string {
	st := m.w.Stats
	return fmt.Sprintf(" · %s out · landed %s lifetime · %s seized", plural(len(m.w.Exports.Loads), "load"), cash(st.ExportCash), plural(st.ExportsSeized, "load"))
}

// laneSection is a lane in the pane: what it is, what it carries and
// pays tonight, the loads out and how to order.
func (m *Model) laneSection(l content.LaneConfig) section {
	w := m.w
	lg := m.rules.Logistics
	lines := []string{}
	status, _ := cellText(kText, 0, m.laneStatus(l))
	lines = append(lines, m.laneStatus(l).(styled).st.Render(status))
	lines = append(lines, wrapped(theme.Subtle, fmt.Sprintf("Bought off the Dutchman's book and sent out of %s by %s: it never touches a stash or a corner, and lands paid in dirty cash.", w.CityName(l.City), l.Mode))...)
	lines = append(lines,
		row("carries", fmt.Sprintf("up to %s a night", plural(lg.LaneCapacity(w, l), "unit"))),
		row("out", plural(l.Days, "day")+" at sea"),
	)
	if l.Mode == "plane" {
		lines[len(lines)-1] = row("out", plural(l.Days, "day")+" in the air")
	}
	lines = append(lines, row("seized", format.Pct(lg.LaneRisk(w, l, w.Day), 1)+" a load"))
	if lg.Watched(w, w.Day) {
		lines = append(lines, theme.Warning.Render("The feds are watching: every lane runs hot."))
	}
	if lg.LaneOpen(w, l) {
		for _, id := range l.Products {
			cost, abroad := lg.ExportCost(w, id), lg.ExportPrice(w, l, id)
			if cost <= 0 || abroad <= 0 {
				continue
			}
			note := fmt.Sprintf("%s abroad on %s off the book", price(abroad), price(cost))
			if g := w.Glut(l.ID, id); g > 0 {
				note += fmt.Sprintf(" · glut -%s", format.Pct(g, 0))
			}
			lines = append(lines, row(w.ProductName(id), note))
		}
		if o := w.ExportOrder(l.ID); o.On() {
			units, cost := lg.LoadTonight(w, l, lg.Budget(w))
			lines = append(lines, row("tonight", fmt.Sprintf("%d %s for %s", units, w.ProductName(o.Product), money(cost))))
		}
		lines = append(lines, keyRow("t", "set the order"))
	}
	for _, ld := range w.ExportsOut(l.ID) {
		lines = append(lines, row(fmt.Sprintf("d%d", ld.Lands), fmt.Sprintf("%d %s · %s due", ld.Units, w.ProductName(ld.Product), money(ld.Revenue()))))
	}
	return section{strings.ToUpper(l.Name), lines}
}

// askExport opens the order dialog on the lane under the cursor.
func (m *Model) askExport() {
	if m.w.Over != nil {
		return
	}
	l := m.ledgerLaneSelected()
	if l == nil {
		return
	}
	if !m.rules.Logistics.LaneOpen(m.w, *l) {
		m.refuse(fmt.Sprintf("Can't order: the %s needs %s.", l.Name, m.laneAsset(*l)))
		return
	}
	o := m.w.ExportOrder(l.ID)
	m.expProduct = 0
	for i, id := range m.laneProducts(*l) {
		if id == o.Product {
			m.expProduct = i
		}
	}
	m.openAmount(modeExport, "blank = off", m.rules.Logistics.LaneCapacity(m.w, *l), false, l.ID)
	if o.On() {
		m.amt.Set(o.Units)
	}
}

// laneProducts are the products the lane ships that are on the
// market: what the dialog's ←→ walks.
func (m *Model) laneProducts(l content.LaneConfig) []string {
	var out []string
	for _, id := range l.Products {
		if m.rules.Logistics.ExportCost(m.w, id) > 0 {
			out = append(out, id)
		}
	}
	return out
}

// exportLane is the lane the dialog is open on, or nil.
func (m *Model) exportLane() *content.LaneConfig { return m.rules.Logistics.Lane(m.amt.subject) }

func (m *Model) keyExport(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	l := m.exportLane()
	if l == nil {
		m.mode = modePlay
		return m, nil
	}
	if ps := m.laneProducts(*l); len(ps) > 0 {
		switch k.String() {
		case "left", "right":
			d := 1
			if k.String() == "left" {
				d = -1
			}
			m.expProduct = (m.expProduct + d + len(ps)) % len(ps)
			return m, nil
		}
	}
	return m.keyAmount(k, func() int { return m.rules.Logistics.LaneCapacity(m.w, *l) }, m.confirmExport)
}

// confirmExport sets the order, or says why it cannot.
func (m *Model) confirmExport() {
	l := m.exportLane()
	ps := m.laneProducts(*l)
	n, ok := m.amt.Number()
	if !ok {
		m.amt.err = dialogError(errNotAWholeNumber)
		return
	}
	product := ""
	if len(ps) > 0 {
		product = ps[m.expProduct%len(ps)]
	}
	if err := m.sess.SetExport(l.ID, product, n); err != nil {
		m.amt.err = dialogError(err)
		return
	}
	m.mode = modePlay
	if n == 0 || product == "" {
		m.say(fmt.Sprintf("The %s is off: nothing leaves on it.", l.Name))
		return
	}
	m.say(fmt.Sprintf("The %s loads %d %s a night from tonight, as the till allows.", l.Name, n, m.w.ProductName(product)))
}

// viewExport is the order dialog: the product, the units a night and
// what tonight's load costs and would fetch.
func (m *Model) viewExport() string {
	w := m.w
	l := m.exportLane()
	if l == nil {
		return m.modal("EXPORT", []string{"No lane."}, m.modalFooter())
	}
	lg := m.rules.Logistics
	ps := m.laneProducts(*l)
	body := []string{
		theme.Subtle.Render(fmt.Sprintf("Leaves %s every night with what the order and the till allow.", w.CityName(l.City))),
		"",
	}
	product := ""
	if len(ps) > 0 {
		product = ps[m.expProduct%len(ps)]
		names := make([]string, len(ps))
		for i, id := range ps {
			names[i] = w.ProductName(id)
		}
		body = append(body, row("product", dialCells(names, m.expProduct%len(ps))))
	}
	body = append(body, row("units", m.amountField(lg.LaneCapacity(w, *l)).View()+theme.Subtle.Render(" a night")))
	if n, ok := m.amt.Number(); ok && n > 0 && product != "" {
		cost, abroad := lg.ExportCost(w, product), lg.ExportPrice(w, *l, product)
		units := min(n, lg.LaneCapacity(w, *l))
		body = append(body, row("tonight", fmt.Sprintf("%s off the book, %s on landing", money(int(cost*float64(units))), theme.Gold.Render(money(int(abroad*float64(units)))))))
	}
	body = append(body, "", theme.Subtle.Render(fmt.Sprintf("Lands in %s, paid in dirty cash; %s seized. Blank is off.", plural(l.Days, "day"), format.Pct(lg.LaneRisk(w, *l, w.Day), 1))))
	if m.amt.err != "" {
		body = append(body, "", theme.Bad.Render(m.amt.err))
	}
	return m.modal("EXPORT · "+strings.ToUpper(l.Name), body, m.modalFooter())
}
