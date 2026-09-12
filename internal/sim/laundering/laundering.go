// Package laundering simulates the fronts: businesses that turn dirty cash
// into clean cash at a capped rate, cost upkeep to keep open, and get
// audited when pushed. The launder dial trades throughput against audits
// across every front at once. The wash always leaves a float of dirty
// cash in the till, so owning more fronts than you feed never starves the
// street. Heat learns of an audit the morning after, from the front's
// record: this sim steps after heat. A front grows by levels (#192):
// clean cash invested in it earns clean income of its own every day,
// dirty or no dirty, scales what it washes and costs, and is what its
// books have to explain when the auditors come.
package laundering

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the laundering simulation.
type Sim struct {
	cfg  content.LaunderingConfig
	inf  content.InformantTuning
	tree content.UpgradesConfig
}

// New builds a laundering sim from the config, copying what it reads
// (#144): its own laundering.toml, the crew's [informant] line for the
// loyalty under which an audited front's accountant talks, and the
// upgrade tree, whose Laundering branch it folds at the top of its
// step (#118): wash_mul on every front's throughput, audit_risk_mul on
// its audit risk, audit_seize_mul on what an audit takes, upkeep_mul on
// what a front costs, audit_freeze_cut on how long an audit shuts it and
// float_mul on the float, through World.Float, the one number the wash,
// the road and a supply contract read.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Laundering, inf: cfg.Crew.Informant, tree: cfg.Upgrades}
}

func (s *Sim) Name() string { return "laundering" }

// Tuning exposes the laundering constants the UI needs to explain itself.
func (s *Sim) Tuning() content.LaunderingTuning { return s.cfg.Laundering }

// Growth exposes the [growth] table (#192): what a level adds to the
// audit risk, the wash a front's income has to explain, and the level
// that makes the paper.
func (s *Sim) Growth() content.GrowthConfig { return s.cfg.Growth }

// Offshore exposes the [offshore] table (#195): the lot, the fee and
// what retiring takes.
func (s *Sim) Offshore() content.OffshoreConfig { return s.cfg.Offshore }

// Lots is how many lots over the unnoticed line a day's move offshore
// of amount is: the pages the heat sim files a lot the morning after.
// Nothing at or under the lot, and nothing at all with no lot in the
// file.
func (s *Sim) Lots(amount int) int {
	lot := s.cfg.Offshore.Lot
	if lot <= 0 || amount <= lot {
		return 0
	}
	return (amount - lot + lot - 1) / lot
}

// Fee is what the account keeps of a move of amount.
func (s *Sim) Fee(amount int) int {
	return int(math.Round(float64(amount) * s.cfg.Offshore.Fee))
}

// Retire is World.Retire at the file's terms.
func (s *Sim) Retire(w *game.World) error {
	return w.Retire(s.cfg.Offshore.RetireCash, s.cfg.Offshore.RetireDays)
}

// CanRetire is World.CanRetire at the file's terms.
func (s *Sim) CanRetire(w *game.World) bool {
	return w.CanRetire(s.cfg.Offshore.RetireCash, s.cfg.Offshore.RetireDays)
}

// Dial returns the tuning for a launder dial position.
func (s *Sim) Dial(d events.Launder) content.LaunderConfig { return s.cfg.DialFor(d) }

// Seed sets a fresh world's dial to normal.
func (s *Sim) Seed(w *game.World) { w.Laundering.Dial = events.LaunderNormal }

// Migrate brings a save from before fronts existed up to date: nothing
// owned, the dial at normal.
func (s *Sim) Migrate(w *game.World) {
	if len(w.Fronts) == 0 {
		s.Seed(w)
	}
}

// announce reports a front whose offer opens this morning (#148): an
// offer is stamped in Offered the tick its line is crossed and an
// Unlocked{Gate: "front"} goes out, once. The line is read against the
// peak the clock is about to stamp, max(Stats.PeakCash, Cash()), and
// announce runs last in the step, after the wash and the upkeep, because
// nothing after the laundering sim moves cash: so the report line, the
// headline and the ledger's `open to you` are the same morning, and a
// front bought that morning was announced (TestFrontOpensTheMorningThe
// LedgerSays). A front already owned when its line is first read is
// stamped silently (a save from before the field catches up the first
// morning it is stepped), as is one with no line at all. No dice.
func (s *Sim) announce(w *game.World, t *game.Tick) {
	peak := max(w.Stats.PeakCash, w.Cash())
	for _, o := range s.Offers() {
		if w.Laundering.Offered[o.ID] || o.UnlockCash > peak {
			continue
		}
		if w.Laundering.Offered == nil {
			w.Laundering.Offered = map[string]bool{}
		}
		w.Laundering.Offered[o.ID] = true
		if o.UnlockCash > 0 && w.Front(o.ID) == nil {
			t.Emit(events.Unlocked{Day: t.Day, Gate: "front", ID: o.ID, Name: o.Name, Why: "peak cash " + format.Cash(o.UnlockCash), Cost: o.Cost})
		}
	}
}

