package harness

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/market"
)

// Quality (#47). Cutter wraps any policy to cut everything it bought
// today by a ratio (`cmd/balance -cut R`): the greed curve is what it
// measures. Cook is laundered with a chemist who cooks meth and
// designer instead of buying them (`-policy cook`).

// Cutter plays policy, then cuts what it bought today where it stands
// by ratio: of the units bought it returns ratio / (1 + ratio) to the
// connect (World.Return, the exact inverse of the buy, so the cash
// comes back and the price walks back) and cuts the lot to put the
// same units back at nothing, so the bag holds what the policy meant it
// to hold, the order it placed stands, and the cut's whole saving is
// the returned units' cost. The lot's mean falls as if the units bought
// alone were cut (a cut is the same units at nothing wherever they
// came from). A chemist on the payroll keeps what the file says; the
// wrapper hires none.
func Cutter(cfg *content.Config, ratio float64, policy Policy) Policy {
	mk, err := market.New(cfg)
	if err != nil {
		panic("harness.Cutter: " + err.Error())
	}
	cs := crew.New(cfg)
	return func(w *game.World) {
		policy(w)
		if ratio <= 0 || w.Today.LieLow {
			return
		}
		city := w.Player.Location
		for _, id := range w.Products {
			bought := w.Bought(city, id)
			if bought <= 0 || mk.CutMax(id) <= 0 {
				continue
			}
			back := bought - int(math.Round(float64(bought)/(1+ratio)))
			if back <= 0 {
				continue
			}
			if _, err := w.Return(city, id, back); err != nil {
				continue
			}
			stock := w.Stock(city, id)
			if stock <= 0 {
				continue
			}
			r := math.Min(mk.CutMax(id), float64(back)/float64(stock))
			_, _ = w.Cut(city, id, r, mk.CutMax(id), mk.CutCost(id), cs.CutBonus(w), cs.ChemistName(w))
		}
	}
}

// Cook is laundered with a chemist (#47): it hires the first chemist
// looking for work (the least skilled runner making room on a full
// roster, as the delegated player does for a lieutenant), and every
// morning orders a cook of every product the file cooks toward the
// share of the bag the demand gives it, instead of buying it; the rest
// of the ladder it buys as laundered does. The lot lands cook_days
// later at the chemist's quality and sells with everything else.
func Cook(cfg *content.Config, lieLowAt float64) Policy {
	mk, err := market.New(cfg)
	if err != nil {
		panic("harness.Cook: " + err.Error())
	}
	cs := crew.New(cfg)
	hot := TooHot(cfg, lieLowAt)
	cooks := func(id string) bool { return mk.Cooks(id) }
	return func(w *game.World) {
		washUp(cfg, w)
		hireChemist(cfg, w)
		staff(cfg, w, w.Player.Location, 0)
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		standSomewhere(w)
		cookToward(cfg, mk, cs, w)
		restockOnly(cfg, w, func(id string) bool { return !cooks(id) })
		sellEverything(w, events.DialNormal)
	}
}

// hireChemist signs the chemist in the pool when none is on the
// payroll, firing the least skilled runner to make room on a full
// roster with three or more of them.
func hireChemist(cfg *content.Config, w *game.World) {
	if w.Crew.Chemist() != nil {
		return
	}
	var chem *game.CrewMember
	for i := range w.Crew.Candidates {
		if c := &w.Crew.Candidates[i]; c.Role == game.RoleChemist && (chem == nil || c.Skill > chem.Skill) {
			chem = c
		}
	}
	if chem == nil {
		return
	}
	maxCrew := crew.New(cfg).MaxCrew(w)
	if len(w.Crew.Members) >= maxCrew {
		if w.Crew.Runners() < 3 || len(w.Crew.FiredToday) > 0 {
			return
		}
		worst := -1
		for i, m := range w.Crew.Members {
			if m.Role == "runner" && (worst < 0 || m.Skill < w.Crew.Members[worst].Skill) {
				worst = i
			}
		}
		if worst < 0 {
			return
		}
		_, _ = w.Fire(w.Crew.Members[worst].ID)
	}
	if w.Player.DirtyCash >= chem.Fee+cfg.Market.Market.StartCash {
		_, _ = w.Hire(chem.ID, maxCrew)
	}
}

// cookToward orders a cook of every product the file cooks, where you
// stand, toward the share of the street's bag the demand gives it
// (restock's rule), counting what is already on its way, up to the
// chemist's batch and the till; nothing with no chemist.
func cookToward(cfg *content.Config, mk *market.Sim, cs *crew.Sim, w *game.World) {
	if w.Crew.Chemist() == nil {
		return
	}
	city := w.Here()
	total := 0.0
	for _, id := range w.Products {
		if m := city.Market[id]; m != nil && (!m.NoSupply || mk.Cooks(id)) {
			total += m.Demand
		}
	}
	if total <= 0 {
		return
	}
	for _, id := range w.Products {
		m := city.Market[id]
		if m == nil || !mk.Cooks(id) {
			continue
		}
		target := int(float64(w.StreetCapacity(city.ID)) * m.Demand / total)
		short := target - w.Stock(city.ID, id) - w.Crew.Cooking(city.ID, id)
		units := min(short, cs.Batch(w), w.Free(city.ID)-w.Crew.Cooking(city.ID, id))
		if cost := mk.CookCost(id); cost > 0 {
			units = min(units, (w.Player.DirtyCash-cfg.Market.Market.StartCash)/cost)
		}
		if units <= 0 {
			continue
		}
		_, _ = w.CookOrder(city.ID, id, units, mk.CookCost(id), cs.CookDays(), cs.ChemistQuality(w), cs.Batch(w), cs.ChemistName(w))
	}
}
