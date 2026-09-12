package market_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// A fresh run's connects price as today's game did (#72): the street
// connect in each city at the file's flat supplier ratio of street,
// the wholesaler at what the lot was over it, every one in the neutral
// band at their temper's relationship, and the market's supplier price
// the street connect's.
func TestConnectsSeedAtTodaysPrice(t *testing.T) {
	cfg := content.MustLoad()
	w, mk, _ := marketOnly(t, cfg, 3)
	tun := cfg.Suppliers.Suppliers
	if len(w.Suppliers) != 4 {
		t.Fatalf("a run has %d connects, want the street one in each city, the wholesaler and one from the pool: %+v", len(w.Suppliers), w.Suppliers)
	}
	for _, cid := range w.CityOrder {
		street := w.StreetSupplier(cid)
		if street == nil || street.Wholesale {
			t.Fatalf("%s has no street connect", cid)
		}
		if mk.Band(street.Rel) != mk.Neutral() || street.Rel != cfg.Suppliers.Temper[street.Temper].Rel {
			t.Fatalf("%s: rel %.0f in band %d, want the temper's %.0f in the neutral band %d", street.Name, street.Rel, mk.Band(street.Rel), cfg.Suppliers.Temper[street.Temper].Rel, mk.Neutral())
		}
		for id, m := range w.Cities[cid].Market {
			if m.NoSupply {
				if _, ok := street.Price[id]; ok {
					t.Fatalf("%s prices %s, which nobody sells in %s", street.Name, id, cid)
				}
				continue
			}
			if want := m.Price * cfg.Market.Market.SupplierRatio; math.Abs(street.Price[id]-want) > 1e-9 {
				t.Fatalf("%s prices %s at %.4f, want the flat ratio's %.4f", street.Name, id, street.Price[id], want)
			}
			if math.Abs(m.SupplierPrice-street.Price[id]) > 1e-9 {
				t.Fatalf("%s: the market's supplier price for %s is %.4f, the street connect's %.4f", cid, id, m.SupplierPrice, street.Price[id])
			}
		}
		if sup := w.WholesaleSupplier(cid); sup != nil {
			if !sup.Locked(w) {
				t.Fatalf("%s deals with a fresh run", sup.Name)
			}
			for id, m := range w.Cities[cid].Market {
				if want := m.Price * cfg.Market.Market.SupplierRatio * 0.7; !m.NoSupply && math.Abs(sup.Price[id]-want) > 1e-6 {
					t.Fatalf("%s prices %s at %.4f, want 0.7 of the street's %.4f", sup.Name, id, sup.Price[id], want)
				}
			}
		}
	}
	// The bands: the neutral one scales nothing, the top gives the
	// bottom of the band and half as much again a day, the floor the top
	// of the band and half the day.
	street := w.StreetSupplier(w.Home().ID)
	row := cfg.Suppliers.Supplier(street.ID)
	if r := tun.Ratio(*row, tun.Neutral()); math.Abs(r-cfg.Market.Market.SupplierRatio) > 1e-9 {
		t.Fatalf("the neutral band prices at %.4f, want %.4f", r, cfg.Market.Market.SupplierRatio)
	}
	if tun.Ratio(*row, tun.Bands()-1) != row.Ratio[0] || tun.Ratio(*row, 0) != row.Ratio[1] {
		t.Fatalf("the top band prices at %.4f (want %.4f), the floor at %.4f (want %.4f)", tun.Ratio(*row, tun.Bands()-1), row.Ratio[0], tun.Ratio(*row, 0), row.Ratio[1])
	}
	if tun.CapacityMul(tun.Neutral()) != 1 || tun.CreditMul(tun.Neutral()) != 1 || tun.CapacityMul(tun.Bands()-1) <= 1 || tun.CreditMul(0) >= 1 {
		t.Fatalf("the band tables: capacity %v credit %v", tun.Capacity, tun.Credit)
	}
}

