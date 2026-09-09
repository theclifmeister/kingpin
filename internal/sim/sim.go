// Package sim assembles the simulations in their canonical step order.
package sim

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// Simulation is re-exported so callers can refer to it from this package.
type Simulation = game.Simulation

// Set is the constructed simulations plus handles the UI needs.
type Set struct {
	Market *market.Sim
	Heat   *heat.Sim
	News   *news.Sim
}

// Default builds every simulation in the canonical order:
//
//	market -> logistics -> rivals -> crew -> heat -> laundering -> news
//
// Only market, heat and news exist so far; the others slot in as they land.
func Default(cfg *content.Config) (*Set, []game.Simulation, error) {
	n, err := news.New(cfg.Headlines)
	if err != nil {
		return nil, nil, err
	}
	set := &Set{
		Market: market.New(cfg.Market),
		Heat:   heat.New(cfg.Heat, cfg.Market),
		News:   n,
	}
	return set, []game.Simulation{set.Market, set.Heat, set.News}, nil
}

// NewWorld starts a fresh run from config.
func NewWorld(cfg *content.Config, seed uint64) *game.World {
	t := cfg.Market.Market
	return game.NewWorld(seed, t.City, market.StartingProducts(cfg.Market), t.StartCash, t.CarryLimit)
}
