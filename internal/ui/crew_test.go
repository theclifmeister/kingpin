package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The crew screen: h hires the selected candidate, f asks before firing,
// p cycles the pay dial, and none of it happens from other screens.
func TestCrewScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 5000
	m.Update(key("h"))
	if len(m.w.Crew.Members) != 0 {
		t.Fatal("hired from the dashboard")
	}
	m.Update(key("4"))
	if m.screen != screenCrew {
		t.Fatalf("screen = %v", m.screen)
	}
	m.Update(key("h")) // cursor starts on the first candidate when the roster is empty
	if len(m.w.Crew.Members) != 1 {
		t.Fatalf("hire failed: %q", m.status)
	}
	hired := m.w.Crew.Members[0]
	if m.w.Capacity(m.w.Player.Location) != m.w.Player.CarryLimit+hired.Units || m.w.Player.DirtyCash != 5000-hired.Fee {
		t.Fatalf("after hire: capacity %d cash %d, member %+v", m.w.Capacity(m.w.Player.Location), m.w.Player.DirtyCash, hired)
	}
	m.Update(key("p"))
	if m.w.Crew.Pay != events.PayGenerous {
		t.Fatalf("pay after one p = %v", m.w.Crew.Pay)
	}
	m.Update(key("p"))
	m.Update(key("p"))
	if m.w.Crew.Pay != events.PayFair {
		t.Fatalf("pay after three p = %v", m.w.Crew.Pay)
	}
	// Cursor is on the new hire; f asks, anything but y backs out.
	m.Update(key("f"))
	if m.mode != modeConfirmFire {
		t.Fatalf("f did not ask: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || len(m.w.Crew.Members) != 1 {
		t.Fatal("esc fired someone")
	}
	m.Update(key("f"))
	m.Update(key("y"))
	if len(m.w.Crew.Members) != 0 || len(m.w.Crew.FiredToday) != 1 {
		t.Fatalf("y did not fire: %+v", m.w.Crew)
	}
	m.Update(key("n"))
	if m.mode != modeReport || len(m.w.Report.Crew) == 0 {
		t.Fatalf("report has no crew lines: %+v", m.w.Report)
	}
	if !strings.Contains(strings.Join(m.w.Report.Crew, "\n"), hired.Name) {
		t.Fatalf("report does not mention %s: %v", hired.Name, m.w.Report.Crew)
	}
}

