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
