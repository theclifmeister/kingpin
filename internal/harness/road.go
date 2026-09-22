package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/law"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
	"github.com/theclifmeister/kingpin/internal/sim/territory"
)

// road is what the policies that run the route share (#275): the
// route into home, the hub it starts from, and the sims they read,
// built once with the policy. Distributor, Delegated and Boss are its
// steps composed in one order: the dial, the target, the hub (else
// the laundered day), the spending, the crew, the restock and the
// sales, the boss with the war after all of it. It was one function,
// distribute, with a flag for handing home over and one for fighting,
// each read in half a dozen places; the steps are the same code in
// the same order, so every draw and every pinned number is where it
// was. None of the sims keeps anything between days.
type road struct {
	cfg       *content.Config
	laundered Policy
	ld        *laundering.Sim
	tr        *territory.Sim
	lw        *law.Sim
	cs        *crew.Sim
	rv        *rivals.Sim
	lg        *logistics.Sim
	dip       content.DiplomacyTuning
	hot       func(*game.World) bool
	home      string
	// The route into home with the most room, from the city that sells
	// by the lot; the hub is where it starts. nil (and "") with none,
	// and the policy is Laundered.
	route *content.RouteConfig
	hub   string
	// homeCorners is how many corners at home the player's own runners
	// work before any go to the hub.
	homeCorners int
}

func newRoad(cfg *content.Config, lieLowAt float64) *road {
	d := &road{
		cfg:       cfg,
		laundered: Laundered(cfg, lieLowAt),
		ld:        laundering.New(cfg),
		tr:        territory.New(cfg),
		lw:        law.New(cfg),
		cs:        crew.New(cfg),
		rv:        rivals.New(cfg),
		dip:       cfg.Rivals.Diplomacy,
		lg:        logistics.New(cfg),
		home:      cfg.City.Home().ID,
	}
	for _, c := range cfg.City.Cities {
		if !c.Wholesale || c.ID == d.home {
			continue
		}
		for _, r := range d.lg.Routes(c.ID) {
			if r.From == c.ID && r.To == d.home && (d.route == nil || r.Capacity > d.route.Capacity) {
				rc := r
				d.route = &rc
			}
		}
	}
	if d.route != nil {
		d.hub = d.route.From
	}
	d.homeCorners = max(1, cfg.Crew.Crew.MaxCrew/2)
	d.hot = TooHot(cfg, lieLowAt)
	return d
}

// roadCrew is how a road policy staffs itself: what used to be
// distribute's two flags, spelled out.
type roadCrew struct {
	// delegate hires the first lieutenant looking for work ahead of
	// anyone else (the least skilled runner making room) and hands them
	// home; personality, if set, is what they turn out to be.
	delegate    bool
	personality string
	// fireViolent fires a lieutenant whose standing orders show the
	// aggressive dial, the morning they show (the boss).
	fireViolent bool
	// guards is how many enforcers the policy signs once the rival is
	// about and two runners are on: one, or the boss's third of the
	// roster.
	guards int
	// hubCorners is how many hub corners the player's own runners work
	// once home is handed over.
	hubCorners int
}

// trader is Distributor and Delegated: the road, the fair spending and
// the crew as rc says, every product restocked and sold.
func (d *road) trader(rc roadCrew) Policy {
	return func(w *game.World) {
		if d.route == nil {
			d.laundered(w)
			return
		}
		d.dialOn(w)
		if !d.arrive(w, d.target(w, DistributorDays, nil)) {
			return
		}
		d.spendFair(w)
		delegated := d.crew(w, rc)
		restock(d.cfg, w)
		d.sell(w, delegated)
	}
}

