package heat

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// cool is the decay, in every city. Cold contacts make both the base
// rate and lying low better; a zealous chief makes it worse. A feared
// name never quite cools: decay works on what is above the floor, and
// nothing takes heat under it. It sets the day's floor, which the close
// clamps to again.
func (s *Sim) cool(d *day) {
	w, t := d.w, d.t
	decay := s.Decay(w)
	if t.Day < w.Heat.FederalUntil && w.Heat.FederalDecay > 0 {
		decay *= w.Heat.FederalDecay // the feds are in town (#44): what you draw, you keep
	}
	if w.Today.LieLow {
		decay *= math.Max(d.tun.LieLowMultiplier, d.fx.LieLowMultiplier)
		t.Emit(events.LaidLow{Day: t.Day})
		for _, cid := range w.CityOrder {
			d.reasons[cid] = append(d.reasons[cid], "lay low")
		}
	}
	d.floor = s.Floor(w)
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		if c.Heat > d.floor {
			c.Heat -= (c.Heat - d.floor) * decay
		}
		c.Heat = max(d.floor, min(100, c.Heat))
	}
}

// sellCap counts a patrol's cap down a day and lifts it at zero.
func (s *Sim) sellCap(d *day) {
	h := d.h
	if h.SellCapDays > 0 {
		h.SellCapDays--
		if h.SellCapDays == 0 {
			h.SellCap = 0
		}
	}
}

// respond is the threshold responses, highest first, one per day, from
// the police of the hottest city (d.hot); what they take comes out of
// the stash there.
func (s *Sim) respond(d *day) {
	w, t, h, fx := d.w, d.t, d.h, d.fx
	if h.LastResponse == nil {
		h.LastResponse = map[string]int{}
	}
	if h.Responses == nil {
		h.Responses = map[string]int{}
	}
	hot := s.hottest(w)
	d.hot = hot
	resp := s.Thresholds()
	// The favour called in this morning (#228): the chief's people stand
	// down tonight. The one response that would have fired does not
	// (nothing taken, no heat drop, no sweep, not counted), its cooldown
	// starts as if it had, and RaidFellThrough says so; heat is
	// untouched, so the same rung is there when the cooldown lifts. The
	// arrest is the DA's and no chief stops it. fire rolls nothing on
	// the home stream, so a run without a favour is the run before.
	favour := w.Law.FavourOwed == t.Day
	// A task force announced yesterday comes tonight (#48), whatever the
	// heat: it formed, and it acts. The day's one response is its.
	if h.TaskForceDay > 0 && h.TaskForceDay < t.Day {
		h.TaskForceDay = 0
		if r := s.rung(content.TaskForce); r != nil {
			if favour {
				t.Emit(events.RaidFellThrough{Day: t.Day, City: hot.ID, Level: r.Level, Evidence: s.law.Bribes.FavourEvidence})
			} else {
				h.Responses[r.Level]++
				s.fire(w, t, hot, *r, d.attempted[hot.ID], fx)
			}
			h.LastResponse[r.Level] = t.Day
			h.WatchUntil = t.Day + s.CooldownDays(w, r.Level)
		}
		resp = nil
	}
	// An investigation due tonight lands (#343): the sting on its target
	// alone, and the night's one response. A favour called in stands it
	// down as it would the sting, and its cooldown starts tonight. Behind
	// a task force it waits a night. The sting's cooldown otherwise ran
	// from the night it opened: the police were at work all along, and
	// can open the next the night after this one lands.
	if inv := h.Investigation; resp != nil && inv.Open() && t.Day >= inv.Due {
		if r := s.rung(content.Sting); r != nil {
			if favour {
				h.Investigation = game.Investigation{}
				h.LastResponse[r.Level] = t.Day
				t.Emit(events.RaidFellThrough{Day: t.Day, City: inv.City, Level: r.Level, Evidence: s.law.Bribes.FavourEvidence})
				t.Emit(events.InvestigationClosed{Day: t.Day, City: inv.City, Lead: inv.Kind, Target: inv.Target, Name: w.LeadName(inv.Kind, inv.Target), Fell: true})
			} else {
				s.strike(d, *r)
			}
		}
		resp = nil
	}
	for i := len(resp) - 1; i >= 0; i-- {
		r := resp[i]
		if hot.Heat < s.Threshold(w, r, hot) {
			continue
		}
		if r.Level == content.TaskForce && !s.TaskForceEligible(w) {
			continue // nobody without an asset or the pile meets the feds (#48): the ladder is the four rungs it was
		}
		if last, ok := h.LastResponse[r.Level]; ok && t.Day-last < s.CooldownDays(w, r.Level) && r.Level != content.Arrest {
			continue
		}
		if r.Level == content.Sting && s.Investigating() && h.Investigation.Open() {
			continue // the sting is the investigation's while one runs (#343): the rung under it still answers
		}
		if r.Level == content.TaskForce {
			// Announced a day ahead (#48): the morning's news, and the
			// day's response. It comes tomorrow night.
			h.TaskForceDay = t.Day
			t.Emit(events.TaskForceFormed{Day: t.Day, City: hot.ID, Assets: len(w.Assets)})
			break
		}
		if favour && r.Level != content.Arrest {
			t.Emit(events.RaidFellThrough{Day: t.Day, City: hot.ID, Level: r.Level, Evidence: s.law.Bribes.FavourEvidence})
			h.LastResponse[r.Level] = t.Day
			break
		}
		if r.Level == content.Sting && s.Investigating() && s.open(d, hot) {
			// A named target instead of a blind sting (#343), the rung's
			// cooldown running from tonight; with nothing to name, the
			// blind one.
			h.LastResponse[r.Level] = t.Day
			break
		}
		h.Responses[r.Level]++
		s.fire(w, t, hot, r, d.attempted[hot.ID], fx)
		h.LastResponse[r.Level] = t.Day
		break
	}
}

