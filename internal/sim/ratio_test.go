package sim_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
)

// A product put on a market (a new world, a city laid out, a product
// unlocked) is priced at market.toml's supplier_ratio off street until
// the market's next step prices it from the connects: a ratio written
// in the Go would not move with the file.
func TestListedSupplierPriceIsTheFilesRatio(t *testing.T) {
	cfg := content.MustLoad()
	cfg.Market.Market.SupplierRatio = 0.4
	w := game.NewWorld(1, logistics.StartingCities(cfg.City, cfg.Market), 0, 0)
	for _, cid := range w.CityOrder {
		for id, m := range w.Cities[cid].Market {
			if want := m.Price * 0.4; math.Abs(m.SupplierPrice-want) > 1e-9 {
				t.Errorf("%s %s: supplier %.4f, want %.4f", cid, id, m.SupplierPrice, want)
			}
		}
	}
}
