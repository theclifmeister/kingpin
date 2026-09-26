package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// flat is a view's words on one line: the modal's frame and the wrap
// gone.
func flat(s string) string {
	return collapse(strings.NewReplacer("║", " ", "│", " ").Replace(stripANSI(s)))
}

// brokeTonight leaves the run with nothing in stock or on the road and
// tonight's wages taking the till under the cheapest unit.
func brokeTonight(t *testing.T, m *Model) {
	t.Helper()
	w := m.w
	for _, cid := range w.CityOrder {
		for _, id := range w.Products {
			w.SetStock(cid, id, 0)
		}
	}
	w.Shipments, w.Supply, w.Standing = nil, nil, nil
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Brick", Role: game.RoleEnforcer, Skill: 40, Loyalty: 80, Nerve: 50, Wage: 200})
	w.Crew.NextID = 901
	w.Crew.Pay = events.PayGenerous
	w.Player.DirtyCash, w.Player.CleanCash = m.rules.Crew.Wages(w, w.Crew.Pay)+3, 0
}

// TestBrokeTonightIsSaid (#518): the night the wages empty the till
// with nothing to sell, the dashboard's first alert says so in red with
// the figures, the END THE DAY? modal lists it first, and F refuses to
// run, in red, naming it.
func TestBrokeTonightIsSaid(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.startRun(5)
	brokeTonight(t, m)
	as := m.alerts()
	if len(as) == 0 || as[0].kind != engine.AlertBroke {
		t.Fatalf("the first alert: %+v", as)
	}
	wages := m.rules.Crew.Wages(m.w, m.w.Crew.Pay)
	for _, want := range []string{"The run ends broke tonight", money(wages+3) + " in hand", money(wages) + " in wages", "$3 left", "nothing in stock or on the road"} {
		if !strings.Contains(stripANSI(as[0].text), want) {
			t.Errorf("the alert %q lacks %q", stripANSI(as[0].text), want)
		}
	}
	if got := flat(strings.Join(m.previewLines(), "\n")); !strings.Contains(got, "The run ends broke tonight: "+money(wages+3)+" in hand") {
		t.Errorf("the end-day confirmation does not say it:\n%s", got)
	}
	day := m.w.Day
	m.Update(key("F"))
	if m.mode == modeConfirmFast || m.w.Day != day || m.statusKind != statusBad || !strings.Contains(m.status, "the run ends broke tonight") {
		t.Fatalf("F on the broke night: mode %v, day %d, status %q", m.mode, m.w.Day, m.status)
	}
}

// TestBrokeEndingWordsAgreeWithTheRule (#518): the broke rule reads the
// till and the stash, not the connects' credit (crew.Sim.broke), so the
// ending must not say nobody would front you; it says what the rule
// read. And the summary lists the debt still owed.
func TestBrokeEndingWordsAgreeWithTheRule(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.startRun(5)
	brokeTonight(t, m)
	sup := &m.w.Suppliers[0]
	sup.Debt, sup.DebtDue = 1_517, m.w.Day+5
	m.Update(key("n"))
	if m.w.Over == nil || m.w.Over.Cause != content.CauseBroke {
		t.Fatalf("the night ended %+v", m.w.Over)
	}
	ep := m.epilogue()
	if strings.Contains(ep, "Nobody") || strings.Contains(ep, "will front you") {
		t.Errorf("the broke ending says nobody fronts you, which the rule never read: %q", ep)
	}
	if !strings.Contains(ep, "nothing in stock or on the road") {
		t.Errorf("the broke ending does not say what the rule read: %q", ep)
	}
	got := flat(strings.Join(m.summaryLines(), "\n"))
	if want := "owed " + cash(1_517) + " to " + sup.Name; !strings.Contains(got, want) {
		t.Errorf("the summary does not list the debt (%q):\n%s", want, got)
	}
}

