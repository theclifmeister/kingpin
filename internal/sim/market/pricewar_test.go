package market_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// warWorld is marketOnly with the rival on The Docks, next to Fourth &
// Main where you stand: a corner to undercut. Weed is stashed deep so
// the order covers both pools.
func warWorld(t *testing.T, cfg *content.Config, seed uint64) (*game.World, *market.Sim, *game.Clock) {
	t.Helper()
	w, mk, clock := marketOnly(t, cfg, seed)
	if err := w.Post("fourth", game.You); err != nil {
		t.Fatal(err)
	}
	w.Corner("docks").Owner = game.OwnerRival
	w.Corner("heights").Owner = game.OwnerRival // far from you
	w.Rival.Arrived, w.Rival.Leader = 1, "Vasquez"
	w.Player.CarryLimit = 10_000
	w.Player.Stash[w.Home().ID]["weed"] = 5000
	return w, mk, clock
}

func find[T events.Event](evs []events.Event, ok func(T) bool) (T, bool) {
	for _, e := range evs {
		if x, is := e.(T); is && ok(x) {
			return x, true
		}
	}
	var zero T
	return zero, false
}

// An undercut serves Steal of the rival corner's demand on top of your
// own corners', at price_cut off: the night's PlayerSold carries the
// units as Undercut, PlayerUndercut names the corner and the share, the
// rival corner wakes up squeezed by the share of its trade taken, the
// stash is short by both pools, and your own corners moved exactly what
// they move with no price war on.
func TestUndercutServesTheShareCheap(t *testing.T) {
	cfg := content.MustLoad()
	plain, _, pc := warWorld(t, cfg, 5)
	war, mk, wc := warWorld(t, cfg, 5)
	home := war.Home().ID
	docks := war.Corner("docks")
	m := war.Product(home, "weed")
	demand := m.Demand
	price := m.Price
	own := mk.Capacity(war, home, "weed", events.DialNormal)
	steal := mk.Steal(war, *docks, events.DialNormal)
	if want := cfg.Rivals.Pricewar.Steal * war.NextDoor(*docks); math.Abs(steal-want) > 1e-9 || steal <= 0 {
		t.Fatalf("Steal at normal = %.3f, want steal %.2f x next door %.2f", steal, cfg.Rivals.Pricewar.Steal, war.NextDoor(*docks))
	}
	units := int(math.Round(demand * docks.Full("weed") * steal))
	total := 0.0 // the corner's trade at the opening prices
	for _, id := range war.Products {
		if pm := war.Product(home, id); pm != nil {
			total += pm.Demand * docks.Full(id) * pm.Price
		}
	}
	for _, w := range []*game.World{plain, war} {
		if err := w.PlaceSell(home, "weed", 5000, events.DialNormal); err != nil {
			t.Fatal(err)
		}
	}
	if err := war.Undercut("docks", events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if got := mk.UndercutPrice(war, home, "weed", events.DialNormal); math.Abs(got-price*(1-cfg.Rivals.Pricewar.PriceCut)) > 1e-9 {
		t.Fatalf("UndercutPrice %.2f, want %.2f", got, price*(1-cfg.Rivals.Pricewar.PriceCut))
	}
	pe := pc.EndDay(plain)
	we := wc.EndDay(war)
	ps, _ := find(pe, func(events.PlayerSold) bool { return true })
	ws, _ := find(we, func(events.PlayerSold) bool { return true })
	if ps.Sold != own || ps.Undercut != 0 {
		t.Fatalf("with no price war: sold %d undercut %d, want %d and 0", ps.Sold, ps.Undercut, own)
	}
	if ws.Undercut != units || ws.Sold != own+units {
		t.Fatalf("with the price war: sold %d undercut %d, want %d own + %d undercut", ws.Sold, ws.Undercut, own, units)
	}
	u, ok := find(we, func(u events.PlayerUndercut) bool { return u.Corner == "docks" })
	if !ok || u.Units != units || u.Product != "weed" || u.Dial != events.DialNormal {
		t.Fatalf("PlayerUndercut: %+v %v, want %d weed at normal", u, ok, units)
	}
	if want := float64(units) / (demand * docks.Full("weed")); math.Abs(u.Share-want) > 1e-9 {
		t.Fatalf("share %.3f, want %.3f", u.Share, want)
	}
	// The price: price_cut off the street price at the order's impact,
	// which is the average the own units got over the dial's price.
	ownAvg := float64(ws.Revenue-ws.UndercutRevenue) / float64(own)
	cutAvg := float64(ws.UndercutRevenue) / float64(units)
	if want := ownAvg * (1 - cfg.Rivals.Pricewar.PriceCut); math.Abs(cutAvg-want) > 0.02 {
		t.Fatalf("undercut units averaged %.2f, want %.2f (own %.2f less %.0f%%)", cutAvg, want, ownAvg, cfg.Rivals.Pricewar.PriceCut*100)
	}
	if got := war.Stock(home, "weed"); got != 5000-own-units {
		t.Fatalf("stash %d, want %d", got, 5000-own-units)
	}
	// The squeeze: the share of the corner's trade at the opening
	// prices that the units were, one product of five.
	if want := float64(units) * price / total; math.Abs(docks.Squeeze-want) > 1e-9 || docks.Squeeze <= 0 || docks.Squeeze >= 1 {
		t.Fatalf("squeeze %.4f, want %.4f", docks.Squeeze, want)
	}
	// The glut: the extra volume dents the price more than the plain
	// night did.
	if war.Product(home, "weed").Glut <= plain.Product(home, "weed").Glut {
		t.Fatalf("glut %.3f with the price war, %.3f without", war.Product(home, "weed").Glut, plain.Product(home, "weed").Glut)
	}
	if war.Undercuts != nil {
		t.Fatalf("the clock left the undercuts: %v", war.Undercuts)
	}
	// Your own corners' demand is the corners': the squeeze on theirs
	// moves nothing of yours.
	if plain.Demand(home, "weed") != war.Demand(home, "weed") && math.Abs(plain.Demand(home, "weed")-war.Demand(home, "weed")) > 1e-9 {
		t.Fatalf("own demand %.1f with the price war, %.1f without", war.Demand(home, "weed"), plain.Demand(home, "weed"))
	}
	if mk.Steal(war, *war.Corner("fourth"), events.DialNormal) != 0 {
		t.Fatal("Steal on your own corner is not zero")
	}
}

// An order too small for both pools is shared pro rata: the rival's
// corner takes its share of what there is, and nothing is served
// beyond the order or the stash; with no order for the product nothing
// moves at all.
func TestUndercutSharesAShortOrder(t *testing.T) {
	cfg := content.MustLoad()
	w, mk, clock := warWorld(t, cfg, 6)
	home := w.Home().ID
	docks := w.Corner("docks")
	own := mk.Capacity(w, home, "weed", events.DialNormal)
	units := int(math.Round(w.Product(home, "weed").Demand * docks.Full("weed") * mk.Steal(w, *docks, events.DialNormal)))
	short := (own + units) / 2
	if err := w.PlaceSell(home, "weed", short, events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if err := w.Undercut("docks", events.DialNormal); err != nil {
		t.Fatal(err)
	}
	evs := clock.EndDay(w)
	ps, _ := find(evs, func(events.PlayerSold) bool { return true })
	wantCut := int(float64(short) * float64(units) / float64(own+units))
	if ps.Undercut != wantCut || ps.Sold > short || ps.Sold < short-1 {
		t.Fatalf("a short order of %d: sold %d undercut %d, want %d undercut and the order filled", short, ps.Sold, ps.Undercut, wantCut)
	}
	// Pills: stashed, undercut queued, no order: nothing moves.
	w.Player.Stash[home]["pills"] = 500
	if err := w.Undercut("docks", events.DialNormal); err != nil {
		t.Fatal(err)
	}
	evs = clock.EndDay(w)
	if _, ok := find(evs, func(u events.PlayerUndercut) bool { return u.Product == "pills" }); ok || w.Stock(home, "pills") != 500 {
		t.Fatalf("pills moved with no order: %d left", w.Stock(home, "pills"))
	}
}

// Every dial takes its own share: quiet less than normal, aggressive
// more, none over steal at the aggressive impact times the neighbours,
// and the squeeze on the corner stays under one whatever is next door.
func TestUndercutDialsAndTheCap(t *testing.T) {
	cfg := content.MustLoad()
	w, mk, _ := warWorld(t, cfg, 7)
	docks := w.Corner("docks")
	var last float64
	for _, d := range []events.Dial{events.DialQuiet, events.DialNormal, events.DialAggressive} {
		s := mk.Steal(w, *docks, d)
		if s <= last {
			t.Fatalf("Steal at %s is %.3f, not over the dial under it (%.3f)", d, s, last)
		}
		last = s
	}
	// Surround it: a runner on Rail Yard too, and the cap holds.
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 50, Name: "Dre", Role: "runner", Skill: 60, Units: 120, Loyalty: 70, Nerve: 50})
	if err := w.Post("railyard", 50); err != nil {
		t.Fatal(err)
	}
	if nd := w.NextDoor(*docks); nd > game.NextDoorMax || nd <= 1 {
		t.Fatalf("next door %.2f with two corners of yours, want over one and at most %d", nd, game.NextDoorMax)
	}
	if s := mk.Steal(w, *docks, events.DialAggressive); s > cfg.Rivals.Pricewar.Steal*cfg.Market.Dial.Aggressive.Impact*game.NextDoorMax+1e-9 || s >= 1 {
		t.Fatalf("Steal surrounded at aggressive = %.3f", s)
	}
}