// i and $ work only on the crew screen, ask first, and the tell shows on
// the dashboard and the crew screen once the file has grown twice without
// a bust; an investigation that names somebody marks them on the roster.
func TestInvestigateAndPayOffKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.w.Player.DirtyCash = 20_000
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "crew screen") {
		t.Fatalf("i on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("4"))
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "nobody on the payroll") || m.statusKind != statusWarning {
		t.Fatalf("i with no crew: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("h"))
	if len(m.w.Crew.Members) != 1 {
		t.Fatalf("hire failed: %q", m.status)
	}
	hired := m.w.Crew.Members[0]
	cash := m.w.Player.DirtyCash
	m.Update(key("i"))
	if m.mode != modeConfirmInvestigate {
		t.Fatalf("i did not ask: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "investigate confirmation")
	m.Update(key("esc"))
	if m.mode != modePlay || m.w.Today.Investigation != nil || m.w.Player.DirtyCash != cash {
		t.Fatal("esc queued an investigation")
	}
	m.Update(key("i"))
	m.Update(key("y"))
	if m.w.Today.Investigation == nil || m.w.Player.DirtyCash != cash-m.set.Crew.InvestigateCost() {
		t.Fatalf("y did not queue: %+v cash %d status %q", m.w.Today.Investigation, m.w.Player.DirtyCash, m.status)
	}
	m.Update(key("i"))
	if m.mode != modePlay || !strings.Contains(m.status, "already") {
		t.Fatalf("second i: mode %v status %q", m.mode, m.status)
	}
	if pane := paneText(m); !strings.Contains(pane, "questions get asked") {
		t.Fatalf("the crew pane does not show the queued investigation:\n%s", pane)
	}

	// Pay off the new hire.
	cash = m.w.Player.DirtyCash
	m.Update(key("$"))
	if m.mode != modeConfirmPayOff {
		t.Fatalf("$ did not ask: mode %v status %q", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "pay-off confirmation")
	m.Update(key("y"))
	c := m.w.Crew.Member(hired.ID)
	if c.Loyalty != min(100, hired.Loyalty+m.set.Crew.PayoffLoyalty()) || m.w.Player.DirtyCash != cash-m.set.Crew.PayoffCost(hired) || len(m.w.Crew.PaidOffToday) != 1 {
		t.Fatalf("pay off: loyalty %.0f -> %.0f cash %d -> %d status %q", hired.Loyalty, c.Loyalty, cash, m.w.Player.DirtyCash, m.status)
	}

	// The tell, and a named snitch, render everywhere they should.
	m.w.Heat.Leaks = 2
	m.w.Crew.Exposed = hired.ID
	for _, s := range []string{"1", "4"} {
		m.Update(key(s))
		view := stripANSI(m.View())
		assertFits(t, m.View(), 80, 24, "screen "+s+" with the tell")
		if !strings.Contains(view, "Somebody is talking") {
			t.Fatalf("screen %s does not hint at the informant:\n%s", s, view)
		}
	}
	if pane := paneText(m); !strings.Contains(pane, "SNITCH") || !strings.Contains(pane, "snitch      "+hired.Name) {
		t.Fatalf("the pane does not mark the named informant:\n%s", pane)
	}
	endDay(t, m) // the first hire's stage (#149) opens before the report
	if m.mode != modeReport {
		t.Fatalf("mode after n: %v", m.mode)
	}
	all := strings.Join(append(append([]string(nil), m.w.Report.Crew...), m.w.Report.Money...), "\n")
	if !strings.Contains(all, "investigation") || !strings.Contains(all, "Paid off "+hired.Name) || !strings.Contains(all, "Investigation -$") {
		t.Fatalf("report does not cover the night's questions and the pay-off:\n%s", all)
	}
	assertFits(t, m.View(), 80, 24, "report after an investigation")
	m.Update(key("enter"))
	if m.w.Report.CashBefore != 20_000 {
		t.Fatalf("cash before = %d, want the morning's 20000", m.w.Report.CashBefore)
	}
}

// t on the crew screen gives the selected lieutenant a city through a
// picker (and takes it away again); anywhere else it only points at the
// map, where the routes run themselves. The roster
// shows the city and hides the temper until it has been observed, and
// the dashboard says who runs what.
func TestAssignLieutenantKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	w.Player.DirtyCash = 50_000
	w.Crew.Members = append(w.Crew.Members,
		game.CrewMember{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 80, Nerve: 50, Wage: 50},
		game.CrewMember{ID: 2, Name: "Marcus", Role: game.RoleLieutenant, Skill: 70, Loyalty: 80, Nerve: 50, Wage: 150, Personality: "violent"},
	)
	w.Crew.NextID = 2
	other := w.CityOrder[1]

	m.Update(key("t")) // dashboard: a pointer, not the picker
	if m.mode != modePlay || !strings.Contains(m.status, "crew screen (4)") {
		t.Fatalf("t on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("4"))
	m.crewCursor = 0
	m.Update(key("t")) // a runner
	if m.mode != modePlay || !strings.Contains(m.status, "only a lieutenant") || m.statusKind != statusWarning {
		t.Fatalf("t on a runner: mode %v status %q", m.mode, m.status)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "no city") || strings.Contains(view, "violent") {
		t.Fatalf("crew screen before assigning:\n%s", view)
	}
	m.crewCursor = 1
	if pane := paneText(m); strings.Contains(pane, "violent") || !strings.Contains(pane, "temper      shows on the job") || !strings.Contains(pane, "t  give them a city") {
		t.Fatalf("pane before assigning:\n%s", pane)
	}
	m.Update(key("t"))
	if m.mode != modeAssign {
		t.Fatalf("mode after t on a lieutenant = %v (%s)", m.mode, m.status)
	}
	assertFits(t, m.View(), 80, 24, "assign picker")
	m.Update(key("2")) // the second city
	if m.mode != modePlay {
		t.Fatalf("mode after picking = %v", m.mode)
	}
	if lt := w.Crew.Lieutenant(other); lt == nil || lt.ID != 2 || lt.Assigned != w.Day {
		t.Fatalf("after assigning: %+v (%s)", w.Crew.Members[1], m.status)
	}
	view = stripANSI(m.View())
	if !strings.Contains(view, "runs "+w.CityName(other)) || strings.Contains(view, "Bayport ?") {
		t.Fatalf("crew screen after assigning:\n%s", view)
	}
	if pane := paneText(m); !strings.Contains(pane, "temper      shows in 10 days") || strings.Contains(pane, "violent") || !strings.Contains(pane, w.CityName(other)+"     Marcus") {
		t.Fatalf("pane after assigning:\n%s", pane)
	}
	assertFits(t, m.View(), 80, 24, "crew screen with a lieutenant")
	m.Update(key("1"))
	if view := m.View(); !strings.Contains(view, "Marcus runs "+w.CityName(other)) {
		t.Fatalf("dashboard after assigning:\n%s", view)
	}
	// The night's report says what they did, and the temper shows once
	// observed.
	endDay(t, m)
	if m.mode != modeReport {
		t.Fatalf("mode after the day = %v", m.mode)
	}
	found := false
	for _, l := range w.Report.Crew {
		found = found || strings.Contains(l, "Marcus runs "+w.CityName(other))
	}
	if !found {
		t.Fatalf("report crew lines: %v", w.Report.Crew)
	}
	m.Update(key("enter"))
	w.Crew.Member(2).Observed = true
	m.Update(key("4"))
	if pane := paneText(m); !strings.Contains(pane, "temper      violent") || strings.Contains(pane, "shows in") || !strings.Contains(pane, "Marcus · violent") {
		t.Fatalf("pane with the temper observed:\n%s", pane)
	}
	// And back off the city: the last row of the picker.
	m.crewCursor = 1
	m.Update(key("t"))
	m.Update(key("down"))
	m.Update(key("down"))
	m.Update(key("enter"))
	if w.Crew.Lieutenant(other) != nil || w.Crew.Member(2).City != "" {
		t.Fatalf("after unassigning: %+v (%s)", *w.Crew.Member(2), m.status)
	}
	assertFits(t, m.View(), 80, 24, "crew screen after unassigning")
}

// TestCrewScreenAccountantIsNotIdle: an accountant works every front you
// own and has no post, so the status column says so instead of idle, the
// pane's CREW section does not count them idle and says what they add
// (#62, #86).
func TestCrewScreenAccountantIsNotIdle(t *testing.T) {
	m := newTestModel(t, 100, 30)
	fronts := m.cfg.Laundering.Fronts
	m.w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Nadia", Role: "accountant", Skill: 50, Loyalty: 70, Nerve: 50, Wage: 60}}
	m.Update(key("4"))

	m.w.Fronts = []game.Front{{ID: fronts[0].ID, Name: fronts[0].Name}}
	v := stripANSI(m.View())
	if !strings.Contains(v, "the books") {
		t.Fatalf("one front: no 'the books' on the accountant's row:\n%s", v)
	}
	if strings.Contains(v, "idle") {
		t.Fatalf("one front: the accountant is called idle:\n%s", v)
	}
	tun := m.cfg.Laundering.Laundering
	pane := paneText(m)
	for _, want := range []string{
		fmt.Sprintf("accountant  +%s/day a front", money(int(tun.AccountantThroughput*0.5))),
		fmt.Sprintf("audit risk cut %.0f%%", tun.AccountantRiskCut*0.5*100),
		"post        the books",
	} {
		if !strings.Contains(pane, want) {
			t.Fatalf("one front: the pane does not say %q:\n%s", want, pane)
		}
	}
	if strings.Contains(pane, "idle") {
		t.Fatalf("one front: the pane calls the accountant idle:\n%s", pane)
	}

	m.w.Fronts = append(m.w.Fronts, game.Front{ID: fronts[1].ID, Name: fronts[1].Name})
	if v := stripANSI(m.View()); !strings.Contains(v, "2 fronts") || strings.Contains(v, "idle") {
		t.Fatalf("two fronts: want '2 fronts' and no idle:\n%s", v)
	}

	m.w.Fronts = nil
	v = stripANSI(m.View())
	if !strings.Contains(v, "no front") {
		t.Fatalf("no front: want the status:\n%s", v)
	}
	if pane := paneProse(m); !strings.Contains(pane, "An accountant with no front is a wage. Buy on the ledger screen (7).") || !strings.Contains(pane, "post no front to work") {
		t.Fatalf("no front: want the warning in the pane:\n%s", pane)
	}
	if strings.Contains(v, "idle") || strings.Contains(v, "the books") {
		t.Fatalf("no front: accountant is idle or on the books:\n%s", v)
	}
}

// TestCrewScreenUnpostedEnforcer: an enforcer without a corner is unposted
// and the pane counts them guarding nothing; a runner without one is idle
// and counted earning nothing, with the pointer to the map; the idle
// count is runners only (#62, #86).
func TestCrewScreenUnpostedEnforcer(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Moose", Role: "enforcer", Skill: 70, Loyalty: 70, Nerve: 60, Wage: 65}}
	m.Update(key("4"))
	v := stripANSI(m.View())
	if !strings.Contains(v, "unposted") || strings.Contains(v, "idle") {
		t.Fatalf("enforcer: want 'unposted' and no 'idle':\n%s", v)
	}
	pane := paneProse(m)
	if !strings.Contains(pane, "unposted 1 enforcer") || strings.Contains(pane, "idle") || !strings.Contains(pane, "an enforcer guards nothing off a corner. Post them on the map screen (5).") {
		t.Fatalf("enforcer: want the pane to count one guarding nothing:\n%s", pane)
	}

	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 2, Name: "Ray", Role: "runner", Skill: 40, Loyalty: 70, Nerve: 50, Wage: 40, Units: 20})
	v = stripANSI(m.View())
	if !strings.Contains(v, "idle") {
		t.Fatalf("runner: want 'idle':\n%s", v)
	}
	pane = paneProse(m)
	if !strings.Contains(pane, "idle 1 runner") || !strings.Contains(pane, "unposted 1 enforcer") || !strings.Contains(pane, "A runner earns nothing") {
		t.Fatalf("runner: want the pane to count one idle and one unposted:\n%s", pane)
	}

	// The hiring pool names each candidate's role and fee.
	m.w.Crew.Candidates = []game.CrewMember{
		{ID: 3, Name: "Pat", Role: "accountant", Skill: 50, Loyalty: 60, Nerve: 50, Wage: 60, Fee: 100},
		{ID: 4, Name: "Bo", Role: "enforcer", Skill: 50, Loyalty: 60, Nerve: 50, Wage: 60, Fee: 100},
	}
	v = stripANSI(m.View())
	if !strings.Contains(v, "Pat   accountant") || !strings.Contains(v, "Bo    enforcer") || !strings.Contains(v, "$100") {
		t.Fatalf("pool: want the roles and fees:\n%s", v)
	}
}

// paneProse is paneText (map_test.go) with each section's lines run
// together, so a wrapped note reads whole.
func paneProse(m *Model) string {
	var b strings.Builder
	for _, s := range m.details() {
		b.WriteString(s.title + "\n")
		var ls []string
		for _, l := range s.lines {
			ls = append(ls, strings.TrimSpace(stripANSI(l)))
		}
		b.WriteString(spaces.ReplaceAllString(strings.Join(ls, " "), " ") + "\n")
	}
	return b.String()
}

// TestCrewPaneNamesTheCosts: the pane's h, i and $ lines carry the same
// numbers hireSelected, investigateConfirm and payOffConfirm use, and
// the wage line the other two dials' wages (#86).
func TestCrewPaneNamesTheCosts(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.w.Player.DirtyCash = 20_000
	m.Update(key("4"))
	cand := m.w.Crew.Candidates[0]
	pane := paneText(m)
	if !strings.Contains(pane, "h  hire for "+money(cand.Fee)) || !strings.Contains(pane, fmt.Sprintf("%s · skill %d · fee %s", cand.Role, cand.Skill, money(cand.Fee))) {
		t.Fatalf("the candidate's pane does not name the fee %s:\n%s", money(cand.Fee), pane)
	}
	m.Update(key("h"))
	hired := m.w.Crew.Members[0]
	if m.w.Player.DirtyCash != 20_000-cand.Fee || hired.Fee != cand.Fee {
		t.Fatalf("hire charged %d, the pane said %d", 20_000-m.w.Player.DirtyCash, cand.Fee)
	}
	pane = paneText(m)
	crew := m.set.Crew
	want := []string{
		strings.ToUpper(hired.Name),
		fmt.Sprintf("%s · skill %d · hired d%d", hired.Role, hired.Skill, hired.Hired),
		fmt.Sprintf("wage        %s/day fair", money(crew.WageAt(m.w, hired, events.PayFair))),
		fmt.Sprintf("stingy %s · generous %s", money(crew.WageAt(m.w, hired, events.PayStingy)), money(crew.WageAt(m.w, hired, events.PayGenerous))),
		fmt.Sprintf("skims under %.0f · walks at %.0f", crew.Tuning().SkimThreshold, crew.Tuning().QuitThreshold),
		fmt.Sprintf("f  fire: the rest lose %.0f loyalty", crew.Tuning().FireLoyalty),
		fmt.Sprintf("i  ask around %s, names ~%.0f%%", money(crew.InvestigateCost()), crew.InvestigateOdds(m.w)*100),
		fmt.Sprintf("$  pay off for %s: %.0f → %.0f", money(crew.PayoffCost(hired)), hired.Loyalty, min(100, hired.Loyalty+crew.PayoffLoyalty())),
	}
	for _, w := range want {
		if !strings.Contains(pane, w) {
			t.Errorf("the member's pane lacks %q:\n%s", w, pane)
		}
	}
	// The confirmations say the same.
	m.Update(key("i"))
	if v := stripANSI(m.View()); !strings.Contains(v, money(crew.InvestigateCost())) || !strings.Contains(v, fmt.Sprintf("~%.0f%%", crew.InvestigateOdds(m.w)*100)) {
		t.Fatalf("the investigate confirmation disagrees with the pane:\n%s", v)
	}
	m.Update(key("esc"))
	m.Update(key("$"))
	if v := stripANSI(m.View()); !strings.Contains(v, fmt.Sprintf("%s for %s: loyalty %.0f → %.0f", money(crew.PayoffCost(hired)), hired.Name, hired.Loyalty, min(100, hired.Loyalty+crew.PayoffLoyalty()))) {
		t.Fatalf("the pay-off confirmation disagrees with the pane:\n%s", v)
	}
	m.Update(key("esc"))
	// A queued investigation and a named snitch change the lines.
	m.Update(key("i"))
	m.Update(key("y"))
	m.w.Crew.Exposed = hired.ID
	pane = paneText(m)
	for _, w := range []string{"i  ask around: already asking", "SNITCH: talking to the police", "f  fire: the file stops growing", "snitch      " + hired.Name + ", fire them"} {
		if !strings.Contains(pane, w) {
			t.Errorf("the pane lacks %q:\n%s", w, pane)
		}
	}
	for _, s := range m.details() {
		for _, l := range s.lines {
			if lipgloss.Width(l) > paneTextW {
				t.Errorf("pane line wider than %d: %q", paneTextW, stripANSI(l))
			}
		}
	}
}

// TestCrewScreenInTheGrammar: MAIN is the title with the count, the pay
// dial in the dial convention, the warning line cut with … and whole in
// the pane, and the two tables; nothing under them. Every pane line fits
// the pane's width for every row of the rich fixture, at most one blank
// row sits between sections at 80x24 and the title has no `(s)` (#86).
func TestCrewScreenInTheGrammar(t *testing.T) {
	m := richModel(t, 80, 24)
	m.Update(key("4"))
	view := stripANSI(m.View())
	main := strings.Split(stripANSI(m.viewScreen()), "\n")
	if !strings.HasPrefix(main[0], fmt.Sprintf("CREW · %d of %d on the payroll", len(m.w.Crew.Members), m.set.Crew.MaxCrew(m.w))) {
		t.Errorf("title: %q", main[0])
	}
	if !strings.HasPrefix(main[1], fmt.Sprintf("pay  stingy  [fair]  generous   %s/day", money(m.set.Crew.Wages(m.w, events.PayFair)))) {
		t.Errorf("pay line: %q", main[1])
	}
	if !strings.HasPrefix(main[2], "▲ Skimming suspected. Money went missing on day") || !strings.HasSuffix(strings.TrimRight(main[2], " "), "…") {
		t.Errorf("warning line: %q", main[2])
	}
	if !strings.HasPrefix(main[3], "ON THE PAYROLL") {
		t.Errorf("row 3: %q", main[3])
	}
	if pane := paneProse(m); !strings.Contains(pane, "Skimming suspected. Money went missing on day "+fmt.Sprint(m.w.Crew.LastSkim)+". Somebody's loyalty is under") {
		t.Errorf("the pane does not carry the warning whole:\n%s", pane)
	}
	last := ""
	for i, l := range main {
		l = strings.TrimRight(l, " ")
		if l == "" && last == "" && i > 0 {
			// Trailing blank rows are the room MAIN has left, not a gap.
			rest := strings.TrimSpace(strings.Join(main[i:], ""))
			if rest != "" {
				t.Errorf("two blank rows at %d:\n%s", i, view)
			}
		}
		last = l
	}
	if strings.Contains(view, "(s)") {
		t.Errorf("a (s) on the crew screen:\n%s", view)
	}
	// The pane fits for everyone.
	for i := range m.crewRows() {
		m.crewCursor = i
		secs := m.details()
		if len(secs) != 2 || secs[1].title != "CREW" {
			t.Fatalf("row %d: sections %v", i, secs)
		}
		for _, s := range secs {
			for _, l := range s.lines {
				if lipgloss.Width(l) > paneTextW {
					t.Errorf("row %d (%s): pane line wider than %d: %q", i, s.title, paneTextW, stripANSI(l))
				}
			}
		}
	}
	// The strip at 80 is the person after the name.
	m.crewCursor = 0
	c, _, _ := m.crewSelected()
	rows := strings.Split(stripANSI(m.View()), "\n")
	if strip := rows[m.mainHeight()+1]; !strings.HasPrefix(strip, "▸ "+strings.ToUpper(c.Name)+" · "+c.Role+" · skill ") || !strings.HasSuffix(strings.TrimRight(strip, " "), "␣ more") {
		t.Errorf("strip: %q", strip)
	}
}
