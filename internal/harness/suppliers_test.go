package harness

import (
	"bytes"
	"encoding/gob"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// The connects' invariants hold on every day of every policy (#72):
// the relationship in 0..100 and the debt never negative; a frozen
// connect sells nothing, to you or to the road; a day's buys from one
// connect, yours, a contract's and the road's together, never exceed
// its capacity; and cash and stock are conserved across every credit
// buy (the receipt is on the book, the till untouched). The book is
// read every morning, when the day's buys are on it and before the
// market closes it.
func TestSupplierInvariants(t *testing.T) {
	cfg := content.MustLoad()
	policies := map[string]Policy{
		"crewed":      Crewed(cfg, 40),
		"leveraged":   Leveraged(cfg, 40),
		"distributor": Distributor(cfg, 40),
		"boss":        Boss(cfg, 40, ""),
		"stocked":     Stocked(cfg, 40),
		"dealer":      Dealer(cfg, 40),
		"aggressive":  Trader(cfg, events.DialAggressive),
	}
	for name, pol := range policies {
		for seed := uint64(1); seed <= 3; seed++ {
			check := func(w *game.World) {
				for i := range w.Suppliers {
					s := &w.Suppliers[i]
					if s.Rel < 0 || s.Rel > 100 || s.Debt < 0 {
						t.Fatalf("%s seed %d day %d: %s rel %.1f debt %d", name, seed, w.Day, s.Name, s.Rel, s.Debt)
					}
					if s.BoughtToday > s.Cap {
						t.Fatalf("%s seed %d day %d: %s sold %d of %d", name, seed, w.Day, s.Name, s.BoughtToday, s.Cap)
					}
					if s.Frozen(w.Day) && s.BoughtToday > 0 {
						t.Fatalf("%s seed %d day %d: %s sold %d while frozen", name, seed, w.Day, s.Name, s.BoughtToday)
					}
				}
				for _, b := range w.Today.Buys {
					if b.Credit && (b.Contract || w.Supplier(b.Supplier) == nil) {
						t.Fatalf("%s seed %d day %d: a credit receipt from nowhere: %+v", name, seed, w.Day, b)
					}
				}
				pol(w)
			}
			res, err := Run(cfg, seed, 120, check)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range res.Events {
				switch ev := e.(type) {
				case events.WholesaleBought:
					if s := res.World.Supplier(ev.Supplier); s == nil || !s.Wholesale || ev.Units != ev.Lots*s.Lot {
						t.Fatalf("%s seed %d: the road bought %+v", name, seed, ev)
					}
				case events.SupplierBought:
					if ev.Cost != int(math.Ceil(ev.Price*float64(ev.Units))) {
						t.Fatalf("%s seed %d: a receipt that does not add up: %+v", name, seed, ev)
					}
				}
			}
		}
	}
}

// Credit is a real lever and a real risk (#72): the leveraged player's
// median net worth at day 60 is above the crewed one's (the early game
// is capital-starved, which is the point) and at the horizon it is not
// above it by more than the tier band allows (leverage buys time, not
// free money); and over twenty seeds it misses at least one payment.
// With credit withdrawn it is the crewed player exactly.
func TestLeveragedIsALeverNotFreeMoney(t *testing.T) {
	cfg := content.MustLoad()
	var early, late, crewEarly, crewLate []int
	missed := 0
	for seed := uint64(1); seed <= 20; seed++ {
		l, _ := Run(cfg, seed, Horizon, Leveraged(cfg, 40))
		c, _ := Run(cfg, seed, Horizon, Crewed(cfg, 40))
		early, late = append(early, l.NetWorthAt(60)), append(late, l.NetWorthAt(Horizon))
		crewEarly, crewLate = append(crewEarly, c.NetWorthAt(60)), append(crewLate, c.NetWorthAt(Horizon))
		missed += l.World.Stats.LatePayments
		if l.World.Stats.Credit == 0 {
			t.Fatalf("seed %d: the leveraged player never took credit", seed)
		}
	}
	for _, s := range [][]int{early, late, crewEarly, crewLate} {
		sort.Ints(s)
	}
	t.Logf("day 60 median net worth: leveraged %d, crewed %d; day %d: leveraged %d, crewed %d; %d late payments over 20 seeds",
		early[10], crewEarly[10], Horizon, late[10], crewLate[10], missed)
	if early[10] <= crewEarly[10] {
		t.Fatalf("day 60: leveraged %d is not above crewed %d", early[10], crewEarly[10])
	}
	band := 2_000_000.0 / 500_000.0 // tier 2's band, the crewed player's
	if float64(late[10]) > float64(crewLate[10])*band {
		t.Fatalf("day %d: leveraged %d is above crewed %d by more than the band's %.0fx", Horizon, late[10], crewLate[10], band)
	}
	if missed == 0 {
		t.Fatal("twenty leveraged runs and never a payment missed: the deadline does not bite")
	}
	off := NoCredit(cfg)
	a, _ := Run(off, 3, 60, Leveraged(off, 40))
	b, _ := Run(off, 3, 60, Crewed(off, 40))
	if a.NetWorthAt(60) != b.NetWorthAt(60) || a.World.Stats.Credit != 0 {
		t.Fatalf("with credit off the leveraged player made %d, the crewed one %d (credit %d)", a.NetWorthAt(60), b.NetWorthAt(60), a.World.Stats.Credit)
	}
}

// No debt ever ends a run (#72), TestRichHiderIsNeverIndicted's
// sibling: a player who owes every connect a fortune and has nothing,
// hiding, is still free at the horizon whatever the tempers do, and
// the leveraged player's endings are the police's, never the connects'.
func TestDebtNeverEndsTheRun(t *testing.T) {
	cfg := content.MustLoad()
	for _, temper := range content.Tempers {
		for seed := uint64(1); seed <= 3; seed++ {
			w := sim.NewWorld(cfg, seed)
			// Nothing in the till, a stash nobody sells (broke is the
			// old ending, for a player with nothing at all), and every
			// connect owed more than a day brings.
			w.Player.DirtyCash, w.Player.CleanCash = 0, 0
			w.SetStock(w.Home().ID, w.Products[0], 100_000)
			for i := range w.Suppliers {
				s := &w.Suppliers[i]
				s.Temper = temper
				s.Debt, s.DebtDue = 1_000, w.Day+1
			}
			w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Tank", Role: "enforcer", Skill: 60, Loyalty: 80, Nerve: 10, Wage: 0})
			res, err := RunFrom(cfg, w, Horizon, Hide)
			if err != nil {
				t.Fatal(err)
			}
			if res.Over != nil {
				t.Fatalf("%s seed %d: owing a fortune the hider ended on day %d: %s", temper, seed, res.Days, res.Over.Cause)
			}
			late, frozen, collected := 0, 0, 0
			for _, e := range res.Events {
				switch e.(type) {
				case events.DebtLate:
					late++
				case events.SupplierFrozen:
					frozen++
				case events.SupplierCollected:
					collected++
				}
			}
			if late == 0 {
				t.Fatalf("%s seed %d: a fortune owed and never late", temper, seed)
			}
			switch temper {
			case "sharp", "patient":
				if frozen == 0 {
					t.Fatalf("%s seed %d: never frozen out over %d late payments", temper, seed, late)
				}
			case "connected":
				if collected == 0 {
					t.Fatalf("%s seed %d: nobody ever came round over %d late payments", temper, seed, late)
				}
			}
			for _, s := range res.World.Suppliers {
				if s.Debt < 0 || s.Rel < 0 {
					t.Fatalf("%s seed %d: %+v", temper, seed, s)
				}
			}
		}
	}
	for seed := uint64(1); seed <= 5; seed++ {
		res, _ := Run(cfg, seed, Horizon, Leveraged(cfg, 40))
		if res.Over != nil && res.Over.Cause != "arrested" && res.Over.Cause != "indicted" {
			t.Fatalf("seed %d: the leveraged run ended: %s", seed, res.Over.Cause)
		}
	}
}

// The relationship pays (#72): buying lots for sixty days lowers the
// price the crewed player pays a unit against street on every seed, and
// respect (#14) still pulls its way on top of it.
func TestRelationshipPays(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 5; seed++ {
		// What the street connect charges against street each morning,
		// before the day's buys nudge it: the price paid a unit.
		var paid []float64
		crewed := Crewed(cfg, 40)
		res, _ := Run(cfg, seed, 60, func(w *game.World) {
			home := w.Home().ID
			if sup, m := w.StreetSupplier(home), w.Product(home, w.Products[0]); sup != nil && m != nil && m.Price > 0 {
				paid = append(paid, sup.Price[w.Products[0]]/m.Price)
			}
			crewed(w)
		})
		mean := func(vs []float64) float64 {
			s := 0.0
			for _, v := range vs {
				s += v
			}
			return s / float64(len(vs))
		}
		first, last := mean(paid[:10]), mean(paid[len(paid)-10:])
		if last >= first {
			t.Fatalf("seed %d: paid %.4f of street over the first ten days and %.4f over the last ten", seed, first, last)
		}
		street := res.World.StreetSupplier(res.World.Home().ID)
		if street.Band <= cfg.Suppliers.Suppliers.Neutral() {
			t.Fatalf("seed %d: sixty days of lots left %s in band %d (rel %.0f)", seed, street.Name, street.Band, street.Rel)
		}
		mk := newMarket(cfg)
		if got, base := mk.SupplierRatio(res.World, street), cfg.Market.Market.SupplierRatio; got >= base {
			t.Fatalf("seed %d: after sixty days the connect charges %.4f of street, the flat ratio is %.4f", seed, got, base)
		}
		// Respect on top: the same connect, the same band, a lower
		// price.
		plain := mk.SupplierRatio(res.World, street)
		res.World.Player.Reputation.Respect = 100
		if r := mk.SupplierRatio(res.World, street); r >= plain {
			t.Fatalf("seed %d: respected and still paying %.4f (%.4f before)", seed, r, plain)
		}
	}
}

// A bust that loses stock lowers the connect's relationship in that
// city the next morning, a seizure on the road lowers the wholesaler's,
// and at the floor the connect freezes while the other connect in the
// city still sells (#72).
func TestBustsAndSeizuresHurtTheConnects(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 4)
	home, hub := w.CityOrder[0], w.CityOrder[1]
	street := w.StreetSupplier(home)
	w.Player.DirtyCash, w.Stats.PeakCash = 5_000_000, 5_000_000
	w.SetStock(home, w.Products[0], 500)
	// A raid tomorrow: the heat sim fires one at heat over the line on a
	// day something sold; force it by the record instead, which is what
	// the market reads.
	rel := street.Rel
	res, _ := RunFrom(cfg, w, 1, func(w *game.World) {
		w.Heat.Busts = append(w.Heat.Busts, game.Bust{Day: w.Day, City: home, Level: content.Raid, Units: 10 * street.Lot})
	})
	if street = res.World.StreetSupplier(home); street.Rel >= rel {
		t.Fatalf("a raid that took ten lots left %s at rel %.1f (was %.1f)", street.Name, street.Rel, rel)
	}
	// A seizure: a shipment on a road that always seizes.
	risky := *cfg
	risky.Routes.Routes = append([]content.RouteConfig(nil), cfg.Routes.Routes...)
	for i := range risky.Routes.Routes {
		risky.Routes.Routes[i].Risk = 1
	}
	w = sim.NewWorld(&risky, 4)
	w.Player.DirtyCash, w.Stats.PeakCash = 5_000_000, 5_000_000
	whole := w.WholesaleSupplier(hub)
	rel = whole.Rel
	route := risky.Routes.Routes[0]
	_ = w.SetRoute(route.ID, events.RouteNormal)
	_ = w.SetRouteTarget(route.ID, w.Products[0], 300)
	res, _ = RunFrom(&risky, w, 6, Idle)
	if res.World.Stats.Seizures == 0 {
		t.Fatal("no seizure on a road that always seizes")
	}
	if whole = res.World.WholesaleSupplier(hub); whole.Rel >= rel {
		t.Fatalf("a seizure out of %s left %s at rel %.1f (was %.1f)", hub, whole.Name, whole.Rel, rel)
	}
	// At the floor: frozen, the road buys nothing, and the street connect
	// in the same city still sells by hand.
	w = sim.NewWorld(cfg, 4)
	w.Player.DirtyCash, w.Stats.PeakCash = 5_000_000, 5_000_000
	w.WholesaleSupplier(hub).Rel = 0
	_ = w.SetRoute(route.ID, events.RouteNormal)
	_ = w.SetRouteTarget(route.ID, w.Products[0], 300)
	res, _ = RunFrom(cfg, w, 3, Idle)
	for _, e := range res.Events {
		if _, ok := e.(events.WholesaleBought); ok {
			t.Fatal("the road bought from a frozen wholesaler")
		}
	}
	if !res.World.WholesaleSupplier(hub).Frozen(res.World.Day) {
		t.Fatal("the wholesaler at rel 0 is taking calls")
	}
	_ = res.World.Travel(hub)
	if _, err := res.World.Buy(res.World.StreetSupplier(hub).ID, res.World.Products[0], 10, false, 0); err != nil {
		t.Fatalf("the street connect in %s would not sell: %v", hub, err)
	}
}

