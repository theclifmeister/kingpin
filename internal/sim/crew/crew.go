// Package crew simulates the people on the payroll: who is looking for
// work, what they cost, how loyal they feel and what they do about it.
// Runners raise how much product the operation can move; disloyal crew skim
// the takings and eventually walk.
package crew

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Roles a candidate can be generated with, in order of how often.
var roles = []string{"runner", "runner", "enforcer"}

// Sim is the crew simulation.
type Sim struct {
	cfg   content.CrewConfig
	names []string
}

// New builds a crew sim from config and the name pool.
func New(cfg content.CrewConfig, names content.NamesConfig) *Sim {
	return &Sim{cfg: cfg, names: names.Crew}
}

func (s *Sim) Name() string { return "crew" }

// Tuning exposes the crew constants the UI needs to explain itself.
func (s *Sim) Tuning() content.CrewTuning { return s.cfg.Crew }

// MaxCrew is the roster cap.
func (s *Sim) MaxCrew() int { return s.cfg.Crew.MaxCrew }

// WageAt is what m costs per day at pay dial p.
func (s *Sim) WageAt(m game.CrewMember, p events.Pay) int {
	return int(math.Round(float64(m.Wage) * s.cfg.PayFor(p).Wage))
}

// Wages is the whole roster's daily bill at pay dial p.
func (s *Sim) Wages(w *game.World, p events.Pay) int {
	n := 0
	for _, m := range w.Crew.Members {
		n += s.WageAt(m, p)
	}
	return n
}

// Seed fills the hiring pool for a fresh world so the player can hire on
// day 0. It draws from rng, which the caller derives from the seed.
func (s *Sim) Seed(w *game.World, rng rand) {
	w.Crew.Pay = events.PayFair
	s.refill(w, rng)
}

// Migrate brings a save from before the crew existed up to date: an empty
// roster, fair pay and a hiring pool drawn from the current day's RNG.
func (s *Sim) Migrate(w *game.World) {
	if len(w.Crew.Candidates) == 0 {
		s.Seed(w, game.RNGFor(w.Seed, w.Day))
	}
}

// rand is the subset of *math/rand/v2.Rand the sim uses.
type rand interface {
	IntN(int) int
	Float64() float64
}

