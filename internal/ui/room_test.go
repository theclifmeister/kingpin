package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// fileAndHeat puts the file a page from an indictment and the heat
// where you stand over the sting line: a danger and a heat line on one
// morning.
func fileAndHeat(m *Model) {
	w := m.w
	w.Heat.Evidence = m.rules.Heat.EvidenceArrest(w) - 1
	w.Here().Heat = 60
}

// The file outranks the heat (#534): at 80x24, where ALERTS has room
// for a line, the file alert is the one shown, with the cursor on it,
// and the title counts the alerts left out and says space shows them.
func TestFileAlertOutranksHeatAt80x24(t *testing.T) {
	m := richModel(t, 80, 24)
	fileAndHeat(m)
	m.alertCursor = 0
	m.Update(key("1"))
	as := m.sess.Alerts()
	if len(as) < 2 || as[0].Kind != engine.AlertFile && !as[0].Danger() {
		t.Fatalf("the loudest alert is %+v", as)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "▸ File ") {
		t.Errorf("80x24 does not show the file alert:\n%s", view)
	}
	if want := fmt.Sprintf("ALERTS · +%d more (space)", len(as)-1); !strings.Contains(view, want) {
		t.Errorf("80x24 does not count the hidden alerts as %q:\n%s", want, view)
	}
	// space shows every one of them whole.
	m.Update(key(" "))
	prose := scrolledProse(t, m)
	for _, a := range m.alerts() {
		if first := strings.Fields(stripANSI(a.text)); len(first) > 0 && !strings.Contains(prose, strings.Join(first[:min(4, len(first))], " ")) {
			t.Errorf("the overlay lacks %q:\n%s", stripANSI(a.text), prose)
		}
	}
}

// ALERTS keeps its loudest line under a long CART (#534: at 100x30 the
// cart filled the pane and the alerts fell below it as "…"), the rest
// counted in its title, at every size the pane is drawn.
func TestAlertsKeepALineUnderTheCart(t *testing.T) {
	for _, sz := range [][2]int{{100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		fillCart(t, m)
		fileAndHeat(m)
		m.alertCursor = 0
		m.Update(key("1"))
		pane := paneRender(m)
		if !strings.Contains(pane, "CART") {
			t.Fatalf("%dx%d: no cart in the pane:\n%s", sz[0], sz[1], pane)
		}
		if !strings.Contains(pane, "ALERTS") || !strings.Contains(pane, "▸ File ") {
			t.Errorf("%dx%d: the pane lost the loudest alert under the cart:\n%s", sz[0], sz[1], pane)
		}
		if strings.Contains(pane, "ALERTS · +") && !strings.Contains(pane, "more (space)") {
			t.Errorf("%dx%d: the hidden alerts are not counted:\n%s", sz[0], sz[1], pane)
		}
	}
}

// The DA race has its section where its alert opens (#534): the
// ledger's DA RACE, the cursor on the city's row, the pane saying who
// sits, the odds as the count reads them, the day and the price.
func TestDARaceOpensItsSection(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Law.CampaignOpen = true
	w.Player.CleanCash = 50_000
	next := m.rules.Law.NextElection(w)
	if next <= w.Day {
		w.Law.DA.ElectedDay = w.Day + 20 - m.rules.Law.Tuning().TermDays
		next = m.rules.Law.NextElection(w)
	}
	var race *engine.Alert
	for _, a := range m.sess.Alerts() {
		if a.Kind == engine.AlertDARace {
			race = &a
		}
	}
	if race == nil {
		t.Fatalf("no DA race alert with the window open: %+v", m.sess.Alerts())
	}
	m.openAlert(*race)
	if m.screen != screenLedger || m.ledgerSelected().kind != ledgerRace || m.raceCity(m.ledgerSelected().i) != race.City {
		t.Fatalf("the alert opened screen %v on %+v", m.screen, m.ledgerSelected())
	}
	lo, reform, moderate := m.rules.Law.Odds(w)
	if d := lo + reform + moderate - 1; d > 1e-9 || d < -1e-9 {
		t.Errorf("the odds sum to %v", lo+reform+moderate)
	}
	prose := paneProse(m)
	for _, want := range []string{"DA RACE · " + strings.ToUpper(w.CityName(race.City)), fmt.Sprintf("day %d", next), "DA " + w.Law.DA.Name,
		"law-and-order", "reform", "a moderate", "a point", "Clean cash, spent at the count", "back a ticket"} {
		if !strings.Contains(prose, want) {
			t.Errorf("the race's section lacks %q:\n%s", want, prose)
		}
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "DA RACE · the vote on day") {
		t.Errorf("the ledger lacks the DA RACE block:\n%s", view)
	}
}

// The DA race's result has its line in the report (#534), and WORDS
// names the race.
func TestDARaceIsNamedInWords(t *testing.T) {
	m := newTestModel(t, 80, 24)
	if !strings.Contains(stripANSI(strings.Join(m.helpLines(), "\n")), "DA race") {
		t.Error("WORDS does not name the DA race")
	}
}

// d deliver is on the market's KEYS on a contract at every size (#535:
// at 100x30 the box ended before it and a handoff was never made).
func TestDeliverKeyAtEverySize(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := newTestModel(t, sz[0], sz[1])
		w := m.w
		w.SetStock(w.Player.Location, w.Products[0], 60)
		offer(m, 40, w.Player.Location)
		offer(m, 25, w.Player.Location)
		m.Update(key("2"))
		toBuyers(t, m)
		m.Update(key("a"))
		what := fmt.Sprintf("%dx%d market on a contract", sz[0], sz[1])
		if m.paneShown() {
			assertKeysShown(t, m, paneRender(m), what)
		}
		m.Update(key(" "))
		assertKeysShown(t, m, scrolledProse(t, m), what+", the overlay")
		if !strings.Contains(spaces.ReplaceAllString(scrolledProse(t, m), " "), "d deliver") {
			t.Errorf("%s: no d deliver", what)
		}
	}
}

