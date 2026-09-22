// Package heat simulates law-enforcement pressure. Every city has its own
// heat: it rises with what the player sold there today and how loudly,
// decays over time, and the hottest city's police answer at the
// thresholds. The case (the DA's file) and the response ladder are the
// player's, wherever they are.
package heat

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the heat simulation.
type Sim struct {
	cfg    content.HeatConfig
	market content.MarketConfig
	ship   content.ShippingTuning
	tree   content.UpgradesConfig
	rep    content.ReputationFX
	lt     content.LieutenantTuning
	law    content.LawConfig
	houses content.HousesTuning
	deed   content.DeedTuning   // #194: raid_mul on a house on a deeded block, forfeit_evidence the morning after a forfeiture
	intel  content.IntelTuning  // #45: what a cop's word is worth
	assets content.AssetsConfig // #48: the floor an owned asset puts under every city, and which asset the task force takes
}

// New builds a heat sim from the config, copying what it reads (#144):
// its own heat.toml; the market config for per-product and per-dial
// heat multipliers, the shipping tuning for what a seizure on
// the road adds, the upgrade tree for what the Security and Legal
// branches take off, of the reputation effects the two that are its
// (fear puts a floor under heat, notoriety makes you the target), the
// lieutenant tuning for what a temper does to a city's heat and what a
// flipped one feeds the DA, and the law tables (#41) for what the chief,
// the DA and a city's pressure do to its own thresholds, cooldown and
// decay; it reads who they are off w.Law and never adds a page for them;
// and the houses' tuning (#73) for what a unit moved between places
// draws; and the assets (#48) for the heat floor an owned one puts
// under every city, since the task force that takes one is this sim's
// rung.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Heat, market: cfg.Market, ship: cfg.Routes.Shipping, tree: cfg.Upgrades, rep: cfg.Reputation.Effects, lt: cfg.Crew.Lieutenant, law: cfg.Law, houses: cfg.Houses.Houses, deed: cfg.City.Deed, assets: cfg.Assets, intel: cfg.Intel.Intel}
}

// RaidWeight is a house's weight in the raid's roll over the places
// holding stock (#73): the heat of its block, times city.toml [deed]
// raid_mul where the block is yours (#194: a house on a deeded block
// is not known from the street). A house on no block weighs 1.
func (s *Sim) RaidWeight(w *game.World, h *game.House) float64 {
	weight := 1.0
	if c := w.Corner(h.Corner); c != nil {
		weight = c.Heat
		if c.Deed != nil && s.deed.On() {
			weight *= s.deed.RaidMul
		}
	}
	return weight
}

// Chief is what the sitting police chief does to the tuning: multipliers
// on the response cooldown, what a patrol lets through and the decay.
func (s *Sim) Chief(w *game.World) content.ChiefConfig {
	return s.law.ChiefFor(w.Law.Chief.Personality)
}

// DA is what the sitting district attorney does to the tuning:
// multipliers on the pages an indictment needs and the sting line.
func (s *Sim) DA(w *game.World) content.DAConfig { return s.law.DAFor(w.Law.DA.Stance) }

// CooldownDays is how long a response level waits before it can fire
// again: the base, plus the Security branch, and for a sting or a raid
// times the chief, plus bribe_cooldown at the bought chief's share
// (#42), never under one day. The patrol keeps its cadence under every
// chief: it fires as often as its cap lifts, and a chief who sent it
// back sooner would never lift it. A rung with cooldown_days of its own
// (#48, the task force: federal, and long) waits that, flat.
func (s *Sim) CooldownDays(w *game.World, level string) int {
	if r := s.rung(level); r != nil && r.Cooldown > 0 {
		return r.Cooldown
	}
	days := float64(s.cfg.Heat.CooldownDays + s.Effects(w).CooldownBonus)
	if level != content.Patrol {
		days *= s.Chief(w).Cooldown
		days += float64(s.law.Effects.BribeCooldown) * s.Bought(w)
	}
	return max(1, int(math.Round(days)))
}

// BribeDecayMul and BribeCooldown are what a bought chief is worth at
// full share (#42), for the UI to explain itself.
func (s *Sim) BribeDecayMul() float64 { return math.Max(1, s.law.Effects.BribeDecayMul) }
func (s *Sim) BribeCooldown() int     { return s.law.Effects.BribeCooldown }

// Bought is the sitting chief's share of a bribe's effect today (#42):
// 1 for a corrupt one who took it, lazy_effect for a lazy one, 0 for
// nobody bought or a deal the cold ended.
func (s *Sim) Bought(w *game.World) float64 {
	if !w.Law.ChiefBoughtOn(w.Day) {
		return 0
	}
	return max(0, min(1, w.Law.ChiefShare))
}