// Offers lists every front the config knows, cheapest first, priced for
// BuyFront. Locked ones are included so the UI can show what is coming.
func (s *Sim) Offers() []game.FrontOffer {
	out := make([]game.FrontOffer, 0, len(s.cfg.Fronts))
	for _, f := range s.cfg.Fronts {
		out = append(out, offer(f))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Cost < out[j].Cost })
	return out
}

// Offer returns the priced offer for a front id.
func (s *Sim) Offer(id string) (game.FrontOffer, bool) {
	f := s.cfg.Front(id)
	if f == nil {
		return game.FrontOffer{}, false
	}
	return offer(*f), true
}

func offer(f content.FrontConfig) game.FrontOffer {
	return game.FrontOffer{
		ID: f.ID, Name: f.Name, Cost: f.Cost, Throughput: f.Throughput,
		Upkeep: f.Upkeep, AuditRisk: f.AuditRisk, UnlockCash: f.UnlockCash,
	}
}

// Buy buys the front with id for the player: BuyFront with the config's
// price, or ErrNoFront for an id the config does not know.
func (s *Sim) Buy(w *game.World, id string) (game.Front, error) {
	o, ok := s.Offer(id)
	if !ok {
		return game.Front{}, game.ErrNoFront
	}
	return w.BuyFront(o)
}

// accountants is what the accountants on the payroll add to every front's
// throughput and what they multiply its audit risk by. Each one counts by
// skill: a skill-100 accountant adds the full bonus and takes the full cut
// of whatever risk the ones before them left.
func (s *Sim) accountants(w *game.World) (throughput float64, risk float64) {
	tun := s.cfg.Laundering
	risk = 1
	for _, m := range w.Crew.Members {
		if m.Role != "accountant" {
			continue
		}
		skill := float64(m.Skill) / 100
		throughput += tun.AccountantThroughput * skill
		risk *= 1 - tun.AccountantRiskCut*skill
	}
	return throughput, math.Max(0, risk)
}

// Throughput is how much dirty cash a front can wash today at the current
// dial, with the accountants' help and the tree's wash_mul, whether or
// not it is open.
func (s *Sim) Throughput(w *game.World, f game.Front) int {
	return s.throughput(w, f, game.FoldEffects(w, s.tree))
}

func (s *Sim) throughput(w *game.World, f game.Front, fx game.Effects) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	acct, _ := s.accountants(w)
	return int(math.Round((float64(fc.Throughput)*levelMul(*fc, f.Level) + acct) * s.Dial(w.Laundering.Dial).Mul * fx.WashMul))
}

// levelMul is what a front's levels multiply its throughput and upkeep
// by: level_mul to the level, exactly one at level zero.
func levelMul(fc content.FrontConfig, level int) float64 {
	if level <= 0 {
		return 1
	}
	return math.Pow(fc.LevelMul, float64(level))
}

// Income is what a front earns a day on its own in clean cash at its
// level (#192): the first level earns the file's income, each after it
// level_mul times the one before, and the levels add up. Nothing at
// level zero, and nothing while the place is shut.
func (s *Sim) Income(f game.Front) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	return income(*fc, f.Level)
}

func income(fc content.FrontConfig, level int) int {
	n := 0
	for k := 0; k < level; k++ {
		n += int(math.Round(float64(fc.Income) * math.Pow(fc.LevelMul, float64(k))))
	}
	return n
}

// LevelCost is what the next n levels of a front cost between them in
// clean cash: the first level the file's level_cost, each after it
// level_mul times the one before.
func (s *Sim) LevelCost(f game.Front, n int) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	return levelCost(*fc, f.Level, n)
}

func levelCost(fc content.FrontConfig, from, n int) int {
	total := 0
	for k := from; k < from+n; k++ {
		total += int(math.Round(float64(fc.LevelCost) * math.Pow(fc.LevelMul, float64(k))))
	}
	return total
}

