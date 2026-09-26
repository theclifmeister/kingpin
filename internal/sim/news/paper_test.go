package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// paper is a news sim and a fresh world for the #523 tests.
func paper(t *testing.T, seed uint64) (*news.Sim, *game.World) {
	t.Helper()
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return n, sim.NewWorld(cfg, seed)
}

// headlines is the journal's lines for day, bar the digest's.
func headlines(w *game.World, day int) []string {
	var out []string
	for _, h := range w.Journal {
		if h.Day == day && h.Source != "digest" {
			out = append(out, h.Text)
		}
	}
	return out
}

// The paper prints a headline once a day (#523: "Heroin demand craters
// after overdose scare" twice on day 120).
func TestNoHeadlineTwiceADay(t *testing.T) {
	n, w := paper(t, 3)
	home := w.Home().ID
	for d := 1; d <= 5; d++ {
		n.Step(w, gametest.TickOn(w, d, events.Enforcement{Day: d, City: home, Level: content.Arrest}, events.Enforcement{Day: d, City: home, Level: content.Arrest}))
		w.Day = d
		seen := map[string]bool{}
		for _, h := range headlines(w, d) {
			if seen[h] {
				t.Fatalf("day %d printed %q twice", d, h)
			}
			seen[h] = true
		}
	}
}

// A raid names a stash house only when it hit one (#523: "SWAT storms
// Eastside stash house" with no house rented).
func TestRaidNamesAHouseOnlyWhenItHitOne(t *testing.T) {
	n, w := paper(t, 3)
	home := w.Home().ID
	named := false
	for d := 1; d <= 60; d++ {
		ev := events.Enforcement{Day: d, City: home, Level: content.Raid}
		if d%2 == 0 {
			ev.House, ev.HouseName = "lockup", "Lock-up at the Docks"
		}
		n.Step(w, gametest.TickOn(w, d, ev))
		w.Day = d
		for _, h := range headlines(w, d) {
			if strings.Contains(h, "stash house") || (ev.House == "" && strings.Contains(h, "SWAT")) {
				t.Fatalf("day %d, house %q: %q", d, ev.House, h)
			}
			if strings.Contains(h, "SWAT storms the Lock-up at the Docks") {
				named = true
			}
		}
	}
	if !named {
		t.Fatal("no raid on the house named it in 30 nights")
	}
}

// The ticker's filler is for quiet days (#523: "Weather: unseasonably
// warm" beside "Whiteout: Coast Road impassable"): on a night an
// incident lands the flavour is left out, and the home stream is drawn
// as it was.
func TestNoFillerOnAnIncidentNight(t *testing.T) {
	cfg := content.MustLoad()
	flavour := map[string]bool{}
	for _, f := range cfg.Headlines.Flavour {
		if !strings.Contains(f, "{{") {
			flavour[f] = true
		}
	}
	quiet, busy := 0, 0
	n, w := paper(t, 5)
	for d := 1; d <= 200; d++ {
		var evs []events.Event
		if d%2 == 0 {
			evs = append(evs, events.Incident{Day: d, ID: "snowstorm", Name: "Snowstorm", City: w.Home().ID})
		}
		tk := gametest.TickOn(w, d, evs...)
		n.Step(w, tk)
		w.Day = d
		for _, h := range headlines(w, d) {
			if flavour[h] {
				if d%2 == 0 {
					busy++
				} else {
					quiet++
				}
			}
		}
	}
	if busy > 0 || quiet == 0 {
		t.Fatalf("filler on %d incident nights and %d quiet ones; want none and some", busy, quiet)
	}
}

// A crew headline names the city the member works (#523: "Old hands on
// Bayport's corners: Petra" for a runner on an Eastside corner, the
// player standing in Bayport).
func TestCrewHeadlinesUseTheMembersCity(t *testing.T) {
	n, w := paper(t, 3)
	var away *game.City
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c != nil && c.ID != w.Here().ID && len(c.Corners) > 0 {
			away = c
			break
		}
	}
	if away == nil {
		t.Skip("one city")
	}
	w.Crew.Members = []game.CrewMember{{ID: 7, Name: "Petra", Role: game.RoleRunner, Loyalty: 80}}
	away.Corners[0].Owner, away.Corners[0].Runner = game.OwnerPlayer, 7
	for d := 1; d <= 10; d++ {
		n.Step(w, gametest.TickOn(w, d, events.CrewTrait{Day: d, ID: 7, Name: "Petra", Role: game.RoleRunner, Trait: "steady", Days: 40, Good: true},
			events.CrewQuit{Day: d, ID: 9, Name: "Gone", Role: game.RoleRunner, City: away.ID}))
		w.Day = d
		for _, h := range headlines(w, d) {
			if (strings.Contains(h, "Petra") || strings.Contains(h, "Gone") || strings.Contains(h, "walks")) && strings.Contains(h, w.Here().Name) && !strings.Contains(h, away.Name) {
				t.Fatalf("day %d: %q names where you stand, not %s", d, h, away.Name)
			}
		}
	}
}

