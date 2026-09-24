package heat_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
)

// Targeted investigations (#343): the police who would send the sting
// name the biggest source of the city's heat instead, and lead_days
// nights later the sting lands on that target alone.

// blind is cfg with investigations off: the blind sting every test of
// the ladder before #343 pins.
func blind(cfg *content.Config) *content.Config {
	c := *cfg
	c.Heat.Investigation.Enabled = false
	return &c
}

// investigating is cfg with investigations on, whatever the file ships.
func investigating(cfg *content.Config) *content.Config {
	c := *cfg
	c.Heat.Investigation.Enabled = true
	return &c
}

// opened is the tick's InvestigationOpened, or nil.
func opened(tk *game.Tick) *events.InvestigationOpened {
	for _, e := range tk.Events() {
		if ev, ok := e.(events.InvestigationOpened); ok {
			return &ev
		}
	}
	return nil
}

// closedOf is the tick's InvestigationClosed, or nil.
func closedOf(tk *game.Tick) *events.InvestigationClosed {
	for _, e := range tk.Events() {
		if ev, ok := e.(events.InvestigationClosed); ok {
			return &ev
		}
	}
	return nil
}

// freeCorner is a corner in the home city other than the first that
// nobody holds, for the player to walk to.
func freeCorner(t *testing.T, w *game.World) *game.Corner {
	t.Helper()
	for i := range w.Home().Corners {
		if c := &w.Home().Corners[i]; i > 0 && c.Owner == game.OwnerNone {
			return c
		}
	}
	t.Fatal("no free corner at home")
	return nil
}

// The opening is an argmax over the window, no dice: the biggest source
// in the hottest city, a tie to the corner before the product before
// the house, another city's sources never this city's, and with
// nothing to name the blind sting as it always was.
func TestInvestigationNamesTheBiggestSource(t *testing.T) {
	cfg := investigating(content.MustLoad())
	s := heat.New(cfg)
	sting := rung(cfg, content.Sting)
	for _, tc := range []struct {
		name  string
		trail func(home, hub string) map[string]float64
		kind  string
		want  string // the target; "" is the blind sting
	}{
		{"one corner", func(h, _ string) map[string]float64 {
			return map[string]float64{game.LeadKey(h, game.LeadCorner, "a"): 5}
		}, game.LeadCorner, "a"},
		{"the bigger corner", func(h, _ string) map[string]float64 {
			return map[string]float64{game.LeadKey(h, game.LeadCorner, "a"): 5, game.LeadKey(h, game.LeadCorner, "b"): 8}
		}, game.LeadCorner, "b"},
		{"a product over a corner", func(h, _ string) map[string]float64 {
			return map[string]float64{game.LeadKey(h, game.LeadCorner, "a"): 5, game.LeadKey(h, game.LeadProduct, "weed"): 6}
		}, game.LeadProduct, "weed"},
		{"a tie goes to the corner", func(h, _ string) map[string]float64 {
			return map[string]float64{game.LeadKey(h, game.LeadProduct, "weed"): 5, game.LeadKey(h, game.LeadCorner, "a"): 5}
		}, game.LeadCorner, "a"},
		{"a house", func(h, _ string) map[string]float64 {
			return map[string]float64{game.LeadKey(h, game.LeadCorner, "a"): 2, game.LeadKey(h, game.LeadHouse, "safe"): 9}
		}, game.LeadHouse, "safe"},
		{"another city's lead is not this city's", func(h, hub string) map[string]float64 {
			return map[string]float64{game.LeadKey(hub, game.LeadCorner, "z"): 50, game.LeadKey(h, game.LeadCorner, "a"): 1}
		}, game.LeadCorner, "a"},
		{"nothing to name", func(_, hub string) map[string]float64 {
			return map[string]float64{game.LeadKey(hub, game.LeadCorner, "z"): 50}
		}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := world(t, cfg)
			home := w.Home()
			w.Heat.Trail = tc.trail(home.ID, w.CityOrder[1])
			home.Heat = over(w, s, sting, home)
			tk := step(w, s)
			op, ev := opened(tk), enforcement(tk)
			if tc.want == "" {
				if op != nil || ev == nil || ev.Level != content.Sting {
					t.Fatalf("nothing to name: opened %+v, enforcement %+v; want the blind sting", op, ev)
				}
				return
			}
			if op == nil || ev != nil {
				t.Fatalf("opened %+v, enforcement %+v; want an investigation and no sting", op, ev)
			}
			inv := w.Heat.Investigation
			if op.Lead != tc.kind || op.Target != tc.want || op.City != home.ID || op.Due != w.Day+cfg.Heat.Investigation.LeadDays || inv.Kind != tc.kind || inv.Target != tc.want || inv.Due != op.Due {
				t.Fatalf("opened %+v, investigation %+v; want %s %s due %d", op, inv, tc.kind, tc.want, w.Day+cfg.Heat.Investigation.LeadDays)
			}
			if got := heat.Leads(w, home.ID); len(got) == 0 || got[0].Kind != tc.kind || got[0].Target != tc.want {
				t.Fatalf("leads %+v", got)
			}
		})
	}
}

