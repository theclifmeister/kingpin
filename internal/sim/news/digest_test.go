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
