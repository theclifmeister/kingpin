package crew

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Crew life (#46): kin, ageing, arrests and getting shot. Everything
// here rolls off the life stream (Tick.Sub("life")) and never the home
// city's, so a run with crew.toml's [life] table boxed (harness.NoLife)
// is byte-for-byte the run before the feature: the stream is drawn but
// nothing acts on it. What the heat sim did last night reaches the
// crew through Heat.Sweep (heat steps after crew, so its night is our
// morning, the way the law's is heat's, #42); what the rival did
// tonight is on the tick's events (rivals step before crew).

// Life exposes the life tuning the UI needs to explain itself.
func (s *Sim) Life() content.LifeTuning { return s.cfg.Life }

// BailCost is the clean cash that walks m out of a cell tomorrow: the
// role's bail.
func (s *Sim) BailCost(m game.CrewMember) int { return s.cfg.Role[m.Role].Bail }

// Retiring reports whether m's next birthday is the one they retire on.
func (s *Sim) Retiring(m game.CrewMember) bool {
	life := s.cfg.Life
	return life.On() && m.Age > 0 && m.Age+1 >= life.RetireAge
}

// Birthday is the day m turns a year older: every year_days on the
// payroll from the day they signed; 0 with ageing off.
func (s *Sim) Birthday(m game.CrewMember, day int) int {
	life := s.cfg.Life
	if !life.On() {
		return 0
	}
	since := day - m.Hired
	return day + life.YearDays - since%life.YearDays
}

// DriversWanted reports whether drivers come looking for work (#46):
// once a route has run.
func DriversWanted(w *game.World) bool { return w.Stats.Shipments > 0 }

// age is a candidate's age at generation, off the life stream; 0 with
// the table boxed.
func (s *Sim) age(life rand) int {
	l := s.cfg.Life
	if !l.On() {
		return 0
	}
	return l.AgeMin + life.IntN(max(1, l.AgeMax-l.AgeMin+1))
}

// MigrateAges gives every member and candidate of a save from before
// ages an age (13 -> 14), off the seed and the day and never the home
// stream, so the run replays as it did.
func (s *Sim) MigrateAges(w *game.World) {
	life := (&game.Tick{Day: w.Day, Seed: w.Seed}).Sub("ages")
	for i := range w.Crew.Members {
		if w.Crew.Members[i].Age == 0 {
			w.Crew.Members[i].Age = s.age(life)
		}
	}
	for i := range w.Crew.Candidates {
		if w.Crew.Candidates[i].Age == 0 {
			w.Crew.Candidates[i].Age = s.age(life)
		}
	}
}

// life is the crew's day beyond the roster: the cells open, the sweep
// last night rolls arrests, the strikes and pushes tonight wound and
// kill, birthdays and retirements, and the kin's loyalty follows what
// was done to theirs. The order fixes what the life stream rolls.
func (s *Sim) life(w *game.World, t *game.Tick, fx game.Effects) {
	life := s.cfg.Life
	if !life.On() {
		return
	}
	rng := t.Sub("life")
	c := &w.Crew

	// 1. What you did today reaches the kin (the firings, the pay-offs
	// and the bail money are on the scratch): a firing costs each kin
	// kin_loyalty unless the fired one was talking, a pay-off lifts it.
	for _, m := range c.FiredToday {
		if !m.Informant {
			s.kinLoyalty(w, m, -life.KinLoyalty)
		}
	}
	for _, p := range c.PaidOffToday {
		if m := c.Member(p.ID); m != nil {
			s.kinLoyalty(w, *m, life.KinLoyalty)
		}
	}
	for _, b := range c.BailedToday {
		t.Emit(events.CrewBailed{Day: t.Day, ID: b.ID, Name: b.Name, Cost: b.Cost})
	}

	// 2. The cells open and the wounded get up. Bailed, the release
	// buys loyalty; not, they come back under the informant line and
	// roll the turn at once: the DA had them for jail_days and your
	// dealing made the witness (#27 bends for something you did).
	inf := s.cfg.Informant
	for i := range c.Members {
		m := &c.Members[i]
		if m.JailedUntil > 0 && m.JailedUntil <= t.Day {
			ev := events.CrewReleased{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Bailed: m.Bailed}
			if m.Bailed {
				m.Loyalty = math.Min(100, m.Loyalty+life.BailLoyalty)
			} else {
				m.Loyalty = math.Max(0, math.Min(m.Loyalty, inf.Loyalty-life.JailLoyalty))
				if !m.Informant && rng.Float64() < life.ReleaseTurn*fx.InformantChanceMul {
					m.Informant = true
					w.Stats.Informants++
					t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: m.ID, Name: m.Name})
				}
			}
			m.JailedUntil, m.Bailed = 0, false
			t.Emit(ev)
		}
		if m.WoundedUntil > 0 && m.WoundedUntil <= t.Day {
			m.WoundedUntil = 0
			t.Emit(events.CrewRecovered{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role})
		}
	}

	// 3. Last night's sweep: each runner and enforcer who stood on a
	// corner the police hit rolls arrest_chance; a raid that took stock
	// is the lab's too, and the chemist rolls the same. The driver of a
	// shipment seized today (logistics steps before crew) goes with it,
	// no dice.
	if sw := w.Heat.Sweep; sw.Day > 0 && sw.Day == t.Day-1 && life.ArrestChance > 0 {
		for _, id := range sw.Crew {
			m := c.Member(id)
			if m == nil || m.Jailed(t.Day) {
				continue
			}
			if rng.Float64() < life.ArrestChance {
				s.jail(w, t, m, sw.City, w.PostOf(m.ID), "")
			}
		}
		if content.Rank(sw.Level) >= content.Rank(content.Raid) && sw.Units > 0 { // a raid, or the task force (#48)
			if m := c.Chemist(); m != nil && !m.Jailed(t.Day) && rng.Float64() < life.ArrestChance {
				s.jail(w, t, m, sw.City, nil, "")
			}
		}
	}
	for _, e := range t.Events() {
		if ev, ok := e.(events.ShipmentSeized); ok && ev.Driver != 0 {
			if m := c.Member(ev.Driver); m != nil && !m.Jailed(t.Day) {
				s.jail(w, t, m, ev.From, nil, ev.Route)
			}
		}
	}

	// 4. Shot. Your enforcers going in on a strike, and the guard on a
	// corner the rival pushed and did not take: a wound or a death by
	// wound_chance and kill_chance times the force's multiplier for a
	// strike and the rival's personality's for a push; the rival's
	// muscle dies the same way on the other side of it, and the count
	// (Stats.Bodies) is both. A death removes them to Fallen and their
	// kin take it hardest.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			mul := life.ForceMul(ev.Force.String())
			var went []int
			for _, m := range c.Members {
				if m.Role == "enforcer" && m.Fit(t.Day) {
					went = append(went, m.ID)
				}
			}
			if len(went) > 0 {
				s.shoot(w, t, rng, went[rng.IntN(len(went))], w.Corner(ev.Corner), mul, true)
			}
			s.theirs(w, t, rng, w.Corner(ev.Corner), mul, true)
		case events.RivalPushed:
			corner := w.Corner(ev.Corner)
			if corner == nil || corner.Enforcer <= 0 {
				continue // nobody stood in the way, nobody was shot
			}
			mul := life.PersonalityMul(w.Rival.Personality)
			s.shoot(w, t, rng, corner.Enforcer, corner, mul, false)
			s.theirs(w, t, rng, corner, mul, false)
		}
	}

	// 5. Birthdays: a year older every year_days on the payroll, nerve
	// going past nerve_age, and at retire_age the farewell. Skill grows
	// every day to the cap, faster on generous pay.
	growth := life.SkillGrowth * s.cfg.PayFor(c.Pay).Wage
	kept := c.Members[:0]
	for _, m := range c.Members {
		if growth > 0 && m.Skill < life.SkillCap {
			m.Growth += growth
			for m.Growth >= 1 && m.Skill < life.SkillCap {
				m.Skill++
				m.Growth--
			}
		}
		since := t.Day - m.Hired
		if m.Age > 0 && since > 0 && since%life.YearDays == 0 {
			m.Age++
			if m.Age > life.NerveAge {
				m.Nerve = max(0, m.Nerve-life.NerveLoss)
			}
			if m.Age >= life.RetireAge {
				s.retire(w, t, rng, m, fx)
				continue
			}
		}
		kept = append(kept, m)
	}
	c.Members = kept

	// 6. Kin: a hire's cousin, partner or friend turns up in the pool at
	// the discount, one extra face beside the usual ones.
	for _, h := range c.HiredToday {
		m := c.Member(h.ID)
		if m == nil || rng.Float64() >= life.KinChance {
			continue
		}
		s.kinFace(w, t, rng, m, fx)
	}
}