// The connects survive a save with debt in flight (#72): a leveraged
// run saved and loaded plays on as one that never stopped, and a
// schema-10 save gets one connect a city at today's price and plays on.
func TestSaveMigratesTheSuppliers(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pol := Leveraged(cfg, 40)
	a, _ := Run(cfg, 2, 60, pol)
	b, _ := Run(cfg, 2, 30, pol)
	if b.World.Owed() == 0 {
		t.Fatal("no debt in flight on day 30 to save")
	}
	if err := game.Save(1, b.World); err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := RunFrom(cfg, loaded, 30, pol)
	if a.World.Cash() != c.World.Cash() || a.World.Owed() != c.World.Owed() || a.NetWorthAt(60) != c.NetWorthAt(30) {
		t.Fatalf("the saved run diverged: cash %d/%d owed %d/%d worth %d/%d", a.World.Cash(), c.World.Cash(), a.World.Owed(), c.World.Owed(), a.NetWorthAt(60), c.NetWorthAt(30))
	}
	// A schema-10 save: the world as it was before the connects, the
	// Suppliers field absent from the stream. gob leaves a nil slice out,
	// so the same World type with Suppliers nil is that save.
	old, _ := Run(cfg, 5, 20, Crewed(cfg, 40))
	prices := map[string]float64{}
	for _, cid := range old.World.CityOrder {
		for id, m := range old.World.Cities[cid].Market {
			prices[cid+"/"+id] = m.SupplierPrice
		}
	}
	old.World.Suppliers = nil
	old.World.SchemaVersion = 10
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(old.World); err != nil {
		t.Fatal(err)
	}
	p, _ := game.SavePath(2)
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := game.Load(2); err == nil {
		t.Fatal("a schema-10 save loaded without a migration")
	}
	got, err := game.Load(2, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != game.SchemaVersion || len(got.Suppliers) != 3 {
		t.Fatalf("migrated: schema %d, %d connects", got.SchemaVersion, len(got.Suppliers))
	}
	for _, cid := range got.CityOrder {
		street := got.StreetSupplier(cid)
		if street == nil {
			t.Fatalf("%s has no street connect after the migration", cid)
		}
		for id, m := range got.Cities[cid].Market {
			if m.NoSupply {
				continue
			}
			if street.Price[id] != prices[cid+"/"+id] || m.SupplierPrice != prices[cid+"/"+id] {
				t.Fatalf("%s: %s priced at %.4f after the migration, saved at %.4f (market %.4f)", cid, id, street.Price[id], prices[cid+"/"+id], m.SupplierPrice)
			}
		}
	}
	on, err := RunFrom(cfg, got, 10, Crewed(cfg, 40))
	if err != nil || on.Over != nil || on.World.Day != 30 {
		t.Fatalf("the migrated save did not play on: %v %v day %d", err, on.Over, on.World.Day)
	}
}

// newMarket is the market sim on its own, for reading what a connect
// charges.
func newMarket(cfg *content.Config) *market.Sim {
	mk, err := market.New(cfg)
	if err != nil {
		panic(err)
	}
	return mk
}
