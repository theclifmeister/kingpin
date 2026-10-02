package main

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui"
)

// The alerts' parity (#558): the web's alerts.js against the TUI's
// words, clause by clause. TestWebClient words the fixture's alerts
// (alertFixture) through alerts.js on the same morning's view, and
// alertParity checks that every clause of the TUI's line is in the
// web's, bar the ones webLeavesOut names. So a clause the TUI gains,
// such as a number an alert's field carries, reaches the web or is
// written down here as left out, and a clause left out that the web
// has since caught up on is taken off the list. The TUI's words are
// worked out live (ui.AlertWords), so nothing here goes stale with
// them.

// webLeavesOut are the clauses of the TUI's alert lines the web does
// not say, by kind, as alertClauses cuts them, each with why. A
// pointer to a TUI screen or key ("on the crew screen (4)", "(i)") is
// cut before the clauses are, so it needs no row. A row is a gap to
// close or a difference to keep: when the web catches up, the test
// says so and the row goes. docs/web.md "Keeping up with the TUI".
var webLeavesOut = map[engine.AlertKind][]string{
	// The TUI's own way in, a key or a menu in words: the web answers
	// an alert by where it lands (landing.js), so these stay out.
	engine.AlertLanded:   {"s sells it", "standing if it should go nightly"},
	engine.AlertExports:  {"then t on a lane", "t on a lane sets a nightly load"},
	engine.AlertTalking:  {"somebody is talking", "investigate"},
	engine.AlertStraight: {"go straight (walk away) or play on"},
	engine.AlertVanish:   {"vanish on the new identity with $700 offshore (walk away) or play on"},

	// The remedies and the other facts are said since #573 (ruled
	// 2026-10-01: ported, not left to the landing). The TUI counts the quiet days off the streak and the account off
	// the world; the web, the days and the cash short the alert carries.
	engine.AlertRetire: {"retiring", "5 of 14 quiet days", "$749k short"},
}

// pointer is a TUI screen's pointer and keyHint a key's, which the web
// words its own way (a panel, a button), cut before the comparison.
var (
	pointer = regexp.MustCompile(` ?on the [a-z]+ screen \(\d+\)`)
	keyHint = regexp.MustCompile(` \([a-zA-Z]\)`)
	clause  = regexp.MustCompile(`[.:;]\s+|[.:;]$|,\s+|\s+·\s+`)
)