// A sale splits its heat over the corners it moved on and the window
// forgets 1/window_days of itself a night; with investigations off
// nothing is tallied.
func TestTheTrail(t *testing.T) {
	cfg := investigating(content.MustLoad())
	s := heat.New(cfg)
	w := world(t, cfg)
	home := w.Home()
	w.SetStock(home.ID, w.Products[0], 100)
	step(w, s, sale(w, home.ID, 50))
	key := game.LeadKey(home.ID, game.LeadCorner, home.Corners[0].ID)
	first := w.Heat.Trail[key]
	if first <= 0 || len(w.Heat.Trail) != 1 {
		t.Fatalf("after a sale on one corner the trail is %v", w.Heat.Trail)
	}
	step(w, s)
	keep := 1 - 1/float64(cfg.Heat.Investigation.WindowDays)
	if got := w.Heat.Trail[key]; math.Abs(got-first*keep) > 1e-9 {
		t.Fatalf("a quiet night left %.4f of %.4f; want %.4f", got, first, first*keep)
	}

	off := blind(cfg)
	w = world(t, off)
	w.SetStock(w.Home().ID, w.Products[0], 100)
	sOff := heat.New(off)
	step(w, sOff, sale(w, w.Home().ID, 50))
	w.Home().Heat = over(w, sOff, sting(off), w.Home())
	step(w, sOff, sale(w, w.Home().ID, 50))
	if w.Heat.Trail != nil || w.Heat.Investigation.Open() {
		t.Fatalf("off, the trail is %v and the investigation %+v", w.Heat.Trail, w.Heat.Investigation)
	}
}

func sting(cfg *content.Config) content.ResponseConfig { return rung(cfg, content.Sting) }

// openOnCorner opens an investigation on the first home corner, the one
// the player stands on, by selling there over the sting line, and plays
// the nights up to the one it lands on.
func openOnCorner(t *testing.T, cfg *content.Config, s *heat.Sim) *game.World {
	t.Helper()
	w := world(t, cfg)
	home := w.Home()
	w.SetStock(home.ID, w.Products[0], 1000)
	w.Player.DirtyCash = 10_000
	home.Heat = over(w, s, sting(cfg), home)
	if op := opened(step(w, s, sale(w, home.ID, 10))); op == nil || op.Target != home.Corners[0].ID {
		t.Fatalf("no investigation on the corner: %+v", op)
	}
	for w.Day+1 < w.Heat.Investigation.Due {
		home.Heat = 30 // under every line: nothing else answers
		step(w, s)
	}
	return w
}

// A target taken out of use in time files nothing and takes nothing:
// walk off the named corner and sell from another, and the night the
// investigation lands it closes on nothing, cooling nothing.
func TestSuspendedTargetFilesNothing(t *testing.T) {
	cfg := investigating(content.MustLoad())
	s := heat.New(cfg)
	w := openOnCorner(t, cfg, s)
	home := w.Home()
	if err := w.Post(freeCorner(t, w).ID, game.You); err != nil {
		t.Fatal(err)
	}
	stock, cash, file := w.Stock(home.ID, w.Products[0]), w.Player.DirtyCash, w.Heat.Evidence
	openedOn := w.Heat.Investigation.Opened
	home.Heat = 50
	tk := step(w, s, sale(w, home.ID, 10))
	cl := closedOf(tk)
	if cl == nil || cl.Hit || cl.Evidence != 0 || cl.Target != home.Corners[0].ID {
		t.Fatalf("closed %+v; want a miss on the corner", cl)
	}
	if ev := enforcement(tk); ev != nil {
		t.Fatalf("a suspended corner drew %+v", ev)
	}
	if w.Heat.Evidence != file || w.Stock(home.ID, w.Products[0]) != stock || w.Player.DirtyCash != cash || w.Heat.Investigation.Open() {
		t.Fatalf("file %d (was %d), stock %d (was %d), cash %d (was %d), investigation %+v", w.Heat.Evidence, file, w.Stock(home.ID, w.Products[0]), stock, w.Player.DirtyCash, cash, w.Heat.Investigation)
	}
	if w.Heat.LastResponse[content.Sting] != openedOn {
		t.Fatalf("the sting's cooldown runs from %v, not the night the investigation opened (%d)", w.Heat.LastResponse, openedOn)
	}

	// Lying low on the night is a suspension too: nothing sold, nothing filed.
	w = openOnCorner(t, cfg, s)
	file = w.Heat.Evidence
	tk = step(w, s)
	if cl := closedOf(tk); cl == nil || cl.Hit || w.Heat.Evidence != file {
		t.Fatalf("a quiet night: closed %+v, file %d", cl, w.Heat.Evidence)
	}
}

