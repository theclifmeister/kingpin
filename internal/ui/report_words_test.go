package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// One number for one thing on every screen (#502): the plan's line and
// the ambitions screen read the same percentage for the plan, and the
// quiet streak retiring counts is the same count on the dashboard, the
// reserve dialog and the walk-away.
func TestOneNumberOnEveryScreen(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	off := m.rules.Laundering.Offshore()
	if err := m.sess.PinAmbition(content.AmbitionRetire); err != nil {
		t.Fatal(err)
	}
	w.Offshore, w.QuietDays = off.RetireCash*88/100, off.RetireDays

	// The account 88% and the quiet days met: the plan is 88% on both
	// (the mean, 94%, was the ambitions screen's).
	if line := stripANSI(m.planFact()); !strings.Contains(line, "plan Retire clean 88% · the account 88%") {
		t.Errorf("the plan's line: %q", line)
	}
	if rep := stripANSI(strings.Join(m.planReport(), "\n")); !strings.Contains(rep, "Retire clean 88%: the account 88%") {
		t.Errorf("the PLAN line: %q", rep)
	}
	m.openAmbitions(false)
	found := false
	for _, l := range strings.Split(stripANSI(m.viewAmbitions()), "\n") {
		if strings.Contains(l, "Retire clean") {
			found = true
			if !strings.Contains(l, " 88") || strings.Contains(l, " 94") {
				t.Errorf("the ambitions screen's row: %q", l)
			}
		}
	}
	if !found {
		t.Error("no Retire clean row on the ambitions screen")
	}
	m.Update(key("esc"))

	// Nine quiet days of fourteen: said so on every screen.
	w.QuietDays = 9
	want := "9 of 14 quiet days"
	if off.RetireDays != 14 {
		t.Fatalf("retire_days is %d; the words below are for 14", off.RetireDays)
	}
	if line := stripANSI(m.retireLine()); !strings.Contains(line, want) {
		t.Errorf("the dashboard's retire line: %q", line)
	}
	for _, r := range m.exitRows() {
		if r.cause == content.CauseRetired && !strings.Contains(r.short, want) {
			t.Errorf("the walk-away's Retire row: %q", r.short)
		}
	}
	m.askReserve()
	if view := stripANSI(m.viewReserve()); !strings.Contains(strings.Join(strings.Fields(strings.ReplaceAll(view, "║", "")), " "), want) {
		t.Errorf("the reserve dialog lacks %q:\n%s", want, view)
	}
	if a, ok := m.plan(); !ok || !strings.Contains(planParts(a), "quiet days 9/14") {
		t.Errorf("the plan's parts: %q", planParts(a))
	}
}

// Going straight against a street that sold nothing (#502): a lie-low
// night reads `against $0 last night`, not the $1 the step was bumped to.
func TestLieLowStreetIsZero(t *testing.T) {
	st := engine.AmbitionStepView{Unit: game.UnitIncome, Have: 500, Need: 0, Done: true}
	if got := stepWords(st); got != "$500/day against $0 last night" {
		t.Errorf("a lie-low night's street: %q", got)
	}
	m := richModel(t, 120, 40)
	terms := m.sess.AmbitionTerms()
	terms.Street, terms.LegitIncome = 0, 500
	if terms.LegitDays <= 0 {
		t.Skip("going straight is boxed")
	}
	a, ok := game.AmbitionOf(m.w, content.AmbitionLegit, terms)
	if !ok || a.Steps[0].Need != 0 || !a.Steps[0].Done || a.Steps[0].Frac() != 1 {
		t.Errorf("the income step on a night that sold nothing: %+v", a.Steps[0])
	}
}

// A buy on the book says the unit it comes to and the markup over the
// cash price a unit, and the debt it adds is that total (#502: `$216.27`
// on the connect, `$2,985 for 12` on credit, and `owe $2,961` on the
// dashboard with no word of the ×1.15).
func TestCreditSaysItsUnit(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	for _, sup := range w.SuppliersIn(w.Player.Location) {
		if sup.CreditDays <= 0 || sup.CreditRatio <= 1 || sup.Debt > 0 {
			continue
		}
		for id := range sup.Price {
			if !sup.Sells(id) || !w.Available(sup, id) {
				continue
			}
			qty := 12
			terms := m.creditTerms(sup, id, qty)
			p, err := w.Buy(sup.ID, id, qty, true, 0)
			if err != nil {
				continue
			}
			want := money(p.Cost) + ": " + price(p.UnitPrice) + " a unit, " + format.Times(sup.CreditRatio, 2) + " the cash " + price(p.UnitPrice/sup.CreditRatio)
			if terms != want {
				t.Errorf("the credit terms read %q, want %q", terms, want)
			}
			if line := stripANSI(m.debtLine()); !strings.Contains(line, "owe "+cash(p.Cost)) {
				t.Errorf("the debt line %q is not the %s the book took", line, cash(p.Cost))
			}
			return
		}
	}
	t.Skip("no connect here gives credit on the fixture")
}

// A route whose name has its article is not given a second (#502: "Buy
// the customs agent on the The Channel").
func TestTheChannelHasOneThe(t *testing.T) {
	if got := format.The("The Channel"); got != "The Channel" {
		t.Errorf("The Channel reads %q", got)
	}
	if got := format.The("Interstate"); got != "the Interstate" {
		t.Errorf("the Interstate reads %q", got)
	}
}

// The summary keeps a lieutenant's cut apart from the skim (#502: a
// run's $31K skimmed read as the lieutenants' cuts counted in it).
func TestSummaryKeepsTheCutApart(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Stats.Cuts, m.w.Stats.Skimmed = 31_000, 4_000
	m.w.Over = m.w.End(content.CauseIndicted, m.w.Day, "")
	m.mode = modeOver
	lines := stripANSI(strings.Join(m.summaryLines(), "\n"))
	if !strings.Contains(lines, "$4,000 skimmed") || !strings.Contains(lines, "$31K kept by the crew who ran it for you, apart from the skim") {
		t.Errorf("the summary's money:\n%s", lines)
	}
}
