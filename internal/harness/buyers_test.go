package harness

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// contractEvents splits a run's events by kind, for the tests below.
type contractEvents struct {
	offered   []events.ContractOffered
	delivered []events.ContractDelivered
	failed    []events.ContractFailed
	expired   []events.ContractExpired
}

func contracts(res Result) contractEvents {
	var c contractEvents
	for _, e := range res.Events {
		switch ev := e.(type) {
		case events.ContractOffered:
			c.offered = append(c.offered, ev)
		case events.ContractDelivered:
			c.delivered = append(c.delivered, ev)
		case events.ContractFailed:
			c.failed = append(c.failed, ev)
		case events.ContractExpired:
			c.expired = append(c.expired, ev)
		}
	}
	return c
}

// contractOffer is a contract on the table in city for so many of the
// first product, for the tests that need one without waiting on the deck.
func contractOffer(w *game.World, city string, units int, premium float64) game.Contract {
	product := w.Products[0]
	return w.OfferContract(game.Contract{
		Buyer: "test", Name: "a tester", Pitch: "A tester wants some.", City: city, Product: product, Units: units,
		Premium: premium, Penalty: 6, PenaltyCash: 0.2, HeatMul: 0.4, Street: w.Product(city, product).Price,
		Since: w.Day, Expires: w.Day + 2, Due: w.Day + 3,
	})
}

// probe is a pair of simulations slipped into the step order either side
// of the market: the first records every stash and the cash, the second
// checks the day's handoffs against what was there.
type probe struct {
	stash map[string]map[string]int
	cash  int
	check func(t *game.Tick, before map[string]map[string]int, cash int, w *game.World)
}

type probeBefore struct{ *probe }
type probeAfter struct{ *probe }

func (probeBefore) Name() string { return "probe-before" }
func (probeAfter) Name() string  { return "probe-after" }

func (p probeBefore) Step(w *game.World, t *game.Tick) {
	p.stash = map[string]map[string]int{}
	for _, cid := range w.CityOrder {
		p.stash[cid] = map[string]int{}
		for id, q := range w.StashOf(cid) {
			p.stash[cid][id] = q
		}
	}
	p.cash = w.Player.DirtyCash
}

func (p probeAfter) Step(w *game.World, t *game.Tick) { p.check(t, p.stash, p.cash, w) }