// boss is BossAt: the road with the pipeline kept full and only what
// is worth the heat on it, the boss's spending, the crew with the
// boss's guards and a violent lieutenant fired, and the war (or the
// table) after everything else, whatever the day came to.
func (d *road) boss(personality string, margin float64) Policy {
	rc := roadCrew{
		delegate:    true,
		personality: personality,
		fireViolent: true,
		guards:      max(1, d.cfg.Crew.Crew.MaxCrew/3),
		hubCorners:  HubCorners,
	}
	// The boss works every corner in the hub, the lieutenant's people
	// being their own.
	if d.hub != "" {
		rc.hubCorners = len(d.cfg.City.City(d.hub).Corners)
	}
	return func(w *game.World) {
		if d.route == nil {
			d.laundered(w)
			return
		}
		defer war(w, d.rv, d.dip, d.hot)
		d.dialOn(w)
		// The boss keeps the pipeline full: the road takes days, and a
		// target under that many days of demand starves home between
		// landings.
		days := max(float64(DistributorDays), float64(d.lg.Days(w, *d.route, w.Route(d.route.ID).Dial.Ship())+2))
		worthIt := func(id string) bool { return worth(d.cfg, w, id) }
		if !d.arrive(w, d.target(w, days, worthIt)) {
			return
		}
		d.spendBoss(w, margin)
		delegated := d.crew(w, rc)
		restockOnly(d.cfg, w, worthIt)
		d.sell(w, delegated)
	}
}

// dialOn turns the route's dial to normal, once; it is left there.
func (d *road) dialOn(w *game.World) {
	if !w.Route(d.route.ID).Dial.On() {
		_ = w.SetRoute(d.route.ID, events.RouteNormal)
	}
}

// target sets the route's target, refreshed as home's corners come and
// go: days of home's demand for every product the hub's wholesaler
// sells under DistributorMargin of home's street (and, with only, that
// only passes), nothing for the rest. It returns the wholesaler.
func (d *road) target(w *game.World, days float64, only func(string) bool) *game.Supplier {
	wholesale := w.WholesaleSupplier(d.hub)
	for _, id := range w.Products {
		homeP := w.Product(d.home, id)
		target := 0
		if wholesale != nil && homeP != nil && wholesale.Price[id] > 0 && wholesale.Price[id] <= homeP.Price*DistributorMargin && (only == nil || only(id)) {
			target = int(days * w.Demand(d.home, id))
		}
		_ = w.SetRouteTarget(d.route.ID, id, target)
	}
	return wholesale
}

// arrive plays the laundered day while the wholesaler will not deal
// and reports false; once it deals it goes to the hub and reports true.
func (d *road) arrive(w *game.World, wholesale *game.Supplier) bool {
	if wholesale == nil || wholesale.Locked(w) {
		d.laundered(w)
		return false
	}
	if w.Player.Location != d.hub {
		_ = w.Travel(d.hub)
	}
	return true
}

// spendFair is the distributor's spending: the laundered fronts, the
// tree at three times the price (the stash spots are what a lot needs
// room for) and fair pay.
func (d *road) spendFair(w *game.World) {
	washUp(d.ld, w)
	BuyUpgrades(d.cfg, w, 3)
	w.SetPay(events.PayFair)
}

// spendBoss is the boss's spending, at a thinner margin (the pile is
// what draws the police at this scale, and a front or a node is where
// it goes), with generous pay: wages are noise against the takings,
// and the loyalty is what keeps a ten-strong roster from firing itself
// one a day. margin is the levels' (BossAt).
func (d *road) spendBoss(w *game.World, margin float64) {
	washUpAt(d.ld, w, BossMargin)
	BuyUpgrades(d.cfg, w, BossMargin)
	w.SetPay(events.PayGenerous)
	// The boss pays the town (#193): the reform ticket in every city
	// when a campaign opens, and goodwill wherever the pressure is up,
	// as Funded does where it stands.
	backReform(d.cfg, w)
	for _, cid := range w.CityOrder {
		payTown(d.cfg, w, w.Cities[cid])
	}
	// Then the property (#194): the block under a corner of its own,
	// cheapest first, one a day, over the same campaign's worth, and
	// never past what the DA lets the washed figure explain (the
	// forfeiture's line, DeedLimit).
	BuyDeed(d.tr, w, BossMargin, d.cfg.Law.Campaign.Fill(), d.lw.DeedLimit(w))
	// Then the businesses (#192): the levels take what the town and the
	// property left, over a campaign's worth kept in hand for the next
	// election, so the boss's civic spending is what it was.
	InvestOver(d.ld, w, margin, d.cfg.Law.Campaign.Fill())
	// And a lot a day offshore (#195) over the same reserve, never over
	// the line and never retiring: its numbers are the horizon's.
	ReserveLot(d.ld, w, d.cfg.Law.Campaign.Fill())
}

