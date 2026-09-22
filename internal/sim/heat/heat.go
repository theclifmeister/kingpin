// Package heat simulates law-enforcement pressure. Every city has its own
// heat: it rises with what the player sold there today and how loudly,
// decays over time, and the hottest city's police answer at the
// thresholds. The case (the DA's file) and the response ladder are the
// player's, wherever they are.
package heat

import (
	"fmt"
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

// ForfeitEvidence is the pages a deed the DA seized files the morning
// after (#194): what the property dialog warns with.
func (s *Sim) ForfeitEvidence() int { return s.deed.ForfeitEvidence }

// StructureEvidence is the pages a lot of clean cash moved offshore
// over the line files the morning after (#195): what the reserve dialog
// warns with.
func (s *Sim) StructureEvidence() int { return s.cfg.Heat.StructureEvidence }

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
	return math.Max(0, math.Min(1, w.Law.ChiefShare))
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

// LieutenantHeat is what the temper of whoever runs a city does to the
// heat of every sale there: 1 for a city nobody runs.
func (s *Sim) LieutenantHeat(w *game.World, city string) float64 {
	if lt := w.Crew.Lieutenant(city); lt != nil {
		return s.lt.Temper(lt.Personality).Heat
	}
	return 1
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
	return math.Max(s.rep.FearHeatFloor*math.Max(0, math.Min(1, w.Player.Reputation.Fear/100)), s.AssetFloor(w))
}

// PersonalHeat is the weight of a unit you move yourself, relative to a
// unit a nobody moves: notoriety makes you the one they are watching.
func (s *Sim) PersonalHeat(w *game.World) float64 {
	return content.Scale(w.Player.Reputation.Notoriety, s.rep.NotorietyHeat)
}

// Effects is what the player's upgrades do to heat today.
func (s *Sim) Effects(w *game.World) game.Effects { return game.FoldEffects(w, s.tree) }

// EvidenceArrest is how thick the DA's file has to be for an indictment,
// after a retained lawyer has had his say and for the DA in office: a
// law-and-order DA needs fewer pages, a reformer more, a bought one
// (#42) more again, never under one.
func (s *Sim) EvidenceArrest(w *game.World) int {
	base := max(s.cfg.Heat.EvidenceArrest, s.Effects(w).EvidenceArrest)
	if base <= 0 {
		return 0
	}
	v := float64(base) * s.DA(w).EvidenceArrest
	// A bought DA sits on the file (#42): more pages before it is a case.
	if mul := s.law.Effects.BribedDAEvidenceMul; mul > 0 && w.Law.DABoughtOn(w.Day) {
		v *= mul
	}
	return max(1, int(math.Round(v)))
}

func (s *Sim) Name() string { return "heat" }

// Thresholds returns the response thresholds in ascending order, for the UI.
func (s *Sim) Thresholds() []content.ResponseConfig {
	out := append([]content.ResponseConfig(nil), s.cfg.Responses...)
	sort.Slice(out, func(i, j int) bool { return out[i].Threshold < out[j].Threshold })
	return out
}

func (s *Sim) dialHeat(d events.Dial) float64 {
	switch d {
	case events.DialQuiet:
		return s.market.Dial.Quiet.Heat
	case events.DialAggressive:
		return s.market.Dial.Aggressive.Heat
	default:
		return s.market.Dial.Normal.Heat
	}
}

func (s *Sim) dialFill(d events.Dial) float64 {
	switch d {
	case events.DialQuiet:
		return s.market.Dial.Quiet.Fill
	case events.DialAggressive:
		return s.market.Dial.Aggressive.Fill
	default:
		return s.market.Dial.Normal.Fill
	}
}

// SaleHeat is the heat drawn in a city by trying to move wanted units of
// a product there at a dial. Heat follows volume: every unit is a
// transaction somebody could see, weighted by how much the product itself
// draws attention, how loud the dial is, which corners it moves on
// (CornerWeight) and how closely the city's police look. The UI's dial
// preview uses it too, so the estimate is always honest.
func (s *Sim) SaleHeat(w *game.World, city, product string, wanted int, dial events.Dial) float64 {
	tun := s.cfg.Heat
	pc := s.market.Product(product)
	c := w.City(city)
	if pc == nil || c == nil || tun.StreetUnits <= 0 {
		return 0
	}
	fx := s.Effects(w)
	attempted := math.Min(float64(wanted), math.Round(w.Demand(city, product)*fx.DemandMul*s.dialFill(dial)*fx.FillMul))
	return tun.SaleHeat * fx.SaleHeatMul * attempted * s.CornerWeight(w, city, product) * c.HeatMul * pc.Heat / tun.StreetUnits * s.dialHeat(dial) * s.LieutenantHeat(w, city)
}

// CrewHeat is what a unit a runner moves draws relative to one you move
// yourself: the tuning, times what the Security branch takes off it (#60:
// the tier-3 and tier-4 nodes are how an operation's volume outgrows the
// street's notice while you, on your own corner, are as hot as ever).
func (s *Sim) CrewHeat(w *game.World) float64 {
	return s.cfg.Heat.CrewHeat * s.Effects(w).CrewHeatMul
}

// ContractHeat is the heat a handoff of units of a product to a buyer
// draws in a city (#71): the per-unit weight a street sale carries (the
// tuning, the Security branch, the product and the city) with the
// buyer's own multiplier where a sale has its corners and its dial. A
// bulk handoff is one big exposure: no corner discounts it, no patrol
// caps it, and the volume is what scales it. It is SaleHeat's formula
// less the fill, the corner weight, the dial and the lieutenant, which
// that signature cannot leave out; the two sit side by side on purpose.
// The market screen's buyers panel previews it.
func (s *Sim) ContractHeat(w *game.World, city, product string, units int, mul float64) float64 {
	tun := s.cfg.Heat
	pc := s.market.Product(product)
	c := w.City(city)
	if pc == nil || c == nil || tun.StreetUnits <= 0 || units <= 0 {
		return 0
	}
	fx := s.Effects(w)
	return tun.SaleHeat * fx.SaleHeatMul * float64(units) * c.HeatMul * pc.Heat / tun.StreetUnits * mul
}

// MoveHeat is the heat moving units of a product between two places in
// a city draws (#73): a unit sold on a standard corner's weight (the
// tuning, the Security branch, the product and the city) at move_heat,
// on no corner and at no dial. It is not dealing: no page (#27).
func (s *Sim) MoveHeat(w *game.World, city, product string, units int) float64 {
	return s.ContractHeat(w, city, product, units, s.houses.MoveHeat)
}

// CornerWeight is the heat one unit of a product draws on average across
// the corners it moves on in a city, relative to a unit a nobody moves
// themselves on a standard corner. A sale spreads over the worked corners
// by their share; each corner has its own heat, a unit a runner moves
// counts at the crew discount (they are on the corner, you are not), and
// a unit you move yourself counts your notoriety.
func (s *Sim) CornerWeight(w *game.World, city, product string) float64 {
	total, weighted := 0.0, 0.0
	personal, crew := s.PersonalHeat(w), s.CrewHeat(w)
	c0 := w.City(city)
	if c0 == nil {
		return 0
	}
	for _, c := range c0.Corners {
		if !c.Worked() {
			continue
		}
		share := c.Share(product)
		total += share
		unit := c.Heat
		if c.Runner != game.You {
			unit *= crew
		} else {
			unit *= personal
		}
		weighted += share * unit
	}
	if total <= 0 {
		return 0
	}
	return weighted / total
}

// SloppyHeat is the premium low-skill runners add for moving units in a
// city today.
func (s *Sim) SloppyHeat(w *game.World, city string, units int) float64 {
	return s.Sloppiness(w, city) * s.cfg.Heat.SloppyHeat * float64(units)
}

// Sloppiness is how much of a city's volume moves through a sloppy
// runner's hands, as a fraction: each runner working a corner there
// counts how far they fall below the sloppy-skill line (a skill-0 runner
// 1, a skilled one 0) times that corner's share of the corners you work
// there. A runner without a corner is not on the street to be noticed.
func (s *Sim) Sloppiness(w *game.World, city string) float64 {
	line := float64(s.cfg.Heat.SloppySkill)
	c0 := w.City(city)
	if line <= 0 || c0 == nil {
		return 0
	}
	total, sloppy := 0.0, 0.0
	for _, c := range c0.Corners {
		if !c.Worked() {
			continue
		}
		total += c.Demand
		if c.Runner == game.You {
			continue
		}
		if m := w.Crew.Member(c.Runner); m != nil && float64(m.Skill) < line {
			sloppy += c.Demand * (line - float64(m.Skill)) / line
		}
	}
	if total <= 0 {
		return 0
	}
	return sloppy / total
}

// Step applies today's heat sources to every city, decays each, then has
// the hottest city's police check the thresholds.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Heat
	fx := s.Effects(w)
	h := &w.Heat
	here := w.Player.Location
	home := w.Home().ID
	from := map[string]float64{}
	reasons := map[string][]string{}
	units := map[string]int{}
	attempted := map[string]bool{}
	for _, cid := range w.CityOrder {
		from[cid] = w.Cities[cid].Heat
	}
	add := func(city string, v float64, why string) {
		c := w.Cities[city]
		if c == nil {
			return
		}
		c.Heat += v
		if why != "" {
			reasons[city] = append(reasons[city], fmt.Sprintf("%s (+%.1f)", why, v))
		}
	}

	// Sales from the market sim, earlier in this tick, city by city. Heat
	// follows the volume you tried to move at that dial, not what a
	// patrol cap let through: standing on a corner shouting is the
	// exposure. Which corners, and whether you or a runner stood there,
	// weight it.
	for _, e := range t.Events() {
		ps, ok := e.(events.PlayerSold)
		if !ok || ps.Wanted == 0 {
			continue
		}
		attempted[ps.City] = true
		why := fmt.Sprintf("moved %d %s %s", ps.Sold, w.ProductName(ps.Product), ps.Dial)
		if ps.Delegated {
			why = fmt.Sprintf("%s moved %d %s %s", ps.LieutenantName, ps.Sold, w.ProductName(ps.Product), ps.Dial)
		} else if ps.Standing {
			why = fmt.Sprintf("standing order moved %d %s %s", ps.Sold, w.ProductName(ps.Product), ps.Dial)
		}
		add(ps.City, s.SaleHeat(w, ps.City, ps.Product, ps.Wanted, ps.Dial), why)
		units[ps.City] += ps.Sold
	}

	// The war, from the rivals sim, in the rival's city: enforcers you
	// sent in, a rival's call to the precinct, the police clearing the
	// front line. A shipment seized on the road, from the logistics sim,
	// is heat in both cities it joined.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			add(home, ev.Heat, fmt.Sprintf("enforcers %s %s", pastTense(ev.Force), ev.Name))
		case events.RivalBoosted:
			add(home, ev.Heat, "enforcers robbed "+ev.Name)
		case events.PoliceTipped:
			// Your tip on a rival corner (#70): no heat, but talking to
			// the police is talking to the police, and tip_evidence is
			// the chance the DA's file gains a page anyway. #27's rule
			// bends only because you did something; the roll is the
			// books side stream's, so a run that never tips keeps its
			// dice. It does not reset the retainer's clock unless it
			// files.
			if tun.TipEvidence > 0 && t.Sub("books").Float64() < tun.TipEvidence {
				h.Evidence++
				h.EvidenceDay = t.Day
				reasons[home] = append(reasons[home], fmt.Sprintf("your tip: the DA's file on you grows (%d)", h.Evidence))
			}
		case events.RivalTippedPolice:
			add(home, ev.Heat, "somebody tipped the police")
		case events.FactionPushed:
			// Two factions fighting (#43): the heat is the city's, both
			// sides' wars raising it.
			add(ev.City, ev.Heat, "the factions fought over "+ev.Name)
		case events.WarEscalated:
			if ev.Stage == "crackdown" {
				add(home, ev.Heat, "the crackdown")
			}
		case events.ShipmentSeized:
			why := fmt.Sprintf("%d %s seized on the %s", ev.Units, w.ProductName(ev.Product), ev.Mode)
			add(ev.From, s.ship.SeizureHeat, why)
			add(ev.To, s.ship.SeizureHeat, why)
			// Sent fast, it was asking to be looked at: a page in the
			// file (#27: a case is built from what you did). A seizure
			// is not a bust and never a reason to search the stash.
			if ev.Dial == events.ShipFast && s.ship.SeizureEvidence > 0 {
				h.Evidence += s.ship.SeizureEvidence
				h.EvidenceDay = t.Day
				reasons[ev.To] = append(reasons[ev.To], fmt.Sprintf("sent fast: the DA's file on you grows (%d)", h.Evidence))
			}
		}
	}

	// A buyer's contract handed over today, from the market sim (#71). It
	// is dealing like any sale: the city counts as attempted, so a sting
	// tonight finds something to file (#27 holds for a handoff exactly as
	// for a sale), and it weighs what the buyer's own multiplier says, on
	// no corner at all.
	for _, e := range t.Events() {
		cd, ok := e.(events.ContractDelivered)
		if !ok || cd.Units == 0 {
			continue
		}
		attempted[cd.City] = true
		add(cd.City, s.ContractHeat(w, cd.City, cd.Product, cd.Units, cd.HeatMul), fmt.Sprintf("handed %d %s to %s", cd.Units, w.ProductName(cd.Product), cd.Name))
	}

	// Stock driven between places today (#73): a car ride is exposure,
	// not dealing, so it is heat and never a page.
	for _, mv := range w.Today.Moved {
		if v := s.MoveHeat(w, mv.City, mv.Product, mv.Units); v > 0 {
			add(mv.City, v, fmt.Sprintf("moved %d %s between places", mv.Units, w.ProductName(mv.Product)))
		}
		t.Emit(events.StockMoved{Day: t.Day, City: mv.City, From: placeName(w, mv.From), To: placeName(w, mv.To), Product: mv.Product, Units: mv.Units})
	}

	// Sloppy runners get noticed: every unit moved with a low-skill crew
	// on the corners adds a premium.
	for _, cid := range w.CityOrder {
		if v := s.SloppyHeat(w, cid, units[cid]); v > 0 {
			add(cid, v, "sloppy crew")
		}
	}

	// An informant on the payroll. The crew sim's turn event starts the
	// clock (the laundering sim, stepping after this one, flips an
	// accountant without it: the clock then runs from today, the last day
	// nobody was talking); every informant_days after that the DA gets a
	// page whatever was sold, and the lawyer cannot thin a witness. The
	// heat it adds, where you are, is left out of the reasons on purpose:
	// a delta the dial does not explain, and a file that grew without a
	// bust, are the tells. Once nobody is talking the count that shows
	// them resets.
	// A flipped lieutenant is the same clock with thicker pages: they
	// know where everything is.
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CrewTurnedInformant:
			h.LeakDay = ev.Day
		case events.LieutenantFlipped:
			h.LeakDay = ev.Day
		case events.CrewRetired:
			// A sour retiree talks on the way out (#46): one page, the
			// informant's, whatever was sold, the exception applied
			// once. Somebody on your payroll did something.
			if ev.Sour && tun.InformantEvidence > 0 {
				h.Evidence += tun.InformantEvidence
				h.EvidenceDay = t.Day
				reasons[here] = append(reasons[here], fmt.Sprintf("%s talked on the way out: the DA's file on you grows (%d)", ev.Name, h.Evidence))
			}
		}
	}
	if w.Crew.Informants() == 0 {
		h.Leaks = 0
		h.LeakDay = t.Day
	} else if tun.InformantDays > 0 && t.Day-h.LeakDay >= tun.InformantDays {
		h.LeakDay = t.Day
		h.Leaks++
		add(here, tun.InformantHeat, "")
		pages := tun.InformantEvidence
		for _, m := range w.Crew.Members {
			if m.Informant && m.Lieutenant() {
				pages = max(pages, s.lt.Evidence)
			}
		}
		if pages > 0 {
			h.Evidence += pages
			h.EvidenceDay = t.Day
			reasons[here] = append(reasons[here], fmt.Sprintf("the DA's file on you grows (%d)", h.Evidence))
		}
	}

	// An envelope that blew up last night (#42, w.Law.Backfired: the law
	// sim steps after this one, so its night is our morning) is heat
	// where you are and pages in the file whatever was sold: bribing is
	// something you did, the second bend in #27 besides the informant's.
	// The DA's file on your envelopes (w.Law.Filed) is pages the same way.
	if b := s.law.Bribes; w.Law.Backfired > 0 && w.Law.Backfired == t.Day-1 {
		add(here, b.BackfireHeat, "the envelope came back")
		if b.BackfireEvidence > 0 {
			h.Evidence += b.BackfireEvidence
			h.EvidenceDay = t.Day
			reasons[here] = append(reasons[here], fmt.Sprintf("the bribe backfired: the DA's file on you grows (%d)", h.Evidence))
		}
	}
	// The favour called in this morning (#228, w.Law.FavourOwed, the
	// night it is owed on, this tick's): the chief's name is in your ledger now, and the
	// DA's file gains favour_evidence pages whatever was sold, the
	// third bend in #27 beside the informant's and the backfire's,
	// because calling it was something you did. The response it stops
	// is below, with the ladder.
	if b := s.law.Bribes; w.Law.FavourOwed == t.Day && b.FavourEvidence > 0 {
		h.Evidence += b.FavourEvidence
		h.EvidenceDay = t.Day
		reasons[here] = append(reasons[here], fmt.Sprintf("the favour: the chief's name is in your ledger, and the DA's file on you grows (%d)", h.Evidence))
	}
	if b := s.law.Bribes; w.Law.Filed > 0 && w.Law.Filed == t.Day-1 && b.LeadEvidence > 0 {
		h.Evidence += b.LeadEvidence
		h.EvidenceDay = t.Day
		reasons[here] = append(reasons[here], fmt.Sprintf("the DA opened a file on your envelopes: the DA's file on you grows (%d)", h.Evidence))
	}
	// A deed the DA seized last night (#194, w.Law.Forfeited, the same
	// way): the money had no story, and buying the block was something
	// you did. Pages where you are, whatever was sold.
	if w.Law.Forfeited > 0 && w.Law.Forfeited == t.Day-1 && s.deed.ForfeitEvidence > 0 {
		h.Evidence += s.deed.ForfeitEvidence
		h.EvidenceDay = t.Day
		reasons[here] = append(reasons[here], fmt.Sprintf("the forfeiture: the DA's file on you grows (%d)", h.Evidence))
	}

	// Sitting on a pile of dirty cash is its own tell, wherever you sit,
	// past what your fronts give a story to (Cover).
	if thr := s.DirtyCashThreshold(w); thr > 0 && w.Player.DirtyCash > thr+s.Cover(w) {
		mult := float64(w.Player.DirtyCash-thr-s.Cover(w)) / float64(thr)
		add(here, tun.DirtyCashHeat*mult, "dirty cash")
	}

	// An audit at one of your fronts yesterday is a tell too: the books
	// were looked at. Only a front that was being run greedy gives the DA
	// something to file (#27: a case is built from what you did, not
	// what you have). Laundering steps after heat, so the front's record
	// is how yesterday's audit reaches today's heat.
	for _, f := range w.Fronts {
		if f.Audited == 0 || f.Audited != t.Day-1 {
			continue
		}
		if pages := max(0, tun.AuditEvidence-fx.AuditEvidenceCut); f.AuditDial == events.LaunderGreedy && pages > 0 {
			h.Evidence += pages
			h.EvidenceDay = t.Day
			add(here, tun.AuditHeat, fmt.Sprintf("audit at %s, run greedy: the DA's file grows", f.Name))
		} else {
			add(here, tun.AuditHeat, fmt.Sprintf("audit at %s", f.Name))
		}
	}

	// Clean cash moved offshore yesterday over the lot (#195): the DA
	// reads the transfers, a page a lot over the line, wherever you
	// are (structuring is something you did, #27; a move under the
	// lot, or none, files nothing). Laundering steps after heat, so
	// its record is how last night's move reaches today's file.
	if st := w.Laundering.Structured; st.Day != 0 && st.Day == t.Day-1 && st.Lots > 0 && tun.StructureEvidence > 0 {
		pages := st.Lots * tun.StructureEvidence
		h.Evidence += pages
		h.EvidenceDay = t.Day
		reasons[here] = append(reasons[here], fmt.Sprintf("money moved offshore in lumps: the DA's file grows (%d)", h.Evidence))
	}

	// Decay, in every city. Cold contacts make both the base rate and
	// lying low better; a zealous chief makes it worse. A feared name
	// never quite cools: decay works on what is above the floor, and
	// nothing takes heat under it.
	decay := s.Decay(w)
	if t.Day < w.Heat.FederalUntil && w.Heat.FederalDecay > 0 {
		decay *= w.Heat.FederalDecay // the feds are in town (#44): what you draw, you keep
	}
	if w.Today.LieLow {
		decay *= math.Max(tun.LieLowMultiplier, fx.LieLowMultiplier)
		t.Emit(events.LaidLow{Day: t.Day})
		for _, cid := range w.CityOrder {
			reasons[cid] = append(reasons[cid], "lay low")
		}
	}
	floor := s.Floor(w)
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		if c.Heat > floor {
			c.Heat -= (c.Heat - floor) * decay
		}
		c.Heat = math.Max(floor, math.Min(100, c.Heat))
	}

	if h.SellCapDays > 0 {
		h.SellCapDays--
		if h.SellCapDays == 0 {
			h.SellCap = 0
		}
	}

	// Threshold responses, highest first, one per day, from the police
	// of the hottest city; what they take comes out of the stash there.
	if h.LastResponse == nil {
		h.LastResponse = map[string]int{}
	}
	if h.Responses == nil {
		h.Responses = map[string]int{}
	}
	hot := s.hottest(w)
	resp := s.Thresholds()
	// The favour called in this morning (#228): the chief's people stand
	// down tonight. The one response that would have fired does not
	// (nothing taken, no heat drop, no sweep, not counted), its cooldown
	// starts as if it had, and RaidFellThrough says so; heat is
	// untouched, so the same rung is there when the cooldown lifts. The
	// arrest is the DA's and no chief stops it. fire rolls nothing on
	// the home stream, so a run without a favour is the run before.
	favour := w.Law.FavourOwed == t.Day
	// A task force announced yesterday comes tonight (#48), whatever the
	// heat: it formed, and it acts. The day's one response is its.
	if h.TaskForceDay > 0 && h.TaskForceDay < t.Day {
		h.TaskForceDay = 0
		if r := s.rung(content.TaskForce); r != nil {
			if favour {
				t.Emit(events.RaidFellThrough{Day: t.Day, City: hot.ID, Level: r.Level, Evidence: s.law.Bribes.FavourEvidence})
			} else {
				h.Responses[r.Level]++
				s.fire(w, t, hot, *r, attempted[hot.ID], fx)
			}
			h.LastResponse[r.Level] = t.Day
			h.WatchUntil = t.Day + s.CooldownDays(w, r.Level)
		}
		resp = nil
	}
	for i := len(resp) - 1; i >= 0; i-- {
		r := resp[i]
		if hot.Heat < s.Threshold(w, r, hot) {
			continue
		}
		if r.Level == content.TaskForce && !s.TaskForceEligible(w) {
			continue // nobody without an asset or the pile meets the feds (#48): the ladder is the four rungs it was
		}
		if last, ok := h.LastResponse[r.Level]; ok && t.Day-last < s.CooldownDays(w, r.Level) && r.Level != content.Arrest {
			continue
		}
		if r.Level == content.TaskForce {
			// Announced a day ahead (#48): the morning's news, and the
			// day's response. It comes tomorrow night.
			h.TaskForceDay = t.Day
			t.Emit(events.TaskForceFormed{Day: t.Day, City: hot.ID, Assets: len(w.Assets)})
			break
		}
		if favour && r.Level != content.Arrest {
			t.Emit(events.RaidFellThrough{Day: t.Day, City: hot.ID, Level: r.Level, Evidence: s.law.Bribes.FavourEvidence})
			h.LastResponse[r.Level] = t.Day
			break
		}
		h.Responses[r.Level]++
		s.fire(w, t, hot, r, attempted[hot.ID], fx)
		h.LastResponse[r.Level] = t.Day
		break
	}

	// A retained lawyer lets the file go cold: a page drops off after
	// enough days without a new one. Only dealing adds pages (#27), so
	// lying low is how a case is left to die.
	if w.Over == nil && fx.EvidenceDecayDays > 0 && h.Evidence > 0 && t.Day-h.EvidenceDay >= fx.EvidenceDecayDays {
		h.Evidence--
		h.EvidenceDay = t.Day
		reasons[here] = append(reasons[here], fmt.Sprintf("the case goes cold (file %d)", h.Evidence))
	}

	// Every sting and raid goes in a file. A thick enough file is a case,
	// and the case runs the exit plans (#49): a fall guy takes it, else
	// a new identity makes it the vanished ending, else it is the end.
	if arrest := s.EvidenceArrest(w); w.Over == nil && arrest > 0 && h.Evidence >= arrest {
		if !s.takeFall(w, t, fx) {
			w.Over = w.End(s.exit(content.CauseIndicted, fx), t.Day, "")
			t.Emit(events.Enforcement{Day: t.Day, City: hot.ID, Level: content.Arrest, StockLost: map[string]int{}})
			t.Emit(events.GameOver{Day: t.Day, Cause: w.Over.Cause})
		}
	}

	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		c.Heat = math.Max(floor, math.Min(100, c.Heat))
		if c.Heat > h.Peak {
			h.Peak = c.Heat
		}
		t.Emit(events.HeatChanged{Day: t.Day, City: cid, From: from[cid], To: c.Heat, Reasons: reasons[cid]})
	}

	// A cop paid today (#45) says what the police here do next: the
	// rung the ladder stands at and the first night it can fire, as the
	// night leaves them, at the cop's accuracy off the intel stream.
	s.cop(w, t)
}

