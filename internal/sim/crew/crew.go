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

// announce reports a role that joins the hiring pool this morning
// (#148): the accountant the first morning rolesFor holds a front, the
// lieutenant the first morning LieutenantsWanted holds. Each is stamped
// in Offered and announced once with an Unlocked{Gate: "role"}; a save
// from before the field catches up the first morning. No dice, and the
// pool itself is untouched: the face comes when it next rotates.
func (s *Sim) announce(w *game.World, t *game.Tick) {
	offer := func(role, name, why string) {
		if w.Crew.Offered[role] {
			return
		}
		if w.Crew.Offered == nil {
			w.Crew.Offered = map[string]bool{}
		}
		w.Crew.Offered[role] = true
		t.Emit(events.Unlocked{Day: t.Day, Gate: "role", ID: role, Name: name, Why: why})
	}
	if len(w.Fronts) > 0 {
		offer("accountant", "Accountants", "a front owned")
	}
	if LieutenantsWanted(w) {
		offer(game.RoleLieutenant, "Lieutenants", "corners in two cities")
	}
	if s.ChemistsWanted(w) {
		offer(game.RoleChemist, "Chemists", w.ProductName(s.cfg.Role[game.RoleChemist].UnlockProduct)+" on the ladder")
	}
}

// Sim is the crew simulation.
type Sim struct {
	cfg      content.CrewConfig
	names    []string
	chemists []string // the chemist's names (#47), a pool of their own
	rep      content.ReputationFX
	tree     content.UpgradesConfig
}

// New builds a crew sim from the config, copying what it reads (#144):
// its own crew.toml, the crew name pool and the upgrade tree. Of the
// reputation effects it reads two: respect slows loyalty's decay,
// notoriety cuts what a candidate asks to sign. Of the tree it folds the
// Crew branch at the top of its step (#118): wage_mul on the bill,
// loyalty_loss_mul on a day's loss (the way respect scales it),
// danger_loyalty_mul on what a sting or raid costs, skim_chance_mul and
// informant_chance_mul on the rolls under the lines, crew_slots on
// MaxCrew, candidates_bonus and pool_days_cut on the pool, skill_bonus
// and start_loyalty_bonus on a generated candidate and hire_fee_mul on
// their fee, both fixed when they are generated.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Crew, names: cfg.Names.Crew, chemists: cfg.Names.Chemists, rep: cfg.Reputation.Effects, tree: cfg.Upgrades}
}

// LoyaltyLoss is what the player's respect and the tree leave of a day's
// loyalty loss: 1 for a nobody with no nodes, less for a name the crew
// are proud to work for, times loyalty_loss_mul.
func (s *Sim) LoyaltyLoss(w *game.World) float64 {
	return s.loyaltyLoss(w, game.FoldEffects(w, s.tree))
}

func (s *Sim) loyaltyLoss(w *game.World, fx game.Effects) float64 {
	return content.Cut(w.Player.Reputation.Respect, s.rep.RespectLoyaltyCut) * fx.LoyaltyLossMul
}

// HireFee is what a candidate of the given skill asks to sign today: the
// tuning, less what the player's notoriety takes off, times the tree's
// hire_fee_mul. A candidate's fee is fixed when they are generated, so
// the pool catches up as it rotates.
func (s *Sim) HireFee(w *game.World, skill int) int {
	return s.hireFee(w, skill, game.FoldEffects(w, s.tree))
}

func (s *Sim) hireFee(w *game.World, skill int, fx game.Effects) int {
	tun := s.cfg.Crew
	fee := tun.HireFeeBase + int(math.Round(tun.HireFeePerSkill*float64(skill)))
	return int(math.Round(float64(fee) * content.Cut(w.Player.Reputation.Notoriety, s.rep.NotorietyHireCut) * fx.HireFeeMul))
}

func (s *Sim) Name() string { return "crew" }

// Tuning exposes the crew constants the UI needs to explain itself.
func (s *Sim) Tuning() content.CrewTuning { return s.cfg.Crew }

// MaxCrew is the roster cap: the tuning, plus the tree's crew_slots,
// plus the people every lieutenant running a city brings with them.
func (s *Sim) MaxCrew(w *game.World) int {
	return s.cfg.Crew.MaxCrew + game.FoldEffects(w, s.tree).CrewSlots + w.Crew.Lieutenants()*s.cfg.Role[game.RoleLieutenant].Crew
}

// Candidates is how many people are looking for work at any time: the
// tuning plus the tree's candidates_bonus.
func (s *Sim) Candidates(w *game.World) int {
	return s.candidates(game.FoldEffects(w, s.tree))
}

