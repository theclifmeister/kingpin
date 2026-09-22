package rivals

import (
	"fmt"
	"math"
	"slices"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The table (#43): the one rival became three to six factions. The
// rival at home is the first of them and keeps every roll it made on
// the tick's stream; the rest are seeded and stepped on streams of
// their own, and what the table does to the table (factions pushing on
// each other, absorbing one another, poaching your crew, bowing) rolls
// on streams of its own too, so a run with one faction is the duel
// byte-for-byte. Poaching and homage need a table: with one faction in
// the run nobody poaches and nobody bows.

// seedTable seeds the factions beside the rival at home (#43) off the
// factions stream of day 0: min to max of them by seed, at most one
// chaotic across the table, each with a leader nobody else has, a
// personality, a supplier price and a home city by seed, and the chest
// and the muscle the rival at home arrives with. A table already seated
// is left alone. The rival at home's own draws are made before this and
// nothing here touches the stream they came off.
func (s *Sim) seedTable(w *game.World) {
	f := s.cfg.Factions
	if len(w.Rivals) != 1 || f.Max < 2 {
		return
	}
	rng := game.SubRNG(w.Seed, 0, game.StreamFactions)
	n := f.Min
	if f.Max > f.Min {
		n += rng.IntN(f.Max - f.Min + 1)
	}
	chaotic := w.Rival().Personality == "chaotic"
	away := 0
	var used []string
	for _, r := range w.Rivals {
		used = append(used, r.Leader)
	}
	for i := 2; i <= n; i++ {
		r := &game.RivalState{ID: fmt.Sprintf("f%d", i)}
		var free []string
		for _, name := range s.names {
			if !slices.Contains(used, name) {
				free = append(free, name)
			}
		}
		r.Leader = fmt.Sprintf("Faction %d", i)
		if len(free) > 0 {
			r.Leader = free[rng.IntN(len(free))]
		}
		used = append(used, r.Leader)
		pool := content.Personalities
		if chaotic {
			pool = slices.DeleteFunc(slices.Clone(pool), func(p string) bool { return p == "chaotic" })
		}
		r.Personality = pool[rng.IntN(len(pool))]
		chaotic = chaotic || r.Personality == "chaotic"
		tun := s.cfg.Rivals
		r.Supplier = tun.SupplierMin + rng.Float64()*(tun.SupplierMax-tun.SupplierMin)
		// Where it lives: home, or at away the other city (the roll is
		// made whatever the file says, so the seats do not move when
		// the chance does).
		if roll := rng.Float64(); len(w.CityOrder) > 1 && roll < f.Away && away < f.AwayMax {
			r.Home = w.CityOrder[1+rng.IntN(len(w.CityOrder)-1)]
			away++
		}
		r.Cash = s.cost(w, r, tun.StartCash)
		r.Muscle = tun.StartMuscle
		r.Trust = s.cfg.Personality[r.Personality].Trust
		w.Rivals = append(w.Rivals, r)
	}
}

// MigrateFactions is the 13 -> 14 step (#43): the one rival a save
// carried is seated as the first faction (game.World.SeatRival) and the
// table is seeded beside it off the seed's factions stream, so a save
// picks up the factions the seed would have dealt a fresh run.
func (s *Sim) MigrateFactions(w *game.World) {
	w.SeatRival()
	s.seedTable(w)
}

// table43 is what the table does to the table after every faction has
// stepped: a faction with no corners for absorb_days is absorbed by the
// one that took its last, the richest faction poaches your least loyal
// member, and a faction your enforcers have beaten enough offers
// homage. Each rolls on a stream of its own; none of it happens in a
// duel.
func (s *Sim) table43(w *game.World, t *game.Tick) {
	for _, r := range w.Rivals {
		if r != nil && !r.Gone() {
			s.absorb(w, t, r)
		}
	}
	if len(w.Rivals) < 2 {
		return
	}
	s.poachCrew(w, t)
	for _, r := range w.Rivals {
		if r != nil && r.Alive() {
			s.homage(w, t, r)
		}
	}
}

// absorb is a faction that has stood absorb_days with no corners since
// another faction took its last being swallowed by it (#43): its muscle
// joins the taker, its deals end and its offers lapse, and it steps no
// more. One routed by you or the police is not absorbed: it regroups as
// it always did, so the duel is the duel.
func (s *Sim) absorb(w *game.World, t *game.Tick, r *game.RivalState) {
	if r.Arrived == 0 || r.Routed == 0 || r.LastTakenBy == "" || w.RivalHeldBy(r.Faction()) > 0 || t.Day-r.Routed < s.cfg.Factions.AbsorbDays {
		return
	}
	by := w.Faction(r.LastTakenBy)
	ev := events.RivalAbsorbed{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Muscle: r.Muscle}
	if by != nil && !by.Gone() {
		by.Muscle += r.Muscle
		ev.By, ev.ByFaction = by.Leader, by.Faction()
		r.AbsorbedBy = by.Faction()
	}
	r.Muscle = 0
	r.Absorbed = t.Day
	s.retire(w, t, r)
	w.Stats.Absorbed++
	t.Emit(ev)
}

// retire ends a faction's business at the table: its deals end, its
// offers lapse, its tell is dropped, and whoever stood with it stands
// alone.
func (s *Sim) retire(w *game.World, t *game.Tick, r *game.RivalState) {
	id := r.Faction()
	for _, d := range r.Deals {
		t.Emit(events.DealEnded{Day: t.Day, Rival: r.Leader, Faction: id, Deal: d.Kind})
	}
	r.Deals = nil
	r.Eyeing, r.EyeingDay = "", 0
	w.Offers = slices.DeleteFunc(w.Offers, func(o game.Offer) bool { return o.With() == id })
	for _, o := range w.Rivals {
		if o != nil && (o.Ally == id || o.Against == id) {
			o.Ally, o.Against = "", ""
		}
	}
}

// killed honours the rival_leader_killed incident (#44, #43) the same
// tick: the faction holding most of the incident's city loses its
// leader and fragments as an arrest fragments it. No dice of its own.
func (s *Sim) killed(w *game.World, t *game.Tick) {
	for _, e := range t.Events() {
		ev, ok := e.(events.Incident)
		if !ok || !ev.LeaderKilled {
			continue
		}
		if r := w.StrongestFaction(ev.City); r != nil {
			s.fragment(w, t, r, true)
		}
	}
}

// fragment is a faction losing its leader (#43): arrested on your tips
// (its heat past leader_arrest_heat) or killed by the world. It steps
// no more; its corners drift to the street over fragment_days in an
// order the factions stream draws (drift); the market spikes its city's
// products the morning after (the market sim reads Fragmented); and its
// muscle turn up in your hiring pool at a discount (the crew sim reads
// the event's Muscle).
func (s *Sim) fragment(w *game.World, t *game.Tick, r *game.RivalState, killed bool) {
	if r.Gone() {
		return
	}
	rng := t.Sub(game.StreamFactions)
	var ids []string
	for _, c := range s.corners(w, r) {
		if owns(c, r) {
			ids = append(ids, c.ID)
		}
	}
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	ev := events.RivalLeaderArrested{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), City: s.city(w, r).ID, Corners: len(ids), Muscle: r.Muscle, Killed: killed}
	r.Fragmented = t.Day
	r.Fragments = ids
	r.Muscle = 0
	r.Heat = 0
	r.War = 0
	s.retire(w, t, r)
	w.Stats.Fragmented++
	t.Emit(ev)
}

// drift is a fragmented faction's corners going back to the street
// (#43): what is left of them spread over the days left of
// fragment_days, in the order the leader's fall drew, one a day at
// least. A corner somebody took meanwhile is skipped.
func (s *Sim) drift(w *game.World, t *game.Tick, r *game.RivalState) {
	if r.Fragmented == 0 || len(r.Fragments) == 0 {
		return
	}
	left := max(1, r.Fragmented+s.cfg.Factions.FragmentDays-t.Day)
	n := (len(r.Fragments) + left - 1) / left
	for i := 0; i < n && len(r.Fragments) > 0; i++ {
		id := r.Fragments[0]
		r.Fragments = r.Fragments[1:]
		c := w.Corner(id)
		if c == nil || !owns(*c, r) {
			continue
		}
		c.Hand(game.OwnerNone, "", t.Day)
		t.Emit(events.RivalAbandoned{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Corner: c.ID, Name: c.Name, Reason: "fragmented"})
	}
	if len(r.Fragments) == 0 {
		r.Fragments = nil
	}
}

// contest is a faction pushing on the corners of the other factions it
// borders (#43), on the factions stream: at its push chance (slower at
// its cap or resting, harder with a grudge against the holder), muscle
// against muscle at the push odds, allies lending theirs. Every push
// draws push_heat on the city and the law counts it; one that lands
// hands the corner over, and the loser holds a grudge and trusts the
// winner less. A faction with nothing bordering another rolls nothing.
func (s *Sim) contest(w *game.World, t *game.Tick, r *game.RivalState) {
	f := s.cfg.Factions
	pc := s.personality(r)
	rng := t.Sub(game.StreamFactions)
	ground := s.corners(w, r)
	for i := range ground {
		c := &ground[i]
		if c.Owner != game.OwnerRival || owns(*c, r) || r.Muscle == 0 || !w.ContestedBy(*c, r.Faction()) {
			continue
		}
		v := w.Faction(c.FactionID())
		if v == nil || v.Gone() {
			continue
		}
		chance := pc.PushChance
		if (w.RivalHeldBy(r.Faction()) >= s.MaxCorners(w, r) || !s.Rested(r, t.Day)) && r.Grudges[v.Faction()] == 0 {
			chance *= pc.PushPastCap
		}
		chance *= 1 + 0.25*float64(min(r.Grudges[v.Faction()], 4))
		if rng.Float64() >= chance {
			continue
		}
		r.Observed = true
		s.sideWith(w, r, v.Faction())
		ev := events.FactionPushed{Day: t.Day, City: c.City, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), Against: v.Faction(), AgainstRival: v.Leader, Heat: f.PushHeat}
		if rng.Float64() < s.FactionOdds(w, r, v) {
			ev.Taken = true
			s.take(w, r, c, t.Day)
			r.Flips++
			if v.Grudges == nil {
				v.Grudges = map[string]int{}
			}
			v.Grudges[r.Faction()]++
			s.trust(v, r.Faction(), -f.GrudgeTrust)
			v.LastTakenBy = r.Faction()
			if v.Muscle > 0 {
				v.Muscle--
			}
			if w.RivalHeldBy(v.Faction()) == 0 {
				v.Routed = t.Day
			}
		} else if rng.Float64() < 0.5 {
			r.Muscle-- // held off, and it cost them
		}
		t.Emit(ev)
	}
}