// kinLoyalty moves the loyalty of every kin of m on the payroll by d.
func (s *Sim) kinLoyalty(w *game.World, m game.CrewMember, d float64) {
	for _, id := range m.Kin {
		if k := w.Crew.Member(id); k != nil {
			k.Loyalty = math.Max(0, math.Min(100, k.Loyalty+d))
		}
	}
}

// jail puts m in a cell for jail_days: off the corner, the house and
// the road, selling and guarding nothing until they are out.
func (s *Sim) jail(w *game.World, t *game.Tick, m *game.CrewMember, city string, corner *game.Corner, route string) {
	life := s.cfg.Life
	ev := events.CrewArrested{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, City: city, Days: life.JailDays, Bail: s.BailCost(*m), Route: route}
	if corner != nil {
		ev.Corner, ev.CornerName = corner.ID, corner.Name
	}
	w.Recall(m.ID)
	m.JailedUntil = t.Day + life.JailDays
	m.Bailed = false
	w.Stats.Arrests++
	t.Emit(ev)
}

// shoot rolls a death and then a wound for the member with id on a
// corner, at mul times the table's chances.
func (s *Sim) shoot(w *game.World, t *game.Tick, rng rand, id int, corner *game.Corner, mul float64, strike bool) {
	life := s.cfg.Life
	m := w.Crew.Member(id)
	if m == nil {
		return
	}
	ev := events.CrewShot{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Rival: w.Rival.Leader, Faction: w.Rival.Faction(), Strike: strike}
	if corner != nil {
		ev.Corner, ev.CornerName, ev.City = corner.ID, corner.Name, corner.City
	}
	switch {
	case rng.Float64() < life.KillChance*mul:
		ev.Dead = true
		s.fall(w, t, *m, corner)
	case rng.Float64() < life.WoundChance*mul:
		ev.Days = life.WoundDays
		w.Recall(m.ID)
		m.WoundedUntil = t.Day + life.WoundDays
		w.Stats.Wounded++
	default:
		return
	}
	t.Emit(ev)
}

