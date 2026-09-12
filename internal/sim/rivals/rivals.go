// Package rivals simulates the faction competing for the home city's
// corners. One per run: it moves in on a free corner, earns off what it holds,
// claims more by personality, pushes on the player's corners it borders,
// undercuts them there, calls the police after a loss, and resolves the
// player's strikes against it. Violence is abstract: enforcers go in at a
// force and the dice decide. A war that stays loud ends with the police
// clearing both sides.
package rivals

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the rival simulation.
type Sim struct {
	cfg   content.RivalsConfig
	names []string
	rep   content.ReputationFX
	law   content.LawFX
	tree  content.UpgradesConfig
}

// New builds a rival sim from the config, copying what it reads (#144):
// its own rivals.toml and the leader name pool. Of the reputation
// effects it reads one: fear slows its pushes. Of the law's
// (#41) it reads one: a loud city makes its phone calls land. Of the
// upgrade tree (#119) it reads two: the guard on every contested corner
// and the pace of its pushes.
func New(cfg *content.Config) *Sim {
	return &Sim{cfg: cfg.Rivals, names: cfg.Names.Rivals, rep: cfg.Reputation.Effects, law: cfg.Law.Effects, tree: cfg.Upgrades}
}

// Effects is what the owned upgrades do to the rival's fight (#119),
// folded wherever the sim reads the street's numbers, so the odds the
// picker and the map show are the ones the dice use.
func (s *Sim) Effects(w *game.World) game.Effects { return game.FoldEffects(w, s.tree) }

// PushPace is what the player's reputation and the tree do to the
// rival's chance of pushing on a corner today: a feared player is
// pushed on less, and held ground (rival_push_mul) less again.
func (s *Sim) PushPace(w *game.World) float64 {
	return content.Cut(w.Player.Reputation.Fear, s.rep.FearPushCut) * s.Effects(w).RivalPushMul
}

// ClaimPace is what the player's fear does to the rival's chance of
// setting up on a free corner: a feared name is left the city.
func (s *Sim) ClaimPace(w *game.World) float64 {
	return content.Cut(w.Player.Reputation.Fear, s.rep.RivalClaimCut)
}

// ClaimScale is the pace's multiplier on the rival's claim chance (#60):
// claim_scale_min with none of home's corners held by the player,
// claim_scale_max with all of them, in between by the share. A player
// with nothing gets time; one with most of the city gets a fight. Zero
// tuning is a flat pace.
func (s *Sim) ClaimScale(w *game.World) float64 {
	pace := s.cfg.Pace
	if pace.ClaimScaleMin <= 0 && pace.ClaimScaleMax <= 0 {
		return 1
	}
	n := len(s.corners(w))
	if n == 0 {
		return pace.ClaimScaleMin
	}
	share := float64(w.HeldIn(w.Home().ID)) / float64(n)
	return pace.ClaimScaleMin + (pace.ClaimScaleMax-pace.ClaimScaleMin)*share
}

// Rested reports whether the rival may set up on a free corner today
// (#60): its last claim (the day it chose the corner: the tell, #69)
// was claim_cooldown days ago or more, or your enforcers have been in
// within those days, when it grows as fast as it can. Resting, it
// pushes on your corners at push_past_cap like a rival held at its cap:
// a slower spread must not be a longer fight at full pace.
func (s *Sim) Rested(w *game.World, day int) bool {
	r := w.Rival
	cooldown := s.cfg.Pace.ClaimCooldown
	return r.LastClaim == 0 || day-r.LastClaim >= cooldown || (r.LastStruck > 0 && day-r.LastStruck < cooldown)
}

// MaxCorners is the most corners its personality sets up on: max_share
// of home's map, rounded (0.6 of ten is six).
func (s *Sim) MaxCorners(w *game.World) int {
	return int(math.Round(s.personality(w).MaxShare * float64(len(s.corners(w)))))
}

// TipPace is what the home city's public pressure does to the rival's
// chance of turning a grudge into a phone call: a city that wants
// arrests gets its calls answered.
func (s *Sim) TipPace(w *game.World) float64 {
	return content.Scale(w.Home().Pressure, s.law.PressureTip)
}

func (s *Sim) Name() string { return "rivals" }

// Tuning exposes the rival constants the UI needs to explain itself.
func (s *Sim) Tuning() content.RivalsTuning { return s.cfg.Rivals }

// rand is the subset of *math/rand/v2.Rand the sim uses.
type rand interface {
	IntN(int) int
	Float64() float64
}

