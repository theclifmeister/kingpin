package heat

import (
	"fmt"
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// LieutenantHeat is what the temper of whoever runs a city does to the
// heat of every sale there: 1 for a city nobody runs.
func (s *Sim) LieutenantHeat(w *game.World, city string) float64 {
	if lt := w.Crew.Lieutenant(city); lt != nil {
		return s.lt.Temper(lt.Personality).Heat
	}
	return 1
}

// PersonalHeat is the weight of a unit you move yourself, relative to a
// unit a nobody moves: notoriety makes you the one they are watching.
func (s *Sim) PersonalHeat(w *game.World) float64 {
	return content.Scale(w.Player.Reputation.Notoriety, s.rep.NotorietyHeat)
}

func (s *Sim) dialHeat(d events.Dial) float64 { return s.market.Dial.For(d).Heat }

func (s *Sim) dialFill(d events.Dial) float64 { return s.market.Dial.For(d).Fill }

// SaleHeat is the heat drawn in a city by trying to move wanted units of
// a product there at a dial. Heat follows volume: every unit is a
// transaction somebody could see, weighted by how much the product itself
// draws attention, how loud the dial is, which corners it moves on
// (CornerWeight), how closely the city's police look and what a front
// standing there draws (EffectsIn, #344: the nightclub). The UI's dial
// preview uses it too, so the estimate is always honest.
func (s *Sim) SaleHeat(w *game.World, city, product string, wanted int, dial events.Dial) float64 {
	tun := s.cfg.Heat
	pc := s.market.Product(product)
	c := w.City(city)
	if pc == nil || c == nil || tun.StreetUnits <= 0 {
		return 0
	}
	fx := s.EffectsIn(w, city)
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
	fx := s.EffectsIn(w, city)
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

// add puts v heat on a city and, with a why, a line in its report:
// "why (+v)". A city not on the map takes nothing.
func (d *day) add(city string, v float64, why string) {
	c := d.w.Cities[city]
	if c == nil {
		return
	}
	c.Heat += v
	if why != "" {
		d.reasons[city] = append(d.reasons[city], fmt.Sprintf("%s (+%.1f)", why, v))
	}
}

// sales is the heat of the sales from the market sim, earlier in this
// tick, city by city. Heat follows the volume you tried to move at that
// dial, not what a patrol cap let through: standing on a corner shouting
// is the exposure. Which corners, and whether you or a runner stood
// there, weight it.
func (s *Sim) sales(d *day) {
	w := d.w
	for _, e := range d.t.Events() {
		ps, ok := e.(events.PlayerSold)
		if !ok || ps.Wanted == 0 {
			continue
		}
		d.attempted[ps.City] = true
		why := fmt.Sprintf("moved %d %s %s", ps.Sold, w.ProductName(ps.Product), ps.Dial)
		if ps.Delegated {
			why = fmt.Sprintf("%s moved %d %s %s", ps.LieutenantName, ps.Sold, w.ProductName(ps.Product), ps.Dial)
		} else if ps.Standing {
			why = fmt.Sprintf("standing order moved %d %s %s", ps.Sold, w.ProductName(ps.Product), ps.Dial)
		}
		v := s.SaleHeat(w, ps.City, ps.Product, ps.Wanted, ps.Dial)
		d.add(ps.City, v, why)
		s.leadSale(d, ps.City, ps.Product, v)
		d.units[ps.City] += ps.Sold
	}
}

// war is the war's heat, from the rivals sim, in the rival's city:
// enforcers you sent in, a rival's call to the precinct, the police
// clearing the front line. A shipment seized on the road, from the
// logistics sim, is heat in both cities it joined. Two of them are pages
// too, both for something you did: your tip and a shipment sent fast.
func (s *Sim) war(d *day) {
	w, t, tun, home := d.w, d.t, d.tun, d.home
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerStruck:
			d.add(home, ev.Heat, fmt.Sprintf("enforcers %s %s", pastTense(ev.Force), ev.Name))
		case events.RivalBoosted:
			d.add(home, ev.Heat, "enforcers robbed "+ev.Name)
		case events.PoliceTipped:
			// Your tip on a rival corner (#70): no heat, but talking to
			// the police is talking to the police, and tip_evidence is
			// the chance the DA's file gains a page anyway. #27's rule
			// bends only because you did something; the roll is the
			// books side stream's, so a run that never tips keeps its
			// dice. It does not reset the retainer's clock unless it
			// files.
			if tun.TipEvidence > 0 && t.Sub(game.StreamBooks).Float64() < tun.TipEvidence {
				d.file(home, 1, "your tip: the DA's file on you grows")
			}
		case events.RivalTippedPolice:
			d.add(home, ev.Heat, "somebody tipped the police")
		case events.FactionPushed:
			// Two factions fighting (#43): the heat is the city's, both
			// sides' wars raising it.
			d.add(ev.City, ev.Heat, "the factions fought over "+ev.Name)
		case events.WarEscalated:
			if ev.Stage == events.StageCrackdown {
				d.add(home, ev.Heat, "the crackdown")
			}
		case events.ShipmentSeized:
			why := fmt.Sprintf("%d %s seized on the %s", ev.Units, w.ProductName(ev.Product), ev.Mode)
			d.add(ev.From, s.ship.SeizureHeat, why)
			d.add(ev.To, s.ship.SeizureHeat, why)
			// Sent fast, it was asking to be looked at: a page in the
			// file (#27: a case is built from what you did). A seizure
			// is not a bust and never a reason to search the stash.
			if ev.Dial == events.ShipFast && s.ship.SeizureEvidence > 0 {
				d.file(ev.To, s.ship.SeizureEvidence, "sent fast: the DA's file on you grows")
			}
		}
	}
}

