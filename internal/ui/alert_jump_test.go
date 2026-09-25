package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestEveryAlertHasATarget (#352): every act every kind can carry lands
// on a screen the TUI has, opens a dialog it knows and puts a cursor on
// a subject it can select.
func TestEveryAlertHasATarget(t *testing.T) {
	for _, k := range engine.AlertKinds() {
		acts := engine.ActsOf(k)
		if len(acts) == 0 {
			t.Errorf("%s names no act", k)
		}
		for _, a := range acts {
			if _, ok := alertScreen(a.Screen); !ok {
				t.Errorf("%s: the screen %q is none of the TUI's", k, a.Screen)
			}
			if _, ok := alertModes[a.Mode]; a.Mode != "" && !ok {
				t.Errorf("%s: the dialog %q is none of the TUI's", k, a.Mode)
			}
			if _, ok := alertSubjects[a.Subject]; a.Subject != "" && !ok {
				t.Errorf("%s: the subject %q is none the TUI selects", k, a.Subject)
			}
		}
	}
}

// pressAlert selects the morning's first alert of the kind in the
// dashboard's ALERTS with ] and opens it with o, and returns it.
func pressAlert(t *testing.T, m *Model, kind engine.AlertKind) engine.Alert {
	t.Helper()
	m.Update(key("1"))
	as := m.sess.Alerts()
	for i, a := range as {
		if a.Kind != kind {
			continue
		}
		for m.alertCursor != i {
			m.Update(key("]"))
		}
		if v := stripANSI(strings.Join(m.alertLines(paneTextW, 8), "\n")); !strings.Contains(v, "▸ ") {
			t.Fatalf("no alert is marked in ALERTS:\n%s", v)
		}
		m.Update(key("o"))
		return a
	}
	t.Fatalf("no %s alert in %+v", kind, as)
	return engine.Alert{}
}

// TestAlertJumpOpensItsTarget (#352): from the dashboard, ] picks an
// alert and o lands where it is answered, the subject under the cursor:
// a member with no post and a corner nobody works open the post picker
// on the corner with the runner under the picker's cursor; a member near
// a line their row on the crew screen; a full stash and a known house
// the house on the ledger; a buyer's contract due its row on the market.
// Every other kind lands on its screen. The report after a fast-forward
// stopped on an alert lists o and opens it too.
func TestAlertJumpOpensItsTarget(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	for _, c := range w.Corners() {
		if c.Held() {
			w.Corner(c.ID).Hand(game.OwnerNone, "", w.Day)
		}
	}
	idle := &w.Here().Corners[2]
	idle.Owner = game.OwnerPlayer
	w.Crew.Members = []game.CrewMember{{ID: 9, Name: "Vee", Role: game.RoleRunner, Skill: 50, Loyalty: 60, Nerve: 50, Wage: 50}}
	w.Player.DirtyCash = 100000

	if keys := stripANSI(strings.Join(paneKeyLines(m), " ")); !strings.Contains(keys, "o open alert") || !strings.Contains(keys, "[ ] alert") {
		t.Fatalf("the dashboard's KEYS lack the alert keys: %s", keys)
	}
	for _, kind := range []engine.AlertKind{engine.AlertUnposted, engine.AlertIdleCorner} {
		a := pressAlert(t, m, kind)
		if m.mode != modePost || m.screen != screenMap {
			t.Fatalf("%s: o landed on mode %v screen %v", kind, m.mode, m.screen)
		}
		if c := m.mapSelected(); c == nil || c.ID != idle.ID || a.Corner != idle.ID {
			t.Fatalf("%s: the picker is on %+v, the alert on %q", kind, c, a.Corner)
		}
		if who := m.postRows(game.RoleRunner)[m.pick.cursor]; who.ID != 9 {
			t.Fatalf("%s: the picker's cursor is on %s", kind, who.Name)
		}
		m.Update(key("esc"))
	}
	// Posted from the picker, both alerts go.
	pressAlert(t, m, engine.AlertUnposted)
	m.Update(key("enter"))
	if idle.Runner != 9 {
		t.Fatalf("enter in the picker posted %d on %s", idle.Runner, idle.Name)
	}
	for _, a := range m.sess.Alerts() {
		if a.Kind == engine.AlertUnposted || a.Kind == engine.AlertIdleCorner {
			t.Errorf("posted, and still %+v", a)
		}
	}

	// A member near the walk: their row on the crew screen.
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 10, Name: "Tariq", Role: game.RoleAccountant, Loyalty: m.cfg.Crew.Crew.QuitThreshold + 1, Nerve: 50, Wage: 50})
	pressAlert(t, m, engine.AlertCrewLine)
	if c, _, _ := m.crewSelected(); m.screen != screenCrew || m.mode != modePlay || c.ID != 10 {
		t.Fatalf("crew line: screen %v mode %v on %s", m.screen, m.mode, c.Name)
	}

	// A buyer's contract due tomorrow: its row on the market.
	c := offer(m, 10, w.Here().ID)
	w.Contract(c.ID).Status = game.ContractAccepted
	w.Contract(c.ID).Due = w.Day + 1
	pressAlert(t, m, engine.AlertContractDue)
	if got := m.selectedContract(); m.screen != screenMarket || got == nil || got.ID != c.ID {
		t.Fatalf("contract due: screen %v on %+v", m.screen, got)
	}

	// The report after a fast-forward that stopped on an alert.
	a := m.sess.Alerts()[0]
	m.fastStop, m.fastAlert = "Stopped after 1 day: "+m.alertOf(a).why+".", &a
	m.mode = modeReport
	if f := footerKeys(m); !strings.Contains(f, "o open alert") {
		t.Fatalf("the report's footer lacks the jump: %s", f)
	}
	m.Update(key("o"))
	if s, _ := alertScreen(a.Act.Screen); m.screen != s || (a.Act.Mode == "" && m.mode != modePlay) {
		t.Fatalf("o on the report: screen %v mode %v for %+v", m.screen, m.mode, a.Act)
	}
	m.fastStop, m.fastAlert = "", nil
	m.mode = modeReport
	if f := footerKeys(m); strings.Contains(f, "open alert") {
		t.Fatalf("a report with no stop lists the jump: %s", f)
	}
	m.Update(key("esc"))
}

