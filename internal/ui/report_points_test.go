package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// screenWords is a TUI screen or key named in a report line's words:
// "screen (7)", "(map, r)", "(2, d)", "(f)", "on the dashboard".
var screenWords = regexp.MustCompile(`screen\b|\(\w+, \w\)|\(\w\)|on the dashboard|on the map \(`)

// TestReportLinesPointAndTheTUIWordsThem (#560): a report line that
// points at a screen names none in its words; it carries the act, and
// the TUI words its own pointer, so its line reads as it did before the
// act (the front, the spy, the lure, the books and the scouts lines
// reworded so the words read whole without one).
func TestReportLinesPointAndTheTUIWordsThem(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	cases := []struct {
		kind string
		evs  []events.Event
		want string // the line as the TUI draws it
	}{
		{"reign", []events.Event{events.ReignBegan{City: "eastside"}},
			"The city is yours: every crew in Eastside is gone or paying. Take the crown when you are ready (walk away on the dashboard), or reign."},
		{"straight", []events.Event{events.StraightOpened{Income: 900, Street: 800}},
			"The fronts earn $900 a day against $800 off the street: you could go straight (walk away on the dashboard), or play on."},
		{"idle_crew", []events.Event{events.LieutenantActed{Name: "Vic", CityName: "Eastside", Took: []events.Took{{Name: "Jo", Role: game.RoleRunner, Corner: "5th & Main"}}}},
			"  Vic put idle crew to work in Eastside: Jo on 5th & Main. Nobody ordered it; post them yourself on the map screen (5) to keep them elsewhere."},
		{"front", []events.Event{events.Unlocked{Gate: "front", ID: "laundromat", Name: "Laundromat", Cost: 5000}},
			"The Laundromat is open to you on the ledger screen (7): it washes over the till."},
		{"role", []events.Event{events.Unlocked{Gate: "role", ID: game.RoleChemist, Name: "Chemists", Why: "Meth on the ladder"}},
			"Chemists want work on the crew screen (4): Meth on the ladder."},
		{"asset", []events.Event{events.Unlocked{Gate: "asset", ID: "yacht", Name: "A yacht", Cost: 900000}},
			"A yacht is for sale on the ledger screen (7), clean cash: $900K."},
		{"spy", []events.Event{events.SpyPlanted{Name: "Jo", Rival: "Marco"}},
			"Jo went under with Marco's crew tonight. They sell nothing for you now, and what they report every few days is kept on the intel screen (9)."},
		{"lure", []events.Event{events.IntelFalse{FactKind: game.FactRisk, Name: "the coast road", Rival: "Marco"}},
			"The word on the coast road was Marco's: they had customs waiting. Their name is filed on the intel screen (9)."},
		{"contract", []events.Event{events.ContractAccepted{Name: "a foreman at the docks", City: "eastside", Product: "weed", Units: 30, Due: 12}},
			"You took the order from a foreman at the docks: 30 Weed by day 12. Deliver it there (2, d)."},
		{"informant", []events.Event{events.InvestigationRun{Found: true, Name: "Jo"}},
			"The investigation named Jo: they have been talking to the police. Fire them (f) and the file stops growing."},
		{"books", []events.Event{events.RivalScouted{Read: true, Cash: 9000, Income: 800, Muscle: 3, Wages: 300}},
			"books read, mind its age on the rivals screen (8)."},
		{"offer", []events.Event{events.DealOffered{Rival: "Marco", Deal: "truce", Terms: "a truce for 10 days", Expires: 7}},
			"It stands 3 days: answer it on the rivals screen (8)."},
		{"scouts", []events.Event{events.RivalScouting{City: "eastside", Rival: "Marco", Recruit: 8, Arrive: 12}},
			"Take the corners there, or hit the scouts on the rivals screen (8)."},
	}
	for _, c := range cases {
		n, err := news.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 4)
		w.Day = 4
		n.Step(w, gametest.TickOn(w, 5, c.evs...))
		var got []string
		for _, sec := range engine.ReportSections(w.Report) {
			for _, l := range sec.Lines {
				if screenWords.MatchString(l) {
					t.Errorf("%s: a %s line names a screen: %q", c.kind, sec.ID, l)
				}
			}
			for _, p := range sec.Points {
				if p.Kind == c.kind {
					got = append(got, pointed(sec.Lines[p.Line], p.At, p.Act))
				}
			}
		}
		if len(got) != 1 {
			t.Errorf("%s: %d lines point, want 1: %q", c.kind, len(got), got)
			continue
		}
		if c.kind == "books" {
			// The books' line is long; its end is the pointer's.
			c.want = strings.TrimPrefix(c.want, "books read, ")
		}
		if !strings.HasSuffix(got[0], c.want) {
			t.Errorf("%s: the TUI reads\n%q\nwant it to end\n%q", c.kind, got[0], c.want)
		}
	}
}

// TestPointerKeysAreBound (#560): every key a report line's pointer
// names is bound on the screen it names.
func TestPointerKeysAreBound(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		act engine.Act
		key string
		on  screen
	}{
		{engine.Act{Screen: game.ScreenDashboard, Mode: game.ModeWalk}, "w", screenDashboard},
		{engine.Act{Screen: game.ScreenMap, Mode: game.ModeDial}, "r", screenMap},
		{engine.Act{Screen: game.ScreenMarket, Mode: game.ModeDeliver}, "d", screenMarket},
		{engine.Act{Screen: game.ScreenCrew, Mode: game.ModeFire}, "f", screenCrew},
	} {
		found := false
		for _, b := range bindings {
			for _, s := range b.screens {
				if b.key == c.key && s == c.on {
					found = true
				}
			}
		}
		if !found || pointerWords(c.act) == "" {
			t.Errorf("%+v: %q is not bound on screen %d, or the pointer is %q", c.act, c.key, c.on, pointerWords(c.act))
		}
	}
	for _, name := range []string{game.ScreenDashboard, game.ScreenMarket, game.ScreenCrew, game.ScreenMap, game.ScreenLedger, game.ScreenRivals, game.ScreenIntel} {
		if pointerWords(engine.Act{Screen: name}) == "" {
			t.Errorf("no pointer to the %s screen", name)
		}
	}
}
