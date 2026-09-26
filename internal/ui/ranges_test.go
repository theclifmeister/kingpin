package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestNumberFieldMaxNeverUnderItsMin (#526): a field's max is never
// under its min; its range reads `$50,000 … $X`, and m and the arrows
// stay in it.
func TestNumberFieldMaxNeverUnderItsMin(t *testing.T) {
	f := newNumberField("blank = the float")
	f.money, f.min, f.max = true, 50_000, 23_689
	if v := stripANSI(f.View()); !strings.Contains(v, "$50,000 … $50,000") || strings.Contains(v, "max") {
		t.Errorf("the field reads %q, want its range from the min", v)
	}
	f.Update(key("m"))
	if n, _ := f.Number(); n != 50_000 {
		t.Errorf("m set %d, want the min it tops out at", n)
	}
	f.SetValue("")
	f.Update(key("down"))
	if n, _ := f.Number(); n != 50_000 {
		t.Errorf("down from blank set %d, under the min", n)
	}
	f.SetValue("10000")
	if f.Outside() >= 0 {
		t.Error("10000 under a 50,000 min does not read as under")
	}
	f.SetValue("60000")
	if f.Outside() <= 0 {
		t.Error("60000 over a 50,000 top does not read as over")
	}
}

// TestTillRangeWithCashUnderTheFloat (#526): with the dirty in hand
// under the float the till's field reads the float to the rot line, not
// a max under the float; it opens blank whatever the till, and blank is
// the float; under the float is the float, said; over the rot line is
// set to it and said, and the next enter sets it. The wash details read
// the till as set.
func TestTillRangeWithCashUnderTheFloat(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	w.Player.DirtyCash = 23_689
	w.Fronts = append(w.Fronts, game.Front{ID: "laundromat", Name: "Laundromat"})
	float, top := m.floatLine(), m.tillMax()
	if top < float {
		t.Fatalf("the till's top %d is under the float %d", top, float)
	}
	m.Update(key("7"))
	m.Update(key("T"))
	view := squash(stripANSI(m.View()))
	if want := money(float) + " … " + money(top); !strings.Contains(view, want) {
		t.Fatalf("the till's field lacks its range %q:\n%s", want, view)
	}
	if strings.Contains(view, money(w.Player.DirtyCash)+" max") {
		t.Fatalf("the till still reads the cash in hand as its max:\n%s", view)
	}
	for _, r := range "10000" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modePlay || w.Laundering.Till != 0 || !strings.Contains(m.status, "never under") {
		t.Fatalf("10,000 under the float: mode %v till %d status %q err %q", m.mode, w.Laundering.Till, m.status, m.amt.err)
	}
	m.Update(key("T"))
	for _, r := range "999999999" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modeTill || w.Laundering.Till != 0 || !strings.Contains(m.amt.err, "tops out") {
		t.Fatalf("over the top: mode %v till %d err %q", m.mode, w.Laundering.Till, m.amt.err)
	}
	m.Update(key("enter"))
	if w.Laundering.Till != top {
		t.Fatalf("the second enter set %d, want the top %d", w.Laundering.Till, top)
	}
	if err := m.sess.SetTill(top + 1); err == nil {
		t.Error("the session took a till over the rot line")
	}
	// A till set: the dialog opens blank, and blank is the float.
	if err := m.sess.SetTill(150_000); err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(sectionText(m.washSection()), " "); !strings.Contains(squash(text), cash(150_000)) {
		t.Errorf("the wash details do not read the till as set:\n%s", text)
	}
	m.Update(key("T"))
	if m.amt.Value() != "" {
		t.Fatalf("the till opened on %q, not blank", m.amt.Value())
	}
	if view := squash(stripANSI(m.View())); !strings.Contains(view, money(150_000)) {
		t.Errorf("the till row does not read the till as set:\n%s", view)
	}
	m.Update(key("enter"))
	if w.Laundering.Till != 0 {
		t.Fatalf("blank left the till at %d, not the float", w.Laundering.Till)
	}
}

// TestSweepRangeWithCashUnderTheLine (#526): the sweep's line tops out
// at the clean in hand; a line typed over it is set to it and said, and
// the next enter sets it.
func TestSweepRangeWithCashUnderTheLine(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	w.Player.CleanCash = 2_100
	m.Update(key("7"))
	m.Update(key("S"))
	if m.mode != modeSweep {
		t.Fatalf("S on the ledger: mode %v, status %q", m.mode, m.status)
	}
	for _, r := range "5000" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modeSweep || w.Laundering.Sweep.On || !strings.Contains(m.amt.err, "at most") {
		t.Fatalf("5,000 over $2,100 clean: mode %v sweep %+v err %q", m.mode, w.Laundering.Sweep, m.amt.err)
	}
	m.Update(key("enter"))
	if !w.Laundering.Sweep.On || w.Laundering.Sweep.Keep != 2_100 {
		t.Fatalf("the second enter set %+v, want a line of $2,100", w.Laundering.Sweep)
	}
}

// TestFundSpendsOnlyOnGive (#526): with a campaign open the fund's first
// page's enter reads `next`, the campaign page shows the clean in hand
// as it stands and the first page's goodwill only as `if you give`, esc
// gives nothing and says so, and the give says what was given.
func TestFundSpendsOnlyOnGive(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	w.Player.CleanCash = 212_000
	w.Law.CampaignOpen = true
	m.Update(key("7"))
	m.Update(key("f"))
	view := stripANSI(m.View())
	if !strings.Contains(view, "enter next") || strings.Contains(view, "enter give") {
		t.Fatalf("the first page's enter is not `next`:\n%s", view)
	}
	for _, r := range "100000" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.modalStep() != 1 {
		t.Fatalf("enter on the first page: step %d", m.modalStep())
	}
	view = squash(stripANSI(m.View()))
	if !strings.Contains(view, cash(212_000)+" clean") {
		t.Errorf("the campaign page does not show the clean as it stands:\n%s", view)
	}
	if !strings.Contains(view, "if you give") || !strings.Contains(view, money(112_000)+" clean left") {
		t.Errorf("the campaign page does not show the goodwill as `if you give`:\n%s", view)
	}
	m.Update(key("esc"))
	if w.Player.CleanCash != 212_000 || w.FundedToday(w.Player.Location) != 0 || !strings.Contains(m.status, "Nothing given") {
		t.Fatalf("esc: clean %d funded %d status %q", w.Player.CleanCash, w.FundedToday(w.Player.Location), m.status)
	}
	m.Update(key("f"))
	for _, r := range "100000" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	m.Update(key("enter"))
	if w.Player.CleanCash != 112_000 || !strings.Contains(m.status, "Gave") || !strings.Contains(m.status, money(100_000)) {
		t.Fatalf("give: clean %d status %q", w.Player.CleanCash, m.status)
	}
}
