package harness

import (
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// houseOn is a house on a block of the home city for a fixture: capacity
// units, a nominal price and rent, open from the start.
func houseOn(w *game.World, id, corner string, capacity int) game.HouseOffer {
	return game.HouseOffer{ID: id, Name: "House " + id, City: w.Home().ID, Corner: corner, Capacity: capacity, Price: 1, Rent: 1}
}

// rented puts houses on the home city's blocks and fills them from a
// pile of units, house-first as AddStock lands it, for a fixture that
// wants the stock spread. The world is quiet (no corners, so nothing
// sells) and rich.
func rented(t *testing.T, cfg *content.Config, seed uint64, units int, capacities ...int) *game.World {
	t.Helper()
	w := quiet(cfg, seed)
	w.Player.DirtyCash, w.Player.CleanCash = 1_000_000, 1_000_000 // the rent is nobody's business here
	blocks := []string{"precinct", "docks", "strip", "heights", "riverside", "oldmill"}
	for i, c := range capacities {
		if _, err := w.BuyHouse(houseOn(w, "h"+string(rune('a'+i)), blocks[i%len(blocks)], c)); err != nil {
			t.Fatal(err)
		}
	}
	w.Today.HousesBought = nil
	w.AddStock(w.Home().ID, cfg.Market.Products[0].ID, units)
	return w
}

// raidTonight has the home city's police raid tonight whatever the
// day's decay does, and the sting line under it so the raid is the one
// that fires.
func raidTonight(w *game.World) { w.Home().Heat = 100 }

// enforcement is the sting or raid the run fired, or nil.
func enforcement(res Result) *events.Enforcement {
	for _, e := range res.Events {
		if ev, ok := e.(events.Enforcement); ok && (ev.Level == content.Raid || ev.Level == content.Sting) {
			return &ev
		}
	}
	return nil
}

// A raid hits one place, not the operation (#73): with the stock in one
// house it takes stock_loss of it, with the same stock spread over three
// it takes stock_loss of one and leaves the other two untouched, so
// spreading loses a smaller fraction of the stock per raid. The
// Enforcement names the house and its StockLost is exactly what left it.
func TestRaidHitsOnePlaceSoSpreadingLosesLess(t *testing.T) {
	cfg := content.MustLoad()
	product := cfg.Market.Products[0].ID
	one, three := 0.0, 0.0
	for seed := uint64(1); seed <= 5; seed++ {
		for _, spread := range []bool{false, true} {
			var w *game.World
			if spread {
				w = rented(t, cfg, seed, 600, 200, 200, 200)
			} else {
				w = rented(t, cfg, seed, 600, 600)
			}
			home := w.Home().ID
			before := map[string]int{}
			for _, h := range w.Houses {
				before[h.ID] = h.Units()
			}
			raidTonight(w)
			res, err := RunFrom(cfg, w, 1, Idle)
			if err != nil {
				t.Fatal(err)
			}
			ev := enforcement(res)
			if ev == nil || ev.Level != content.Raid || ev.House == "" {
				t.Fatalf("seed %d spread %v: no raid on a house: %+v", seed, spread, ev)
			}
			lost := ev.StockLost[product]
			if lost == 0 {
				t.Fatalf("seed %d: the raid took nothing", seed)
			}
			touched := 0
			for _, h := range res.World.Houses {
				switch {
				case h.ID == ev.House:
					touched++
					if before[h.ID]-h.Units() != lost || !h.Known || h.Raided != 1 {
						t.Fatalf("seed %d: the raided house %+v lost %d, the raid says %d", seed, h, before[h.ID]-h.Units(), lost)
					}
				case h.Units() != before[h.ID] || h.Known:
					t.Fatalf("seed %d: %s was touched by a raid on %s: %+v", seed, h.Name, ev.HouseName, h)
				}
			}
			if touched != 1 || res.World.Street(home, product) != 0 {
				t.Fatalf("seed %d: %d houses touched, street %d", seed, touched, res.World.Street(home, product))
			}
			frac := float64(lost) / 600
			if spread {
				three += frac
			} else {
				one += frac
			}
		}
	}
	one, three = one/5, three/5
	t.Logf("fraction of the stock a raid takes: one house %.2f, three houses %.2f", one, three)
	if three >= one/2 {
		t.Fatalf("spreading over three houses loses %.2f of the stock a raid against %.2f in one", three, one)
	}
}

// The informant's raid is specific (#13's promise, #73): with an
// informant on the payroll the raid goes to the fullest house, which the
// police now know about (HouseCompromised, informant), takes the whole
// of it and nothing from the others. A raid on a known house is the
// next raid's target too.
func TestInformantRaidTakesTheWholeKnownHouse(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Market.Market.ShockChance, cfg.Market.Market.SlumpChance = 0, 0
	product := cfg.Market.Products[0].ID
	for seed := uint64(1); seed <= 3; seed++ {
		w := rented(t, cfg, seed, 500, 300, 100, 100)
		// h1 300 (the most room first), then the rest into b and c.
		full := w.Fullest(w.Home().ID, false)
		if full == nil {
			t.Fatal("nothing stashed")
		}
		Plant(cfg, w)
		raidTonight(w)
		before := map[string]int{}
		for _, h := range w.Houses {
			before[h.ID] = h.Units()
		}
		res, err := RunFrom(cfg, w, 1, Idle)
		if err != nil {
			t.Fatal(err)
		}
		ev := enforcement(res)
		if ev == nil || ev.Level != content.Raid || !ev.Stash || ev.House != full.ID || ev.StockLost[product] != before[full.ID] {
			t.Fatalf("seed %d: the informant's raid: %+v, the fullest house %s held %d", seed, ev, full.ID, before[full.ID])
		}
		told := false
		for _, e := range res.Events {
			if c, ok := e.(events.HouseCompromised); ok {
				if c.House != full.ID || c.Why != "informant" {
					t.Fatalf("seed %d: compromised %+v", seed, c)
				}
				told = true
			}
		}
		if !told {
			t.Fatalf("seed %d: no HouseCompromised", seed)
		}
		for _, h := range res.World.Houses {
			if h.ID == full.ID {
				if h.Units() != 0 || !h.Known {
					t.Fatalf("seed %d: the known house after: %+v", seed, h)
				}
			} else if h.Units() != before[h.ID] || h.Known {
				t.Fatalf("seed %d: another house was touched: %+v", seed, h)
			}
		}
		if res.World.Stock(w.Home().ID, product) != 500-before[full.ID] {
			t.Fatalf("seed %d: stock after %d", seed, res.World.Stock(w.Home().ID, product))
		}
	}
	// With no house at all it is the whole street, as it always was
	// (TestInformantRaidAndFiring pins that too).
	w := quiet(cfg, 1)
	w.Player.DirtyCash = 100_000
	w.SetStock(w.Home().ID, product, 80)
	Plant(cfg, w)
	raidTonight(w)
	res, err := RunFrom(cfg, w, 1, Idle)
	if err != nil {
		t.Fatal(err)
	}
	if ev := enforcement(res); ev == nil || !ev.Stash || ev.House != "" || ev.StockLost[product] != 80 {
		t.Fatalf("the informant's raid with no house: %+v", ev)
	}
}

// A cheap empty house never shields the street (#73): the raid's roll is
// over the places holding stock, the street among them, so with nothing
// in the house the street is hit as if there were no house, and with a
// unit in a house on a hot block the street is still hit on some seeds.
func TestDecoyHouseNeverShieldsTheStreet(t *testing.T) {
	cfg := content.MustLoad()
	product := cfg.Market.Products[0].ID
	street, house := 0, 0
	for seed := uint64(1); seed <= 12; seed++ {
		// An empty house: the street every time.
		w := rented(t, cfg, seed, 0, 400)
		w.SetStock(w.Home().ID, product, 200)
		raidTonight(w)
		res, err := RunFrom(cfg, w, 1, Idle)
		if err != nil {
			t.Fatal(err)
		}
		if ev := enforcement(res); ev == nil || ev.House != "" || ev.StockLost[product] != 100 {
			t.Fatalf("seed %d: with an empty decoy the raid hit %+v", seed, ev)
		}
		// A unit in a house on Precinct Row (heat 1.6) against 200 on
		// the street (weight 1): the street on some seeds, the house on
		// others.
		w = rented(t, cfg, seed, 1, 400)
		w.SetStock(w.Home().ID, product, 200)
		raidTonight(w)
		res, err = RunFrom(cfg, w, 1, Idle)
		if err != nil {
			t.Fatal(err)
		}
		ev := enforcement(res)
		if ev == nil {
			t.Fatalf("seed %d: no raid", seed)
		}
		if ev.House == "" {
			street++
		} else {
			house++
		}
	}
	t.Logf("a unit in a decoy on Precinct Row: the street hit %d times, the house %d, over 12 seeds", street, house)
	if street == 0 || house == 0 {
		t.Fatalf("the roll is not a roll: street %d house %d", street, house)
	}
}

// A guard is worth its wage (#73): a house on the Docks with a skill-90
// enforcer inside is robbed materially less often than one without over
// 200 days and five seeds, and an enforcer cannot guard a house and a
// corner at once.
func TestGuardIsWorthItsWage(t *testing.T) {
	cfg := content.MustLoad()
	product := cfg.Market.Products[0].ID
	robbed := map[bool]int{}
	for seed := uint64(1); seed <= 5; seed++ {
		for _, guarded := range []bool{false, true} {
			w := rented(t, cfg, seed, 0, 400)
			w.Houses[0].Corner = "docks" // risk 1.6: the robbers' block
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 700, Name: "Moose", Role: "enforcer", Skill: 90, Loyalty: 100, Nerve: 80, Wage: 1})
			if guarded {
				if err := w.Guard(w.Houses[0].ID, 700); err != nil {
					t.Fatal(err)
				}
			}
			res, err := RunFrom(cfg, w, Horizon, func(w *game.World) {
				// Something to steal every night, and nothing on the street.
				w.SetStock(w.Home().ID, product, 0)
				if h := w.House("ha"); h != nil && h.Units() < 300 {
					w.MoveStock(w.Home().ID, game.Street, "ha", product, 0)
					w.AddStock(w.Home().ID, product, 300-h.Units())
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range res.Events {
				if ev, ok := e.(events.HouseRobbed); ok {
					if ev.Guarded != guarded || ev.House != "ha" {
						t.Fatalf("seed %d guarded %v: %+v", seed, guarded, ev)
					}
					robbed[guarded]++
				}
			}
			if guarded && res.World.House("ha").Guard != 700 {
				t.Fatalf("seed %d: the guard left the house", seed)
			}
		}
	}
	t.Logf("robberies of a house on the Docks over %d days and 5 seeds: unguarded %d, guarded %d", Horizon, robbed[false], robbed[true])
	if robbed[false] < 5 || robbed[true]*2 >= robbed[false] {
		t.Fatalf("the guard is not worth the wage: unguarded %d, guarded %d", robbed[false], robbed[true])
	}
	// One enforcer, one job.
	w := rented(t, cfg, 1, 0, 400)
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 700, Name: "Moose", Role: "enforcer", Skill: 90})
	if err := w.Guard("ha", 700); err != nil {
		t.Fatal(err)
	}
	if err := w.Post(w.Home().Corners[0].ID, 700); err != nil {
		t.Fatal(err)
	}
	if w.House("ha").Guard != 0 || w.PostOf(700) == nil {
		t.Fatal("an enforcer guards a house and a corner at once")
	}
	if err := w.Guard("ha", 700); err != nil || w.PostOf(700) != nil || w.House("ha").Guard != 700 {
		t.Fatal("guarding the house did not take them off the corner")
	}
}

