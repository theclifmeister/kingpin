package rivals

import (
	"fmt"
	"math"
	"slices"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Diplomacy (#32) is the table: deals are state on the rival, proposed by
// either side, answered and kept or broken inside the rival's step. The
// rival's answer is a chance built from what it thinks of the deal, the
// terms, its trust in the player, its personality, the war and the
// player's standing; the dice are the tick's. Since #43 every faction
// has a table of its own and a betrayal of one is remembered by all
// (betrayal_spread).

// MigrateDiplomacy brings a save from before the table up to date: the
// rival's trust starts where its personality puts it.
func (s *Sim) MigrateDiplomacy(w *game.World) {
	r := w.Rival()
	if r.Leader != "" && r.Trust == 0 {
		r.Trust = s.personality(r).Trust
	}
}

// Diplomacy exposes the table's constants for the UI.
func (s *Sim) Diplomacy() content.DiplomacyTuning { return s.cfg.Diplomacy }

// Distrusted reports whether a faction is still refusing everything
// after a betrayal, as of the tick of day.
func (s *Sim) Distrusted(r *game.RivalState, day int) bool {
	return r.Betrayed > 0 && day-r.Betrayed < s.cfg.Diplomacy.DistrustDays
}

// Favour is how the terms of a deal sit with the faction, -1 (the
// hardest ask) to 1 (the easiest): a short truce, a fat tribute, a
// modest line. A split that asks for more of the city than the
// faction's trust allows, or for a corner it holds, is refused
// outright: Favour returns -inf.
func (s *Sim) Favour(w *game.World, r *game.RivalState, d game.Deal) float64 {
	dip := s.cfg.Diplomacy
	ramp := func(v, lo, mid, hi float64) float64 {
		// 1 at lo, 0 at mid, -1 at hi, clamped.
		if v <= mid {
			if mid == lo {
				return 1
			}
			return math.Min(1, (mid-v)/(mid-lo))
		}
		if hi == mid {
			return -1
		}
		return math.Max(-1, (mid-v)/(hi-mid))
	}
	switch d.Kind {
	case game.DealTruce:
		days := dip.TruceDays
		return ramp(float64(d.Terms.Days), float64(days[0]), float64(days[1]), float64(days[2]))
	case game.DealTribute:
		cuts := dip.TributeCuts
		cut := 0.0
		if v := s.TributeBase(w, r); v > 0 {
			cut = float64(d.Terms.PerDay) / v
		}
		// A fat cut is the easy ask: the ramp runs the other way.
		return -ramp(cut, cuts[0], cuts[1], cuts[2])
	case game.DealSplit:
		total, ask := 0.0, 0.0
		for _, c := range s.corners(w, r) {
			total += c.Demand
			if slices.Contains(d.Terms.Corners, c.ID) {
				if c.Owner == game.OwnerRival {
					return math.Inf(-1)
				}
				ask += c.Demand
			}
		}
		fair := dip.SplitFair + dip.SplitTrust*r.Trust/100
		if total > 0 {
			ask /= total
		}
		if ask > fair {
			return math.Inf(-1)
		}
		return max(-1, min(1, (fair-ask)*dip.SplitTerms))
	}
	return math.Inf(-1)
}

// Chance is the chance a faction accepts a deal put to it tonight. The
// UI's propose dialog shows it, so the estimate is what the dice use.
func (s *Sim) Chance(w *game.World, r *game.RivalState, d game.Deal) float64 {
	dip := s.cfg.Diplomacy
	if !r.Alive() || s.Distrusted(r, w.Day+1) {
		return 0
	}
	favour := s.Favour(w, r, d)
	if math.IsInf(favour, -1) {
		return 0
	}
	if o := w.Today.Strike; o != nil && o.Force != events.ForceWarn {
		if c := w.Corner(o.Corner); c != nil && owns(*c, r) {
			return 0 // you sent the enforcers in the same night
		}
	}
	dc := s.cfg.Deal[d.Kind]
	p := dc.Base + dc.Terms*favour +
		dip.AcceptTrust*r.Trust/100 +
		s.personality(r).DealBias +
		dip.AcceptWar*r.War/100 +
		s.rep.FearDeal*max(0, min(1, w.Player.Reputation.Fear/100))
	return max(0, min(1, p))
}

// seal makes a deal live from tonight, on exactly the terms given.
func (s *Sim) seal(w *game.World, t *game.Tick, r *game.RivalState, d game.Deal) {
	d.Since = t.Day
	d.Until = 0
	if r != w.Rival() {
		d.Faction = r.Faction() // the rival at home's deals read as they always did
	}
	if d.Kind == game.DealTruce {
		d.Until = t.Day + d.Terms.Days
	}
	r.Deals = append(r.Deals, d)
	r.Observed = true
	w.Stats.Deals++
	t.Emit(events.DealAccepted{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Deal: d.Kind, Terms: w.Describe(d), Until: d.Until, Offered: d.Offered})
}

// betray is the player breaking a deal: it is gone, trust falls to the
// floor and the faction takes nothing for a while. The phone call is
// made once per night, by the caller. Every other faction remembers it
// (#43, betrayal_spread): their trust in you falls by the spread the
// same step, once a night however many deals broke.
func (s *Sim) betray(w *game.World, t *game.Tick, r *game.RivalState, d game.Deal, why string) {
	r.Deals = slices.DeleteFunc(r.Deals, func(x game.Deal) bool { return x.Kind == d.Kind })
	r.Trust = math.Min(r.Trust, s.cfg.Diplomacy.BetrayalFloor)
	r.Betrayed = t.Day
	r.Observed = true
	w.Stats.Betrayals++
	t.Emit(events.DealBroken{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Deal: d.Kind, By: "you", Why: why})
	s.spread(w, t, r)
}

// spread is a betrayal remembered by the table (#43): every other
// faction alive loses betrayal_spread of its trust in you, once a step
// per faction betrayed.
func (s *Sim) spread(w *game.World, t *game.Tick, r *game.RivalState) {
	spread := s.cfg.Diplomacy.BetrayalSpread
	if spread <= 0 {
		return
	}
	for _, e := range t.Events() {
		if ev, ok := e.(events.TrustSpread); ok && ev.Faction == r.Faction() {
			return
		}
	}
	n := 0
	for _, o := range w.Rivals {
		if o == nil || o == r || o.Gone() || o.Leader == "" {
			continue
		}
		o.Trust = math.Max(0, o.Trust-spread)
		n++
	}
	if n > 0 {
		t.Emit(events.TrustSpread{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Spread: spread, Others: n})
	}
}

// table runs the morning's business: a routed faction's deals end,
// offers lapse, the offers the player took are sealed, tribute is paid
// or missed, a homage is paid or given up, a split corner given up is
// noticed. It returns whether the player betrayed anything, for the
// strike to add to.
func (s *Sim) table(w *game.World, t *game.Tick, r *game.RivalState) bool {
	id := r.Faction()
	// A faction run out of town has nothing to deal about: what stood
	// ends.
	if w.RivalHeldBy(id) == 0 {
		for _, d := range r.Deals {
			t.Emit(events.DealEnded{Day: t.Day, Rival: r.Leader, Faction: id, Deal: d.Kind})
		}
		r.Deals = nil
		w.Offers = slices.DeleteFunc(w.Offers, func(o game.Offer) bool { return o.With() == id })
		return false
	}
	w.Offers = slices.DeleteFunc(w.Offers, func(o game.Offer) bool { return o.With() == id && t.Day > o.Expires })
	for _, o := range w.Today.Accepted {
		if o.With() != id || w.DealWith(id, o.Deal.Kind) != nil {
			continue
		}
		d := o.Deal
		d.Offered = true
		s.seal(w, t, r, d)
	}
	betrayed := false
	if d := w.DealWith(id, game.DealTribute); d != nil {
		if w.Player.DirtyCash < d.Terms.PerDay {
			s.betray(w, t, r, *d, "the tribute went unpaid")
			betrayed = true
		} else {
			w.Player.DirtyCash -= d.Terms.PerDay
			r.Cash += d.Terms.PerDay
			w.Stats.Tribute += d.Terms.PerDay
			t.Emit(events.TributePaid{Day: t.Day, Rival: r.Leader, Faction: id, Amount: d.Terms.PerDay})
		}
	}
	// A homage (#43) runs the other way: it pays you out of its chest,
	// and a night it cannot pay ends it (they stopped bowing; nothing
	// you did).
	if d := w.DealWith(id, game.DealHomage); d != nil {
		if r.Cash < d.Terms.PerDay {
			r.Deals = slices.DeleteFunc(r.Deals, func(x game.Deal) bool { return x.Kind == game.DealHomage })
			t.Emit(events.DealEnded{Day: t.Day, Rival: r.Leader, Faction: id, Deal: game.DealHomage})
		} else {
			r.Cash -= d.Terms.PerDay
			w.Player.DirtyCash += d.Terms.PerDay
			w.Stats.Homage += d.Terms.PerDay
			t.Emit(events.TributePaid{Day: t.Day, Rival: r.Leader, Faction: id, Amount: d.Terms.PerDay, ToYou: true})
		}
	}
	if d := w.DealWith(id, game.DealSplit); d != nil {
		for _, cid := range w.Today.Abandoned {
			if d.Covers(cid) {
				name := cid
				if c := w.Corner(cid); c != nil {
					name = c.Name
				}
				s.betray(w, t, r, *d, fmt.Sprintf("you walked off %s", name))
				betrayed = true
				break
			}
		}
	}
	return betrayed
}

// crossed is the player's strike tonight breaking every peace with the
// faction struck: a push or a hit under a live deal is a betrayal of
// all of them. A warning is not.
func (s *Sim) crossed(w *game.World, t *game.Tick, r *game.RivalState, o *game.StrikeOrder) bool {
	if o == nil || o.Force == events.ForceWarn {
		return false
	}
	c := w.Corner(o.Corner)
	if c == nil || !owns(*c, r) {
		return false
	}
	why := fmt.Sprintf("your enforcers %s %s", pastTense(o.Force), c.Name)
	if o.Boost {
		why = "your enforcers robbed " + c.Name
	}
	return s.breakAll(w, t, r, why)
}

// breakAll is the player breaking every live deal with a faction at
// once, for the reason given; it reports whether there was one to
// break.
func (s *Sim) breakAll(w *game.World, t *game.Tick, r *game.RivalState, why string) bool {
	betrayed := false
	for _, kind := range []string{game.DealTruce, game.DealTribute, game.DealSplit, game.DealHomage} {
		if d := w.DealWith(r.Faction(), kind); d != nil {
			s.betray(w, t, r, *d, why)
			betrayed = true
		}
	}
	return betrayed
}

func pastTense(f events.Force) string {
	switch f {
	case events.ForceHit:
		return "hit"
	default:
		return "pushed on"
	}
}

// answer is the faction's reply to tonight's proposal, if it was put to
// it.
func (s *Sim) answer(w *game.World, t *game.Tick, r *game.RivalState, rng game.Rand) {
	p := w.Today.Proposal
	if p == nil || r.Arrived == 0 || w.Faction(p.Faction) != r {
		return
	}
	r.Observed = true
	d := *p
	d.Offered = false
	if w.DealWith(r.Faction(), d.Kind) == nil && rng.Float64() < s.Chance(w, r, d) {
		s.seal(w, t, r, d)
		return
	}
	w.Stats.DealsRefused++
	t.Emit(events.DealRefused{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Deal: d.Kind, Terms: w.Describe(d)})
}

// whim is the faction breaking a deal of its own accord: a chaotic one
// does, by personality; the others never.
func (s *Sim) whim(w *game.World, t *game.Tick, r *game.RivalState, rng game.Rand) {
	pc := s.personality(r)
	if pc.Betrayal <= 0 {
		return
	}
	for i := 0; i < len(r.Deals); i++ {
		d := r.Deals[i]
		if !d.Live(t.Day) || rng.Float64() >= pc.Betrayal {
			continue
		}
		r.Deals = append(r.Deals[:i], r.Deals[i+1:]...)
		i--
		r.Observed = true
		w.Stats.BetrayedBy++
		t.Emit(events.DealBroken{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Deal: d.Kind, By: "rival", Why: "they felt like it"})
		s.betrayed(w, t, r)
	}
}

// offLimits reports whether a deal keeps the faction off a corner
// tonight: a truce, a tribute or a homage keeps it off every corner of
// yours, a split off every corner on your side of the line.
func (s *Sim) offLimits(w *game.World, r *game.RivalState, c *game.Corner) bool {
	if w.AtPeaceWith(r.Faction()) {
		return true
	}
	if d := w.DealWith(r.Faction(), game.DealSplit); d != nil && d.Covers(c.ID) {
		return true
	}
	return false
}

// keep closes the night's books on the deals: the ones that ran out end,
// the rest earn trust, faster for a respected player.
func (s *Sim) keep(w *game.World, t *game.Tick, r *game.RivalState) {
	dip := s.cfg.Diplomacy
	earn := dip.TrustKept * content.Scale(w.Player.Reputation.Respect, s.rep.RespectTrust)
	for i := 0; i < len(r.Deals); i++ {
		d := r.Deals[i]
		if d.Until > 0 && t.Day+1 >= d.Until {
			r.Deals = append(r.Deals[:i], r.Deals[i+1:]...)
			i--
			t.Emit(events.DealEnded{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Deal: d.Kind})
			continue
		}
		r.Trust = math.Min(100, r.Trust+earn)
	}
}

// offer is the faction putting a deal on the table when its situation
// calls for one: an expansionist that cannot pay its muscle asks for a
// truce, an opportunist with the upper hand on the front line demands
// tribute, a defensive one asks for a long truce once the war is loud,
// a chaotic one with a front line asks for a short one on a whim. One
// offer of its own on the table at a time.
func (s *Sim) offer(w *game.World, t *game.Tick, r *game.RivalState, rng game.Rand) {
	tun := s.cfg.Rivals
	dip := s.cfg.Diplomacy
	pc := s.personality(r)
	id := r.Faction()
	if r.Arrived == 0 || w.RivalHeldBy(id) == 0 || w.Held() == 0 || s.offering(w, id) || pc.OfferChance <= 0 || s.Distrusted(r, t.Day) {
		return
	}
	var d game.Deal
	switch r.Personality {
	case "expansionist":
		if r.Cash >= s.Wages(w, r)*dip.LowCashDays {
			return
		}
		d = game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: dip.TruceDays[1]}}
	case "opportunist":
		front, guard := 0, 0.0
		for _, c := range s.corners(w, r) {
			if c.Held() && w.ContestedBy(c, id) {
				front++
				guard += s.Guard(w, &c)
			}
		}
		if front == 0 || float64(r.Muscle) < dip.UpperHand*math.Max(1, guard) {
			return
		}
		d = game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: s.cut(w, r, dip.TributeCuts[1])}}
	case "defensive":
		if r.War < tun.WarThreshold {
			return
		}
		d = game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: dip.TruceDays[2]}}
	case "chaotic":
		if s.frontline(w, r) == 0 {
			return
		}
		d = game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: dip.TruceDays[0]}}
	default:
		return
	}
	if w.DealWith(id, d.Kind) != nil || (w.Today.Proposal != nil && w.Today.Proposal.Kind == d.Kind && w.Faction(w.Today.Proposal.Faction) == r) || rng.Float64() >= pc.OfferChance {
		return
	}
	s.putOffer(w, t, r, d)
}

