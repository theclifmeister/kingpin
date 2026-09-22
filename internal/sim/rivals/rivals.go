// Package rivals simulates the factions competing for the cities'
// corners: three to six per run (#43), the first of them the rival at
// home. Each moves in on a free corner, earns off what it holds, claims
// more by personality, pushes on the corners of yours and of the other
// factions it borders, undercuts you there, calls the police after a
// loss, and resolves the player's strikes against it. Violence is
// abstract: enforcers go in at a force and the dice decide. A war that
// stays loud ends with the police clearing both sides.
package rivals

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Sim is the rival simulation.
type Sim struct {
	cfg        content.RivalsConfig
	names      []string
	rep        content.ReputationFX
	law        content.LawFX
	tree       content.UpgradesConfig
	deed       content.DeedTuning
	intel      content.IntelTuning // #45: what a push shows, the feed
	routes     []string            // the route ids, in file order, for a lie about a road (#45)
	routeNames map[string]string
	routeAsset map[string]string // the asset a route needs to be open (#48), "" for none
}

// New builds a rival sim from the config, copying what it reads (#144):
// its own rivals.toml and the leader name pool. Of the reputation
// effects it reads one: fear slows its pushes. Of the law's
// (#41) it reads one: a loud city makes its phone calls land. Of the
// upgrade tree (#119) it reads two: the guard on every contested corner
// and the pace of its pushes. Of the deeds (#194, city.toml [deed]) it
// reads one: push_mul, on its push at a block of yours and on its
// defence of a block that is yours.
func New(cfg *content.Config) *Sim {
	s := &Sim{cfg: cfg.Rivals, names: cfg.Names.Rivals, rep: cfg.Reputation.Effects, law: cfg.Law.Effects, tree: cfg.Upgrades, deed: cfg.City.Deed, intel: cfg.Intel.Intel, routeNames: map[string]string{}, routeAsset: map[string]string{}}
	for _, r := range cfg.Routes.Routes {
		s.routes = append(s.routes, r.ID)
		s.routeNames[r.ID] = r.Name
		s.routeAsset[r.ID] = r.Asset
	}
	return s
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

// DeedMul is what the deed to a block does to the rival's fight for it
// (#194): push_mul on a corner whose block is yours, 1 on any other. It
// is never zero (content.DeedTuning.validate): a deed slows the rival
// and never stops it, the Street branch's rule.
func (s *Sim) DeedMul(c *game.Corner) float64 {
	if c == nil || c.Deed == nil || !s.deed.On() {
		return 1
	}
	return s.deed.PushMul
}

// PushPaceOn is PushPace on one corner of yours: the deed to its block
// (#194) slows the rival's push on it by push_mul.
func (s *Sim) PushPaceOn(w *game.World, c *game.Corner) float64 {
	return s.PushPace(w) * s.DeedMul(c)
}

// ClaimPace is what the player's fear does to the rival's chance of
// setting up on a free corner: a feared name is left the city.
func (s *Sim) ClaimPace(w *game.World) float64 {
	return content.Cut(w.Player.Reputation.Fear, s.rep.RivalClaimCut)
}

// ClaimScale is the pace's multiplier on a faction's claim chance (#60):
// claim_scale_min with none of its city's corners held by the player,
// claim_scale_max with all of them, in between by the share. A player
// with nothing gets time; one with most of the city gets a fight. Zero
// tuning is a flat pace.
func (s *Sim) ClaimScale(w *game.World, r *game.RivalState) float64 {
	pace := s.cfg.Pace
	if pace.ClaimScaleMin <= 0 && pace.ClaimScaleMax <= 0 {
		return 1
	}
	n := len(s.corners(w, r))
	if n == 0 {
		return pace.ClaimScaleMin
	}
	share := float64(w.HeldIn(s.city(w, r).ID)) / float64(n)
	return pace.ClaimScaleMin + (pace.ClaimScaleMax-pace.ClaimScaleMin)*share
}

// Rested reports whether a faction may set up on a free corner today
// (#60): its last claim (the day it chose the corner: the tell, #69)
// was claim_cooldown days ago or more, or your enforcers have been in
// within those days, when it grows as fast as it can. Resting, it
// pushes on your corners at push_past_cap like a rival held at its cap:
// a slower spread must not be a longer fight at full pace.
func (s *Sim) Rested(r *game.RivalState, day int) bool {
	cooldown := s.cfg.Pace.ClaimCooldown
	return r.LastClaim == 0 || day-r.LastClaim >= cooldown || (r.LastStruck > 0 && day-r.LastStruck < cooldown)
}

// MaxCorners is the most corners its personality sets up on: max_share
// of its city's map, rounded (0.6 of ten is six).
func (s *Sim) MaxCorners(w *game.World, r *game.RivalState) int {
	return int(math.Round(s.personality(r).MaxShare * float64(len(s.corners(w, r)))))
}

// TipPace is what a faction's city's public pressure does to its
// chance of turning a grudge into a phone call: a city that wants
// arrests gets its calls answered.
func (s *Sim) TipPace(w *game.World, r *game.RivalState) float64 {
	return content.Scale(s.city(w, r).Pressure, s.law.PressureTip)
}

func (s *Sim) Name() string { return "rivals" }

// Tuning exposes the rival constants the UI needs to explain itself.
func (s *Sim) Tuning() content.RivalsTuning { return s.cfg.Rivals }

// Factions exposes the table's tuning (#43) for the UI and the harness.
func (s *Sim) Factions() content.FactionsTuning { return s.cfg.Factions }

// rand is the subset of *math/rand/v2.Rand the sim uses.
type rand interface {
	IntN(int) int
	Float64() float64
}

// Seed picks the run's rival at home: a leader, a personality and a
// supplier price, all from rng so they are part of the seed. It arrives
// later. The rest of the table (#43) is seeded beside it off the
// factions' own stream (seedTable), so the rival at home is the rival
// the seed always drew.
func (s *Sim) Seed(w *game.World, rng rand) {
	tun := s.cfg.Rivals
	r := w.Rival()
	r.Leader = "Nobody"
	if len(s.names) > 0 {
		r.Leader = s.names[rng.IntN(len(s.names))]
	}
	r.ID = game.FactionRival
	r.Personality = content.Personalities[rng.IntN(len(content.Personalities))]
	r.Supplier = tun.SupplierMin + rng.Float64()*(tun.SupplierMax-tun.SupplierMin)
	r.Cash = s.cost(w, r, tun.StartCash)
	r.Muscle = tun.StartMuscle
	r.Trust = s.cfg.Personality[r.Personality].Trust
	s.seedTable(w)
}

// Migrate brings a save from before the rival existed up to date: the
// faction is picked from the current day's RNG and arrives on schedule.
func (s *Sim) Migrate(w *game.World) {
	if w.Rival().Leader == "" {
		s.Seed(w, game.RNGFor(w.Seed, w.Day))
	}
}

func (s *Sim) personality(r *game.RivalState) content.PersonalityConfig {
	return s.cfg.Personality[r.Personality]
}

// city is the city a faction lives in: its home, or the home city for
// the rival at home.
func (s *Sim) city(w *game.World, r *game.RivalState) *game.City { return w.CityOf(r) }

// corners is the ground a faction fights over: its city's.
func (s *Sim) corners(w *game.World, r *game.RivalState) []game.Corner {
	if c := s.city(w, r); c != nil {
		return c.Corners
	}
	return nil
}

// owns reports whether a corner is the faction's.
func owns(c game.Corner, r *game.RivalState) bool { return c.FactionID() == r.Faction() }

// Step runs the table's day (#43): every faction in id order, the rival
// at home on the tick's stream as it always was and the rest each on
// its own (Tick.Sub("faction:" + id)), then what the table does to the
// table: absorption, the leader an incident killed, poaching and
// homage, on streams of their own. A run with one faction makes the
// rolls the duel made and no other.
func (s *Sim) Step(w *game.World, t *game.Tick) {
	// The squeeze on the player's corners is written afresh every step,
	// once, before any faction writes its own (undercut).
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			if cs[i].Owner != game.OwnerRival {
				cs[i].Squeeze = 0
			}
		}
	}
	s.killed(w, t)
	for i, r := range w.Rivals {
		if r == nil {
			continue
		}
		if r.Gone() {
			s.drift(w, t, r)
			continue
		}
		var rng rand = t.RNG
		if i > 0 {
			rng = t.Sub(game.StreamFactionOf + r.Faction())
		}
		s.step(w, t, r, rng)
	}
	s.table43(w, t)
	// Intel (#45): what the night showed you of the factions, no dice,
	// and what the ones that distrust you feed you, off the intel stream.
	s.observe(w, t)
	for _, r := range w.Rivals {
		if r != nil {
			s.feed(w, t, r)
		}
	}
	s.war(w, t)
	s.endings(w, t)
}

