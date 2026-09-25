package news_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// TestLeadIsTheBiggestThree (#354): a table of nights and the lead each
// must produce, biggest first: every kind scores its [digest] weight
// times its count, the top three lead, a tie goes to the kind listed
// first, a quiet night has no lead, and each line's act lands on the
// screen that deals with it, on the subject it names. The journal keeps
// the lines under the digest's source.
func TestLeadIsTheBiggestThree(t *testing.T) {
	cfg := content.MustLoad()
	wt := cfg.Headlines.Digest.Weights
	if wt["corner_lost"] <= wt["crew_lost"] || wt["crew_lost"] != 2*wt["corner_won"] || wt["seizure"] > 5*wt["pages"] {
		t.Fatalf("the table below is written for the file's weights: %v", wt)
	}
	type want struct {
		kind, text string
		act        game.Act
	}
	for _, tc := range []struct {
		name  string
		setup func(w *game.World) []events.Event
		want  []want
	}{
		{"a quiet night", func(w *game.World) []events.Event { return nil }, nil},
		{"two corners, a quit, pages and a claim", func(w *game.World) []events.Event {
			c := w.Home().Corners
			return []events.Event{
				events.CornerLost{Corner: c[0].ID, Name: c[0].Name, Reason: "idle", Owner: game.OwnerPlayer},
				events.CornerTaken{Corner: c[1].ID, Name: c[1].Name, Rival: "Sal", From: game.OwnerPlayer},
				events.CrewQuit{ID: 9, Name: "Ray", Role: game.RoleRunner},
				events.LeadsFiled{Leads: 1, Evidence: 2},
				events.CornerClaimed{Corner: c[2].ID, Name: c[2].Name, Worker: "you"},
			}
		}, []want{
			{"corner_lost", "Lost 2 corners: ", game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}},
			{"crew_lost", "Lost Ray (quit).", game.Act{Screen: game.ScreenCrew}},
			{"pages", "The DA filed 2 pages on you.", game.Act{Screen: game.ScreenDashboard}},
		}},
		{"a raid thick with pages", func(w *game.World) []events.Event {
			return []events.Event{
				events.Enforcement{City: w.Home().ID, Level: content.Raid, StockLost: map[string]int{"weed": 40}, CashLost: 2_000, Evidence: 5},
			}
		}, []want{
			{"pages", "The DA filed 5 pages on you.", game.Act{Screen: game.ScreenDashboard}},
			{"seizure", "The police took 40 units and $2,000.", game.Act{Screen: game.ScreenDashboard}},
		}},
		{"a tie goes to the kind listed first", func(w *game.World) []events.Event {
			c := w.Home().Corners
			return []events.Event{
				events.CornerClaimed{Corner: c[3].ID, Name: c[3].Name, Worker: "you"},
				events.CornerClaimed{Corner: c[4].ID, Name: c[4].Name, Worker: "you"},
				events.CrewDefected{ID: 9, Name: "Dee", Rival: "Sal"},
			}
		}, []want{
			{"crew_lost", "Lost Dee (defected).", game.Act{Screen: game.ScreenCrew}},
			{"corner_won", "Took 2 corners: ", game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}},
		}},
		{"a corner nobody works and a week of profit gone", func(w *game.World) []events.Event {
			c := &w.Home().Corners[5]
			c.Owner = game.OwnerPlayer
			for d := 1; d <= 7; d++ {
				w.Flows = append(w.Flows, game.CashFlow{Day: d, Opening: game.Pools{Dirty: 1_000 * d}, Closing: game.Pools{Dirty: 1_000*d + 10_000},
					Lines: []game.FlowLine{{Cat: game.FlowSales, Pools: game.Pools{Dirty: 10_000}}}})
			}
			return nil
		}, []want{
			{"flow", "Profit fell 100% on the week: $0 against +$10K a night.", game.Act{Screen: game.ScreenLedger}},
			{"idle_corner", "Nobody works a corner: ", game.Act{Screen: game.ScreenMap, Mode: game.ModePost, Subject: game.OnCorner}},
		}},
		{"money put offshore is not the night's loss (#422)", func(w *game.World) []events.Event {
			for d := 1; d <= 7; d++ {
				w.Flows = append(w.Flows, game.CashFlow{Day: d, Opening: game.Pools{Dirty: 1_000 * d}, Closing: game.Pools{Dirty: 1_000*d + 10_000},
					Lines: []game.FlowLine{{Cat: game.FlowSales, Pools: game.Pools{Dirty: 10_000}}}})
			}
			return []events.Event{events.Reserved{Day: 21, Amount: 47_500, Fee: 2_500}}
		}, []want{
			// the fee is the night's; the $47,500 in the account is not ("-$50K" before)
			{"flow", "The night made -$2,500 against the week's +$10K a night.", game.Act{Screen: game.ScreenLedger}},
		}},
		{"an investigation and the scouts", func(w *game.World) []events.Event {
			c := w.Home().Corners[2]
			return []events.Event{
				events.InvestigationOpened{Day: 21, City: c.City, Lead: game.LeadCorner, Target: c.ID, Name: c.Name, Due: 24},
				events.RivalScouting{City: w.Home().ID, Rival: "Sal", Faction: "f2", Recruit: 25, Arrive: 30},
				events.CornerClaimed{Corner: c.ID, Name: c.Name, Worker: "you"},
			}
		}, []want{
			{"investigation", "The police opened an investigation on ", game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}},
			{"scouts", "Sal's scouts are in ", game.Act{Screen: game.ScreenRivals}},
			{"corner_won", "Took a corner: ", game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}},
		}},
		{"shot and alive is laid up, not lost (#423)", func(w *game.World) []events.Event {
			return []events.Event{
				events.CrewShot{ID: 9, Name: "Cash", Role: game.RoleEnforcer, Days: 12},
				events.CrewQuit{ID: 8, Name: "Ray", Role: game.RoleRunner},
			}
		}, []want{
			{"crew_lost", "Lost Ray (quit). Laid up: Cash (shot, 12 days).", game.Act{Screen: game.ScreenCrew}},
		}},
		{"only shot and alive", func(w *game.World) []events.Event {
			return []events.Event{events.CrewShot{ID: 9, Name: "Cash", Role: game.RoleEnforcer, Days: 12}}
		}, []want{
			{"crew_lost", "Laid up: Cash (shot, 12 days).", game.Act{Screen: game.ScreenCrew}},
		}},
		{"an enforcer off a lost corner is idle too (#419)", func(w *game.World) []events.Event {
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 90, Name: "Bird", Role: game.RoleEnforcer, Loyalty: 60})
			return nil
		}, []want{
			{"idle_runner", "An enforcer is idle: Bird.", game.Act{Screen: game.ScreenCrew, Subject: game.OnMember}},
		}},
		{"idle runners and enforcers in one line", func(w *game.World) []events.Event {
			w.Crew.Members = append(w.Crew.Members,
				game.CrewMember{ID: 90, Name: "Bird", Role: game.RoleEnforcer, Loyalty: 60},
				game.CrewMember{ID: 91, Name: "Cash", Role: game.RoleRunner, Loyalty: 60})
			return nil
		}, []want{
			{"idle_runner", "2 of the crew are idle: Cash (runner), Bird (enforcer).", game.Act{Screen: game.ScreenCrew, Subject: game.OnMember}},
		}},
		{"only three lead", func(w *game.World) []events.Event {
			c := w.Home().Corners
			return []events.Event{
				events.CornerLost{Corner: c[0].ID, Name: c[0].Name, Reason: "crackdown", Owner: game.OwnerPlayer},
				events.CrewArrested{ID: 9, Name: "Ray", Role: game.RoleRunner},
				events.ShipmentSeized{Units: 30},
				events.RivalMovedIn{Rival: "Sal", Corner: c[6].ID, Name: c[6].Name},
				events.CornerClaimed{Corner: c[2].ID, Name: c[2].Name, Worker: "you"},
			}
		}, []want{
			{"corner_lost", "Lost a corner: ", game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}},
			{"seizure", "The police took 30 units.", game.Act{Screen: game.ScreenDashboard}},
			{"crew_lost", "Lost Ray (arrested).", game.Act{Screen: game.ScreenCrew}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := news.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			w := sim.NewWorld(cfg, 4)
			w.Day = 20
			evs := tc.setup(w)
			n.Step(w, gametest.TickOn(w, 21, evs...))
			lead := w.Report.Lead
			if len(lead) != len(tc.want) {
				t.Fatalf("lead %+v, want %d lines", lead, len(tc.want))
			}
			var journal []string
			for _, h := range w.Journal {
				if h.Source == "digest" {
					journal = append(journal, h.Text)
				}
			}
			for i, l := range lead {
				wl := tc.want[i]
				if l.Kind != wl.kind || !strings.HasPrefix(l.Text, wl.text) || l.Act != wl.act {
					t.Errorf("line %d: %+v, want %+v", i+1, l, wl)
				}
				if l.Act.Subject == game.OnCorner && w.Corner(l.Corner) == nil {
					t.Errorf("line %d names corner %q, none of the map's", i+1, l.Corner)
				}
				if i >= len(journal) || journal[i] != l.Text {
					t.Errorf("the journal's digest lines %q lack line %d", journal, i+1)
				}
			}
		})
	}
}

