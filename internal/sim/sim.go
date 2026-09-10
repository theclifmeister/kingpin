// Package sim assembles the simulations in their canonical step order.
package sim

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/news"
	"github.com/theclifmeister/kingpin/internal/sim/reputation"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// Simulation is re-exported so callers can refer to it from this package.
type Simulation = game.Simulation

// Set is the constructed simulations plus handles the UI needs.
type Set struct {
	Market     *market.Sim
	Territory  *territory.Sim
	Rivals     *rivals.Sim
	Crew       *crew.Sim
	Heat       *heat.Sim
	Laundering *laundering.Sim
	Reputation *reputation.Sim
	News       *news.Sim
}

// Default builds every simulation in the canonical order:
//
//	market -> logistics -> territory -> rivals -> crew -> heat -> laundering -> reputation -> news
//
// Territory settles who stands where before the rival moves on it; the
// rival goes before crew and heat so its strikes and tips land on today's
// loyalty and heat; laundering goes after heat so the wash works on what
// the day's stings left; reputation reads the whole day and goes before
// news so a band it crosses is a headline. Logistics slots in when it
// lands.
func Default(cfg *content.Config) (*Set, []game.Simulation, error) {
	n, err := news.New(cfg.Headlines, cfg.Dilemmas)
	if err != nil {
		return nil, nil, err
	}
	set := &Set{
		Market:     market.New(cfg.Market, cfg.Upgrades, cfg.Reputation.Effects),
		Territory:  territory.New(cfg.City),
		Rivals:     rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects),
		Crew:       crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects),
		Heat:       heat.New(cfg.Heat, cfg.Market, cfg.Upgrades, cfg.Reputation.Effects),
		Laundering: laundering.New(cfg.Laundering, cfg.Crew),
		Reputation: reputation.New(cfg.Reputation),
		News:       n,
	}
	return set, []game.Simulation{set.Market, set.Territory, set.Rivals, set.Crew, set.Heat, set.Laundering, set.Reputation, set.News}, nil
}

// Migrations is the chain that upgrades older saves to the current schema.
func (s *Set) Migrations() []game.Migration {
	return []game.Migration{
		{From: 1, Apply: s.Crew.Migrate},       // 1 -> 2: the crew arrived
		{From: 2, Apply: s.Territory.Migrate},  // 2 -> 3: the city got corners
		{From: 3, Apply: s.Rivals.Migrate},     // 3 -> 4: a rival came to town
		{From: 4, Apply: game.MigrateUpgrades}, // 4 -> 5: the upgrade tree, and the DA's file got a date
		{From: 5, Apply: s.Laundering.Migrate}, // 5 -> 6: fronts and the launder dial
	}
}

// NewWorld starts a fresh run from config: the city's corners with the
// player on the starting one, a hiring pool and a rival drawn from the
// day-0 RNG so they are part of the seed like everything else, and the
// launder dial at normal.
func NewWorld(cfg *content.Config, seed uint64) *game.World {
	t := cfg.Market.Market
	w := game.NewWorld(seed, t.City, market.StartingProducts(cfg.Market), t.StartCash, t.CarryLimit)
	territory.New(cfg.City).Seed(w)
	rng := game.RNGFor(seed, 0)
	crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects).Seed(w, rng)
	rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects).Seed(w, rng)
	laundering.New(cfg.Laundering, cfg.Crew).Seed(w)
	return w
}
