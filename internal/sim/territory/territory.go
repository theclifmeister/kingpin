// Package territory simulates the corners of every city: which ones the
// player holds, which drift back to the street because nobody works them,
// and which get robbed. Corners are the demand pool the market serves;
// this sim only decides who is standing on them. The rival's moves on
// them are the rivals sim, which steps next.
package territory

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the territory simulation.
type Sim struct {
	cfg  content.CityConfig
	tree content.UpgradesConfig
}

// New builds a territory sim from the city config and the upgrade tree,
// which it folds for the two Street effects it reads (#119): the drift
// days and the robbery chance.
func New(cfg content.CityConfig, tree content.UpgradesConfig) *Sim { return &Sim{cfg: cfg, tree: tree} }

func (s *Sim) Name() string { return "territory" }

// Tuning exposes the territory constants the UI needs to explain itself.
func (s *Sim) Tuning() content.TerritoryTuning { return s.cfg.Territory }

// Effects is what the owned upgrades do to the street (#119), folded at
// the top of the step and for every number the map shows.
func (s *Sim) Effects(w *game.World) game.Effects { return game.FoldEffects(w, s.tree) }

// DriftDays is how many days a held corner nobody works lasts before it
// goes back to the street: drift_days plus the tree's drift_days_bonus
// (the corner boys). Zero in the file is never, and stays never.
func (s *Sim) DriftDays(w *game.World) int { return s.driftDays(s.Effects(w)) }

func (s *Sim) driftDays(fx game.Effects) int {
	if s.cfg.Territory.DriftDays <= 0 {
		return 0
	}
	return s.cfg.Territory.DriftDays + fx.DriftDaysBonus
}

// Seed lays every city's corners out in a fresh world and stands the
// player on the starting one.
func (s *Sim) Seed(w *game.World) {
	for _, c := range s.cfg.Cities {
		if city := w.Cities[c.ID]; city != nil {
			city.Corners = StartingCorners(c)
		}
	}
	_ = w.Post(s.cfg.Territory.Start, game.You)
}

// Migrate brings a save from before corners existed up to date: every
// city without corners is laid out, and if the player stands nowhere they
// are put on the starting corner, where the whole game used to happen.
// It is run for a pre-3 save and again after the cities arrived, so it
// only ever fills what is missing.
func (s *Sim) Migrate(w *game.World) {
	for _, c := range s.cfg.Cities {
		if city := w.Cities[c.ID]; city != nil && len(city.Corners) == 0 {
			city.Corners = StartingCorners(c)
		}
	}
	if w.PostOf(game.You) == nil && w.Player.Location == s.cfg.Home().ID {
		_ = w.Post(s.cfg.Territory.Start, game.You)
	}
}

// StartingCorners converts a city's config into the corner list it starts
// with. Every corner is free.
func StartingCorners(cfg content.CityEntry) []game.Corner {
	out := make([]game.Corner, 0, len(cfg.Corners))
	for _, c := range cfg.Corners {
		var taste map[string]float64 // nil when the corner has no taste: gob drops empty maps anyway
		if len(c.Taste) > 0 {
			taste = make(map[string]float64, len(c.Taste))
			for k, v := range c.Taste {
				taste[k] = v
			}
		}
		out = append(out, game.Corner{
			ID: c.ID, City: cfg.ID, Name: c.Name, X: c.X, Y: c.Y,
			Demand: c.Demand, Taste: taste, Heat: c.Heat, Risk: c.Risk,
			Owner: game.OwnerNone,
		})
	}
	return out
}

// RobberyChance is the chance a corner gets stuck up today: the base rate,
// the corner's risk, the tree's robbery_mul (the watchmen, the dogs),
// less what its enforcer takes off. A skill-100 enforcer removes the
// full cut; a skill-0 one, half of it.
func (s *Sim) RobberyChance(w *game.World, c *game.Corner) float64 {
	return s.robberyChance(s.Effects(w), w, c)
}

func (s *Sim) robberyChance(fx game.Effects, w *game.World, c *game.Corner) float64 {
	tun := s.cfg.Territory
	p := tun.RobberyChance * c.Risk * fx.RobberyMul
	if m := w.Crew.Member(c.Enforcer); m != nil && c.Enforcer != 0 {
		p *= 1 - tun.EnforcerCut*(0.5+float64(m.Skill)/200)
	}
	return math.Max(0, math.Min(1, p))
}