// Seed picks the run's rival: a leader, a personality and a supplier
// price, all from rng so they are part of the seed. It arrives later.
func (s *Sim) Seed(w *game.World, rng rand) {
	tun := s.cfg.Rivals
	r := &w.Rival
	r.Leader = "Nobody"
	if len(s.names) > 0 {
		r.Leader = s.names[rng.IntN(len(s.names))]
	}
	r.ID = game.FactionRival
	r.Personality = content.Personalities[rng.IntN(len(content.Personalities))]
	r.Supplier = tun.SupplierMin + rng.Float64()*(tun.SupplierMax-tun.SupplierMin)
	r.Cash = s.cost(w, tun.StartCash)
	r.Muscle = tun.StartMuscle
	r.Trust = s.cfg.Personality[r.Personality].Trust
}

// Migrate brings a save from before the rival existed up to date: the
// faction is picked from the current day's RNG and arrives on schedule.
func (s *Sim) Migrate(w *game.World) {
	if w.Rival.Leader == "" {
		s.Seed(w, game.RNGFor(w.Seed, w.Day))
	}
}

func (s *Sim) personality(w *game.World) content.PersonalityConfig {
	return s.cfg.Personality[w.Rival.Personality]
}

// corners is the ground the rival fights over: the home city's.
func (s *Sim) corners(w *game.World) []game.Corner {
	if h := w.Home(); h != nil {
		return h.Corners
	}
	return nil
}

// Strength is the weight the crew's enforcers bring to a strike: each one
// counts 0.5 + skill/100, and one guarding a corner counts half of that,
// they are busy.
func (s *Sim) Strength(w *game.World) float64 {
	n := 0.0
	for _, m := range w.Crew.Members {
		if m.Role != "enforcer" {
			continue
		}
		v := 0.5 + float64(m.Skill)/100
		if w.PostOf(m.ID) != nil {
			v /= 2
		}
		n += v
	}
	return n
}

// frontline is how many rival corners border the player's; the rival's
// muscle stands there.
func (s *Sim) frontline(w *game.World) int {
	n := 0
	for _, c := range s.corners(w) {
		if c.Owner == game.OwnerRival && w.Contested(c) {
			n++
		}
	}
	return n
}

// Defence is the muscle the rival puts on one corner when struck: its
// muscle spread over the front line, times its personality's defence.
func (s *Sim) Defence(w *game.World) float64 {
	return float64(w.Rival.Muscle) / float64(max(1, s.frontline(w))) * s.personality(w).Defence
}

// Odds is the chance a strike at a force takes the corner today. The UI's
// strike picker shows it, so the estimate is what the dice use.
func (s *Sim) Odds(w *game.World, force events.Force) float64 {
	fc := s.cfg.ForceFor(force)
	attack := s.Strength(w) * fc.Attack
	if attack <= 0 {
		return 0
	}
	return fc.Flip * attack / (attack + s.Defence(w))
}

// StrikeHeat is what a strike on a corner at a force draws.
func (s *Sim) StrikeHeat(c *game.Corner, force events.Force) float64 {
	return s.cfg.ForceFor(force).Heat * c.Heat
}

// Guard is the weight of whoever stands on a player corner when it is
// pushed: an enforcer counts 1 + skill/50, you count 1.5, a runner 0.5,
// and the tree's guard_bonus (the front line) is a body on every one,
// so a corner it covers is never walked onto unopposed.
func (s *Sim) Guard(w *game.World, c *game.Corner) float64 {
	g := float64(s.Effects(w).GuardBonus)
	if m := w.Crew.Member(c.Enforcer); m != nil && c.Enforcer != 0 {
		g += 1 + float64(m.Skill)/50
	}
	switch {
	case c.Runner == game.You:
		g += 1.5
	case c.Runner != 0:
		g += 0.5
	}
	return g
}

// PushOdds is the chance one of the rival's pushes flips a player corner:
// its muscle on the front line against whoever is standing there.
func (s *Sim) PushOdds(w *game.World, c *game.Corner) float64 {
	attack := float64(w.Rival.Muscle) / float64(max(1, s.frontline(w)))
	defence := s.Guard(w, c)
	if attack <= 0 {
		return 0
	}
	return s.cfg.Rivals.PushFlip * attack / (attack + defence)
}

