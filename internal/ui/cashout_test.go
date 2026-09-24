package ui

import (
	"strings"
	"testing"
)

// TestCashOutDialog (#395): c on the ledger opens it and refuses with
// no clean cash; blank draws what tonight's wages are short, the fee on
// top; esc draws nothing; enter lands the cash at once and the
// morning's MONEY books it; the cash flow reconciles; the wages alert
// points at the ledger while there is clean cash to draw.
func TestCashOutDialog(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	l := m.rules.Laundering
	m.Update(key("7"))
	w.Player.CleanCash = 0
	m.Update(key("c"))
	if m.mode != modePlay || !strings.Contains(m.status, "no clean cash") {
		t.Fatalf("c with no clean cash: mode %v status %q", m.mode, m.status)
	}
	wages := m.rules.Crew.Wages(w, w.Crew.Pay)
	if wages <= 0 {
		t.Fatal("the rich fixture pays no wages")
	}
	w.Player.DirtyCash, w.Player.CleanCash = wages/2, 1_000_000
	short := wages - wages/2
	m.Update(key("1"))
	if alerts := stripANSI(strings.Join(m.alertLines(80, 12), "\n")); !strings.Contains(alerts, "cash out clean on the ledger") {
		t.Fatalf("the wages alert does not point at the cash-out:\n%s", alerts)
	}
	m.Update(key("7"))
	m.Update(key("c"))
	if m.mode != modeCashOut {
		t.Fatalf("c on the ledger: mode %v", m.mode)
	}
	assertFits(t, m.View(), 80, 24, "cash-out dialog")
	if view := stripANSI(m.View()); !strings.Contains(view, "short tonight") {
		t.Fatalf("the dialog does not say the wages are short:\n%s", view)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || w.Player.CleanCash != 1_000_000 || w.Today.CashedOut.Amount != 0 {
		t.Fatal("esc cashed out")
	}
	m.Update(key("c"))
	m.Update(key("enter"))
	got := w.Today.CashedOut
	if m.mode != modePlay || got.Amount == 0 || got.Amount-got.Fee < short || got.Fee != l.CashOutFee(got.Amount) {
		t.Fatalf("enter on blank: mode %v drew %+v for %d short, status %q", m.mode, got, short, m.status)
	}
	if w.Player.CleanCash != 1_000_000-got.Amount || w.Player.DirtyCash != wages/2+got.Amount-got.Fee {
		t.Fatalf("the piles: clean %d dirty %d after %+v", w.Player.CleanCash, w.Player.DirtyCash, got)
	}
	if !strings.Contains(m.status, "Cashed out") {
		t.Fatalf("status %q", m.status)
	}
	m.Update(key("1"))
	if alerts := stripANSI(strings.Join(m.alertLines(80, 12), "\n")); strings.Contains(alerts, "Wages") {
		t.Fatalf("the wages are covered and the alert still warns:\n%s", alerts)
	}
	endDay(t, m)
	m.Update(key("enter"))
	if money := strings.Join(w.Report.Money, "\n"); !strings.Contains(money, "Cashed out") {
		t.Fatalf("the morning's money lacks the cash-out:\n%s", money)
	}
	if !w.Report.Flow.Reconciles() {
		t.Fatalf("the flow does not reconcile: %+v", w.Report.Flow)
	}
}