// step runs one faction's day: arrival, money, the table (offers taken,
// tribute paid, deals broken), the player's strike, its answer to the
// player's proposal, claims, pushes, undercutting, tips, the war getting
// louder or crushed, the deals kept, and an offer of its own. Every
// numbered phase is a method of its own (#275), called here in the order
// the dice always ran in, so the function reads as the night's agenda:
// the money in economy.go, the strike and the pushes in strike.go, the
// claim in claim.go, the price war in pricewar.go, the war in war.go.
func (s *Sim) step(w *game.World, t *game.Tick, r *game.RivalState, rng rand) {
	tun := s.cfg.Rivals
	if r.Leader == "" {
		s.Seed(w, t.RNG)
	}
	pc := s.personality(r)

	// 1. Arrival: the first day from arrive_day with somewhere to stand.
	if r.Arrived == 0 {
		s.arrive(w, t, r, rng)
		return
	}

	warBefore := r.War

	// 2. Money (#139): the take, and the muscle it pays for.
	s.payroll(w, r)

	// 2b. The table: offers lapse, the ones you took are sealed, tribute
	// is paid or missed, a split corner you walked off is noticed.
	betrayed := s.table(w, t, r)

	// 2c. The heads you paid to go home (#70) leave before the night's
	// fighting, off the books side stream; and of the heads away, one
	// finds its way back every away_days.
	s.poach(w, t, r)
	s.comeBack(w, t, r)

	// 3. The player's strike resolves before the rival moves; a push or a
	// hit under a deal is a betrayal of every deal, and a betrayal is
	// paid back with one phone call tonight, whatever else the night
	// brings. Then it answers what you proposed, and a chaotic one may
	// tear something up on a whim.
	betrayed = s.struck(w, t, r, rng) || betrayed
	// Your tip to the police (#70) lands after the enforcers: under a
	// peace it is a betrayal too, and past the notice line the police
	// raid the corner tonight.
	betrayed = s.tip(w, t, r) || betrayed
	if betrayed {
		r.Tips++
		t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Heat: tun.TipHeat})
	}
	if r.Gone() {
		return // the leader went in the van tonight (tip): nothing more moves
	}
	s.answer(w, t, r, rng)
	s.whim(w, t, r, rng)

	// 3b. Defectors (last night's, off the crew sim's queue, #144).
	s.defectors(w, t, r, rng)

	// 3c. The price war (#68): a corner the market squeezed today (the
	// player's orders next door took a share of its trade) is a day
	// starved; enough days running and it answers by temper. Business
	// costs a little war and a grudge, never public pressure.
	s.pricewar(w, t, r, rng)

	// 4. Claims: the corner eyed yesterday (#69) first, then today's
	// roll for the next one.
	s.resolveEyeing(w, t, r)
	s.claim(w, t, r, rng, pc)

	// 5. Pushes on the player's corners it borders.
	s.push(w, t, r, rng, pc)

	// 5b. Pushes on the other factions' corners it borders (#43), on the
	// table's own stream: none in a duel, so the duel's dice stand.
	s.contest(w, t, r)

	// 6. A grudge is paid back with a phone call, unless there is a peace.
	s.grudge(w, t, r, rng, pc)

	// 6b. The police's attention on it (#70): fading, and up with every
	// push it made tonight while there was any.
	s.heat(r, s.pushes(t, r))

	// 7. The war: crossing the line makes headlines; loud enough and the
	// police clear both sides; otherwise it fades a little.
	s.escalate(w, t, r, warBefore)

	// 8. Undercutting on whatever is contested after today's moves, then
	// the deals kept and, maybe, one of its own on the table.
	s.undercut(w, t, r)
	s.keep(w, t, r)
	s.offer(w, t, r, rng)
	s.settle(t, r)

	// 9. The scout (#70) reads the books as the night leaves them, off
	// the books side stream.
	s.scout(w, t, r)
}

