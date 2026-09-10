// Package crew simulates the people on the payroll: who is looking for
// work, what they cost, how loyal they feel and what they do about it.
// Runners raise how much product the operation can hold and, posted on a
// corner, work it; accountants help the fronts wash; disloyal crew skim
// the takings (or the wash), the nervous among them start talking to the
// police, and at the bottom they walk, or go over to the rival with the
// corner they ran. The player's investigation into who is talking
// resolves here too.
package crew

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Roles a candidate can be generated with, weighted by how often. An
// accountant only comes looking for work once there is a front to keep the
// books of.
var (
	roles      = []string{"runner", "runner", "enforcer"}
	rolesFront = []string{"runner", "runner", "enforcer", "accountant"}
)

func rolesFor(w *game.World) []string {
	if len(w.Fronts) > 0 {
		return rolesFront
	}
	return roles
}

// Sim is the crew simulation.
type Sim struct {
	cfg   content.CrewConfig
	names []string
	rep   content.ReputationFX
}

// New builds a crew sim from config and the name pool. Of the reputation
// effects it reads two: respect slows loyalty's decay, notoriety cuts
// what a candidate asks to sign.
func New(cfg content.CrewConfig, names content.NamesConfig, rep content.ReputationFX) *Sim {
	return &Sim{cfg: cfg, names: names.Crew, rep: rep}
}

// LoyaltyLoss is what the player's respect leaves of a day's loyalty
// loss: 1 for a nobody, less for a name the crew are proud to work for.
func (s *Sim) LoyaltyLoss(w *game.World) float64 {
	return content.Cut(w.Player.Reputation.Respect, s.rep.RespectLoyaltyCut)
}

// HireFee is what a candidate of the given skill asks to sign today: the
// tuning, less what the player's notoriety takes off. A candidate's fee
// is fixed when they are generated, so the pool catches up as it rotates.
func (s *Sim) HireFee(w *game.World, skill int) int {
	tun := s.cfg.Crew
	fee := tun.HireFeeBase + int(math.Round(tun.HireFeePerSkill*float64(skill)))
	return int(math.Round(float64(fee) * content.Cut(w.Player.Reputation.Notoriety, s.rep.NotorietyHireCut)))
}

func (s *Sim) Name() string { return "crew" }

// Tuning exposes the crew constants the UI needs to explain itself.
func (s *Sim) Tuning() content.CrewTuning { return s.cfg.Crew }

// MaxCrew is the roster cap.
func (s *Sim) MaxCrew() int { return s.cfg.Crew.MaxCrew }

// InvestigateCost is what asking questions costs.
func (s *Sim) InvestigateCost() int { return s.cfg.Informant.InvestigateCost }

// InvestigateOdds is the chance tonight's investigation names the
// informant, if there is one: a base, plus the best enforcer's skill, plus
// what every investigation that named nobody taught. The UI shows it, so
// it is what the dice use.
func (s *Sim) InvestigateOdds(w *game.World) float64 {
	tun := s.cfg.Informant
	best := 0
	for _, m := range w.Crew.Members {
		if m.Role == "enforcer" && m.Skill > best {
			best = m.Skill
		}
	}
	p := tun.InvestigateBase + tun.InvestigateSkill*float64(best)/100 + tun.InvestigateLearn*float64(w.Crew.Investigated)
	return math.Max(0, math.Min(1, p))
}

// PayoffCost is what buying m's loyalty costs.
func (s *Sim) PayoffCost(m game.CrewMember) int {
	return m.Wage * s.cfg.Informant.PayoffWages
}

