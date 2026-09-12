package rivals

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The books (#70): the player's fourth answer to the rival, beside the
// enforcers, the table and the price war. The rival is a machine with a
// weak point the player could not see or reach: its muscle is what the
// take pays for and its chest is what a hire or a claim needs (#139).
// Four moves make it legible and let the player drain it, each queued
// on the world as intent and resolved here: a scout reads the books
// (Rival.Known, a snapshot that goes stale), a boost sends the enforcers
// in for a corner's takings rather than the ground (the strike's order,
// the strike's odds, the strike's roll), a tip sends the police to one
// of its corners (Rival.Heat, and past the notice line a raid), and a
// buy-off pays its muscle to go home. The scout and the buy-off roll on
// the books side stream, Tick.Sub("books"), so a run that never uses
// them is byte-for-byte the old run; the boost rolls where the strike
// rolls, since it is the strike order, one roll either way.

// Books exposes the scouting tuning for the UI and the harness.
func (s *Sim) Books() content.BooksTuning { return s.cfg.Books }

// BoostTuning exposes the boost's numbers.
func (s *Sim) BoostTuning() content.BoostTuning { return s.cfg.Boost }

// TipTuning exposes the tip's numbers.
func (s *Sim) TipTuning() content.TipTuning { return s.cfg.Tip }

// PoachTuning exposes the buy-off's numbers.
func (s *Sim) PoachTuning() content.PoachTuning { return s.cfg.Poach }

// ScoutCost is what a look at the rival's books costs.
func (s *Sim) ScoutCost() int { return s.cfg.Books.ScoutCost }

// ScoutOdds is the chance tonight's scout reads the rival's books: the
// base, plus the best enforcer's skill, plus a little for every scout
// that read nothing since the last that did. The confirmation shows it
// and the dice use it; it is the shape of the crew's investigation so
// the two read the same to the player.
func (s *Sim) ScoutOdds(w *game.World) float64 {
	tun := s.cfg.Books
	best := 0
	for _, m := range w.Crew.Members {
		if m.Role == "enforcer" && m.Skill > best {
			best = m.Skill
		}
	}
	p := tun.ScoutBase + tun.ScoutSkill*float64(best)/100 + tun.ScoutLearn*float64(w.Rival.Scouted)
	return math.Max(0, math.Min(1, p))
}

// Stale reports whether the books as last read are stale_days old or
// more on day; false while they have never been read (there is nothing
// to be stale).
func (s *Sim) Stale(w *game.World, day int) bool {
	k := w.Rival.Known
	return k.Read() && k.Age(day) >= s.cfg.Books.StaleDays
}

// BoostTake is what a boost that lands on a corner takes tonight: take
// of the corner's day of takings, the street value it moved (trade, a
// price war's squeeze off it), which is the till on the corner, not the
// rival's margin of it: at half the margin a boost took $530 and the
// chest grew regardless, and a drain that never binds is not a move.
// The confirmation shows it and the boost takes it off the chest.
func (s *Sim) BoostTake(w *game.World, c game.Corner) int {
	return int(math.Round(s.cfg.Boost.Take * s.trade(w, c)))
}

// BoostHeat is what a boost on a corner draws, landed or not: the
// boost's heat times the corner's.
func (s *Sim) BoostHeat(c *game.Corner) float64 { return s.cfg.Boost.Heat * c.Heat }

// MusclePrice is what a head of the rival's muscle costs to buy off
// tonight: muscle_price corner-days plus cash_share of its chest per
// head it keeps (a well-paid crew costs more), cut by the player's
// respect. The dialog shows it and BuyOff pays it.
func (s *Sim) MusclePrice(w *game.World) int {
	p := s.cfg.Poach
	r := w.Rival
	v := p.MusclePrice*s.CornerDay(w) + p.CashShare*float64(r.Cash)/float64(max(1, r.Muscle))
	v *= content.Cut(w.Player.Reputation.Respect, s.rep.PoachPriceCut)
	return max(1, int(math.Round(v)))
}

// RaidReady reports whether the police would act on the rival's heat
// on the night of day: the last raid was raid_days ago or more. A tip
// meanwhile builds heat the police sit on.
func (s *Sim) RaidReady(w *game.World, day int) bool {
	r := w.Rival
	return r.LastRaid == 0 || day-r.LastRaid >= s.cfg.Tip.RaidDays
}

// boost resolves the enforcers going in for a corner's takings (#70):
// the force's odds on the strike's roll, and landing, take of what the
// corner earned the rival today off its chest and into your dirty cash,
// the corner's ownership untouched. It draws the boost's heat and adds
// its war whatever happens, costs the force's trust, and the toll on the
// enforcers is the boost's loyalty landing or fail_loss failing, when a
// rival with more than fail_muscle heads also hurts one of them. It
// holds no grudge: masked men took the till, and the war and the
// force's trust are what it costs at the table (a grudge a landed boost
// brought the saboteur 24 calls in 65 days, a hit war's noise). Nor is
// it the strike that lifts the rival's claim cooldown (#60, LastStruck):
// a robbery is not a fight for ground, and a rival robbed every few
// nights that claimed as fast as it could held more of the city than
// one left alone.
func (s *Sim) boost(w *game.World, t *game.Tick, o *game.StrikeOrder, c *game.Corner) {
	b := s.cfg.Boost
	fc := s.cfg.ForceFor(o.Force)
	r := &w.Rival
	ev := events.RivalBoosted{
		Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), Force: o.Force,
		Heat: s.BoostHeat(c), Toll: b.Loyalty,
	}
	w.Stats.Boosts++
	r.Observed = true
	r.War += b.War
	r.Trust = math.Max(0, r.Trust-fc.Trust)
	if t.RNG.Float64() < s.Odds(w, o.Force) {
		ev.Taken = true
		ev.Cash = s.BoostTake(w, *c)
		r.Cash -= ev.Cash
		w.Player.DirtyCash += ev.Cash
		w.Stats.Boosted += ev.Cash
	} else {
		ev.Toll = b.FailLoss
		if r.Muscle > b.FailMuscle {
			ev.Hurt = b.FailHurt
		}
	}
	t.Emit(ev)
}