// TestCreditDueAgreesWithTheBook (#518): on a clean book the pane says
// the connect's days to pay; on one that owes, credit taken is due with
// the debt, and the pane says that day, which is the day a credit buy
// books.
func TestCreditDueAgreesWithTheBook(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.startRun(5)
	var sup *game.Supplier
	for i := range m.w.Suppliers {
		if s := &m.w.Suppliers[i]; s.CreditDays > 0 && s.City == m.w.Player.Location {
			sup = s
			break
		}
	}
	if sup == nil {
		t.Skip("no connect with credit where you stand")
	}
	if got, want := creditDue(sup), fmt.Sprintf("%dd to pay", sup.CreditDays); got != want {
		t.Fatalf("a clean book: %q, want %q", got, want)
	}
	sup.Debt, sup.DebtDue = 100, m.w.Day+1
	if got, want := creditDue(sup), fmt.Sprintf("due d%d with the debt", m.w.Day+1); got != want {
		t.Fatalf("an open book: %q, want %q", got, want)
	}
	id := ""
	for _, p := range m.w.Products {
		if sup.Sells(p) {
			id = p
			break
		}
	}
	if _, err := m.w.Buy(sup.ID, id, 1, true, 0); err != nil {
		t.Skipf("the credit buy: %v", err)
	}
	if sup.DebtDue != m.w.Day+1 {
		t.Fatalf("credit on an open book is due day %d, the pane said %d", sup.DebtDue, m.w.Day+1)
	}
}

// TestWarrantOnTheHeatPanelAndF (#519): a warrant out is marked on the
// dashboard's HEAT panel and F refuses to run the night, in red.
func TestWarrantOnTheHeatPanelAndF(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.startRun(5)
	m.Update(key("n"))
	closeMorning(t, m)
	m.w.Heat.WarrantDay = m.w.Day
	if got := stripANSI(strings.Join(m.heatLines(40, false), "\n")); !strings.Contains(got, "WARRANT: served tonight on a sale") {
		t.Fatalf("the HEAT panel does not mark the warrant:\n%s", got)
	}
	day := m.w.Day
	m.Update(key("F"))
	if m.mode == modeConfirmFast || m.w.Day != day || m.statusKind != statusBad || !strings.Contains(m.status, "a warrant for your arrest") {
		t.Fatalf("F with a warrant out: mode %v, day %d, status %q", m.mode, m.w.Day, m.status)
	}
}

// TestNewStopsHaveWords (#518, #519, #520): a missed payroll, a quiet
// streak lost and a war over with its reason each read as a stop.
func TestNewStopsHaveWords(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.startRun(5)
	for _, c := range []struct {
		e    events.Event
		want string
	}{
		{events.CrewPaid{Short: 442}, "payroll missed, $442 short"},
		{events.QuietBroken{Day: 57, Days: 9, Cause: events.QuietHeat, City: m.w.Home().ID}, "the quiet streak (9 days) was reset by heat at the retire line in " + m.w.Home().Name + " on day 57"},
		{events.WarEnded{Rival: "Lena", Why: "you hold no corner left to fight from: the war is lost", Lost: true}, "the war on Lena's crew is over: you hold no corner left to fight from"},
	} {
		if got := m.stopEvent(c.e); !strings.Contains(got, c.want) {
			t.Errorf("%T reads %q, want %q", c.e, got, c.want)
		}
	}
}

// TestTakenOutWordsAreTheRule (#520): the declaration, WORDS and the
// README say which corner loss ends the run in the one phrase,
// takenOutWords, the rule rivals.Sim's endings reads (the last corner
// you hold, taken that night by a faction whose war with you is open:
// TestADeclaredWarIsOpen pins that a war under the line, undeclared,
// ends nothing when the last corner goes).
func TestTakenOutWordsAreTheRule(t *testing.T) {
	m := richModel(t, 120, 40)
	phrase := strings.TrimSuffix(takenOutWords, ",")
	m.w.Crew.Members = nil // under taken_out_muscle: the declaration warns (TestWarWarnsShortOfMuscle)
	if !strings.Contains(m.warMuscleLine(), phrase) {
		t.Errorf("the declaration's warning: %q", m.warMuscleLine())
	}
	if got := flat(strings.Join(m.helpLines(), "\n")); !strings.Contains(got, "taken out "+phrase) {
		t.Errorf("WORDS has no taken out entry saying %q", phrase)
	}
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(collapse(string(raw)), phrase) {
		t.Errorf("the README's endings do not say %q", phrase)
	}
}

