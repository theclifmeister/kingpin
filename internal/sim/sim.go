// Package sim assembles the simulations in their canonical step order.
package sim

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/news"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// Simulation is re-exported so callers can refer to it from this package.
type Simulation = game.Simulation

// Set is the constructed simulations plus handles the UI needs.
type Set struct {
	Market    *market.Sim
	Territory *territory.Sim
	Rivals    *rivals.Sim
	Crew      *crew.Sim
	Heat      *heat.Sim
	News      *news.Sim
}

// Default builds every simulation in the canonical order:
//
//	market -> logistics -> territory -> rivals -> crew -> heat -> laundering -> news
//
// Territory settles who stands where before the rival moves on it; the
// rival goes before crew and heat so its strikes and tips land on today's
// loyalty and heat. Logistics and laundering slot in as they land.
func Default(cfg *content.Config) (*Set, []game.Simulation, error) {
	n, err := news.New(cfg.Headlines)
	if err != nil {
		return nil, nil, err
	}
	set := &Set{
		Market:    market.New(cfg.Market),
		Territory: territory.New(cfg.City),
		Rivals:    rivals.New(cfg.Rivals, cfg.Names),
		Crew:      crew.New(cfg.Crew, cfg.Names),
		Heat:      heat.New(cfg.Heat, cfg.Market),
		News:      n,
	}
	return set, []game.Simulation{set.Market, set.Territory, set.Rivals, set.Crew, set.Heat, set.News}, nil
}

// Migrations is the chain that upgrades older saves to the current schema.
func (s *Set) Migrations() []game.Migration {
	return []game.Migration{
		{From: 1, Apply: s.Crew.Migrate},      // 1 -> 2: the crew arrived
		{From: 2, Apply: s.Territory.Migrate}, // 2 -> 3: the city got corners
		{From: 3, Apply: s.Rivals.Migrate},    // 3 -> 4: a rival came to town
	}
}

// NewWorld starts a fresh run from config: the city's corners with the
// player on the starting one, and a hiring pool and a rival drawn from the
// day-0 RNG so they are part of the seed like everything else.
func NewWorld(cfg *content.Config, seed uint64) *game.World {
	t := cfg.Market.Market
	w := game.NewWorld(seed, t.City, market.StartingProducts(cfg.Market), t.StartCash, t.CarryLimit)
	territory.New(cfg.City).Seed(w)
	rng := game.RNGFor(seed, 0)
	crew.New(cfg.Crew, cfg.Names).Seed(w, rng)
	rivals.New(cfg.Rivals, cfg.Names).Seed(w, rng)
	return w
}
