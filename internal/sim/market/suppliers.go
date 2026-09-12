package market

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The connects (#72). The market sim owns them: it seeds them, stamps
// their price, capacity and credit limit each morning from the band the
// relationship sits in, books what was bought against them, moves the
// relationship on a lot, a debt cleared or missed, a bust and a seizure,
// collects a debt on its day and lets the temper answer a short one,
// and, at the top band, warns of a shock it can see coming. The heat,
// logistics and crew sims react to what it emits, never the reverse:
// the busts it reads are the heat sim's record on the world
// (HeatState.Busts), the seizures the logistics sim's
// (LogisticsState.Seizures), both yesterday's, as the price shock is.

// SuppliersTuning is the [suppliers] table: how the relationship moves.
func (s *Sim) SuppliersTuning() content.SupplierTuning { return s.scfg.Suppliers }

// Band is the relationship band rel sits in, 0 the worst; Bands is how
// many there are and Neutral the middle one, where a fresh run starts.
func (s *Sim) Band(rel float64) int { return s.scfg.Suppliers.Band(rel) }

// Bands is how many relationship bands there are.
func (s *Sim) Bands() int { return s.scfg.Suppliers.Bands() }

// Neutral is the band a fresh run sits in on every connect.
func (s *Sim) Neutral() int { return s.scfg.Suppliers.Neutral() }

// SupplierRatio is a connect's price as a fraction of street today: the
// band the relationship sits in on their ratio band, less what a
// supplier contact and the player's respect take off, and, for the
// wholesaler, the ticket (wholesale_mul). A connect the file does not
// know reads the file's flat ratio.
func (s *Sim) SupplierRatio(w *game.World, sup *game.Supplier) float64 {
	return s.supplierRatio(w, sup, game.FoldEffects(w, s.tree))
}

func (s *Sim) supplierRatio(w *game.World, sup *game.Supplier, fx game.Effects) float64 {
	ratio := s.cfg.Market.SupplierRatio
	if sc := s.scfg.Supplier(sup.ID); sc != nil {
		ratio = s.scfg.Suppliers.Ratio(*sc, s.Band(sup.Rel))
	}
	ratio *= fx.SupplierMul * content.Cut(w.Player.Reputation.Respect, s.rep.RespectSupplierCut)
	if sup.Wholesale {
		ratio *= fx.WholesaleMul
	}
	return ratio
}

// BaseRatio is the file's flat supplier ratio less what the contact and
// respect take off: what a city with no connect charges, and what the
// dashboard's line reads against.
func (s *Sim) BaseRatio(w *game.World) float64 {
	return s.cfg.Market.SupplierRatio * game.FoldEffects(w, s.tree).SupplierMul * content.Cut(w.Player.Reputation.Respect, s.rep.RespectSupplierCut)
}

// RatioAt is what a connect would charge as a fraction of street in a
// relationship band: the market screen's ladder.
func (s *Sim) RatioAt(w *game.World, sup *game.Supplier, band int) float64 {
	sc := s.scfg.Supplier(sup.ID)
	if sc == nil {
		return s.BaseRatio(w)
	}
	fx := game.FoldEffects(w, s.tree)
	ratio := s.scfg.Suppliers.Ratio(*sc, band) * fx.SupplierMul * content.Cut(w.Player.Reputation.Respect, s.rep.RespectSupplierCut)
	if sup.Wholesale {
		ratio *= fx.WholesaleMul
	}
	return ratio
}

// connect builds a connect from its row in the file at its temper's
// starting relationship, nothing stamped yet.
func (s *Sim) connect(sc content.SupplierConfig) game.Supplier {
	sup := game.Supplier{
		ID: sc.ID, Name: sc.Name, City: sc.City, Products: append([]string(nil), sc.Products...),
		Temper: sc.Temper, Lot: sc.Lot, SmallLot: math.Max(1, sc.SmallLot),
		CreditDays: sc.CreditDays, CreditRatio: sc.CreditRatio, UnlockCash: sc.UnlockCash, UnlockRel: sc.UnlockRel,
		Wholesale: sc.Wholesale, Rel: s.scfg.Temper[sc.Temper].Rel, Price: map[string]float64{},
	}
	return sup
}

// Seed puts the connects in a fresh run: every one in the file that is
// not in the pool, in file order, and one from the pool drawn with rng
// (the day-0 stream, after the crew, the rival and the law have drawn
// theirs), each at their temper's relationship and stamped for the
// morning at the middle of their band, which is today's price.
func (s *Sim) Seed(w *game.World, rng *rand.Rand) {
	var pool []content.SupplierConfig
	for _, sc := range s.scfg.Deck {
		if w.Cities[sc.City] == nil {
			continue
		}
		if sc.Pool {
			pool = append(pool, sc)
			continue
		}
		w.AddSupplier(s.connect(sc))
	}
	if len(pool) > 0 {
		w.AddSupplier(s.connect(pool[rng.IntN(len(pool))]))
	}
	s.stampAll(w)
	for i := range w.Suppliers {
		w.Suppliers[i].Opened = !w.Suppliers[i].Locked(w)
	}
}

// Migrate is the 10 -> 11 step: a save from before the connects gets
// the ones every run has, the street connect in each city and the
// wholesaler, at today's price: the street connect's is the market's
// supplier price as saved, pressure and all, and the wholesaler's that
// times what the lot was over it, so the morning plays as it would
// have. No pool connect: the old game had none. A save that has
// connects is left alone.
func (s *Sim) Migrate(w *game.World) {
	if len(w.Suppliers) > 0 {
		return
	}
	fx := game.FoldEffects(w, s.tree)
	for _, sc := range s.scfg.Deck {
		if sc.Pool || w.Cities[sc.City] == nil {
			continue
		}
		sup := w.AddSupplier(s.connect(sc))
		street := s.scfg.Suppliers.Ratio(*s.streetRow(sc.City), s.Neutral())
		for id, m := range w.Cities[sc.City].Market {
			if m.NoSupply || !sup.Sells(id) {
				continue
			}
			price := m.SupplierPrice
			if sup.Wholesale && street > 0 {
				price *= s.scfg.Suppliers.Ratio(sc, s.Neutral()) / street * fx.WholesaleMul
			}
			sup.Price[id] = price
		}
		s.stampBand(w, sup)
		sup.Opened = !sup.Locked(w)
	}
	w.RefreshSupplierPrices()
}

// streetRow is the file's street connect in a city.
func (s *Sim) streetRow(city string) *content.SupplierConfig {
	for i := range s.scfg.Deck {
		if sc := &s.scfg.Deck[i]; sc.City == city && !sc.Pool && !sc.Wholesale {
			return sc
		}
	}
	return nil
}

// stampAll stamps every connect's price, capacity and credit limit for
// the morning from the street price and the band, and brings the
// markets' supplier prices into line.
func (s *Sim) stampAll(w *game.World) {
	fx := game.FoldEffects(w, s.tree)
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		s.stampBand(w, sup)
		city := w.Cities[sup.City]
		if city == nil {
			continue
		}
		for id, m := range city.Market {
			s.stampPrice(w, sup, id, m, fx)
		}
	}
	w.RefreshSupplierPrices()
}