// A robbery makes a house known (#73): the next raid finds it whatever
// the roll would have said.
func TestRobberyMakesTheHouseKnown(t *testing.T) {
	cfg := content.MustLoad()
	product := cfg.Market.Products[0].ID
	w := rented(t, cfg, 3, 400, 200, 200)
	// The second house is known, as a robbery there would leave it.
	w.Houses[1].Known = true
	raidTonight(w)
	res, err := RunFrom(cfg, w, 1, Idle)
	if err != nil {
		t.Fatal(err)
	}
	if ev := enforcement(res); ev == nil || ev.House != w.Houses[1].ID || ev.StockLost[product] != 100 {
		t.Fatalf("the raid did not find the known house: %+v", ev)
	}
}

// Rent is a laundering pull (#73): the stashed player with no fronts and
// what clean cash it started with runs it out and the landlord throws it
// out within 100 days; the same player with fronts keeps every house it
// takes. The numbers are logged for the PR.
func TestRentIsALaunderingPull(t *testing.T) {
	cfg := content.MustLoad()
	lost := map[bool]int{}
	held := map[bool]int{}
	rent := map[bool]int{}
	for seed := uint64(1); seed <= 5; seed++ {
		for _, fronts := range []bool{false, true} {
			w := sim.NewWorld(cfg, seed)
			w.Player.CleanCash = 5_000 // enough to take the first house, not to keep it
			res, err := RunFrom(cfg, w, 100, Stashed(cfg, 40, StashHouses, fronts))
			if err != nil {
				t.Fatal(err)
			}
			lost[fronts] += res.World.Stats.HousesLost
			held[fronts] += len(res.World.Houses)
			rent[fronts] += res.World.Stats.Rent
			if !fronts && res.World.Player.CleanCash > 4_000 && res.World.Stats.HousesLost == 0 && len(res.World.Houses) > 0 {
				t.Fatalf("seed %d: no fronts, clean cash %d, %d houses kept, none lost", seed, res.World.Player.CleanCash, len(res.World.Houses))
			}
		}
	}
	t.Logf("over 100 days and 5 seeds: no fronts lost %d houses, holds %d, paid $%d rent; with fronts lost %d, holds %d, paid $%d", lost[false], held[false], rent[false], lost[true], held[true], rent[true])
	if lost[false] == 0 {
		t.Fatal("the stashed player with no fronts kept every house for 100 days")
	}
	if lost[true] > 0 || held[true] < 5 {
		t.Fatalf("the stashed player with fronts lost %d houses and holds %d", lost[true], held[true])
	}
}

