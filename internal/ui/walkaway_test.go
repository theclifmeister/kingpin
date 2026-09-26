package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// The walk away and the endings as a player meets them (#494, #498,
// docs/endings.md): each confirmation in its own words with its y
// listed, the terms the plans' own, what this morning would leave
// unsettled, a ready ending pointed at the walk away, the ending's
// summary left on a deliberate key, and the score spelled out.

// openEveryWayOut puts every way out in hand on the rich fixture.
func openEveryWayOut(m *Model) {
	w := m.w
	off := m.rules.Laundering.Offshore()
	w.Offshore, w.QuietDays = off.RetireCash, off.RetireDays
	w.Upgrades["identity"] = true
	w.Reign, w.ReignSlip = w.Day, 0
	w.LegitDays = m.cfg.Laundering.Businessman.LegitDays
}

// TestEachEndingConfirmsInItsOwnWords: the four confirmations each say
// their own ending (going straight read as the vanish's) and each
// footer lists y with the ending's verb.
func TestEachEndingConfirmsInItsOwnWords(t *testing.T) {
	m := richModel(t, 100, 30)
	openEveryWayOut(m)
	m.Update(key("1"))
	want := map[string][2]string{
		"Retire":         {"Retire on", "y retire"},
		"Vanish":         {"Vanish on the new identity", "y vanish"},
		"Take the crown": {"Take the crown on day", "y crown"},
		"Go straight":    {"Go straight on", "y go straight"},
	}
	seen := map[string]string{}
	for i, r := range m.exitRows() {
		if !r.open {
			t.Fatalf("%s is not open: %s", r.name, r.short)
		}
		m.Update(key("w"))
		m.Update(key(string(rune('1' + i))))
		if m.mode != modeExit || m.exit.step != 1 {
			t.Fatalf("%s: mode %v step %d err %q", r.name, m.mode, m.exit.step, m.exit.err)
		}
		v := stripANSI(m.View())
		if !strings.Contains(v, want[r.name][0]) {
			t.Errorf("%s: the confirmation lacks %q:\n%s", r.name, want[r.name][0], v)
		}
		if f := footerKeys(m); !strings.Contains(f, want[r.name][1]) {
			t.Errorf("%s: the footer %q lacks %q", r.name, f, want[r.name][1])
		}
		for other, copy := range want {
			if other != r.name && strings.Contains(v, copy[0]) {
				t.Errorf("%s confirms in %s's words:\n%s", r.name, other, v)
			}
		}
		seen[r.name] = v
		m.Update(key("esc"))
	}
	if len(seen) != len(want) {
		t.Fatalf("confirmed %d ways out, want %d", len(seen), len(want))
	}
}

// TestWalkAwayTermsAreThePlans: the closed rows name the conditions
// the ambitions panel lists: going straight the goodwill as well as
// the fronts, the crown the share, the crews and the days it holds,
// vanishing the lawyer before the papers.
func TestWalkAwayTermsAreThePlans(t *testing.T) {
	m := richModel(t, 120, 40)
	terms := map[string]string{}
	for _, r := range m.exitRows() {
		terms[r.cause] = r.terms
	}
	for cause, words := range map[string][]string{
		content.CauseBusinessman: {"fronts out-earn the street", "goodwill", "pressure", "nights running"},
		content.CauseKingpin:     {"of home's corners", "every crew gone or paying", "days running"},
		content.CauseVanished:    {"new identity", "lawyer"},
		content.CauseRetired:     {"offshore", "quiet"},
	} {
		for _, w := range words {
			if !strings.Contains(terms[cause], w) {
				t.Errorf("%s's terms %q lack %q", cause, terms[cause], w)
			}
		}
	}
}

// TestWalkAwaySaysWhatIsPending (#494): a reserve made this morning is
// named on the dialog (it lands tonight, and a walk away now neither
// scores it nor leaves it behind); last night's lump over the lot
// closes every way out, the dialog saying the pages, and y cannot get
// past it.
func TestWalkAwaySaysWhatIsPending(t *testing.T) {
	m := richModel(t, 120, 40)
	openEveryWayOut(m)
	w := m.w
	w.Player.CleanCash = 3_780
	if err := m.sess.Reserve(3_780); err != nil {
		t.Fatal(err)
	}
	m.Update(key("1"))
	m.Update(key("w"))
	if v := stripANSI(m.View()); !strings.Contains(v, "$3,780 lands offshore tonight: end the day first") {
		t.Fatalf("the dialog does not list the reserve:\n%s", v)
	}
	m.Update(key("enter"))
	if v := stripANSI(m.View()); m.exit.step != 1 || !strings.Contains(v, "$3,780 lands offshore tonight") {
		t.Fatalf("the confirmation does not list the reserve (step %d):\n%s", m.exit.step, v)
	}
	m.Update(key("esc"))

	// Last night's lump, three lots: its pages close every way out.
	lot := m.rules.Laundering.Offshore().Lot
	w.Today.Reserved = 0
	w.Laundering.Structured.Day, w.Laundering.Structured.Amount, w.Laundering.Structured.Lots = w.Day, 3*lot, m.rules.Laundering.Lots(3*lot)
	pages := m.sess.PagesDue()
	if pages <= 0 {
		t.Fatalf("no pages due on a lump of %d", 3*lot)
	}
	for _, r := range m.exitRows() {
		if r.open || !strings.Contains(r.short, "DA's file tonight") {
			t.Fatalf("%s with the pages due: open %v short %q", r.name, r.open, r.short)
		}
	}
	m.Update(key("w"))
	if v := stripANSI(m.View()); !strings.Contains(v, "Last night's transfer offshore puts "+plural(pages, "page")+" in the DA's file tonight") {
		t.Fatalf("the dialog does not say the pages:\n%s", v)
	}
	m.Update(key("enter"))
	if m.exit.step != 0 || !strings.Contains(m.exit.err, "end the day first") {
		t.Fatalf("enter with the pages due: step %d err %q", m.exit.step, m.exit.err)
	}
	m.Update(key("y"))
	if w.Over != nil {
		t.Fatalf("y with the pages due ended the run: %+v", w.Over)
	}
}