// stampBand stamps what the band the relationship sits in does to the
// connect's capacity and credit limit today.
func (s *Sim) stampBand(w *game.World, sup *game.Supplier) {
	sc := s.scfg.Supplier(sup.ID)
	if sc == nil {
		return
	}
	tun := s.scfg.Suppliers
	sup.Band = tun.Band(sup.Rel)
	sup.Cap = max(1, int(math.Round(float64(sc.Capacity)*tun.CapacityMul(sup.Band))))
	sup.Limit = int(math.Round(float64(sc.CreditLimit) * tun.CreditMul(sup.Band)))
}

// stampPrice resets a connect's price for a product to their fraction
// of street: what the pressure of the day's buys is nudged from.
func (s *Sim) stampPrice(w *game.World, sup *game.Supplier, id string, m *game.ProductMarket, fx game.Effects) {
	if m.NoSupply || !sup.Sells(id) {
		delete(sup.Price, id)
		return
	}
	sup.Price[id] = m.Price * s.supplierRatio(w, sup, fx)
}

// book closes yesterday's book on every connect at the top of the step:
// the relationship falls for a bust that took stock in their city
// yesterday (the heat sim's record), a seizure on the road out of their
// city yesterday (the wholesaler's, the logistics sim's record) and for
// being left alone; a debt due today is collected, and a short one
// answered by the temper; under the floor they stop taking calls; a
// door that has opened is news; then the day's capacity and credit
// limit are stamped from the band. What the lots bought do to the
// relationship is credit's, after the supply contracts have bought
// theirs. No dice but the connected temper's nerve roll, off the
// connects' own side stream.
func (s *Sim) book(w *game.World, t *game.Tick) {
	tun := s.scfg.Suppliers
	// The day's receipts by hand, and what went on each book.
	credit := map[string]int{}
	for _, b := range w.Buys {
		if b.Contract {
			continue
		}
		sup := w.Supplier(b.Supplier)
		if sup == nil {
			continue
		}
		t.Emit(events.SupplierBought{Day: t.Day, City: b.City, Supplier: sup.ID, Name: sup.Name, Product: b.Product, Units: b.Qty, Price: b.UnitPrice, Cost: b.Cost, Credit: b.Credit, SmallLot: b.SmallLot})
		if b.Credit {
			credit[sup.ID] += b.Cost
		}
	}
	for i := range w.Suppliers {
		if sup := &w.Suppliers[i]; credit[sup.ID] > 0 {
			t.Emit(events.CreditTaken{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Amount: credit[sup.ID], Debt: sup.Debt, Due: sup.DebtDue})
		}
	}
	if w.Owed() > 0 {
		w.Stats.DebtDays++ // a day ended owing somebody
	}
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		// A bust or a seizure costs by the lot lost, up to seize_rel:
		// a sting that takes a handful of units is a bad night, a
		// raid or a lost shipment is their product gone.
		for _, b := range w.Heat.Busts {
			if b.Day == t.Day-1 && b.City == sup.City && b.Units > 0 {
				sup.Rel -= s.seizeRel(sup, b.Units)
			}
		}
		if sup.Wholesale {
			for _, z := range w.Logistics.Seizures {
				if z.Day == t.Day-1 && z.From == sup.City {
					sup.Rel -= s.seizeRel(sup, z.Units)
				}
			}
		}
		// A connect you do not use forgets you, good or bad: the
		// relationship fades toward what the temper starts at.
		if tun.QuietDays > 0 && (sup.Bought == 0 || t.Day-sup.LastBought > tun.QuietDays) {
			base := s.scfg.Temper[sup.Temper].Rel
			sup.Rel += (base - sup.Rel) * tun.QuietDecay
		}
		sup.Rel = math.Max(0, math.Min(100, sup.Rel))
		if sup.Debt > 0 && sup.DebtDue <= t.Day {
			s.collect(w, t, sup)
		}
		sup.Rel = math.Max(0, math.Min(100, sup.Rel))
		if sup.Rel < tun.FreezeRel && !sup.Frozen(t.Day) {
			sup.FrozenUntil = t.Day + tun.FreezeDays
			t.Emit(events.SupplierFrozen{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Days: tun.FreezeDays, Why: "floor"})
		}
		s.stampBand(w, sup)
	}
	// The doors: a connect who will deal with you from this morning.
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		if !sup.Opened && !sup.Locked(w) {
			sup.Opened = true
			if sup.UnlockCash > 0 || sup.UnlockRel > 0 {
				t.Emit(events.SupplierUnlocked{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name})
			}
		}
	}
}

