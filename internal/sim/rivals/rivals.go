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
}

// New builds a rival sim from config and the leader name pool. Of the
// reputation effects it reads one: fear slows its pushes. Of the law's
// (#41) it reads one: a loud city makes its phone calls land.
func New(cfg content.RivalsConfig, names content.NamesConfig, rep content.ReputationFX, law content.LawFX) *Sim {
	return &Sim{cfg: cfg, names: names.Rivals, rep: rep, law: law}
}

// PushPace is what the player's reputation does to the rival's chance
// of pushing on a corner today: a feared player is pushed on less.
func (s *Sim) PushPace(w *game.World) float64 {
	return content.Cut(w.Player.Reputation.Fear, s.rep.FearPushCut)
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
// (#60): its last claim was claim_cooldown days ago or more, or your
// enforcers have been in within those days, when it grows as fast as it
// can. Resting, it pushes on your corners at push_past_cap like a rival
// held at its cap: a slower spread must not be a longer fight at full
// pace.
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
	r.Personality = content.Personalities[rng.IntN(len(content.Personalities))]
	r.Supplier = tun.SupplierMin + rng.Float64()*(tun.SupplierMax-tun.SupplierMin)
	r.Cash = tun.StartCash
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
// pushed: an enforcer counts 1 + skill/50, you count 1.5, a runner 0.5.
func (s *Sim) Guard(w *game.World, c *game.Corner) float64 {
	g := 0.0
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

// Income is what the rival's corners earn it in a day.
func (s *Sim) Income(w *game.World) int {
	v := 0.0
	for _, c := range s.corners(w) {
		if c.Owner != game.OwnerRival {
			continue
		}
		for _, id := range w.Products {
			if m := w.Product(c.City, id); m != nil {
				v += m.Demand * c.Share(id) * m.Price
			}
		}
	}
	return int(math.Round(v * s.cfg.Rivals.Margin))
}

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
			s.take(c, t.Day)
			r.Arrived = t.Day
			r.Claims++
			t.Emit(events.RivalMovedIn{Day: t.Day, Rival: r.Leader, Corner: c.ID, Name: c.Name})
		}
		s.undercut(w, t)
		return
	}

	warBefore := r.War

	// 2. Money: income, wages, recruiting. Muscle it cannot pay walks.
	r.Cash += s.Income(w)
	wages := r.Muscle * tun.MuscleWage
	if wages > r.Cash {
		r.Muscle = r.Cash / max(1, tun.MuscleWage)
		wages = r.Muscle * tun.MuscleWage
	}
	r.Cash -= wages
	want := int(math.Round(pc.MusclePerCorner * float64(w.RivalHeld()+1)))
	for r.Muscle < want && r.Cash >= tun.MuscleFee+tun.ClaimCost {
		r.Cash -= tun.MuscleFee
		r.Muscle++
	}

	// 2b. The table: offers lapse, the ones you took are sealed, tribute
	// is paid or missed, a split corner you walked off is noticed.
	betrayed := s.table(w, t)

	// 3. The player's strike resolves before the rival moves; a push or a
	// hit under a deal is a betrayal of every deal, and a betrayal is
	// paid back with one phone call tonight, whatever else the night
	// brings. Then it answers what you proposed, and a chaotic one may
	// tear something up on a whim.
	if o := w.Strike; o != nil {
		s.strike(w, t, o)
		betrayed = s.crossed(w, t, o) || betrayed
	}
	if betrayed {
		r.Tips++
		t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Heat: tun.TipHeat})
	}
	s.answer(w, t)
	s.whim(w, t)

	// 3b. Defectors: each one joins its muscle, and walks it onto the
	// corner they ran if nobody stands there; if somebody does, it is a
	// push like any other, with the defector's help counted in. Only a
	// corner of the city it fights over: it never sets up elsewhere.
	for _, l := range r.Leads {
		r.Muscle++
		r.Observed = true
		c := w.Corner(l.Corner)
		if c == nil || c.City != w.Home().ID || !c.Held() || s.offLimits(w, c) {
			continue
		}
		if s.Guard(w, c) == 0 || t.RNG.Float64() < s.PushOdds(w, c) {
			s.take(c, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, From: game.OwnerPlayer, Handed: l.Name})
			continue
		}
		r.War += tun.PushWar
		t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader})
	}
	r.Leads = nil

	// 4. Claims: a free corner, by personality, up to what it wants, at
	// the pace (#60): faster the more of the city you hold, slower the
	// more you are feared, and never twice within claim_cooldown days
	// unless your enforcers have been in within those days, when it
	// grows as fast as it can (the roll is made either way, so the
	// seed's dice stay put; its arrival is not a claim, so the second
	// corner comes as it likes).
	if w.RivalHeld() < s.MaxCorners(w) && r.Cash >= tun.ClaimCost && (r.Routed == 0 || t.Day-r.Routed >= tun.RegroupDays) && t.RNG.Float64() < pc.ClaimChance*s.ClaimScale(w)*s.ClaimPace(w) {
		if s.Rested(w, t.Day) {
			if c := s.pickFree(w, t.RNG, t.Day, w.RivalHeld() == 0); c != nil {
				r.Cash -= tun.ClaimCost
				r.Claims++
				r.LastClaim = t.Day
				s.take(c, t.Day)
				t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, From: game.OwnerNone})
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
			s.take(c, t.Day)
			r.Flips++
			r.LastFlip = t.Day
			w.Stats.CornersLost++
			t.Emit(events.CornerTaken{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, From: game.OwnerPlayer})
			continue
		}
		if c.Enforcer != 0 && t.RNG.Float64() < 0.5 {
			r.Muscle-- // held off, and it cost them
		}
		t.Emit(events.RivalPushed{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader})
	}

	// 6. A grudge is paid back with a phone call, unless there is a peace;
	// a city under pressure listens harder.
	if r.Grudge > 0 && !w.AtPeace() && t.RNG.Float64() < pc.TipChance*s.TipPace(w) {
		r.Grudge--
		r.Tips++
		r.Observed = true
		t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Heat: tun.TipHeat})
	}

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
}

