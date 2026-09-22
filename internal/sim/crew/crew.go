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

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Roles a candidate can be generated with, weighted by how often. An
// accountant only comes looking for work once there is a front to keep the
// books of.
var (
	roles      = []string{game.RoleRunner, game.RoleRunner, game.RoleEnforcer}
	rolesFront = []string{game.RoleRunner, game.RoleRunner, game.RoleEnforcer, game.RoleAccountant}
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
		offer(game.RoleAccountant, "Accountants", "a front owned")
	}
	if LieutenantsWanted(w) {
		offer(game.RoleLieutenant, "Lieutenants", "corners in two cities")
	}
	if s.ChemistsWanted(w) {
		offer(game.RoleChemist, "Chemists", w.ProductName(s.cfg.Role[game.RoleChemist].UnlockProduct)+" on the ladder")
	}
	if FixersWanted(w) {
		offer(game.RoleFixer, "Fixers", "an envelope paid")
	}
	if DriversWanted(w) {
		offer(game.RoleDriver, "Drivers", "a route run")
	}
}

// FixersWanted is whether fixers come looking for work (#42): once an
// envelope has gone to the chief or the DA, or a checkpoint or a customs
// agent has been paid. Word gets round that you pay. (The issue gated
// them on a front or a route; that would put a face in the pool of
// every run with a front, displacing a runner in runs that never bribe,
// so the gate is the first payment and the roll is a side stream's.)
func FixersWanted(w *game.World) bool { return w.Stats.Bribes > 0 || w.Stats.Checkpoints > 0 }

// Sim is the crew simulation.
type Sim struct {
	cfg      content.CrewConfig
	names    []string
	chemists []string // the chemist's names (#47), a pool of their own
	drivers  []string // the driver's names (#46), the same
	rep      content.ReputationFX
	tree     content.UpgradesConfig
	lab      *content.AssetConfig   // #48: the lab asset's row, what it does to a cook in its city; nil with none in the file
	fac      content.FactionsTuning // the table (#43): the discount a fragmented faction's muscle sign for
	intel    content.IntelTuning    // the spies (#45): their cadence, their accuracy and their odds of being found
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
	s := &Sim{cfg: cfg.Crew, names: cfg.Names.Crew, chemists: cfg.Names.Chemists, drivers: cfg.Names.Drivers, rep: cfg.Reputation.Effects, tree: cfg.Upgrades, fac: cfg.Rivals.Factions, intel: cfg.Intel.Intel}
	if lab := cfg.Assets.ByEffect(content.AssetLab); lab != nil {
		row := *lab
		s.lab = &row
	}
	return s
}

// Lab is the lab asset's row while it is owned, standing and in city
// (#48), or nil: what multiplies a cook there.
func (s *Sim) Lab(w *game.World, city string) *content.AssetConfig {
	if s.lab == nil || s.lab.City != city || !w.AssetLive(s.lab.ID) {
		return nil
	}
	return s.lab
}

// BatchIn is the most units the best chemist cooks an order in a city:
// Batch, times the lab's lab_mul where the lab stands (#48).
func (s *Sim) BatchIn(w *game.World, city string) int {
	b := s.Batch(w)
	if lab := s.Lab(w, city); lab != nil && b > 0 {
		b = int(math.Round(float64(b) * lab.LabMul))
	}
	return b
}

// QualityIn is what a cook in a city lands at: the best chemist's
// quality, or the lab's lab_quality where the lab stands and that is
// higher (#48); 0 with no chemist.
func (s *Sim) QualityIn(w *game.World, city string) float64 {
	q := s.ChemistQuality(w)
	if lab := s.Lab(w, city); lab != nil && q > 0 {
		q = math.Max(q, lab.LabQuality)
	}
	return q
}