// TestAlertJumpOnTheLedger (#352): a full stash and a house the police
// know about open the ledger on the house.
func TestAlertJumpOnTheLedger(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	if len(w.Houses) == 0 {
		t.Fatal("the rich fixture has no house")
	}
	h := &w.Houses[len(w.Houses)-1]
	h.Known = true
	pressAlert(t, m, engine.AlertHouseKnown)
	if r := m.ledgerSelected(); m.screen != screenLedger || r.kind != ledgerHouse || w.Houses[r.i].ID != h.ID {
		t.Fatalf("house known: screen %v row %+v", m.screen, r)
	}
	h.Known = false
	m.ledgerCursor = 0
	w.SetStock(h.City, w.Products[0], w.StockIn(h.City)+w.Capacity(h.City))
	pressAlert(t, m, engine.AlertStashFull)
	if r := m.ledgerSelected(); m.screen != screenLedger || r.kind != ledgerHouse || w.Houses[r.i].City != h.City {
		t.Fatalf("stash full: screen %v row %+v", m.screen, r)
	}
}

// TestEveryActLands (#352): every act of every kind, opened on the rich
// fixture with its subject there, lands on its screen and dialog with
// the subject under the cursor.
func TestEveryActLands(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	con := offer(m, 10, w.Here().ID)
	member := w.Crew.Members[0]
	corner := w.Here().Corners[1]
	for _, k := range engine.AlertKinds() {
		for _, act := range engine.ActsOf(k) {
			m.mode, m.screen = modePlay, screenDashboard
			a := engine.Alert{Kind: k, Act: act, Member: member.ID, Corner: corner.ID, City: w.Houses[0].City,
				Contract: con.ID, Supplier: w.Suppliers[0].ID, House: w.Houses[0].ID}
			m.openAlert(a)
			want, _ := alertScreen(act.Screen)
			wantMode := modePlay
			if act.Mode == engine.ModePost {
				wantMode = modePost
			}
			if m.screen != want || m.mode != wantMode {
				t.Errorf("%s %+v: screen %v mode %v", k, act, m.screen, m.mode)
				continue
			}
			ok := true
			switch act.Subject {
			case engine.SubjectMember:
				if act.Mode == "" {
					c, _, _ := m.crewSelected()
					ok = c.ID == member.ID
				}
			case engine.SubjectCorner:
				ok = m.mapSelected().ID == corner.ID
			case engine.SubjectContract:
				ok = m.selectedContract() != nil && m.selectedContract().ID == con.ID
			case engine.SubjectSupplier:
				ok = m.selectedSupplier() != nil && m.selectedSupplier().ID == w.Suppliers[0].ID
			case engine.SubjectCity:
				if m.screen == screenMap { // the port (#476): the map turned to the city
					ok = m.shown().ID == a.City
					break
				}
				r := m.ledgerSelected()
				ok = r.kind == ledgerHouse && w.Houses[r.i].ID == w.Houses[0].ID
			case engine.SubjectHouse:
				r := m.ledgerSelected()
				ok = r.kind == ledgerHouse && w.Houses[r.i].ID == w.Houses[0].ID
			}
			if !ok {
				t.Errorf("%s %+v: the subject is not under the cursor", k, act)
			}
			if m.mode != modePlay {
				m.Update(key("esc"))
			}
		}
	}
}