// strike resolves the player's enforcers going in on a rival corner.
func (s *Sim) strike(w *game.World, t *game.Tick, o *game.StrikeOrder) {
	r := &w.Rival
	c := w.Corner(o.Corner)
	if c == nil || c.Owner != game.OwnerRival || w.Crew.Role("enforcer") == 0 {
		return
	}
	fc := s.cfg.ForceFor(o.Force)
	ev := events.CornerStruck{
		Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Force: o.Force,
		Heat: s.StrikeHeat(c, o.Force), Toll: fc.Loyalty,
	}
	w.Stats.Strikes++
	r.Observed = true
	r.LastStruck = t.Day
	r.War += fc.War
	r.Trust = math.Max(0, r.Trust-fc.Trust)
	if t.RNG.Float64() < s.Odds(w, o.Force) {
		ev.Taken = true
		c.Owner, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerPlayer, 0, 0, 0, 0, t.Day
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

// take hands a corner to the rival, sending whoever was on it home.
func (s *Sim) take(c *game.Corner, day int) {
	c.Owner, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerRival, 0, 0, 0, 0, day
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
// street price by the share of worked demand that is contested.
func (s *Sim) undercut(w *game.World, t *game.Tick) {
	tun := s.cfg.Rivals
	r := &w.Rival
	share := s.personality(w).Undercut
	var names []string
	total, squeezed := 0.0, 0.0
	ground := s.corners(w)
	for i := range ground {
		c := &ground[i]
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
	t.Emit(events.RivalUndercut{Day: t.Day, Rival: r.Leader, Corners: names, Share: share})
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
		owner := c.Owner
		c.Owner, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerNone, 0, 0, 0, 0, t.Day
		ev.Lost = append(ev.Lost, c.Name)
		t.Emit(events.CornerLost{Day: t.Day, Corner: c.ID, Name: c.Name, Reason: "crackdown", Owner: owner})
	}
	r.Muscle = int(math.Round(float64(r.Muscle) * (1 - tun.CrackdownMuscle)))
	r.War = 0
	if w.RivalHeld() == 0 && r.Routed < t.Day {
		r.Routed = t.Day
	}
	t.Emit(ev)
}
