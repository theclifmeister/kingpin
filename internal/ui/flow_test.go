package ui

import (
	"strings"
	"testing"
)

// TestReportLeadsWithTheFlow (#351): the report's MONEY section opens on
// the night's cash flow, the opening, the categories that moved and the
// closing (the piles as they stand), then the itemised lines and the
// cash before and after, and the report still fits 80x24.
func TestReportLeadsWithTheFlow(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		endDay(t, m)
		if m.mode != modeReport {
			t.Fatalf("%dx%d: mode %v after the day, want the report", sz[0], sz[1], m.mode)
		}
		r := m.w.Report
		var body []string
		for _, l := range m.reportLines() {
			body = append(body, stripANSI(l))
		}
		at := func(prefix string) int {
			for i, l := range body {
				if strings.HasPrefix(strings.TrimSpace(l), prefix) {
					return i
				}
			}
			return -1
		}
		head, open, sales, closing, cashLine := at("MONEY"), at("Opening"), at("Sales, net of cuts"), at("Closing"), at("Cash ")
		if head < 0 || !(head < open && open < sales && sales < closing && closing < cashLine) {
			t.Fatalf("%dx%d: MONEY %d, Opening %d, Sales %d, Closing %d, Cash %d: want the flow first, in that order\n%s", sz[0], sz[1], head, open, sales, closing, cashLine, strings.Join(body, "\n"))
		}
		want := []string{"Closing", money(r.Flow.Closing.Dirty), money(r.Flow.Closing.Clean), money(r.CashAfter)}
		if got := strings.Fields(body[closing]); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Fatalf("%dx%d: the closing row %q, want %q", sz[0], sz[1], got, want)
		}
		assertFits(t, m.View(), sz[0], sz[1], "the report with its flow")
	}
}

// TestLedgerShowsTheFlow (#351): the ledger's foot is FLOW, the last
// nights by category and their net, and the window runs down to it with
// the cursor on the last row; it fits at every common size.
func TestLedgerShowsTheFlow(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		m.Update(key("7"))
		for range len(m.ledgerRows()) {
			m.Update(key("down"))
		}
		view := stripANSI(m.View())
		if !strings.Contains(view, "FLOW · the last") || !strings.Contains(view, "Net") || !strings.Contains(view, "Sales, net of cuts") {
			t.Fatalf("%dx%d: the ledger's foot has no FLOW:\n%s", sz[0], sz[1], view)
		}
		assertFrame(t, m, "the ledger's FLOW")
	}
}