// paneKeyLines is the pane's KEYS section's lines.
func paneKeyLines(m *Model) []string {
	var out []string
	for _, b := range m.keysFor(m.screen) {
		out = append(out, b.key+" "+b.label)
	}
	return out
}

// The file alert says how close the indictment is and where the pages
// come off (#414).
func TestFileAlertWords(t *testing.T) {
	m := richModel(t, 120, 40)
	limit := m.rules.Heat.EvidenceArrest(m.w)
	if limit < 2 {
		t.Skip("no file limit on the fixture")
	}
	m.w.Heat.Evidence = limit - 1
	m.Update(key("1"))
	view := stripANSI(m.View())
	if want := fmt.Sprintf("File %d/%d: one more", limit-1, limit); !strings.Contains(view, want) {
		t.Errorf("the dashboard's alerts lack %q:\n%s", want, view)
	}
	got := m.alertsOf(engine.AlertFile)
	if len(got) != 1 || !strings.Contains(stripANSI(got[0].text), "one more bust indicts you") || !strings.Contains(stripANSI(got[0].text), "Legal upgrades on the upgrades screen (6)") {
		t.Errorf("the file alert: %+v", got)
	}
}

// TestNoCornerAlertWords (#471): the last corner in a city gone is a
// red alert with the ways back, the free corner named, the map's keys
// and the other city, and its o opens the post picker on that corner.
func TestNoCornerAlertWords(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home := w.Home()
	for i := range home.Corners {
		if home.Corners[i].Held() {
			home.Corners[i].Yours = true
			home.Corners[i].Hand(game.OwnerRival, "", w.Day)
		}
	}
	got := m.alertsOf(engine.AlertNoCorner)
	if len(got) != 1 {
		t.Fatalf("no_corner alerts %+v", got)
	}
	text := stripANSI(got[0].text)
	for _, want := range []string{"You hold no corner in " + home.Name + ": nothing sells there.", "Post on ", "(w)", "buy a block (d) on the map screen (5)", ", or sell in "} {
		if !strings.Contains(text, want) {
			t.Errorf("the alert %q lacks %q", text, want)
		}
	}
	m.Update(key("1"))
	for _, a := range m.sess.Alerts() {
		if a.Kind == engine.AlertNoCorner {
			m.openAlert(a)
		}
	}
	if m.screen != screenMap || m.mode != modePost {
		t.Fatalf("the jump landed on screen %v mode %v", m.screen, m.mode)
	}
}

// TestWagesAlertSaysTheAccountIsOut (#471): with money offshore, the
// wages the till cannot pay say the account does not count toward
// them and nothing comes back from it: two runs ended broke with
// $226K and $510K there.
func TestWagesAlertSaysTheAccountIsOut(t *testing.T) {
	m := richModel(t, 120, 40)
	m.w.Player.DirtyCash, m.w.Player.CleanCash, m.w.Offshore = 0, 0, 226_000
	got := m.alertsOf(engine.AlertWages)
	if len(got) != 1 {
		t.Fatalf("no wages alert with an empty till: %+v", got)
	}
	if text := stripANSI(got[0].text); !strings.Contains(text, "The $226,000 offshore does not count: nothing comes back from it.") {
		t.Errorf("the wages alert: %q", text)
	}
}
