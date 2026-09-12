package ui

import (
	"strings"
	"testing"
)

// The reserve dialog (#195): o on the ledger opens it, off the ledger it
// points at the ledger; it refuses with no clean cash; blank sends a
// lot; the pane says what the DA reads, red past the lot; the cash
// line reads dirty · clean · offshore; the modal fits 80x24 and esc
// leaves the world alone; the alert and the street's last line say how
// far off retiring is once the account has something in it.
func TestReserveDialog(t *testing.T) {
	m := richModel(t, 80, 24)
	w := m.w
	l := m.set.Laundering
	off := l.Offshore()
	m.Update(key("o"))
	if m.mode != modePlay || !strings.Contains(m.status, "ledger") {
		t.Fatalf("o on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("7"))
	w.Player.CleanCash = 0
	m.Update(key("o"))
	if m.mode != modePlay || !strings.Contains(m.status, "clean cash") {
		t.Fatalf("o with no clean cash: mode %v status %q", m.mode, m.status)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "offshore $0") {
		t.Fatalf("the cash line lacks the account:\n%s", view)
	}
	w.Player.CleanCash = off.Lot * 3
	m.Update(key("o"))
	if m.mode != modeReserve {
		t.Fatalf("o on the ledger: mode %v", m.mode)
	}
	assertFits(t, m.View(), 80, 24, "reserve dialog")
	if view := stripANSI(m.View()); !strings.Contains(view, "under the lot") {
		t.Fatalf("a blank amount is not read as a lot:\n%s", view)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || w.Today.Reserved != 0 || w.Player.CleanCash != off.Lot*3 {
		t.Fatal("esc reserved")
	}
	// Blank: a lot, under the line.
	m.Update(key("o"))
	m.Update(key("enter"))
	if m.mode != modePlay || w.Today.Reserved != off.Lot || w.Player.CleanCash != off.Lot*2 || !strings.Contains(m.status, "nobody reads it") {
		t.Fatalf("enter on blank: mode %v reserved %d clean %d status %q", m.mode, w.Today.Reserved, w.Player.CleanCash, m.status)
	}
	// Twice the lot more: over the line, the pane says so in the
	// dialog and the status alarms.
	m.Update(key("o"))
	m.Update(key("m"))
	if view := stripANSI(m.View()); !strings.Contains(view, "over the lot") {
		t.Fatalf("the max is not read as over the lot:\n%s", view)
	}
	m.Update(key("enter"))
	if m.mode != modePlay || w.Today.Reserved != off.Lot*3 || w.Player.CleanCash != 0 || !strings.Contains(m.status, "DA will read it") {
		t.Fatalf("enter on the max: mode %v reserved %d clean %d status %q", m.mode, w.Today.Reserved, w.Player.CleanCash, m.status)
	}
	if w.NetWorth() < off.Lot*3 {
		t.Fatalf("net worth %d does not count the money in transit", w.NetWorth())
	}
	// The night moves it: the account, the fee, the pages in the
	// morning's file, and the alert.
	w.SetLieLow(true) // nothing sold: the pages that come are the move's
	before := w.Heat.Evidence
	endDay(t, m)
	m.Update(key("enter"))
	if w.Offshore != off.Lot*3-l.Fee(off.Lot*3) || w.Heat.Evidence != before {
		t.Fatalf("the morning after: offshore %d evidence %d (was %d); the pages come the morning after the move, not the same night", w.Offshore, w.Heat.Evidence, before)
	}
	m.Update(key("1"))
	if line := stripANSI(m.retireLine()); !strings.Contains(line, "retire in") || !strings.Contains(line, "short") {
		t.Fatalf("the alert does not say how far off retiring is: %q", line)
	}
	if alerts := stripANSI(strings.Join(m.alertLines(60, 9), "\n")); !strings.Contains(alerts, "retire in") {
		t.Fatalf("the alerts lack the retirement line:\n%s", alerts)
	}
	w.SetLieLow(true)
	endDay(t, m)
	m.Update(key("enter"))
	if want := before + l.Lots(off.Lot*3)*m.set.Heat.StructureEvidence(); w.Heat.Evidence != want {
		t.Fatalf("two mornings after: evidence %d, want %d", w.Heat.Evidence, want)
	}
	// Retirement within reach: the alert and the summary row.
	w.Offshore = off.RetireCash
	w.QuietDays = off.RetireDays
	if line := stripANSI(m.retireLine()); !strings.Contains(line, "You could retire") {
		t.Fatalf("the alert does not say retiring is open: %q", line)
	}
	if err := l.Retire(w); err != nil || w.Over == nil || w.Over.Cause != "retired" {
		t.Fatalf("retire: %v %+v", err, w.Over)
	}
	m.mode = modeOver
	if view := stripANSI(m.View()); !strings.Contains(view, "offshore") {
		t.Fatalf("the summary lacks the account:\n%s", view)
	}
}