// Every day of every policy that deals: a handoff never exceeds the stash
// it came from; the stash and the cash move by exactly what was handed
// over and sold and paid; a contract is resolved once; nothing is taken
// after its offer lapsed or delivered after its due day; and demand is
// the corners' whatever is in flight.
func TestContractInvariants(t *testing.T) {
	cfg := content.MustLoad()
	for seed := uint64(1); seed <= 4; seed++ {
		var welshed int
		for _, p := range []struct {
			name string
			pol  Policy
		}{{"dealer", Dealer(cfg, 40)}, {"welsher", Welsher(cfg, 40, &welshed)}} {
			w := sim.NewWorld(cfg, seed)
			set, sims, err := sim.Default(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var checked int
			pr := &probe{check: func(tk *game.Tick, stash map[string]map[string]int, cash int, w *game.World) {
				sold := map[string]map[string]int{}
				delivered := map[string]map[string]int{}
				revenue, penalties := 0, 0
				for _, e := range tk.Events() {
					switch ev := e.(type) {
					case events.PlayerSold:
						if sold[ev.City] == nil {
							sold[ev.City] = map[string]int{}
						}
						sold[ev.City][ev.Product] += ev.Sold
						revenue += ev.Revenue
					case events.ContractDelivered:
						if delivered[ev.City] == nil {
							delivered[ev.City] = map[string]int{}
						}
						delivered[ev.City][ev.Product] += ev.Units
						revenue += ev.Revenue
						if ev.Units > stash[ev.City][ev.Product] {
							t.Fatalf("seed %d %s day %d: handed over %d %s in %s with %d in the stash", seed, p.name, tk.Day, ev.Units, ev.Product, ev.City, stash[ev.City][ev.Product])
						}
					case events.ContractFailed:
						penalties += ev.Cash
					}
				}
				for cid, products := range stash {
					for id, q := range products {
						if got, want := w.Stock(cid, id), q-sold[cid][id]-delivered[cid][id]; got != want {
							t.Fatalf("seed %d %s day %d: %s in %s went %d -> %d with %d sold and %d handed over", seed, p.name, tk.Day, id, cid, q, got, sold[cid][id], delivered[cid][id])
						}
					}
				}
				if got, want := w.Player.DirtyCash, cash+revenue-penalties; got != want {
					t.Fatalf("seed %d %s day %d: dirty cash went %d -> %d with %d in and %d out", seed, p.name, tk.Day, cash, got, revenue, penalties)
				}
				for _, cid := range w.CityOrder {
					for _, id := range w.Products {
						if m := w.Product(cid, id); m != nil && w.Demand(cid, id) != m.Demand*w.HeldShare(cid, id) {
							t.Fatalf("seed %d %s day %d: demand for %s in %s is not the corners' with contracts in flight", seed, p.name, tk.Day, id, cid)
						}
					}
				}
				checked++
			}}
			order := append([]game.Simulation{probeBefore{pr}, sims[0], probeAfter{pr}}, sims[1:]...)
			clock := game.NewClock(nil, order...)
			var all []events.Event
			for d := 0; d < 120 && w.Over == nil; d++ {
				p.pol(w)
				all = append(all, clock.EndDay(w)...)
			}
			if checked == 0 {
				t.Fatalf("seed %d %s: the probe never ran", seed, p.name)
			}
			_ = set
			res := Result{Events: all, World: w}
			ce := contracts(res)
			if len(ce.delivered) == 0 {
				t.Fatalf("seed %d %s: nothing was ever delivered", seed, p.name)
			}
			due := map[int]int{}
			for _, o := range ce.offered {
				due[o.ID] = o.Due
			}
			resolved := map[int]int{}
			for _, d := range ce.delivered {
				if d.Day-1 > due[d.ID] {
					t.Fatalf("seed %d %s: contract %d delivered on day %d, due day %d", seed, p.name, d.ID, d.Day-1, due[d.ID])
				}
				if d.Complete {
					resolved[d.ID]++
				}
			}
			for _, f := range ce.failed {
				resolved[f.ID]++
			}
			for _, x := range ce.expired {
				resolved[x.ID]++
			}
			for id, n := range resolved {
				if n != 1 {
					t.Fatalf("seed %d %s: contract %d resolved %d times", seed, p.name, id, n)
				}
			}
			for _, c := range w.Contracts {
				if c.Accepted > 0 && c.Accepted > c.Expires {
					t.Fatalf("seed %d %s: contract %d taken on day %d after it lapsed on day %d", seed, p.name, c.ID, c.Accepted, c.Expires)
				}
			}
			// Demand does not read the contracts: take them off the
			// world and it is the same number.
			w.Contracts = nil
			for _, cid := range w.CityOrder {
				for _, id := range w.Products {
					if m := w.Product(cid, id); m != nil && w.Demand(cid, id) != m.Demand*w.HeldShare(cid, id) {
						t.Fatalf("seed %d %s: demand for %s in %s moved without contracts", seed, p.name, id, cid)
					}
				}
			}
		}
	}
}

// A player who has lost every corner can still work a contract: it needs
// no corner, and it pays.
func TestCornerlessPlayerCanWorkAContract(t *testing.T) {
	cfg := content.MustLoad()
	w := sim.NewWorld(cfg, 3)
	for _, cid := range w.CityOrder {
		for i := range w.Cities[cid].Corners {
			c := &w.Cities[cid].Corners[i]
			c.Owner, c.Runner, c.Enforcer = game.OwnerNone, 0, 0
		}
	}
	if w.Worked() != 0 || w.Held() != 0 {
		t.Fatal("still holding ground")
	}
	home := w.Home().ID
	c := contractOffer(w, home, 30, 1.5)
	if err := w.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	w.SetStock(home, w.Products[0], 30)
	if err := w.Deliver(c.ID, 30); err != nil {
		t.Fatal(err)
	}
	// A street order goes nowhere without a corner; the handoff does not.
	if err := w.PlaceSell(home, w.Products[1], 0, events.DialNormal); err == nil {
		t.Fatal("a sale of nothing was accepted")
	}
	cash := w.Player.DirtyCash
	street := w.Product(home, w.Products[0]).Price
	res, err := RunFrom(cfg, w, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	ce := contracts(res)
	if len(ce.delivered) != 1 || !ce.delivered[0].Complete || ce.delivered[0].Units != 30 {
		t.Fatalf("the handoff did not go through: %+v", ce.delivered)
	}
	if want := int(street*1.5*30 + 0.5); w.Player.DirtyCash-cash < want-1 || w.Player.DirtyCash-cash > want+1 {
		t.Fatalf("paid %d, want %d", w.Player.DirtyCash-cash, want)
	}
	if w.Stock(home, w.Products[0]) != 0 {
		t.Fatalf("%d left in the stash", w.Stock(home, w.Products[0]))
	}
	for _, e := range res.Events {
		if ps, ok := e.(events.PlayerSold); ok && ps.Sold > 0 {
			t.Fatalf("the street sold %d with no corner", ps.Sold)
		}
	}
}

// The premium is a bet against the day, not the signing: over 20 seeds
// some handoff pays less a unit than the street was the day the buyer
// asked, and the bulk mover pays under the street on the day itself.
func TestPremiumIsABet(t *testing.T) {
	cfg := content.MustLoad()
	underSigning, underStreet := 0, 0
	for seed := uint64(1); seed <= 20; seed++ {
		res, err := Run(cfg, seed, 120, Dealer(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range contracts(res).delivered {
			if d.Price < d.Signed {
				underSigning++
			}
			if d.Price < d.Street {
				underStreet++
			}
			if d.Price != d.Street*premiumOf(res.World, d.ID) && premiumOf(res.World, d.ID) > 0 {
				t.Fatalf("seed %d: contract %d paid %.2f on a street of %.2f", seed, d.ID, d.Price, d.Street)
			}
		}
	}
	if underSigning == 0 {
		t.Fatal("no handoff in 20 seeds paid less than the street the day the buyer asked")
	}
	if underStreet == 0 {
		t.Fatal("no handoff in 20 seeds paid less than the street on the day")
	}
	t.Logf("%d handoffs paid under the signing-day street, %d under the day's", underSigning, underStreet)
}

// premiumOf is a contract's premium if it is still on the books, else 0.
func premiumOf(w *game.World, id int) float64 {
	if c := w.Contract(id); c != nil {
		return c.Premium
	}
	return 0
}

// Failure is felt: on the morning a contract fails, the welsher's respect
// is down by the event's penalty (through the day's fade, clamped at
// zero) from where the day's other sources would have left it, and the
// buyer let down is not back inside blacklist_days. That the same seed
// played straight ends day 90 with more respect is logged, not pinned:
// the two runs diverge at the welsh and the crook sometimes lands more
// deliveries after it (4 of 5 seeds under the flat rival pace, 2 of 5
// under #60's), which is a race and not the mechanism.
func TestFailureIsFelt(t *testing.T) {
	cfg := content.MustLoad()
	pace := cfg.Buyers.Buyers
	rep := cfg.Reputation
	felt := 0
	// Both start with some respect to lose: the first contract fails in
	// the first month, when a fresh name has none and a penalty on zero
	// clamps to zero.
	start := func(seed uint64) *game.World {
		w := sim.NewWorld(cfg, seed)
		w.Player.Reputation.Respect = 30
		return w
	}
	for seed := uint64(1); seed <= 5; seed++ {
		var welshed int
		honest, err := RunFrom(cfg, start(seed), 90, Dealer(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		// Respect as each day is played (w.Day is the days ended, so the
		// policy on w.Day d precedes the tick whose events say d+1): the
		// failure morning's drop is read against the day before.
		before := map[int]float64{}
		welsher := Welsher(cfg, 40, &welshed)
		crook, err := RunFrom(cfg, start(seed), 90, func(w *game.World) {
			before[w.Day] = w.Player.Reputation.Respect
			welsher(w)
		})
		if err != nil {
			t.Fatal(err)
		}
		ce := contracts(crook)
		var failed *events.ContractFailed
		for i := range ce.failed {
			if ce.failed[i].ID == welshed {
				failed = &ce.failed[i]
			}
		}
		if failed == nil {
			t.Fatalf("seed %d: the welsher's contract %d never failed", seed, welshed)
		}
		if failed.Respect <= 0 || failed.Notoriety != pace.NotorietyPenalty || failed.Blacklisted != failed.Day+pace.BlacklistDays {
			t.Fatalf("seed %d: a failure without a penalty: %+v", seed, failed)
		}
		for _, o := range ce.offered {
			if o.Buyer == failed.Buyer && o.Day > failed.Day && o.Day < failed.Blacklisted {
				t.Fatalf("seed %d: %s came back on day %d after being let down on day %d", seed, o.Buyer, o.Day, failed.Day)
			}
		}
		// The morning it failed: where the day's other respect sources
		// (deliveries, pay-offs, the payroll, a deal kept) would have
		// left it, less the penalty, faded like every day, floored at 0;
		// the fade is affine so the penalty's share is penalty x (1 -
		// decay). Read after the day, off the next morning's value.
		others := 0.0
		for _, e := range crook.Events {
			switch ev := e.(type) {
			case events.ContractDelivered:
				if ev.Day == failed.Day {
					others += ev.Respect
				}
			case events.CrewPaidOff:
				if ev.Day == failed.Day {
					others += rep.Respect.Payoff
				}
			case events.CrewPaid:
				switch {
				case ev.Day != failed.Day:
				case ev.Short > 0:
					others += rep.Respect.ShortPay
				case ev.Pay == events.PayGenerous:
					others += rep.Respect.GenerousPay
				}
			case events.DealAccepted:
				if ev.Day <= failed.Day {
					others += rep.Respect.DealKept // at most one deal a day could be live
				}
			}
		}
		was, after := before[failed.Day-1], before[failed.Day]
		if failed.Day >= crook.Days {
			after = crook.World.Player.Reputation.Respect
		}
		unpunished := (was + others) * (1 - rep.Reputation.Decay)
		want := math.Max(0, unpunished-failed.Respect*(1-rep.Reputation.Decay))
		if math.Abs(after-want) > 0.01 || after >= unpunished {
			t.Fatalf("seed %d: respect %.2f the morning before the failure, %.2f after, penalty %.1f: want %.2f", seed, was, after, failed.Respect, want)
		}
		if crook.World.Player.Reputation.Respect < honest.World.Player.Reputation.Respect {
			felt++
		}
		t.Logf("seed %d: respect %.2f -> %.2f on the failure morning (penalty %.1f); day 90: %.1f delivered, %.1f welshed on contract %d (day %d)", seed, was, after, failed.Respect, honest.World.Player.Reputation.Respect, crook.World.Player.Reputation.Respect, welshed, failed.Day)
	}
	t.Logf("the welsher ended day 90 with less respect than the honest dealer on %d of 5 seeds (a race after the welsh, not pinned)", felt)
}

// Heat scales with the handoff: a unit handed to a buyer draws more heat
// than a unit a runner moves on a corner, every buyer in the deck weighs
// more than a runner's unit, a sting on a night the only dealing was a
// handoff adds pages (#27: it is dealing), a sting on a night an offer
// merely lapsed adds none, and hard product handed over is pressure
// where it was handed over.
func TestHandoffHeat(t *testing.T) {
	cfg := content.MustLoad()
	for _, b := range cfg.Buyers.Deck {
		if b.Heat <= cfg.Heat.Heat.CrewHeat {
			t.Errorf("buyer %s draws %.2f a unit, no more than a runner's %.2f", b.ID, b.Heat, cfg.Heat.Heat.CrewHeat)
		}
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Per unit, on a crewed operation: the cheapest buyer's unit against
	// a runner's unit on a standard corner, through the same helpers
	// SaleHeat multiplies (the tuning, the Security branch, the product,
	// the city), so the two cannot drift apart. The old form divided a
	// demand-capped sale of a hundred units by a flat hundred and read
	// the corner mix with you on one of them, which is a seed's story.
	res, err := Run(cfg, 1, 40, Crewed(cfg, 40))
	if err != nil {
		t.Fatal(err)
	}
	w := res.World
	home := w.Home().ID
	product := w.Products[1]
	least := 10.0
	for _, b := range cfg.Buyers.Deck {
		least = min(least, b.Heat)
	}
	handoff := set.Heat.ContractHeat(w, home, product, 100, least) / 100
	runner := set.Heat.ContractHeat(w, home, product, 100, set.Heat.CrewHeat(w)) / 100
	if handoff <= runner || runner <= 0 {
		t.Fatalf("a unit handed over draws %.4f, a unit a runner moves %.4f", handoff, runner)
	}
	// Over a run: the heat the dealing draws per unit moved (the report's
	// "moved" and "handed" lines, before decay).
	perUnit := func(p Policy) float64 {
		heat, units := 0.0, 0
		for seed := uint64(1); seed <= 3; seed++ {
			r, err := Run(cfg, seed, 120, p)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range r.Events {
				switch ev := e.(type) {
				case events.HeatChanged:
					for _, why := range ev.Reasons {
						if strings.HasPrefix(why, "moved ") || strings.HasPrefix(why, "handed ") {
							var v float64
							if _, err := fmt.Sscanf(why[strings.LastIndex(why, "(+")+2:], "%f", &v); err == nil {
								heat += v
							}
						}
					}
				case events.PlayerSold:
					units += ev.Sold
				case events.ContractDelivered:
					units += ev.Units
				}
			}
		}
		return heat / float64(units)
	}
	// Logged, not pinned: the dealer keeps a contract's units off the
	// street (harness reserve) and times the handoff, so the blend over a
	// run sits near the crewed player's; the handoff itself is the
	// hotter unit, above.
	t.Logf("heat drawn a unit over 120 days: dealer %.5f, crewed %.5f", perUnit(Dealer(cfg, 40)), perUnit(Crewed(cfg, 40)))

	// A sting on a delivery-only night files pages; on a night with only
	// a lapsed offer it files none.
	sting := func(deliver bool) events.Enforcement {
		w := sim.NewWorld(cfg, 2)
		fixed := Appoint(cfg, w, "lazy", "moderate")
		home := w.Home().ID
		w.Home().Heat = 66 // over the sting line once the day has decayed it
		c := contractOffer(w, home, 20, 1.5)
		if deliver {
			if err := w.AcceptContract(c.ID); err != nil {
				t.Fatal(err)
			}
			w.SetStock(home, w.Products[0], 20)
			if err := w.Deliver(c.ID, 20); err != nil {
				t.Fatal(err)
			}
		} else {
			w.Contract(c.ID).Expires = w.Day // lapses tonight, unanswered
		}
		r, err := RunFrom(fixed, w, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range r.Events {
			if ev, ok := e.(events.Enforcement); ok && ev.Level == "sting" {
				return ev
			}
		}
		t.Fatalf("no sting fired at heat 66 (deliver %v): %v", deliver, r.World.Report.Heat)
		return events.Enforcement{}
	}
	if ev := sting(true); ev.Evidence == 0 {
		t.Fatalf("a sting on a delivery-only night filed nothing: %+v", ev)
	}
	if ev := sting(false); ev.Evidence != 0 {
		t.Fatalf("a sting on a night an offer lapsed filed %d pages", ev.Evidence)
	}

	// Hard product handed over is pressure in that city.
	hard := cfg.Law.Pressure.Hard[0]
	pressure := func(deliver bool) float64 {
		w := sim.NewWorld(cfg, 2)
		home := w.Home().ID
		pc := cfg.Market.Product(hard)
		for _, cid := range w.CityOrder {
			w.AddProduct(cid, game.StartingProduct{ID: pc.ID, Name: pc.Name, Price: pc.BasePrice, Demand: pc.Demand})
		}
		if deliver {
			c := w.OfferContract(game.Contract{Buyer: "test", Name: "a tester", City: home, Product: hard, Units: 100, Premium: 1.5, HeatMul: 0.5, Since: w.Day, Expires: w.Day + 1, Due: w.Day + 1})
			if err := w.AcceptContract(c.ID); err != nil {
				t.Fatal(err)
			}
			w.SetStock(home, hard, 100)
			if err := w.Deliver(c.ID, 100); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := RunFrom(cfg, w, 1, nil); err != nil {
			t.Fatal(err)
		}
		return w.Home().Pressure
	}
	if with, without := pressure(true), pressure(false); with <= without {
		t.Fatalf("pressure %.2f after handing over %s, %.2f without", with, hard, without)
	}
}

// The dealer beats the crewed player on median net worth at day 120 over
// five seeds: the buyers are a second stream, and it pays to visit the
// market screen. The tier rows in TestMoneyCurve are not touched here:
// they are re-measured once #60 lands.
func TestDealerBeatsCrewed(t *testing.T) {
	cfg := content.MustLoad()
	median := func(p Policy) (int, []int) {
		var worth []int
		for seed := uint64(1); seed <= 5; seed++ {
			res, err := Run(cfg, seed, 120, p)
			if err != nil {
				t.Fatal(err)
			}
			worth = append(worth, res.NetWorthAt(120))
		}
		sort.Ints(worth)
		return worth[len(worth)/2], worth
	}
	dealer, dw := median(Dealer(cfg, 40))
	crewed, cw := median(Crewed(cfg, 40))
	t.Logf("net worth on day 120: dealer median %d %v, crewed median %d %v", dealer, dw, crewed, cw)
	if dealer <= crewed {
		t.Fatalf("dealer median net worth %d on day 120 is not over crewed's %d", dealer, crewed)
	}
}

// Every buyer in the deck comes looking somewhere across a few seeds and
// policies, and none of their pitches renders with a hole in it.
func TestEveryBuyerIsDealtAndReadsClean(t *testing.T) {
	cfg := content.MustLoad()
	seen := map[string]int{}
	for seed := uint64(1); seed <= 4; seed++ {
		for i, policy := range []Policy{Dealer(cfg, 40), Laundered(cfg, 40)} {
			w := sim.NewWorld(cfg, seed)
			if i == 1 {
				w.Player.DirtyCash = 60_000 // the buyers the rich draw: a front, the top of the ladder
			}
			res, err := RunFrom(cfg, w, Horizon, policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range contracts(res).offered {
				seen[o.Buyer]++
				for _, s := range []string{o.Name, o.Pitch} {
					if strings.Contains(s, "<no value>") || strings.Contains(s, "{{") || s == "" {
						t.Errorf("buyer %s on day %d renders %q", o.Buyer, o.Day, s)
					}
				}
				if o.Units <= 0 || o.Premium <= 0 || o.Due < o.Day || o.Expires > o.Due {
					t.Errorf("buyer %s on day %d offered nonsense: %+v", o.Buyer, o.Day, o)
				}
			}
		}
	}
	var missing []string
	for _, b := range cfg.Buyers.Deck {
		if seen[b.ID] == 0 {
			missing = append(missing, b.ID)
		}
	}
	if len(missing) > 0 {
		t.Errorf("buyers never dealt across 8 runs: %v (seen %v)", missing, seen)
	}
}

// A buyer's gates hold: the unlock, the blacklist, once, and a trigger in
// the dilemma deck's shape.
func TestBuyerTriggersHold(t *testing.T) {
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	base := func() *game.World {
		w := sim.NewWorld(cfg, 1)
		w.Day = 20
		w.Stats.PeakCash = 50_000
		return w
	}
	rows := []struct {
		name    string
		buyer   content.BuyerConfig
		falsify func(w *game.World)
		verify  func(w *game.World)
	}{
		{"unlock_cash", content.BuyerConfig{ID: "u", UnlockCash: 10_000}, func(w *game.World) { w.Stats.PeakCash = 9_999 }, func(w *game.World) { w.Stats.PeakCash = 10_000 }},
		{"blacklist", content.BuyerConfig{ID: "b"}, func(w *game.World) { w.Buyers.Blacklist = map[string]int{"b": 21} }, func(w *game.World) { w.Buyers.Blacklist = map[string]int{"b": 20} }},
		{"once", content.BuyerConfig{ID: "o", Once: true}, func(w *game.World) { w.Buyers.Drawn = map[string]int{"o": 1} }, func(w *game.World) { w.Buyers.Drawn = map[string]int{"o": 0} }},
		{"corners", content.BuyerConfig{ID: "c", Trigger: content.CardTrigger{Corners: 2}}, func(w *game.World) {}, func(w *game.World) {
			w.Player.DirtyCash += 10_000
			m, err := w.Hire(w.Crew.Candidates[0].ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Post(w.Home().Corners[1].ID, m.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"fronts", content.BuyerConfig{ID: "f", Trigger: content.CardTrigger{Fronts: true}}, func(w *game.World) {}, func(w *game.World) { w.Fronts = []game.Front{{ID: "x", Name: "X"}} }},
		{"heat_max", content.BuyerConfig{ID: "h", Trigger: content.CardTrigger{HeatMax: 40}}, func(w *game.World) { w.Home().Heat = 41 }, func(w *game.World) { w.Home().Heat = 40 }},
	}
	for _, r := range rows {
		w := base()
		r.falsify(w)
		if set.Market.BuyerEligible(w, r.buyer, w.Day) {
			t.Errorf("%s: eligible when it should not be", r.name)
		}
		w = base()
		r.verify(w)
		if !set.Market.BuyerEligible(w, r.buyer, w.Day) {
			t.Errorf("%s: not eligible when it should be", r.name)
		}
	}
}

// Same seed, same buyers on the same days; and a run with the deck boxed
// is the same world as one with it dealt, contracts aside: the deck
// touches nothing else.
func TestContractsAreDeterministicAndBoxable(t *testing.T) {
	cfg := content.MustLoad()
	days := func(res Result) []string {
		var out []string
		for _, o := range contracts(res).offered {
			out = append(out, strings.Join([]string{o.Buyer, o.City, o.Product}, "/")+":"+itoa(o.Day))
		}
		return out
	}
	for seed := uint64(1); seed <= 3; seed++ {
		a, err := Run(cfg, seed, 120, Dealer(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		b, err := Run(cfg, seed, 120, Dealer(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if da, db := days(a), days(b); !reflect.DeepEqual(da, db) || len(da) == 0 {
			t.Fatalf("seed %d: buyers differ between two identical runs:\n%v\n%v", seed, da, db)
		}
		if len(a.Events) != len(b.Events) || a.EndCash != b.EndCash {
			t.Fatalf("seed %d: runs differ: %d vs %d events, $%d vs $%d", seed, len(a.Events), len(b.Events), a.EndCash, b.EndCash)
		}
		boxed := *cfg
		boxed.Buyers.Deck = nil
		dealt, err := Run(cfg, seed, 120, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		quiet, err := Run(&boxed, seed, 120, Crewed(cfg, 40))
		if err != nil {
			t.Fatal(err)
		}
		if len(contracts(dealt).offered) == 0 || len(contracts(quiet).offered) != 0 {
			t.Fatalf("seed %d: the deck was not dealt, or was dealt from the box", seed)
		}
		// The deck's own marks: the contracts, its pacing, its headlines
		// (source "buyers") and the report that carried them.
		dealt.World.Contracts, dealt.World.Buyers = nil, game.BuyersState{}
		var journal []game.Headline
		for _, h := range dealt.World.Journal {
			if h.Source != "buyers" {
				journal = append(journal, h)
			}
		}
		dealt.World.Journal, quiet.World.Journal = journal, append([]game.Headline(nil), quiet.World.Journal...)
		dealt.World.Report, quiet.World.Report = nil, nil
		if !reflect.DeepEqual(dealt.World, quiet.World) {
			t.Fatalf("seed %d: a run with the deck dealt differs from one with it boxed beyond the contracts", seed)
		}
	}
}

func itoa(n int) string {
	return string(rune('0'+n/100)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}

// Contracts survive a save: accepted, part delivered, and the deck's
// pacing and blacklist with them, and the run plays on identically.
func TestSaveKeepsContracts(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	var welshed int
	play := func(w *game.World, days int) Result {
		res, err := RunFrom(cfg, w, days, Welsher(cfg, 40, &welshed))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	a := sim.NewWorld(cfg, 4)
	play(a, 40)
	b := sim.NewWorld(cfg, 4)
	welshed = 0
	play(b, 40)
	// Take an offer and hand over part of it, so the save carries every
	// state a contract can be in.
	home := b.Home().ID
	c := contractOffer(b, home, 20, 1.5)
	if err := b.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	b.AddStock(home, b.Products[0], 5)
	if err := b.Deliver(c.ID, 5); err != nil {
		t.Fatal(err)
	}
	contractOffer(a, home, 20, 1.5)
	if err := a.AcceptContract(b.Contract(c.ID).ID); err != nil {
		t.Fatal(err)
	}
	a.AddStock(home, a.Products[0], 5)
	if err := a.Deliver(c.ID, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrom(cfg, a, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrom(cfg, b, 1, nil); err != nil {
		t.Fatal(err)
	}
	if got := b.Contract(c.ID); got == nil || got.Delivered != 5 || got.Status != game.ContractAccepted {
		t.Fatalf("no part-delivered contract to save: %+v", got)
	}
	if err := game.Save(1, b); err != nil {
		t.Fatal(err)
	}
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := game.Load(1, set.Migrations()...)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.Contracts, b2.Contracts) || !reflect.DeepEqual(b.Buyers, b2.Buyers) {
		t.Fatalf("contracts did not survive the save:\n%+v\n%+v", b.Contracts, b2.Contracts)
	}
	welshed = 0
	ra := play(a, 40)
	welshed = 0
	rb := play(b2, 40)
	if ra.EndCash != rb.EndCash || len(ra.Events) != len(rb.Events) || !reflect.DeepEqual(a.Contracts, b2.Contracts) {
		t.Fatalf("the run diverged after a save: $%d/%d events vs $%d/%d events", ra.EndCash, len(ra.Events), rb.EndCash, len(rb.Events))
	}
}
