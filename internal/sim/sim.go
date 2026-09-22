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
		{From: 12, Apply: s.Market.MigrateLots},     // 12 -> 13: quality (#47), the old stock at the default and every corner's customers coming back
		{From: 13, Apply: s.Crew.MigrateAges},       // 13 -> 14: crew life (#46), every member and candidate given an age off the seed
		{From: 14, Apply: s.Rivals.MigrateFactions}, // 14 -> 15: the table (#43), the one rival seated first and the rest seeded off the seed
		{From: 15, Apply: game.MigrateBooks},        // 15 -> 16: intel (#45), the books a scout read filed as facts
	}
}

// NewWorld starts a fresh run from config: every city's corners with the
// player on the starting one at home, a hiring pool, a rival, a chief and
// a DA and the connects (#72) drawn from the day-0 RNG so they are part
// of the seed like everything else, and the launder dial at normal. It
// is NewWorldWith as the default character with nothing on: the run as
// it is, byte for byte (#50).
func NewWorld(cfg *content.Config, seed uint64) *game.World {
	return NewWorldWith(cfg, seed, game.Start{})
}

// NewWorldWith is NewWorld as a start (#50, docs/profile.md): the
// character's start from characters.toml applied once, here, after the
// sims have seeded the world on the day-0 stream and before the first
// day steps, and the hard DA seated. No sim reads the start: what a
// character changes is on the world on day 0 (a member on the payroll,
// a node owned, a product listed, where you stand, the reputation, a
// fact in the file) and the run plays on from there as any run would.
// The character's own dice are Tick{Day 0}.Sub("character"), never the
// home stream, so the pool, the table and the law are the seed's
// whatever the character; the default character (an empty or the
// first row's id) is stamped as "" and leaves the world untouched.
func NewWorldWith(cfg *content.Config, seed uint64, start game.Start) *game.World {
	t := cfg.Market.Market
	w := game.NewWorld(seed, logistics.StartingCities(cfg.City, cfg.Market), t.StartCash, t.CarryLimit)
	if cfg.Characters.IsDefault(start.Character) {
		start.Character = ""
	}
	w.Start = start
	ch := cfg.Characters.Character(start.Character)
	territory.New(cfg).Seed(w)
	rng := game.RNGFor(seed, 0)
	cs := crew.New(cfg)
	cs.Seed(w, rng)
	rivals.New(cfg).Seed(w, rng)
	laundering.New(cfg).Seed(w)
	law.New(cfg).Seed(w, rng)
	if start.HardDA {
		// The personalities the law sim would have drawn, set and not
		// pinned: the elections and the chief's term run as they do.
		w.Law.DA.Stance = "law_and_order"
		w.Law.Chief.Personality = "zealous"
	}
	mk, err := market.New(cfg)
	if err == nil {
		if ch != nil {
			for _, id := range ch.Start.Products {
				mk.List(w, id) // before the connects are priced, so they price it
			}
		}
		mk.Seed(w, rng)
	}
	if ch != nil {
		applyStart(cfg, w, cs, ch.Start)
	}
	return w
}

// applyStart puts a character's start on the world on day 0.
func applyStart(cfg *content.Config, w *game.World, cs *crew.Sim, s content.StartConfig) {
	dice := (&game.Tick{Day: 0, Seed: w.Seed}).Sub("character")
	var joined []game.CrewMember
	for _, role := range s.Crew {
		joined = append(joined, cs.Join(w, role, dice))
	}
	for _, id := range s.Upgrades {
		grant(cfg.Upgrades, w, id)
	}
	if s.City != "" {
		// Home's starting corner goes back to the street: you stand on
		// the character's, in its city.
		if c := w.PostOf(game.You); c != nil {
			_ = w.Abandon(c.ID)
		}
		_ = w.Travel(s.City)
		_ = w.Post(s.Corner, game.You)
	}
	// The corners held on day 0 (#232): yours from the start, the start
	// crew posted on them in order (a runner works one, an enforcer
	// guards one; a corner nobody works drifts as any does).
	for i, id := range s.Corners {
		c := w.Corner(id)
		if c == nil || c.Owner == game.OwnerRival {
			continue
		}
		c.Owner, c.Faction, c.Since, c.Idle = game.OwnerPlayer, "", 0, 0
		if i < len(joined) {
			_ = w.Post(id, joined[i].ID)
		}
	}
	r := &w.Player.Reputation
	r.Fear, r.Respect, r.Notoriety = s.Reputation.Fear, s.Reputation.Respect, s.Reputation.Notoriety
	if s.KnowChief {
		w.Learn(game.Fact{Subject: game.SubjectChief, Kind: game.FactPersonality, Value: w.Law.Chief.Personality, Confidence: 1, Day: 0, Source: game.SourceSeen})
	}
}

// grant gives the world a node of the tree for nothing, its
// prerequisites first, the way harness.Own does: through BuyUpgrade,
// so a carry bonus or a supplier discount lands as a purchase would.
func grant(tree content.UpgradesConfig, w *game.World, id string) {
	u := tree.Upgrade(id)
	if u == nil || w.Owns(id) {
		return
	}
	for _, req := range u.Requires {
		grant(tree, w, req)
	}
	if u.Clean {
		w.Player.CleanCash += u.Cost
	} else {
		w.Player.DirtyCash += u.Cost
	}
	_, _ = w.BuyUpgrade(tree, id)
	w.Today.UpgradesToday = nil // a start is not a purchase to report
}
