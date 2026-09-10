package territory_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

func world(t *testing.T, cfg *content.Config) (*game.World, *territory.Sim) {
	t.Helper()
	w := game.NewWorld(7, []game.StartingCity{{ID: cfg.City.Home().ID, Name: "Testville", Products: []game.StartingProduct{{ID: "weed", Name: "Weed", Price: 20, Demand: 60}}}}, 10_000, 100)
	s := territory.New(cfg.City)
	s.Seed(w)
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120},
		{ID: 2, Name: "Tank", Role: "enforcer", Skill: 50},
	}
	return w, s
}

func step(w *game.World, s *territory.Sim, extra ...events.Event) []events.Event {
	t := &game.Tick{Day: w.Day + 1, RNG: game.RNGFor(w.Seed, w.Day+1)}
	for _, e := range extra {
		t.Emit(e)
	}
	s.Step(w, t)
	w.Day++
	return t.Events()
}

func kinds(evs []events.Event) map[string]int {
	out := map[string]int{}
	for _, e := range evs {
		out[e.Kind()]++
	}
	return out
}

func TestSeedAndClaims(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg)
	if len(w.Home().Corners) != len(cfg.City.Home().Corners) || w.Held() != 1 || w.Worked() != 1 {
		t.Fatalf("seeded %d corners, %d held, %d worked", len(w.Home().Corners), w.Held(), w.Worked())
	}
	start := w.Corner(cfg.City.Territory.Start)
	if start == nil || start.Runner != game.You {
		t.Fatalf("you are not on the starting corner: %+v", start)
	}
	// The first day reports the corner you started on; taking another
	// during the day reports that one too.
	if err := w.Post("docks", 1); err != nil {
		t.Fatal(err)
	}
	evs := step(w, s)
	claimed := 0
	for _, e := range evs {
		if c, ok := e.(events.CornerClaimed); ok {
			claimed++
			switch c.Corner {
			case cfg.City.Territory.Start:
				if c.Worker != "you" {
					t.Fatalf("start corner worker %q", c.Worker)
				}
			case "docks":
				if c.Worker != "Dre" {
					t.Fatalf("docks worker %q", c.Worker)
				}
			default:
				t.Fatalf("claimed %s", c.Corner)
			}
		}
	}
	if claimed != 2 {
		t.Fatalf("%d claims reported: %v", claimed, kinds(evs))
	}
	if k := kinds(step(w, s)); k["CornerClaimed"] != 0 {
		t.Fatalf("claims reported twice: %v", k)
	}
	// Migration lays the city out for a save that has none.
	old := game.NewWorld(1, []game.StartingCity{{ID: cfg.City.Home().ID, Name: "Testville"}}, 500, 100)
	s.Migrate(old)
	if old.Worked() != 1 || old.Corner(cfg.City.Territory.Start).Runner != game.You {
		t.Fatalf("migrated world: %d worked", old.Worked())
	}
}

// A held corner nobody works drifts back to the street after drift_days,
// and a post pointing at someone who is gone counts as nobody.
func TestIdleCornersDrift(t *testing.T) {
	cfg := content.MustLoad()
	w, s := world(t, cfg)
	drift := cfg.City.Territory.DriftDays
	if err := w.Post("docks", 1); err != nil {
		t.Fatal(err)
	}
	if err := w.Post("docks", 2); err != nil {
		t.Fatal(err)
	}
	w.Crew.Members = w.Crew.Members[1:] // Dre vanishes without being recalled
	for i := 0; i < drift-1; i++ {
		evs := step(w, s)
		if k := kinds(evs); k["CornerLost"] != 0 {
			t.Fatalf("day %d: lost a corner before drift_days: %v", w.Day, k)
		}
		if d := w.Corner("docks"); !d.Held() || d.Runner != 0 || d.Idle != i+1 {
			t.Fatalf("day %d: docks %+v", w.Day, *d)
		}
	}
	evs := step(w, s)
	if k := kinds(evs); k["CornerLost"] != 1 {
		t.Fatalf("no CornerLost on day %d: %v", w.Day, k)
	}
	if d := w.Corner("docks"); d.Held() || d.Enforcer != 0 || w.PostOf(2) != nil {
		t.Fatalf("after drifting: %+v", *d)
	}
	if !w.Corner(cfg.City.Territory.Start).Worked() {
		t.Fatal("your own corner drifted with you on it")
	}
}

// Robberies happen on worked corners, take cash and stock in proportion to
// the corner's share, and an enforcer makes them rarer.
func TestRobberies(t *testing.T) {
	cfg := content.MustLoad()
	count := func(guard bool) (robberies, cash, stock int) {
		w, s := world(t, cfg)
		if err := w.Post("docks", 1); err != nil {
			t.Fatal(err)
		}
		if guard {
			if err := w.Post("docks", 2); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Abandon(cfg.City.Territory.Start); err != nil {
			t.Fatal(err)
		}
		for day := 0; day < 400; day++ {
			w.Stash(w.Home().ID)["weed"] = 100
			w.Player.DirtyCash = 10_000
			for _, e := range step(w, s, events.PlayerSold{Day: w.Day + 1, City: w.Home().ID, Product: "weed", Sold: 60, Revenue: 1200}) {
				r, ok := e.(events.CornerRobbed)
				if !ok {
					continue
				}
				robberies++
				cash += r.Cash
				stock += r.StockLost["weed"]
				if r.Corner != "docks" || r.Cash <= 0 || r.StockLost["weed"] <= 0 {
					t.Fatalf("robbery %+v", r)
				}
				if w.Player.DirtyCash != 10_000-r.Cash || w.Stock(w.Home().ID, "weed") != 100-r.StockLost["weed"] {
					t.Fatalf("robbery not applied: cash %d stock %d for %+v", w.Player.DirtyCash, w.Stock(w.Home().ID, "weed"), r)
				}
			}
		}
		return
	}
	n, cash, stock := count(false)
	tun := cfg.City.Territory
	if n == 0 {
		t.Fatal("400 days on the docks and never robbed")
	}
	// The docks are the only corner worked, so a robbery takes the full
	// robbery_cash of the takings and robbery_stock of the stock.
	if want := int(1200*tun.RobberyCash) * n; cash != want {
		t.Fatalf("%d robberies took $%d, want $%d", n, cash, want)
	}
	if want := int(100*tun.RobberyStock) * n; stock != want {
		t.Fatalf("%d robberies took %d units, want %d", n, stock, want)
	}
	g, _, _ := count(true)
	if float64(g) > float64(n)*0.6 {
		t.Fatalf("an enforcer only cut robberies from %d to %d", n, g)
	}
	w, s := world(t, cfg)
	d := w.Corner("docks")
	if p := s.RobberyChance(w, d); p != tun.RobberyChance*d.Risk {
		t.Fatalf("unguarded chance %.4f", p)
	}
}