// The relationship stamps the morning (#72): at the top band the price
// is the bottom of the connect's band and the capacity and the credit
// limit are the top band's; at the floor the price is the top of the
// band and they stop taking calls, and the other connect in the city
// still sells.
func TestRelBandsStampTheMorning(t *testing.T) {
	cfg := content.MustLoad()
	w, mk, clock := marketOnly(t, cfg, 5)
	hub := w.CityOrder[1]
	w.Player.DirtyCash, w.Stats.PeakCash = 10_000_000, 10_000_000
	street, whole := w.StreetSupplier(hub), w.WholesaleSupplier(hub)
	street.Rel, whole.Rel = 100, 0
	clock.EndDay(w)
	weed := w.Products[0]
	sc := cfg.Suppliers.Supplier(street.ID)
	m := w.Product(hub, weed)
	if want := m.Price * sc.Ratio[0]; math.Abs(street.Price[weed]-want) > 1e-9 || street.Band != mk.Bands()-1 {
		t.Fatalf("at rel 100 %s prices %s at %.4f (band %d), want the bottom of the band %.4f", street.Name, weed, street.Price[weed], street.Band, want)
	}
	tun := mk.SuppliersTuning()
	if street.Cap != int(math.Round(float64(sc.Capacity)*tun.CapacityMul(tun.Bands()-1))) || street.Limit != int(math.Round(float64(sc.CreditLimit)*tun.CreditMul(tun.Bands()-1))) {
		t.Fatalf("at rel 100 %s has %d a day and runs you %d, want the top band's %v of %d and %v of %d", street.Name, street.Cap, street.Limit, tun.Capacity, sc.Capacity, tun.Credit, sc.CreditLimit)
	}
	wc := cfg.Suppliers.Supplier(whole.ID)
	if want := m.Price * wc.Ratio[1]; math.Abs(whole.Price[weed]-want) > 1e-9 || whole.Band != 0 {
		t.Fatalf("at rel 0 %s prices %s at %.4f (band %d), want the top of the band %.4f", whole.Name, weed, whole.Price[weed], whole.Band, want)
	}
	if !whole.Frozen(w.Day) {
		t.Fatalf("at rel 0 %s still takes calls", whole.Name)
	}
	if _, err := w.Restock(hub, weed, 1, 0); err == nil {
		t.Fatal("the road bought from a frozen wholesaler")
	}
	_ = w.Travel(hub)
	if _, err := w.Buy(whole.ID, weed, 100, false, 0); err == nil {
		t.Fatal("bought from a frozen connect")
	}
	if _, err := w.Buy(street.ID, weed, 10, false, 0); err != nil {
		t.Fatalf("the other connect would not sell: %v", err)
	}
	if w.SupplierPrice(hub, weed) != street.Price[weed] {
		t.Fatalf("the supplier price is %.4f, want the one connect selling's %.4f", w.SupplierPrice(hub, weed), street.Price[weed])
	}
}