// A sacrifice pays: left running, the named corner takes the sting (its
// share of the street and the pile, the investigation's pages, the
// corner swept) and the hit takes closed_heat_drop off the city's heat,
// exactly: the same night with a drop of nothing reads that much hotter.
func TestSacrificeCoolsTheCity(t *testing.T) {
	cfg := investigating(content.MustLoad())
	flat := investigating(content.MustLoad())
	flat.Heat.Investigation.ClosedHeatDrop = 0
	var heats [2]float64
	for i, c := range []*content.Config{cfg, flat} {
		s := heat.New(c)
		w := openOnCorner(t, c, s)
		home := w.Home()
		home.Heat = 50
		file := w.Heat.Evidence
		tk := step(w, s, sale(w, home.ID, 10))
		cl, ev := closedOf(tk), enforcement(tk)
		pages := c.Heat.Investigation.Evidence
		if pages == 0 {
			pages = sting(c).Evidence
		}
		if cl == nil || !cl.Hit || ev == nil || ev.Level != content.Sting || ev.Evidence != pages || w.Heat.Evidence != file+pages {
			t.Fatalf("the sacrifice: closed %+v, enforcement %+v, file %d (was %d); want %d pages", cl, ev, w.Heat.Evidence, file, pages)
		}
		if len(ev.Corners) != 1 || ev.Corners[0] != home.Corners[0].ID || w.Heat.Sweep.Day != w.Day || ev.StockLost[w.Products[0]] == 0 || ev.CashLost == 0 {
			t.Fatalf("the sting on the corner: %+v, sweep %+v", ev, w.Heat.Sweep)
		}
		heats[i] = home.Heat
	}
	if drop := heats[1] - heats[0]; math.Abs(drop-cfg.Heat.Investigation.ClosedHeatDrop) > 1e-9 || drop <= 0 {
		t.Fatalf("the sacrifice cooled the city by %.2f; want closed_heat_drop %.2f", drop, cfg.Heat.Investigation.ClosedHeatDrop)
	}
}

// The product and the house: the product's street stock in the city
// goes whatever was sold, pages only if it sold that night; the house is
// raided at the raid's share, pages only on a night you dealt in its
// city; either one emptied in time is a miss.
func TestProductAndHouseHits(t *testing.T) {
	cfg := investigating(content.MustLoad())
	s := heat.New(cfg)
	product := "weed"
	due := func(w *game.World, kind, target string) {
		w.Heat.Investigation = game.Investigation{City: w.Home().ID, Kind: kind, Target: target, Opened: w.Day, Due: w.Day + 1}
	}

	w := world(t, cfg)
	home := w.Home().ID
	w.SetStock(home, product, 100)
	due(w, game.LeadProduct, product)
	tk := step(w, s, events.PlayerSold{Day: w.Day + 1, City: home, Product: product, Wanted: 10, Sold: 10, Dial: events.DialNormal})
	if cl, ev := closedOf(tk), enforcement(tk); cl == nil || !cl.Hit || ev == nil || ev.StockLost[product] != 100 || w.Street(home, product) != 0 || ev.Evidence == 0 {
		t.Fatalf("the product sold: closed %+v, enforcement %+v, street %d", cl, ev, w.Street(home, product))
	}

	w = world(t, cfg)
	due(w, game.LeadProduct, product)
	if cl := closedOf(step(w, s)); cl == nil || cl.Hit || w.Heat.Evidence != 0 {
		t.Fatalf("the product moved out: closed %+v, file %d", cl, w.Heat.Evidence)
	}

	var offer *content.HouseConfig
	for i, o := range cfg.Houses.Offers {
		if o.City == home {
			offer = &cfg.Houses.Offers[i]
			break
		}
	}
	if offer == nil {
		t.Fatal("no house to buy at home")
	}
	raid := rung(cfg, content.Raid)
	for _, dealt := range []bool{false, true} {
		w = world(t, cfg)
		w.Player.DirtyCash = 10_000_000
		if _, err := w.BuyHouse(game.HouseOffer{ID: offer.ID, Name: offer.Name, City: offer.City, Corner: offer.Corner, Capacity: offer.Capacity, Price: offer.Price, Rent: offer.Rent}); err != nil {
			t.Fatal(err)
		}
		w.Player.DirtyCash = 1000
		w.Houses[0].Stock = map[string]int{product: 100}
		due(w, game.LeadHouse, w.Houses[0].ID)
		var evs []events.Event
		if dealt {
			w.SetStock(home, product, 110)
			evs = append(evs, sale(w, home, 10))
		}
		tk = step(w, s, evs...)
		cl, ev := closedOf(tk), enforcement(tk)
		if cl == nil || !cl.Hit || ev == nil || ev.House != w.Houses[0].ID || ev.StockLost[product] != int(math.Round(100*raid.StockLoss)) || (ev.Evidence > 0) != dealt {
			t.Fatalf("the house, dealt %v: closed %+v, enforcement %+v", dealt, cl, ev)
		}
	}
}