// Income is what the rival's corners earn it in a day: margin of the
// street value each moves, less what a price war (#68) is taking off it
// (Corner.Squeeze, which the market sim writes on the rival's corners).
func (s *Sim) Income(w *game.World) int {
	v := 0.0
	for _, c := range s.corners(w) {
		if c.Owner == game.OwnerRival {
			v += s.trade(w, c)
		}
	}
	return int(math.Round(v * s.cfg.Rivals.Margin))
}

// trade is the street value a rival corner moves in a day, squeeze off,
// in the products the street sells there (#139): the port's product
// (no_supply at home) comes by the road, which the rival does not run.
func (s *Sim) trade(w *game.World, c game.Corner) float64 {
	v := 0.0
	for _, id := range w.Products {
		if m := w.Product(c.City, id); m != nil && !m.NoSupply {
			v += m.Demand * c.Share(id) * m.Price
		}
	}
	return v
}

// CornerIncome is what one of the rival's corners earns it in a day
// with no price war on it: what an undercut's share is a share of. The
// undercut picker shows the rival's loss off it.
func (s *Sim) CornerIncome(w *game.World, c game.Corner) int {
	v := 0.0
	for _, id := range w.Products {
		if m := w.Product(c.City, id); m != nil && !m.NoSupply {
			v += m.Demand * c.Full(id) * m.Price
		}
	}
	return int(math.Round(v * s.cfg.Rivals.Margin))
}

// Standard is the street value a standard corner at home moves in a day
// in the products the street there sells (#139): demand per standard
// corner times price, summed over the ladder as unlocked, the port's
// product left out. It is what a corner-day is a margin of.
func (s *Sim) Standard(w *game.World) float64 {
	h := w.Home()
	if h == nil {
		return 0
	}
	v := 0.0
	for _, id := range w.Products {
		if m := h.Market[id]; m != nil && !m.NoSupply {
			v += m.Demand * m.Price
		}
	}
	return v
}

// TributeBase is what a tribute is a cut of (#162): the street value the
// corners the player works at home move in a day in the products the
// rival deals in, the port's product (no_supply at home) left out as
// Income leaves it out of the rival's own take, so a cut of it and the
// take are in one unit. The rival lives at home, so that is the street
// it is talking about. Favour, the opportunist's demand, the propose
// dialog and the rivals pane all read it.
func (s *Sim) TributeBase(w *game.World) float64 {
	h := w.Home()
	if h == nil {
		return 0
	}
	v := 0.0
	for _, id := range w.Products {
		if m := h.Market[id]; m != nil && !m.NoSupply {
			v += w.Demand(h.ID, id) * m.Price
		}
	}
	return v
}

// CornerDay is the unit the rival's money is priced in (#139): what a
// standard corner at home earns it in a day, margin of Standard. Its
// wage, its fee, its claim and the chest it arrives with are so many
// corner-days, so they climb the ladder with its take.
func (s *Sim) CornerDay(w *game.World) float64 { return s.Standard(w) * s.cfg.Rivals.Margin }

// cost is a price in corner-days as today's dollars, rounded.
func (s *Sim) cost(w *game.World, days float64) int {
	return int(math.Round(days * s.CornerDay(w)))
}

// Wage is what one head of the rival's muscle costs it a day: the
// corner's guard (muscle_wage corner-days) over the heads its
// personality puts on a corner, never under a dollar.
func (s *Sim) Wage(w *game.World) int {
	heads := s.personality(w).MusclePerCorner
	if heads <= 0 {
		heads = 1 // no personality yet: a head a corner
	}
	return max(1, s.cost(w, s.cfg.Rivals.MuscleWage/heads))
}

// Wages is the rival's wage bill for the day: its muscle at the wage.
func (s *Sim) Wages(w *game.World) int { return w.Rival.Muscle * s.Wage(w) }

// Fee is what recruiting a head costs it today.
func (s *Sim) Fee(w *game.World) int { return s.cost(w, s.cfg.Rivals.MuscleFee) }

// ClaimCost is what setting up on a free corner costs it today.
func (s *Sim) ClaimCost(w *game.World) int { return s.cost(w, s.cfg.Rivals.ClaimCost) }

// Afford is the muscle the day's take pays for: its income over the
// wage. It recruits no further than it, and a payroll over it runs up
// arrears until a head walks (#139).
func (s *Sim) Afford(w *game.World) int { return s.Income(w) / s.Wage(w) }