// poach resolves the player paying the rival's muscle to go home (#70),
// off the books side stream: at the odds the heads leave, never more
// than it has (what was paid for the rest comes back) and it never
// finds out; failing, the money is gone and it holds a grudge.
func (s *Sim) poach(w *game.World, t *game.Tick) {
	o := w.Poach
	if o == nil {
		return
	}
	r := &w.Rival
	ev := events.RivalMusclePoached{Day: t.Day, Rival: r.Leader, Faction: r.Faction(), Wanted: o.Units, Cost: o.Cost}
	if t.Sub("books").Float64() < s.cfg.Poach.Odds {
		ev.Got = min(o.Units, r.Muscle)
		r.Muscle -= ev.Got
		s.sendAway(w, t, ev.Got)
		w.Stats.Poached += ev.Got
		if ev.Got < o.Units {
			ev.Refund = o.Cost * (o.Units - ev.Got) / o.Units
			w.Player.DirtyCash += ev.Refund
		}
	} else {
		ev.Failed = true
		r.Grudge += s.cfg.Poach.Grudge
		r.Observed = true
	}
	t.Emit(ev)
}

// tip resolves the player's tip to the police on a rival corner (#70):
// the rival's heat rises, its trust falls, and under a truce or a
// tribute it is a betrayal of every deal (the truce binds it to no tips,
// so it binds you); past the notice line the police raid the corner
// tonight, and that is what it holds a grudge over (a grudge a tip made
// an expansionist push at full pace past its cap and take the
// saboteur's corners faster than it took the passive player's). A tip on a corner that is no longer the rival's
// by the time the night comes is dropped. It returns whether a deal was
// broken, for the caller's phone call.
func (s *Sim) tip(w *game.World, t *game.Tick) bool {
	o := w.Tipoff
	if o == nil {
		return false
	}
	c := w.Corner(o.Corner)
	if c == nil || c.Owner != game.OwnerRival {
		return false
	}
	tp := s.cfg.Tip
	r := &w.Rival
	w.Stats.Tips++
	r.Heat = math.Min(100, r.Heat+tp.Heat)
	r.Trust = math.Max(0, r.Trust-tp.Trust)
	r.Observed = true
	ev := events.PoliceTipped{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), RivalHeat: r.Heat}
	if w.AtPeace() {
		ev.Betrayal = s.breakAll(w, t, "you tipped the police on "+c.Name)
	}
	t.Emit(ev)
	if r.Heat >= tp.PoliceNotice && s.RaidReady(w, t.Day) {
		s.raid(w, t, c)
	}
	return ev.Betrayal
}