// A corner a push took says what beat you (#419): the heads that
// pushed, the enforcer they got past or nobody guarding, the odds.
func TestTakenCornerSaysWhatBeatYou(t *testing.T) {
	cfg := content.MustLoad()
	for _, tc := range []struct {
		ev   events.CornerTaken
		want string
	}{
		{events.CornerTaken{Name: "The Projects", Rival: "Mona", From: game.OwnerPlayer, Odds: 0.38, Muscle: 4, Guard: "Bird"},
			"Mona's crew TOOK The Projects from you: 4 heads pushed past Bird, a 38% push. Your people walked home."},
		{events.CornerTaken{Name: "The Docks", Rival: "Mona", From: game.OwnerPlayer, Odds: 0.55, Muscle: 1},
			"Mona's crew TOOK The Docks from you: 1 head pushed on nobody guarding it, a 55% push. Your people walked home."},
		{events.CornerTaken{Name: "The Docks", Rival: "Mona", From: game.OwnerPlayer},
			"Mona's crew TOOK The Docks from you. Your people walked home."},
	} {
		n, err := news.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 4)
		w.Day = 20
		n.Step(w, gametest.TickOn(w, 21, tc.ev))
		if !slices.Contains(w.Report.Territory, tc.want) {
			t.Errorf("territory %q, want %q", w.Report.Territory, tc.want)
		}
	}
}