func (s *Sim) candidates(fx game.Effects) int { return s.cfg.Crew.Candidates + fx.CandidatesBonus }

// PoolDays is how often the hiring pool rotates: the tuning less the
// tree's pool_days_cut, never under a day; 0 is a pool that never does.
func (s *Sim) PoolDays(w *game.World) int {
	return s.poolDays(game.FoldEffects(w, s.tree))
}

func (s *Sim) poolDays(fx game.Effects) int {
	if s.cfg.Crew.PoolDays <= 0 {
		return 0
	}
	return max(1, s.cfg.Crew.PoolDays-fx.PoolDaysCut)
}

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

// WageAt is what m costs per day at pay dial p, after the tree's
// wage_mul: the roster's column, and the bill is the sum.
func (s *Sim) WageAt(w *game.World, m game.CrewMember, p events.Pay) int {
	return s.wageAt(m, p, game.FoldEffects(w, s.tree))
}

func (s *Sim) wageAt(m game.CrewMember, p events.Pay, fx game.Effects) int {
	return int(math.Round(float64(m.Wage) * s.cfg.PayFor(p).Wage * fx.WageMul))
}

// Wages is the whole roster's daily bill at pay dial p.
func (s *Sim) Wages(w *game.World, p events.Pay) int {
	return s.wages(w, p, game.FoldEffects(w, s.tree))
}

func (s *Sim) wages(w *game.World, p events.Pay, fx game.Effects) int {
	n := 0
	for _, m := range w.Crew.Members {
		n += s.wageAt(m, p, fx)
	}
	return n
}

// Seed fills the hiring pool for a fresh world so the player can hire on
// day 0. It draws from rng, which the caller derives from the seed.
func (s *Sim) Seed(w *game.World, rng rand) {
	w.Crew.Pay = events.PayFair
	s.refill(w, rng, nil, game.FoldEffects(w, s.tree))
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
// started above the threshold. The tree folds once at the top (#118)
// and every number below is the tuning times it.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Crew
	fx := game.FoldEffects(w, s.tree)
	c := &w.Crew
	s.announce(w, t)
	s.land(w, t)

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
		if t.RNG.Float64() < tun.SkimChance*fx.SkimChanceMul*deter {
			cut := tun.SkimShare * (0.5 + float64(m.Greed)/100)
			if m.Role == "accountant" {
				washShare += cut
			} else {
				share += cut
			}
			skimmers++
		}
	}
	// The lieutenants' cut of their cities' takings, and what a greedy
	// one skims on top, at any loyalty: nobody deters the boss of a city.
	acted := map[int]*events.LieutenantActed{}
	extra := s.take(w, t, acted)
	if extra > 0 {
		skimmers++
	}
	if skimmers > 0 {
		amount := min(int(math.Round(float64(revenue)*math.Min(share, tun.SkimCap))), w.Player.DirtyCash-extra) + extra
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
	// A lieutenant turns under a higher line and without dice: they know
	// where everything is, and the DA knows it.
	inf := s.cfg.Informant
	for i := range c.Members {
		m := &c.Members[i]
		if m.Informant {
			continue
		}
		if m.Lieutenant() && m.Loyalty < s.cfg.Lieutenant.Flip {
			m.Informant = true
			w.Stats.Informants++
			t.Emit(events.LieutenantFlipped{Day: t.Day, ID: m.ID, Name: m.Name, City: m.City})
			continue
		}
		if m.Loyalty >= inf.Loyalty || m.Nerve >= inf.Nerve {
			continue
		}
		if t.RNG.Float64() < inf.Chance*fx.InformantChanceMul {
			m.Informant = true
			w.Stats.Informants++
			t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: m.ID, Name: m.Name})
		}
	}

	// 2. Wages. Coming up short is remembered.
	short := 0
	if len(c.Members) > 0 {
		wages := s.wages(w, c.Pay, fx)
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
	if o := w.Today.Investigation; o != nil {
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
	hurt := 0
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			if ev.Taken {
				toll += ev.Toll / 2
			} else {
				toll += ev.Toll
			}
		case events.RivalBoosted:
			// A boost (#70) is the enforcers going in too: its toll by
			// nerve as a strike's, and a failure against real muscle
			// hurts the one with the least nerve.
			toll += ev.Toll
			hurt += ev.Hurt
		}
	}
	if hurt > 0 {
		var worst *game.CrewMember
		for i := range c.Members {
			m := &c.Members[i]
			if m.Role == "enforcer" && (worst == nil || m.Nerve < worst.Nerve) {
				worst = m
			}
		}
		if worst != nil {
			worst.Skill = max(1, worst.Skill-hurt)
		}
	}
	danger := false
	for _, level := range []string{content.Sting, content.Raid} {
		if d, ok := w.Heat.LastResponse[level]; ok && t.Day-d <= tun.DangerDays {
			danger = true
		}
	}
	shield := math.Pow(1-s.cfg.Role["enforcer"].Protection, float64(enforcers))
	loss := s.loyaltyLoss(w, fx)
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
			d -= tun.DangerLoyalty * fx.DangerLoyaltyMul * float64(100-m.Nerve) / 100 * shield
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
	// the lead next step, off c.Leads: last night's are consumed by now,
	// the rival steps first, so tonight's start the queue afresh; #144)
	// if it is one the rival fights over: the rival lives at home, so a
	// corner in another city is just a corner left. A lieutenant running
	// a city walks with it.
	c.Leads = nil
	kept := c.Members[:0]
	for _, m := range c.Members {
		if m.Loyalty > tun.QuitThreshold {
			kept = append(kept, m)
			continue
		}
		if m.Runs() {
			delete(acted, m.ID)
			s.walk(w, t, m)
			continue
		}
		post := w.PostOf(m.ID)
		w.Recall(m.ID)
		if w.RivalHeld() == 0 {
			t.Emit(events.CrewQuit{Day: t.Day, Name: m.Name, Role: m.Role})
			continue
		}
		ev := events.CrewDefected{Day: t.Day, Name: m.Name, Role: m.Role, Rival: w.Rival.Leader, Faction: w.Rival.Faction()}
		lead := game.Lead{Name: m.Name}
		if post != nil && post.City == w.Home().ID {
			ev.Corner, ev.CornerName = post.ID, post.Name
			lead.Corner = post.ID
		}
		c.Leads = append(c.Leads, lead)
		w.Stats.Defections++
		t.Emit(ev)
	}
	c.Members = kept

	// 6. The lieutenants' night: each runs their city with whoever is
	// left, and reports in the morning.
	for _, cid := range w.CityOrder {
		lt := c.Lieutenant(cid)
		if lt == nil {
			continue
		}
		ev := acted[lt.ID]
		if ev == nil { // assigned today, after the takings were counted
			ev = &events.LieutenantActed{Day: t.Day, ID: lt.ID, Name: lt.Name, City: cid, CityName: w.CityName(cid), Dial: s.Dial(*lt)}
		}
		s.delegate(w, t, lt, ev)
		t.Emit(*ev)
	}

	// 7. The hiring pool rotates on a schedule and refills after hires.
	if days := s.poolDays(fx); days > 0 && t.Day-c.PoolDay >= days {
		c.Candidates = nil
		c.PoolDay = t.Day
	}
	s.refill(w, t.RNG, t.Sub("chemist"), fx)
}