// offering reports whether a faction has an offer on the table.
func (s *Sim) offering(w *game.World, id string) bool {
	for _, o := range w.Offers {
		if o.With() == id {
			return true
		}
	}
	return false
}

// putOffer puts a faction's deal on the table for offer_days.
func (s *Sim) putOffer(w *game.World, t *game.Tick, r *game.RivalState, d game.Deal) {
	dip := s.cfg.Diplomacy
	d.Offered = true
	r.NextOffer++
	o := game.Offer{ID: r.NextOffer, Deal: d, Expires: t.Day + dip.OfferDays - 1}
	if r != w.Rival() {
		d.Faction = r.Faction() // the rival at home's offers read as they always did
		o.Deal.Faction = r.Faction()
		o.Faction = r.Faction()
	}
	w.Offers = append(w.Offers, o)
	r.Observed = true
	t.Emit(events.DealOffered{Day: t.Day, ID: o.ID, Rival: r.Leader, Faction: r.Faction(), Deal: d.Kind, Terms: w.Describe(d), Expires: o.Expires})
}

// cut is a tribute at a cut of TributeBase, the player's daily street
// value in the products the faction deals in, rounded to two figures
// and never under the minimum.
func (s *Sim) cut(w *game.World, r *game.RivalState, share float64) int {
	return s.round(share * s.TributeBase(w, r))
}

// round is a sum to two figures, never under the tribute minimum.
func (s *Sim) round(v float64) int {
	if v < 100 {
		return max(s.cfg.Diplomacy.TributeMin, int(math.Round(v)))
	}
	mag := math.Pow(10, math.Floor(math.Log10(v))-1)
	return max(s.cfg.Diplomacy.TributeMin, int(math.Round(v/mag)*mag))
}

// Cut is the tribute a cut of today's TributeBase comes to, for the
// propose dialog and the diplomat policy.
func (s *Sim) Cut(w *game.World, r *game.RivalState, share float64) int { return s.cut(w, r, share) }
