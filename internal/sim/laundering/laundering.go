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
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the laundering simulation.
type Sim struct {
	cfg content.LaunderingConfig
	inf content.InformantTuning
}

// New builds a laundering sim from config. It takes the crew config for
// the loyalty line under which an audited front's accountant talks.
func New(cfg content.LaunderingConfig, crew content.CrewConfig) *Sim {
	return &Sim{cfg: cfg, inf: crew.Informant}
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
// dial, with the accountants' help, whether or not it is open.
func (s *Sim) Throughput(w *game.World, f game.Front) int {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	acct, _ := s.accountants(w)
	return int(math.Round((float64(fc.Throughput) + acct) * s.Dial(w.Laundering.Dial).Mul))
}

// AuditRisk is the chance a front is audited today at the current dial,
// after the accountants' cut.
func (s *Sim) AuditRisk(w *game.World, f game.Front) float64 {
	fc := s.cfg.Front(f.ID)
	if fc == nil {
		return 0
	}
	_, cut := s.accountants(w)
	return math.Max(0, math.Min(1, fc.AuditRisk*s.Dial(w.Laundering.Dial).Risk*cut))
}

// Washable is the dirty cash the fronts may take today: what is over the
// float.
func (s *Sim) Washable(w *game.World) int {
	return max(0, w.Player.DirtyCash-s.cfg.Laundering.Float)
}

// Capacity is the most the open fronts can wash today between them.
func (s *Sim) Capacity(w *game.World) int {
	n := 0
	for _, f := range w.Fronts {
		if !f.Frozen(w.Day + 1) {
			n += s.Throughput(w, f)
		}
	}
	return n
}

// Upkeep is what the open fronts cost in clean cash today between them.
func (s *Sim) Upkeep(w *game.World) int {
	n := 0
	for _, f := range w.Fronts {
		if fc := s.cfg.Front(f.ID); fc != nil && !f.Frozen(w.Day+1) {
			n += fc.Upkeep
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
	clear := 1.0
	for _, f := range w.Fronts {
		if !f.Frozen(w.Day + 1) {
			clear *= 1 - s.AuditRisk(w, f)
		}
	}
	return 1 - clear
}

// Step reports yesterday's purchases, then for every open front washes
// what it can, pays its upkeep and rolls for an audit. A front whose
// upkeep cannot be paid shuts; an audited one shuts longer and loses part
// of what it washed today. Heat reads the audit off the front tomorrow.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Laundering
	dial := w.Laundering.Dial
	total, upkeep, washing := 0, 0, 0
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
		if amt := min(s.Throughput(w, *f), s.Washable(w)); amt > 0 {
			w.Player.DirtyCash -= amt
			w.Player.CleanCash += amt
			f.WashedToday = amt
			f.Washed += amt
			w.Stats.Laundered += amt
			total += amt
			washing++
		}

		// 2. Upkeep, in clean cash. Unpaid, the place shuts.
		if fc.Upkeep > w.Player.CleanCash {
			f.FrozenUntil = t.Day + tun.UpkeepFreezeDays
			t.Emit(events.FrontFrozen{Day: t.Day, Front: f.ID, Name: f.Name, Upkeep: fc.Upkeep, Days: tun.UpkeepFreezeDays})
			continue
		}
		w.Player.CleanCash -= fc.Upkeep
		upkeep += fc.Upkeep

		// 3. The audit. It freezes the front and takes a slice of what
		// went through the books today.
		if t.RNG.Float64() >= s.AuditRisk(w, *f) {
			continue
		}
		seized := min(int(math.Round(float64(f.WashedToday)*tun.AuditSeize)), w.Player.CleanCash)
		w.Player.CleanCash -= seized
		w.Stats.Seized += seized
		f.FrozenUntil = t.Day + tun.AuditFreezeDays
		f.Audited = t.Day
		f.AuditDial = dial
		t.Emit(events.FrontAudited{Day: t.Day, Front: f.ID, Name: f.Name, Dial: dial, Seized: seized, Days: tun.AuditFreezeDays})

		// 4. The auditors question the books' keeper. The least loyal
		// accountant under the informant line (#13) is turned, no dice:
		// the heat sim starts its clock the day nobody was talking ends.
		s.flip(w, t)
	}
	if total > 0 || upkeep > 0 {
		t.Emit(events.CashLaundered{Day: t.Day, Amount: total, Upkeep: upkeep, Fronts: washing})
	}
}
