package content

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
)

// MarketConfig mirrors market.toml.
type MarketConfig struct {
	Market   MarketTuning    `toml:"market"`
	Products []ProductConfig `toml:"product"`
	Dial     DialTable       `toml:"dial"`
	Supply   SupplyTuning    `toml:"supply"`
	Standing StandingTuning  `toml:"standing"`
	Quality  QualityTuning   `toml:"quality"`
}

// QualityTuning is product quality (#47): where the connects sell and
// the price multiplier is 1 (Default), the multiplier at quality 0
// (LowMul) and 100 (HighMul), linear either side of Default; the
// corners' repeat business (RepeatStart to begin with, RepeatLoss off
// it a day the product sold there is under RepeatFloor, RepeatGain back
// any other day, never under RepeatMin); and the overdoses: hard product
// (OdProducts) under OdQuality rolls once per OdUnits sold in a city in
// a day at OdChance. A zero table is the feature in its box: Default 0
// reads as game.StreetQuality and every multiplier as 1.
type QualityTuning struct {
	Default     float64  `toml:"default"`
	LowMul      float64  `toml:"low_mul"`
	HighMul     float64  `toml:"high_mul"`
	RepeatStart float64  `toml:"repeat_start"`
	RepeatFloor float64  `toml:"repeat_floor"`
	RepeatLoss  float64  `toml:"repeat_loss"`
	RepeatGain  float64  `toml:"repeat_gain"`
	RepeatMin   float64  `toml:"repeat_min"`
	OdQuality   float64  `toml:"od_quality"`
	OdUnits     float64  `toml:"od_units"`
	OdChance    float64  `toml:"od_chance"`
	OdProducts  []string `toml:"od_products"`
}

// Mul is the price multiplier at a quality: LowMul at 0, 1 at Default,
// HighMul at 100, linear between; 1 everywhere with the table zero.
func (q QualityTuning) Mul(quality float64) float64 {
	d := q.Default
	if d <= 0 || d >= 100 {
		return 1
	}
	quality = clamp01(quality/100) * 100
	if quality < d {
		low := q.LowMul
		if low <= 0 {
			low = 1
		}
		return low + (1-low)*quality/d
	}
	high := q.HighMul
	if high <= 0 {
		high = 1
	}
	return 1 + (high-1)*(quality-d)/(100-d)
}

// validate refuses a [quality] table the sims cannot read: a default
// off the scale, a multiplier under zero, a repeat band outside 0..1
// or a product the ladder does not list; a zero table is the box.
func (q QualityTuning) validate(m MarketConfig) error {
	if q.Default < 0 || q.Default > 100 || q.LowMul < 0 || q.HighMul < 0 || q.OdQuality < 0 || q.OdUnits < 0 || q.OdChance < 0 || q.OdChance > 1 {
		return fmt.Errorf("bad [quality] table %+v", q)
	}
	for _, v := range []float64{q.RepeatStart, q.RepeatLoss, q.RepeatGain, q.RepeatMin} {
		if v < 0 || v > 1 {
			return fmt.Errorf("[quality] repeat_* %v (0..1)", v)
		}
	}
	if q.RepeatMin > q.RepeatStart && q.RepeatStart > 0 {
		return fmt.Errorf("[quality] repeat_min %v over repeat_start %v", q.RepeatMin, q.RepeatStart)
	}
	for _, id := range q.OdProducts {
		if m.Product(id) == nil {
			return fmt.Errorf("[quality] od_products names %q, not a product", id)
		}
	}
	for _, p := range m.Products {
		if p.CutMax < 0 || p.CutCost < 0 || p.CookCost < 0 {
			return fmt.Errorf("[[product]] %s: cut_max %v cut_cost %d cook_cost %d (not negative)", p.ID, p.CutMax, p.CutCost, p.CookCost)
		}
	}
	return nil
}

// Hard reports whether a product is one that overdoses.
func (q QualityTuning) Hard(product string) bool {
	for _, p := range q.OdProducts {
		if p == product {
			return true
		}
	}
	return false
}

// StandingTuning is the standing orders (#114): Cut is the share of a
// standing order's take the crew keep, the penalty the routine costs.
type StandingTuning struct {
	Cut float64 `toml:"cut"`
}