// seizeRel is what a bust or a seizure of units of a connect's product
// costs the relationship: seize_rel a lot lost, at most seize_rel.
func (s *Sim) seizeRel(sup *game.Supplier, units int) float64 {
	tun := s.scfg.Suppliers
	if sup.Lot <= 0 {
		return tun.SeizeRel
	}
	return math.Min(tun.SeizeRel, tun.SeizeRel*float64(units)/float64(sup.Lot))
}

// credit is the relationship rising by the lots bought since the book
// was last closed: the hand's through the day, the road's this morning
// and the supply contracts' just now, so a contract's lot counts the
// morning it is bought as the hand's does (TestSupplyMatchesTheHand);
// then the day's count starts again and the band is stamped, so the
// capacity and the credit limit the day runs on are the new band's.
func (s *Sim) credit(w *game.World) {
	tun := s.scfg.Suppliers
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		if sup.Lot > 0 && sup.BoughtToday > 0 {
			sup.Rel = math.Min(100, sup.Rel+tun.RelPerLot*float64(sup.BoughtToday)/float64(sup.Lot))
		}
		sup.BoughtToday = 0
		s.stampBand(w, sup)
	}
}

// collect takes a debt on its day, dirty cash first, then clean. Paid
// in full it is Rel for keeping your word; short, the temper answers:
// the patient one extends once and after that freezes you out, the
// sharp one freezes you out and adds a fee, the connected one sends
// somebody for your muscle, or for the stash where you have none. What
// is still owed is due again in extend_days. Never a game over: debt is
// pressure, not an ending.
func (s *Sim) collect(w *game.World, t *game.Tick, sup *game.Supplier) {
	tun := s.scfg.Suppliers
	owed := sup.Debt
	paid := w.TakeCash(sup.Debt)
	sup.Debt -= paid
	w.Stats.Repaid += paid
	if sup.Debt <= 0 {
		sup.Debt, sup.DebtDue, sup.Extended = 0, 0, false
		sup.Rel += tun.RelPaid
		t.Emit(events.DebtPaid{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Amount: paid})
		return
	}
	sup.Late++
	w.Stats.LatePayments++
	sup.Rel -= tun.RelLate
	ev := events.DebtLate{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Temper: sup.Temper, Owed: owed, Paid: paid}
	freeze := false
	switch sup.Temper {
	case "patient":
		if !sup.Extended {
			sup.Extended = true
			ev.What = "extended"
		} else {
			freeze = true
		}
	case "sharp":
		fee := int(math.Ceil(float64(sup.Debt) * tun.LateFee))
		sup.Debt += fee
		ev.Fee = fee
		freeze = true
	case "connected":
		ev.What = "collected"
		s.sendSomebody(w, t, sup)
	}
	if freeze {
		ev.What = "frozen"
		sup.FrozenUntil = max(sup.FrozenUntil, t.Day+tun.FreezeDays)
	}
	sup.DebtDue = t.Day + tun.ExtendDays
	ev.Left, ev.Due = sup.Debt, sup.DebtDue
	t.Emit(ev)
	if freeze {
		t.Emit(events.SupplierFrozen{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Days: tun.FreezeDays, Why: "late"})
	}
}

