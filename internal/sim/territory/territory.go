// Package territory simulates the corners: which ones the player holds,
// which drift back to the street because nobody works them, and which get
// robbed. Corners are the demand pool the market serves; this sim only
// decides who is standing on them. The rival's moves on them are the
// rivals sim, which steps next.
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
	cfg content.CityConfig
}

// New builds a territory sim from the city config.
func New(cfg content.CityConfig) *Sim { return &Sim{cfg: cfg} }

func (s *Sim) Name() string { return "territory" }

// Tuning exposes the territory constants the UI needs to explain itself.
func (s *Sim) Tuning() content.TerritoryTuning { return s.cfg.Territory }

// Seed lays the city's corners out in a fresh world and stands the player
// on the starting one.
func (s *Sim) Seed(w *game.World) {
	w.Territory.Corners = StartingCorners(s.cfg)
	_ = w.Post(s.cfg.Territory.Start, game.You)
}

// Migrate brings a save from before corners existed up to date: the city
// is laid out and the player is on the starting corner, where the whole
// game used to happen.
func (s *Sim) Migrate(w *game.World) {
	if len(w.Territory.Corners) == 0 {
		s.Seed(w)
	}
}

// StartingCorners converts config into the corner list a new world starts
// with. Every corner is free.
func StartingCorners(cfg content.CityConfig) []game.Corner {
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
			ID: c.ID, Name: c.Name, X: c.X, Y: c.Y,
			Demand: c.Demand, Taste: taste, Heat: c.Heat, Risk: c.Risk,
			Owner: game.OwnerNone,
		})
	}
	return out
}

// RobberyChance is the chance a corner gets stuck up today: the base rate,
// the corner's risk, less what its enforcer takes off. A skill-100
// enforcer removes the full cut; a skill-0 one, half of it.
func (s *Sim) RobberyChance(w *game.World, c *game.Corner) float64 {
	tun := s.cfg.Territory
	p := tun.RobberyChance * c.Risk
	if m := w.Crew.Member(c.Enforcer); m != nil && c.Enforcer != 0 {
		p *= 1 - tun.EnforcerCut*(0.5+float64(m.Skill)/200)
	}
	return math.Max(0, math.Min(1, p))
}

// Step reports today's claims, drops corners nobody has worked for a
// while, and rolls for robberies on the corners that are worked.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Territory

	// Today's takings per product, for the robbers.
	revenue := map[string]int{}
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok {
			revenue[ps.Product] += ps.Revenue
		}
	}

	for i := range w.Territory.Corners {
		c := &w.Territory.Corners[i]
		if !c.Held() {
			continue
		}
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
			if tun.DriftDays > 0 && c.Idle >= tun.DriftDays {
				c.Owner, c.Runner, c.Enforcer, c.Idle, c.Since = game.OwnerNone, 0, 0, 0, t.Day
				t.Emit(events.CornerLost{Day: t.Day, Corner: c.ID, Name: c.Name, Reason: "idle", Owner: game.OwnerPlayer})
			}
			continue
		}
		c.Idle = 0

		// 2. Robbery. The stick-up takes a slice of today's takings and of
		// the stock, sized by this corner's share of what you work.
		if t.RNG.Float64() >= s.RobberyChance(w, c) {
			continue
		}
		ev := events.CornerRobbed{Day: t.Day, Corner: c.ID, Name: c.Name, StockLost: map[string]int{}}
		ids := append([]string(nil), w.Products...)
		sort.Strings(ids)
		for _, id := range ids {
			frac := 0.0
			if held := w.HeldShare(id); held > 0 {
				frac = c.Share(id) / held
			}
			ev.Cash += int(math.Round(float64(revenue[id]) * tun.RobberyCash * frac))
			if lost := int(math.Round(float64(w.Player.Stock[id]) * tun.RobberyStock * frac)); lost > 0 {
				w.Player.Stock[id] -= lost
				ev.StockLost[id] = lost
			}
		}
		ev.Cash = min(ev.Cash, w.Player.DirtyCash)
		if ev.Cash == 0 && len(ev.StockLost) == 0 {
			continue // nothing on the corner worth taking
		}
		w.Player.DirtyCash -= ev.Cash
		w.Stats.Robbed += ev.Cash
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