// Decay is the fraction of heat above the floor that fades in a day: the
// base or the cold contacts, times the chief, times bribe_decay_mul at
// the bought chief's share (#42).
func (s *Sim) Decay(w *game.World) float64 {
	d := math.Max(s.cfg.Heat.Decay, s.Effects(w).Decay) * s.Chief(w).Decay
	if mul := s.law.Effects.BribeDecayMul; mul > 0 {
		d *= 1 + (mul-1)*s.Bought(w)
	}
	return math.Min(1, d)
}

// PatrolCap is the share of demand a patrol in a city lets through: the
// response's, or the lookouts', times the chief, less what the city's
// pressure takes off, never over one.
func (s *Sim) PatrolCap(w *game.World, r content.ResponseConfig, city *game.City) float64 {
	cap := math.Max(r.Cap, s.Effects(w).PatrolCap) * s.Chief(w).Cap
	if city != nil {
		cap *= content.Cut(city.Pressure, s.law.Effects.PressureCapCut)
	}
	return math.Min(1, cap)
}

// Threshold is the heat at which a response fires in a city today: the
// ladder's line, moved by the law (#41). The sting line is the DA's: it
// is where a case starts, and a law-and-order DA wants it lower, a
// reformer higher, and one whose ticket ran on your money (#193,
// DA.Backed) higher again by law.toml's backed_sting. Every other line
// (the patrols, the raid, the task force, the arrest) is the police's,
// and drops the louder the city is: yesterday's pressure, since the
// law sim steps after this one. The task force's (#48) drops again
// under an extradition treaty (an incident's window on the heat state,
// LineUntil / LineMul), read for tonight's tick.
func (s *Sim) Threshold(w *game.World, r content.ResponseConfig, city *game.City) float64 {
	v := r.Threshold
	if r.Level == content.Sting {
		v *= s.DA(w).Sting
		if w.Law.DA.Backed && s.law.Effects.BackedSting > 0 {
			v *= s.law.Effects.BackedSting
		}
		return v
	}
	if city != nil {
		v *= content.Cut(city.Pressure, s.law.Effects.PressureThresholdCut)
	}
	if r.Level == content.TaskForce && w.Day+1 < w.Heat.LineUntil && w.Heat.LineMul > 0 {
		v *= w.Heat.LineMul
	}
	return v
}

// ThresholdsIn is the response ladder as it stands in a city today, for
// the UI: the same lines the dice use, every rung.
func (s *Sim) ThresholdsIn(w *game.World, city *game.City) []content.ResponseConfig {
	out := s.Thresholds()
	for i := range out {
		out[i].Threshold = s.Threshold(w, out[i], city)
	}
	return out
}

// Ladder is the ladder the player faces in a city today (#48): the
// lines as they stand, without the task force's rung while it cannot
// form (TaskForceEligible). The dashboard's gauge marks it, so the
// task force's line shows only once an asset is owned or the pile is
// over taskforce_cash.
func (s *Sim) Ladder(w *game.World, city *game.City) []content.ResponseConfig {
	all := s.ThresholdsIn(w, city)
	if s.TaskForceEligible(w) {
		return all
	}
	out := all[:0:0]
	for _, r := range all {
		if r.Level != content.TaskForce {
			out = append(out, r)
		}
	}
	return out
}

// TaskForceEligible reports whether a task force can form against the
// player (#48): an asset owned, or dirty cash over taskforce_cash. A
// player with neither plays the four-rung ladder as it always was; the
// rung is skipped, so nothing about their run moves.
func (s *Sim) TaskForceEligible(w *game.World) bool {
	if len(w.Assets) > 0 {
		return true
	}
	line := s.cfg.Heat.TaskforceCash
	return line > 0 && w.Player.DirtyCash > line
}

// TaskForceCash is the dirty cash over which a task force can form with
// no asset owned, for the UI.
func (s *Sim) TaskForceCash() int { return s.cfg.Heat.TaskforceCash }

// TaskForceForming reports whether a task force was announced this
// morning and comes tonight (#48): the day to lie low.
func (s *Sim) TaskForceForming(w *game.World) bool {
	return w.Heat.TaskForceDay > 0 && w.Heat.TaskForceDay == w.Day
}

// rung is the file's row for a level, or nil.
func (s *Sim) rung(level string) *content.ResponseConfig {
	for i := range s.cfg.Responses {
		if s.cfg.Responses[i].Level == level {
			return &s.cfg.Responses[i]
		}
	}
	return nil
}

// AssetFloor is the heat floor the assets owned and standing put under
// every city (#48): the highest heat_floor among them, federal
// attention that decay never takes a city under. Nothing with none.
func (s *Sim) AssetFloor(w *game.World) float64 {
	floor := 0.0
	for _, a := range s.assets.Offers {
		if w.AssetLive(a.ID) {
			floor = math.Max(floor, a.HeatFloor)
		}
	}
	return floor
}