// grudge is a grudge paid back with a phone call (step 6), unless there
// is a peace; a city under pressure listens harder. The roll is made
// only with a grudge held and no peace, as it always was.
func (s *Sim) grudge(w *game.World, t *game.Tick, r *game.RivalState, rng rand, pc content.PersonalityConfig) {
	if r.Grudge > 0 && !w.AtPeaceWith(r.Faction()) && rng.Float64() < pc.TipChance*s.TipPace(w, r) {
		r.Grudge--
		r.Tips++
		r.Observed = true
		t.Emit(events.RivalTippedPolice{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Heat: s.cfg.Rivals.TipHeat})
	}
}

// pushes counts the pushes a faction made tonight (step 6b), a lead's, a
// price war's or its own, off the tick's events: every RivalPushed of
// its, and every corner it took off you but a defector's hand-over.
func (s *Sim) pushes(t *game.Tick, r *game.RivalState) int {
	n := 0
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.RivalPushed:
			if ev.Faction == r.Faction() {
				n++
			}
		case events.CornerTaken:
			if ev.Faction == r.Faction() && ev.From == game.OwnerPlayer && ev.Handed == "" {
				n++
			}
		}
	}
	return n
}

// settle closes a faction's night (step 8): observed once it has been
// in the city observe_days, and its war, trust, chest and muscle held
// to their ranges.
func (s *Sim) settle(t *game.Tick, r *game.RivalState) {
	if !r.Observed && t.Day-r.Arrived >= s.cfg.Rivals.ObserveDays {
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
