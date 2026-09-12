package content

import (
	"fmt"
	"slices"
)

// SuppliersConfig mirrors suppliers.toml (#72): the connects, what a
// temper starts you at, and how the relationship moves and what it
// buys.
type SuppliersConfig struct {
	Suppliers SupplierTuning          `toml:"suppliers"`
	Temper    map[string]TemperConfig `toml:"temper"`
	Deck      []SupplierConfig        `toml:"supplier"`
}

// SupplierTuning is how every connect's relationship moves: what a lot
// bought, a debt cleared, a late payment, a bust and a seizure do to
// Rel, when a connect starts to forget you, where they stop taking
// calls, what a late payment costs under each temper, and what each
// band of Rel does to the capacity and the credit limit. Band is the
// width of a band; Capacity and Credit have one entry a band, the
// middle one 1.
type SupplierTuning struct {
	BandWidth      float64   `toml:"band"`
	RelPerLot      float64   `toml:"rel_per_lot"`
	RelPaid        float64   `toml:"rel_paid"`
	RelLate        float64   `toml:"rel_late"`
	SeizeRel       float64   `toml:"seize_rel"`
	QuietDays      int       `toml:"quiet_days"`
	QuietDecay     float64   `toml:"quiet_decay"`
	FreezeRel      float64   `toml:"freeze_rel"`
	FreezeDays     int       `toml:"freeze_days"`
	LateFee        float64   `toml:"late_fee"`
	ExtendDays     int       `toml:"extend_days"`
	CollectLoyalty float64   `toml:"collect_loyalty"`
	CollectHurt    float64   `toml:"collect_hurt"`
	Capacity       []float64 `toml:"capacity"`
	Credit         []float64 `toml:"credit"`
}

// TemperConfig is what a temper starts the relationship at, and what it
// fades back toward once the connect is left alone.
type TemperConfig struct {
	Rel float64 `toml:"rel"`
}

// Tempers are the tempers a connect can have: what a missed payment
// does. The patient one extends once, the sharp one freezes you out and
// adds a fee, the connected one sends somebody.
var Tempers = []string{"patient", "sharp", "connected"}

// SupplierConfig is one connect: who and where, what they sell (an
// empty list is everything the city's supplier sells), their price band
// as a fraction of the city's street price, the lot they deal in and
// what a buy under it costs extra (SmallLot, 1 for nothing), the units
// a day they can get you, their temper, their credit terms (CreditDays
// 0 is none), and what opens the door: peak cash, and a word from the
// street connect in their city at UnlockRel. Wholesale marks the
// connect the routes buy from; Pool marks one drawn by seed.
type SupplierConfig struct {
	ID          string    `toml:"id"`
	Name        string    `toml:"name"`
	City        string    `toml:"city"`
	Products    []string  `toml:"products"`
	Ratio       []float64 `toml:"ratio"`
	Lot         int       `toml:"lot"`
	SmallLot    float64   `toml:"small_lot"`
	Capacity    int       `toml:"capacity"`
	Temper      string    `toml:"temper"`
	CreditDays  int       `toml:"credit_days"`
	CreditLimit int       `toml:"credit_limit"`
	CreditRatio float64   `toml:"credit_ratio"`
	UnlockCash  int       `toml:"unlock_cash"`
	UnlockRel   float64   `toml:"unlock_rel"`
	Wholesale   bool      `toml:"wholesale"`
	Pool        bool      `toml:"pool"`
}

// Supplier returns the connect with id, or nil.
func (c SuppliersConfig) Supplier(id string) *SupplierConfig {
	for i := range c.Deck {
		if c.Deck[i].ID == id {
			return &c.Deck[i]
		}
	}
	return nil
}

// Bands is how many relationship bands there are: one entry of the
// capacity table each.
func (t SupplierTuning) Bands() int { return len(t.Capacity) }

// Neutral is the band a fresh run sits in: the middle one, where the
// price is the middle of the ratio band and nothing is scaled.
func (t SupplierTuning) Neutral() int { return t.Bands() / 2 }

// Band is the relationship band rel falls in, 0 for the worst.
func (t SupplierTuning) Band(rel float64) int {
	if t.BandWidth <= 0 || t.Bands() == 0 {
		return 0
	}
	return max(0, min(t.Bands()-1, int(rel/t.BandWidth)))
}

// Ratio is a connect's price as a fraction of street at a relationship
// band: the middle of their ratio band in the neutral band, the bottom
// at the top band, the top at the bottom band, a straight line between.
func (t SupplierTuning) Ratio(s SupplierConfig, band int) float64 {
	lo, hi := s.Ratio[0], s.Ratio[1]
	mid := (lo + hi) / 2
	n, top := t.Neutral(), t.Bands()-1
	switch {
	case band < n && n > 0:
		return mid + (hi-mid)*float64(n-band)/float64(n)
	case band > n && top > n:
		return mid - (mid-lo)*float64(band-n)/float64(top-n)
	}
	return mid
}