// Cover is how much of a dirty-cash pile the player's fronts give a
// story to, on top of heat.toml's threshold: dirty_cash_cover times what
// they paid for every front. A rich hider with no fronts is covered for
// nothing (#27 holds: the pile still draws the police, it is just not a
// countdown); a tier-4 operation whose takings outrun the wash ladder is
// covered for what its fronts are worth.
func (s *Sim) Cover(w *game.World) int {
	cost := 0
	for _, f := range w.Fronts {
		cost += f.Cost
	}
	return int(math.Round(s.cfg.Heat.DirtyCashCover * float64(cost)))
}

// DirtyCashThreshold is the dirty cash the police read nothing into,
// before what the fronts cover: heat.toml's line, raised by the Security
// branch (dirty_cash_threshold_mul). The dashboard's warning reads it,
// so the line the player sees is the one the dice use.
func (s *Sim) DirtyCashThreshold(w *game.World) int {
	return int(math.Round(float64(s.cfg.Heat.DirtyCashThreshold) * s.Effects(w).DirtyCashThresholdMul))
}

// Floor is the heat a feared player never cools below: decay works on
// what is above it. A nobody's floor is zero. An asset owned (#48) puts
// its own floor under every city, and the higher of the two holds.
func (s *Sim) Floor(w *game.World) float64 {
	return max(s.rep.FearHeatFloor*max(0, min(1, w.Player.Reputation.Fear/100)), s.AssetFloor(w))
}

// Effects is what the player's upgrades do to heat today.
func (s *Sim) Effects(w *game.World) game.Effects { return game.FoldEffects(w, s.tree) }

func (s *Sim) Name() string { return "heat" }

// Thresholds returns the response thresholds in ascending order, for the UI.
func (s *Sim) Thresholds() []content.ResponseConfig {
	out := append([]content.ResponseConfig(nil), s.cfg.Responses...)
	sort.Slice(out, func(i, j int) bool { return out[i].Threshold < out[j].Threshold })
	return out
}

// day is one Step's working state (#275): what its phases share. from is
// each city's heat as the day began, reasons what its report says moved
// it, units what you moved there and attempted whether you dealt there
// (a sting on a quiet day files nothing, #27); floor is the fear and
// asset floor the decay leaves and settle clamps to, hot the city
// whose police answered tonight.
type day struct {
	w         *game.World
	t         *game.Tick
	tun       content.HeatTuning
	fx        game.Effects
	h         *game.HeatState
	here      string
	home      string
	from      map[string]float64
	reasons   map[string][]string
	units     map[string]int
	attempted map[string]bool
	floor     float64
	hot       *game.City
}

// Step applies today's heat sources to every city, decays each, then has
// the hottest city's police check the thresholds. It runs as phases in a
// fixed order (#275): the sources (sources.go) and the pages
// (evidence.go) in the order the reasons read, the decay and the ladder
// (ladder.go), the file going cold and the case (evidence.go, exit.go),
// then the settling and the cop (intel.go). The order is the reasons'
// order and the dice's: a phase moved is a report that reads differently.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	d := &day{
		w: w, t: t, tun: s.cfg.Heat, fx: s.Effects(w), h: &w.Heat,
		here: w.Player.Location, home: w.Home().ID,
		from: map[string]float64{}, reasons: map[string][]string{},
		units: map[string]int{}, attempted: map[string]bool{},
	}
	for _, cid := range w.CityOrder {
		d.from[cid] = w.Cities[cid].Heat
	}

	s.sales(d)
	s.war(d)
	s.contracts(d)
	s.moves(d)
	s.sloppy(d)
	s.informants(d)
	s.envelopes(d)
	s.forfeiture(d)
	s.dirtyCash(d)
	s.audits(d)
	s.structuring(d)

	s.cool(d)
	s.sellCap(d)
	s.respond(d)

	s.cold(d)
	s.indict(d)
	s.settle(d)

	// A cop paid today (#45) says what the police here do next: the
	// rung the ladder stands at and the first night it can fire, as the
	// night leaves them, at the cop's accuracy off the intel stream.
	s.cop(w, t)
}

// settle clamps every city to the floor and 100, records the peak and
// sends each city's HeatChanged with the day's reasons.
func (s *Sim) settle(d *day) {
	for _, cid := range d.w.CityOrder {
		c := d.w.Cities[cid]
		c.Heat = max(d.floor, min(100, c.Heat))
		if c.Heat > d.h.Peak {
			d.h.Peak = c.Heat
		}
		d.t.Emit(events.HeatChanged{Day: d.t.Day, City: cid, From: d.from[cid], To: c.Heat, Reasons: d.reasons[cid]})
	}
}
