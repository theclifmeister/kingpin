package harness

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// A standing order sells like the hand (#114): over 30 days on one seed
// a standing order for 40 weed at normal sells the same units and draws
// the same heat, night for night, as the same order placed by hand
// every day, and takes exactly the cut less cash. The two players buy
// the same 40 a day, so the only thing between them is who placed the
// order.
func TestStandingSellsLikeTheHand(t *testing.T) {
	cfg := content.MustLoad()
	cut := cfg.Market.Standing.Cut
	if cut <= 0 {
		t.Fatal("[standing] cut is zero; the cut is the point")
	}
	start := func() *game.World {
		w := sim.NewWorld(cfg, 7)
		w.Player.DirtyCash = 30_000
		return w
	}
	buy := func(w *game.World) {
		standSomewhere(w)
		_, _ = w.Buy(w.Products[0], 40, cfg.Market.Market.BuyPricePressure)
	}
	hand := func(w *game.World) {
		buy(w)
		home, weed := w.Player.Location, w.Products[0]
		if q := min(40, w.Stock(home, weed)); q > 0 {
			if err := w.PlaceSell(home, weed, q, events.DialNormal); err != nil {
				t.Fatal(err)
			}
		}
	}
	placed := false
	standing := func(w *game.World) {
		buy(w)
		if !placed {
			if err := w.PlaceStanding(w.Player.Location, w.Products[0], 40, events.DialNormal); err != nil {
				t.Fatal(err)
			}
			placed = true
		}
	}
	a, err := RunFrom(cfg, start(), 30, hand)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RunFrom(cfg, start(), 30, standing)
	if err != nil {
		t.Fatal(err)
	}
	if a.Over != nil || b.Over != nil {
		t.Fatalf("a run ended: hand %v, standing %v", a.Over, b.Over)
	}
	type night struct {
		sold, wanted int
		heat         float64
	}
	nights := func(r Result) (ns []night, cuts int, revenue int) {
		byDay := map[int]*night{}
		for _, e := range r.Events {
			switch ev := e.(type) {
			case events.PlayerSold:
				if byDay[ev.Day] == nil {
					byDay[ev.Day] = &night{}
				}
				byDay[ev.Day].sold += ev.Sold
				byDay[ev.Day].wanted += ev.Wanted
				cuts += ev.Cut
				revenue += ev.Revenue
				if ev.Cut != int(math.Round(float64(ev.Revenue)*cut)) && ev.Standing {
					t.Fatalf("day %d: cut %d on a take of %d, the cut is %.2f", ev.Day, ev.Cut, ev.Revenue, cut)
				}
			case events.HeatChanged:
				if byDay[ev.Day] == nil {
					byDay[ev.Day] = &night{}
				}
				byDay[ev.Day].heat = ev.To
			}
		}
		for d := 0; d < 30; d++ {
			if n := byDay[d]; n != nil {
				ns = append(ns, *n)
			} else {
				ns = append(ns, night{})
			}
		}
		return ns, cuts, revenue
	}
	an, acuts, _ := nights(a)
	bn, bcuts, brevenue := nights(b)
	if acuts != 0 {
		t.Fatalf("the hand paid a cut of %d", acuts)
	}
	if bcuts == 0 || brevenue == 0 {
		t.Fatalf("the standing order paid no cut (%d) on a take of %d", bcuts, brevenue)
	}
	for d := range an {
		if an[d] != bn[d] {
			t.Fatalf("day %d: the hand sold %d/%d at heat %.2f, the standing order %d/%d at %.2f", d, an[d].sold, an[d].wanted, an[d].heat, bn[d].sold, bn[d].wanted, bn[d].heat)
		}
	}
	if a.World.TotalStock() != b.World.TotalStock() {
		t.Fatalf("stock %d by hand, %d standing", a.World.TotalStock(), b.World.TotalStock())
	}
	if got, want := a.World.Player.DirtyCash-b.World.Player.DirtyCash, bcuts; got != want {
		t.Fatalf("the standing order left %d less cash than the hand; the cuts came to %d", got, want)
	}
	if got, want := b.World.Stats.Cuts, bcuts; got != want {
		t.Fatalf("Stats.Cuts %d, the cuts came to %d", got, want)
	}
	t.Logf("30 nights: %d sold either way; the standing order paid %d of %d (%.1f%%)", sumSold(b), bcuts, brevenue, float64(bcuts)/float64(brevenue)*100)
}

func sumSold(r Result) int {
	n := 0
	for _, e := range r.Events {
		if ev, ok := e.(events.PlayerSold); ok {
			n += ev.Sold
		}
	}
	return n
}