// MaxLevel is how far a front can be levelled; zero for one with no
// levels to buy.
func (s *Sim) MaxLevel(f game.Front) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	return fc.MaxLevel
}

// Levels prices a front's next n levels for World.Invest.
func (s *Sim) Levels(f game.Front, n int) game.LevelOffer {
	return game.LevelOffer{Front: f.ID, Levels: n, Cost: s.LevelCost(f, n), Max: s.MaxLevel(f)}
}

// Invest buys the next n levels of the front with id: World.Invest at
// the config's price, or ErrNoFront for a front not owned.
func (s *Sim) Invest(w *game.World, id string, n int) error {
	f := w.Front(id)
	if f == nil {
		return game.ErrNoFront
	}
	return w.Invest(s.Levels(*f, n))
}

// LegitIncome is what the fronts earn a day on their own, net of their
// upkeep (#192): the number the businessman ending reads against the
// street (#49) and the ledger prints beside the wash. Every front
// counts, open or shut, an unlevelled one at its upkeep alone, so a
// row of fronts that only wash reads as the loss on paper it is.
func (s *Sim) LegitIncome(w *game.World) int {
	fx := game.FoldEffects(w, s.tree)
	n := 0
	for _, f := range w.Fronts {
		if fc := s.cfg.Front(f.ID); fc != nil {
			n += income(*fc, f.Level) - upkeep(*fc, f.Level, fx)
		}
	}
	return n
}

// AuditRisk is the chance a front is audited today at the current dial,
// after the accountants' cut, the tree's audit_risk_mul and the levels
// (#192): audit_level per level, and the wash its income does not
// explain, read as today's throughput since the day's wash is not in yet.
func (s *Sim) AuditRisk(w *game.World, f game.Front) float64 {
	fx := game.FoldEffects(w, s.tree)
	return s.auditRisk(w, f, fx, s.throughput(w, f, fx))
}

func (s *Sim) auditRisk(w *game.World, f game.Front, fx game.Effects, wash int) float64 {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	_, cut := s.accountants(w)
	return math.Max(0, math.Min(1, fc.AuditRisk*s.Dial(w.Laundering.Dial).Risk*cut*fx.AuditRiskMul*s.growthRisk(*fc, f.Level, wash)))
}

// growthRisk is what a front's levels multiply its audit risk by (#192):
// one plus audit_level a level, times one plus the excess of the wash
// over legit_ratio times the front's own income. A front with no level
// is the front before it had any and reads the file exactly; from the
// first level the books have to explain the wash, so a big washer with
// a token level is the one the audit finds and a front whose income
// covers its wash is not.
func (s *Sim) growthRisk(fc content.FrontConfig, level, wash int) float64 {
	if level <= 0 {
		return 1
	}
	g := s.cfg.Growth
	mul := 1 + g.AuditLevel*float64(level)
	if inc := income(fc, level); g.LegitRatio > 0 && inc > 0 {
		mul *= 1 + math.Max(0, float64(wash)/(g.LegitRatio*float64(inc))-1)
	}
	return mul
}

// FrontUpkeep is what a front costs in clean cash a day, after the
// tree's upkeep_mul and its levels.
func (s *Sim) FrontUpkeep(w *game.World, f game.Front) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	return upkeep(*fc, f.Level, game.FoldEffects(w, s.tree))
}

func upkeep(fc content.FrontConfig, level int, fx game.Effects) int {
	return int(math.Round(float64(fc.Upkeep) * levelMul(fc, level) * fx.UpkeepMul))
}

// Float is the dirty cash the wash never takes the till below:
// laundering.toml's float folded by the tree (World.Float), the number
// the road and a supply contract keep to as well.
func (s *Sim) Float(w *game.World) int { return w.Float(s.tree, s.cfg.Laundering.Float) }

// Washable is the dirty cash the fronts may take today: what is over the
// float.
func (s *Sim) Washable(w *game.World) int {
	return max(0, w.Player.DirtyCash-s.Float(w))
}

// AuditFreezeDays is how long an audit shuts a front, after the tree's
// audit_freeze_cut, never under a day.
func (s *Sim) AuditFreezeDays(w *game.World) int {
	return auditFreezeDays(s.cfg.Laundering, game.FoldEffects(w, s.tree))
}

func auditFreezeDays(tun content.LaunderingTuning, fx game.Effects) int {
	return max(1, tun.AuditFreezeDays-fx.AuditFreezeCut)
}