// Want is the muscle its personality keeps for the corners it holds:
// muscle_per_corner per corner plus one, rounded, less the heads bought
// off or arrested and not yet back (#70, Rival.Away).
func (s *Sim) Want(w *game.World) int {
	return max(0, int(math.Round(s.personality(w).MusclePerCorner*float64(w.RivalHeld()+1)))-w.Rival.Away)
}

// Pricewar exposes the price war's tuning for the UI and the harness.
func (s *Sim) Pricewar() content.PricewarTuning { return s.cfg.Pricewar }

// Step runs the rival's day: arrival, money, the table (offers taken,
// tribute paid, deals broken), the player's strike, its answer to the
// player's proposal, claims, pushes, undercutting, tips, the war getting
// louder or crushed, the deals kept, and an offer of its own.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	tun := s.cfg.Rivals
	r := &w.Rival
	if r.Leader == "" {
		s.Seed(w, t.RNG)
	}
	pc := s.personality(w)

	// 1. Arrival: the first day from arrive_day with somewhere to stand.
	if r.Arrived == 0 {
		if t.Day < tun.ArriveDay {
			s.undercut(w, t)
			return
		}
		if c := s.pickFree(w, t.RNG, t.Day, true); c != nil {
			s.take(w, c, t.Day)
			r.Arrived = t.Day
			r.Claims++
			t.Emit(events.RivalMovedIn{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
		}
		s.undercut(w, t)
		return
	}

	warBefore := r.War

	// 2. Money (#139): the take, and the muscle it pays for. The wages
	// come out of the chest; what the day's take did not cover of them
	// is owed (Arrears), a surplus day pays the owing down, and once a
	// full wage is owed a head walks, one a day, never one of the
	// start_muscle it came with (those the chest carries). A chest that
	// cannot meet the payroll is the last resort, muscle down to what it
	// holds. Then it recruits up to what it wants and what the take pays
	// for, while the chest covers a fee and a claim besides. No dice:
	// income and expenses are arithmetic.
	income := s.Income(w)
	r.Cash += income
	wage := s.Wage(w)
	wages := r.Muscle * wage
	if wages > r.Cash {
		r.Muscle = r.Cash / wage
		wages = r.Muscle * wage
	}
	r.Cash -= wages
	r.Arrears = math.Max(0, r.Arrears+float64(wages-income))
	if r.Arrears >= float64(wage) && r.Muscle > tun.StartMuscle {
		r.Muscle--
		r.Arrears -= float64(wage)
	}
	fee, claim := s.Fee(w), s.ClaimCost(w)
	for r.Muscle < min(s.Want(w), s.Afford(w)) && r.Cash >= fee+claim {
		r.Cash -= fee
		r.Muscle++
	}

	// 2b. The table: offers lapse, the ones you took are sealed, tribute
	// is paid or missed, a split corner you walked off is noticed.
	betrayed := s.table(w, t)

	// 2c. The heads you paid to go home (#70) leave before the night's
	// fighting, off the books side stream; and of the heads away, one
	// finds its way back every away_days.
	s.poach(w, t)
	s.comeBack(w, t)

	// 3. The player's strike resolves before the rival moves; a push or a
	// hit under a deal is a betrayal of every deal, and a betrayal is
	// paid back with one phone call tonight, whatever else the night
	// brings. Then it answers what you proposed, and a chaotic one may
	// tear something up on a whim.
	if o := w.Strike; o != nil {
		s.strike(w, t, o)
		betrayed = s.crossed(w, t, o) || betrayed
	}
	// Your tip to the police (#70) lands after the enforcers: under a
	// peace it is a betrayal too, and past the notice line the police
	// raid the corner tonight.
	betrayed = s.tip(w, t) || betrayed
	if betrayed {
		r.Tips++
		t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Heat: tun.TipHeat})
	}
	s.answer(w, t)
	s.whim(w, t)

	// 3b. Defectors (last night's, off the crew sim's queue, #144): each
	// one joins its muscle, and walks it onto the corner they ran if
	// nobody stands there; if somebody does, it is a push like any
	// other, with the defector's help counted in. Only a corner of the
	// city it fights over: it never sets up elsewhere.
	for _, l := range w.Crew.Leads {
		r.Muscle++
		r.Observed = true
		c := w.Corner(l.Corner)
		if c == nil || c.City != w.Home().ID || !c.Held() || s.offLimits(w, c) {
			continue
		}
		if s.Guard(w, c) == 0 || t.RNG.Float64() < s.PushOdds(w, c) {
			s.take(w, c, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer, Handed: l.Name})
			continue
		}
		r.War += tun.PushWar
		t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction()})
	}

	// 3c. The price war (#68): a corner the market squeezed today (the
	// player's orders next door took a share of its trade) is a day
	// starved; enough days running and it answers by temper. Business
	// costs a little war and a grudge, never public pressure.
	s.pricewar(w, t)

	// 4. Claims: a free corner, by personality, up to what it wants, at
	// the pace (#60): faster the more of the city you hold, slower the
	// more you are feared, and never twice within claim_cooldown days
	// unless your enforcers have been in within those days, when it
	// grows as fast as it can (the roll is made either way, so the
	// seed's dice stay put; its arrival is not a claim, so the second
	// corner comes as it likes). The claim is telegraphed (#69): the
	// corner it eyed yesterday is resolved first, then today's roll
	// picks the next one and gives the tell. No new dice: the roll is
	// the one made before, its corner taken a day later; and the
	// cooldown runs from the tell, the day it chose, so the pace #60
	// set is the pace it keeps (a claim a day later is not a claim
	// cycle a day longer), and a claim kept off a corner rests it too.
	s.resolveEyeing(w, t)
	if w.RivalHeld() < s.MaxCorners(w) && (r.Routed == 0 || t.Day-r.Routed >= tun.RegroupDays) && t.RNG.Float64() < pc.ClaimChance*s.ClaimScale(w)*s.ClaimPace(w) {
		// The chest is asked after the roll (#139): a rival that cannot
		// pay for a corner today rolls all the same, so the seed's dice
		// do not move when its money binds.
		if s.Rested(w, t.Day) && r.Cash >= s.ClaimCost(w) {
			if c := s.pickFree(w, t.RNG, t.Day, w.RivalHeld() == 0); c != nil {
				r.Eyeing, r.EyeingDay, r.LastClaim = c.ID, t.Day, t.Day
				t.Emit(events.RivalEyeing{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
			}
		}
	}

	// 5. Pushes on the player's corners it borders: at full pace while it
	// wants more ground or has a grudge to pay back, at its personality's
	// pace past that, and slower against a player it fears. Every push is
	// noise; one that lands flips the corner and sends its people home.
	// A deal keeps it off: every corner under a truce or a tribute, your
	// side of the line under a split.
	pace := s.PushPace(w)
	ground := s.corners(w)
	for i := range ground {
		c := &ground[i]
		if !c.Held() || !w.Contested(*c) || r.Muscle == 0 || s.offLimits(w, c) {
			continue
		}
		chance := pc.PushChance * pace
		if (w.RivalHeld() >= s.MaxCorners(w) || !s.Rested(w, t.Day)) && r.Grudge == 0 {
			chance *= pc.PushPastCap // held at its cap, or resting after a claim (#60): the slow pace, not a pause
		}
		if r.Personality == "opportunist" && (c.Enforcer == 0 || w.Home().Heat > 50) {
			chance *= 2
		}
		chance *= 1 + 0.25*float64(min(r.Grudge, 4))
		if t.RNG.Float64() >= chance {
			continue
		}
		r.Observed = true
		r.War += tun.PushWar
		if t.RNG.Float64() < s.PushOdds(w, c) {
			s.take(w, c, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer})
			continue
		}
		if c.Enforcer != 0 && t.RNG.Float64() < 0.5 {
			r.Muscle-- // held off, and it cost them
		}
		t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction()})
	}

	// 6. A grudge is paid back with a phone call, unless there is a peace;
	// a city under pressure listens harder.
	if r.Grudge > 0 && !w.AtPeace() && t.RNG.Float64() < pc.TipChance*s.TipPace(w) {
		r.Grudge--
		r.Tips++
		r.Observed = true
		t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Heat: tun.TipHeat})
	}

	// 6b. The police's attention on it (#70): fading, and up with every
	// push it made tonight (a lead's, a price war's or its own: the
	// events say) while there was any.
	pushes := 0
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.RivalPushed:
			pushes++
		case events.CornerTaken:
			if ev.From == game.OwnerPlayer && ev.Handed == "" {
				pushes++
			}
		}
	}
	s.heat(w, pushes)

	// 7. The war: crossing the line makes headlines; loud enough and the
	// police clear both sides; otherwise it fades a little.
	if warBefore < tun.WarThreshold && r.War >= tun.WarThreshold {
		t.Emit(events.WarEscalated{Day: t.Day, Stage: "open", War: r.War})
	}
	if r.War >= tun.CrackdownThreshold {
		s.crackdown(w, t)
	} else {
		r.War -= r.War * tun.WarDecay
	}

	// 8. Undercutting on whatever is contested after today's moves, then
	// the deals kept and, maybe, one of its own on the table.
	s.undercut(w, t)
	s.keep(w, t)
	s.offer(w, t)
	if !r.Observed && t.Day-r.Arrived >= tun.ObserveDays {
		r.Observed = true
	}
	r.War = math.Max(0, math.Min(100, r.War))
	r.Trust = math.Max(0, math.Min(100, r.Trust))
	if r.Cash < 0 {
		r.Cash = 0
	}
	if r.Muscle < 0 {
		r.Muscle = 0
	}

	// 9. The scout (#70) reads the books as the night leaves them, off
	// the books side stream.
	s.scout(w, t)
}