// theirs rolls a death on the rival's side of a strike or a push: a
// body the count takes and the paper prints; the rival's headcount is
// the rivals sim's own business.
func (s *Sim) theirs(w *game.World, t *game.Tick, rng rand, corner *game.Corner, mul float64, strike bool) {
	if rng.Float64() >= s.cfg.Life.KillChance*mul {
		return
	}
	ev := events.CrewShot{Day: t.Day, Rival: w.Rival.Leader, Faction: w.Rival.Faction(), Dead: true, Theirs: true, Strike: strike}
	if corner != nil {
		ev.Corner, ev.CornerName, ev.City = corner.ID, corner.Name, corner.City
	}
	w.Stats.Bodies++
	t.Emit(ev)
}

// fall takes m off the payroll for good: onto Fallen, the count, and
// their kin's loyalty and nerve.
func (s *Sim) fall(w *game.World, t *game.Tick, m game.CrewMember, corner *game.Corner) {
	life := s.cfg.Life
	w.Recall(m.ID)
	f := game.Fallen{ID: m.ID, Name: m.Name, Role: m.Role, Age: m.Age, Day: t.Day}
	if corner != nil {
		f.Corner, f.CornerName = corner.ID, corner.Name
	}
	w.Crew.Fallen = append(w.Crew.Fallen, f)
	w.Stats.Bodies++
	w.Stats.Fallen++
	for i, o := range w.Crew.Members {
		if o.ID == m.ID {
			w.Crew.Members = append(w.Crew.Members[:i], w.Crew.Members[i+1:]...)
			break
		}
	}
	for _, id := range m.Kin {
		if k := w.Crew.Member(id); k != nil {
			k.Loyalty = math.Max(0, k.Loyalty-life.KinBodyLoyalty)
			k.Nerve = min(100, k.Nerve+life.KinBodyNerve)
		}
	}
}