// refill tops the candidate pool up to size with fresh faces, and, once
// meth is on the ladder, adds the one chemist looking for work beside
// them (#47), drawn off the chemist's own stream (nil at seed: the
// ladder has no meth on day 0) with a name from their own list, so the
// faces the home stream draws are the faces it always drew.
func (s *Sim) refill(w *game.World, rng, chem rand, fx game.Effects) {
	faces := 0
	for _, c := range w.Crew.Candidates {
		if c.Role != game.RoleChemist {
			faces++
		}
	}
	for ; faces < s.candidates(fx); faces++ {
		w.Crew.Candidates = append(w.Crew.Candidates, s.generate(w, rng, fx))
	}
	if chem != nil && s.ChemistsWanted(w) && !s.chemistLooking(w) {
		w.Crew.Candidates = append(w.Crew.Candidates, s.chemist(w, chem, fx))
	}
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

// ChemistsWanted reports whether a chemist comes looking for work: the
// role's unlock_product (meth) is on the ladder.
func (s *Sim) ChemistsWanted(w *game.World) bool {
	id := s.cfg.Role[game.RoleChemist].UnlockProduct
	if id == "" || w.Home() == nil {
		return false
	}
	return w.Home().Market[id] != nil
}

// chemist rolls the chemist looking for work (#47): the generate roll
// off the chemist's stream, with a name from the chemists' list.
func (s *Sim) chemist(w *game.World, rng rand, fx game.Effects) game.CrewMember {
	used := map[string]bool{}
	for _, m := range w.Crew.Members {
		used[m.Name] = true
	}
	for _, m := range w.Crew.Candidates {
		used[m.Name] = true
	}
	var free []string
	for _, n := range s.chemists {
		if !used[n] {
			free = append(free, n)
		}
	}
	sort.Strings(free)
	name := "The Chemist"
	if len(free) > 0 {
		name = free[rng.IntN(len(free))]
	}
	tun := s.cfg.Crew
	rc := s.cfg.Role[game.RoleChemist]
	skill := min(100, 15+rng.IntN(71)+fx.SkillBonus)
	m := game.CrewMember{
		ID:      w.Crew.NextID + 1,
		Name:    name,
		Role:    game.RoleChemist,
		Skill:   skill,
		Loyalty: float64(min(100, tun.StartLoyaltyMin+rng.IntN(max(1, tun.StartLoyaltyMax-tun.StartLoyaltyMin+1))+fx.StartLoyaltyBonus)),
		Greed:   5 + rng.IntN(91),
		Nerve:   5 + rng.IntN(91),
		Wage:    int(math.Round(rc.WageBase + rc.WagePerSkill*float64(skill))),
		Fee:     s.hireFee(w, skill, fx),
	}
	w.Crew.NextID = m.ID
	return m
}

// ChemistQuality is the quality the best chemist on the payroll makes
// (#47): quality_base + quality_per_skill x skill, what a cook lands
// at; 0 with none.
func (s *Sim) ChemistQuality(w *game.World) float64 {
	m := w.Crew.Chemist()
	if m == nil {
		return 0
	}
	rc := s.cfg.Role[game.RoleChemist]
	return math.Max(0, math.Min(100, rc.QualityBase+rc.QualityPerSkill*float64(m.Skill)))
}

// CutBonus is the quality points a cut keeps with the best chemist's
// hand on it: cut_bonus x skill; 0 with none.
func (s *Sim) CutBonus(w *game.World) float64 {
	m := w.Crew.Chemist()
	if m == nil {
		return 0
	}
	return s.cfg.Role[game.RoleChemist].CutBonus * float64(m.Skill)
}

// Batch is the most units the best chemist cooks an order:
// batch_per_skill x skill; 0 with none.
func (s *Sim) Batch(w *game.World) int {
	m := w.Crew.Chemist()
	if m == nil {
		return 0
	}
	return int(math.Round(s.cfg.Role[game.RoleChemist].BatchPerSkill * float64(m.Skill)))
}

// CookDays is how long a cook takes.
func (s *Sim) CookDays() int { return max(1, s.cfg.Role[game.RoleChemist].CookDays) }

// ChemistName is the best chemist's name, "" with none: what a cook
// order is signed with.
func (s *Sim) ChemistName(w *game.World) string {
	if m := w.Crew.Chemist(); m != nil {
		return m.Name
	}
	return ""
}

// land puts every cook that is ready into its city's stash at the
// quality it was ordered at (#47) and reports it; the chemist need not
// still be on the payroll (the precursors were bought), the lot is.
func (s *Sim) land(w *game.World, t *game.Tick) {
	if len(w.Crew.Cooks) == 0 {
		return
	}
	kept := w.Crew.Cooks[:0]
	for _, k := range w.Crew.Cooks {
		if k.Ordered == t.Day-1 {
			t.Emit(events.CookOrdered{Day: t.Day, City: k.City, Product: k.Product, Units: k.Units, Quality: k.Quality, Cost: k.Cost, Days: k.Ready - k.Ordered, Chemist: k.Chemist})
		}
		if k.Ready > t.Day {
			kept = append(kept, k)
			continue
		}
		w.AddStock(k.City, k.Product, k.Units, k.Quality)
		t.Emit(events.Cooked{Day: t.Day, City: k.City, Product: k.Product, Units: k.Units, Quality: k.Quality, Cost: k.Cost, Chemist: k.Chemist})
	}
	w.Crew.Cooks = kept
	if len(w.Crew.Cooks) == 0 {
		w.Crew.Cooks = nil
	}
}

// generate rolls a new candidate whose name is not already in use. The
// tree's skill_bonus and start_loyalty_bonus land on the roll, never
// over 100, and the fee is priced on the skill they arrive with; none
// of it adds a draw, so a run owning nothing rolls the pool it always
// did.
func (s *Sim) generate(w *game.World, rng rand, fx game.Effects) game.CrewMember {
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
	// Lieutenants are rare, and only come looking once there is a second
	// city to hand over; the roll is only made then, so a run in one city
	// draws the same pool it always did.
	personality := ""
	if lt := s.cfg.Lieutenant; LieutenantsWanted(w) && rng.Float64() < lt.Chance {
		role = game.RoleLieutenant
		personality = content.LieutenantPersonalities[rng.IntN(len(content.LieutenantPersonalities))]
	}
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
		Fee:     s.hireFee(w, skill, fx),

		Personality: personality, // "" for anyone but a lieutenant
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