// sendSomebody is the connected temper's answer to a short payment:
// their people find your best enforcer, who loses loyalty and, failing
// a nerve roll, is hurt (skill lost, off their corner); with no
// enforcer on the payroll they take product from the stash in their
// city, what they sell, up to what is owed at their price.
func (s *Sim) sendSomebody(w *game.World, t *game.Tick, sup *game.Supplier) {
	tun := s.scfg.Suppliers
	ev := events.SupplierCollected{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Owed: sup.Debt}
	var muscle *game.CrewMember
	for i := range w.Crew.Members {
		m := &w.Crew.Members[i]
		if m.Role == "enforcer" && (muscle == nil || m.Skill > muscle.Skill) {
			muscle = m
		}
	}
	if muscle != nil {
		muscle.Loyalty = math.Max(0, muscle.Loyalty-tun.CollectLoyalty)
		ev.Member, ev.MemberName, ev.Loyalty = muscle.ID, muscle.Name, tun.CollectLoyalty
		if t.Sub("suppliers").Float64()*100 >= float64(muscle.Nerve) {
			muscle.Skill = max(1, muscle.Skill-int(tun.CollectHurt))
			w.Recall(muscle.ID)
			ev.Hurt = true
		}
		t.Emit(ev)
		return
	}
	stash := w.Stash(sup.City)
	for _, id := range w.Products {
		price := sup.Price[id]
		if !sup.Sells(id) || price <= 0 || stash[id] <= 0 || sup.Debt <= 0 {
			continue
		}
		units := min(stash[id], int(math.Ceil(float64(sup.Debt)/price)))
		stash[id] -= units
		sup.Debt = max(0, sup.Debt-int(math.Round(float64(units)*price)))
		w.Stats.Collected += units
		ev.Units += units
		ev.Product = id
	}
	if sup.Debt == 0 {
		sup.DebtDue, sup.Extended = 0, false
	}
	t.Emit(ev)
}