// TestFireSaysTheWarNeedsMuscle (#520): firing an enforcer mid-war that
// takes the payroll under taken_out_muscle says so on the confirmation;
// with no war it does not.
func TestFireSaysTheWarNeedsMuscle(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.startRun(5)
	w := m.w
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Brick", Role: game.RoleEnforcer, Skill: 40, Loyalty: 80, Nerve: 50, Wage: 50})
	w.Crew.NextID = 901
	m.subjectID = 900
	if got := flat(m.fireConfirm()); strings.Contains(got, "taken out") {
		t.Fatalf("no war, and the fire warns: %s", got)
	}
	r := w.Rival()
	r.Arrived, r.War = 1, m.cfg.Rivals.Rivals.WarThreshold
	if got := flat(m.fireConfirm()); !strings.Contains(got, "At war with "+m.rivalName(r)) || !strings.Contains(got, "the run ends taken out") {
		t.Fatalf("the fire mid-war does not warn: %s", got)
	}
}

// TestAssignSaysWhenATurnEndsTheRun (#520): the assign dialog and WORDS
// say when a turned lieutenant ends the run: the share of your corners,
// the minimum, and that it is read the night they turn.
func TestAssignSaysWhenATurnEndsTheRun(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.startRun(5)
	lt := withLieutenant(t, m)
	terms := m.rules.Crew.Lieutenancy()
	m.subjectID = lt.ID
	got := flat(m.viewAssign())
	for _, want := range []string{"50% of your corners", "3 corners at least", "ends the run betrayed", "checked only the night they turn"} {
		if !strings.Contains(got, want) {
			t.Errorf("the assign dialog lacks %q:\n%s", want, got)
		}
	}
	if terms.BetrayShare != 0.5 || terms.BetrayCorners != 3 {
		t.Fatalf("the words above are written for the file's 50%% and 3: %+v", terms)
	}
	if help := flat(strings.Join(m.helpLines(), "\n")); !strings.Contains(help, "betrayed a lieutenant turns on 50% of your corners, 3+: that night") {
		t.Errorf("WORDS has no betrayed entry:\n%s", help)
	}
}

// TestUnderTheLineAgreesWithTheRoster (#520): a lieutenant at 29.6
// loyalty is under the 30 line to the crew sim; the roster shows 29, and
// the stop names 29, so the two agree; at exactly 30 there is no alert.
func TestUnderTheLineAgreesWithTheRoster(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.startRun(5)
	lt := withLieutenant(t, m)
	flip := m.rules.Crew.FlipLine()
	lt.Loyalty = flip
	for _, a := range m.alertsOf(engine.AlertCrewLine) {
		if strings.Contains(a.why, "under the") {
			t.Fatalf("a lieutenant at exactly %.0f is under the line: %q", flip, a.why)
		}
	}
	lt.Loyalty = flip - 0.4
	lines := m.alertsOf(engine.AlertCrewLine)
	if len(lines) != 1 || lines[0].why != "Wally at 29 loyalty, under the 30 line" {
		t.Fatalf("the stop at %.1f: %+v", lt.Loyalty, lines)
	}
	for i, c := range m.w.Crew.Members {
		if c.ID == lt.ID {
			m.crewCursor = i
		}
	}
	var pane []string
	for _, s := range m.crewDetails() {
		pane = append(pane, s.lines...)
	}
	if got := flat(strings.Join(pane, "\n")); !strings.Contains(got, "loyalty") || !strings.Contains(got, " 29 turns under 30") {
		t.Fatalf("the roster at %.1f:\n%s", lt.Loyalty, got)
	}
}