// Step pays wages, lets disloyal members skim, drifts loyalty, and handles
// quitting and the hiring pool. Skimming is checked on how people felt this
// morning, before today's drift, so a member never skims on a day they
// started above the threshold.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Crew
	c := &w.Crew

	for _, m := range c.HiredToday {
		t.Emit(events.CrewHired{Day: t.Day, Name: m.Name, Role: m.Role, Fee: m.Fee})
	}
	for _, m := range c.FiredToday {
		t.Emit(events.CrewFired{Day: t.Day, Name: m.Name, Role: m.Role})
	}

	// 1. Skimming, on this morning's loyalty.
	revenue := 0
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok {
			revenue += ps.Revenue
		}
	}
	enforcers := c.Role("enforcer")
	deter := math.Pow(1-s.cfg.Role["enforcer"].Deterrence, float64(enforcers))
	share := 0.0
	skimmers := 0
	for _, m := range c.Members {
		if m.Loyalty >= tun.SkimThreshold {
			continue
		}
		if t.RNG.Float64() < tun.SkimChance*deter {
			share += tun.SkimShare * (0.5 + float64(m.Greed)/100)
			skimmers++
		}
	}
	if skimmers > 0 && revenue > 0 {
		amount := int(math.Round(float64(revenue) * math.Min(share, tun.SkimCap)))
		amount = min(amount, w.Player.DirtyCash)
		if amount > 0 {
			w.Player.DirtyCash -= amount
			w.Stats.Skimmed += amount
			c.LastSkim = t.Day
			t.Emit(events.CrewSkimmed{Day: t.Day, Amount: amount, Skimmers: skimmers})
		}
	}

	// 2. Wages. Coming up short is remembered.
	short := 0
	if len(c.Members) > 0 {
		wages := s.Wages(w, c.Pay)
		paid := min(wages, w.Player.DirtyCash)
		short = wages - paid
		w.Player.DirtyCash -= paid
		w.Stats.Wages += paid
		t.Emit(events.CrewPaid{Day: t.Day, Pay: c.Pay, Wages: paid, Short: short})
	}

	// Wages are the only way money leaves without something coming back.
	// If they empty the till with nothing left to sell, the run is over:
	// there is no move that makes money from nothing.
	if w.Player.TotalStock() == 0 && float64(w.Player.DirtyCash) < cheapestUnit(w) {
		w.Over = &game.Ending{Day: t.Day, Cause: "broke", PeakCash: w.Stats.PeakCash}
		t.Emit(events.GameOver{Day: t.Day, Cause: "broke"})
		return
	}

	// 3. Loyalty drift: pay, greed, danger, firings, unpaid wages.
	danger := false
	for _, level := range []string{"sting", "raid"} {
		if d, ok := w.Heat.LastResponse[level]; ok && t.Day-d <= tun.DangerDays {
			danger = true
		}
	}
	shield := math.Pow(1-s.cfg.Role["enforcer"].Protection, float64(enforcers))
	base := s.cfg.PayFor(c.Pay).Loyalty
	base -= tun.FireLoyalty * float64(len(c.FiredToday))
	if short > 0 {
		base -= tun.UnpaidLoyalty
	}
	for i := range c.Members {
		m := &c.Members[i]
		d := base - tun.GreedDrift*float64(m.Greed)/100
		if danger {
			d -= tun.DangerLoyalty * float64(100-m.Nerve) / 100 * shield
		}
		m.Loyalty = math.Max(0, math.Min(100, m.Loyalty+d))
	}

	// 4. Quitting.
	kept := c.Members[:0]
	for _, m := range c.Members {
		if m.Loyalty <= tun.QuitThreshold {
			t.Emit(events.CrewQuit{Day: t.Day, Name: m.Name, Role: m.Role})
			continue
		}
		kept = append(kept, m)
	}
	c.Members = kept

	// 5. The hiring pool rotates on a schedule and refills after hires.
	if tun.PoolDays > 0 && t.Day-c.PoolDay >= tun.PoolDays {
		c.Candidates = nil
		c.PoolDay = t.Day
	}
	s.refill(w, t.RNG)
}

// refill tops the candidate pool up to size with fresh faces.
func (s *Sim) refill(w *game.World, rng rand) {
	tun := s.cfg.Crew
	for len(w.Crew.Candidates) < tun.Candidates {
		w.Crew.Candidates = append(w.Crew.Candidates, s.generate(w, rng))
	}
}

// generate rolls a new candidate whose name is not already in use.
func (s *Sim) generate(w *game.World, rng rand) game.CrewMember {
	tun := s.cfg.Crew
	used := map[string]bool{}
	for _, m := range w.Crew.Members {
		used[m.Name] = true
	}
	for _, m := range w.Crew.Candidates {
		used[m.Name] = true
	}
	var free []string
	for _, n := range s.names {
		if !used[n] {
			free = append(free, n)
		}
	}
	sort.Strings(free)
	name := "Nobody"
	if len(free) > 0 {
		name = free[rng.IntN(len(free))]
	}
	role := roles[rng.IntN(len(roles))]
	rc := s.cfg.Role[role]
	skill := 15 + rng.IntN(71)
	m := game.CrewMember{
		ID:      w.Crew.NextID + 1,
		Name:    name,
		Role:    role,
		Skill:   skill,
		Loyalty: float64(tun.StartLoyaltyMin + rng.IntN(max(1, tun.StartLoyaltyMax-tun.StartLoyaltyMin+1))),
		Greed:   5 + rng.IntN(91),
		Nerve:   5 + rng.IntN(91),
		Wage:    int(math.Round(rc.WageBase + rc.WagePerSkill*float64(skill))),
		Fee:     tun.HireFeeBase + int(math.Round(tun.HireFeePerSkill*float64(skill))),
	}
	if role == "runner" {
		m.Units = int(math.Round(tun.UnitsPerSkill * float64(skill)))
	}
	w.Crew.NextID = m.ID
	return m
}

// cheapestUnit is the lowest supplier price on the board.
func cheapestUnit(w *game.World) float64 {
	price := math.Inf(1)
	for _, m := range w.Market {
		price = math.Min(price, m.SupplierPrice)
	}
	if math.IsInf(price, 1) {
		return 0
	}
	return price
}
