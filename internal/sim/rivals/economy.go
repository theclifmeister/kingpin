// The faction's money (#139): its take, the corner-day its prices are
// counted in, the wage, the fee and the claim, and the payroll that
// runs them every night (step 2).

package rivals

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Income is what a faction's corners earn it in a day: margin of the
// street value each moves, less what a price war (#68) is taking off it
// (Corner.Squeeze, which the market sim writes on the rival's corners).
func (s *Sim) Income(w *game.World, r *game.RivalState) int {
	v := 0.0
	for _, c := range s.corners(w, r) {
		if owns(c, r) {
			v += s.trade(w, c)
		}
	}
	return int(math.Round(v * s.cfg.Rivals.Margin))
}

// trade is the street value a rival corner moves in a day, squeeze off,
// in the products the street sells there (#139): the port's product
// (no_supply at home) comes by the road, which the rival does not run.
func (s *Sim) trade(w *game.World, c game.Corner) float64 { return w.Trade(c) }

// CornerIncome is what one of a faction's corners earns it in a day
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

// Standard is the street value a standard corner in a faction's city
// moves in a day in the products the street there sells (#139): demand
// per standard corner times price, summed over the ladder as unlocked,
// the port's product left out. It is what a corner-day is a margin of.
func (s *Sim) Standard(w *game.World, r *game.RivalState) float64 {
	h := s.city(w, r)
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
// corners the player works in the faction's city move in a day in the
// products the faction deals in, the port's product (no_supply at
// home) left out as Income leaves it out of the rival's own take, so a
// cut of it and the take are in one unit. Favour, the opportunist's
// demand, the propose dialog and the rivals pane all read it.
//
// Since #532 it is what you took there, not what you could: your sales
// in the city a night over the last tribute_days nights (World.Taking,
// in the same products), where a night is recorded; the potential below
// only where none is (a save from before, or tribute_days 0).
func (s *Sim) TributeBase(w *game.World, r *game.RivalState) float64 {
	h := s.city(w, r)
	if h == nil {
		return 0
	}
	if s.cfg.Diplomacy.TributeDays > 0 {
		if v, _, ok := w.Taking(h.ID); ok {
			return float64(v)
		}
	}
	v := 0.0
	for _, id := range w.Products {
		if m := h.Market[id]; m != nil && !m.NoSupply {
			v += w.Demand(h.ID, id) * m.Price
		}
	}
	return v
}

// takings records tonight's sales in every city (#532, World.Takings):
// the tick's PlayerSold revenue in the products a faction deals in
// there (the port's no_supply product left out, as TributeBase and
// Income leave it), kept to tribute_days nights. A city gets a row the
// first night anything sells there. No dice.
func (s *Sim) takings(w *game.World, t *game.Tick) {
	n := s.cfg.Diplomacy.TributeDays
	if n <= 0 {
		return
	}
	sold := map[string]int{}
	for _, e := range t.Events() {
		ev, ok := e.(events.PlayerSold)
		if !ok || ev.Revenue <= 0 {
			continue
		}
		if m := w.Product(ev.City, ev.Product); m != nil && !m.NoSupply {
			sold[ev.City] += ev.Revenue
		}
	}
	for _, cid := range w.CityOrder {
		row, ok := w.Takings[cid]
		if !ok && sold[cid] == 0 {
			continue
		}
		row = append(row, sold[cid])
		if len(row) > n {
			row = append([]int(nil), row[len(row)-n:]...)
		}
		if w.Takings == nil {
			w.Takings = map[string][]int{}
		}
		w.Takings[cid] = row
	}
}

// CornerDay is the unit a faction's money is priced in (#139): what a
// standard corner in its city earns it in a day, margin of Standard. Its
// wage, its fee, its claim and the chest it arrives with are so many
// corner-days, so they climb the ladder with its take.
func (s *Sim) CornerDay(w *game.World, r *game.RivalState) float64 {
	return s.Standard(w, r) * s.cfg.Rivals.Margin
}

// cost is a price in corner-days as today's dollars, rounded.
func (s *Sim) cost(w *game.World, r *game.RivalState, days float64) int {
	return int(math.Round(days * s.CornerDay(w, r)))
}

// Wage is what one head of a faction's muscle costs it a day: the
// corner's guard (muscle_wage corner-days) over the heads its
// personality puts on a corner, never under a dollar.
func (s *Sim) Wage(w *game.World, r *game.RivalState) int {
	heads := s.personality(r).MusclePerCorner
	if heads <= 0 {
		heads = 1 // no personality yet: a head a corner
	}
	return max(1, s.cost(w, r, s.cfg.Rivals.MuscleWage/heads))
}

// Wages is a faction's wage bill for the day: its muscle at the wage.
func (s *Sim) Wages(w *game.World, r *game.RivalState) int { return r.Muscle * s.Wage(w, r) }

// Fee is what recruiting a head costs it today.
func (s *Sim) Fee(w *game.World, r *game.RivalState) int { return s.cost(w, r, s.cfg.Rivals.MuscleFee) }

// ClaimCost is what setting up on a free corner costs it today.
func (s *Sim) ClaimCost(w *game.World, r *game.RivalState) int {
	return s.cost(w, r, s.cfg.Rivals.ClaimCost)
}

// Afford is the muscle the day's take pays for: its income over the
// wage. It recruits no further than it, and a payroll over it runs up
// arrears until a head walks (#139).
func (s *Sim) Afford(w *game.World, r *game.RivalState) int { return s.Income(w, r) / s.Wage(w, r) }

// Want is the muscle its personality keeps for the corners it holds:
// muscle_per_corner per corner plus one, rounded, less the heads bought
// off or arrested and not yet back (#70, Rival.Away).
func (s *Sim) Want(w *game.World, r *game.RivalState) int {
	return max(0, int(math.Round(s.personality(r).MusclePerCorner*float64(w.RivalHeldBy(r.Faction())+1)))-r.Away)
}

// landless is what a run-out faction spends off its chest tonight
// (#530): with no corner anywhere and no take, landless_burn of the
// chest and at least landless_floor corner-days, never more than it
// holds, so the absorb_days "sooner if broke" comes (absorb reads the
// chest against a claim). A faction on the street, or not yet on it,
// spends nothing; with neither number in the file nothing is spent,
// the run before. No dice.
func (s *Sim) landless(w *game.World, r *game.RivalState, income int) int {
	f := s.cfg.Factions
	if (f.LandlessBurn <= 0 && f.LandlessFloor <= 0) || r.Arrived == 0 || income > 0 || r.Cash <= 0 {
		return 0
	}
	if w.RivalHeldBy(r.Faction()) > 0 || (r.Routed == 0 && r.RaidedOut == 0) {
		return 0
	}
	return s.burn(w, r, r.Cash)
}

// burn is a night's landless spend off a chest of cash (#530).
func (s *Sim) burn(w *game.World, r *game.RivalState, cash int) int {
	f := s.cfg.Factions
	return min(cash, max(int(math.Round(float64(cash)*f.LandlessBurn)), s.cost(w, r, f.LandlessFloor)))
}

// Broke is the night a landless faction's chest falls under a claim,
// spending tonight's wages and what landless spends from tonight at
// today's prices (#530): what the crown's count reads for "sooner if
// broke". 0 when it does not within limit nights, or nothing is spent.
func (s *Sim) Broke(w *game.World, r *game.RivalState, limit int) int {
	f := s.cfg.Factions
	if f.LandlessBurn <= 0 && f.LandlessFloor <= 0 {
		return 0
	}
	cash, claim, wages := r.Cash, s.ClaimCost(w, r), s.Wages(w, r)
	for n := 1; n <= limit; n++ {
		cash -= min(cash, wages)
		cash -= s.burn(w, r, cash)
		if cash < claim {
			return w.Day + n
		}
	}
	return 0
}

// payroll is the faction's money for the day (step 2, #139): the take,
// and the muscle it pays for. The wages come out of the chest; what the
// day's take did not cover of them is owed (Arrears), a surplus day pays
// the owing down, and once a full wage is owed a head walks, one a day,
// never one of the start_muscle it came with (those the chest carries).
// A chest that cannot meet the payroll is the last resort, muscle down
// to what it holds. Then it recruits up to what it wants and what the
// take pays for, while the chest covers a fee and a claim besides. No
// dice: income and expenses are arithmetic.
func (s *Sim) payroll(w *game.World, r *game.RivalState) {
	income := s.Income(w, r)
	r.Cash += income
	wage := s.Wage(w, r)
	wages := r.Muscle * wage
	if wages > r.Cash {
		r.Muscle = r.Cash / wage
		wages = r.Muscle * wage
	}
	r.Cash -= wages
	r.Arrears = math.Max(0, r.Arrears+float64(wages-income))
	if r.Arrears >= float64(wage) && r.Muscle > s.cfg.Rivals.StartMuscle {
		r.Muscle--
		r.Arrears -= float64(wage)
	}
	r.Cash -= s.landless(w, r, income)
	fee, claim := s.Fee(w, r), s.ClaimCost(w, r)
	for r.Muscle < min(s.Want(w, r), s.Afford(w, r)) && r.Cash >= fee+claim {
		r.Cash -= fee
		r.Muscle++
	}
}