// TestHouseInvariants runs the stashed player, spread and in one house,
// and pins on every day: TotalStock is every street plus every house
// plus the road and nothing else; no house over its capacity; a house
// known stays known until it is dropped; a guard is an enforcer on no
// corner; a house robbery or raid took its stock from that one house
// (the event's StockLost is at most what it held).
func TestHouseInvariants(t *testing.T) {
	cfg := content.MustLoad()
	for _, houses := range []int{StashHouses, 1} {
		w := sim.NewWorld(cfg, 4)
		pol := Stashed(cfg, 40, houses, true)
		known := map[string]bool{}
		check := func(w *game.World) {
			street, housed, road := 0, 0, 0
			for _, cid := range w.CityOrder {
				street += w.Player.StockIn(cid)
			}
			for _, h := range w.Houses {
				housed += h.Units()
				if h.Units() > h.Capacity {
					t.Fatalf("day %d: %s holds %d of %d", w.Day, h.Name, h.Units(), h.Capacity)
				}
				if known[h.ID] && !h.Known {
					t.Fatalf("day %d: %s forgot it was known", w.Day, h.Name)
				}
				if h.Known {
					known[h.ID] = true
				}
				if h.Guard != 0 {
					if m := w.Crew.Member(h.Guard); m == nil || m.Role != "enforcer" || w.PostOf(h.Guard) != nil {
						t.Fatalf("day %d: the guard of %s is %+v, posted %v", w.Day, h.Name, m, w.PostOf(h.Guard))
					}
				}
			}
			for _, s := range w.Shipments {
				road += s.Units
			}
			if w.TotalStock() != street+housed+road || w.Stashed() != street+housed {
				t.Fatalf("day %d: total %d, street %d + houses %d + road %d", w.Day, w.TotalStock(), street, housed, road)
			}
		}
		res, err := RunFrom(cfg, w, 120, func(w *game.World) {
			check(w)
			pol(w)
			check(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		check(res.World)
		for _, e := range res.Events {
			switch ev := e.(type) {
			case events.HouseRobbed:
				if sum(ev.StockLost) == 0 {
					t.Fatalf("a robbery of %s took nothing", ev.Name)
				}
			case events.Enforcement:
				if ev.House != "" && ev.HouseName == "" {
					t.Fatalf("a raid names the house id and not its name: %+v", ev)
				}
			}
		}
	}
}

func sum(m map[string]int) int {
	n := 0
	for _, q := range m {
		n += q
	}
	return n
}

// The stashed player is deterministic and survives a save (#73): the
// same seed plays the same, and a run saved on day 50 and played on is
// the run that never stopped, houses, stock, known marks and guards
// included.
func TestStashedIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pol := Stashed(cfg, 40, StashHouses, true)
	a, _ := Run(cfg, 3, 100, pol)
	b, _ := Run(cfg, 3, 100, pol)
	if a.NetWorthAt(100) != b.NetWorthAt(100) || len(a.World.Houses) != len(b.World.Houses) {
		t.Fatalf("the same seed diverged: %d / %d", a.NetWorthAt(100), b.NetWorthAt(100))
	}
	if len(a.World.Houses) == 0 {
		t.Fatal("the stashed player bought no house in 100 days")
	}
	c, _ := Run(cfg, 3, 50, pol)
	if err := game.Save(1, c.World); err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Houses) != len(c.World.Houses) {
		t.Fatalf("the save lost houses: %d / %d", len(loaded.Houses), len(c.World.Houses))
	}
	for i, h := range loaded.Houses {
		o := c.World.Houses[i]
		if h.ID != o.ID || h.Units() != o.Units() || h.Known != o.Known || h.Guard != o.Guard {
			t.Fatalf("house %d after the save: %+v / %+v", i, h, o)
		}
	}
	d, _ := RunFrom(cfg, loaded, 50, pol)
	if a.NetWorthAt(100) != d.NetWorthAt(50) || a.World.Stashed() != d.World.Stashed() {
		t.Fatalf("the saved run diverged: %d / %d, stashed %d / %d", a.NetWorthAt(100), d.NetWorthAt(50), a.World.Stashed(), d.World.Stashed())
	}
}

// TestStashedNumbers logs the balance the issue's acceptance asks for:
// the stashed player spread over StashHouses against one house and
// against laundered at the tier days, the rent each paid and what each
// lost out of the houses, over five seeds.
func TestStashedNumbers(t *testing.T) {
	cfg := content.MustLoad()
	type row struct {
		name string
		pol  Policy
	}
	rows := []row{
		{"stashed", Stashed(cfg, 40, StashHouses, true)},
		{"stashed -houses 1", Stashed(cfg, 40, 1, true)},
		{"laundered", Laundered(cfg, 40)},
	}
	worth := map[string][]int{}
	for _, r := range rows {
		var at70, at120 []int
		rent, lost, raided := 0, 0, 0
		for seed := uint64(1); seed <= 5; seed++ {
			res, err := Run(cfg, seed, 120, r.pol)
			if err != nil {
				t.Fatal(err)
			}
			at70 = append(at70, res.NetWorthAt(70))
			at120 = append(at120, res.NetWorthAt(120))
			rent += res.World.Stats.Rent
			lost += res.World.Stats.HouseUnits
			for _, e := range res.Events {
				if ev, ok := e.(events.Enforcement); ok && ev.House != "" {
					raided += sum(ev.StockLost)
				}
			}
		}
		sort.Ints(at70)
		sort.Ints(at120)
		worth[r.name] = []int{at70[2], at120[2]}
		t.Logf("%-18s day 70 $%d, day 120 $%d (medians); rent $%d, %d units lost out of the houses, %d of them to the police, over 5 seeds", r.name, at70[2], at120[2], rent/5, lost/5, raided/5)
	}
	if s, l := worth["stashed"][0], worth["laundered"][0]; s < l*8/10 {
		t.Fatalf("stashed at day 70 is $%d against laundered's $%d: the houses cost more than a fifth", s, l)
	}
}