// FactionOdds is the chance a faction's push on another's corner lands:
// its front-line muscle, with its allies', against the holder's
// front-line muscle at its defence.
func (s *Sim) FactionOdds(w *game.World, r, v *game.RivalState) float64 {
	attack := float64(r.Muscle)/float64(max(1, s.frontline(w, r))) + s.AllyMuscle(w, r.Faction(), v.Faction())
	if attack <= 0 {
		return 0
	}
	return s.cfg.Rivals.PushFlip * attack / (attack + s.Defence(w, v))
}

// sideWith is the alliance (#43): when an expansionist pushes on
// somebody, every defensive faction in its city sides with them, its
// trust in them up by ally_trust, and stands against the expansionist.
// victim is a faction id or FactionYou.
func (s *Sim) sideWith(w *game.World, r *game.RivalState, victim string) {
	if r.Personality != "expansionist" {
		return
	}
	for _, d := range w.Rivals {
		if d == nil || d == r || !d.Alive() || d.Personality != "defensive" || s.city(w, d) != s.city(w, r) || d.Faction() == victim {
			continue
		}
		d.Ally, d.Against = victim, r.Faction()
		if victim == game.FactionYou {
			d.Trust = math.Min(100, d.Trust+s.cfg.Factions.AllyTrust)
		} else {
			s.trust(d, victim, s.cfg.Factions.AllyTrust)
		}
	}
}