// resolveEyeing is the claim the rival telegraphed yesterday (#69): it
// sets up on the corner it eyed, claim_cost spent, unless somebody got
// there first, when the claim fails, no cash is spent and it holds a
// grudge (RivalOutbid). A body on the corner is what a Post puts there:
// you, a runner or an enforcer, since a corner with an enforcer on it
// is held and it never walks onto held ground without a push. A corner
// it can no longer set up on for its own reasons (a split now covers
// it, it holds its share, it was routed, the corner is no longer free
// and not yours either, or its chest no longer covers the claim at
// today's prices, #139) is dropped without a word.
func (s *Sim) resolveEyeing(w *game.World, t *game.Tick) {
	tun := s.cfg.Rivals
	r := &w.Rival
	if r.Eyeing == "" {
		return
	}
	id := r.Eyeing
	r.Eyeing, r.EyeingDay = "", 0
	c := w.Corner(id)
	if c == nil || c.City != w.Home().ID || c.Owner == game.OwnerRival {
		return
	}
	if split := w.Deal(game.DealSplit); split != nil && split.Covers(c.ID) {
		return
	}
	if w.RivalHeld() >= s.MaxCorners(w) || (r.Routed > 0 && t.Day-r.Routed < tun.RegroupDays) || r.Cash < s.ClaimCost(w) {
		return
	}
	if c.Held() {
		r.Grudge += tun.OutbidGrudge
		t.Emit(events.RivalOutbid{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name})
		return
	}
	r.Cash -= s.ClaimCost(w)
	r.Claims++
	s.take(w, c, t.Day)
	t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerNone})
}