// A status message too long for the bar says `␣ more`, and space opens
// it whole at the head of the overlay (#535: "… P…" hid a campaign's
// confirmation). The next key forgets it.
func TestCutStatusOpensWhole(t *testing.T) {
	for _, width := range []int{80, 100, 120} {
		m := newTestModel(t, width, 30)
		msg := strings.Repeat("Gave Eastside $100,000 clean for goodwill and the campaign. ", 3)
		m.say(msg)
		rows := strings.Split(stripANSI(m.View()), "\n")
		if bar := rows[len(rows)-1]; !strings.HasSuffix(strings.TrimRight(bar, " "), "␣ more") {
			t.Errorf("%d columns: the cut bar does not say ␣ more: %q", width, bar)
		}
		m.Update(key(" "))
		if m.mode != modeDetails {
			t.Fatalf("%d columns: space opened mode %v", width, m.mode)
		}
		if prose := spaces.ReplaceAllString(scrolledProse(t, m), " "); !strings.Contains(prose, "STATUS "+strings.TrimSpace(msg)) {
			t.Errorf("%d columns: the overlay does not lead with the message whole:\n%s", width, prose)
		}
		m.Update(key("esc"))
		m.Update(key(" "))
		if prose := scrolledProse(t, m); strings.Contains(prose, "STATUS") {
			t.Errorf("%d columns: the message outlived its key:\n%s", width, prose)
		}
		m.Update(key("esc"))
	}
}

// A picker that cuts the name under its cursor names it whole under
// the table (#535: "Construction …" in BUY A FRONT at 80x24).
func TestPickerNamesTheCutRow(t *testing.T) {
	m := richModel(t, 80, 24)
	m.Update(key("7"))
	m.askFront()
	m.Update(key("enter")) // the kind: a front
	if m.mode != modeFront {
		t.Fatalf("mode %v", m.mode)
	}
	for i, o := range m.frontRows() {
		m.front.cursor = i
		if view := stripANSI(m.View()); !strings.Contains(view, o.Name) {
			t.Errorf("row %d: %q is not whole in BUY A FRONT:\n%s", i, o.Name, view)
		}
	}
}

// The dashboard's war line keeps its warning at every size (#535: at
// 100x30 it read "no enforcer at work, so nobody goes in …").
func TestWarLineKeepsItsWarning(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		w := m.w
		var kept []game.CrewMember
		for _, c := range w.Crew.Members {
			if c.Role != game.RoleEnforcer {
				kept = append(kept, c)
			}
		}
		w.Crew.Members = kept
		w.War = w.Rival().Faction()
		m.Update(key("1"))
		main := mainText(m)
		if !strings.Contains(main, "War on") || !strings.Contains(main, "nobody goes in tonight") {
			t.Errorf("%dx%d: the war line lost its warning:\n%s", sz[0], sz[1], main)
		}
	}
}

// The ledger's pile, money and road lines wrap rather than cut (#535:
// "rats and damp take ~$98…" could not be opened).
func TestLedgerPileWraps(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := lateModel(t, sz[0], sz[1])
		m.Update(key("7"))
		pile := stripANSI(m.pileLine())
		if pile == "" {
			t.Fatalf("%dx%d: the late fixture has no pile line", sz[0], sz[1])
		}
		main := spaces.ReplaceAllString(strings.Join(strings.Fields(mainText(m)), " "), " ")
		if !strings.Contains(main, strings.Join(strings.Fields(pile), " ")) {
			t.Errorf("%dx%d: the pile line is not whole on the ledger:\n%s", sz[0], sz[1], mainText(m))
		}
	}
}