// CapacityMul is what a band does to a connect's capacity a day.
func (t SupplierTuning) CapacityMul(band int) float64 { return at(t.Capacity, band) }

// CreditMul is what a band does to a connect's credit limit.
func (t SupplierTuning) CreditMul(band int) float64 { return at(t.Credit, band) }

func at(table []float64, i int) float64 {
	if len(table) == 0 {
		return 1
	}
	return table[max(0, min(len(table)-1, i))]
}

// validate checks the file reads as connects: a band table with an odd
// number of bands and the middle at 1, every temper in the table, every
// connect somewhere that exists with a ratio band that reads, a lot and
// a capacity, a known temper and products the city's supplier sells,
// one street connect a city, a wholesale connect only in a wholesale
// city, and the pool not empty.
func (c SuppliersConfig) validate(market MarketConfig, cities CityConfig) error {
	t := c.Suppliers
	if t.BandWidth <= 0 || t.Bands() < 3 || t.Bands()%2 == 0 || len(t.Credit) != t.Bands() {
		return fmt.Errorf("bad band tables: band %v, capacity %v, credit %v", t.BandWidth, t.Capacity, t.Credit)
	}
	if t.Capacity[t.Neutral()] != 1 || t.Credit[t.Neutral()] != 1 {
		return fmt.Errorf("the neutral band must scale nothing: capacity %v credit %v", t.Capacity, t.Credit)
	}
	if t.RelPerLot < 0 || t.RelPaid < 0 || t.RelLate < 0 || t.SeizeRel < 0 || t.QuietDays < 0 || t.QuietDecay < 0 || t.QuietDecay > 1 ||
		t.FreezeRel < 0 || t.FreezeDays < 1 || t.LateFee < 0 || t.ExtendDays < 1 || t.CollectLoyalty < 0 || t.CollectHurt < 0 {
		return fmt.Errorf("bad [suppliers] table %+v", t)
	}
	for _, temper := range Tempers {
		tc, ok := c.Temper[temper]
		if !ok || tc.Rel < 0 || tc.Rel > 100 {
			return fmt.Errorf("no [temper.%s] table, or its rel is not in 0..100", temper)
		}
		if t.Band(tc.Rel) != t.Neutral() {
			return fmt.Errorf("[temper.%s] rel %v is not in the neutral band", temper, tc.Rel)
		}
	}
	seen := map[string]bool{}
	street := map[string]int{}
	pool := 0
	for _, s := range c.Deck {
		if s.ID == "" || seen[s.ID] || s.Name == "" {
			return fmt.Errorf("connect %q missing an id or a name, or defined twice", s.ID)
		}
		seen[s.ID] = true
		city := cities.City(s.City)
		if city == nil {
			return fmt.Errorf("connect %q is in %q, which is no city", s.ID, s.City)
		}
		if len(s.Ratio) != 2 || s.Ratio[0] <= 0 || s.Ratio[0] > s.Ratio[1] {
			return fmt.Errorf("connect %q ratio %v is not [lo, hi]", s.ID, s.Ratio)
		}
		if s.Lot < 1 || s.Capacity < 1 || s.SmallLot < 0 || s.CreditDays < 0 || s.CreditLimit < 0 || (s.CreditDays > 0 && s.CreditRatio < 1) ||
			s.UnlockCash < 0 || s.UnlockRel < 0 || s.UnlockRel > 100 {
			return fmt.Errorf("bad connect %+v", s)
		}
		if !slices.Contains(Tempers, s.Temper) {
			return fmt.Errorf("connect %q has temper %q", s.ID, s.Temper)
		}
		for _, p := range s.Products {
			if market.Product(p) == nil {
				return fmt.Errorf("connect %q sells %q, which is no product", s.ID, p)
			}
			if city.Product(p).NoSupply {
				return fmt.Errorf("connect %q sells %q in %s, where nobody does", s.ID, p, s.City)
			}
		}
		if s.Wholesale && !city.Wholesale {
			return fmt.Errorf("connect %q is the wholesaler in %s, which does not sell by the lot", s.ID, s.City)
		}
		switch {
		case s.Pool:
			pool++
		case !s.Wholesale:
			street[s.City]++
		}
	}
	for _, city := range cities.Cities {
		if street[city.ID] != 1 {
			return fmt.Errorf("%s has %d street connects, want one", city.ID, street[city.ID])
		}
	}
	if pool == 0 {
		return fmt.Errorf("no connect in the pool")
	}
	return nil
}