// TestReadyEndingPointsAtTheWalkAway (#498): a plan whose ending is
// open says where it is taken on the ambitions panel (w on the
// dashboard), the report's PLAN line says so too, and the dashboard's
// alerts carry it.
func TestReadyEndingPointsAtTheWalkAway(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	off := m.rules.Laundering.Offshore()
	m.Update(key("1"))
	m.Update(key("a"))
	if v := stripANSI(m.View()); strings.Contains(v, "on the dashboard walks away") {
		t.Fatalf("a plan not ready points at the walk away:\n%s", v)
	}
	m.Update(key("esc"))
	w.Offshore, w.QuietDays = off.RetireCash, off.RetireDays
	if err := m.sess.PinAmbition(content.AmbitionRetire); err != nil {
		t.Fatal(err)
	}
	m.Update(key("a"))
	if v := stripANSI(m.View()); !strings.Contains(v, "Ready to take: w on the dashboard walks away on it.") {
		t.Fatalf("the ready plan does not point at the walk away:\n%s", v)
	}
	m.Update(key("esc"))
	if got := strings.Join(m.planReport(), " "); !strings.Contains(got, "ready. Walk away on the dashboard") {
		t.Fatalf("the PLAN line: %q", got)
	}
	if alerts := stripANSI(strings.Join(m.alertLines(120, 12), "\n")); !strings.Contains(alerts, "You could retire") {
		t.Fatalf("the alerts do not say retiring is open:\n%s", alerts)
	}
	w.Upgrades["identity"] = true
	if alerts := stripANSI(strings.Join(m.alertLines(120, 12), "\n")); !strings.Contains(alerts, "vanish on the new identity") {
		t.Fatalf("the alerts do not say vanishing is open:\n%s", alerts)
	}
}

// TestTheEndingTakesADeliberateKey (#498): the morning a run ends, the
// summary does not close on esc (a tester mashing esc through the
// reports went past it unread); its footer lists M menu, which leaves.
// Continued from its save, the summary closes on esc as before.
func TestTheEndingTakesADeliberateKey(t *testing.T) {
	m := richModel(t, 100, 30)
	openEveryWayOut(m)
	m.Update(key("1"))
	m.Update(key("w"))
	m.Update(key("enter"))
	m.Update(key("y"))
	if m.mode != modeOver || m.w.Over == nil {
		t.Fatalf("y: mode %v over %+v", m.mode, m.w.Over)
	}
	for _, k := range []string{"esc", "enter", "down", "esc"} {
		m.Update(key(k))
	}
	if m.mode != modeOver {
		t.Fatalf("mashed keys left the fresh summary: mode %v", m.mode)
	}
	if f := footerKeys(m); !strings.Contains(f, "M menu") || strings.Contains(f, "esc close") {
		t.Fatalf("the fresh summary's footer: %q", f)
	}
	m.Update(key("M"))
	if m.mode != modeStart {
		t.Fatalf("M: mode %v", m.mode)
	}
	if err := m.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	if m.mode != modeOver {
		t.Fatalf("continued: mode %v", m.mode)
	}
	if f := footerKeys(m); !strings.Contains(f, "esc close") {
		t.Fatalf("the summary seen before: %q", f)
	}
	m.Update(key("esc"))
	if m.mode != modeStart {
		t.Fatalf("esc on a summary seen before: mode %v", m.mode)
	}
}

// TestScoreLineSpellsItOut (#498): the summary's score line says what
// the score is, the offshore account over one plus the bodies, and
// with nothing offshore that only the account scores; a tie with
// another run says tied.
func TestScoreLineSpellsItOut(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Offshore = 0
	w.Over = w.End(content.CauseIndicted, w.Day, "")
	m.finish(false)
	v := stripANSI(strings.Join(m.summaryLines(), "\n"))
	for _, want := range []string{"the offshore account $0 ÷ (1 + 0 bodies)", "nothing was moved offshore, and only the account scores"} {
		if !strings.Contains(v, want) {
			t.Errorf("the score line lacks %q:\n%s", want, v)
		}
	}
	m.profile.Runs = append(m.profile.Runs, m.profile.Runs[len(m.profile.Runs)-1])
	if got := m.rankLine(); !strings.HasPrefix(got, "tied 1st of") {
		t.Fatalf("two runs at $0: %q", got)
	}
}