// Eyeing is the corner the rival sets up on next step, or nil: the tell
// (#69) the map marks and the panels name.
func (s *Sim) Eyeing(w *game.World) *game.Corner {
	if w.Rival.Eyeing == "" {
		return nil
	}
	return w.Corner(w.Rival.Eyeing)
}

// strike resolves the player's enforcers going in on a rival corner, or
// for its takings (#70, boost).
func (s *Sim) strike(w *game.World, t *game.Tick, o *game.StrikeOrder) {
	r := &w.Rival
	c := w.Corner(o.Corner)
	if c == nil || c.Owner != game.OwnerRival || w.Crew.Role("enforcer") == 0 {
		return
	}
	if o.Boost {
		s.boost(w, t, o, c)
		return
	}
	fc := s.cfg.ForceFor(o.Force)
	ev := events.CornerStruck{
		Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), Force: o.Force,
		Heat: s.StrikeHeat(c, o.Force), Toll: fc.Loyalty,
	}
	w.Stats.Strikes++
	r.Observed = true
	r.LastStruck = t.Day
	r.War += fc.War
	r.Trust = math.Max(0, r.Trust-fc.Trust)
	if t.RNG.Float64() < s.Odds(w, o.Force) {
		ev.Taken = true
		c.Owner, c.Faction, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerPlayer, "", 0, 0, 0, 0, t.Day
		c.Starved, c.StarvedDay = 0, 0
		r.Grudge++
		w.Stats.CornersWon++
		if r.Muscle > 0 {
			r.Muscle--
		}
		if w.RivalHeld() == 0 {
			ev.Routed = true
			r.Routed = t.Day
		}
	}
	t.Emit(ev)
}

// take hands a corner to the rival, naming its faction on it (#144),
// sending whoever was on it home.
func (s *Sim) take(w *game.World, c *game.Corner, day int) {
	c.Owner, c.Faction, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerRival, w.Rival.Faction(), 0, 0, 0, 0, day
	c.Starved, c.StarvedDay = 0, 0
}