// Hottest is the city whose police answer tonight, for the favour's
// dialog (#228).
func (s *Sim) Hottest(w *game.World) *game.City { return s.hottest(w) }

// hottest is the city whose police answer today: the hottest, and where
// the player is when it is a tie.
func (s *Sim) hottest(w *game.World) *game.City {
	best := w.Here()
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c.Heat > best.Heat {
			best = c
		}
	}
	return best
}

// fire applies one response in a city. attempted says whether the player
// tried to sell there today: a sting or raid that turns up on a day
// nothing moved still costs stock and cash and cools heat, but finds
// nothing worth a file. Dirty cash draws attention; only dealing builds a
// case. The Security branch softens what a response takes; a lawyer thins
// what goes in the file. A sting or a raid hits one place (#73, place):
// a house the police know about, else one house or the street by the
// heat of its block, else the street. A raid while an informant is on
// the payroll goes straight to the fullest house, which they know, and
// takes every unit in it, whatever the safehouse would have saved; with
// no house it is the whole street, as it always was.
func (s *Sim) fire(w *game.World, t *game.Tick, city *game.City, r content.ResponseConfig, attempted bool, fx game.Effects) {
	ev := events.Enforcement{Day: t.Day, City: city.ID, Level: r.Level, StockLost: map[string]int{}}
	switch r.Level {
	case content.Patrol:
		w.Heat.SellCapDays = r.CapDays
		w.Heat.SellCap = s.PatrolCap(w, r, city)
	case content.Arrest:
		if s.takeFall(w, t, fx) {
			return
		}
		w.Over = w.End(s.exit(content.CauseArrested, fx), t.Day, "")
		t.Emit(ev)
		t.Emit(events.GameOver{Day: t.Day, Cause: w.Over.Cause})
		return
	default: // sting, raid, task force
		// What the rung takes, folded as Rungs shows it (bite): the
		// feds take the file's numbers (#48) and an asset besides
		// (seize, below).
		b := bite(r, fx)
		stockLoss, cashLoss := b.StockLoss, b.CashLoss
		told := r.Level == content.Raid && w.Crew.Informants() > 0
		if told {
			stockLoss, ev.Stash = 1, true
		}
		if house := s.place(w, t, city.ID, told); house != nil {
			ev.House, ev.HouseName = house.ID, house.Name
			for _, id := range w.SortedProducts() {
				if lost := w.TakeFromHouse(house.ID, id, int(math.Round(float64(house.Stock[id])*stockLoss))); lost > 0 {
					ev.StockLost[id] = lost
					w.Stats.HouseUnits += lost
				}
			}
			house.Raided++
			t.Emit(events.HouseRaided{Day: t.Day, House: house.ID, Name: house.Name, City: city.ID, Level: r.Level, Whole: told, StockLost: ev.StockLost})
			if !house.Known && len(ev.StockLost) > 0 {
				house.Known = true
				t.Emit(events.HouseCompromised{Day: t.Day, House: house.ID, Name: house.Name, City: city.ID, Why: "bust"})
			}
		} else {
			for id, q := range w.StreetOf(city.ID) {
				if lost := w.TakeStreet(city.ID, id, int(math.Round(float64(q)*stockLoss))); lost > 0 {
					ev.StockLost[id] = lost
				}
			}
		}
		ev.CashLost = int(math.Round(float64(w.Player.DirtyCash) * cashLoss))
		w.Player.DirtyCash -= ev.CashLost
		switch r.Level {
		case content.Raid:
			w.Stats.Raids++
		case content.TaskForce:
			w.Stats.TaskForces++
			s.seize(w, t, city)
		default:
			w.Stats.Stings++
		}
		// Who stood where when the police came (#46): the corners in
		// the city your crew worked or guarded and the crew on them,
		// stamped for the crew sim to roll the arrests over in the
		// morning (it steps before this one). You are never on the list:
		// your own arrest is the ladder's top rung.
		sweep := game.Sweep{Day: t.Day, City: city.ID, Level: r.Level}
		for _, c := range city.Corners {
			if !c.Held() || (c.Runner <= 0 && c.Enforcer <= 0) {
				continue
			}
			sweep.Corners = append(sweep.Corners, c.ID)
			for _, id := range []int{c.Runner, c.Enforcer} {
				if id > 0 {
					sweep.Crew = append(sweep.Crew, id)
				}
			}
		}
		for _, n := range ev.StockLost {
			sweep.Units += n
		}
		ev.Corners = sweep.Corners
		w.Heat.Sweep = sweep
		// The record the market sim reads the next morning (#72): a
		// bust that took product is held against you by the connect
		// in that city. Kept a month, like the seizures.
		units := 0
		for _, n := range ev.StockLost {
			units += n
		}
		if units > 0 {
			w.Heat.Busts = append(w.Heat.Busts, game.Bust{Day: t.Day, City: city.ID, Level: r.Level, Units: units})
			kept := w.Heat.Busts[:0]
			for _, b := range w.Heat.Busts {
				if t.Day-b.Day <= s.cfg.Heat.BustDays {
					kept = append(kept, b)
				}
			}
			w.Heat.Busts = kept
		}
	}
	if attempted {
		ev.Evidence = bite(r, fx).Evidence
		if ev.Evidence > 0 {
			w.Heat.Evidence += ev.Evidence
			w.Heat.EvidenceDay = t.Day
		}
	}
	// The first raid convinces them they got you. The second one does not.
	// Each repeat of the same response cools things down less.
	n := float64(max(1, w.Heat.Responses[r.Level]))
	city.Heat -= r.HeatDrop / n
	t.Emit(ev)
}