// Hottest is the city whose police answer tonight, for the favour's
// dialog (#228).
func (s *Sim) Hottest(w *game.World) *game.City { return s.hottest(w) }

// hottest is the city whose police answer today: the hottest, and where
// the player is when it is a tie.
func (s *Sim) hottest(w *game.World) *game.City {
	best := w.Here()
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c.Heat > best.Heat {
			best = c
		}
	}
	return best
}

// fire applies one response in a city. attempted says whether the player
// tried to sell there today: a sting or raid that turns up on a day
// nothing moved still costs stock and cash and cools heat, but finds
// nothing worth a file. Dirty cash draws attention; only dealing builds a
// case. The Security branch softens what a response takes; a lawyer thins
// what goes in the file. A sting or a raid hits one place (#73, place):
// a house the police know about, else one house or the street by the
// heat of its block, else the street. A raid while an informant is on
// the payroll goes straight to the fullest house, which they know, and
// takes every unit in it, whatever the safehouse would have saved; with
// no house it is the whole street, as it always was.
func (s *Sim) fire(w *game.World, t *game.Tick, city *game.City, r content.ResponseConfig, attempted bool, fx game.Effects) {
	ev := events.Enforcement{Day: t.Day, City: city.ID, Level: r.Level, StockLost: map[string]int{}}
	switch r.Level {
	case content.Patrol:
		w.Heat.SellCapDays = r.CapDays
		w.Heat.SellCap = s.PatrolCap(w, r, city)
	case content.Arrest:
		if s.takeFall(w, t, fx) {
			return
		}
		w.Over = w.End(s.exit(content.CauseArrested, fx), t.Day, "")
		t.Emit(ev)
		t.Emit(events.GameOver{Day: t.Day, Cause: w.Over.Cause})
		return
	default: // sting, raid, task force
		stockLoss, cashLoss := r.StockLoss, r.CashLoss
		told := r.Level == content.Raid && w.Crew.Informants() > 0
		switch r.Level {
		case content.Raid:
			stockLoss *= fx.RaidLossMul
			cashLoss *= fx.RaidLossMul
			if told {
				stockLoss, ev.Stash = 1, true
			}
		case content.TaskForce:
			// The feds take the file's numbers (#48): no node softens
			// them, and they take an asset besides (seize, below).
		default:
			stockLoss *= fx.StingStockMul
		}
		if house := s.place(w, t, city.ID, told); house != nil {
			ev.House, ev.HouseName = house.ID, house.Name
			for _, id := range sortedProducts(w) {
				if lost := w.TakeFromHouse(house.ID, id, int(math.Round(float64(house.Stock[id])*stockLoss))); lost > 0 {
					ev.StockLost[id] = lost
					w.Stats.HouseUnits += lost
				}
			}
			house.Raided++
			t.Emit(events.HouseRaided{Day: t.Day, House: house.ID, Name: house.Name, City: city.ID, Level: r.Level, Whole: told, StockLost: ev.StockLost})
			if !house.Known && len(ev.StockLost) > 0 {
				house.Known = true
				t.Emit(events.HouseCompromised{Day: t.Day, House: house.ID, Name: house.Name, City: city.ID, Why: "bust"})
			}
		} else {
			for id, q := range w.StreetOf(city.ID) {
				if lost := w.TakeStreet(city.ID, id, int(math.Round(float64(q)*stockLoss))); lost > 0 {
					ev.StockLost[id] = lost
				}
			}
		}
		ev.CashLost = int(math.Round(float64(w.Player.DirtyCash) * cashLoss))
		w.Player.DirtyCash -= ev.CashLost
		switch r.Level {
		case content.Raid:
			w.Stats.Raids++
		case content.TaskForce:
			w.Stats.TaskForces++
			s.seize(w, t, city)
		default:
			w.Stats.Stings++
		}
		// Who stood where when the police came (#46): the corners in
		// the city your crew worked or guarded and the crew on them,
		// stamped for the crew sim to roll the arrests over in the
		// morning (it steps before this one). You are never on the list:
		// your own arrest is the ladder's top rung.
		sweep := game.Sweep{Day: t.Day, City: city.ID, Level: r.Level}
		for _, c := range city.Corners {
			if !c.Held() || (c.Runner <= 0 && c.Enforcer <= 0) {
				continue
			}
			sweep.Corners = append(sweep.Corners, c.ID)
			for _, id := range []int{c.Runner, c.Enforcer} {
				if id > 0 {
					sweep.Crew = append(sweep.Crew, id)
				}
			}
		}
		for _, n := range ev.StockLost {
			sweep.Units += n
		}
		ev.Corners = sweep.Corners
		w.Heat.Sweep = sweep
		// The record the market sim reads the next morning (#72): a
		// bust that took product is held against you by the connect
		// in that city. Kept a month, like the seizures.
		units := 0
		for _, n := range ev.StockLost {
			units += n
		}
		if units > 0 {
			w.Heat.Busts = append(w.Heat.Busts, game.Bust{Day: t.Day, City: city.ID, Level: r.Level, Units: units})
			kept := w.Heat.Busts[:0]
			for _, b := range w.Heat.Busts {
				if t.Day-b.Day <= bustDays {
					kept = append(kept, b)
				}
			}
			w.Heat.Busts = kept
		}
	}
	if attempted {
		ev.Evidence = max(0, r.Evidence-fx.EvidenceCut)
		if ev.Evidence > 0 {
			w.Heat.Evidence += ev.Evidence
			w.Heat.EvidenceDay = t.Day
		}
	}
	// The first raid convinces them they got you. The second one does not.
	// Each repeat of the same response cools things down less.
	n := float64(max(1, w.Heat.Responses[r.Level]))
	city.Heat -= r.HeatDrop / n
	t.Emit(ev)
}