// pickFree chooses the free corner the rival sets up on. Arriving (or
// starting over) it takes the biggest one that does not border the player
// if there is one, so it grows toward you. After that its personality
// says: the biggest free corner anywhere, one next to its own, or any.
// Arriving, and for arrive_grace days after, a corner you have ever
// worked is not one it sets up on (#60).
func (s *Sim) pickFree(w *game.World, rng rand, day int, arriving bool) *game.Corner {
	var free, quiet, adjacent []*game.Corner
	ground := s.corners(w)
	split := w.Deal(game.DealSplit)
	grace := arriving || (w.Rival.Arrived > 0 && day-w.Rival.Arrived < s.cfg.Pace.ArriveGrace)
	for i := range ground {
		c := &ground[i]
		if c.Owner != game.OwnerNone || (split != nil && split.Covers(c.ID)) || (grace && c.Yours) {
			continue
		}
		free = append(free, c)
		next, own := false, false
		for _, o := range ground {
			if c.Borders(o) {
				next = next || o.Held()
				own = own || o.Owner == game.OwnerRival
			}
		}
		if !next {
			quiet = append(quiet, c)
		}
		if own {
			adjacent = append(adjacent, c)
		}
	}
	if len(free) == 0 {
		return nil
	}
	pool := free
	grow := s.personality(w).Grow
	switch {
	case arriving:
		if len(quiet) > 0 {
			pool = quiet
		}
	case grow == "adjacent":
		if len(adjacent) == 0 {
			return nil
		}
		pool = adjacent
	}
	if grow == "random" {
		return pool[rng.IntN(len(pool))]
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Demand > pool[j].Demand })
	return pool[0]
}

// undercut marks the player's contested corners as squeezed and drags the
// street price by the share of worked demand that is contested. The
// squeeze on the rival's own corners is the market sim's (#68, the
// player's price war) and is left alone.
func (s *Sim) undercut(w *game.World, t *game.Tick) {
	tun := s.cfg.Rivals
	r := &w.Rival
	share := s.personality(w).Undercut
	var names []string
	total, squeezed := 0.0, 0.0
	ground := s.corners(w)
	for i := range ground {
		c := &ground[i]
		if c.Owner == game.OwnerRival {
			continue
		}
		c.Squeeze = 0
		if !c.Held() || !w.Contested(*c) || s.offLimits(w, c) {
			continue
		}
		c.Squeeze = share
		names = append(names, c.Name)
		if c.Worked() {
			squeezed += c.Demand
		}
	}
	for _, c := range ground {
		if c.Worked() {
			total += c.Demand
		}
	}
	if len(names) == 0 {
		return
	}
	if total > 0 && squeezed > 0 {
		connect := (1 - r.Supplier) / math.Max(0.01, 1-tun.SupplierMin) // 1 for the best connect it can have
		drag := tun.UndercutPrice * connect * squeezed / total
		for _, m := range w.Home().Market {
			m.Price *= 1 - drag
		}
	}
	t.Emit(events.RivalUndercut{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corners: names, Share: share})
}