// crew is the road's roster: runners, and enforcers for home once the
// rival is about. Whoever has sunk to skimming goes, one a day. A
// delegating policy hires the first lieutenant looking for work ahead
// of anyone else and hands them home; while one is on the payroll it
// leaves home to them. It reports whether home is handed over.
func (d *road) crew(w *game.World, rc roadCrew) bool {
	home, hub := d.home, d.hub
	fireSkimmer(w, d.cfg.Crew.Crew)
	var lt *game.CrewMember
	for i := range w.Crew.Members {
		if w.Crew.Members[i].Lieutenant() {
			lt = &w.Crew.Members[i]
		}
	}
	// The boss reads the market screen: a lieutenant whose standing
	// orders are at the aggressive dial is a violent one, and runs the
	// city into an arrest the first night a shipment lands, so they go
	// the morning the orders show, before those resolve. (A careful one
	// sells half of what the corners take, but firing them costs more
	// than they do: the loyalty, their people and the wait for the next
	// one looking for work.)
	if rc.fireViolent && lt != nil && len(w.Crew.FiredToday) == 0 {
		for _, id := range w.Products {
			if o, ok := w.StandingOrder(home, id); ok && o.Dial == events.DialAggressive {
				_, _ = w.Fire(lt.ID)
				lt = nil
				break
			}
		}
	}
	want := game.RoleRunner
	if w.Rival().Arrived > 0 && w.Crew.Role(game.RoleEnforcer) < rc.guards && w.Crew.Runners() >= 2 {
		want = game.RoleEnforcer
	}
	if rc.delegate && lt == nil {
		for _, c := range w.Crew.Candidates {
			if c.Lieutenant() {
				want = game.RoleLieutenant
			}
		}
		// A full roster makes room for them: the least skilled runner
		// goes, and the lieutenant's people more than make up for it.
		if want == game.RoleLieutenant && len(w.Crew.Members) >= d.cs.MaxCrew(w) && len(w.Crew.FiredToday) == 0 {
			fireWorstRunner(w)
		}
	}
	if best := bestCandidate(w, want); best >= 0 && len(w.Crew.Members) < d.cs.MaxCrew(w) {
		if c := w.Crew.Candidates[best]; w.Player.DirtyCash >= c.Fee+d.cfg.Market.Market.StartCash {
			if m, err := w.Hire(c.ID, d.cs.MaxCrew(w)); err == nil && m.Lieutenant() {
				lt = w.Crew.Member(m.ID)
				if rc.personality != "" {
					lt.Personality = rc.personality
				}
			}
		}
	}
	if lt != nil && lt.City != home {
		_ = w.Assign(lt.ID, home)
	}
	// A lieutenant whose loyalty is sliding toward the flip line is paid
	// off before they get there: the player who reads the roster keeps
	// the one person who knows everything sweet.
	if lt != nil && lt.Loyalty < d.cs.FlipLine()+10 && len(w.Crew.PaidOffToday) == 0 {
		if cost := d.cs.PayoffCost(*lt); w.Player.DirtyCash >= cost+d.cfg.Market.Market.StartCash {
			_, _ = w.PayOff(lt.ID, cost, d.cs.PayoffLoyalty())
		}
	}
	delegated := lt != nil
	// Arriving with the whole crew posted at home, one runner comes off
	// the smallest home corner to work here: the route needs somebody on
	// this end, and a lieutenant only comes looking once corners are
	// held in both cities.
	if !delegated && w.WorkedIn(hub) == 0 && w.WorkedIn(home) > d.homeCorners {
		if c := pickCorner(w, func(c game.Corner) bool { return c.City == home && c.Worked() && c.Runner != game.You }, func(c game.Corner) float64 { return -c.Demand }); c != nil {
			w.Recall(c.Runner)
		}
	}
	// Idle runners take corners at home until homeCorners are worked,
	// then here; the enforcer guards the home corner the rival borders.
	// With home handed over, hubCorners runners work here, the rest wait
	// for the lieutenant, enforcer included, and whoever the lieutenant
	// had no corner for last night works here after all.
	for _, m := range w.Crew.Members {
		if w.PostOf(m.ID) != nil {
			continue
		}
		if delegated && (m.Role != game.RoleRunner || (w.WorkedIn(hub) >= rc.hubCorners && m.Hired == w.Day)) {
			continue
		}
		switch m.Role {
		case game.RoleRunner:
			// Home first, until homeCorners are worked or the rival has
			// left nothing to work; then here.
			for _, city := range []string{home, hub} {
				if city == home && (delegated || w.WorkedIn(home) >= d.homeCorners) {
					continue
				}
				c := pickCorner(w, func(c game.Corner) bool { return c.City == city && c.Held() && c.Runner == 0 }, size)
				if c == nil {
					c = pickCorner(w, func(c game.Corner) bool { return c.City == city && c.Owner == game.OwnerNone }, size)
				}
				if c != nil {
					_ = w.Post(c.ID, m.ID)
					break
				}
			}
		case game.RoleEnforcer:
			if c := pickCorner(w, func(c game.Corner) bool { return c.City == home && c.Worked() && c.Enforcer == 0 }, guardScore(w)); c != nil {
				_ = w.Post(c.ID, m.ID)
			}
		}
	}
	return delegated
}