// Capacity is the most the open fronts can wash today between them.
func (s *Sim) Capacity(w *game.World) int {
	fx := game.FoldEffects(w, s.tree)
	n := 0
	for _, f := range w.Fronts {
		if !f.Frozen(w.Day + 1) {
			n += s.throughput(w, f, fx)
		}
	}
	return n
}

// Upkeep is what the open fronts cost in clean cash today between them.
func (s *Sim) Upkeep(w *game.World) int {
	fx := game.FoldEffects(w, s.tree)
	n := 0
	for _, f := range w.Fronts {
		if fc := s.cfg.Front(f.ID); fc != nil && !f.Frozen(w.Day+1) {
			n += upkeep(*fc, f.Level, fx)
		}
	}
	return n
}

// flip turns the least loyal accountant under the informant loyalty line
// who is not already talking, if there is one, and reports it the way the
// crew sim does: bookkeeping, never a headline.
func (s *Sim) flip(w *game.World, t *game.Tick) {
	var pick *game.CrewMember
	for i := range w.Crew.Members {
		m := &w.Crew.Members[i]
		if m.Role != "accountant" || m.Informant || m.Loyalty >= s.inf.Loyalty {
			continue
		}
		if pick == nil || m.Loyalty < pick.Loyalty {
			pick = m
		}
	}
	if pick == nil {
		return
	}
	pick.Informant = true
	w.Stats.Informants++
	t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: pick.ID, Name: pick.Name})
}

// AnyAuditRisk is the chance at least one open front is audited today.
func (s *Sim) AnyAuditRisk(w *game.World) float64 {
	fx := game.FoldEffects(w, s.tree)
	clear := 1.0
	for _, f := range w.Fronts {
		if !f.Frozen(w.Day + 1) {
			clear *= 1 - s.auditRisk(w, f, fx, s.throughput(w, f, fx))
		}
	}
	return 1 - clear
}

// Step reports yesterday's purchases and investments, then for every
// open front washes what it can, pays its upkeep, pays out what its
// levels earn and rolls for an audit. A front whose upkeep cannot be
// paid shuts and earns nothing; an audited one shuts longer and loses
// part of what it washed today. Heat reads the audit off the front tomorrow. The tree folds
// once at the top (#118) and every number below is the tuning times it,
// the same number the ledger shows.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Laundering
	fx := game.FoldEffects(w, s.tree)
	dial := w.Laundering.Dial
	total, paid, washing, earned := 0, 0, 0, 0
	for i := range w.Fronts {
		f := &w.Fronts[i]
		f.WashedToday = 0
		fc := s.cfg.Front(f.ID)
		if fc == nil {
			continue
		}
		if f.Bought == t.Day-1 {
			t.Emit(events.FrontBought{Day: t.Day, Front: f.ID, Name: f.Name, Cost: fc.Cost})
		}
		s.reportGrowth(w, t, f, *fc)
		if f.Frozen(t.Day) {
			continue
		}

		// 1. The wash: dirty in, clean out, up to today's throughput and
		// never below the float.
		if amt := min(s.throughput(w, *f, fx), s.Washable(w)); amt > 0 {
			w.Player.DirtyCash -= amt
			w.Player.CleanCash += amt
			f.WashedToday = amt
			f.Washed += amt
			w.Stats.Laundered += amt
			total += amt
			washing++
		}

		// 2. Upkeep, in clean cash, out of the pile as it stands: the
		// wash just in and whatever was kept back. Unpaid, the place
		// shuts, and earns nothing while it is shut.
		due := upkeep(*fc, f.Level, fx)
		if due > w.Player.CleanCash {
			f.FrozenUntil = t.Day + tun.UpkeepFreezeDays
			t.Emit(events.FrontFrozen{Day: t.Day, Front: f.ID, Name: f.Name, Upkeep: due, Days: tun.UpkeepFreezeDays})
			continue
		}
		w.Player.CleanCash -= due
		paid += due

		// 2b. The levels' income (#192): clean, earned on its own, in
		// after the upkeep. A levelled front pays its own way out of
		// yesterday's income, if it was kept: spend every clean dollar
		// on levels and one thin morning shuts the places that were
		// paying you. That is what the reserve is for.
		if inc := income(*fc, f.Level); inc > 0 {
			w.Player.CleanCash += inc
			w.Stats.Earned += inc
			earned += inc
		}

		// 3. The audit. It freezes the front and takes a slice of what
		// went through the books today. The roll is the one roll there
		// is: a level moves the odds (#192), never the dice.
		if t.RNG.Float64() >= s.auditRisk(w, *f, fx, f.WashedToday) {
			continue
		}
		seized := min(int(math.Round(float64(f.WashedToday)*tun.AuditSeize*fx.AuditSeizeMul)), w.Player.CleanCash)
		w.Player.CleanCash -= seized
		w.Stats.Seized += seized
		freeze := auditFreezeDays(tun, fx)
		f.FrozenUntil = t.Day + freeze
		f.Audited = t.Day
		f.AuditDial = dial
		t.Emit(events.FrontAudited{Day: t.Day, Front: f.ID, Name: f.Name, Dial: dial, Seized: seized, Days: freeze})

		// 4. The auditors question the books' keeper. The least loyal
		// accountant under the informant line (#13) is turned, no dice:
		// the heat sim starts its clock the day nobody was talking ends.
		s.flip(w, t)
	}
	if total > 0 || paid > 0 || earned > 0 {
		t.Emit(events.CashLaundered{Day: t.Day, Amount: total, Upkeep: paid, Fronts: washing, Earned: earned})
	}
	s.reserve(w, t)
	s.quiet(w, t)
	s.announce(w, t)
}