// seize is the task force taking an asset (#48): one a firing, the one
// in the city it came to if any, else the costliest owned; gone, not
// frozen. AssetSeized goes out beside the Enforcement and the
// laundering sim, which owns the assets and steps after this one,
// takes it off the books on the event (as it does the tunnel the
// police found, TunnelFound). Nothing with nothing owned: a task force
// formed on the pile alone takes the stock and the cash a raid does.
// No dice.
func (s *Sim) seize(w *game.World, t *game.Tick, city *game.City) {
	pick := -1
	for i, a := range w.Assets {
		switch {
		case pick < 0:
			pick = i
		case a.City == city.ID && w.Assets[pick].City != city.ID:
			pick = i
		case (a.City == city.ID) == (w.Assets[pick].City == city.ID) && a.Cost > w.Assets[pick].Cost:
			pick = i
		}
	}
	if pick < 0 {
		return
	}
	a := w.Assets[pick]
	t.Emit(events.AssetSeized{Day: t.Day, City: city.ID, Asset: a.ID, Name: a.Name, Cost: a.Cost})
}

// place is the one place a sting or a raid in a city hits (#73): nil for
// the street. An informant on the payroll (told) points the police at
// the fullest house, which becomes Known; then the fullest house they
// know about; then, with houses there holding anything, one roll off the
// houses' side stream over every place holding stock, each house
// weighted by the heat of its block and the street by a standard
// corner's, so a cheap empty house on a quiet block never shields the
// street. With no house holding anything the street it is, no roll: a
// run with no house draws nothing the old run did not.
func (s *Sim) place(w *game.World, t *game.Tick, city string, told bool) *game.House {
	if told {
		if h := w.Fullest(city, false); h != nil && !h.Known {
			h.Known = true
			t.Emit(events.HouseCompromised{Day: t.Day, House: h.ID, Name: h.Name, City: city, Why: "informant"})
		}
	}
	if h := w.Fullest(city, true); h != nil {
		return h
	}
	type candidate struct {
		house  *game.House
		weight float64
	}
	var cands []candidate
	total := 0.0
	for i := range w.Houses {
		h := &w.Houses[i]
		if h.City != city || h.Units() == 0 {
			continue
		}
		weight := s.RaidWeight(w, h)
		cands = append(cands, candidate{h, weight})
		total += weight
	}
	if len(cands) == 0 {
		return nil
	}
	if w.Player.StockIn(city) > 0 {
		cands = append(cands, candidate{nil, 1})
		total++
	}
	if len(cands) == 1 {
		return cands[0].house
	}
	roll := t.Sub("houses").Float64() * total
	for _, c := range cands {
		roll -= c.weight
		if roll < 0 {
			return c.house
		}
	}
	return cands[len(cands)-1].house
}