// trust moves a faction's trust in another by delta, clamped.
func (s *Sim) trust(r *game.RivalState, id string, delta float64) {
	if r.Trusts == nil {
		r.Trusts = map[string]float64{}
	}
	r.Trusts[id] = max(0, min(100, r.TrustIn(id)+delta))
}

// AllyMuscle is the muscle the factions standing with attacker against
// defender lend a push (#43): ally_share of each ally's front-line
// muscle, once its trust in the attacker is at ally_line. attacker is a
// faction id or FactionYou.
func (s *Sim) AllyMuscle(w *game.World, attacker, defender string) float64 {
	f := s.cfg.Factions
	v := 0.0
	for _, d := range w.Rivals {
		if d == nil || !d.Alive() || d.Ally != attacker || d.Against != defender || d.Muscle == 0 {
			continue
		}
		trust := d.TrustIn(attacker)
		if attacker == game.FactionYou {
			trust = d.Trust
		}
		if trust < f.AllyLine {
			continue
		}
		v += float64(d.Muscle) / float64(max(1, s.frontline(w, d))) * f.AllyShare
	}
	return v
}

// Allies lists the factions standing with you (#43), for the screens.
func (s *Sim) Allies(w *game.World) []*game.RivalState {
	var out []*game.RivalState
	for _, d := range w.Rivals {
		if d != nil && d.Alive() && d.Ally == game.FactionYou && d.Trust >= s.cfg.Factions.AllyLine {
			out = append(out, d)
		}
	}
	return out
}

// PoachOffer is what a faction would offer one of your crew a day
// (#43): poach_mul of their wage at the crew's pay dial as the crew sim
// pays it, which the sim cannot see, so the wage on the roster stands
// in.
func (s *Sim) PoachOffer(m game.CrewMember) int {
	return int(math.Round(float64(m.Wage) * s.cfg.Factions.PoachMul))
}