// reserve moves what the player sent offshore today (#195) into the
// account, less the fee, and records the move (Structured) for the
// heat sim, which files the lots over the line the morning after:
// laundering steps after heat, so the record is how tonight's move
// reaches tomorrow's file. Nothing moved records nothing, so a run
// that never reserves is the run before. No dice.
func (s *Sim) reserve(w *game.World, t *game.Tick) {
	amt := w.Today.Reserved
	if amt <= 0 {
		return
	}
	fee := s.Fee(amt)
	w.Offshore += amt - fee
	w.Stats.Reserved += amt - fee
	w.Stats.Fees += fee
	lots := s.Lots(amt)
	w.Laundering.Structured = game.Structuring{Day: t.Day, Amount: amt, Lots: lots}
	t.Emit(events.Reserved{Day: t.Day, Amount: amt - fee, Fee: fee, Lots: lots})
}

// quiet counts the quiet days retiring needs (#195): a day is quiet
// when every city's heat is under retire_heat, nobody struck or pushed
// a corner and the police answered nowhere (the tick's events: the
// rivals and heat sims step before this one), and no buyer's contract
// is live. A loud day zeroes the count. With no retire_days in the
// file nothing is counted, and a save from before the count starts at
// zero, which is what a loud day would have left.
func (s *Sim) quiet(w *game.World, t *game.Tick) {
	if s.cfg.Offshore.RetireDays <= 0 {
		return
	}
	loud := false
	for _, cid := range w.CityOrder {
		if w.Cities[cid].Heat >= s.cfg.Offshore.RetireHeat {
			loud = true
		}
	}
	for _, e := range t.Events() {
		switch e.(type) {
		case events.Enforcement, events.CornerStruck, events.RivalPushed, events.WarEscalated:
			loud = true
		}
	}
	for _, c := range w.Contracts {
		if c.Live(t.Day) {
			loud = true
		}
	}
	if loud {
		w.QuietDays = 0
		return
	}
	w.QuietDays++
}

// reportGrowth reports the levels bought at a front today (#192,
// FrontInvested, bookkeeping) and, the first time the front stands at
// the growth table's headline level or past it, that its growth made
// the paper (FrontGrew), stamping Grew so it is news once. The stamp is
// what the law sim reads the morning after for the pressure; the
// notoriety is the headline's. No dice.
func (s *Sim) reportGrowth(w *game.World, t *game.Tick, f *game.Front, fc content.FrontConfig) {
	levels, cost := 0, 0
	for _, inv := range w.Today.Invested {
		if inv.Front == f.ID {
			levels += inv.Levels
			cost += inv.Cost
		}
	}
	if levels > 0 {
		t.Emit(events.FrontInvested{Day: t.Day, Front: f.ID, Name: f.Name, Levels: levels, Level: f.Level, Cost: cost, Income: income(fc, f.Level)})
	}
	if hl := s.cfg.Growth.HeadlineLevel; hl > 0 && f.Level >= hl && f.Grew == 0 {
		f.Grew = t.Day
		t.Emit(events.FrontGrew{Day: t.Day, Front: f.ID, Name: f.Name, Level: f.Level})
	}
}