// placeName names a place a move joined: the house, or the street.
func placeName(w *game.World, id string) string {
	if h := w.House(id); h != nil {
		return h.Name
	}
	return "the street"
}

// sortedProducts is the ladder in a fixed order, for a walk over a
// house's stock whose order is reported.
func sortedProducts(w *game.World) []string {
	ids := append([]string(nil), w.Products...)
	sort.Strings(ids)
	return ids
}

// bustDays is how long a bust stays on the record for the connects.
const bustDays = 30

// pastTense is what the enforcers did, for the heat report.
func pastTense(f events.Force) string {
	switch f {
	case events.ForceWarn:
		return "warned"
	case events.ForceHit:
		return "hit"
	default:
		return "pushed"
	}
}

// takeFall is the fall guy's one job: if the player owns one who has not
// taken his fall (fall_guys is a count, one fall each: World.FallGuyLeft),
// the case that would have ended the run closes on him instead. The file
// is wiped, heat drops to 50 everywhere and half of all cash goes on
// making it stick. It reports whether he took it.
// exit is the cause the case ends the run with once the fall guys are
// spent (#49): the cause the police wrote (indicted, arrested), or
// vanished with a new identity from the tree (upgrades.toml identity,
// fx.Identities), which turns the end into an exit once, since the run
// is over either way. A read on the tree, no dice.
func (s *Sim) exit(cause string, fx game.Effects) string {
	if fx.Identities > 0 {
		return content.CauseVanished
	}
	return cause
}

