package content

import (
	"fmt"
	"slices"
)

// BuyersConfig mirrors buyers.toml (#71): the buyers and their pacing.
type BuyersConfig struct {
	Buyers BuyersTuning  `toml:"buyers"`
	Deck   []BuyerConfig `toml:"buyer"`
}

// BuyersTuning paces the offers the way DilemmasTuning paces the cards:
// none for MinGap days after the last, then a rising chance each day
// until one is certain at MaxGap, one on the table at a time.
type BuyersTuning struct {
	MinGap           int     `toml:"min_gap"`
	MaxGap           int     `toml:"max_gap"`
	OfferDays        int     `toml:"offer_days"`        // days an offer stands, never past its due day
	BlacklistDays    int     `toml:"blacklist_days"`    // a buyer let down stays away this long
	Respect          float64 `toml:"respect"`           // respect a contract delivered in full earns
	NotorietyPenalty float64 `toml:"notoriety_penalty"` // notoriety a failed contract adds
	KeepDays         int     `toml:"keep_days"`         // resolved contracts stay on the books this long
}

// BuyerConfig is one buyer: who they are, where and what they want, the
// bands the terms are drawn from, and what they do to you. Every price
// is a multiplier of the street price in the buyer's city on the day of
// delivery. Size is in days of one standard corner's demand for the
// product there. Pitch is a text/template over BuyerSlots.
type BuyerConfig struct {
	ID          string      `toml:"id"`
	Name        string      `toml:"name"`
	Pitch       string      `toml:"pitch"`
	City        string      `toml:"city"`      // city id or "any"
	Product     string      `toml:"product"`   // product id or "any"
	MinPrice    float64     `toml:"min_price"` // with "any": only products whose home base price is at least this
	Size        []float64   `toml:"size"`      // [lo, hi] days of a standard corner's demand
	Days        []int       `toml:"days"`      // [lo, hi] days to deliver from the offer
	Premium     []float64   `toml:"premium"`   // [lo, hi] multiplier on the street price at delivery
	Penalty     float64     `toml:"penalty"`   // respect lost when short at the due day
	PenaltyCash float64     `toml:"penalty_cash"`
	Heat        float64     `toml:"heat"`   // per unit handed over, relative to a street unit
	Weight      float64     `toml:"weight"` // relative draw weight; 0 means 1
	Once        bool        `toml:"once"`
	UnlockCash  int         `toml:"unlock_cash"`
	Trigger     CardTrigger `toml:"trigger"`
}

// Buyer returns the buyer with id, or nil.
func (b BuyersConfig) Buyer(id string) *BuyerConfig {
	for i := range b.Deck {
		if b.Deck[i].ID == id {
			return &b.Deck[i]
		}
	}
	return nil
}

// BuyerAny is the city or product value that leaves the choice to the
// deal.
const BuyerAny = "any"

// validate checks the deck reads as a deck: ids unique, every band a
// [lo, hi] pair with lo <= hi and lo positive, every city and product a
// known one or "any", and the pacing sane.
func (b BuyersConfig) validate(market MarketConfig, cities CityConfig) error {
	t := b.Buyers
	if t.MinGap < 1 || t.MaxGap < t.MinGap {
		return fmt.Errorf("min_gap %d and max_gap %d must be 1 <= min <= max", t.MinGap, t.MaxGap)
	}
	if t.OfferDays < 1 || t.BlacklistDays < 0 || t.KeepDays < 0 || t.Respect < 0 || t.NotorietyPenalty < 0 {
		return fmt.Errorf("bad [buyers] table %+v", t)
	}
	seen := map[string]bool{}
	for _, c := range b.Deck {
		if c.ID == "" || c.Name == "" || c.Pitch == "" {
			return fmt.Errorf("buyer %q needs an id, a name and a pitch", c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("buyer %q is defined twice", c.ID)
		}
		seen[c.ID] = true
		if c.City != BuyerAny && cities.City(c.City) == nil {
			return fmt.Errorf("buyer %q: unknown city %q", c.ID, c.City)
		}
		if c.Product != BuyerAny && market.Product(c.Product) == nil {
			return fmt.Errorf("buyer %q: unknown product %q", c.ID, c.Product)
		}
		if len(c.Size) != 2 || c.Size[0] <= 0 || c.Size[0] > c.Size[1] {
			return fmt.Errorf("buyer %q: size must be [lo, hi] with 0 < lo <= hi", c.ID)
		}
		if len(c.Days) != 2 || c.Days[0] < 1 || c.Days[0] > c.Days[1] {
			return fmt.Errorf("buyer %q: days must be [lo, hi] with 1 <= lo <= hi", c.ID)
		}
		if len(c.Premium) != 2 || c.Premium[0] <= 0 || c.Premium[0] > c.Premium[1] {
			return fmt.Errorf("buyer %q: premium must be [lo, hi] with 0 < lo <= hi", c.ID)
		}
		if c.Penalty < 0 || c.PenaltyCash < 0 || c.PenaltyCash > 1 || c.Heat <= 0 || c.Weight < 0 || c.UnlockCash < 0 || c.MinPrice < 0 {
			return fmt.Errorf("buyer %q: bad terms %+v", c.ID, c)
		}
	}
	return nil
}

// Wants reports whether a buyer would take a product: theirs, or any
// listed one whose home base price clears their floor.
func (c BuyerConfig) Wants(p ProductConfig) bool {
	if c.Product != BuyerAny {
		return c.Product == p.ID
	}
	return p.BasePrice >= c.MinPrice
}

// In reports whether a buyer deals in a city.
func (c BuyerConfig) In(city string) bool { return c.City == BuyerAny || c.City == city }

// BuyerSlots are what a pitch can name, filled from the contract when it
// is offered.
type BuyerSlots struct {
	Name    string
	Units   int
	Product string
	City    string
	Days    int
	Premium string // "1.4x"
}

// IDs lists the deck's ids in file order.
func (b BuyersConfig) IDs() []string {
	out := make([]string, 0, len(b.Deck))
	for _, c := range b.Deck {
		out = append(out, c.ID)
	}
	return slices.Clip(out)
}