// PayoffLoyalty is what a pay-off buys.
func (s *Sim) PayoffLoyalty() float64 { return s.cfg.Informant.PayoffLoyalty }

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
	fired := 0 // firings the rest hold against you: an informant's is not one
	for _, m := range c.FiredToday {
		if !m.Informant {
			fired++
		}
		t.Emit(events.CrewFired{Day: t.Day, Name: m.Name, Role: m.Role, Informant: m.Informant})
	}
	for _, p := range c.PaidOffToday {
		t.Emit(events.CrewPaidOff{Day: t.Day, Name: p.Name, Cost: p.Cost})
	}

	// 1. Skimming, on this morning's loyalty. Street crew skim the day's
	// takings; an accountant skims the wash, and the wash they can see is
	// the last one the fronts did (laundering steps after crew), so it
	// comes out of clean cash.
	revenue, wash := 0, 0
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok {
			revenue += ps.Revenue
		}
	}
	for _, f := range w.Fronts {
		wash += f.WashedToday
	}
	enforcers := c.Role("enforcer")
	deter := math.Pow(1-s.cfg.Role["enforcer"].Deterrence, float64(enforcers))
	share, washShare := 0.0, 0.0
	skimmers := 0
	for _, m := range c.Members {
		if m.Loyalty >= tun.SkimThreshold {
			continue
		}
		if t.RNG.Float64() < tun.SkimChance*deter {
			cut := tun.SkimShare * (0.5 + float64(m.Greed)/100)
			if m.Role == "accountant" {
				washShare += cut
			} else {
				share += cut
			}
			skimmers++
		}
	}
	if skimmers > 0 {
		amount := min(int(math.Round(float64(revenue)*math.Min(share, tun.SkimCap))), w.Player.DirtyCash)
		fromWash := min(int(math.Round(float64(wash)*math.Min(washShare, tun.SkimCap))), w.Player.CleanCash)
		if amount+fromWash > 0 {
			w.Player.DirtyCash -= amount
			w.Player.CleanCash -= fromWash
			w.Stats.Skimmed += amount + fromWash
			c.LastSkim = t.Day
			t.Emit(events.CrewSkimmed{Day: t.Day, Amount: amount + fromWash, Skimmers: skimmers, FromWash: fromWash})
		}
	}

	// Turning, on the same morning loyalty: the disloyal and nervous start
	// talking. Nothing is shown; the heat sim starts its clock on the event.
	inf := s.cfg.Informant
	for i := range c.Members {
		m := &c.Members[i]
		if m.Informant || m.Loyalty >= inf.Loyalty || m.Nerve >= inf.Nerve {
			continue
		}
		if t.RNG.Float64() < inf.Chance {
			m.Informant = true
			w.Stats.Informants++
			t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: m.ID, Name: m.Name})
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
	// If they empty the till with nothing left to sell, anywhere or on
	// the road, the run is over: there is no move that makes money from
	// nothing.
	if w.TotalStock() == 0 && float64(w.Player.DirtyCash) < cheapestUnit(w) {
		w.Over = &game.Ending{Day: t.Day, Cause: "broke", PeakCash: w.Stats.PeakCash}
		t.Emit(events.GameOver{Day: t.Day, Cause: "broke"})
		return
	}

	// 3. The investigation: it names an informant with the odds the UI
	// showed, or nobody, and being asked costs everyone a little loyalty
	// either way when it comes up empty.
	asked := false
	if o := w.Investigation; o != nil {
		ev := events.InvestigationRun{Day: t.Day, Cost: o.Cost}
		w.Stats.Investigations++
		if c.Informants() > 0 && t.RNG.Float64() < s.InvestigateOdds(w) {
			pick := t.RNG.IntN(c.Informants())
			for _, m := range c.Members {
				if !m.Informant {
					continue
				}
				if pick == 0 {
					ev.Found, ev.Name = true, m.Name
					c.Exposed = m.ID
				}
				pick--
			}
			c.Investigated = 0
		} else {
			asked = true
			c.Investigated++
		}
		t.Emit(ev)
	}

	// 4. Loyalty drift: pay, greed, danger, firings, unpaid wages, an
	// investigation that named nobody, and for the enforcers, the strike
	// they went on today: a toll from the rivals sim that the nervous feel
	// most and a win halves. A respected boss's crew feel every loss less.
	toll := 0.0
	for _, e := range t.Events() {
		if cs, ok := e.(events.CornerStruck); ok {
			if cs.Taken {
				toll += cs.Toll / 2
			} else {
				toll += cs.Toll
			}
		}
	}
	danger := false
	for _, level := range []string{"sting", "raid"} {
		if d, ok := w.Heat.LastResponse[level]; ok && t.Day-d <= tun.DangerDays {
			danger = true
		}
	}
	shield := math.Pow(1-s.cfg.Role["enforcer"].Protection, float64(enforcers))
	loss := s.LoyaltyLoss(w)
	base := s.cfg.PayFor(c.Pay).Loyalty
	base -= tun.FireLoyalty * float64(fired)
	if short > 0 {
		base -= tun.UnpaidLoyalty
	}
	if asked {
		base -= inf.InvestigateLoyalty
	}
	for i := range c.Members {
		m := &c.Members[i]
		d := base - tun.GreedDrift*float64(m.Greed)/100
		if danger {
			d -= tun.DangerLoyalty * float64(100-m.Nerve) / 100 * shield
		}
		if m.Role == "enforcer" {
			d -= toll * float64(100-m.Nerve) / 100
		}
		if d < 0 {
			d *= loss
		}
		m.Loyalty = math.Max(0, math.Min(100, m.Loyalty+d))
	}

	// 5. Quitting, or defecting: whoever walks leaves their corner
	// unworked, and while the rival holds ground in the city they go to
	// it instead, and walk it onto that corner (the rival sim acts on
	// the lead next step).
	kept := c.Members[:0]
	for _, m := range c.Members {
		if m.Loyalty > tun.QuitThreshold {
			kept = append(kept, m)
			continue
		}
		post := w.PostOf(m.ID)
		w.Recall(m.ID)
		if w.RivalHeld() == 0 {
			t.Emit(events.CrewQuit{Day: t.Day, Name: m.Name, Role: m.Role})
			continue
		}
		ev := events.CrewDefected{Day: t.Day, Name: m.Name, Role: m.Role, Rival: w.Rival.Leader}
		lead := game.Lead{Name: m.Name}
		if post != nil {
			ev.Corner, ev.CornerName = post.ID, post.Name
			lead.Corner = post.ID
		}
		w.Rival.Leads = append(w.Rival.Leads, lead)
		w.Stats.Defections++
		t.Emit(ev)
	}
	c.Members = kept

	// 6. The hiring pool rotates on a schedule and refills after hires.
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
	roles := rolesFor(w)
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
		Fee:     s.HireFee(w, skill),
	}
	if role == "runner" {
		m.Units = int(math.Round(tun.UnitsPerSkill * float64(skill)))
	}
	w.Crew.NextID = m.ID
	return m
}

// cheapestUnit is the lowest supplier price where the player is.
func cheapestUnit(w *game.World) float64 {
	price := math.Inf(1)
	if c := w.Here(); c != nil {
		for _, m := range c.Market {
			price = math.Min(price, m.SupplierPrice)
		}
	}
	if math.IsInf(price, 1) {
		return 0
	}
	return price
}