func (s *Sim) takeFall(w *game.World, t *game.Tick, fx game.Effects) bool {
	if !w.FallGuyLeft(fx) {
		return false
	}
	w.FallsTaken++
	w.Heat.Evidence = 0
	w.Heat.EvidenceDay = t.Day
	for _, c := range w.Cities {
		c.Heat = math.Min(c.Heat, 50)
	}
	lost := w.Player.DirtyCash/2 + w.Player.CleanCash/2
	w.Player.DirtyCash -= w.Player.DirtyCash / 2
	w.Player.CleanCash -= w.Player.CleanCash / 2
	t.Emit(events.FallGuyBurned{Day: t.Day, CashLost: lost})
	return true
}

// Next is the police's next move in a city as the ladder stands (#45):
// the highest rung whose line the city's heat is at or over (the lowest
// rung, the patrol, under every line) and the first day it can fire,
// tomorrow or the day its cooldown lifts (the arrest has none). It is
// the truth a cop's word is right about; the file never reads it.
func (s *Sim) Next(w *game.World, city *game.City, day int) (level string, from int) {
	resp := s.Thresholds()
	if len(resp) == 0 {
		return "", day + 1
	}
	level = resp[0].Level
	for _, r := range resp {
		if city.Heat >= s.Threshold(w, r, city) {
			level = r.Level
		}
	}
	from = day + 1
	if last, ok := w.Heat.LastResponse[level]; ok && level != content.Arrest {
		from = max(from, last+s.CooldownDays(w, level))
	}
	return level, from
}