// alertClauses is a TUI alert line as the clauses the web must say:
// lower case, the pointers and key hints cut, split at the stops,
// colons, semicolons, commas and middle dots.
func alertClauses(text string) []string {
	text = keyHint.ReplaceAllString(pointer.ReplaceAllString(text, ""), "")
	var out []string
	for _, c := range clause.Split(strings.ToLower(text), -1) {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// parityAlert is one alert of the fixture with the TUI's words for it.
type parityAlert struct {
	Alert engine.Alert `json:"alert"`
	TUI   string       `json:"-"`
}

// alertFixture writes, to path, a morning's view and one alert or more
// of every kind on it, as {view, alerts}, and returns the alerts with
// the TUI's words. The morning is seed 7's first, standing in the
// second city, with what the alerts name put on it under names of the
// fixture's own (a crew of three, a buyer, a front, a house, the rival
// at home as Sal), so webLeavesOut's rows read the same whatever the
// content's seeds name; the alerts are made up with every field their
// kind carries, money under $1,000 so the TUI's short cash and the
// web's full dollars print alike. An alert of each branch the TUI
// words apart (a lieutenant's line and a runner's, each thing an
// investigation names) is its own row.
func alertFixture(t *testing.T, path string) []parityAlert {
	t.Helper()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(7, game.Start{})
	w.Day = 5 // a due day of 0 is left off the wire
	if len(w.CityOrder) < 2 || len(w.Suppliers) == 0 || len(w.Rivals) == 0 {
		t.Fatalf("seed 7's first morning has %d cities, %d connects and %d factions: the fixture needs two, one and one", len(w.CityOrder), len(w.Suppliers), len(w.Rivals))
	}
	home := w.CityOrder[0]
	w.Player.Location = w.CityOrder[1]
	here := w.Here()
	here.Corners[0].Name = "the Wharf"
	corner := here.Corners[0].ID
	product := w.Products[len(w.Products)-1]
	w.Crew.Members = append(w.Crew.Members,
		game.CrewMember{ID: 1, Name: "Dre", Role: game.RoleRunner, Loyalty: 50},
		game.CrewMember{ID: 2, Name: "Moose", Role: game.RoleEnforcer, Loyalty: 50},
		game.CrewMember{ID: 3, Name: "Vito", Role: game.RoleLieutenant, Loyalty: 25},
	)
	w.Crew.NextID = 3
	w.Contracts = append(w.Contracts, game.Contract{ID: 1, Name: "Ray", City: here.ID, Product: product, Units: 10, Delivered: 4, Due: w.Day})
	w.Fronts = append(w.Fronts, game.Front{ID: "parity", Name: "Sunny Car Wash", City: here.ID})
	w.Houses = append(w.Houses, game.House{ID: "parity", Name: "the Lockup", City: here.ID, Corner: corner, Capacity: 100, Stock: map[string]int{}})
	w.Suppliers[0].Name = "Cass"
	w.Rivals[0].Leader = "Sal"
	faction := w.Rivals[0].Faction()
	w.Law.Chief.Name = "Ambrose"
	w.Offshore, w.Player.CleanCash, w.Stats.PeakCash = 700, 300, 400
	off := cfg.Laundering.Offshore
	w.QuietDays = off.RetireDays - 9
	amb := cfg.Ambitions.Ambitions[0]
	alerts := []engine.Alert{
		{Kind: engine.AlertArrest, City: here.ID, Heat: 82, Line: 80, Days: 2},
		{Kind: engine.AlertBroke, Have: 224, Amount: 221, Count: 30, Gap: 3, Line: 40},
		{Kind: engine.AlertTalking},
		{Kind: engine.AlertPages, Level: engine.PagesInformant, Have: 1, Count: 3, Amount: 6},
		{Kind: engine.AlertPages, Level: engine.PagesRetiree, Have: 2, Count: 5, Amount: 6},
		{Kind: engine.AlertPages, Level: engine.PagesTip, Have: 1, Count: 2, Amount: 6},
		{Kind: engine.AlertTaskForce},
		{Kind: engine.AlertFile, Count: 4, Amount: 7, Have: 3},
		{Kind: engine.AlertInvestigation, Target: game.LeadCorner, Corner: corner, City: here.ID, Days: 3},
		{Kind: engine.AlertInvestigation, Target: game.LeadProduct, Product: product, City: home, Days: 1},
		{Kind: engine.AlertInvestigation, Target: game.LeadHouse, House: "parity", City: here.ID, Days: 2},
		{Kind: engine.AlertWarMuscle, Level: faction, Have: 0, Amount: 2, Count: 1},
		{Kind: engine.AlertNoCorner, City: here.ID, Corner: corner},
		{Kind: engine.AlertContractDue, Contract: 1, Due: w.Day},
		{Kind: engine.AlertDebtDue, Supplier: w.Suppliers[0].ID, Due: w.Day + 1, Amount: 600, Have: 450},
		{Kind: engine.AlertHeat, City: here.ID, Heat: 61, Line: 60, Level: content.TaskForce},
		{Kind: engine.AlertFrontShut, Front: "parity", Days: 7, Amount: 120, Have: 90},
		{Kind: engine.AlertFloat, Have: 400, Amount: 900},
		{Kind: engine.AlertTill, Amount: 800, Days: 3},
		{Kind: engine.AlertWages, Amount: 500, Have: 350},
		{Kind: engine.AlertCrewLine, Member: 1, Cross: "walk", Gap: 3.2, Days: 2},
		{Kind: engine.AlertCrewLine, Member: 1, Cross: "under", Line: 20},
		{Kind: engine.AlertCrewLine, Member: 3, Cross: "under", Line: 30},
		{Kind: engine.AlertSkim, Day: 3},
		{Kind: engine.AlertUnposted, Member: 1, Corner: corner, City: here.ID},
		{Kind: engine.AlertUnposted, Member: 2, Corner: corner, City: here.ID},
		{Kind: engine.AlertUnposted, Member: 1, City: here.ID},
		{Kind: engine.AlertIdleCorner, Corner: corner, City: here.ID, Days: 2},
		{Kind: engine.AlertStashFull, City: here.ID, Count: 400, Amount: 400},
		{Kind: engine.AlertLanded, City: here.ID, Product: product, Count: 40},
		{Kind: engine.AlertScouts, Faction: faction, City: here.ID, Level: "scouting", Days: 4},
		{Kind: engine.AlertScouts, Faction: faction, City: home, Level: "recruiting"},
		{Kind: engine.AlertGate, Gate: &engine.Gate{Kind: "front", ID: "laundromat", Name: "Laundromat", Line: 900}, Amount: 500},
		{Kind: engine.AlertPort, City: home, Count: 6, Product: product, Amount: 950, Supplier: w.Suppliers[0].ID, Share: 0.35},
		{Kind: engine.AlertExports, City: here.ID, Product: product, Amount: 310, Have: 45, Count: 160},
		{Kind: engine.AlertExports, City: here.ID, Product: product, Amount: 310, Have: 45, Count: 160, Ready: true},
		{Kind: engine.AlertHouseKnown, House: "parity"},
		{Kind: engine.AlertDARace, Days: 5},
		{Kind: engine.AlertRetire, Days: 9, Amount: off.RetireCash - w.Offshore},
		{Kind: engine.AlertFavour, Level: content.TaskForce},
		{Kind: engine.AlertReign, Days: 9, Count: 2, Amount: 300},
		{Kind: engine.AlertReign, Days: 9, Slip: 2},
		{Kind: engine.AlertExposure, Count: 2, Amount: 700, Heat: 4},
		{Kind: engine.AlertStraight, Amount: 900},
		{Kind: engine.AlertVanish},
		{Kind: engine.AlertPlan, Ambition: amb.ID, Count: 2, Steps: 4},
		{Kind: engine.AlertPlan, Ambition: amb.ID, Count: 4, Steps: 4, Ready: true},
	}
	var kinds []engine.AlertKind
	for _, a := range alerts {
		kinds = append(kinds, a.Kind)
	}
	for _, k := range engine.AlertKinds() {
		if !slices.Contains(kinds, k) {
			t.Errorf("the parity fixture has no %s alert: add one with every field it carries", k)
		}
	}
	words := ui.AlertWords(cfg, s, alerts)
	out := make([]parityAlert, len(alerts))
	for i, a := range alerts {
		if words[i] == "" {
			t.Errorf("the TUI words the fixture's %s %+v as nothing: it names something the morning lacks", a.Kind, a)
		}
		out[i] = parityAlert{Alert: a, TUI: words[i]}
	}
	b, err := json.Marshal(struct {
		View   engine.View    `json:"view"`
		Alerts []engine.Alert `json:"alerts"`
	}{s.View(), alerts})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}

// alertParity holds the web's words for the fixture's alerts to the
// TUI's: every clause the TUI says, the web says, or webLeavesOut names
// it; and every clause webLeavesOut names is one the web still leaves
// out.
func alertParity(t *testing.T, fixture []parityAlert, web []string) {
	t.Helper()
	if len(web) != len(fixture) {
		t.Fatalf("the web worded %d of the parity fixture's %d alerts", len(web), len(fixture))
	}
	used := map[engine.AlertKind]map[string]bool{}
	for i, f := range fixture {
		said := strings.ToLower(web[i])
		for _, c := range alertClauses(f.TUI) {
			if strings.Contains(said, c) {
				continue
			}
			if slices.Contains(webLeavesOut[f.Alert.Kind], c) {
				if used[f.Alert.Kind] == nil {
					used[f.Alert.Kind] = map[string]bool{}
				}
				used[f.Alert.Kind][c] = true
				continue
			}
			t.Errorf("%s: the web leaves out the TUI's %q\n  TUI: %s\n  web: %s\nport it to web/js/alerts.js, or name it in webLeavesOut with why (docs/web.md)", f.Alert.Kind, c, f.TUI, web[i])
		}
	}
	for kind, clauses := range webLeavesOut {
		for _, c := range clauses {
			if !used[kind][c] {
				t.Errorf("%s: webLeavesOut names %q, which the web now says or the TUI no longer does: take it off", kind, c)
			}
		}
	}
}
