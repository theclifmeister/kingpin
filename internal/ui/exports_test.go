package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// giveBook hands the run the Dutchman's book (#391), so the lane it
// opens is on the ledger.
func giveBook(m *Model) {
	b := m.cfg.Assets.ByEffect(content.AssetSupplier)
	if !m.w.HasAsset(b.ID) {
		m.w.Assets = append(m.w.Assets, game.Asset{ID: b.ID, Name: b.Name, Effect: b.Effect, City: b.City, Cost: b.Cost, Upkeep: b.Upkeep, Bought: m.w.Day})
	}
}

// onLane puts the ledger's cursor on the first open lane.
func onLane(t *testing.T, m *Model) {
	t.Helper()
	for i, r := range m.ledgerRows() {
		if r.kind == ledgerLane && m.rules.Logistics.LaneOpen(m.w, m.exportLanes()[r.i]) {
			m.ledgerCursor = i
			return
		}
	}
	t.Fatal("no open lane on the ledger")
}

// The export lanes in the grammar (#391): no EXPORTS block before the
// book; with it every lane is listed, the shut ones with what opens
// them; t on an open lane opens the order dialog, ←→ turns the
// product, the field takes the units and enter sets the order, which
// the ledger then shows; blank turns the lane off. The ledger fits at
// the three sizes throughout.
func TestExportsInTheGrammar(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		m.Update(key("7"))
		if strings.Contains(stripANSI(m.View()), "EXPORTS") || m.exportsShown() {
			t.Fatalf("%dx%d: EXPORTS on the ledger with no book", sz[0], sz[1])
		}
		giveBook(m)
		onLane(t, m)
		assertFrame(t, m, "ledger on a lane")
		view := stripANSI(m.View())
		for _, want := range []string{"EXPORTS", "Rotterdam"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%dx%d: the ledger lacks %q:\n%s", sz[0], sz[1], want, view)
			}
		}
		// The table reads as its columns say, in the vocabulary.
		hook := tableHook
		tableHook = func(cols []col, lines []string) { checkTable(t, cols, lines) }
		table(exportCols, m.exportTable(m.exportLanes()), -1, 200)
		tableHook = hook
		doc := headerKinds(t)
		for _, c := range exportCols {
			if doc[c.title] != kindName(c.kind) {
				t.Errorf("the header %q renders as %s; docs/format.md says %q", c.title, kindName(c.kind), doc[c.title])
			}
		}
		lane := *m.ledgerLaneSelected()
		m.Update(key("t"))
		if m.mode != modeExport {
			t.Fatalf("%dx%d: t on a lane: mode %v status %q", sz[0], sz[1], m.mode, m.status)
		}
		ps := m.laneProducts(lane)
		m.Update(key("right"))
		want := ps[1%len(ps)]
		m.Update(key("1"))
		m.Update(key("2"))
		m.Update(key("0"))
		m.Update(key("0"))
		m.Update(key("enter"))
		if m.mode != modePlay {
			t.Fatalf("%dx%d: enter left the dialog in %v: %q", sz[0], sz[1], m.mode, m.amt.err)
		}
		if o := m.w.ExportOrder(lane.ID); o.Product != want || o.Units != 1200 {
			t.Fatalf("%dx%d: the order is %+v, want 1200 %s", sz[0], sz[1], o, want)
		}
		if !strings.Contains(stripANSI(m.View()), m.w.ProductName(want)) {
			t.Fatalf("%dx%d: the ledger does not show the order", sz[0], sz[1])
		}
		m.Update(key("t"))
		for range 4 {
			m.Update(key("backspace"))
		}
		m.Update(key("enter"))
		if o := m.w.ExportOrder(lane.ID); o.On() {
			t.Fatalf("%dx%d: blank left the order at %+v", sz[0], sz[1], o)
		}
	}
}