// sell places the day's sales: everything at home (unless home is
// handed over) and everything in the hub. Heat anywhere over the line
// is a day off instead.
func (d *road) sell(w *game.World, delegated bool) {
	if d.hot(w) {
		w.SetLieLow(true)
		return
	}
	for _, id := range w.Products {
		if q := w.Stock(d.home, id); q > 0 && !delegated {
			_ = w.PlaceSell(d.home, id, q, events.DialNormal)
		}
		if q := w.Stock(d.hub, id); q > 0 {
			_ = w.PlaceSell(d.hub, id, q, events.DialNormal)
		}
	}
}

// war is the boss's answer to the rival, after the day's trading is
// queued: the enforcers against the rival's biggest corner at push when
// a corner is contested, heat is under the line and the odds are over
// BossOdds; else the diplomat's table, a truce proposed whenever a
// corner was lost in the last DiplomatDays and any truce offered taken.
func war(w *game.World, rv *rivals.Sim, dip content.DiplomacyTuning, hot func(*game.World) bool) {
	r := Nearest(w)
	if r.Arrived == 0 {
		return
	}
	for _, o := range w.Offers {
		if o.Deal.Kind == game.DealTruce {
			_, _ = w.Accept(o.ID)
		}
	}
	contested := false
	for _, c := range w.Home().Corners {
		if c.Owner == game.OwnerPlayer && w.Contested(c) {
			contested = true
		}
	}
	// A push when the odds clear the line; otherwise it talks. A strike
	// under a deal would be a betrayal, so never at peace. (A hit at the
	// same line wins corners and loses the run: 6.6 strikes a run took 4
	// corners, brought 23 crackdowns over 20 seeds and indicted 4 of
	// them, for a lower median at the horizon than talking.)
	if contested && !w.AtPeaceWith(r.Faction()) && !hot(w) && rv.Odds(w, r, events.ForcePush) >= BossOdds {
		if c := pickCorner(w, func(c game.Corner) bool { return c.FactionID() == r.Faction() }, size); c != nil {
			_ = w.SendEnforcers(c.ID, events.ForcePush)
			return
		}
	}
	if w.AtPeaceWith(r.Faction()) || w.Today.Proposal != nil || r.LastFlip == 0 || w.Day-r.LastFlip > DiplomatDays {
		return
	}
	_ = w.ProposeTo(r.Faction(), game.DealTruce, game.Terms{Days: dip.TruceDays[1]})
}
