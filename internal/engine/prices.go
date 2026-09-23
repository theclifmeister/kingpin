package engine

import "github.com/theclifmeister/kingpin/internal/game"

// PriceFacts is what a product's price is doing in a city (#138, #298):
// the numbers every screen that shows a price reads, so a decision made
// in a dialog is made on the number the market shows.
type PriceFacts struct {
	Unit   float64 `json:"unit"`  // the supplier price the facts are read against; 0 where nobody sells it
	Delta  float64 `json:"delta"` // yesterday's close to today's, in percent
	Lo     float64 `json:"lo"`    // the range of the history
	Hi     float64 `json:"hi"`
	Margin float64 `json:"margin"` // the street over Unit, in percent; 0 where Unit is
}

// Facts is FactsAt against the market's own supplier price (the best
// available connect's).
func Facts(p *game.ProductMarket) PriceFacts { return FactsAt(p, p.SupplierPrice) }

// FactsAt reads a product market's price facts against a supplier price
// of the caller's: a chosen connect's (#72), whose price and margin are
// what a buy from it pays and makes, or 0 where they do not deal in it.
// A product the city's supplier does not sell (NoSupply) has no unit
// and no margin.
func FactsAt(p *game.ProductMarket, unit float64) PriceFacts {
	f := PriceFacts{Unit: unit, Lo: p.Price, Hi: p.Price}
	if n := len(p.History); n >= 2 {
		if from := p.History[n-2]; from != 0 {
			f.Delta = (p.History[n-1] - from) / from * 100
		}
	}
	for _, v := range p.History {
		f.Lo = min(f.Lo, v)
		f.Hi = min(max(f.Hi, v), 1e9)
	}
	if p.NoSupply {
		f.Unit = 0
	}
	if f.Unit > 0 {
		f.Margin = (p.Price - f.Unit) / f.Unit * 100
	}
	return f
}

// BuyRoom is what a buy from a connect can take right now (#356): Max,
// the most it takes (World.MaxBuy: the cash or their book, the room
// and their day, the least of the three), and the stash it lands in,
// Held of Capacity units, so a front end can draw the room before and
// after the buy.
type BuyRoom struct {
	Max      int `json:"max"`
	Held     int `json:"held"`
	Capacity int `json:"capacity"`
}

// MaxBuy is BuyRoom for a product from a connect, cash or on credit: a
// buy of Max never meets a refusal for the cash, the room or the
// connect's day (#356).
func (s *Session) MaxBuy(supplier, product string, credit bool) (BuyRoom, error) {
	sup := s.w.Supplier(supplier)
	if sup == nil {
		return BuyRoom{}, game.ErrNoSupplier
	}
	return BuyRoom{Max: s.w.MaxBuy(sup, product, credit), Held: s.w.StockIn(sup.City), Capacity: s.w.Capacity(sup.City)}, nil
}

// RestockPlan is what topping a city's stash up to days of demand would
// buy by hand now (World.RestockPlan, #356), keeping the supply
// contracts' float in the till as they do (market.Sim.Float): the
// lines a front end shows for review and then buys, one Buy each. It
// is never null on the wire.
func (s *Session) RestockPlan(city string, days float64) []game.RestockLine {
	plan := s.w.RestockPlan(city, days, s.set.Market.Float(s.w))
	if plan == nil {
		plan = []game.RestockLine{}
	}
	return plan
}
