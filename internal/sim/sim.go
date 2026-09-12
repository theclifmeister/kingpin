// Package sim assembles the simulations in their canonical step order.
package sim

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/law"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/market"
	"github.com/theclifmeister/kingpin/internal/sim/news"
	"github.com/theclifmeister/kingpin/internal/sim/reputation"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
	"github.com/theclifmeister/kingpin/internal/sim/world"
)

// Simulation is re-exported so callers can refer to it from this package.
type Simulation = game.Simulation

// Set is the constructed simulations plus handles the UI needs.
type Set struct {
	World      *world.Sim
	Market     *market.Sim
	Logistics  *logistics.Sim
	Territory  *territory.Sim
	Rivals     *rivals.Sim
	Crew       *crew.Sim
	Heat       *heat.Sim
	Law        *law.Sim
	Laundering *laundering.Sim
	Reputation *reputation.Sim
	News       *news.Sim
}

// Default builds every simulation in the canonical order, each from the
// one *content.Config (a constructor copies the slices it reads and no
// more, #144):
//
//	world -> market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news
//
// The world goes first (#44): an incident lands on the world before
// anything reads it, so every sim reacts the same day (the market prices
// the shock, the road finds the route shut, the law seats the chief).
// Logistics lands shipments after the day's sales, so what arrives sells
// tomorrow, and before heat, so a seizure is today's heat (the market
// reads it off the world tomorrow); territory settles who stands where
// before the rival moves on it; the rival goes before crew and heat so
// its strikes and tips land on today's loyalty and heat; the law goes
// after heat so the police answer on yesterday's pressure and the
// pressure counts today's violence; laundering goes after heat so the
// wash works on what the day's stings left; reputation reads the whole
// day and goes before news so a band it crosses is a headline.
func Default(cfg *content.Config) (*Set, []game.Simulation, error) {
	n, err := news.New(cfg)
	if err != nil {
		return nil, nil, err
	}
	mk, err := market.New(cfg)
	if err != nil {
		return nil, nil, err
	}
	set := &Set{
		World:      world.New(cfg),
		Market:     mk,
		Logistics:  logistics.New(cfg),
		Territory:  territory.New(cfg),
		Rivals:     rivals.New(cfg),
		Crew:       crew.New(cfg),
		Heat:       heat.New(cfg),
		Law:        law.New(cfg),
		Laundering: laundering.New(cfg),
		Reputation: reputation.New(cfg),
		News:       n,
	}
	return set, []game.Simulation{set.World, set.Market, set.Logistics, set.Territory, set.Rivals, set.Crew, set.Heat, set.Law, set.Laundering, set.Reputation, set.News}, nil
}

// Migrations is the chain that upgrades older saves to the current schema.
// A pre-7 save has no cities for the earlier steps to lay corners out in,
// so the territory step is repeated once the cities exist and seeds only
// what is missing.
func (s *Set) Migrations() []game.Migration {
	return []game.Migration{
		{From: 1, Apply: s.Crew.Migrate},       // 1 -> 2: the crew arrived
		{From: 2, Apply: s.Territory.Migrate},  // 2 -> 3: the city got corners
		{From: 3, Apply: s.Rivals.Migrate},     // 3 -> 4: a rival came to town
		{From: 4, Apply: game.MigrateUpgrades}, // 4 -> 5: the upgrade tree, and the DA's file got a date
		{From: 5, Apply: s.Laundering.Migrate}, // 5 -> 6: fronts and the launder dial
		{From: 6, Apply: func(w *game.World) { // 6 -> 7: a second city, routes between them, stashes
			s.Logistics.Migrate(w)
			s.Territory.Migrate(w)
		}},
		{From: 7, Apply: s.Rivals.MigrateDiplomacy}, // 7 -> 8: the rival's trust, deals and offers
		{From: 8, Apply: s.Law.Migrate},             // 8 -> 9: a chief and a DA took office
		{From: 9, Apply: game.MigrateFallGuys},      // 9 -> 10: the fall guy became a count (#117)
		{From: 10, Apply: s.Market.Migrate},         // 10 -> 11: the connects (#72), one a city at today's price
		{From: 11, Apply: game.MigrateHouses},       // 11 -> 12: the stash houses (#73), the old pile in a starter house
	}
}

// NewWorld starts a fresh run from config: every city's corners with the
// player on the starting one at home, a hiring pool, a rival, a chief and
// a DA and the connects (#72) drawn from the day-0 RNG so they are part
// of the seed like everything else, and the launder dial at normal.
func NewWorld(cfg *content.Config, seed uint64) *game.World {
	t := cfg.Market.Market
	w := game.NewWorld(seed, logistics.StartingCities(cfg.City, cfg.Market), t.StartCash, t.CarryLimit)
	territory.New(cfg).Seed(w)
	rng := game.RNGFor(seed, 0)
	crew.New(cfg).Seed(w, rng)
	rivals.New(cfg).Seed(w, rng)
	laundering.New(cfg).Seed(w)
	law.New(cfg).Seed(w, rng)
	if mk, err := market.New(cfg); err == nil {
		mk.Seed(w, rng)
	}
	return w
}