// SupplyTuning is the supply contracts (#113): Markup is the supplier's
// price for a standing order as a multiple of the price you would pay
// by hand, the penalty the routine costs; Float is the dirty cash a
// contract leaves in the till, folded by the tree like the wash's
// (World.Float). It is the street's own restock, so the laundering
// float is not its rule: a run starts with a fraction of that.
type SupplyTuning struct {
	Markup float64 `toml:"markup"`
	Float  int     `toml:"float"`
}

type MarketTuning struct {
	StartCash         int     `toml:"start_cash"`
	CarryLimit        int     `toml:"carry_limit"`
	HistoryDays       int     `toml:"history_days"`
	SupplierRatio     float64 `toml:"supplier_ratio"`
	BuyPricePressure  float64 `toml:"buy_price_pressure"`
	PriceFloorRatio   float64 `toml:"price_floor_ratio"`
	PriceCeilingRatio float64 `toml:"price_ceiling_ratio"`
	Reversion         float64 `toml:"reversion"`
	GlutDecay         float64 `toml:"glut_decay"`
	SaleImpact        float64 `toml:"sale_impact"`
	ShockChance       float64 `toml:"shock_chance"`
	SlumpChance       float64 `toml:"slump_chance"`
}

type ProductConfig struct {
	ID          string  `toml:"id"`
	Name        string  `toml:"name"`
	BasePrice   float64 `toml:"base_price"`
	Volatility  float64 `toml:"volatility"`
	Demand      float64 `toml:"demand"`
	DemandNoise float64 `toml:"demand_noise"`
	Heat        float64 `toml:"heat"`
	UnlockCash  int     `toml:"unlock_cash"` // supplier offers it once peak cash reaches this; 0 = from day one
	CutMax      float64 `toml:"cut_max"`     // the most a cut can add, as a ratio of the units (#47); 0 cannot be cut
	CutCost     int     `toml:"cut_cost"`    // dirty cash a unit the cut adds
	CookCost    int     `toml:"cook_cost"`   // dirty cash a unit a chemist cooks it for; 0 cannot be cooked
}

type DialTable struct {
	Quiet      DialConfig `toml:"quiet"`
	Normal     DialConfig `toml:"normal"`
	Aggressive DialConfig `toml:"aggressive"`
}

// For returns the tuning for a sell dial position: the market's fill,
// price and impact and the heat sim's weight read one row (#275).
func (t DialTable) For(d events.Dial) DialConfig {
	return threeWay(int(d), t.Quiet, t.Normal, t.Aggressive)
}

// threeWay picks a three-way dial's row by its position (#275): the low
// notch at 0, the high at 2 and the middle for anything else, as every
// three-way dial's String reads a value off its table (events/dials.go).
func threeWay[T any](pos int, low, mid, high T) T {
	switch pos {
	case 0:
		return low
	case 2:
		return high
	default:
		return mid
	}
}

type DialConfig struct {
	Fill   float64 `toml:"fill"`
	Price  float64 `toml:"price"`
	Impact float64 `toml:"impact"`
	Heat   float64 `toml:"heat"`
}

// Product returns the config for id, or nil.
func (m MarketConfig) Product(id string) *ProductConfig {
	for i := range m.Products {
		if m.Products[i].ID == id {
			return &m.Products[i]
		}
	}
	return nil
}

// validate refuses a market the sims cannot trade in: no products, a
// supply contract cheaper than the hand or with a negative float, a
// standing order whose crew keep all of it, and a [quality] table that
// does not read (QualityTuning.validate).
func (m MarketConfig) validate() error {
	if len(m.Products) == 0 {
		return fmt.Errorf("no products defined")
	}
	if m.Supply.Markup < 1 || m.Supply.Float < 0 {
		return fmt.Errorf("[supply] markup %v (at least 1) float %d (not negative)", m.Supply.Markup, m.Supply.Float)
	}
	if m.Standing.Cut < 0 || m.Standing.Cut >= 1 {
		return fmt.Errorf("[standing] cut %v (0 up to 1)", m.Standing.Cut)
	}
	return m.Quality.validate(m)
}