// Due is the response the police would make tonight on the heat as it
// stands this morning (#228), for the favour: the task force announced
// yesterday, else the highest rung the hottest city meets whose
// cooldown has lifted, as Step reads the ladder. It is "" when nothing
// would fire, when the rung met is the task force's (tonight it is
// announced, not made), the patrol's (a cadence, not worth a call) or
// the arrest's (the DA's, and no chief stops it). The night's decay
// runs before the ladder is read, so it is what the morning says, not a
// promise; the favour is spent on the call either way.
func (s *Sim) Due(w *game.World) string {
	if s.TaskForceForming(w) {
		return content.TaskForce
	}
	hot := s.hottest(w)
	resp := s.Thresholds()
	for i := len(resp) - 1; i >= 0; i-- {
		r := resp[i]
		if hot.Heat < s.Threshold(w, r, hot) {
			continue
		}
		if r.Level == content.TaskForce && !s.TaskForceEligible(w) {
			continue
		}
		if last, ok := w.Heat.LastResponse[r.Level]; ok && w.Day+1-last < s.CooldownDays(w, r.Level) && r.Level != content.Arrest {
			continue
		}
		switch r.Level {
		case content.TaskForce, content.Patrol, content.Arrest:
			return ""
		}
		return r.Level
	}
	return ""
}