// seize is the task force taking an asset (#48): one a firing, the one
// in the city it came to if any, else the costliest owned; gone, not
// frozen. AssetSeized goes out beside the Enforcement and the
// laundering sim, which owns the assets and steps after this one,
// takes it off the books on the event (as it does the tunnel the
// police found, TunnelFound). Nothing with nothing owned: a task force
// formed on the pile alone takes the stock and the cash a raid does.
// No dice.
func (s *Sim) seize(w *game.World, t *game.Tick, city *game.City) {
	pick := -1
	for i, a := range w.Assets {
		switch {
		case pick < 0:
			pick = i
		case a.City == city.ID && w.Assets[pick].City != city.ID:
			pick = i
		case (a.City == city.ID) == (w.Assets[pick].City == city.ID) && a.Cost > w.Assets[pick].Cost:
			pick = i
		}
	}
	if pick < 0 {
		return
	}
	a := w.Assets[pick]
	t.Emit(events.AssetSeized{Day: t.Day, City: city.ID, Asset: a.ID, Name: a.Name, Cost: a.Cost})
}

// place is the one place a sting or a raid in a city hits (#73): nil for
// the street. An informant on the payroll (told) points the police at
// the fullest house, which becomes Known; then the fullest house they
// know about; then, with houses there holding anything, one roll off the
// houses' side stream over every place holding stock, each house
// weighted by the heat of its block and the street by a standard
// corner's, so a cheap empty house on a quiet block never shields the
// street. With no house holding anything the street it is, no roll: a
// run with no house draws nothing the old run did not.
func (s *Sim) place(w *game.World, t *game.Tick, city string, told bool) *game.House {
	if told {
		if h := w.Fullest(city, false); h != nil && !h.Known {
			h.Known = true
			t.Emit(events.HouseCompromised{Day: t.Day, House: h.ID, Name: h.Name, City: city, Why: "informant"})
		}
	}
	if h := w.Fullest(city, true); h != nil {
		return h
	}
	type candidate struct {
		house  *game.House
		weight float64
	}
	var cands []candidate
	total := 0.0
	for i := range w.Houses {
		h := &w.Houses[i]
		if h.City != city || h.Units() == 0 {
			continue
		}
		weight := s.RaidWeight(w, h)
		cands = append(cands, candidate{h, weight})
		total += weight
	}
	if len(cands) == 0 {
		return nil
	}
	if w.Player.StockIn(city) > 0 {
		cands = append(cands, candidate{nil, 1})
		total++
	}
	if len(cands) == 1 {
		return cands[0].house
	}
	roll := t.Sub(game.StreamHouses).Float64() * total
	for _, c := range cands {
		roll -= c.weight
		if roll < 0 {
			return c.house
		}
	}
	return cands[len(cands)-1].house
}