// What a temper does about a short payment (#72), one case each: the
// patient one extends once and then freezes you out, the sharp one
// freezes you out and adds the fee, the connected one sends somebody
// for your best enforcer (loyalty, and skill on a failed nerve roll)
// or, with no enforcer, takes product from the stash in their city.
// Paid in full it is relationship; never a game over.
func TestTempersAnswerALatePayment(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Suppliers.Suppliers
	debt := func(w *game.World, sup *game.Supplier, owed int) {
		sup.Debt, sup.DebtDue = owed, w.Day+1
	}
	late := func(evs []events.Event) (events.DebtLate, bool) {
		for _, e := range evs {
			if ev, ok := e.(events.DebtLate); ok {
				return ev, true
			}
		}
		return events.DebtLate{}, false
	}
	frozen := func(evs []events.Event) bool {
		for _, e := range evs {
			if _, ok := e.(events.SupplierFrozen); ok {
				return true
			}
		}
		return false
	}
	// Paid on the day.
	w, _, clock := marketOnly(t, cfg, 1)
	home := w.Home().ID
	sup := w.StreetSupplier(home)
	w.Player.DirtyCash, w.Player.CleanCash = 300, 900
	rel := sup.Rel
	debt(w, sup, 1_000)
	evs := clock.EndDay(w)
	paid := false
	for _, e := range evs {
		if ev, ok := e.(events.DebtPaid); ok && ev.Amount == 1_000 {
			paid = true
		}
	}
	if !paid || sup.Debt != 0 || sup.DebtDue != 0 || w.Player.DirtyCash != 0 || w.Player.CleanCash != 200 || math.Abs(sup.Rel-(rel+tun.RelPaid)) > 1e-9 {
		t.Fatalf("paid on the day: paid %v, debt %d due %d, cash %d/%d, rel %.1f -> %.1f", paid, sup.Debt, sup.DebtDue, w.Player.DirtyCash, w.Player.CleanCash, rel, sup.Rel)
	}
	for _, temper := range content.Tempers {
		w, _, clock := marketOnly(t, cfg, 1)
		sup := w.StreetSupplier(home)
		sup.Temper = temper
		sup.Rel = 60
		w.Player.DirtyCash, w.Player.CleanCash = 100, 0
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Tank", Role: "enforcer", Skill: 60, Loyalty: 80, Nerve: 0, Wage: 55})
		w.Stash(home)[w.Products[0]] = 50
		debt(w, sup, 1_000)
		evs := clock.EndDay(w)
		ev, ok := late(evs)
		if !ok || ev.Paid != 100 || ev.Owed != 1_000 || ev.Temper != temper {
			t.Fatalf("%s: no late payment, or the wrong one: %+v", temper, ev)
		}
		if sup.Debt < 900 || sup.DebtDue != w.Day+tun.ExtendDays || sup.Rel > 60-tun.RelLate+1e-9 || sup.Late != 1 || w.Stats.LatePayments != 1 {
			t.Fatalf("%s: after a late payment debt %d due %d (day %d) rel %.1f late %d", temper, sup.Debt, sup.DebtDue, w.Day, sup.Rel, sup.Late)
		}
		if w.Over != nil {
			t.Fatalf("%s: a debt ended the run", temper)
		}
		switch temper {
		case "patient":
			if ev.What != "extended" || !sup.Extended || sup.Frozen(w.Day) || frozen(evs) || sup.Debt != 900 {
				t.Fatalf("patient: %+v, extended %v frozen %v debt %d", ev, sup.Extended, sup.Frozen(w.Day), sup.Debt)
			}
			// The second miss is a freeze.
			w.Player.DirtyCash = 0
			sup.DebtDue = w.Day + 1
			evs = clock.EndDay(w)
			ev, _ = late(evs)
			if ev.What != "frozen" || !sup.Frozen(w.Day) || !frozen(evs) {
				t.Fatalf("patient, twice: %+v, frozen %v", ev, sup.Frozen(w.Day))
			}
		case "sharp":
			fee := int(math.Ceil(900 * tun.LateFee))
			if ev.What != "frozen" || ev.Fee != fee || sup.Debt != 900+fee || !sup.Frozen(w.Day) || !frozen(evs) || sup.FrozenUntil != w.Day+tun.FreezeDays {
				t.Fatalf("sharp: %+v, debt %d frozen until %d (day %d)", ev, sup.Debt, sup.FrozenUntil, w.Day)
			}
		case "connected":
			var col events.SupplierCollected
			for _, e := range evs {
				if c, ok := e.(events.SupplierCollected); ok {
					col = c
				}
			}
			m := w.Crew.Member(900)
			if ev.What != "collected" || col.Member != 900 || !col.Hurt || col.Loyalty != tun.CollectLoyalty || m.Loyalty != 80-tun.CollectLoyalty || m.Skill != 60-int(tun.CollectHurt) || sup.Frozen(w.Day) {
				t.Fatalf("connected: %+v, collected %+v, enforcer %+v, frozen %v", ev, col, m, sup.Frozen(w.Day))
			}
			if sup.Debt != 900 || w.Stock(home, w.Products[0]) != 50 {
				t.Fatalf("connected with muscle on the payroll: debt %d, stash %d (the stash is for when there is none)", sup.Debt, w.Stock(home, w.Products[0]))
			}
		}
	}
	// Connected, no crew: product from the stash in their city, worth
	// what is owed at their price, no more than is there.
	w, _, clock = marketOnly(t, cfg, 1)
	sup = w.StreetSupplier(home)
	sup.Temper = "connected"
	w.Player.DirtyCash = 0
	weed := w.Products[0]
	w.Stash(home)[weed] = 1_000
	price := sup.Price[weed]
	debt(w, sup, 500)
	evs = clock.EndDay(w)
	var col events.SupplierCollected
	for _, e := range evs {
		if c, ok := e.(events.SupplierCollected); ok {
			col = c
		}
	}
	units := int(math.Ceil(500 / price))
	if col.Units != units || col.Product != weed || w.Stock(home, weed) != 1_000-units || sup.Debt != 0 || w.Stats.Collected != units {
		t.Fatalf("connected with no crew: collected %+v, stash %d, debt %d (price %.2f)", col, w.Stock(home, weed), sup.Debt, price)
	}
}