// Poacher is the faction that would poach tonight: the richest alive
// with poach_cash corner-days in the chest, or nil.
func (s *Sim) Poacher(w *game.World) *game.RivalState {
	var best *game.RivalState
	for _, r := range w.Rivals {
		if r == nil || !r.Alive() || r.Cash < s.cost(w, r, s.cfg.Factions.PoachCash) {
			continue
		}
		if best == nil || r.Cash > best.Cash {
			best = r
		}
	}
	return best
}

// poachCrew is a faction with money to spare offering your least loyal
// member better wages (#43), off the poach stream: under poach_line
// they go (CrewPoached; the crew sim drops them and queues the lead the
// faction acts on next step, the defection of #13 with a faction
// named), over it they stay a little less loyal. One offer a night
// across the table, at poach_chance.
func (s *Sim) poachCrew(w *game.World, t *game.Tick) {
	f := s.cfg.Factions
	r := s.Poacher(w)
	if r == nil || len(w.Crew.Members) == 0 || f.PoachChance <= 0 {
		return
	}
	if t.Sub(game.StreamPoach).Float64() >= f.PoachChance {
		return
	}
	pick := -1
	for i, c := range w.Crew.Members {
		if c.Runs() {
			continue // a lieutenant is a table of their own (#31)
		}
		if pick < 0 || c.Loyalty < w.Crew.Members[pick].Loyalty {
			pick = i
		}
	}
	if pick < 0 {
		return
	}
	m := w.Crew.Members[pick]
	ev := events.CrewPoached{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Rival: r.Leader, Faction: r.Faction(), Wages: s.PoachOffer(m)}
	if m.Loyalty >= f.PoachLine {
		ev.Stayed, ev.Dip = true, f.PoachDip
	} else {
		w.Stats.CrewPoached++
	}
	r.Observed = true
	t.Emit(ev)
}

// homage is a faction your enforcers have taken tribute_corners off
// offering to pay you (#43, #32's tribute in reverse): homage_cut of
// its take a day, at homage_chance a night it qualifies, off the homage
// stream, while it is not distrusting you, has no homage running and no
// offer on the table.
func (s *Sim) homage(w *game.World, t *game.Tick, r *game.RivalState) {
	f := s.cfg.Factions
	id := r.Faction()
	if f.TributeCorners <= 0 || r.LostToYou < f.TributeCorners || w.RivalHeldBy(id) == 0 || s.Distrusted(r, t.Day) || w.DealWith(id, game.DealHomage) != nil || s.offering(w, id) {
		return
	}
	if t.Sub(game.StreamHomage).Float64() >= f.HomageChance {
		return
	}
	per := s.round(f.HomageCut * float64(s.Income(w, r)))
	if per <= 0 {
		return
	}
	s.putOffer(w, t, r, game.Deal{Kind: game.DealHomage, Terms: game.Terms{PerDay: per}})
}

// Crowd is the number of factions on the ground in a faction's city,
// itself included, at least one: with shared_pace the table claims at
// the duel's pace between them, each at its chance over the crowd. A
// faction that has not arrived is not on the ground, so the rival at
// home claims at the duel's pace until the second faction moves in.
func (s *Sim) Crowd(w *game.World, r *game.RivalState) int {
	if !s.cfg.Factions.SharedPace {
		return 1
	}
	n := 0
	for _, o := range w.Rivals {
		if o != nil && o.Alive() && s.city(w, o) == s.city(w, r) {
			n++
		}
	}
	return max(1, n)
}

// ArriveDay is the first day a faction may move in: arrive_day for the
// rival at home, arrive_gap days later for each seat after it, so the
// table fills the way the duel did, one arrival at a time.
func (s *Sim) ArriveDay(w *game.World, r *game.RivalState) int {
	i := max(0, w.FactionIndex(r.Faction()))
	return s.cfg.Rivals.ArriveDay + i*s.cfg.Factions.ArriveGap
}

// TableFull reports whether the table holds table_share of a faction's
// city between them (#43): past it no faction sets up on a free corner
// there, and the table grows only by taking from each other or from
// you, so the ground the duel left the player is the ground the table
// leaves. Zero tuning is no cap.
func (s *Sim) TableFull(w *game.World, r *game.RivalState) bool {
	share := s.cfg.Factions.TableShare
	if share <= 0 {
		return false
	}
	city := s.city(w, r)
	if city == nil {
		return false
	}
	held := 0
	for _, c := range city.Corners {
		if c.Owner == game.OwnerRival {
			held++
		}
	}
	return held >= int(math.Round(share*float64(len(city.Corners))))
}
