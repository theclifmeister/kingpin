package crew

import (
	"math"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Lieutenants run a city for the player (Phase 3.3). Everything here is
// a policy over the actions the player has, run inside the crew step for
// each city that has one: nothing new is simulated, only who decides.
// The cut and the greedy skim come off the day's takings before wages;
// a flip is the turning rule with a higher line and no dice; a walk is
// the quit rule taking the city with it; and delegation, after the
// roster has settled, is the posting, the abandoning and the standing
// orders for tomorrow.

// LieutenantsWanted reports whether lieutenants come looking for work:
// once the player holds corners in two cities there is a city to hand
// over.
// The crew screen's pool title reads it through the sim (#148).
func (s *Sim) LieutenantsWanted(w *game.World) bool { return LieutenantsWanted(w) }

func LieutenantsWanted(w *game.World) bool {
	cities := 0
	for _, cid := range w.CityOrder {
		for _, c := range w.Cities[cid].Corners {
			if c.Held() {
				cities++
				break
			}
		}
	}
	return cities >= 2
}

// RevealDays is how long a lieutenant has to run a city before the
// report names their temperament.
func (s *Sim) RevealDays() int { return s.cfg.Lieutenant.RevealDays }

// FlipLine is the loyalty under which a lieutenant turns informant.
func (s *Sim) FlipLine() float64 { return s.cfg.Lieutenant.Flip }

// Cut is the share of their city's takings a lieutenant keeps.
func (s *Sim) Cut() float64 { return s.cfg.Role[game.RoleLieutenant].Cut }

// Dial is the sell dial a lieutenant's temperament favours.
func (s *Sim) Dial(m game.CrewMember) events.Dial {
	return s.cfg.Lieutenant.Temper(m.Personality).SellDial()
}

// take is the lieutenants' cut of today's takings in their cities, off
// dirty cash now, and what a greedy one skims on top, whatever their
// loyalty. It returns the skim, capped by what is left, for the caller to
// take with the day's other missing money, and starts the report entry
// for each lieutenant.
func (s *Sim) take(w *game.World, t *game.Tick, acted map[int]*events.LieutenantActed) int {
	cut := s.Cut()
	revenue := map[int]int{}
	for _, e := range t.Events() {
		if ps, ok := e.(events.PlayerSold); ok && ps.Lieutenant != 0 {
			revenue[ps.Lieutenant] += ps.Revenue
		}
	}
	skimmed := 0
	for _, cid := range w.CityOrder {
		lt := w.Crew.Lieutenant(cid)
		if lt == nil {
			continue
		}
		ev := &events.LieutenantActed{Day: t.Day, ID: lt.ID, Name: lt.Name, City: cid, CityName: w.CityName(cid), Dial: s.Dial(*lt)}
		acted[lt.ID] = ev
		ev.Revenue = revenue[lt.ID]
		ev.Cut = min(int(math.Round(float64(ev.Revenue)*cut)), w.Player.DirtyCash)
		w.Player.DirtyCash -= ev.Cut
		w.Stats.Cuts += ev.Cut
		if share := s.cfg.Lieutenant.Temper(lt.Personality).Skim; share > 0 {
			ev.Skimmed = min(int(math.Round(float64(ev.Revenue)*share)), w.Player.DirtyCash-skimmed)
			skimmed += ev.Skimmed
		}
	}
	return skimmed
}

// walk is a lieutenant leaving with the city they ran: every corner the
// player holds there (bar the one the player stands on) goes to the
// rival if it holds ground in that city, else back to the street, and
// the stash there is gone. The caller drops them from the roster.
//
// The hand-over is the one write into w.Rival outside the rivals sim
// (#144, the exception TestRivalStateHasOneWriter lists): the corners
// change hands tonight, since the news sim's card triggers read the
// rival's ground this same tick and the morning's map shows it, and
// the flip's bookkeeping (Flips, LastFlip, Observed) goes with them,
// since the boss and the diplomat read LastFlip in the morning, before
// the rivals sim could stamp it off the event a step later.
func (s *Sim) walk(w *game.World, t *game.Tick, lt game.CrewMember) {
	city := w.Cities[lt.City]
	ev := events.LieutenantWalked{Day: t.Day, ID: lt.ID, Name: lt.Name, City: lt.City, CityName: w.CityName(lt.City)}
	if city != nil {
		toRival := city == w.Home() && w.RivalHeld() > 0
		for i := range city.Corners {
			c := &city.Corners[i]
			if !c.Held() || c.Runner == game.You {
				continue
			}
			ev.Corners = append(ev.Corners, c.Name)
			c.Runner, c.Enforcer, c.Idle, c.Squeeze, c.Robbed, c.Since = 0, 0, 0, 0, 0, t.Day
			if toRival {
				c.Owner, c.Faction = game.OwnerRival, w.Rival.Faction()
				w.Rival.Flips++
				w.Rival.LastFlip = t.Day
				w.Stats.CornersLost++
			} else {
				c.Owner, c.Faction = game.OwnerNone, ""
			}
		}
		if toRival {
			ev.Rival, ev.Faction = w.Rival.Leader, w.Rival.Faction()
			w.Rival.Observed = true
		}
		for id, q := range w.StashOf(lt.City) {
			ev.Units += w.TakeStock(lt.City, id, q)
		}
	}
	w.Stats.Walked++
	w.DropStanding(lt.City)
	t.Emit(ev)
}

// delegate is a lieutenant's night in their city: the crew come off a
// corner robbed twice, idle runners go on the best corners (a held one
// nobody works, else the biggest free one), idle enforcers guard the
// worked ones if the temperament bothers, whatever they cannot staff is
// given up, and every product in the stash gets a standing order at
// their dial for tomorrow. Nothing here rolls dice.
func (s *Sim) delegate(w *game.World, t *game.Tick, lt *game.CrewMember, ev *events.LieutenantActed) {
	city := w.Cities[lt.City]
	if city == nil {
		return
	}
	tp := s.cfg.Lieutenant.Temper(lt.Personality)
	split := w.Deal(game.DealSplit)
	keep := func(c *game.Corner) bool { // a corner the lieutenant leaves alone
		return c.Runner == game.You || (split != nil && c.City == w.Home().ID && split.Covers(c.ID))
	}

	// 1. A corner robbed twice is not worth the stock: the crew come off.
	for i := range city.Corners {
		c := &city.Corners[i]
		if !c.Held() || c.Robbed < 2 || keep(c) {
			continue
		}
		w.Recall(c.Runner)
		w.Recall(c.Enforcer)
	}

	// 2. Idle runners onto the best corners, biggest first; Post refuses
	// a corner the split gives the rival, so the next one is tried.
	byDemand := func(ok func(c *game.Corner) bool) []*game.Corner {
		var out []*game.Corner
		for i := range city.Corners {
			if c := &city.Corners[i]; c.Robbed < 2 && ok(c) {
				out = append(out, c)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Demand > out[j].Demand })
		return out
	}
	post := func(id int, ok func(c *game.Corner) bool) *game.Corner {
		for _, c := range byDemand(ok) {
			claim := !c.Held()
			if w.Post(c.ID, id) != nil {
				continue
			}
			if claim {
				c.Since = t.Day // tonight's claim, reported by the territory sim in the morning
			}
			return c
		}
		return nil
	}
	for i := range w.Crew.Members {
		m := &w.Crew.Members[i]
		if m.Role != "runner" || w.PostOf(m.ID) != nil {
			continue
		}
		c := post(m.ID, func(c *game.Corner) bool { return c.Held() && c.Runner == 0 })
		if c == nil {
			c = post(m.ID, func(c *game.Corner) bool { return c.Owner == game.OwnerNone })
		}
		if c != nil {
			ev.Posted = append(ev.Posted, c.Name)
		}
	}

	// 3. Enforcers, if the temperament bothers: the contested corners
	// first, then the riskiest.
	if tp.Guard {
		for i := range w.Crew.Members {
			m := &w.Crew.Members[i]
			if m.Role != "enforcer" || w.PostOf(m.ID) != nil {
				continue
			}
			var best *game.Corner
			score := func(c *game.Corner) float64 {
				if w.Contested(*c) {
					return 10 + c.Demand
				}
				return c.Risk
			}
			for j := range city.Corners {
				c := &city.Corners[j]
				if c.Worked() && c.Enforcer == 0 && c.Robbed < 2 && (best == nil || score(c) > score(best)) {
					best = c
				}
			}
			if best == nil {
				break
			}
			if w.Post(best.ID, m.ID) == nil {
				ev.Guarded = append(ev.Guarded, best.Name)
			}
		}
	}

	// 4. What they cannot staff they give up, rather than let it sit.
	for i := range city.Corners {
		c := &city.Corners[i]
		if !c.Held() || c.Runner != 0 || keep(c) {
			continue
		}
		if w.Abandon(c.ID) == nil {
			ev.Dropped = append(ev.Dropped, c.Name)
		}
	}

	// 5. Standing orders for tomorrow: everything in the stash, at
	// their dial. The player's own order for a product wins the day.
	for _, key := range delegatedKeys(w, lt.City) {
		delete(w.Delegated, key)
	}
	for _, id := range w.Products {
		if q := w.Stock(lt.City, id); q > 0 {
			w.Delegate(lt.City, id, q, ev.Dial)
			ev.Orders++
		}
	}

	// You learn what they are like by watching them work.
	if !lt.Observed && t.Day-lt.Assigned >= s.cfg.Lieutenant.RevealDays {
		lt.Observed = true
		ev.Revealed = true
	}
	if lt.Observed {
		ev.Personality = lt.Personality
	}

	// 6. The buy side (#174): a supply contract for every product a
	// connect here sells, at the temper's stock_days of the demand the
	// worked corners serve, shared out by demand where the levels
	// together would overfill the stash (the way the stocked player
	// shares the bag). The market sim fills it in the morning at the
	// contract markup, for cash, where the player has set no contract
	// of their own and never on a lie-low day; the standing orders count
	// on what it brings (World.SupplyDue, cut to the room), as a sell
	// order may. What this morning's contracts bought is the report's.
	s.restock(w, t, lt, city, tp, ev)
}

// restock is the lieutenant's buy side (#174): see delegate, step 6.
func (s *Sim) restock(w *game.World, t *game.Tick, lt *game.CrewMember, city *game.City, tp content.LieutenantPersonality, ev *events.LieutenantActed) {
	for _, key := range delegatedSupplyKeys(w, lt.City) {
		delete(w.DelegatedSupply, key)
	}
	sold := func(id string) bool { // a connect here deals in it, and the door is open
		for _, sup := range w.SuppliersIn(lt.City) {
			if sup.Sells(id) && !sup.Locked(w) {
				return true
			}
		}
		return false
	}
	levels := map[string]float64{}
	total, want := 0.0, 0.0
	served := game.FoldEffects(w, s.tree).DemandMul // what the corners serve under the tree, as the market serves an order
	for _, id := range w.Products {
		m := city.Market[id]
		if m == nil || m.NoSupply || tp.StockDays <= 0 || !sold(id) {
			continue
		}
		levels[id] = tp.StockDays * w.Demand(lt.City, id) * served
		total += m.Demand
		want += levels[id]
	}
	room := float64(w.Capacity(lt.City))
	for _, id := range w.Products {
		level, ok := levels[id]
		if !ok {
			continue
		}
		if want > room && total > 0 {
			level = math.Min(level, room*city.Market[id].Demand/total)
		}
		if units := int(level); units > 0 {
			w.DelegateSupply(lt.City, id, units)
			ev.Contracts++
		}
	}
	for _, id := range w.Products {
		due := min(w.SupplyDue(lt.City, id), w.Free(lt.City))
		if due <= 0 {
			continue
		}
		if q := w.Stock(lt.City, id); q == 0 {
			ev.Orders++ // nothing stashed tonight, so step 5 placed none
		}
		w.Delegate(lt.City, id, w.Stock(lt.City, id)+due, ev.Dial)
	}
	for _, e := range t.Events() {
		if sb, ok := e.(events.SupplyBought); ok && sb.City == lt.City && sb.Lieutenant == lt.Name {
			ev.Bought = append(ev.Bought, events.Bought{Product: sb.Product, Units: sb.Units, Cost: sb.Cost})
		}
	}
}

// delegatedSupplyKeys lists the lieutenant's supply contracts in a
// city, in a fixed order.
func delegatedSupplyKeys(w *game.World, city string) []string {
	var keys []string
	for k, c := range w.DelegatedSupply {
		if c.City == city {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// delegatedKeys lists the standing orders in a city, in a fixed order.
func delegatedKeys(w *game.World, city string) []string {
	var keys []string
	for k, o := range w.Delegated {
		if o.City == city {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}