// A bust that took product in a city lowers the connect's relationship
// there the next morning, by the lot lost up to seize_rel; a seizure on
// the road out of a city lowers its wholesaler's; under the floor they
// stop taking calls and say so; left alone they drift back toward the
// temper's start.
func TestBustsSeizuresAndQuietMoveRel(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Suppliers.Suppliers
	w, _, clock := marketOnly(t, cfg, 2)
	home, hub := w.CityOrder[0], w.CityOrder[1]
	street, whole := w.StreetSupplier(home), w.WholesaleSupplier(hub)
	street.Rel, whole.Rel = 60, 60
	w.Heat.Busts = []game.Bust{{Day: w.Day + 1, City: home, Level: "sting", Units: street.Lot / 2}}
	w.Logistics.Seizures = []game.Seizure{{Day: w.Day + 1, From: hub, To: home, Product: w.Products[0], Units: 10 * whole.Lot}}
	street.LastBought, whole.LastBought, street.Bought, whole.Bought = w.Day, w.Day, 1, 1
	// The record is yesterday's on the morning the tick brings: day+1
	// is the tick, so the entries must be dated day (the tick reads
	// t.Day-1).
	w.Heat.Busts[0].Day, w.Logistics.Seizures[0].Day = w.Day, w.Day
	// The market reads yesterday's records against t.Day-1 = w.Day.
	clock.EndDay(w)
	if want := 60 - tun.SeizeRel/2; math.Abs(street.Rel-want) > 1e-9 {
		t.Fatalf("half a lot busted: %s rel %.1f, want %.1f", street.Name, street.Rel, want)
	}
	if want := 60 - tun.SeizeRel; math.Abs(whole.Rel-want) > 1e-9 {
		t.Fatalf("ten lots seized: %s rel %.1f, want %.1f", whole.Name, whole.Rel, want)
	}
	// Down to the floor: frozen, with the event, and the street connect
	// in the same city still selling.
	w.Player.DirtyCash, w.Stats.PeakCash = 10_000_000, 10_000_000
	whole.Rel = tun.FreezeRel - 1
	evs := clock.EndDay(w)
	got := false
	for _, e := range evs {
		if ev, ok := e.(events.SupplierFrozen); ok && ev.Supplier == whole.ID && ev.Why == "floor" {
			got = true
		}
	}
	if !got || !whole.Frozen(w.Day) {
		t.Fatalf("at the floor %s was not frozen (event %v)", whole.Name, got)
	}
	if _, err := w.Restock(hub, w.Products[0], 1, 0); err == nil {
		t.Fatal("the road bought from the frozen wholesaler")
	}
	_ = w.Travel(hub)
	if _, err := w.Buy(w.StreetSupplier(hub).ID, w.Products[0], 5, false, 0); err != nil {
		t.Fatalf("the street connect in the same city would not sell: %v", err)
	}
	// Left alone past quiet_days the relationship fades toward the
	// temper's start, from either side.
	w, _, clock = marketOnly(t, cfg, 2)
	street = w.StreetSupplier(home)
	base := cfg.Suppliers.Temper[street.Temper].Rel
	street.Rel, street.Bought, street.LastBought = 90, 1, 0
	w.Day = tun.QuietDays + 5
	before := street.Rel
	clock.EndDay(w)
	if street.Rel >= before || street.Rel < base {
		t.Fatalf("quiet from 90: rel %.1f -> %.1f, want it fading toward %.0f", before, street.Rel, base)
	}
	street.Rel = 10
	before = street.Rel
	clock.EndDay(w)
	if street.Rel <= before || street.Rel > base {
		t.Fatalf("quiet from 10: rel %.1f -> %.1f, want it fading toward %.0f", before, street.Rel, base)
	}
}