// Under a truce, a tribute or a split, and on a corner none of your
// worked corners borders, nothing moves: the action refuses it and the
// market, asked again at night, skips one that was legal in the morning
// and is not by then.
func TestUndercutIsRefusedUnderADeal(t *testing.T) {
	cfg := content.MustLoad()
	w, _, clock := warWorld(t, cfg, 8)
	home := w.Home().ID
	if err := w.Undercut("heights", events.DialNormal); err != game.ErrNotNextDoor {
		t.Fatalf("a rival corner far away: %v, want %v", err, game.ErrNotNextDoor)
	}
	if err := w.Undercut("fourth", events.DialNormal); err == nil {
		t.Fatal("your own corner was undercut")
	}
	w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 10}, Since: w.Day, Until: w.Day + 10}}
	if err := w.Undercut("docks", events.DialNormal); err != game.ErrAtPeace {
		t.Fatalf("under a truce: %v, want %v", err, game.ErrAtPeace)
	}
	w.Rival.Deals = []game.Deal{{Kind: game.DealTribute, Terms: game.Terms{PerDay: 100}, Since: w.Day}}
	if err := w.Undercut("docks", events.DialNormal); err != game.ErrAtPeace {
		t.Fatalf("under a tribute: %v, want %v", err, game.ErrAtPeace)
	}
	w.Rival.Deals = []game.Deal{{Kind: game.DealSplit, Terms: game.Terms{Corners: []string{"fourth"}}, Since: w.Day}}
	if err := w.Undercut("docks", events.DialNormal); err == nil {
		t.Fatal("under a split the rival's side of the line was undercut")
	}
	w.Rival.Deals = nil
	// Legal in the morning, at peace by night: the offer taken today is
	// not sealed until the rival step, so the truce is set by hand.
	if err := w.Undercut("docks", events.DialNormal); err != nil {
		t.Fatal(err)
	}
	if err := w.PlaceSell(home, "weed", w.Stock(home, "weed"), events.DialNormal); err != nil {
		t.Fatal(err)
	}
	w.Rival.Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 10}, Since: w.Day, Until: w.Day + 10}}
	evs := clock.EndDay(w)
	if _, ok := find(evs, func(events.PlayerUndercut) bool { return true }); ok || w.Corner("docks").Squeeze != 0 {
		t.Fatalf("an undercut resolved under a truce: squeeze %.2f", w.Corner("docks").Squeeze)
	}
	w.Rival.Deals = nil
	// Recalled by night: nobody of yours next door, nothing moves.
	if err := w.Undercut("docks", events.DialNormal); err != nil {
		t.Fatal(err)
	}
	w.Recall(game.You)
	if err := w.PlaceSell(home, "weed", w.Stock(home, "weed"), events.DialNormal); err != nil {
		t.Fatal(err)
	}
	evs = clock.EndDay(w)
	if _, ok := find(evs, func(events.PlayerUndercut) bool { return true }); ok {
		t.Fatal("an undercut resolved with nobody next door")
	}
	// Called off, nothing is queued; the map is nil again.
	if err := w.Post("fourth", game.You); err != nil {
		t.Fatal(err)
	}
	if err := w.Undercut("docks", events.DialAggressive); err != nil {
		t.Fatal(err)
	}
	if d, ok := w.Undercutting("docks"); !ok || d != events.DialAggressive {
		t.Fatalf("queued: %v %v", d, ok)
	}
	w.CancelUndercut("docks")
	if _, ok := w.Undercutting("docks"); ok || w.Undercuts != nil {
		t.Fatalf("after calling off: %v", w.Undercuts)
	}
}

// A market stepped with no undercut queued is the old market: the rival
// corner's squeeze is zero every morning and the night's events read
// the same as before the price war, so a run that never undercuts
// replays byte for byte.
func TestNoUndercutIsTheOldNight(t *testing.T) {
	cfg := content.MustLoad()
	a, _, ac := warWorld(t, cfg, 9)
	b, _, bc := warWorld(t, cfg, 9)
	b.Undercuts = map[string]events.Dial{} // an empty map is nothing queued
	home := a.Home().ID
	for day := 0; day < 5; day++ {
		for _, w := range []*game.World{a, b} {
			if err := w.PlaceSell(home, "weed", w.Stock(home, "weed"), events.DialNormal); err != nil {
				t.Fatal(err)
			}
		}
		ae, be := ac.EndDay(a), bc.EndDay(b)
		if fmt.Sprintf("%#v", ae) != fmt.Sprintf("%#v", be) {
			t.Fatalf("day %d: the events differ:\n%#v\n%#v", day, ae, be)
		}
		if a.Corner("docks").Squeeze != 0 || a.Corner("docks").Starved != 0 {
			t.Fatalf("day %d: the rival corner is squeezed with no price war", day)
		}
	}
}
