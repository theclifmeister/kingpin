package crew

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// skim takes the street crew's cut of the day's takings, the
// accountants' of the wash and the lieutenants' take.
func (s *Sim) skim(n *night) {
	w, t, c, tun, fx := n.w, n.t, n.c, n.tun, n.fx
	// Skimming, on this morning's loyalty. Street crew skim the day's
	// takings; an accountant skims the wash, and the wash they can see is
	// the last one the fronts did (laundering steps after crew), so it
	// comes out of clean cash.
	revenue, wash := 0, 0
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok {
			revenue += ps.Revenue
		}
	}
	for _, f := range w.Fronts {
		wash += f.WashedToday
	}
	n.enforcers = c.Role(game.RoleEnforcer)
	deter := math.Pow(1-s.cfg.Role[game.RoleEnforcer].Deterrence, s.deterrence(c, n.enforcers)) // a hothead counts (#346)
	share, washShare := 0.0, 0.0
	skimmers := 0
	for _, m := range c.Members {
		if m.Loyalty >= tun.SkimThreshold {
			continue
		}
		if t.RNG.Float64() < s.skimOdds(m, fx, deter) {
			cut := tun.SkimShare * (0.5 + float64(m.Greed)/100)
			if m.Role == game.RoleAccountant {
				washShare += cut
			} else {
				share += cut
			}
			skimmers++
		}
	}
	// The lieutenants' cut of their cities' takings, and what a greedy
	// one skims on top, at any loyalty: nobody deters the boss of a city.
	n.acted = map[int]*events.LieutenantActed{}
	extra := s.take(w, t, n.acted)
	if extra > 0 {
		skimmers++
	}
	if skimmers > 0 {
		amount := min(int(math.Round(float64(revenue)*math.Min(share, tun.SkimCap))), w.Player.DirtyCash-extra) + extra
		fromWash := min(int(math.Round(float64(wash)*math.Min(washShare, tun.SkimCap))), w.Player.CleanCash)
		if amount+fromWash > 0 {
			w.Player.DirtyCash -= amount
			w.Player.CleanCash -= fromWash
			w.Stats.Skimmed += amount + fromWash
			c.LastSkim = t.Day
			t.Emit(events.CrewSkimmed{Day: t.Day, Amount: amount + fromWash, Skimmers: skimmers, FromWash: fromWash})
		}
	}
}

// turn has the disloyal and nervous start talking, on the morning's
// loyalty.
func (s *Sim) turn(n *night) {
	w, t, c, fx := n.w, n.t, n.c, n.fx
	// Turning, on the same morning loyalty: the disloyal and nervous start
	// talking. Nothing is shown; the heat sim starts its clock on the event.
	// A lieutenant turns under a higher line and without dice: they know
	// where everything is, and the DA knows it. A fixer whose envelope
	// blew up last night (#42, w.Law.Backfired) loses backfire_loyalty
	// and, under the line, turns the way an audit turns an accountant
	// (#29): no dice, they were the one holding the bag.
	inf := s.cfg.Informant
	if w.Law.Backfired > 0 && w.Law.Backfired == t.Day-1 {
		if f := c.Fixer(); f != nil {
			f.Loyalty = math.Max(0, f.Loyalty-s.cfg.Role[game.RoleFixer].BackfireLoyalty)
			if !f.Informant && f.Loyalty < inf.Loyalty {
				f.Informant = true
				w.Stats.Informants++
				t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: f.ID, Name: f.Name})
			}
		}
	}
	for i := range c.Members {
		m := &c.Members[i]
		if m.Informant {
			continue
		}
		if m.Lieutenant() && m.Loyalty < s.cfg.Lieutenant.Flip {
			m.Informant = true
			w.Stats.Informants++
			t.Emit(events.LieutenantFlipped{Day: t.Day, ID: m.ID, Name: m.Name, City: m.City})
			// The betrayal (#49): a lieutenant running a city that
			// holds betray_share of your corners, betray_corners at
			// least, knows where everything is, and the run ends the
			// night they turn. A read on the map, no dice; 0 boxes it.
			if lt := s.cfg.Lieutenant; lt.BetrayShare > 0 && m.Runs() && w.Over == nil {
				if held, there := w.Held(), w.HeldIn(m.City); there >= max(1, lt.BetrayCorners) && float64(there) >= lt.BetrayShare*float64(held) {
					w.Over = w.End(content.CauseBetrayed, t.Day, m.Name)
					t.Emit(events.GameOver{Day: t.Day, Cause: content.CauseBetrayed})
				}
			}
			continue
		}
		if m.Loyalty >= inf.Loyalty || m.Nerve >= inf.Nerve {
			continue
		}
		if t.RNG.Float64() < inf.Chance*fx.InformantChanceMul {
			m.Informant = true
			w.Stats.Informants++
			t.Emit(events.CrewTurnedInformant{Day: t.Day, ID: m.ID, Name: m.Name})
		}
	}
}

// pay pays the roster's wages at the dial, and remembers what came up
// short.
func (s *Sim) pay(n *night) {
	w, t, c, fx := n.w, n.t, n.c, n.fx
	// Wages. Coming up short is remembered.
	if len(c.Members) > 0 {
		wages := s.wages(w, c.Pay, fx)
		paid := min(wages, w.Player.DirtyCash)
		n.short = wages - paid
		w.Player.DirtyCash -= paid
		w.Stats.Wages += paid
		t.Emit(events.CrewPaid{Day: t.Day, Pay: c.Pay, Wages: paid, Short: n.short})
	}
}

// broke ends the run when the wages have emptied the till with nothing
// left to sell, and says so.
func (s *Sim) broke(n *night) bool {
	w, t := n.w, n.t
	// Wages are the only way money leaves without something coming back.
	// If they empty the till with nothing left to sell, anywhere or on
	// the road, the run is over: there is no move that makes money from
	// nothing.
	if w.Over == nil && w.TotalStock() == 0 && float64(w.Player.DirtyCash) < cheapestUnit(w) {
		w.Over = w.End(content.CauseBroke, t.Day, "")
		t.Emit(events.GameOver{Day: t.Day, Cause: content.CauseBroke})
		return true
	}
	return false
}

// cheapestUnit is the lowest supplier price where the player is.
func cheapestUnit(w *game.World) float64 {
	price := math.Inf(1)
	if c := w.Here(); c != nil {
		for _, m := range c.Market {
			price = math.Min(price, m.SupplierPrice)
		}
	}
	if math.IsInf(price, 1) {
		return 0
	}
	return price
}