// The lots bought build the relationship (#72): rel_per_lot a lot, by
// hand, by contract and by the road alike, credited the morning after
// with the day's capacity counting them, and the door opens on a pool
// connect once the street connect vouches.
func TestLotsBuildRel(t *testing.T) {
	cfg := content.MustLoad()
	tun := cfg.Suppliers.Suppliers
	w, _, clock := marketOnly(t, cfg, 4)
	home := w.Home().ID
	street := w.StreetSupplier(home)
	weed := w.Products[0]
	w.Player.DirtyCash, w.Player.CarryLimit = 1_000_000, 10_000
	rel := street.Rel
	if _, err := w.Buy(street.ID, weed, 2*street.Lot, false, 0); err != nil {
		t.Fatal(err)
	}
	if street.BoughtToday != 2*street.Lot || street.Left() != street.Cap-2*street.Lot {
		t.Fatalf("booked %d against the day, %d left of %d", street.BoughtToday, street.Left(), street.Cap)
	}
	clock.EndDay(w)
	if want := rel + 2*tun.RelPerLot; math.Abs(street.Rel-want) > 1e-9 || street.BoughtToday != 0 {
		t.Fatalf("two lots: rel %.2f -> %.2f, want %.2f; today %d", rel, street.Rel, want, street.BoughtToday)
	}
	// Over the day's capacity is refused, by the hand and the plan.
	street.Cap = 10
	if _, err := w.Buy(street.ID, weed, 11, false, 0); err == nil {
		t.Fatal("bought past the day's capacity")
	}
	if err := w.SetSupply(home, weed, w.Stock(home, weed)+500); err != nil {
		t.Fatal(err)
	}
	mk, _ := market.New(cfg.Market, cfg.City, cfg.Routes.Shipping, cfg.Upgrades, cfg.Reputation.Effects, cfg.Buyers, cfg.Suppliers, cfg.Rivals.Pricewar)
	plan := mk.Plan(w)
	if len(plan) != 1 || plan[0].Units != 10 || plan[0].Why != "supplier" || plan[0].Supplier != street.ID {
		t.Fatalf("the plan against a connect with 10 left: %+v", plan)
	}
	street.FrozenUntil = w.Day + 3
	if plan := mk.Plan(w); len(plan) != 1 || plan[0].Units != 0 || plan[0].Why != "supplier" {
		t.Fatalf("the plan against a frozen connect: %+v", plan)
	}
	w.ClearSupply(home, weed)
	street.FrozenUntil, street.Cap = 0, 10_000
	// The pool connect: locked until the street connect vouches.
	var pool *game.Supplier
	for i := range w.Suppliers {
		if s := &w.Suppliers[i]; s.UnlockRel > 0 {
			pool = s
		}
	}
	if pool == nil {
		t.Fatal("no pool connect in the run")
	}
	w.Stats.PeakCash = max(w.Stats.PeakCash, pool.UnlockCash)
	st := w.StreetSupplier(pool.City)
	st.Rel = pool.UnlockRel - 1
	clock.EndDay(w)
	if !pool.Locked(w) || pool.Opened {
		t.Fatalf("%s deals before %s vouches", pool.Name, st.Name)
	}
	st.Rel = pool.UnlockRel + 5 // a word to spare: left alone, the relationship fades a little each morning
	evs := clock.EndDay(w)
	opened := false
	for _, e := range evs {
		if ev, ok := e.(events.SupplierUnlocked); ok && ev.Supplier == pool.ID {
			opened = true
		}
	}
	if pool.Locked(w) || !pool.Opened || !opened {
		t.Fatalf("%s did not open once %s vouched (event %v)", pool.Name, st.Name, opened)
	}
}