// cop files the police's next move where the player stands off a cop
// paid today (World.Today.Cop): right at cop_accuracy for the price
// (less money, less often), else off by a rung or up to cop_slip days,
// rolled on Tick.Sub("intel"). The law sim, stepping after, files the
// chief's temper off the same envelope.
func (s *Sim) cop(w *game.World, t *game.Tick) {
	o := w.Today.Cop
	if o == nil {
		return
	}
	tun := s.intel
	city := w.Here()
	level, from := s.Next(w, city, t.Day)
	p := tun.Accuracy(o.Amount)
	rng := t.Sub("intel")
	if rng.Float64() >= p {
		// A wrong word: the rung beside it, or a few days out.
		resp := s.Thresholds()
		if rng.IntN(2) == 0 && len(resp) > 1 {
			i := content.Rank(level) - 1 // its place on the ladder, 0-based
			switch {
			case i <= 0:
				i = 1
			case i >= len(resp)-1:
				i = len(resp) - 2
			case rng.IntN(2) == 0:
				i++
			default:
				i--
			}
			level = resp[i].Level
		} else {
			from += 1 + rng.IntN(max(1, tun.CopSlip))
		}
	}
	f := game.Fact{
		Subject: city.ID, Kind: game.FactResponse, Value: level, Number: float64(from),
		Confidence: p, Day: t.Day, Source: game.SourceCop, Stale: tun.StaleRate, Forget: tun.Forget,
	}
	w.Learn(f)
	t.Emit(events.IntelGained{Day: t.Day, Subject: city.ID, FactKind: game.FactResponse, Value: level, Confidence: p, Source: game.SourceCop, Name: city.Name})
}