// CookCostIn is what a unit's precursors cost in a city: the product's
// cook_cost as the caller prices it, at the lab's lab_cost_mul where the
// lab stands (#48).
func (s *Sim) CookCostIn(w *game.World, city string, cost int) int {
	if lab := s.Lab(w, city); lab != nil && cost > 0 {
		return max(1, int(math.Round(float64(cost)*lab.LabCostMul)))
	}
	return cost
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
		if m.Role == game.RoleEnforcer && m.Skill > best {
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
// day 0. It draws from rng, which the caller derives from the seed; the
// ages (#46) come off day 0's life stream, so the faces are the faces
// the home stream always drew.
func (s *Sim) Seed(w *game.World, rng rand) {
	w.Crew.Pay = events.PayFair
	life := (&game.Tick{Day: 0, Seed: w.Seed}).Sub(game.StreamLife)
	s.refill(w, rng, nil, nil, nil, life, game.FoldEffects(w, s.tree))
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

// night is one Step's working state (#275): what its phases share.
// fired is the firings the rest hold against you (an informant's is not
// one), enforcers the enforcers at work as the skim counted them (the
// drift's shield reads the same count), acted the lieutenants' reports
// the take opens and the night fills, short what the wages came up
// short and asked whether an investigation named nobody.
type night struct {
	w         *game.World
	t         *game.Tick
	tun       content.CrewTuning
	fx        game.Effects
	c         *game.CrewState
	fired     int
	enforcers int
	acted     map[int]*events.LieutenantActed
	short     int
	asked     bool
}

// Step pays wages, lets disloyal members skim, drifts loyalty, and handles
// quitting and the hiring pool. Skimming is checked on how people felt this
// morning, before today's drift, so a member never skims on a day they
// started above the threshold. The tree folds once at the top (#118)
// and every number below is the tuning times it. It runs as phases in a
// fixed order (#275): the day's hires and firings and the pool's
// rotation, crew life (life.go) and the spies (spy.go), the skim, the
// turning, the wages and the broke check (pay.go), the investigation,
// the drift and the quitting (loyalty.go), the lieutenants' night
// (lieutenant.go) and the refill (pool.go). The order is the dice's: a
// phase moved is a run that rolls differently.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	n := &night{w: w, t: t, tun: s.cfg.Crew, fx: game.FoldEffects(w, s.tree), c: &w.Crew}
	s.announce(w, t)
	s.land(w, t)
	s.roster(n)
	s.rotate(n)

	// Crew life (#46): the cells, last night's sweep, tonight's
	// shooting, the birthdays and the kin, all off the life stream.
	s.life(w, t, n.fx)

	// The spies (#45): tonight's plant, the reports due and who was
	// found, off the intel stream.
	s.spies(w, t)

	s.skim(n)
	s.turn(n)
	s.pay(n)
	if s.broke(n) {
		return
	}
	s.investigate(n)
	s.drift(n)
	s.quit(n)
	s.lieutenants(n)
	s.pool(n)
}

// roster reports the day's hires, firings and pay-offs, and counts the
// firings the rest hold against you.
func (s *Sim) roster(n *night) {
	t, c := n.t, n.c
	for _, m := range c.HiredToday {
		t.Emit(events.CrewHired{Day: t.Day, Name: m.Name, Role: m.Role, Fee: m.Fee})
	}
	for _, m := range c.FiredToday {
		if !m.Informant {
			n.fired++
		}
		t.Emit(events.CrewFired{Day: t.Day, Name: m.Name, Role: m.Role, Informant: m.Informant})
	}
	for _, p := range c.PaidOffToday {
		t.Emit(events.CrewPaidOff{Day: t.Day, Name: p.Name, Cost: p.Cost})
	}
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

// ChemistQuality is the quality the best chemist on the payroll makes
// (#47): quality_base + quality_per_skill x skill, what a cook lands
// at; 0 with none.
func (s *Sim) ChemistQuality(w *game.World) float64 {
	m := w.Crew.Chemist()
	if m == nil {
		return 0
	}
	return s.QualityOf(m.Skill)
}

// QualityOf is the quality a chemist of a skill makes: what a candidate
// would cook at.
func (s *Sim) QualityOf(skill int) float64 {
	rc := s.cfg.Role[game.RoleChemist]
	return math.Max(0, math.Min(100, rc.QualityBase+rc.QualityPerSkill*float64(skill)))
}

// BatchOf is the most units a chemist of a skill cooks an order.
func (s *Sim) BatchOf(skill int) int {
	return int(math.Round(s.cfg.Role[game.RoleChemist].BatchPerSkill * float64(skill)))
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
	return s.BatchOf(m.Skill)
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