// A standing order of yours that sold out with as much again in the
// stash says it is too small (#418); one the stash could not cover, or
// the lieutenant's, does not.
func TestSmallStandingOrderIsFlagged(t *testing.T) {
	cfg := content.MustLoad()
	for _, tc := range []struct {
		name  string
		stock int
		ev    events.PlayerSold
		flag  bool
	}{
		{"sold out, 25 left", 25, events.PlayerSold{Wanted: 5, Sold: 5, Standing: true}, true},
		{"sold out, 3 left", 3, events.PlayerSold{Wanted: 5, Sold: 5, Standing: true}, false},
		{"short of demand", 25, events.PlayerSold{Wanted: 5, Sold: 4, Standing: true}, false},
		{"placed today", 25, events.PlayerSold{Wanted: 5, Sold: 5}, false},
		{"the lieutenant's", 25, events.PlayerSold{Wanted: 5, Sold: 5, Standing: true, Delegated: true}, false},
	} {
		n, err := news.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := sim.NewWorld(cfg, 4)
		w.Day = 20
		city, product := w.Home().ID, w.Products[0]
		w.SetStock(city, product, tc.stock)
		ev := tc.ev
		ev.Day, ev.City, ev.Product, ev.Dial, ev.AvgPrice, ev.Revenue = 21, city, product, events.DialNormal, 20, 20*ev.Sold
		n.Step(w, gametest.TickOn(w, 21, ev))
		got := strings.Join(w.Report.Sales, "\n")
		if strings.Contains(got, "the standing order sold all") != tc.flag {
			t.Errorf("%s: sales %q", tc.name, w.Report.Sales)
		}
	}
}

// The falling profit line points at the road (#446) when the run is at
// Territory, no route is on and the last week made no more than the one
// before it: the one city's ceiling. A route on, an earlier tier or a
// rising fortnight keeps the line as it was.
func TestFallingProfitPointsAtTheRoad(t *testing.T) {
	cfg := content.MustLoad()
	territory := 0
	for i, tier := range cfg.Progression.Tiers {
		if tier.ID == "territory" {
			territory = i + 1
		}
	}
	if territory == 0 {
		t.Fatal("no territory tier in progression.toml")
	}
	night := func(d, made int) game.CashFlow {
		return game.CashFlow{Day: d, Opening: game.Pools{Dirty: 100_000}, Closing: game.Pools{Dirty: 100_000 + made},
			Lines: []game.FlowLine{{Cat: game.FlowSales, Pools: game.Pools{Dirty: made}}}}
	}
	for _, tc := range []struct {
		name   string
		tier   int
		route  bool
		rising bool
		road   bool
	}{
		{"territory, no road, a flat fortnight", territory, false, false, true},
		{"a route on", territory, true, false, false},
		{"an earlier tier", territory - 1, false, false, false},
		{"a rising fortnight", territory, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := news.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			w := sim.NewWorld(cfg, 4)
			w.Day = 30
			w.Progression.Reached = map[int]int{}
			for i := 2; i <= tc.tier; i++ {
				w.Progression.Reached[i] = i
			}
			if tc.route {
				if err := w.SetRoute(cfg.Routes.Routes[0].ID, events.RouteNormal); err != nil {
					t.Fatal(err)
				}
			}
			for d := 1; d <= 14; d++ {
				made := 10_000
				if tc.rising && d > 7 {
					made = 12_000
				}
				w.Flows = append(w.Flows, night(d, made))
			}
			// Tonight makes nothing (the sim's own flow, not in Flows): a fall.
			n.Step(w, gametest.TickOn(w, 31))
			var flow *game.Line
			for i := range w.Report.Lead {
				if w.Report.Lead[i].Kind == "flow" {
					flow = &w.Report.Lead[i]
				}
			}
			if flow == nil {
				t.Fatalf("no flow line: %+v", w.Report.Lead)
			}
			hinted := strings.Contains(flow.Text, "the road to ")
			if hinted != tc.road || (hinted && flow.Act.Screen != game.ScreenMap) || (!hinted && flow.Act.Screen != game.ScreenLedger) {
				t.Fatalf("hint %v, want %v: %+v", hinted, tc.road, *flow)
			}
		})
	}
}