// warn is the top band's tip: a connect who has you at the top of the
// ladder tells you the day before about a shock or a slump they can see
// coming on something they sell in their city. The market's dice for
// tomorrow are the seed's, so the sim reads them ahead (peek) rather
// than rolling anything: the warning costs no draw and moves no
// stream, and it is right unless a seizure tonight displaces
// tomorrow's roll for the product, when the seizure is the shock.
func (s *Sim) warn(w *game.World, t *game.Tick) {
	top := s.Bands() - 1
	var warners []*game.Supplier
	for i := range w.Suppliers {
		if sup := &w.Suppliers[i]; s.Band(sup.Rel) == top && sup.Open(w) {
			warners = append(warners, sup)
		}
	}
	if len(warners) == 0 {
		return
	}
	for _, sh := range s.peek(w, t) {
		for _, sup := range warners {
			if sup.City != sh.city || !sup.Sells(sh.product) {
				continue
			}
			sup.Warned = t.Day
			t.Emit(events.SupplierWarned{Day: t.Day, City: sup.City, Supplier: sup.ID, Name: sup.Name, Product: sh.product, Slump: sh.slump})
			break
		}
	}
}

// shock is a shock the peek sees coming tomorrow.
type shock struct {
	city, product string
	slump         bool
}

// peek reads tomorrow's shock rolls off the seed: the same draws Step
// makes for each city, in the same order, from the streams it will
// use tomorrow, against the shock state as tonight leaves it.
func (s *Sim) peek(w *game.World, t *game.Tick) []shock {
	tun := s.cfg.Market
	ids := append([]string(nil), w.Products...)
	sort.Strings(ids)
	tomorrow := &game.Tick{Day: t.Day + 1, Seed: w.Seed}
	var out []shock
	for _, cid := range w.CityOrder {
		city := w.Cities[cid]
		rng := game.RNGFor(w.Seed, t.Day+1)
		if city != w.Home() {
			rng = tomorrow.Sub("market:" + cid)
		}
		for _, id := range ids {
			m := city.Market[id]
			if m == nil || s.cfg.Product(id) == nil {
				continue
			}
			if m.ShockDays == 0 {
				r := rng.Float64()
				switch {
				case r < tun.ShockChance:
					rng.Float64()
					rng.IntN(5)
					out = append(out, shock{cid, id, false})
				case r < tun.ShockChance+tun.SlumpChance:
					rng.Float64()
					rng.IntN(5)
					out = append(out, shock{cid, id, true})
				}
			}
			rng.NormFloat64()
			rng.NormFloat64()
		}
	}
	return out
}
