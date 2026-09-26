package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// withLieutenant puts a lieutenant on the payroll running the city you
// are not in, and returns them.
func withLieutenant(t *testing.T, m *Model) *game.CrewMember {
	t.Helper()
	w := m.w
	city := ""
	for _, cid := range w.CityOrder {
		if cid != w.Player.Location {
			city = cid
			break
		}
	}
	if city == "" {
		t.Fatal("one city")
	}
	id := w.Crew.NextID + 1
	w.Crew.NextID = id
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: id, Name: "Wally", Role: game.RoleLieutenant, Personality: "steady", Skill: 50, Loyalty: 80, Nerve: 50, Wage: 50})
	if err := w.Assign(id, city); err != nil {
		t.Fatal(err)
	}
	return w.Crew.Member(id)
}

// Taking a lieutenant off their city over the cap says what happens to
// the extra crew (#497: the roster read 20 of 18 and nothing said so):
// the picker warns before, the status says it after, and the crew
// screen's title carries it while it lasts.
func TestUnassignSaysWhatHappensToTheExtraCrew(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	lt := withLieutenant(t, m)
	for len(w.Crew.Members) <= m.rules.Crew.MaxCrew(w)-m.rules.Crew.Lieutenancy().Crew {
		id := w.Crew.NextID + 1
		w.Crew.NextID = id
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: id, Name: "Extra", Role: game.RoleRunner, Skill: 50, Loyalty: 80, Nerve: 50, Wage: 50})
	}
	m.subjectID = lt.ID
	m.mode = modeAssign
	rows := m.assignRows()
	m.pick.cursor = len(rows) - 1 // nobody's
	view := stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "assign picker over the cap")
	if !strings.Contains(view, "Off the city, the roster is") || !strings.Contains(view, "nobody is let go") {
		t.Fatalf("the picker does not warn:\n%s", view)
	}
	m.confirmAssign()
	most := m.rules.Crew.MaxCrew(w)
	if lt = w.Crew.Member(lt.ID); lt.City != "" || !strings.Contains(m.status, "nobody is let go, and nobody is hired until it is under") || len(w.Crew.Members) <= most {
		t.Fatalf("unassigned over the cap: city %q, %d of %d, status %q", lt.City, len(w.Crew.Members), most, m.status)
	}
	m.Update(key("4"))
	if v := stripANSI(m.View()); !strings.Contains(v, "over: nobody hired") {
		t.Fatalf("the crew screen does not say it is over the cap:\n%s", v)
	}
}

// The lie-low help and its status say whether the lieutenant keeps
// selling (#497: a tester's heat rose over lie-low nights and nothing
// said who sold): nobody does, and with a lieutenant running a city the
// status and the dashboard say so.
func TestLieLowSaysWhetherTheLieutenantSells(t *testing.T) {
	for _, b := range bindings {
		if b.key == "l" && b.label == "lie low" && !strings.Contains(b.help, "lieutenants stop too") {
			t.Fatalf("the lie-low help does not name the lieutenants: %q", b.help)
		}
	}
	m := newTestModel(t, 120, 40)
	m.Update(key("l"))
	if strings.Contains(m.status, "lieutenant") {
		t.Fatalf("no lieutenant, and the status names one: %q", m.status)
	}
	m.Update(key("l"))
	withLieutenant(t, m)
	m.Update(key("l"))
	if !m.w.Today.LieLow || !strings.Contains(m.status, "your lieutenants' included") || !strings.Contains(m.status, "wages and contracts still run") {
		t.Fatalf("lying low with a lieutenant: %v, status %q", m.w.Today.LieLow, m.status)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "No sales, your lieutenants' included") {
		t.Fatalf("the dashboard does not say it:\n%s", v)
	}
}

// A lieutenant under the informant line is an alert (#497: Gato
// betrayed a run sixteen days after his hire and nothing warned as his
// loyalty fell), red, naming the line and the crew screen.
func TestLieutenantUnderTheFlipLineWords(t *testing.T) {
	m := newTestModel(t, 120, 40)
	lt := withLieutenant(t, m)
	flip := m.rules.Crew.FlipLine()
	lt.Loyalty = flip - 1
	lines := m.alertsOf(engine.AlertCrewLine)
	if len(lines) != 1 {
		t.Fatalf("crew line alerts %+v", lines)
	}
	got := stripANSI(lines[0].text)
	if !strings.HasPrefix(got, "Wally is under 30 loyalty: a lieutenant that low talks to the police") || !strings.Contains(got, screenPointer(screenCrew)) {
		t.Errorf("the alert reads %q", got)
	}
	if lines[0].why != "Wally under the 30 line" {
		t.Errorf("the stop reads %q", lines[0].why)
	}
}

