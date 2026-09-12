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
)

// Simulation is re-exported so callers can refer to it from this package.
type Simulation = game.Simulation

// Set is the constructed simulations plus handles the UI needs.
type Set struct {
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

// Default builds every simulation in the canonical order:
//
//	market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news
//
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
	n, err := news.New(cfg.Headlines, cfg.Dilemmas)
	if err != nil {
		return nil, nil, err
	}
	mk, err := market.New(cfg.Market, cfg.City, cfg.Routes.Shipping, cfg.Upgrades, cfg.Reputation.Effects, cfg.Buyers, cfg.Suppliers, cfg.Rivals.Pricewar)
	if err != nil {
		return nil, nil, err
	}
	set := &Set{
		Market:     mk,
		Logistics:  logistics.New(cfg.Routes, cfg.City, cfg.Market, cfg.Upgrades, cfg.Laundering.Laundering.Float),
		Territory:  territory.New(cfg.City, cfg.Upgrades, cfg.Houses.Houses),
		Rivals:     rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects, cfg.Upgrades),
		Crew:       crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects, cfg.Upgrades),
		Heat:       heat.New(cfg.Heat, cfg.Market, cfg.Routes.Shipping, cfg.Upgrades, cfg.Reputation.Effects, cfg.Crew.Lieutenant, cfg.Law, cfg.Houses.Houses),
		Law:        law.New(cfg.Law, cfg.Names),
		Laundering: laundering.New(cfg.Laundering, cfg.Crew, cfg.Upgrades),
		Reputation: reputation.New(cfg.Reputation),
		News:       n,
	}
	return set, []game.Simulation{set.Market, set.Logistics, set.Territory, set.Rivals, set.Crew, set.Heat, set.Law, set.Laundering, set.Reputation, set.News}, nil
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
	territory.New(cfg.City, cfg.Upgrades, cfg.Houses.Houses).Seed(w)
	rng := game.RNGFor(seed, 0)
	crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects, cfg.Upgrades).Seed(w, rng)
	rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects, cfg.Upgrades).Seed(w, rng)
	laundering.New(cfg.Laundering, cfg.Crew, cfg.Upgrades).Seed(w)
	law.New(cfg.Law, cfg.Names).Seed(w, rng)
	if mk, err := market.New(cfg.Market, cfg.City, cfg.Routes.Shipping, cfg.Upgrades, cfg.Reputation.Effects, cfg.Buyers, cfg.Suppliers, cfg.Rivals.Pricewar); err == nil {
		mk.Seed(w, rng)
	}
	return w
}
