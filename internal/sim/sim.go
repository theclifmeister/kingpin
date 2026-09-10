// Package sim assembles the simulations in their canonical step order.
package sim

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// Simulation is re-exported so callers can refer to it from this package.
type Simulation = game.Simulation

// Set is the constructed simulations plus handles the UI needs.
type Set struct {
	Market *market.Sim
	Crew   *crew.Sim
	Heat   *heat.Sim
	News   *news.Sim
}

// Default builds every simulation in the canonical order:
//
//	market -> logistics -> rivals -> crew -> heat -> laundering -> news
//
// Only market, crew, heat and news exist so far; the others slot in as they
// land.
func Default(cfg *content.Config) (*Set, []game.Simulation, error) {
	n, err := news.New(cfg.Headlines)
	if err != nil {
		return nil, nil, err
	}
	set := &Set{
		Market: market.New(cfg.Market),
		Crew:   crew.New(cfg.Crew, cfg.Names),
		Heat:   heat.New(cfg.Heat, cfg.Market),
		News:   n,
	}
	return set, []game.Simulation{set.Market, set.Crew, set.Heat, set.News}, nil
}

// Migrations is the chain that upgrades older saves to the current schema.
func (s *Set) Migrations() []game.Migration {
	return []game.Migration{
		{From: 1, Apply: s.Crew.Migrate}, // 1 -> 2: the crew arrived
	}
}

// NewWorld starts a fresh run from config. The hiring pool is drawn from
// the day-0 RNG so it is part of the seed like everything else.
func NewWorld(cfg *content.Config, seed uint64) *game.World {
	t := cfg.Market.Market
	w := game.NewWorld(seed, t.City, market.StartingProducts(cfg.Market), t.StartCash, t.CarryLimit)
	crew.New(cfg.Crew, cfg.Names).Seed(w, game.RNGFor(seed, 0))
	return w
}