// raid is the police taking a rival corner on the player's tips: the
// corner goes back to the street, raid_muscle of its muscle (at least a
// head) is gone, its heat comes down by the notice line and it holds
// the tip's grudge. A raid is not a rout: the police take a corner, not
// a faction, so a rival raided off its last corner is not sent to
// regroup and sets up again at its own pace (a raid that routed it had
// the tipster holding every rival at zero corners from its arrival for
// four tips a month). Not twice
// within raid_days: the heat builds meanwhile and the raid comes when
// the police are ready, so a tip a night is a corner every raid_days
// at a page a tip in four, not a rival routed in a month for nothing.
func (s *Sim) raid(w *game.World, t *game.Tick, c *game.Corner) {
	tp := s.cfg.Tip
	r := &w.Rival
	lost := int(math.Round(float64(r.Muscle) * tp.RaidMuscle))
	if r.Muscle > 0 && lost == 0 {
		lost = 1
	}
	r.Muscle -= lost
	s.sendAway(w, t, lost)
	r.Heat = math.Max(0, r.Heat-tp.PoliceNotice)
	r.LastRaid = t.Day
	r.Grudge += tp.Grudge
	c.Owner, c.Faction, c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Since = game.OwnerNone, "", 0, 0, 0, 0, t.Day
	c.Starved, c.StarvedDay = 0, 0
	w.Stats.RivalRaids++
	t.Emit(events.RivalRaided{Day: t.Day, Corner: c.ID, Name: c.Name, Rival: r.Leader, Faction: r.Faction(), Muscle: lost})
}

// scout resolves the player's look at the rival's books (#70), off the
// books side stream: at the odds it stamps Known with tonight's numbers
// (the chest and the muscle as the night leaves them, the take and the
// wage bill as today read them), else it reads nothing and the next
// look is a little likelier.
func (s *Sim) scout(w *game.World, t *game.Tick) {
	o := w.Scouting
	if o == nil {
		return
	}
	r := &w.Rival
	w.Stats.Scouts++
	ev := events.RivalScouted{Day: t.Day, Cost: o.Cost}
	if t.Sub("books").Float64() < s.ScoutOdds(w) {
		ev.Read = true
		r.Known = game.Known{Day: t.Day, Cash: r.Cash, Income: s.Income(w), Muscle: r.Muscle, Wages: s.Wages(w)}
		r.Scouted = 0
	} else {
		r.Scouted++
	}
	t.Emit(ev)
}

// sendAway puts n heads out of the rival's reach (#70): bought off or
// in the van, it wants that many fewer until they come back.
func (s *Sim) sendAway(w *game.World, t *game.Tick, n int) {
	if n <= 0 {
		return
	}
	r := &w.Rival
	if r.Away == 0 {
		r.AwayDay = t.Day
	}
	r.Away += n
}

// comeBack is one head of those away finding its way back to the rival
// every away_days days; none away, nothing moves.
func (s *Sim) comeBack(w *game.World, t *game.Tick) {
	r := &w.Rival
	if r.Away <= 0 {
		return
	}
	if t.Day-r.AwayDay >= s.cfg.Poach.AwayDays {
		r.Away--
		r.AwayDay = t.Day
	}
}

// heat is the police's attention on the rival fading a day, and rising
// with every push it made tonight while it already had some (#70): a
// rival already watched draws more with every fight; at zero its own
// violence draws none, so a run that never tips is the old run.
func (s *Sim) heat(w *game.World, pushes int) {
	tp := s.cfg.Tip
	r := &w.Rival
	if r.Heat <= 0 {
		return
	}
	r.Heat += tp.PushHeat * float64(pushes)
	r.Heat -= r.Heat * tp.Decay
	r.Heat = math.Max(0, math.Min(100, r.Heat))
}
