package territory_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

func world(t *testing.T, cfg *content.Config) (*game.World, *territory.Sim) {
	t.Helper()
	w := gametest.City(7, cfg.City.Home().ID, "Testville", 10_000, gametest.Weed)
	s := territory.New(cfg)
	s.Seed(w)
	w.Crew.Members = []game.CrewMember{
		{ID: 1, Name: "Dre", Role: "runner", Skill: 60, Units: 120},
		{ID: 2, Name: "Tank", Role: "enforcer", Skill: 50},
	}
	return w, s
}

func step(w *game.World, s *territory.Sim, extra ...events.Event) []events.Event {
	return gametest.StepUnseeded(w, s, extra...).Events()
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
			w.SetStock(w.Home().ID, "weed", 100)
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

// The Street branch's territory nodes (#119): each moves the one number
// it names and nothing else. The corner boys add two days to the drift
// and leave the robbery chance alone; the watchmen and the dogs cut the
// robbery chance, as a product, before the enforcer's cut, and leave the
// drift alone; and an idle corner with the boys lasts exactly the folded
// days.
func TestStreetNodesMoveTheirNumbers(t *testing.T) {
	cfg := content.MustLoad()
	own := func(ids ...string) (*game.World, *territory.Sim) {
		w, s := world(t, cfg)
		w.Upgrades = map[string]bool{}
		for _, id := range ids {
			if cfg.Upgrades.Upgrade(id) == nil {
				t.Fatalf("no node %s", id)
			}
			w.Upgrades[id] = true
		}
		if err := w.Post("docks", 1); err != nil {
			t.Fatal(err)
		}
		return w, s
	}
	plain, s := own()
	drift := cfg.City.Territory.DriftDays
	base := s.RobberyChance(plain, plain.Corner("docks"))
	guarded := plain.Corner("docks")
	guarded.Enforcer = 2
	baseGuarded := s.RobberyChance(plain, guarded)
	guarded.Enforcer = 0
	cases := []struct {
		nodes   []string
		drift   int
		robbery float64
	}{
		{[]string{"boys"}, drift + 2, 1},
		{[]string{"watch"}, drift, 0.7},
		{[]string{"dogs"}, drift, 0.6},
		{[]string{"boys", "watch", "dogs"}, drift + 2, 0.42},
	}
	for _, tc := range cases {
		w, s := own(tc.nodes...)
		if got := s.DriftDays(w); got != tc.drift {
			t.Errorf("%v: drift %d days, want %d", tc.nodes, got, tc.drift)
		}
		c := w.Corner("docks")
		if got, want := s.RobberyChance(w, c), base*tc.robbery; math.Abs(got-want) > 1e-12 {
			t.Errorf("%v: robbery %.5f, want %.5f", tc.nodes, got, want)
		}
		c.Enforcer = 2
		if got, want := s.RobberyChance(w, c), baseGuarded*tc.robbery; math.Abs(got-want) > 1e-12 {
			t.Errorf("%v with an enforcer: robbery %.5f, want %.5f (the cut comes after the node)", tc.nodes, got, want)
		}
		c.Enforcer = 0
	}
	// An idle corner with the boys goes back to the street on the folded
	// day, not the file's.
	w, s := own("boys")
	w.Crew.Members = w.Crew.Members[1:] // Dre vanishes
	for i := 0; i < drift+1; i++ {
		if k := kinds(step(w, s)); k["CornerLost"] != 0 {
			t.Fatalf("day %d: lost the corner before %d days with the boys: %v", w.Day, drift+2, k)
		}
	}
	if k := kinds(step(w, s)); k["CornerLost"] != 1 || w.Corner("docks").Held() {
		t.Fatalf("day %d: no CornerLost on the folded day: %v", w.Day, k)
	}
}

// The tax (#231): once more than [tax] share of a city's corners are
// held (min_held at least), every corner nobody holds there pays cut of
// its trade a night in dirty cash (jittered a tenth either way on the
// tax's stream), summed as Stats.Taxed and reported per city; a corner
// a faction sets up on pays nothing; the share falling stops it; the
// city's heat and the file never move for it; and with cut at 0 nothing
// is paid and nothing emitted.
func TestTaxStopsWhenTheShareFalls(t *testing.T) {
	cfg := content.MustLoad()
	tax := cfg.City.Tax
	if !tax.On() {
		t.Fatal("the tax is boxed in the file")
	}
	w, s := world(t, cfg)
	home := w.Home()
	n := len(home.Corners)
	for i := range home.Corners {
		c := &home.Corners[i]
		c.Owner, c.Faction, c.Runner, c.Enforcer, c.Since = game.OwnerNone, "", 0, 0, 1
	}
	held := int(tax.Share*float64(n)) + 1
	for i := 0; i < held; i++ {
		c := &home.Corners[i]
		c.Owner, c.Runner = game.OwnerPlayer, 1
	}
	free := n - held
	home.Corners[n-1].Owner, home.Corners[n-1].Faction = game.OwnerRival, "rival" // a faction's corner pays nothing
	free--
	// The table: with a faction still on the ground the city is not
	// yours, whatever the share (an early land grab pays nothing); with
	// every faction gone it is.
	w.Rivals = []*game.RivalState{{Leader: "Sal", Arrived: 1}}
	if s.HoldsTheCity(w, home.ID) {
		t.Fatal("the tax holds with a faction alive at the table")
	}
	if k := kinds(step(w, s)); k["Taxed"] != 0 || w.Stats.Taxed != 0 {
		t.Fatalf("taxed with a faction alive: %v", k)
	}
	w.Rivals[0].Fragmented = 1
	if !s.HoldsTheCity(w, home.ID) {
		t.Fatalf("the share does not hold with %d of %d and the table gone", held, n)
	}
	due, amount := s.TaxDue(w, home.ID)
	if due != free || amount <= 0 {
		t.Fatalf("due %d corners for %d, want %d", due, amount, free)
	}
	heat, file, cash := home.Heat, w.Heat.Evidence, w.Player.DirtyCash
	evs := step(w, s)
	var taxed *events.Taxed
	for _, e := range evs {
		if ev, ok := e.(events.Taxed); ok {
			taxed = &ev
		}
	}
	if taxed == nil || taxed.City != home.ID || taxed.Corners != free || taxed.Amount <= 0 {
		t.Fatalf("the tax: %+v", taxed)
	}
	lo, hi := int(float64(amount)*0.88), int(float64(amount)*1.12)+1
	if taxed.Amount < lo || taxed.Amount > hi || w.Player.DirtyCash != cash+taxed.Amount || w.Stats.Taxed != taxed.Amount {
		t.Fatalf("paid %d for a due of %d, cash %d -> %d, taxed %d", taxed.Amount, amount, cash, w.Player.DirtyCash, w.Stats.Taxed)
	}
	if home.Heat != heat || w.Heat.Evidence != file {
		t.Fatalf("the tax moved the heat or the file: %.1f -> %.1f, %d -> %d", heat, home.Heat, file, w.Heat.Evidence)
	}
	for _, e := range evs {
		if _, ok := e.(events.PlayerSold); ok {
			t.Fatal("the tax is a sale")
		}
	}
	// The same night on the same seed pays the same: the jitter is the
	// tax's own stream.
	again, _ := world(t, cfg)
	again.Cities = w.Cities // not a copy: the world above is stepped no further
	// The share falls: one corner drifts off you, and nobody pays.
	home.Corners[0].Owner, home.Corners[0].Runner = game.OwnerNone, 0
	if s.HoldsTheCity(w, home.ID) {
		t.Fatal("the share holds with one under it")
	}
	taxedBefore := w.Stats.Taxed
	if k := kinds(step(w, s)); k["Taxed"] != 0 || w.Stats.Taxed != taxedBefore {
		t.Fatalf("the tax with the share fallen: %v", k)
	}
	// Boxed: nothing.
	boxed := *cfg
	boxed.City.Tax.Cut = 0
	w, s = world(t, &boxed)
	home = w.Home()
	for i := range home.Corners {
		c := &home.Corners[i]
		c.Owner, c.Faction, c.Runner, c.Enforcer, c.Since = game.OwnerPlayer, "", 1, 0, 1
	}
	home.Corners[n-1].Owner, home.Corners[n-1].Runner = game.OwnerNone, 0
	if s.HoldsTheCity(w, home.ID) {
		t.Fatal("the share holds with the tax boxed")
	}
	if k := kinds(step(w, s)); k["Taxed"] != 0 || w.Stats.Taxed != 0 {
		t.Fatalf("a boxed tax paid: %v", k)
	}
}