// Step reports today's claims, drops corners nobody has worked for a
// while, and rolls for robberies on the corners that are worked, city by
// city.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	fx := s.Effects(w)
	// Today's takings per city and product, for the robbers.
	revenue := map[string]int{}
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok {
			revenue[game.OrderKey(ps.City, ps.Product)] += ps.Revenue
		}
	}
	for _, cid := range w.CityOrder {
		// Home rolls off the day's stream, every other city off its own.
		rng := t.RNG
		if cid != w.Home().ID {
			rng = t.Sub("territory:" + cid)
		}
		s.step(w, t, rng, fx, w.Cities[cid], revenue)
	}
}

// rand is the subset of *math/rand/v2.Rand the sim uses.
type rand interface {
	Float64() float64
}

func (s *Sim) step(w *game.World, t *game.Tick, rng rand, fx game.Effects, city *game.City, revenue map[string]int) {
	tun := s.cfg.Territory
	drift := s.driftDays(fx)
	for i := range city.Corners {
		c := &city.Corners[i]
		if !c.Held() {
			// Off the street long enough, a corner's stick-ups are
			// forgotten: a lieutenant who gave it up will try it again.
			if c.Robbed > 0 && t.Day-c.Since >= drift {
				c.Robbed = 0
			}
			continue
		}
		// Held once is held for the record: the rival's grace period
		// (#60) leaves what you have worked alone.
		c.Yours = true
		// Posts must point at people still on the payroll.
		if c.Runner != 0 && c.Runner != game.You && w.Crew.Member(c.Runner) == nil {
			c.Runner = 0
		}
		if c.Enforcer != 0 && w.Crew.Member(c.Enforcer) == nil {
			c.Enforcer = 0
		}
		if c.Since == t.Day-1 {
			t.Emit(events.CornerClaimed{Day: t.Day, Corner: c.ID, Name: c.Name, Worker: s.worker(w, c.Runner)})
		}

		// 1. Drift: a corner nobody works goes back to the street.
		if c.Runner == 0 {
			c.Idle++
			if drift > 0 && c.Idle >= drift {
				c.Owner, c.Runner, c.Enforcer, c.Idle, c.Since = game.OwnerNone, 0, 0, 0, t.Day
				t.Emit(events.CornerLost{Day: t.Day, Corner: c.ID, Name: c.Name, Reason: "idle", Owner: game.OwnerPlayer})
			}
			continue
		}
		c.Idle = 0

		// 2. Robbery. The stick-up takes a slice of today's takings and of
		// the stock, sized by this corner's share of what you work.
		if rng.Float64() >= s.robberyChance(fx, w, c) {
			continue
		}
		ev := events.CornerRobbed{Day: t.Day, Corner: c.ID, Name: c.Name, StockLost: map[string]int{}}
		ids := append([]string(nil), w.Products...)
		sort.Strings(ids)
		stash := w.Stash(city.ID)
		for _, id := range ids {
			frac := 0.0
			if held := w.HeldShare(city.ID, id); held > 0 {
				frac = c.Share(id) / held
			}
			ev.Cash += int(math.Round(float64(revenue[game.OrderKey(city.ID, id)]) * tun.RobberyCash * frac))
			if lost := int(math.Round(float64(stash[id]) * tun.RobberyStock * frac)); lost > 0 {
				stash[id] -= lost
				ev.StockLost[id] = lost
			}
		}
		ev.Cash = min(ev.Cash, w.Player.DirtyCash)
		if ev.Cash == 0 && len(ev.StockLost) == 0 {
			continue // nothing on the corner worth taking
		}
		w.Player.DirtyCash -= ev.Cash
		w.Stats.Robbed += ev.Cash
		c.Robbed++
		t.Emit(ev)
	}
}

// worker names whoever is on a corner for a headline.
func (s *Sim) worker(w *game.World, id int) string {
	switch {
	case id == game.You:
		return "you"
	case id == 0:
		return "nobody"
	}
	if m := w.Crew.Member(id); m != nil {
		return m.Name
	}
	return "somebody"
}