// A fresh order wins the day (#114): with a standing order for 20 at
// normal, an order placed by hand for 5 at quiet is what sells that
// night, at no cut; the standing order resumes the next night.
func TestStandingYieldsToTheHand(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 3)
	w.Player.DirtyCash = 30_000
	home, weed := w.Player.Location, w.Products[0]
	standSomewhere(w)
	if _, err := w.Buy(weed, 60, 0); err != nil {
		t.Fatal(err)
	}
	if err := w.PlaceStanding(home, weed, 20, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	r, err := RunFrom(cfg, w, 2, func(w *game.World) {
		if w.Day == 0 {
			if err := w.PlaceSell(home, weed, 5, events.DialQuiet); err != nil {
				t.Fatal(err)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var sold []events.PlayerSold
	for _, e := range r.Events {
		if ev, ok := e.(events.PlayerSold); ok {
			sold = append(sold, ev)
		}
	}
	if len(sold) != 2 {
		t.Fatalf("%d sales over two nights: %+v", len(sold), sold)
	}
	if s := sold[0]; s.Wanted != 5 || s.Dial != events.DialQuiet || s.Standing || s.Cut != 0 {
		t.Fatalf("the first night was not the hand's 5 at quiet: %+v", s)
	}
	if s := sold[1]; s.Wanted != 20 || s.Dial != events.DialNormal || !s.Standing || s.Delegated || s.Cut == 0 {
		t.Fatalf("the second night was not the standing 20 at normal: %+v", s)
	}
	if _, ok := w.YourStanding(home, weed); !ok {
		t.Fatal("the standing order did not stand")
	}
	if _, ok := w.Order(home, weed); ok {
		t.Fatal("the day's order was not cleared by the clock")
	}
}

// On every day of every policy (#114): no order, standing or placed,
// sells more than the stash held that morning; a standing order tries
// no more than the stash; a standing order's cut is the file's share of
// its take and a fresh order's or the lieutenant's is nothing; nothing
// sells on a lie-low day; and a standing order stands until cancelled.
func TestStandingNeverOutsellsTheStash(t *testing.T) {
	cfg := content.MustLoad()
	cut := cfg.Market.Standing.Cut
	pols := policies(cfg)
	pols["routine"] = Routine(cfg, 40)
	pols["stocked"] = Stocked(cfg, 40)
	names := make([]string, 0, len(pols))
	for name := range pols {
		names = append(names, name)
	}
	sort.Strings(names)
	standingNights := 0
	for _, name := range names {
		policy := pols[name]
		for seed := uint64(1); seed <= 2; seed++ {
			w := sim.NewWorld(cfg, seed)
			_, sims, err := sim.Default(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var lieLow bool
			var standing map[string]game.SellOrder
			pr := &probe{check: func(tk *game.Tick, stash map[string]map[string]int, cash int, w *game.World) {
				// The stash an order sells from is the morning's plus
				// what the supply contract brought before it resolved.
				for _, e := range tk.Events() {
					if ev, ok := e.(events.SupplyBought); ok {
						stash[ev.City][ev.Product] += ev.Units
					}
				}
				for _, e := range tk.Events() {
					ev, ok := e.(events.PlayerSold)
					if !ok {
						continue
					}
					if lieLow {
						t.Fatalf("%s seed %d day %d: sold %d %s lying low", name, seed, tk.Day, ev.Sold, ev.Product)
					}
					if ev.Sold > stash[ev.City][ev.Product] {
						t.Fatalf("%s seed %d day %d: sold %d %s in %s with %d stashed", name, seed, tk.Day, ev.Sold, ev.Product, ev.City, stash[ev.City][ev.Product])
					}
					switch {
					case ev.Standing && !ev.Delegated:
						standingNights++
						if ev.Wanted > stash[ev.City][ev.Product] {
							t.Fatalf("%s seed %d day %d: a standing order tried %d %s with %d stashed", name, seed, tk.Day, ev.Wanted, ev.Product, stash[ev.City][ev.Product])
						}
						if o, ok := standing[game.OrderKey(ev.City, ev.Product)]; !ok || ev.Wanted > o.Qty || ev.Dial != o.Dial {
							t.Fatalf("%s seed %d day %d: %+v is not the standing order %+v", name, seed, tk.Day, ev, o)
						}
						if want := int(math.Round(float64(ev.Revenue) * cut)); ev.Cut != want {
							t.Fatalf("%s seed %d day %d: cut %d on a take of %d, want %d", name, seed, tk.Day, ev.Cut, ev.Revenue, want)
						}
					default:
						if ev.Cut != 0 {
							t.Fatalf("%s seed %d day %d: a cut of %d on an order that did not stand: %+v", name, seed, tk.Day, ev.Cut, ev)
						}
					}
				}
			}}
			order := append([]game.Simulation{probeBefore{pr}, sims[0], probeAfter{pr}}, sims[1:]...)
			clock := game.NewClock(nil, order...)
			for d := 0; d < 100 && w.Over == nil; d++ {
				policy(w)
				lieLow = w.LieLow
				standing = map[string]game.SellOrder{}
				for k, o := range w.Standing {
					standing[k] = o
				}
				clock.EndDay(w)
				for k, o := range standing {
					if got, ok := w.Standing[k]; !ok || got != o {
						t.Fatalf("%s seed %d day %d: the clock touched the standing order %+v", name, seed, w.Day, o)
					}
				}
			}
		}
	}
	if standingNights == 0 {
		t.Fatal("no policy ever sold on a standing order")
	}
}

// The routine is convenience, not money, and not a trap (#114): the
// routine player, who sells by hand until it has an operation and then
// leaves the selling to standing orders at the crew's cut, ends day 70
// between 85% and 100% of the crewed player over ten seeds.
func TestRoutineIsWithinFifteenPercentOfCrewed(t *testing.T) {
	cfg := content.MustLoad()
	var routine, crewed []int
	for seed := uint64(1); seed <= 10; seed++ {
		r, err := Run(cfg, seed, TierDays[1], Routine(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if r.Over != nil {
			t.Fatalf("seed %d: routine ended on day %d: %s", seed, r.Days, r.Over.Cause)
		}
		if len(r.World.Standing) == 0 {
			t.Fatalf("seed %d: the routine player holds no standing order at day %d", seed, TierDays[1])
		}
		c, _ := Run(cfg, seed, TierDays[1], Crewed(cfg, 40))
		routine = append(routine, r.NetWorthAt(TierDays[1]))
		crewed = append(crewed, c.NetWorthAt(TierDays[1]))
	}
	sort.Ints(routine)
	sort.Ints(crewed)
	r, c := routine[len(routine)/2], crewed[len(crewed)/2]
	t.Logf("day %d median net worth: routine %d, crewed %d (%.1f%%)", TierDays[1], r, c, float64(r)/float64(c)*100)
	if float64(r) < 0.85*float64(c) {
		t.Fatalf("routine median %d is more than 15%% under crewed %d", r, c)
	}
	if r > c {
		t.Fatalf("routine median %d is over crewed %d: the cut is not costing anything", r, c)
	}
}

// A run with standing orders (#114) replays from its seed and survives
// a save: the routine player's events are the same twice over, and a
// run saved on day 40 with its orders standing plays the last 30 days
// on exactly as the run that never stopped.
func TestStandingIsDeterministicAndSaves(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	a, err := Run(cfg, 4, 70, Routine(cfg, 40))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Run(cfg, 4, 70, Routine(cfg, 40))
	if len(a.Events) != len(b.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(a.Events), len(b.Events))
	}
	for i := range a.Events {
		if fmt.Sprintf("%#v", a.Events[i]) != fmt.Sprintf("%#v", b.Events[i]) {
			t.Fatalf("event %d differs:\n%#v\n%#v", i, a.Events[i], b.Events[i])
		}
	}
	c, _ := Run(cfg, 4, 40, Routine(cfg, 40))
	if len(c.World.Standing) == 0 {
		t.Fatal("on day 40 the routine player holds no standing order")
	}
	if err := game.Save(1, c.World); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(loaded.Standing) != fmt.Sprint(c.World.Standing) {
		t.Fatalf("loaded %v, saved %v", loaded.Standing, c.World.Standing)
	}
	d, _ := RunFrom(cfg, loaded, 30, Routine(cfg, 40))
	rest := a.Events[len(c.Events):]
	if len(d.Events) != len(rest) {
		t.Fatalf("after loading, %d events for the last 30 days, want %d", len(d.Events), len(rest))
	}
	for i := range rest {
		if fmt.Sprintf("%#v", rest[i]) != fmt.Sprintf("%#v", d.Events[i]) {
			t.Fatalf("event %d after the save differs:\n%#v\n%#v", i, rest[i], d.Events[i])
		}
	}
	if d.World.NetWorth() != a.World.NetWorth() {
		t.Fatalf("net worth %d after the save, %d straight through", d.World.NetWorth(), a.World.NetWorth())
	}
}