// At the top band a connect warns you the day before a shock or a
// slump on something they sell in their city (#72), and never wrongly:
// every warning is followed by that shock the next morning. The peek
// costs no draw: the run's dice are the run's with the warnings on or
// off.
func TestWarningsAreRightTheDayBefore(t *testing.T) {
	cfg := content.MustLoad()
	w, _, clock := marketOnly(t, cfg, 6)
	home := w.Home().ID
	for i := range w.Suppliers {
		w.Suppliers[i].Rel = 100
	}
	warned := map[string]events.SupplierWarned{}
	shocks := 0
	for day := 0; day < 200; day++ {
		evs := clock.EndDay(w)
		seen := map[string]bool{}
		for _, e := range evs {
			if ev, ok := e.(events.PriceShock); ok {
				key := ev.City + "/" + ev.Product
				seen[key] = true
				if wv, ok := warned[key]; ok {
					if wv.Slump != ev.Slump {
						t.Fatalf("day %d: %s warned of a slump=%v on %s and it was slump=%v", w.Day, wv.Name, wv.Slump, ev.Product, ev.Slump)
					}
					shocks++
				}
			}
		}
		for key, wv := range warned {
			if !seen[key] {
				t.Fatalf("day %d: %s warned of %s in %s yesterday and nothing came", w.Day, wv.Name, wv.Product, wv.City)
			}
		}
		warned = map[string]events.SupplierWarned{}
		for _, e := range evs {
			if ev, ok := e.(events.SupplierWarned); ok {
				sup := w.Supplier(ev.Supplier)
				if sup == nil || sup.City != ev.City || !sup.Sells(ev.Product) || sup.Warned != w.Day {
					t.Fatalf("day %d: a warning from nowhere: %+v", w.Day, ev)
				}
				warned[ev.City+"/"+ev.Product] = ev
			}
		}
	}
	if shocks == 0 {
		t.Fatal("no warning was followed by its shock in 200 days")
	}
	_ = home
	// The dice: a run with every connect at the floor (no warner) and one
	// at the top play the same market, warnings aside.
	a, _, ca := marketOnly(t, cfg, 6)
	b, _, cb := marketOnly(t, cfg, 6)
	for i := range b.Suppliers {
		b.Suppliers[i].Rel = 100
	}
	for day := 0; day < 30; day++ {
		ca.EndDay(a)
		cb.EndDay(b)
		for _, cid := range a.CityOrder {
			for id, m := range a.Cities[cid].Market {
				if m.Price != b.Cities[cid].Market[id].Price {
					t.Fatalf("day %d: the warnings moved %s in %s: %.4f against %.4f", a.Day, id, cid, m.Price, b.Cities[cid].Market[id].Price)
				}
			}
		}
	}
}