// The HEAT lines are the pages in the order they landed, each "+N (now
// M/T)" (#522: the sting's "grows (5)" read above the offshore page's
// "grows (3)", and both read as the pages filed).
func TestPagesReadInTheOrderTheyLanded(t *testing.T) {
	n, w := paper(t, 3)
	home := w.Home().ID
	// The heat sim's order: the police fire before the night settles.
	n.Step(w, gametest.TickOn(w, 1,
		events.Enforcement{Day: 1, City: home, Level: content.Sting, Evidence: 2, File: 5, Arrest: 6},
		events.HeatChanged{Day: 1, City: home, From: 30, To: 20, Reasons: []string{"money moved offshore in lumps: the DA's file grows +1 (now 3/6)"}},
		events.FileChanged{Day: 1, From: 2, To: 5, Arrest: 6}))
	heat := strings.Join(w.Report.Heat, "\n")
	lumps, sting := strings.Index(heat, "in lumps"), strings.Index(heat, "the DA's file on you grows +2 (now 5/6)")
	if lumps < 0 || sting < 0 || sting < lumps {
		t.Fatalf("the pages out of the order they landed:\n%s", heat)
	}
	if lead := w.Report.Lead; len(lead) == 0 || !strings.HasPrefix(lead[0].Text, "The DA filed 3 pages on you") {
		t.Fatalf("the lead: %+v, want the file's 3 pages", lead)
	}
}

// The flow line says what it counts, sales less costs with the stock
// bought aside, and a night you lay low is put down to that (#522:
// "Profit ran ×5.6 … +$4,379" was the night's sales, and a lie-low
// night's fall was blamed on the corners' ceiling).
func TestFlowLineSaysWhatItCounts(t *testing.T) {
	for _, low := range []bool{false, true} {
		n, w := paper(t, 3)
		w.Day = 20
		for d := 13; d <= 19; d++ {
			w.Flows = append(w.Flows, game.CashFlow{Day: d, Opening: game.Pools{Dirty: 1_000 * d}, Closing: game.Pools{Dirty: 1_000*d + 10_000},
				Lines: []game.FlowLine{{Cat: game.FlowSales, Pools: game.Pools{Dirty: 10_000}}}})
		}
		var evs []events.Event
		if low {
			evs = append(evs, events.LaidLow{Day: 21})
		}
		n.Step(w, gametest.TickOn(w, 21, evs...))
		var flow string
		for _, l := range w.Report.Lead {
			if l.Kind == "flow" {
				flow = l.Text
			}
		}
		if strings.Contains(flow, "Profit") || !strings.Contains(strings.ToLower(flow), "sales less costs") {
			t.Errorf("lay low %v: the flow line %q", low, flow)
		}
		if low != strings.HasPrefix(flow, "You lay low last night") || strings.Contains(flow, "ceiling") {
			t.Errorf("lay low %v: the flow line %q", low, flow)
		}
	}
}

// A front bought and shut on one night is not in the paper as reopening
// (#523: "Laundromat reopens with cash to spare" the day it shut).
func TestFrontNewsAgreesWithTheFront(t *testing.T) {
	n, w := paper(t, 3)
	for d := 1; d <= 20; d++ {
		n.Step(w, gametest.TickOn(w, d, events.FrontBought{Day: d, Front: "laundromat", Name: "Laundromat", Cost: 5000},
			events.FrontFrozen{Day: d, Front: "laundromat", Name: "Laundromat", Upkeep: 100, Short: 50, Days: 3}))
		w.Day = d
		for _, h := range headlines(w, d) {
			if strings.Contains(h, "Laundromat") && (strings.Contains(h, "reopens") || strings.Contains(h, "changes hands")) {
				t.Fatalf("day %d: %q on the night it shut", d, h)
			}
		}
		if got := strings.Join(w.Report.Money, "\n"); !strings.Contains(got, "It opened today and shut tonight.") {
			t.Fatalf("day %d: the purchase line: %s", d, got)
		}
	}
}
