package crew

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// rotate empties the hiring pool when its schedule comes round.
func (s *Sim) rotate(n *night) {
	t, c, fx := n.t, n.c, n.fx
	// The hiring pool rotates on a schedule (the refill is the last
	// thing the step does): before crew life, so a kin recommended
	// tonight (#46) is a face in the morning's pool and not one the
	// rotation wiped.
	if days := s.poolDays(fx); days > 0 && t.Day-c.PoolDay >= days {
		c.Candidates = nil
		c.PoolDay = t.Day
	}
}

// pool refills the hiring pool, each extra face off its own stream.
func (s *Sim) pool(n *night) {
	w, t, fx := n.w, n.t, n.fx
	// The hiring pool refills after hires and the rotation.
	var side, drv game.Rand
	if FixersWanted(w) {
		side = t.Sub(game.StreamFixer)
	}
	if DriversWanted(w) {
		drv = t.Sub(game.StreamDriver)
	}
	s.refill(w, t.RNG, t.Sub(game.StreamChemist), side, drv, t.Sub(game.StreamLife), fx)
}

// refill tops the candidate pool up to size with fresh faces, and, once
// meth is on the ladder, adds the one chemist looking for work beside
// them (#47), drawn off the chemist's own stream (nil at seed: the
// ladder has no meth on day 0) with a name from their own list, so the
// faces the home stream draws are the faces it always drew. side is the
// fixers' stream (#42), nil while nobody has paid an envelope; drv the
// drivers' (#46), nil until a route has run, the one driver looking
// for work the chemist's pattern again; life is the life stream, the
// ages (#46). The kin faces (#46) are extra the same way.
func (s *Sim) refill(w *game.World, rng, chem, side, drv, life game.Rand, fx game.Effects) {
	faces := 0
	for _, c := range w.Crew.Candidates {
		if !extra(c) && !former(c) {
			faces++
		}
	}
	for ; faces < s.candidates(fx); faces++ {
		w.Crew.Candidates = append(w.Crew.Candidates, s.generate(w, rng, side, life, fx))
	}
	if chem != nil && s.ChemistsWanted(w) && !s.chemistLooking(w) {
		w.Crew.Candidates = append(w.Crew.Candidates, s.chemist(w, chem, life, fx))
	}
	if drv != nil && DriversWanted(w) && !driverLooking(w) {
		w.Crew.Candidates = append(w.Crew.Candidates, s.driver(w, drv, life, fx))
	}
}

// generate rolls a new candidate whose name is not already in use. The
// tree's skill_bonus and start_loyalty_bonus land on the roll, never
// over 100, and the fee is priced on the skill they arrive with; none
// of it adds a draw, so a run owning nothing rolls the pool it always
// did. The age (#46) is the life stream's draw, not the home stream's.
func (s *Sim) generate(w *game.World, rng, side, life game.Rand, fx game.Effects) game.CrewMember {
	name := s.pickName(w, s.names, "Nobody", rng)
	roles := rolesFor(w)
	role := roles[rng.IntN(len(roles))]
	// Lieutenants are rare, and only come looking once there is a second
	// city to hand over; the roll is only made then, so a run in one city
	// draws the same pool it always did.
	personality := ""
	if lt := s.cfg.Lieutenant; LieutenantsWanted(w) && rng.Float64() < lt.Chance {
		role = game.RoleLieutenant
		personality = content.LieutenantPersonalities[rng.IntN(len(content.LieutenantPersonalities))]
	}
	// Fixers (#42) come looking once you have paid somebody, on their
	// own stream, so a run that pays nobody draws the pool it always
	// did; they never displace a lieutenant.
	if side != nil && role != game.RoleLieutenant && side.Float64() < s.cfg.Role[game.RoleFixer].Chance {
		role = game.RoleFixer
	}
	m := s.roll(w, name, role, rng, fx)
	m.Fee = s.hireFee(w, m.Skill, fx)
	m.Personality = personality // "" for anyone but a lieutenant
	m.Age = s.age(life)
	return m
}

