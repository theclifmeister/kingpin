package ui

import (
	"strings"
	"testing"
)

// TestUpkeepIsSaid (#458, a playtest): a front's upkeep is clean cash,
// and nothing said so. The buy picker names it and warns when the clean
// pile will not cover the first night; the morning a front shuts for it,
// TODAY, the ALERTS and the ledger's status say why and by how much; a
// blank reserve keeps tonight's upkeep back, the reserve, cash-out and
// fund dialogs warn when a move leaves less.
func TestUpkeepIsSaid(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.startRun(testSeed(t))
	w := m.w
	l := m.rules.Laundering
	laundromat := m.cfg.Laundering.Front("laundromat")
	if laundromat == nil {
		t.Fatal("no laundromat in the file")
	}
	upkeep, days := laundromat.Upkeep, l.Tuning().UpkeepFreezeDays
	// Nothing over the till once it is paid for, and no clean cash.
	w.Player.DirtyCash = laundromat.Cost + m.till()
	w.Player.CleanCash = 0
	w.Stats.PeakCash = laundromat.UnlockCash
	m.Update(key("7"))
	m.Update(key("b"))
	m.Update(key("enter")) // the kind: a front
	if m.mode != modeFront || m.frontRows()[m.front.cursor].ID != "laundromat" {
		t.Fatalf("the picker: mode %v on %+v", m.mode, m.frontRows())
	}
	view := stripANSI(m.View())
	for _, want := range []string{"upkeep is paid in clean cash", "Tonight's " + money(upkeep) + " of upkeep is expected to come up " + money(upkeep) + " clean short", "the dirty over the " + money(m.till()) + " till", "y buys it anyway"} {
		if !strings.Contains(squash(view), want) {
			t.Errorf("the buy picker lacks %q:\n%s", want, view)
		}
	}
	assertFits(t, m.View(), 120, 40, "buy picker")
	// Bought into a shut (#496): enter asks, any other key goes back,
	// and only y buys it.
	m.Update(key("enter"))
	if m.mode != modeConfirm || len(w.Fronts) != 0 {
		t.Fatalf("enter over a shut: mode %v, %d fronts; want the confirmation", m.mode, len(w.Fronts))
	}
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "BUY LAUNDROMAT?") || !strings.Contains(view, "clean short") {
		t.Errorf("the confirmation does not say the shut:\n%s", view)
	}
	assertFits(t, m.View(), 120, 40, "buy confirmation")
	m.Update(key("n"))
	if m.mode == modeConfirm || len(w.Fronts) != 0 {
		t.Fatalf("n bought it: mode %v, %d fronts", m.mode, len(w.Fronts))
	}
	m.Update(key("b"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("y"))
	if len(w.Fronts) != 1 {
		t.Fatalf("bought %d fronts: %q", len(w.Fronts), m.status)
	}
	endDay(t, m)
	f := w.Fronts[0]
	if f.Unpaid != upkeep || !f.Frozen(w.Day+1) {
		t.Fatalf("the morning after: %+v, want shut %d short", f, upkeep)
	}
	var lead []string
	for _, ln := range w.Report.Lead {
		lead = append(lead, ln.Text)
	}
	if want := "Laundromat shut " + plural(days, "day") + ": its upkeep is paid in clean cash, and the clean pile was " + money(upkeep) + " short."; !strings.Contains(strings.Join(lead, "\n"), want) {
		t.Errorf("TODAY %q lacks %q", lead, want)
	}
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "upkeep is paid in clean cash") {
		t.Errorf("the report does not lead with the shut front:\n%s", view)
	}
	m.Update(key("enter"))
	alerts := squash(stripANSI(strings.Join(m.alertLines(120, 12), "\n")))
	if want := "Laundromat shut " + plural(days, "day") + ": upkeep unpaid, " + money(upkeep) + " clean short"; !strings.Contains(alerts, want) {
		t.Errorf("the alerts lack %q:\n%s", want, alerts)
	}
	status, _ := cellText(kText, 0, m.frontStatus(f))
	if want := "shut 7d: upkeep unpaid (" + money(upkeep) + " clean)"; days != 7 || status != want {
		t.Errorf("the ledger's status %q, want %q", status, want)
	}

	// A front the night's wash pays for is bought on enter, and nothing
	// warns (#496: a playtest was warned of a shut that never came).
	{
		m2 := newTestModel(t, 120, 40)
		m2.startRun(testSeed(t))
		m2.w.Player.DirtyCash = laundromat.Cost + m2.till() + 10*upkeep
		m2.w.Player.CleanCash = 0
		m2.w.Stats.PeakCash = laundromat.UnlockCash
		m2.Update(key("7"))
		m2.Update(key("b"))
		m2.Update(key("enter"))
		if view := squash(stripANSI(m2.View())); strings.Contains(view, "clean short") {
			t.Errorf("a front the wash pays for is warned of:\n%s", view)
		}
		m2.Update(key("enter"))
		if len(m2.w.Fronts) != 1 || m2.mode == modeConfirm {
			t.Fatalf("enter on a front that pays: %d fronts, mode %v", len(m2.w.Fronts), m2.mode)
		}
	}

	// Open again: a blank reserve keeps tonight's upkeep back.
	w.Fronts[0].FrozenUntil, w.Fronts[0].Unpaid = 0, 0
	w.Player.CleanCash = 10_000
	m.Update(key("7"))
	m.Update(key("o"))
	if m.mode != modeReserve {
		t.Fatalf("o on the ledger: mode %v status %q", m.mode, m.status)
	}
	if got, err := m.reserveAmount(); err != nil || got != 10_000-upkeep {
		t.Fatalf("blank reserves %d (%v), want %d: the clean less tonight's upkeep", got, err, 10_000-upkeep)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "blank keeps it back") {
		t.Errorf("the reserve dialog does not say blank keeps the upkeep:\n%s", view)
	}
	m.amt.SetValue("10000")
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "Leaves $0 clean for "+money(upkeep)+" of upkeep tonight") {
		t.Errorf("the reserve dialog does not warn on moving it all:\n%s", view)
	}
	assertFits(t, m.View(), 120, 40, "reserve dialog")
	m.Update(key("esc"))
	// With no more than the upkeep, blank moves nothing and says why.
	w.Player.CleanCash = upkeep
	m.Update(key("o"))
	m.Update(key("enter"))
	if m.mode != modeReserve || w.ReservedToday() != 0 || !strings.Contains(m.amt.err, "keeps "+money(upkeep)+" clean back") {
		t.Fatalf("blank over the upkeep: mode %v reserved %d err %q", m.mode, w.ReservedToday(), m.amt.err)
	}
	m.Update(key("esc"))

	// The cash-out and the fund dialogs warn the same way.
	w.Player.CleanCash = 10_000
	m.Update(key("c"))
	m.amt.SetValue("10000")
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "of upkeep tonight") {
		t.Errorf("the cash-out dialog does not warn on drawing it all:\n%s", view)
	}
	m.Update(key("esc"))
	m.Update(key("f"))
	if m.mode != modeFund {
		t.Fatalf("f on the ledger: mode %v status %q", m.mode, m.status)
	}
	if got, err := m.fundAmount(m.fundCity()); err != nil || got > 10_000-upkeep {
		t.Fatalf("blank funds %d (%v): more than the clean less the upkeep", got, err)
	}
	m.fnd.amt.SetValue("10000")
	if view := squash(stripANSI(m.View())); !strings.Contains(view, "of upkeep tonight") {
		t.Errorf("the fund dialog does not warn on giving it all:\n%s", view)
	}
}

// squash is a view's words on one line, the wrap and the modal's
// borders gone, so a sentence reads whole wherever it broke.
func squash(s string) string {
	var words []string
	for _, f := range strings.Fields(s) {
		if f != "│" && f != "┃" && f != "║" {
			words = append(words, f)
		}
	}
	return strings.Join(words, " ")
}