// contracts is the heat of a buyer's contract handed over today, from
// the market sim (#71). It is dealing like any sale: the city counts as
// attempted, so a sting tonight finds something to file (#27 holds for a
// handoff exactly as for a sale), and it weighs what the buyer's own
// multiplier says, on no corner at all.
func (s *Sim) contracts(d *day) {
	w := d.w
	for _, e := range d.t.Events() {
		cd, ok := e.(events.ContractDelivered)
		if !ok || cd.Units == 0 {
			continue
		}
		d.attempted[cd.City] = true
		v := s.ContractHeat(w, cd.City, cd.Product, cd.Units, cd.HeatMul)
		d.add(cd.City, v, fmt.Sprintf("handed %d %s to %s", cd.Units, w.ProductName(cd.Product), cd.Name))
		if d.sold != nil {
			d.sold[cd.City+"/"+cd.Product] = true
		}
		d.lead(cd.City, game.LeadProduct, cd.Product, v)
	}
}

// moves is the heat of stock driven between places today (#73): a car
// ride is exposure, not dealing, so it is heat and never a page. Each
// move is reported (StockMoved) whether it drew heat or not.
func (s *Sim) moves(d *day) {
	w, t := d.w, d.t
	for _, mv := range w.Today.Moved {
		if v := s.MoveHeat(w, mv.City, mv.Product, mv.Units); v > 0 {
			d.add(mv.City, v, fmt.Sprintf("moved %d %s between places", mv.Units, w.ProductName(mv.Product)))
			// The house the stock went into is the lead (#343); a move
			// out to the street, the house it came from.
			if h := w.House(mv.To); h != nil {
				d.lead(mv.City, game.LeadHouse, h.ID, v)
			} else if h := w.House(mv.From); h != nil {
				d.lead(mv.City, game.LeadHouse, h.ID, v)
			}
		}
		t.Emit(events.StockMoved{Day: t.Day, City: mv.City, From: placeName(w, mv.From), To: placeName(w, mv.To), Product: mv.Product, Units: mv.Units})
	}
}

// sloppy is what sloppy runners draw: every unit moved with a low-skill
// crew on the corners adds a premium.
func (s *Sim) sloppy(d *day) {
	for _, cid := range d.w.CityOrder {
		if v := s.SloppyHeat(d.w, cid, d.units[cid]); v > 0 {
			d.add(cid, v, "sloppy crew")
		}
	}
}

// dirtyCash is the pile's heat: sitting on a pile of dirty cash is its
// own tell, wherever you sit, past what your fronts give a story to
// (Cover). It runs after the envelopes' pages and before the audits', so
// the reasons where you are read in that order.
func (s *Sim) dirtyCash(d *day) {
	w := d.w
	if line := s.ExposureLine(w); line > 0 && w.Player.DirtyCash > line {
		mult := float64(w.Player.DirtyCash-line) / float64(s.DirtyCashThreshold(w))
		d.add(d.here, d.tun.DirtyCashHeat*mult, "dirty cash")
	}
}

// placeName names a place a move joined: the house, or the street.
func placeName(w *game.World, id string) string {
	if h := w.House(id); h != nil {
		return h.Name
	}
	return "the street"
}

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