// chemist rolls the chemist looking for work (#47): the generate roll
// off the chemist's stream, with a name from the chemists' list.
func (s *Sim) chemist(w *game.World, rng, life game.Rand, fx game.Effects) game.CrewMember {
	m := s.roll(w, s.pickName(w, s.chemists, "The Chemist", rng), game.RoleChemist, rng, fx)
	m.Fee = s.hireFee(w, m.Skill, fx)
	m.Age = s.age(life)
	return m
}

// chemistLooking reports whether the pool holds a chemist.
func (s *Sim) chemistLooking(w *game.World) bool {
	for _, c := range w.Crew.Candidates {
		if c.Role == game.RoleChemist {
			return true
		}
	}
	return false
}

// pickName is a name off pool that nobody on the payroll or in the pool
// has, drawn on rng from the free ones sorted, or fallback when the pool
// is spent (no draw then).
func (s *Sim) pickName(w *game.World, pool []string, fallback string, rng game.Rand) string {
	used := map[string]bool{}
	for _, m := range w.Crew.Members {
		used[m.Name] = true
	}
	for _, m := range w.Crew.Candidates {
		used[m.Name] = true
	}
	var free []string
	for _, n := range pool {
		if !used[n] {
			free = append(free, n)
		}
	}
	sort.Strings(free)
	if len(free) == 0 {
		return fallback
	}
	return free[rng.IntN(len(free))]
}

// roll is a new member of role called name, with the next ID: skill,
// loyalty, greed and nerve drawn on rng in that order, the tree's
// skill_bonus and start_loyalty_bonus on the roll and never over 100,
// the wage off the role's table and a runner's units off the skill. The
// fee, the age and anything else are the caller's: a hire prices one, a
// character's start (Join) has none, and each draws its age on its own
// dice.
func (s *Sim) roll(w *game.World, name, role string, rng game.Rand, fx game.Effects) game.CrewMember {
	tun := s.cfg.Crew
	rc := s.cfg.Role[role]
	skill := min(100, 15+rng.IntN(71)+fx.SkillBonus)
	m := game.CrewMember{
		ID:      w.Crew.NextID + 1,
		Name:    name,
		Role:    role,
		Skill:   skill,
		Loyalty: float64(min(100, tun.StartLoyaltyMin+rng.IntN(max(1, tun.StartLoyaltyMax-tun.StartLoyaltyMin+1))+fx.StartLoyaltyBonus)),
		Greed:   5 + rng.IntN(91),
		Nerve:   5 + rng.IntN(91),
		Wage:    int(math.Round(rc.WageBase + rc.WagePerSkill*float64(skill))),
	}
	if role == game.RoleRunner {
		m.Units = int(math.Round(tun.UnitsPerSkill * float64(skill)))
	}
	w.Crew.NextID = m.ID
	return m
}

// Join puts a member of the role on the payroll on day 0, hired for
// nothing (#50, a character's start): the roll generate makes with the
// role fixed, the name from the pool's list for it (the chemists' for
// a chemist, the drivers' for a driver, the crew's for the rest), the
// stats and the age off rng, which is the character's own stream and
// never the pool's, so the faces the pool deals are the faces it
// always dealt. A lieutenant joins unassigned with a temper off the
// same dice.
func (s *Sim) Join(w *game.World, role string, rng game.Rand) game.CrewMember {
	fx := game.FoldEffects(w, s.tree)
	pool := s.names
	switch role {
	case game.RoleChemist:
		pool = s.chemists
	case game.RoleDriver:
		pool = s.drivers
	}
	m := s.roll(w, s.pickName(w, pool, "Nobody", rng), role, rng, fx)
	m.Hired = w.Day
	m.Age = s.age(rng)
	if role == game.RoleLieutenant {
		m.Personality = content.LieutenantPersonalities[rng.IntN(len(content.LieutenantPersonalities))]
	}
	w.Crew.Members = append(w.Crew.Members, m)
	return m
}