// retire is the farewell at retire_age: a loyal one recommends a kin,
// who is in the pool tomorrow at the discount; a sour one, under the
// informant line, talks on the way out, and the heat sim files the
// page (the informant exception applied once).
func (s *Sim) retire(w *game.World, t *game.Tick, rng rand, m game.CrewMember, fx game.Effects) {
	life := s.cfg.Life
	ev := events.CrewRetired{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Age: m.Age}
	w.Recall(m.ID)
	if m.Runs() {
		w.DropStanding(m.City)
	}
	w.Stats.Retired++
	switch {
	case m.Loyalty >= life.RetireLoyal:
		if k := s.kinFace(w, t, rng, &m, fx); k != nil {
			ev.Kin = k.Name
		}
	case m.Loyalty < s.cfg.Informant.Loyalty:
		ev.Sour = true
	}
	t.Emit(ev)
}

// kinFace puts m's kin in the pool (#46): a face rolled the way a
// candidate is, off the life stream with a name from the crew list,
// at kin_discount off the fee, linked both ways. It is an extra face
// beside the pool's usual ones (refill counts it with the chemist's),
// so the faces the home stream draws are the faces it always drew.
func (s *Sim) kinFace(w *game.World, t *game.Tick, rng rand, m *game.CrewMember, fx game.Effects) *game.CrewMember {
	life := s.cfg.Life
	k := s.generate(w, rng, nil, rng, fx)
	k.Fee = int(math.Round(float64(k.Fee) * (1 - life.KinDiscount)))
	k.Kin = []int{m.ID}
	m.Kin = append(m.Kin, k.ID)
	w.Crew.Candidates = append(w.Crew.Candidates, k)
	t.Emit(events.KinLooking{Day: t.Day, Name: k.Name, Role: k.Role, Of: m.Name, Fee: k.Fee})
	return &w.Crew.Candidates[len(w.Crew.Candidates)-1]
}

// extra reports whether a candidate is one of the faces beside the
// pool's usual ones (#47's chemist, #46's driver and kin): refill fills
// the pool to size without them.
func extra(c game.CrewMember) bool {
	return c.Role == game.RoleChemist || c.Role == game.RoleDriver || len(c.Kin) > 0
}

// driverLooking reports whether the pool holds a driver.
func driverLooking(w *game.World) bool {
	for _, c := range w.Crew.Candidates {
		if c.Role == game.RoleDriver {
			return true
		}
	}
	return false
}

// driver rolls the driver looking for work (#46): the chemist's
// pattern, off the driver's stream with a name from the drivers' list.
func (s *Sim) driver(w *game.World, rng, life rand, fx game.Effects) game.CrewMember {
	used := map[string]bool{}
	for _, m := range w.Crew.Members {
		used[m.Name] = true
	}
	for _, m := range w.Crew.Candidates {
		used[m.Name] = true
	}
	var free []string
	for _, n := range s.drivers {
		if !used[n] {
			free = append(free, n)
		}
	}
	sort.Strings(free)
	name := "The Driver"
	if len(free) > 0 {
		name = free[rng.IntN(len(free))]
	}
	tun := s.cfg.Crew
	rc := s.cfg.Role[game.RoleDriver]
	skill := min(100, 15+rng.IntN(71)+fx.SkillBonus)
	m := game.CrewMember{
		ID:      w.Crew.NextID + 1,
		Name:    name,
		Role:    game.RoleDriver,
		Skill:   skill,
		Loyalty: float64(min(100, tun.StartLoyaltyMin+rng.IntN(max(1, tun.StartLoyaltyMax-tun.StartLoyaltyMin+1))+fx.StartLoyaltyBonus)),
		Greed:   5 + rng.IntN(91),
		Nerve:   5 + rng.IntN(91),
		Wage:    int(math.Round(rc.WageBase + rc.WagePerSkill*float64(skill))),
		Fee:     s.hireFee(w, skill, fx),
		Age:     s.age(life),
	}
	w.Crew.NextID = m.ID
	return m
}

// DriverCut is what a driver of a skill takes off a shipment's risk per
// day on the road: driver_cut x skill/100.
func (s *Sim) DriverCut(skill int) float64 {
	return math.Max(0, math.Min(1, s.cfg.Role[game.RoleDriver].DriverCut*float64(skill)/100))
}
