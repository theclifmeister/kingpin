package engine

import (
	"math"
	"reflect"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The day's preview (#353): before you end the day, what tonight will
// roughly cost and earn, and what is left idle. It is built from the
// estimators the screens already read (the sell dialog's take and heat,
// the supply contracts' plan, the wages, the wash and its upkeep, the
// road's lots and fares) in the order the night moves the money, and it
// never writes the world (TestPreviewNeverWritesTheWorld). Every figure
// is an estimate: the dice (robberies, the police, an audit, a skim,
// tomorrow's prices) are what it cannot know, and it says so (Unknown).

// PreviewUnknown is what a preview leaves out, as ids a front end words:
// the corners and houses robbed, the police, the night's price moves,
// an audit, a skim, and the rivals and the crew's own nights.
var PreviewUnknown = []string{"robbery", "police", "prices", "audit", "skim", "rivals", "crew"}

// DayPreview is tonight, estimated. Flow is the money in #351's cash
// flow's categories (game.FlowCats), so the morning's report can be set
// beside it: its opening is the piles now, each line what the night is
// expected to move by pile, and its closing the projected piles. What
// the day already paid (the cart's buys, Bought) is in the piles it
// opens on, not in the lines.
type DayPreview struct {
	Day     int           `json:"day"`               // the night previewed: the morning it makes
	LieLow  bool          `json:"lie_low,omitempty"` // lying low: nothing sells
	Flow    FlowView      `json:"flow"`
	Bought  int           `json:"bought"`  // the cash the day's buys by hand already cost
	Sales   []SaleView    `json:"sales"`   // per city where anything sells or is handed over tonight
	Idle    []IdleView    `json:"idle"`    // runners and enforcers on no corner and guarding no house: the unposted alerts
	Corners []CornerIdle  `json:"corners"` // corners you hold that nobody works tonight: the idle_corner alerts
	Alerts  []Alert       `json:"alerts"`  // everything that needs you this morning, each with its act (#352)
	Unknown []string      `json:"unknown"` // PreviewUnknown
	Supply  []SupplyView  `json:"supply"`  // the supply contracts' buys due tonight
	Routes  RoutesOutlay  `json:"routes"`  // the road's lots and fares
	Wages   int           `json:"wages"`   // tonight's wages at the pay dial
	Wash    LaunderingDay `json:"wash"`    // the fronts and the assets
}

// SaleView is a city's night on the street, estimated: the units sold
// and handed over to a buyer, the take after the crew's and the
// lieutenant's cuts, and the heat the sales add.
type SaleView struct {
	City      string  `json:"city"`
	Units     int     `json:"units"`
	Delivered int     `json:"delivered,omitempty"` // units handed over on a buyer's contract
	Take      int     `json:"take"`
	Heat      float64 `json:"heat"`
}

// IdleView is a member who earns or guards nothing tonight: a runner
// or an enforcer, fit, on no corner and at no house.
type IdleView struct {
	Member int    `json:"member"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// CornerIdle is a corner you hold that nobody works, with the days
// before it drifts back to the street (the idle_corner alert's).
type CornerIdle struct {
	Corner string `json:"corner"`
	City   string `json:"city"`
	Days   int    `json:"days"`
}

// SupplyView is one supply contract's buy tonight, by the plan.
type SupplyView struct {
	City    string `json:"city"`
	Product string `json:"product"`
	Units   int    `json:"units"`
	Cost    int    `json:"cost"`
}

// RoutesOutlay is what the road spends tonight: whole lots at the
// source and the fares.
type RoutesOutlay struct {
	Lots  int `json:"lots"`
	Fares int `json:"fares"`
}

// LaunderingDay is the night's wash: what goes through the fronts, their
// upkeep and income, and the assets' upkeep.
type LaunderingDay struct {
	Washed int `json:"washed"`
	Upkeep int `json:"upkeep"`
	Income int `json:"income"`
	Assets int `json:"assets"`
}

// Preview is tonight, estimated (#353): nil before a run and once it is
// over. It reads the world and never writes it.
func (s *Session) Preview() *DayPreview {
	w := s.w
	if w == nil || w.Over != nil {
		return nil
	}
	p := &DayPreview{Day: w.Day + 1, LieLow: w.Today.LieLow, Unknown: PreviewUnknown}
	// at is the world with the cash as the night will have moved it so
	// far: the estimators that read the till (the contracts' budget, the
	// road's, the wash) read it here. It is a copy of the struct, and
	// only its Player (a value) is written.
	at := *w
	lines := map[string]game.Pools{}
	book := func(cat string, dirty, clean int) {
		l := lines[cat]
		l.Dirty += dirty
		l.Clean += clean
		lines[cat] = l
		at.Player.DirtyCash += dirty
		at.Player.CleanCash += clean
	}
	take := func(cat string, cost int) { // dirty first, then clean, as World.TakeCash
		n := min(max(cost, 0), at.Player.DirtyCash+at.Player.CleanCash)
		dirty := min(n, at.Player.DirtyCash)
		book(cat, -dirty, -(n - dirty))
	}
	for _, b := range w.Today.Buys {
		if !b.Contract && !b.Credit {
			p.Bought += b.Cost
		}
	}

	// The market: the debts due, the supply contracts' buys, the
	// handoffs and the sales, the contracts that fall short.
	for _, sup := range w.Suppliers {
		if sup.Debt > 0 && sup.DebtDue <= p.Day {
			take(game.FlowPurchases, sup.Debt)
		}
	}
	due := map[string]int{}
	for _, sp := range s.set.Market.Plan(&at) {
		if sp.Units <= 0 {
			continue
		}
		due[game.OrderKey(sp.Contract.City, sp.Contract.Product)] += sp.Units
		p.Supply = append(p.Supply, SupplyView{City: sp.Contract.City, Product: sp.Contract.Product, Units: sp.Units, Cost: sp.Cost})
		book(game.FlowPurchases, -sp.Cost, 0)
	}
	stock := func(city, product string) int { return w.Stock(city, product) + due[game.OrderKey(city, product)] }
	taken := map[string]int{}
	byLt := map[string]int{} // a city's revenue, for its lieutenant's cut
	if !w.Today.LieLow {
		for _, cid := range w.CityOrder {
			sv := SaleView{City: cid}
			for _, c := range w.Contracts {
				m := w.Product(cid, c.Product)
				if c.City != cid || !c.Live(w.Day) || m == nil {
					continue
				}
				key := game.OrderKey(cid, c.Product)
				units := min(w.Today.Deliveries[c.ID], c.Owed(), stock(cid, c.Product)-taken[key])
				if units <= 0 {
					continue
				}
				revenue := int(math.Round(m.Price * c.Premium * s.set.Market.QualityMul(w.Quality(cid, c.Product)) * float64(units)))
				taken[key] += units
				sv.Delivered += units
				sv.Take += revenue
				book(game.FlowSales, revenue, 0)
			}
			for _, id := range w.SortedProducts() {
				o, standing, ok := game.SellOrder{}, false, false
				if o, ok = w.Order(cid, id); !ok {
					if o, ok = w.YourStanding(cid, id); ok {
						standing = true
					} else {
						o, ok = w.DelegatedOrder(cid, id)
					}
				}
				if !ok {
					continue
				}
				key := game.OrderKey(cid, id)
				have := stock(cid, id) - taken[key]
				if standing {
					o.Qty = min(o.Qty, have)
					if o.Qty <= 0 {
						continue
					}
				}
				sold, revenue := s.set.Market.Estimate(w, cid, o, have)
				taken[key] += sold
				kept := 0
				if standing {
					kept = min(int(math.Round(float64(revenue)*s.set.Market.Cut())), at.Player.DirtyCash+revenue)
				}
				book(game.FlowSales, revenue-kept, 0)
				byLt[cid] += revenue
				sv.Units += sold
				sv.Take += revenue - kept
				sv.Heat += s.set.Heat.SaleHeat(w, cid, id, o.Qty, o.Dial) + s.set.Heat.SloppyHeat(w, cid, sold)
			}
			if sv.Units > 0 || sv.Delivered > 0 || sv.Heat > 0 {
				p.Sales = append(p.Sales, sv)
			}
		}
	}
	for _, c := range w.Contracts {
		if c.Status != game.ContractAccepted || c.Due >= p.Day {
			continue
		}
		owed := c.Owed() - min(w.Today.Deliveries[c.ID], c.Owed())
		if m := w.Product(c.City, c.Product); m != nil && owed > 0 {
			take(game.FlowLosses, int(math.Round(c.PenaltyCash*float64(owed)*m.Price)))
		}
	}

	// The road: lots and fares out of what is over the float.
	moved := map[string]int{} // the stashes as the night leaves them for the road
	for k, n := range due {
		moved[k] += n
	}
	for k, n := range taken {
		moved[k] -= n
	}
	p.Routes.Lots, p.Routes.Fares = s.set.Logistics.Outlay(&at, moved)
	book(game.FlowRoutes, -(p.Routes.Lots + p.Routes.Fares), 0)

	// The street: the blocks' rent in, the houses' rent out, the tax.
	for _, c := range w.Deeds() {
		book(game.FlowInvestments, 0, s.set.Territory.DeedRent(c.Deed))
	}
	for _, cid := range w.CityOrder {
		if _, amount := s.set.Territory.TaxDue(w, cid); amount > 0 {
			book(game.FlowTax, amount, 0)
		}
	}
	for _, h := range w.Houses {
		if h.Bought < w.Day && h.Rent > 0 && h.Rent <= at.Player.CleanCash {
			book(game.FlowRoutes, 0, -h.Rent)
		}
	}

	// The crew: the lieutenants' cut of their cities' takings, then the
	// wages at the dial, as far as the dirty cash goes.
	for _, cid := range w.CityOrder {
		if w.Crew.Lieutenant(cid) != nil && byLt[cid] > 0 {
			cut := min(int(math.Round(float64(byLt[cid])*s.set.Crew.Cut())), at.Player.DirtyCash)
			book(game.FlowSales, -cut, 0)
			for i := range p.Sales {
				if p.Sales[i].City == cid {
					p.Sales[i].Take -= cut
				}
			}
		}
	}
	if len(w.Crew.Members) > 0 {
		p.Wages = s.set.Crew.Wages(w, w.Crew.Pay)
		book(game.FlowWages, -min(p.Wages, at.Player.DirtyCash), 0)
	}

	// The wash: front by front, what is over the float goes through,
	// the upkeep comes out of the clean pile as it stands and the levels
	// earn; then the assets' upkeep.
	lw := s.set.Laundering
	for _, f := range w.Fronts {
		if f.Frozen(p.Day) {
			continue
		}
		amt := min(lw.Throughput(w, f), lw.Washable(&at))
		if amt > 0 {
			book(game.FlowLaundering, -amt, amt)
			p.Wash.Washed += amt
		}
		due := lw.FrontUpkeep(w, f)
		if due > at.Player.CleanCash {
			continue
		}
		inc := lw.Income(f)
		book(game.FlowLaundering, 0, inc-due)
		p.Wash.Upkeep += due
		p.Wash.Income += inc
	}
	for _, a := range w.Assets {
		if a.Upkeep <= 0 || a.Frozen(p.Day) || a.Upkeep > at.Player.CleanCash {
			continue
		}
		book(game.FlowLaundering, 0, -a.Upkeep)
		p.Wash.Assets += a.Upkeep
	}

	opening := game.Pools{Dirty: w.Player.DirtyCash, Clean: w.Player.CleanCash}
	f := game.NewCashFlow(p.Day, lines, game.Pools{Dirty: at.Player.DirtyCash, Clean: at.Player.CleanCash})
	f.Opening = opening // NewCashFlow works it back to the same piles
	p.Flow = flowView(f, s.cfg.Headlines.Flow.BigShare)

	// What needs you: every alert this morning, each with the act that
	// answers it (#352); the idle crew and the corners nobody works are
	// the unposted and idle_corner ones among them, summed up.
	p.Alerts = s.Alerts()
	for _, a := range p.Alerts {
		switch a.Kind {
		case AlertUnposted:
			if m := w.Crew.Member(a.Member); m != nil {
				p.Idle = append(p.Idle, IdleView{Member: m.ID, Name: m.Name, Role: m.Role})
			}
		case AlertIdleCorner:
			p.Corners = append(p.Corners, CornerIdle{Corner: a.Corner, City: a.City, Days: a.Days})
		}
	}
	noNulls(reflect.ValueOf(p).Elem()) // a list is [] on the wire, never null (#333)
	return p
}