// The ticket (wholesale_mul, #119) folds onto the wholesale connect's
// price and no other's (#72), through the market sim's stamp.
func TestTicketFoldsOnTheWholesaler(t *testing.T) {
	cfg := content.MustLoad()
	w, mk, clock := marketOnly(t, cfg, 8)
	hub := w.CityOrder[1]
	whole, street := w.WholesaleSupplier(hub), w.StreetSupplier(hub)
	weed := w.Products[0]
	clock.EndDay(w)
	price := w.Product(hub, weed).Price
	wp, sp := whole.Price[weed]/price, street.Price[weed]/price
	w.Player.DirtyCash = 100_000_000
	if u := cfg.Upgrades.Upgrade("ticket"); u == nil {
		t.Fatal("no node ticket")
	}
	w.Upgrades["ticket"] = true
	fx := game.FoldEffects(w, cfg.Upgrades)
	clock.EndDay(w)
	price = w.Product(hub, weed).Price
	if got, want := whole.Price[weed]/price, wp*fx.WholesaleMul; math.Abs(got-want) > 1e-6 {
		t.Fatalf("with the ticket the wholesaler's ratio is %.4f, want %.4f", got, want)
	}
	if got := street.Price[weed] / price; math.Abs(got-sp) > 1e-6 {
		t.Fatalf("with the ticket the street connect's ratio moved: %.4f, want %.4f", got, sp)
	}
	if r := mk.SupplierRatio(w, whole); math.Abs(r-cfg.Market.Market.SupplierRatio*0.7*fx.WholesaleMul) > 1e-9 {
		t.Fatalf("SupplierRatio for the wholesaler %.4f", r)
	}
}

// A schema-10 world gets one connect a city at today's price (#72): the
// street connect's price is the supplier price as saved, pressure and
// all, the wholesaler's that times the lot's fraction, both in the
// neutral band, no pool connect, and the market's supplier price is
// unchanged; a world with connects is left alone.
func TestMigrateSuppliers(t *testing.T) {
	cfg := content.MustLoad()
	set, _, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := sim.NewWorld(cfg, 9)
	// Day-9 prices, a nudge on the home street.
	clock := game.NewClock(nil, set.Market)
	for i := 0; i < 9; i++ {
		clock.EndDay(w)
	}
	before := map[string]float64{}
	for _, cid := range w.CityOrder {
		for id, m := range w.Cities[cid].Market {
			m.SupplierPrice *= 1.03
			before[cid+"/"+id] = m.SupplierPrice
		}
	}
	w.Suppliers = nil
	set.Market.Migrate(w)
	if len(w.Suppliers) != 3 {
		t.Fatalf("migrated %d connects, want the street one in each city and the wholesaler: %+v", len(w.Suppliers), w.Suppliers)
	}
	for _, cid := range w.CityOrder {
		street := w.StreetSupplier(cid)
		if street == nil || street.UnlockRel > 0 || set.Market.Band(street.Rel) != set.Market.Neutral() {
			t.Fatalf("%s: street connect %+v", cid, street)
		}
		for id, m := range w.Cities[cid].Market {
			if m.NoSupply {
				continue
			}
			if street.Price[id] != before[cid+"/"+id] || m.SupplierPrice != before[cid+"/"+id] {
				t.Fatalf("%s: %s priced at %.4f, the market at %.4f, saved %.4f", cid, id, street.Price[id], m.SupplierPrice, before[cid+"/"+id])
			}
			if whole := w.WholesaleSupplier(cid); whole != nil {
				if want := before[cid+"/"+id] * 0.7; math.Abs(whole.Price[id]-want) > 1e-6 {
					t.Fatalf("%s: the wholesaler prices %s at %.4f, want 0.7 of %.4f", cid, id, whole.Price[id], before[cid+"/"+id])
				}
			}
		}
	}
	n := len(w.Suppliers)
	set.Market.Migrate(w)
	if len(w.Suppliers) != n {
		t.Fatal("a world with connects was migrated again")
	}
}
