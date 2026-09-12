package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The books (#70) in the grammar: i on the rivals screen scouts and $
// buys off, each after a confirmation with the numbers the dice use;
// the strike picker's boost rows confirm and queue the night's one
// strike order with the boost flag; t on the map tips the police on the
// rival corner under the cursor; i, $ and t elsewhere are pointed at
// every screen that takes them; the BOOKS block reads ? until a scout
// reads the books and then the snapshot with its age; and each modal
// fits.
func TestBooksKeys(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	// i and $ on the dashboard point at both screens that take them.
	m.Update(key("i"))
	if m.mode != modePlay || m.status != "Investigate on the crew screen (4). Scout on the rivals screen (8)." {
		t.Fatalf("i on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("$"))
	if m.mode != modePlay || m.status != "Pay off on the crew screen (4). Buy off on the rivals screen (8)." {
		t.Fatalf("$ on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("t"))
	if m.mode != modePlay || m.status != "Assign on the crew screen (4). Tip police on the map screen (5)." {
		t.Fatalf("t on the dashboard: mode %v status %q", m.mode, m.status)
	}
	// The scout: the confirmation names the cost and the odds; y queues it,
	// and a second i is refused.
	m.Update(key("8"))
	main := mainText(m)
	for _, want := range []string{"BOOKS · never read", "cash    ?", "muscle  ?", "police none"} {
		if !strings.Contains(main, want) {
			t.Errorf("MAIN lacks %q:\n%s", want, main)
		}
	}
	m.Update(key("i"))
	if m.mode != modeConfirmScout {
		t.Fatalf("i on the rivals screen: mode %v status %q", m.mode, m.status)
	}
	view := stripANSI(m.View())
	for _, want := range []string{"SCOUT THEIR BOOKS?", "$2,000", "~67%", "y scout"} {
		if !strings.Contains(view, want) {
			t.Errorf("the scout confirmation lacks %q:\n%s", want, view)
		}
	}
	cash := w.Cash()
	m.Update(key("y"))
	if m.mode != modePlay || w.Today.Scouting == nil || w.Today.Scouting.Cost != m.set.Rivals.ScoutCost() || w.Cash() != cash-m.set.Rivals.ScoutCost() {
		t.Fatalf("y on the scout: mode %v scouting %+v cash %d -> %d", m.mode, w.Today.Scouting, cash, w.Cash())
	}
	m.Update(key("i"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't scout twice") {
		t.Fatalf("a second i: mode %v status %q", m.mode, m.status)
	}
	// The buy-off: blind, a head at a time; enter pays, the order is
	// queued and the cash is gone.
	m.Update(key("$"))
	if m.mode != modeConfirmBuyOff || m.bo.units.max != 1 {
		t.Fatalf("$ on the rivals screen: mode %v max %d", m.mode, m.bo.units.max)
	}
	view = stripANSI(m.View())
	for _, want := range []string{"BUY OFF THEIR MUSCLE?", "buy blind", "/ 1 max", "y enter pay"} {
		if !strings.Contains(view, want) {
			t.Errorf("the buy-off confirmation lacks %q:\n%s", want, view)
		}
	}
	price := m.set.Rivals.MusclePrice(w)
	dirty := w.Player.DirtyCash
	m.Update(key("enter"))
	if m.mode != modePlay || w.Today.Poach == nil || w.Today.Poach.Units != 1 || w.Today.Poach.Cost != price || w.Player.DirtyCash != dirty-price {
		t.Fatalf("enter on the buy-off: mode %v poach %+v dirty %d -> %d", m.mode, w.Today.Poach, dirty, w.Player.DirtyCash)
	}
	m.Update(key("$"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't buy off twice") {
		t.Fatalf("a second $: mode %v status %q", m.mode, m.status)
	}
	// With the books read the field's max is the muscle as read and the
	// block carries the snapshot with its age.
	w.Today.Poach = nil
	w.Rival.Known = game.Known{Day: w.Day - 3, Cash: 48_000, Income: 12_000, Muscle: 5, Wages: 9_000}
	main = mainText(m)
	for _, want := range []string{"BOOKS · read 3 days ago", "cash    $48K", "muscle  5 heads", "3d"} {
		if !strings.Contains(main, want) {
			t.Errorf("MAIN lacks %q:\n%s", want, main)
		}
	}
	m.Update(key("$"))
	if m.mode != modeConfirmBuyOff || m.bo.units.max != 5 {
		t.Fatalf("$ with the books read: mode %v max %d", m.mode, m.bo.units.max)
	}
	m.Update(key("3"))
	m.Update(key("y"))
	if w.Today.Poach == nil || w.Today.Poach.Units != 3 || w.Today.Poach.Cost != 3*price {
		t.Fatalf("three heads: %+v", w.Today.Poach)
	}
	day := w.Day
	w.Day = w.Rival.Known.Day + m.set.Rivals.Books().StaleDays // the fixture is on day 4: stale is read on a later morning
	if main := mainText(m); !strings.Contains(main, "stale") {
		t.Errorf("MAIN does not say the books are stale:\n%s", main)
	}
	w.Day = day
	// The boost: the picker's fourth row is warn for the till; y queues
	// the strike order with the boost flag and the inspector carries it.
	m.Update(key("5"))
	m.mapCursor = 0
	m.Update(key("w"))
	m.Update(key("4"))
	if m.mode != modeConfirmBoost {
		t.Fatalf("the fourth picker row: mode %v status %q", m.mode, m.status)
	}
	view = stripANSI(m.View())
	for _, want := range []string{"BOOST?", "at warn for the till", "not the corner", "y boost"} {
		if !strings.Contains(view, want) {
			t.Errorf("the boost confirmation lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("y"))
	if m.mode != modePlay || w.Today.Strike == nil || !w.Today.Strike.Boost || w.Today.Strike.Force != events.ForceWarn || w.Today.Strike.Corner != w.Home().Corners[0].ID {
		t.Fatalf("y on the boost: mode %v strike %+v status %q", m.mode, w.Today.Strike, m.status)
	}
	if pane := paneRender(m); !strings.Contains(pane, "boost: the till, ~") || !strings.Contains(pane, "t  tip the police") {
		t.Errorf("the inspector lacks the boost and the tip rows:\n%s", pane)
	}
	// The tip: t confirms, y queues it, a second t is refused, and the
	// inspector reads it.
	m.Update(key("t"))
	if m.mode != modeConfirmTip {
		t.Fatalf("t on the rival corner: mode %v status %q", m.mode, m.status)
	}
	view = stripANSI(m.View())
	for _, want := range []string{"TIP THE POLICE?", "Free.", "0 → 15", "y tip"} {
		if !strings.Contains(view, want) {
			t.Errorf("the tip confirmation lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("y"))
	if m.mode != modePlay || w.Today.Tipoff == nil || w.Today.Tipoff.Corner != w.Home().Corners[0].ID {
		t.Fatalf("y on the tip: mode %v tip %+v status %q", m.mode, w.Today.Tipoff, m.status)
	}
	m.Update(key("t"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't tip twice") {
		t.Fatalf("a second t: mode %v status %q", m.mode, m.status)
	}
	if pane := paneRender(m); !strings.Contains(pane, "tipped tonight") {
		t.Errorf("the inspector does not say the corner is tipped:\n%s", pane)
	}
	// On your own corner t is refused.
	m.mapCursor = m.yourCorner()
	m.Update(key("t"))
	if m.mode != modePlay || !strings.HasPrefix(m.status, "Can't tip the police there") {
		t.Fatalf("t on your corner: mode %v status %q", m.mode, m.status)
	}
	// The night resolves every move and the report carries them.
	w.Today.Tipoff = nil
	w.Today.Poach = nil
	endDay(t, m)
	rep := strings.Join(w.Report.Territory, "\n")
	if !strings.Contains(rep, "books") && !strings.Contains(rep, "scout") {
		t.Errorf("the report does not carry the scout: %v", w.Report.Territory)
	}
	if !strings.Contains(rep, "till") && !strings.Contains(rep, "takings") {
		t.Errorf("the report does not carry the boost: %v", w.Report.Territory)
	}
	if w.Stats.Scouts != 1 || w.Stats.Boosts != 1 {
		t.Fatalf("stats %+v", w.Stats)
	}
}

// Fast-forward stops on a raid the police make on your tip and on a
// boost that failed (#70).
func TestFastForwardStopsOnTheBooks(t *testing.T) {
	m := richModel(t, 80, 24)
	if why := m.stopEvent(events.RivalRaided{Day: 1, Name: "The Docks", Rival: "Dutch"}); why != "the police raided The Docks" {
		t.Fatalf("a raid: %q", why)
	}
	if why := m.stopEvent(events.RivalBoosted{Day: 1, Name: "The Docks", Taken: false}); why != "the boost on The Docks failed" {
		t.Fatalf("a failed boost: %q", why)
	}
	if why := m.stopEvent(events.RivalBoosted{Day: 1, Name: "The Docks", Taken: true}); why != "" {
		t.Fatalf("a boost that landed stops: %q", why)
	}
}
