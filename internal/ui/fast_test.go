package ui

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// fast presses F and runs up to days days.
func fast(t *testing.T, m *Model, days int) {
	t.Helper()
	m.Update(key("F"))
	if m.mode != modeConfirmFast {
		t.Fatalf("F: mode %v, status %q", m.mode, m.status)
	}
	m.fst.days.SetValue(fmt.Sprint(days))
	m.Update(key("enter"))
}

// closeMorning answers a card with its first choice and closes the
// outcome and the report, so the next key lands on the play screen.
func closeMorning(t *testing.T, m *Model) {
	t.Helper()
	if m.mode == modeStage {
		m.Update(key("enter"))
	}
	if m.mode == modeCard {
		m.Update(key("enter"))
		m.Update(key("enter"))
	}
	if m.mode == modeReport {
		m.Update(key("enter"))
	}
	if m.mode != modePlay {
		t.Fatalf("after the morning: mode %v", m.mode)
	}
}

// reportLine is the first line of the report modal's body.
func reportLine(t *testing.T, m *Model) string {
	t.Helper()
	if m.mode != modeReport {
		t.Fatalf("mode %v, not the report: status %q", m.mode, m.status)
	}
	_, _, box := modalBox(t, m.View())
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(stripANSI(box[3])), "║"))
}

// A fast-forward stops on the day a card is dealt, before the report,
// with the card up (a stop never skips a card): the day the n walk
// deals the first card on the rich fixture's seed is the day F stops
// on, whatever else it stopped for on the way, and the report after
// the card names the card.
func TestFastForwardStopsOnACard(t *testing.T) {
	const seed = 4
	byHand := richModelSeeded(t, 80, 24, seed)
	cardDay := 0
	for byHand.w.Day < 40 {
		byHand.Update(key("n"))
		if byHand.mode == modeCard {
			cardDay = byHand.w.Day
			break
		}
		closeMorning(t, byHand)
	}
	if cardDay == 0 {
		t.Fatalf("seed %d dealt no card by day 40", seed)
	}
	m := richModelSeeded(t, 80, 24, seed)
	for m.w.Day < cardDay {
		fast(t, m, 30)
		if m.w.Day > cardDay {
			t.Fatalf("F ran past the card: day %d, the card was dealt on day %d", m.w.Day, cardDay)
		}
		if m.w.Day < cardDay {
			if m.mode == modeCard {
				t.Fatalf("a card on day %d that the n walk did not deal", m.w.Day)
			}
			closeMorning(t, m)
		}
	}
	if m.mode != modeCard || m.w.Dilemmas.Pending == nil {
		t.Fatalf("on the card's day: mode %v pending %v stop %q", m.mode, m.w.Dilemmas.Pending, m.fastStop)
	}
	if !strings.HasSuffix(m.fastStop, ": a card to answer.") {
		t.Fatalf("stop line %q", m.fastStop)
	}
	assertFits(t, m.View(), 80, 24, "card after F")
	m.Update(key("enter")) // decide
	m.Update(key("enter")) // the outcome, then the report
	if got := reportLine(t, m); got != m.fastStop {
		t.Fatalf("the report opens with %q, not %q", got, m.fastStop)
	}
	assertFits(t, m.View(), 80, 24, "report after the card")
}

// A fast-forward stops on the morning an alert is new: heat crossing
// the patrol line stops it once and not again while it stays over, and
// an accepted contract stops it the morning it is due tomorrow and again
// the morning it is due today, the report opening with the reason and
// the status bar saying how many days ran.
func TestFastForwardStopsOnAnAlert(t *testing.T) {
	m := richModelSeeded(t, 80, 24, 1)
	patrol := 0.0
	for _, r := range m.set.Heat.ThresholdsIn(m.w, m.w.Here()) {
		if r.Level == content.Patrol {
			patrol = r.Threshold
		}
	}
	m.w.Here().Heat = patrol - 1 // the fixture's orders take it over tonight
	day := m.w.Day
	fast(t, m, 30)
	if m.w.Day != day+1 || m.w.Here().Heat < patrol {
		t.Fatalf("day %d -> %d, heat %.0f against the line %.0f", day, m.w.Day, m.w.Here().Heat, patrol)
	}
	want := fmt.Sprintf("Stopped after 1 day: heat in %s over the patrol line.", m.w.Here().Name)
	if got := reportLine(t, m); got != want {
		t.Fatalf("the report opens with %q, want %q", got, want)
	}
	if m.status != "Ran 1 day." {
		t.Fatalf("status %q", m.status)
	}
	if len(m.alerts()) == 0 || !strings.Contains(stripANSI(m.alerts()[0].text), "over the patrol line") {
		t.Fatalf("the dashboard's alerts do not carry the heat: %+v", m.alerts())
	}
	closeMorning(t, m)
	// Not new the next morning: F stops for something else.
	m.w.Here().Heat = patrol + 20
	fast(t, m, 30)
	if strings.Contains(m.fastStop, "patrol line") {
		t.Fatalf("stopped again for the heat that was already over the line: %q", m.fastStop)
	}
	closeMorning(t, m)

	// A contract: the fixture's seed brings an offer on the first day;
	// accepted, it stops the run the morning it is due tomorrow and the
	// morning it is due today, and never for the same morning twice.
	m = richModelSeeded(t, 80, 24, 1)
	fast(t, m, 30)
	var c *game.Contract
	for i := range m.w.Contracts {
		if m.w.Contracts[i].Status == game.ContractOffered {
			c = &m.w.Contracts[i]
		}
	}
	if c == nil || !strings.HasSuffix(m.fastStop, "is asking.") {
		t.Fatalf("no offer after the first F: stop %q contracts %+v", m.fastStop, m.w.Contracts)
	}
	if c.Due < m.w.Day+2 {
		t.Fatalf("the offer is due on day %d, too soon for a tomorrow on day %d", c.Due, m.w.Day)
	}
	due := c.Due
	closeMorning(t, m)
	if err := m.w.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		day    int
		reason string
	}{{due - 1, "contract due tomorrow"}, {due, "contract due today"}} {
		for {
			fast(t, m, 30)
			if strings.Contains(m.fastStop, "contract due") {
				break
			}
			if m.w.Day >= want.day {
				t.Fatalf("day %d: stopped for %q, not the contract due on day %d", m.w.Day, m.fastStop, due)
			}
			closeMorning(t, m)
		}
		if m.w.Day != want.day || !strings.HasSuffix(m.fastStop, ": "+want.reason+".") {
			t.Fatalf("day %d stop %q, want day %d %q", m.w.Day, m.fastStop, want.day, want.reason)
		}
		if got := reportLine(t, m); got != m.fastStop {
			t.Fatalf("the report opens with %q, not %q", got, m.fastStop)
		}
		closeMorning(t, m)
	}
	fast(t, m, 30)
	if strings.Contains(m.fastStop, "contract due") {
		t.Fatalf("stopped for the contract past its due day: %q", m.fastStop)
	}
}

