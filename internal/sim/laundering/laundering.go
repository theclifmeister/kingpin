// Package laundering simulates the fronts: businesses that turn dirty cash
// into clean cash at a capped rate, cost upkeep to keep open, and get
// audited when pushed. The launder dial trades throughput against audits
// across every front at once. The wash always leaves a float of dirty
// cash in the till, so owning more fronts than you feed never starves the
// street. Heat learns of an audit the morning after, from the front's
// record: this sim steps after heat.
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
	return int(math.Round((float64(fc.Throughput) + acct) * s.Dial(w.Laundering.Dial).Mul * fx.WashMul))
}

// AuditRisk is the chance a front is audited today at the current dial,
// after the accountants' cut and the tree's audit_risk_mul.
func (s *Sim) AuditRisk(w *game.World, f game.Front) float64 {
	return s.auditRisk(w, f, game.FoldEffects(w, s.tree))
}

func (s *Sim) auditRisk(w *game.World, f game.Front, fx game.Effects) float64 {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	_, cut := s.accountants(w)
	return math.Max(0, math.Min(1, fc.AuditRisk*s.Dial(w.Laundering.Dial).Risk*cut*fx.AuditRiskMul))
}

// FrontUpkeep is what a front costs in clean cash a day, after the
// tree's upkeep_mul.
func (s *Sim) FrontUpkeep(w *game.World, f game.Front) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	return upkeep(*fc, game.FoldEffects(w, s.tree))
}

func upkeep(fc content.FrontConfig, fx game.Effects) int {
	return int(math.Round(float64(fc.Upkeep) * fx.UpkeepMul))
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
			n += upkeep(*fc, fx)
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
			clear *= 1 - s.auditRisk(w, f, fx)
		}
	}
	return 1 - clear
}

// Step reports yesterday's purchases, then for every open front washes
// what it can, pays its upkeep and rolls for an audit. A front whose
// upkeep cannot be paid shuts; an audited one shuts longer and loses part
// of what it washed today. Heat reads the audit off the front tomorrow.
// The tree folds once at the top (#118) and every number below is the
// tuning times it, the same number the ledger shows.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Laundering
	fx := game.FoldEffects(w, s.tree)
	dial := w.Laundering.Dial
	total, paid, washing := 0, 0, 0
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

		// 2. Upkeep, in clean cash. Unpaid, the place shuts.
		due := upkeep(*fc, fx)
		if due > w.Player.CleanCash {
			f.FrozenUntil = t.Day + tun.UpkeepFreezeDays
			t.Emit(events.FrontFrozen{Day: t.Day, Front: f.ID, Name: f.Name, Upkeep: due, Days: tun.UpkeepFreezeDays})
			continue
		}
		w.Player.CleanCash -= due
		paid += due

		// 3. The audit. It freezes the front and takes a slice of what
		// went through the books today.
		if t.RNG.Float64() >= s.auditRisk(w, *f, fx) {
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
	if total > 0 || paid > 0 {
		t.Emit(events.CashLaundered{Day: t.Day, Amount: total, Upkeep: paid, Fronts: washing})
	}
	s.announce(w, t)
}
