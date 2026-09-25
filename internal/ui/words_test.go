package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The summary counts what happened (#465): a member who took a
// faction's offer is a betrayal ("betrayals none" after a runner went
// over), and the ground's lost is every corner that left you, which the
// sims count (TestIdleCornersDrift, TestTipsAndCrackdown).
func TestSummaryCountsTheBetrayalsAndTheGround(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Stats = game.Stats{CrewPoached: 1, CornersWon: 7, CornersLost: 6}
	if got := m.betrayalsLine(); got != "1 member poached" {
		t.Errorf("betrayals: %q", got)
	}
	w.Over = w.End(content.CauseIndicted, w.Day, "")
	m.mode = modeOver
	view := stripANSI(strings.Join(m.summaryLines(), "\n"))
	for _, want := range []string{"1 member poached", "7 won · 6 lost"} {
		if !strings.Contains(view, want) {
			t.Errorf("the summary lacks %q:\n%s", want, view)
		}
	}
}

// THE STORY tells a text once and the whole run (#465): five identical
// patrol lines were the story, and a 563-day run's covered days 524 to
// 551 alone.
func TestStoryTellsTheWholeRunOnce(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Journal = []game.Headline{
		{Day: 12, Source: "crew", Text: "Lost Bird (arrested)."},
		{Day: 200, Source: "rivals", Text: "Dutch's crew moves in on Eastside"},
		{Day: 300, Source: "territory", Text: "You took The Docks."},
	}
	for d := 520; d <= 560; d += 4 {
		w.Journal = append(w.Journal, game.Headline{Day: d, Source: "heat", Text: "Extra patrols hit Eastside after complaints"})
	}
	w.Over = w.End(content.CauseIndicted, 563, "")
	lines := stripANSI(strings.Join(m.storyLines(), "\n"))
	if n := strings.Count(lines, "Extra patrols"); n != 1 {
		t.Errorf("the patrol line told %d times:\n%s", n, lines)
	}
	for _, want := range []string{"day 12", "Lost Bird", "Dutch's crew", "The Docks"} {
		if !strings.Contains(lines, want) {
			t.Errorf("the story lacks %q:\n%s", want, lines)
		}
	}
}

// The plan's line shows its parts (#465): "Retire clean 50%" with $0 of
// $750,000 was the mean of the account and the quiet days, and when the
// quiet days go back to zero it says what reset them.
func TestPlanShowsItsParts(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	if err := m.sess.PinAmbition(content.AmbitionRetire); err != nil {
		t.Fatal(err)
	}
	w.Offshore, w.QuietDays = 0, 14
	line := stripANSI(m.planFact())
	if !strings.Contains(line, "the account 0%") || !strings.Contains(line, "quiet days 14/14") || strings.Contains(line, "50%") {
		t.Errorf("the plan: %q", line)
	}
	w.QuietDays = 0
	m.dayEnded([]events.Event{events.QuietBroken{Day: w.Day, Days: 14, Cause: events.QuietPolice, City: w.Home().ID, Level: content.Sting}})
	want := fmt.Sprintf("reset by a sting in %s on day %d", w.Home().Name, w.Day)
	if line := stripANSI(m.planFact()); !strings.Contains(line, "quiet days 0/14") || !strings.Contains(line, want) {
		t.Errorf("the plan after a sting: %q, want %q", line, want)
	}
	if rep := stripANSI(strings.Join(m.planReport(), "\n")); !strings.Contains(rep, want) {
		t.Errorf("the PLAN line after a sting: %q", rep)
	}
}

// A faction that is gone reads as its word on the dashboard (#465:
// "Ivory · 0 corners · opportunist" after Ivory was absorbed), and the
// fast-forward's stop for the first money offshore does not read as
// the run over ("Stopped after 1 day: retirement.").
func TestGoneRivalAndRetireStopSayWhatHappened(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	r := w.Rival()
	r.Arrived, r.Absorbed, r.AbsorbedBy = 1, w.Day, ""
	for i := range w.Home().Corners {
		if c := &w.Home().Corners[i]; c.Owner == game.OwnerRival && c.Faction == r.Faction() {
			c.Owner, c.Faction = game.OwnerNone, ""
		}
	}
	lines := stripANSI(strings.Join(m.rivalLines(60), "\n"))
	if !strings.Contains(lines, r.Leader+" · scattered") || strings.Contains(lines, "0 corners") {
		t.Errorf("the RIVALS panel on a scattered crew:\n%s", lines)
	}
	why := m.alertOf(engine.Alert{Kind: engine.AlertRetire, Key: "retirement"}).why
	if why == "retirement" || !strings.Contains(why, "offshore account") {
		t.Errorf("the stop reads %q", why)
	}
}