// A fast-forward that meets nothing stops at the cap, its report
// opening with the cap as the reason, r reopening it with the line, the
// journal carrying every day's headlines and the unread count grown by
// them (journalSeen untouched); the line goes with the next day.
func TestFastForwardStopsAtTheCap(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.startRun(5) // a seed whose first week is quiet
	m.Update(key("3"))
	m.Update(key("1"))
	seen, headlines := m.journalSeen, len(m.w.Journal)
	fast(t, m, 3)
	if m.w.Day != 3 || m.mode != modeReport {
		t.Fatalf("day %d mode %v stop %q", m.w.Day, m.mode, m.fastStop)
	}
	if got := reportLine(t, m); got != "Stopped after 3 days: the cap." {
		t.Fatalf("the report opens with %q", got)
	}
	if m.status != "Ran 3 days." {
		t.Fatalf("status %q", m.status)
	}
	assertFits(t, m.View(), 80, 24, "report at the cap")
	m.Update(key("enter"))
	if m.journalSeen != seen || len(m.w.Journal) <= headlines || m.journalUnread() != len(m.w.Journal)-seen {
		t.Fatalf("journal: seen %d -> %d, %d -> %d headlines, %d unread", seen, m.journalSeen, headlines, len(m.w.Journal), m.journalUnread())
	}
	// Every day's headlines are there, not the last morning's alone.
	days := map[int]bool{}
	for _, h := range m.w.Journal[headlines:] {
		days[h.Day] = true
	}
	if len(days) < 2 || !days[1] {
		t.Errorf("the journal carries days %v of the three: %+v", days, m.w.Journal[headlines:])
	}
	m.Update(key("r"))
	if got := reportLine(t, m); got != "Stopped after 3 days: the cap." {
		t.Fatalf("r reopens the report with %q", got)
	}
	m.Update(key("enter"))
	m.Update(key("n"))
	if got := reportLine(t, m); strings.HasPrefix(got, "Stopped") {
		t.Fatalf("the next day's report still opens with %q", got)
	}
	// A blank cap is the default, and the confirmation refuses a cap it
	// cannot run.
	m.Update(key("enter"))
	m.Update(key("F"))
	if n, err := m.fastCap(); n != fastDays || err != nil {
		t.Fatalf("a blank cap reads %d, %v", n, err)
	}
	m.fst.days.SetValue(fmt.Sprint(fastDaysMax + 1))
	m.Update(key("enter"))
	if m.mode != modeConfirmFast || m.fst.err == "" {
		t.Fatalf("%d days: mode %v err %q", fastDaysMax+1, m.mode, m.fst.err)
	}
	m.Update(key("esc"))
	if m.mode != modePlay {
		t.Fatalf("esc: mode %v", m.mode)
	}
}

// Days run by F are the days n ends: F for five quiet days and n five
// times leave the same world, in memory and through a save (gob writes
// a map in whatever order it walks it, so the two are compared decoded,
// not as bytes).
func TestFastForwardIsTheSameDays(t *testing.T) {
	roundTrip := func(w *game.World) *game.World {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(w); err != nil {
			t.Fatal(err)
		}
		var out game.World
		if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	byHand := newTestModel(t, 80, 24)
	byHand.startRun(5)
	for i := 0; i < 5; i++ {
		byHand.Update(key("n"))
		if byHand.mode != modeReport {
			t.Fatalf("n on day %d: mode %v", byHand.w.Day, byHand.mode)
		}
		byHand.Update(key("enter"))
	}
	m := newTestModel(t, 80, 24)
	m.startRun(5)
	fast(t, m, 5)
	if m.w.Day != 5 || m.status != "Ran 5 days." {
		t.Fatalf("F: day %d status %q stop %q", m.w.Day, m.status, m.fastStop)
	}
	if !reflect.DeepEqual(m.w, byHand.w) {
		t.Fatal("F for 5 days and n five times differ")
	}
	if !reflect.DeepEqual(roundTrip(m.w), roundTrip(byHand.w)) {
		t.Fatal("F for 5 days and n five times differ through gob")
	}
	saved, err := game.Load(m.slot, m.set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, roundTrip(byHand.w)) {
		t.Fatal("the save after F differs from the run by hand")
	}
}