// h on a faction's scouts confirms, then says what it did, and the
// morning reports the result (#506); off a faction on its way it
// refuses with where to look rather than doing nothing.
func TestHitScoutsConfirmsAndReports(t *testing.T) {
	m := tableModel(t, 120, 40)
	w := m.w
	away := ""
	for _, cid := range w.CityOrder {
		if cid != w.Home().ID {
			away = cid
		}
	}
	r := w.Rivals[len(w.Rivals)-1]
	r.Arrived, r.ScoutingCity, r.ScoutDay, r.Recruited, r.ScoutsHit = 0, away, w.Day, 0, 0
	r.Absorbed, r.Fragmented = 0, 0
	if w.Crew.OnPayroll(game.RoleEnforcer) == 0 {
		id := w.Crew.NextID + 1
		w.Crew.NextID = id
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: id, Name: "Tank", Role: game.RoleEnforcer, Skill: 50, Loyalty: 80, Nerve: 50, Wage: 50})
	}
	m.Update(key("8"))
	m.factionCursor = 0
	if m.faction() == r {
		t.Fatal("the cursor starts on the scouts")
	}
	m.Update(key("h"))
	if m.mode != modePlay || !strings.Contains(m.status, "Nobody to hit here") || !strings.Contains(m.status, r.Leader) {
		t.Fatalf("h off the scouts: mode %v, status %q", m.mode, m.status)
	}
	m.factionCursor = len(w.Rivals) - 1
	m.Update(key("h"))
	if m.mode != modeConfirm || w.Today.HitScouts != "" {
		t.Fatalf("h on the scouts acted without asking: mode %v, queued %q", m.mode, w.Today.HitScouts)
	}
	m.Update(key("y"))
	if w.Today.HitScouts != r.Faction() || !strings.Contains(m.status, "The enforcers go after") {
		t.Fatalf("confirmed: queued %q, status %q", w.Today.HitScouts, m.status)
	}
	endDay(t, m)
	rep := strings.Join(w.Report.Territory, "\n")
	if !strings.Contains(rep, "scouts") || !(strings.Contains(rep, "Your enforcers ran "+r.Leader) || strings.Contains(rep, "Your enforcers went after "+r.Leader)) {
		t.Fatalf("the morning does not report the hit:\n%s", rep)
	}
}

// The campaign page shows the price of a point of the vote before an
// amount is typed (#506: $15K went in for a tenth of a point before the
// dialog said a point was $200,000).
func TestCampaignShowsThePriceFirst(t *testing.T) {
	m := newTestModel(t, 80, 24)
	w := m.w
	w.Player.CleanCash = 5_000_000
	w.Law.CampaignOpen = true
	m.Update(key("7"))
	m.Update(key("f"))
	m.Update(key("tab"))
	if m.modalStep() != 1 {
		t.Fatalf("not on the campaign page: step %d", m.modalStep())
	}
	assertFits(t, m.View(), 80, 24, "campaign page")
	cmp := m.rules.Law.Campaign()
	view := stripANSI(m.View())
	if !strings.Contains(view, money(cmp.Cash)+" a point of the vote") {
		t.Fatalf("the campaign page does not show the price before the amount:\n%s", view)
	}
}

// A war with no enforcer at work says what it does with nobody to send
// (#506: the war went on with none, "enforcers go to The Heights
// tonight"): nobody goes in and nothing is taken, on the dashboard and
// the rivals pane.
func TestWarWithNobodyToSendSaysSo(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	var kept []game.CrewMember
	for _, c := range w.Crew.Members {
		if c.Role != game.RoleEnforcer {
			kept = append(kept, c)
		}
	}
	w.Crew.Members = kept
	m.Update(key("8"))
	w.War = m.faction().Faction()
	var pane []string
	for _, s := range m.rivalsDetails() {
		pane = append(pane, s.lines...)
	}
	if got := stripANSI(strings.Join(pane, " ")); !strings.Contains(got, "nobody goes in tonight") {
		t.Fatalf("the rivals pane does not say it:\n%s", got)
	}
	m.Update(key("1"))
	if v := stripANSI(m.View()); !strings.Contains(v, "nobody goes in tonight") || strings.Contains(v, "enforcers go to") {
		t.Fatalf("the dashboard says the enforcers go:\n%s", v)
	}
}

// A proposal that replaces tonight's to another faction says so (#506:
// the Preacher's proposal vanished under Carmine's).
func TestProposalSaysWhatItReplaces(t *testing.T) {
	m := tableModel(t, 120, 40)
	w := m.w
	var alive []*game.RivalState
	for _, r := range w.Rivals {
		if r != nil && r.Alive() {
			alive = append(alive, r)
		}
	}
	if len(alive) < 2 {
		t.Fatalf("%d factions on the ground", len(alive))
	}
	if err := w.ProposeTo(alive[0].Faction(), game.DealTruce, game.Terms{Days: 5}); err != nil {
		t.Fatal(err)
	}
	m.Update(key("8"))
	for i, r := range w.Rivals {
		if r == alive[1] {
			m.factionCursor = i
		}
	}
	m.Update(key("d"))
	if m.mode != modePropose {
		t.Fatalf("d: mode %v, status %q", m.mode, m.status)
	}
	m.Update(key("enter")) // truce
	m.Update(key("enter")) // its first term
	if p := w.Today.Proposal; p == nil || w.Faction(p.Faction) != alive[1] || !strings.Contains(m.status, "It replaces a 5-day truce to "+alive[0].Leader+"'s crew") {
		t.Fatalf("the second proposal: %+v, status %q", p, m.status)
	}
}