// pricewar is the rival's side of the player's price war (#68). Every
// corner of its the market squeezed today (Corner.Squeeze, the share of
// its trade the player's orders next door took) is a day starved
// (Corner.Starved, StarvedDay); a rest of more than pricewar_days days
// forgets the count, so a war fought every other day still bites, only
// later. An undercut day costs war, once for the day whatever the
// corners. A corner starved pricewar_days days gets an answer by
// personality, and the answer, when it comes, holds a grudge if the
// tuning says (so tip_chance applies: a grudge a day made the price war
// hotter than a hit war, fourteen calls in 120 days): a defensive or an
// expansionist
// rival pushes on the player corner doing the cutting (the biggest
// worked one next door) at push_chance times pricewar_push, the usual
// push odds deciding whether it takes it; an opportunist abandons the
// corner (RivalAbandoned) and sets up elsewhere as the claim step lets
// it; a chaotic one rolls between the two. The count starts over after
// an answer. Nothing here rolls dice on a corner nobody undercut, so a
// run that never does is the old run.
func (s *Sim) pricewar(w *game.World, t *game.Tick) {
	tun := s.cfg.Pricewar
	r := &w.Rival
	ground := s.corners(w)
	starved := false
	for i := range ground {
		c := &ground[i]
		if c.Owner != game.OwnerRival || c.Squeeze <= 0 {
			continue
		}
		if t.Day-c.StarvedDay > tun.PricewarDays {
			c.Starved = 0
		}
		c.Starved++
		c.StarvedDay = t.Day
		starved = true
	}
	if !starved {
		return
	}
	r.Observed = true
	r.War += tun.War
	pc := s.personality(w)
	for i := range ground {
		c := &ground[i]
		if c.Owner != game.OwnerRival || tun.PricewarDays <= 0 || c.Starved < tun.PricewarDays {
			continue
		}
		abandon := false
		switch r.Personality {
		case "opportunist":
			abandon = true
		case "chaotic":
			abandon = t.RNG.Float64() < 0.5
		}
		if abandon {
			if tun.Grudge {
				r.Grudge++
			}
			c.Owner, c.Faction, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerNone, "", 0, 0, 0, 0, t.Day
			c.Starved, c.StarvedDay = 0, 0
			t.Emit(events.RivalAbandoned{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name, Reason: "pricewar"})
			if w.RivalHeld() == 0 && r.Routed < t.Day {
				r.Routed = t.Day
			}
			continue
		}
		// The push: on the biggest corner of yours working it cheap.
		target := s.cutter(w, *c)
		if target == nil || r.Muscle == 0 || s.offLimits(w, target) {
			continue
		}
		if t.RNG.Float64() >= pc.PushChance*tun.PricewarPush*s.PushPace(w) {
			continue
		}
		if tun.Grudge {
			r.Grudge++
		}
		c.Starved = 0
		r.War += s.cfg.Rivals.PushWar
		if t.RNG.Float64() < s.PushOdds(w, target) {
			s.take(w, target, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: target.ID, Name: target.Name, Rival: r.Leader, Faction: r.Faction(), From: game.OwnerPlayer, Pricewar: true})
			continue
		}
		if target.Enforcer != 0 && t.RNG.Float64() < 0.5 {
			r.Muscle--
		}
		t.Emit(events.RivalPushed{Day: t.Day, Corner: target.ID, Name: target.Name, Rival: r.Leader, Faction: r.Faction(), Pricewar: true})
	}
}

// cutter is the player's corner doing the cutting on a rival corner: the
// biggest worked one next door, or nil.
func (s *Sim) cutter(w *game.World, c game.Corner) *game.Corner {
	ground := s.corners(w)
	var best *game.Corner
	for i := range ground {
		o := &ground[i]
		if !o.Worked() || !c.Borders(*o) {
			continue
		}
		if best == nil || o.Demand > best.Demand {
			best = o
		}
	}
	return best
}

// crackdown is the police ending a loud war: each side loses corners,
// contested ones first (decided before anything is cleared, so both
// sides lose their front line), the rival loses muscle and the player
// draws heat.
func (s *Sim) crackdown(w *game.World, t *game.Tick) {
	tun := s.cfg.Rivals
	r := &w.Rival
	ev := events.WarEscalated{Day: t.Day, Stage: "crackdown", War: r.War, Heat: tun.CrackdownHeat}
	var cleared []*game.Corner
	ground := s.corners(w)
	for _, owner := range []string{game.OwnerPlayer, game.OwnerRival} {
		var held []*game.Corner
		for i := range ground {
			if c := &ground[i]; c.Owner == owner {
				held = append(held, c)
			}
		}
		sort.SliceStable(held, func(i, j int) bool {
			ci, cj := w.Contested(*held[i]), w.Contested(*held[j])
			if ci != cj {
				return ci
			}
			return held[i].Since > held[j].Since
		})
		cleared = append(cleared, held[:min(len(held), tun.CrackdownCorners)]...)
	}
	for _, c := range cleared {
		owner, faction := c.Owner, c.Faction
		c.Owner, c.Faction, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerNone, "", 0, 0, 0, 0, t.Day
		c.Starved, c.StarvedDay = 0, 0
		ev.Lost = append(ev.Lost, c.Name)
		t.Emit(events.CornerLost{Day: t.Day, Corner: c.ID, Name: c.Name, Reason: "crackdown", Owner: owner, Faction: faction})
	}
	r.Muscle = int(math.Round(float64(r.Muscle) * (1 - tun.CrackdownMuscle)))
	r.War = 0
	if w.RivalHeld() == 0 && r.Routed < t.Day {
		r.Routed = t.Day
	}
	t.Emit(ev)
}
