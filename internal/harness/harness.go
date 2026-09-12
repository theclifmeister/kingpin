// Package harness runs the game headless under a scripted policy so balance
// can be measured and invariants tested without a terminal.
package harness

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
	"github.com/theclifmeister/kingpin/internal/sim/heat"
	"github.com/theclifmeister/kingpin/internal/sim/laundering"
	"github.com/theclifmeister/kingpin/internal/sim/law"
	"github.com/theclifmeister/kingpin/internal/sim/logistics"
	"github.com/theclifmeister/kingpin/internal/sim/rivals"
)

// Policy decides the player's actions for the coming day.
type Policy func(w *game.World)

// Chooser answers a dilemma card: the index of the choice to take.
type Chooser func(w *game.World, c *game.Card) int

// Decline takes the last choice on every card, which by the deck's
// convention is what happens when the player does nothing: the passive
// outcome a player who never reads the card would get.
func Decline(_ *game.World, c *game.Card) int { return len(c.Choices) - 1 }

// First takes the first choice on every card: the active one.
func First(*game.World, *game.Card) int { return 0 }

// Result summarises a headless run.
type Result struct {
	Days     int
	Over     *game.Ending
	PeakCash int
	EndCash  int
	NetWorth []int // net worth at the end of each day played, oldest first
	Events   []events.Event
	World    *game.World
}

// NetWorthAt is the net worth at the end of day d, or at the end of the run
// if it finished sooner.
func (r Result) NetWorthAt(d int) int {
	if len(r.NetWorth) == 0 {
		return 0
	}
	return r.NetWorth[min(d, len(r.NetWorth))-1]
}

// Run plays up to days days from a fresh world with the given seed. The
// dilemma deck and the incident table stay in the box: cards are
// choices, a scripted player has none to make, incidents are weather
// (#44), and the invariants the harness pins belong to the other sims.
// RunWith deals the cards; Play deals either.
func Run(cfg *content.Config, seed uint64, days int, policy Policy) (Result, error) {
	return RunFrom(cfg, sim.NewWorld(cfg, seed), days, policy)
}

// RunFrom plays up to days days on from w, which the caller may have set
// up (a cash pile, a crew) to test a situation a fresh run takes a while
// to reach. No cards are dealt and no incidents; see Run.
func RunFrom(cfg *content.Config, w *game.World, days int, policy Policy) (Result, error) {
	return Play(cfg, w, days, policy, Options{})
}

// RunWith plays like RunFrom with the deck in play: every card is answered
// with pick before the policy acts, the way the UI shows the card before
// the day starts. The incident table stays boxed.
func RunWith(cfg *content.Config, w *game.World, days int, policy Policy, pick Chooser) (Result, error) {
	if pick == nil {
		pick = Decline
	}
	return Play(cfg, w, days, policy, Options{Cards: pick})
}

// Options is what a run deals beyond the sims' invariants: the dilemma
// deck, answered by Cards (nil boxes it), and the incident table (#44),
// dealt when Incidents is set. Both are off by default so a pinned
// number reads the sims alone; cmd/balance turns either on.
type Options struct {
	Cards     Chooser
	Incidents bool
}

// Play plays up to days days on from w with what opts deals in play and
// the rest boxed: the general form of Run, RunFrom and RunWith.
func Play(cfg *content.Config, w *game.World, days int, policy Policy, opts Options) (Result, error) {
	boxed := *cfg
	if opts.Cards == nil {
		boxed.Dilemmas.Cards = nil
	}
	if !opts.Incidents {
		boxed.Incidents.Table = nil
	}
	return run(&boxed, w, days, policy, opts.Cards)
}

func run(cfg *content.Config, w *game.World, days int, policy Policy, pick Chooser) (Result, error) {
	_, sims, err := sim.Default(cfg)
	if err != nil {
		return Result{}, err
	}
	clock := game.NewClock(nil, sims...)
	var all []events.Event
	var worth []int
	for d := 0; d < days && w.Over == nil; d++ {
		if c := w.Dilemmas.Pending; c != nil {
			if pick == nil {
				pick = Decline
			}
			_, _ = w.Choose(pick(w, c))
		}
		if policy != nil {
			policy(w)
		}
		all = append(all, clock.EndDay(w)...)
		worth = append(worth, w.NetWorth())
	}
	return Result{Days: w.Day, Over: w.Over, PeakCash: w.Stats.PeakCash, EndCash: w.Cash(), NetWorth: worth, Events: all, World: w}, nil
}

// Idle does nothing; prices drift on their own.
func Idle(*game.World) {}

// Hide lies low every day and never trades: the player who has built
// something and sits on it.
func Hide(w *game.World) { w.SetLieLow(true) }

// Trader restocks every product it can afford and sells everything it holds
// at the given dial, every day. It is deliberately greedy. It buys before it
// queues the sales so stock bought in the morning is on the street the same
// night, the way a player who turns the bag over daily plays.
func Trader(cfg *content.Config, dial events.Dial) Policy {
	return func(w *game.World) {
		standSomewhere(w)
		restock(cfg, w)
		sellEverything(w, dial)
	}
}

// standSomewhere puts you on a corner in the city you are in if you are on
// none: a held one nobody works, else the biggest free one. Nothing sells
// from nowhere.
func standSomewhere(w *game.World) {
	if w.PostOf(game.You) != nil {
		return
	}
	here := w.Player.Location
	if c := pickCorner(w, func(c game.Corner) bool { return c.City == here && c.Held() && c.Runner == 0 }, size); c != nil {
		_ = w.Post(c.ID, game.You)
	} else if c := pickCorner(w, func(c game.Corner) bool { return c.City == here && c.Owner == game.OwnerNone }, size); c != nil {
		_ = w.Post(c.ID, game.You)
	}
}

// size scores a corner by its demand.
func size(c game.Corner) float64 { return c.Demand }

// restock buys from the supplier where you are toward a demand-
// proportional mix that fits what the stash there can hold, so a crashed
// product never hogs the whole bag.
func restock(cfg *content.Config, w *game.World) {
	restockOnly(cfg, w, func(string) bool { return true })
}

// restockOnly is restock over the products ok passes: the bag is shared
// out by demand among those alone.
func restockOnly(cfg *content.Config, w *game.World, ok func(product string) bool) {
	pressure := cfg.Market.Market.BuyPricePressure * game.FoldEffects(w, cfg.Upgrades).BuyPressureMul
	city := w.Here()
	// Tribute is paid tonight out of what is left after the buying: a
	// player who owes it keeps it aside.
	reserve := 0
	if d := w.Deal(game.DealTribute); d != nil {
		reserve = d.Terms.PerDay
	}
	total := 0.0
	for _, id := range w.Products {
		if ok(id) && !city.Market[id].NoSupply {
			total += city.Market[id].Demand
		}
	}
	for _, id := range w.Products {
		if !ok(id) || city.Market[id].NoSupply {
			continue
		}
		m := city.Market[id]
		// The bag is the street's (#73): a house is where the bag is
		// kept, not a bigger bag, so a policy with houses holds what
		// the same policy without them holds and keeps it off the
		// street. With no house StreetCapacity is Capacity.
		target := int(float64(w.StreetCapacity(city.ID)) * m.Demand / total)
		buyHere(w, id, target-w.Stock(city.ID, id), reserve, pressure, false)
	}
}

// buyHere buys up to qty units of a product where you stand from the
// cheapest connect that sells it today (#72): as many as the cash over
// reserve (at the connect's quote, small-lot premium and all), the
// stash's room and the connect's day allow. On credit it takes what
// the connect will run first, at their credit price, and pays cash for
// the rest. It reports how many it bought.
func buyHere(w *game.World, product string, qty, reserve int, pressure float64, credit bool) int {
	city := w.Player.Location
	sup := retail(w, city, product)
	if sup == nil || qty <= 0 {
		return 0
	}
	bought := 0
	if credit && sup.Credit() > 0 {
		n := min(qty, w.Free(city), sup.Left())
		n = afford(sup, product, n, sup.Credit(), true)
		if n > 0 {
			if p, err := w.Buy(sup.ID, product, n, true, pressure); err == nil {
				bought += p.Qty
			}
		}
	}
	n := min(qty-bought, w.Free(city), sup.Left())
	n = afford(sup, product, n, w.Player.DirtyCash-reserve, false)
	if n > 0 {
		if p, err := w.Buy(sup.ID, product, n, false, pressure); err == nil {
			bought += p.Qty
		}
	}
	return bought
}

// retail is the cheapest connect in a city that sells you a product
// today by the unit: the wholesaler's lots are the road's (#61), and a
// scripted player who bought them by hand for the corners there would
// be measuring a second road, not the street; the tier rows are the
// road's.
func retail(w *game.World, city, product string) *game.Supplier {
	var best *game.Supplier
	for _, s := range w.SuppliersIn(city) {
		if s.Wholesale || !w.Available(s, product) {
			continue
		}
		if best == nil || s.Price[product] < best.Price[product] {
			best = s
		}
	}
	return best
}

// afford is the most of n units of a product a connect's quote lets
// cash cover, cash or on credit: the plain price first, and the
// small-lot premium once the buy is under the lot.
func afford(sup *game.Supplier, product string, n, cash int, credit bool) int {
	unit := sup.Price[product]
	if unit <= 0 || n <= 0 || cash <= 0 {
		return 0
	}
	if credit {
		unit *= sup.CreditRatio
	}
	n = min(n, int(float64(cash)/unit))
	if n < sup.Lot && sup.SmallLot > 1 {
		n = min(n, int(float64(cash)/(unit*sup.SmallLot)))
	}
	for n > 0 && sup.Quote(product, n, credit) > cash {
		n--
	}
	return n
}

// sellEverything queues every stash for sale at the dial: the runners sell
// where they stand, you sell where you are.
func sellEverything(w *game.World, dial events.Dial) {
	for _, cid := range w.CityOrder {
		for _, id := range w.Products {
			if q := w.Stock(cid, id); q > 0 {
				_ = w.PlaceSell(cid, id, q, dial)
			}
		}
	}
}

// Careful trades quietly and lies low whenever heat climbs.
func Careful(cfg *content.Config, lieLowAt float64) Policy {
	trade := Trader(cfg, events.DialQuiet)
	hot := TooHot(cfg, lieLowAt)
	return func(w *game.World) {
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		trade(w)
	}
}

// Managed sells at the normal dial and lies low whenever heat reaches
// lieLowAt. It is the baseline for "a player who pays attention".
func Managed(cfg *content.Config, lieLowAt float64) Policy {
	trade := Trader(cfg, events.DialNormal)
	hot := TooHot(cfg, lieLowAt)
	return func(w *game.World) {
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		trade(w)
	}
}

// TooHot reports whether a policy that lies low at line on the ladder as
// heat.toml prints it should lie low today. The line moves with the
// sting threshold (#41: the DA moves it, and the dashboard shows where it
// stands), so a player who pays attention keeps the same distance under
// it: at 40 on a ladder whose sting is 55, they lie low at 36 once the
// sting line is 50. And once the DA's file is within two pages of an
// indictment (the dashboard paints it red) they keep RedMargin more.
// The hottest city is the one whose police answer, so its line is the
// one read.
func TooHot(cfg *content.Config, line float64) func(w *game.World) bool {
	hs := heat.New(cfg)
	var sting *content.ResponseConfig
	for i := range cfg.Heat.Responses {
		if cfg.Heat.Responses[i].Level == content.Sting {
			sting = &cfg.Heat.Responses[i]
		}
	}
	return func(w *game.World) bool {
		at := line
		if sting != nil && sting.Threshold > 0 {
			at = line * hs.Threshold(w, *sting, hottest(w)) / sting.Threshold
		}
		if arrest := hs.EvidenceArrest(w); arrest > 0 && w.Heat.Evidence >= arrest-2 {
			at -= RedMargin
		}
		return w.MaxHeat() >= at
	}
}

// RedMargin is how much lower a policy that pays attention lies low once
// the DA's file is two pages from an indictment.
const RedMargin = 5

// hottest is the city whose police answer today: the hottest, and where
// the player is when it is a tie.
func hottest(w *game.World) *game.City {
	best := w.Here()
	for _, cid := range w.CityOrder {
		if c := w.Cities[cid]; c.Heat > best.Heat {
			best = c
		}
	}
	return best
}

// Upgraded plays like Crewed and spends on the tree: whenever it can pay
// three times the price of the cheapest node it can buy, it buys it,
// Security first, then Operations, then Legal. It is the baseline for "a
// player who invests instead of reinvesting every dollar in stock", and
// since #117 it is the crewed player, the tree's customer: a lone
// trader who buys every node pays for insurance and for crew, front and
// road nodes it cannot use, so buying everything is a trap for one by
// design, and the tree is measured on the operation it is for
// (TestUpgradedBeatsCrewed).
func Upgraded(cfg *content.Config, lieLowAt float64) Policy {
	crewed := Crewed(cfg, lieLowAt)
	return func(w *game.World) {
		BuyUpgrades(cfg, w, 3)
		crewed(w)
	}
}

// TreeOrder is the order BuyUpgrades walks the branches: Security,
// Operations, Legal, then the branches of #118, Crew and Laundering,
// then #119's Street and Logistics.
var TreeOrder = []string{"security", "operations", "legal", "crew", "laundering", "street", "logistics"}

// BuyUpgrades buys, in TreeOrder, the cheapest node the player can buy
// from the right pool with margin times its cost in hand, one per call.
// The Laundering branch waits for a front, the way an accountant does:
// a player with nothing to wash has no use for a bookkeeper (#118: the
// crewed player buying it paid $300k for nothing), and the policies that
// launder buy it as soon as they own one. The Logistics branch (#119)
// waits for a route to be on the same way: its nodes do nothing for a
// player who never runs the road, and the crewed player never does.
func BuyUpgrades(cfg *content.Config, w *game.World, margin float64) {
	for _, branch := range TreeOrder {
		if (branch == "laundering" && len(w.Fronts) == 0) || (branch == "logistics" && !routeOn(w)) {
			continue
		}
		var pick *content.UpgradeConfig
		for _, n := range cfg.Upgrades.Branch(branch) {
			if w.Owns(n.ID) || len(w.Missing(n)) > 0 {
				continue
			}
			if pick == nil || n.Cost < pick.Cost {
				u := n
				pick = &u
			}
		}
		if pick == nil {
			continue
		}
		have := w.Player.DirtyCash
		if pick.Clean {
			have = w.Player.CleanCash
		}
		if float64(have) >= margin*float64(pick.Cost) {
			_, _ = w.BuyUpgrade(cfg.Upgrades, pick.ID)
			return
		}
	}
}

// routeOn reports whether any route's dial is on.
func routeOn(w *game.World) bool {
	for _, rs := range w.Routes {
		if rs.Dial.On() {
			return true
		}
	}
	return false
}

// Own grants upgrades for free, prerequisites and all in the order given,
// so a test can start a run that already has them. It panics on an id
// the tree does not have or a node whose prerequisites are not owned.
func Own(cfg *content.Config, w *game.World, ids ...string) {
	for _, id := range ids {
		u := cfg.Upgrades.Upgrade(id)
		if u == nil {
			panic("harness.Own: no upgrade " + id)
		}
		if u.Clean {
			w.Player.CleanCash += u.Cost
		} else {
			w.Player.DirtyCash += u.Cost
		}
		if _, err := w.BuyUpgrade(cfg.Upgrades, id); err != nil {
			panic("harness.Own: " + err.Error())
		}
	}
	w.Today.UpgradesToday = nil // a grant is not a purchase to report
}

// Crewed plays like Managed but builds a crew: it pays fair, signs the most
// skilled runner on offer whenever it can afford the fee with cash to
// spare, posts every runner on the best free corner, and replaces anyone
// whose loyalty has sunk to where they skim.
func Crewed(cfg *content.Config, lieLowAt float64) Policy {
	return Territory(cfg, lieLowAt, 0)
}

// Territory plays like Crewed but works at most corners corners (0 means
// as many as it can staff), counting the one you stand on, and spends the
// crew slots it has left on enforcers for the corners most likely to be
// robbed. Once the rival is in town it keeps a couple of enforcers on the
// corners it borders, whatever else it is doing. It is the baseline for
// "a player who takes ground"; it never sends them in. It stays in the
// city it is in: Distributor is the one that spreads out.
func Territory(cfg *content.Config, lieLowAt float64, corners int) Policy {
	managed := Managed(cfg, lieLowAt)
	return func(w *game.World) {
		staff(cfg, w, w.Player.Location, corners)
		managed(w)
	}
}

// staff hires and posts the crew for one city, the way Territory plays:
// runners until corners corners there are worked (0 means every corner),
// then enforcers for them; with a rival about, enforcers come first once
// one runner is on. Runners and enforcers already posted elsewhere are
// left where they are. The roster cap is the crew sim's (#118: the
// tree's crew_slots count, so a policy that buys the Crew branch fills
// the room it bought).
func staff(cfg *content.Config, w *game.World, city string, corners int) {
	tun := cfg.Crew.Crew
	if corners <= 0 {
		corners = len(cfg.City.City(city).Corners)
	}
	guards := max(1, tun.MaxCrew/3)
	maxCrew := crew.New(cfg).MaxCrew(w)
	w.SetPay(events.PayFair)
	for _, m := range w.Crew.Members {
		if m.Loyalty < tun.SkimThreshold {
			_, _ = w.Fire(m.ID)
			break // one a day; each firing sours the rest
		}
	}
	// Runners until the corners are staffed, then enforcers for them;
	// with a rival about, enforcers come first once one runner is on.
	runners, worked := 0, w.WorkedIn(city)
	for _, m := range w.Crew.Members {
		if m.Role != "runner" {
			continue
		}
		if p := w.PostOf(m.ID); p == nil || p.City == city {
			runners++
		}
	}
	want := "runner"
	if runners+1 >= corners {
		want = "enforcer"
	}
	if n := w.Crew.Role("enforcer"); want == "enforcer" && n >= min(corners, worked) {
		want = ""
	}
	if w.Rival.Arrived > 0 && w.Crew.Runners() >= 1 && w.Crew.Role("enforcer") < guards {
		want = "enforcer"
		// A full roster of runners makes room: the least skilled goes.
		if len(w.Crew.Members) >= maxCrew && len(w.Crew.FiredToday) == 0 {
			worst := -1
			for i, m := range w.Crew.Members {
				if m.Role == "runner" && (worst < 0 || m.Skill < w.Crew.Members[worst].Skill) {
					worst = i
				}
			}
			if worst >= 0 {
				_, _ = w.Fire(w.Crew.Members[worst].ID)
			}
		}
	}
	best := -1
	for i, c := range w.Crew.Candidates {
		if c.Role != want {
			continue
		}
		if best < 0 || c.Skill > w.Crew.Candidates[best].Skill {
			best = i
		}
	}
	if best >= 0 && len(w.Crew.Members) < maxCrew {
		c := w.Crew.Candidates[best]
		if w.Player.DirtyCash >= c.Fee+cfg.Market.Market.StartCash {
			_, _ = w.Hire(c.ID, maxCrew)
		}
	}
	// Every idle runner takes back a held corner nobody is working,
	// else the biggest free one, up to the cap; every idle enforcer
	// guards the riskiest unguarded one.
	for _, m := range w.Crew.Members {
		if w.PostOf(m.ID) != nil {
			continue
		}
		switch m.Role {
		case "runner":
			if w.WorkedIn(city) >= corners {
				continue
			}
			c := pickCorner(w, func(c game.Corner) bool { return c.City == city && c.Held() && c.Runner == 0 }, size)
			if c == nil {
				c = pickCorner(w, func(c game.Corner) bool { return c.City == city && !c.Held() && c.Owner != game.OwnerRival }, size)
			}
			if c != nil {
				_ = w.Post(c.ID, m.ID)
			}
		case "enforcer":
			// The corners the rival borders first, then the riskiest.
			score := func(c game.Corner) float64 {
				if w.Contested(c) {
					return 10 + c.Demand
				}
				return c.Risk
			}
			if c := pickCorner(w, func(c game.Corner) bool { return c.City == city && c.Worked() && c.Enforcer == 0 }, score); c != nil {
				_ = w.Post(c.ID, m.ID)
			}
		}
	}
}

// Vigilant plays like Crewed and hunts the snitch: whenever the report's
// tell has shown (Heat.Leaks, what the alerts panel hints on) and nobody
// is asking already, it pays for an investigation, and it fires whoever
// one names. It is the baseline for "a player who reads the report".
func Vigilant(cfg *content.Config, lieLowAt float64) Policy {
	crewed := Crewed(cfg, lieLowAt)
	inf := cfg.Crew.Informant
	return func(w *game.World) {
		if m := w.Crew.Member(w.Crew.Exposed); m != nil {
			_, _ = w.Fire(m.ID)
		}
		crewed(w)
		if w.Heat.Leaks >= 2 && w.Today.Investigation == nil && len(w.Crew.FiredToday) == 0 {
			_ = w.Investigate(inf.InvestigateCost)
		}
	}
}

// Plant hires the most skilled runner looking for work, free, and turns
// them: an informant on the payroll from day 0, so a test can measure
// what one costs without waiting for one to turn. It panics if nobody is
// looking for work.
func Plant(cfg *content.Config, w *game.World) game.CrewMember {
	best := -1
	for i, c := range w.Crew.Candidates {
		if c.Role == "runner" && (best < 0 || c.Skill > w.Crew.Candidates[best].Skill) {
			best = i
		}
	}
	if best < 0 {
		best = 0
	}
	if len(w.Crew.Candidates) == 0 {
		panic("harness.Plant: nobody looking for work")
	}
	c := w.Crew.Candidates[best]
	w.Player.DirtyCash += c.Fee
	m, err := w.Hire(c.ID, cfg.Crew.Crew.MaxCrew)
	if err != nil {
		panic("harness.Plant: " + err.Error())
	}
	w.Crew.HiredToday = nil // a plant is not a signing to report
	w.Crew.Member(m.ID).Informant = true
	return *w.Crew.Member(m.ID)
}

// Warlike plays like Territory but fights the rival: whenever it has an
// enforcer and heat is under lieLowAt it sends the enforcers against the
// rival's biggest corner at force, every day, and re-posts a runner on
// whatever it wins. It is the baseline for "a player who goes to war".
func Warlike(cfg *content.Config, lieLowAt float64, corners int, force events.Force) Policy {
	territory := Territory(cfg, lieLowAt, corners)
	hot := TooHot(cfg, lieLowAt)
	return func(w *game.World) {
		territory(w)
		if hot(w) || w.Crew.Role("enforcer") == 0 {
			return
		}
		if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, size); c != nil {
			_ = w.SendEnforcers(c.ID, force)
		}
	}
}

// RivalBooks is the rival's day as the rivals sim keeps it: what its
// corners earn it today (rivals.Sim.Income, a price war's squeeze off)
// and what its muscle costs it (rivals.Sim.Wages, the wage in the
// ladder's unit since #139). It is what says whether money can hurt it:
// the muscle is what the take pays for, so a price war (#68) that cuts
// the take is a head fewer, and the chest (Rival.Cash) is what a drain
// takes and what a hire or a claim needs.
func RivalBooks(cfg *content.Config, w *game.World) (income, wages int) {
	rv := rivals.New(cfg)
	return rv.Income(w), rv.Wages(w)
}

// Pricewar plays like Territory and fights with money (#68): every day
// it can, it undercuts the biggest rival corner next to one it works,
// at the dial, instead of sending the enforcers in; it never strikes.
// It is the baseline for "a player who starves the rival out".
func Pricewar(cfg *content.Config, lieLowAt float64, corners int, dial events.Dial) Policy {
	territory := Territory(cfg, lieLowAt, corners)
	return func(w *game.World) {
		territory(w)
		if w.Today.LieLow {
			return
		}
		if c := pickCorner(w, func(c game.Corner) bool { return w.CanUndercut(c.ID) == nil }, size); c != nil {
			_ = w.Undercut(c.ID, dial)
		}
	}
}

// Outbidder plays like Territory and answers the tell (#69): the
// morning the rival is eyeing a free corner it posts a runner there,
// an idle one if it has one, else the one on its smallest worked
// corner, so the claim finds somebody on it. It never sends the
// enforcers in. It is the scripted player the tell is for.
func Outbidder(cfg *content.Config, lieLowAt float64, corners int) Policy {
	territory := Territory(cfg, lieLowAt, corners)
	return func(w *game.World) {
		territory(w)
		Outbid(w)
	}
}

// Outbid posts a runner on the corner the rival is eyeing, if it is
// still free and there is a runner to post: an idle one, else the one
// on the smallest worked corner. It reports whether somebody was put
// there.
func Outbid(w *game.World) bool {
	eyed := w.Corner(w.Rival.Eyeing)
	if eyed == nil || eyed.Owner != game.OwnerNone {
		return false
	}
	who := 0
	worst := 0.0
	for _, m := range w.Crew.Members {
		if m.Role != "runner" {
			continue
		}
		p := w.PostOf(m.ID)
		if p == nil {
			who = m.ID
			break
		}
		if who == 0 || p.Demand < worst {
			who, worst = m.ID, p.Demand
		}
	}
	if who == 0 {
		return false
	}
	return w.Post(eyed.ID, who) == nil
}

// Diplomat plays like Territory and talks: whenever the rival has taken a
// corner off it in the last DiplomatDays it proposes a truce, and once
// the truce has been refused twice it offers tribute at the fair cut
// instead; it takes any truce the rival offers, and a tribute offer once
// it has been refused twice. It never sends the enforcers in. It is the
// baseline for "a player who buys peace".
func Diplomat(cfg *content.Config, lieLowAt float64, corners int) Policy {
	territory := Territory(cfg, lieLowAt, corners)
	rv := rivals.New(cfg)
	dip := cfg.Rivals.Diplomacy
	return func(w *game.World) {
		territory(w)
		humbled := w.Stats.DealsRefused >= 2
		for _, o := range w.Offers {
			switch o.Deal.Kind {
			case game.DealTruce:
				_, _ = w.Accept(o.ID)
			case game.DealTribute:
				if humbled {
					_, _ = w.Accept(o.ID)
				}
			}
		}
		if w.AtPeace() || w.Today.Proposal != nil || w.Rival.LastFlip == 0 || w.Day-w.Rival.LastFlip > DiplomatDays {
			return
		}
		if humbled {
			_ = w.Propose(game.DealTribute, game.Terms{PerDay: rv.Cut(w, dip.TributeCuts[1])})
			return
		}
		_ = w.Propose(game.DealTruce, game.Terms{Days: dip.TruceDays[1]})
	}
}

// DiplomatDays is how long after losing a corner the diplomat keeps
// asking for peace.
const DiplomatDays = 7

// Laundered plays like Crewed and washes the money: it buys the cheapest
// front it does not own whenever dirty cash is three times the price, runs
// the dial at normal, and drops to careful for LaunderCarefulDays after an
// audit. It is the baseline for "a player who stops sitting on a pile".
func Laundered(cfg *content.Config, lieLowAt float64) Policy {
	crewed := Crewed(cfg, lieLowAt)
	return func(w *game.World) {
		washUp(cfg, w)
		crewed(w)
	}
}

// LaunderCarefulDays is how long the laundered policy runs its fronts
// careful after an audit.
const LaunderCarefulDays = 30

// ReserveLot sends a lot of clean cash offshore (#195), or what there
// is under it over keep, once a day: the move the DA never reads. Keep
// is what stays in the pile (a day's upkeep for the retiree, a
// campaign's worth for the boss): a front's upkeep comes out of the
// pile before its income lands, so a player who moves every clean
// dollar shuts its own fronts on a thin morning. It reports whether
// anything went.
func ReserveLot(ld *laundering.Sim, w *game.World, keep int) bool {
	if w.Today.Reserved > 0 {
		return false
	}
	amt := min(ld.Offshore().Lot, w.Player.CleanCash-keep)
	if amt <= 0 {
		return false
	}
	return w.Reserve(amt) == nil
}

// Retiree plays like Laundered and leaves (#195, #49's policy): a lot
// a day offshore from the first front, keeping a day's upkeep in the
// pile; once the account holds what retiring takes and RetireAfter has
// passed it lies low, every corner recalled, until the days are quiet
// enough, and retires the first day it can. It is the baseline for "a
// player who gets out": its score is the account at exit.
func Retiree(cfg *content.Config, lieLowAt float64) Policy {
	laundered := Laundered(cfg, lieLowAt)
	ld := laundering.New(cfg)
	off := cfg.Laundering.Offshore
	return func(w *game.World) {
		if w.Day >= RetireAfter && w.Offshore >= off.RetireCash {
			if ld.CanRetire(w) {
				_ = ld.Retire(w)
				return
			}
			for _, m := range w.Crew.Members {
				w.Recall(m.ID)
			}
			w.SetLieLow(true)
			return
		}
		laundered(w)
		ReserveLot(ld, w, ld.Upkeep(w))
	}
}

// RetireAfter is the day from which the retiree retires as soon as it
// can: tier 3's checkpoint, so the exit is measured as a tier-4 one.
var RetireAfter = TierDays[2]

// Funded plays like Laundered and buys the city off (#41): whenever the
// pressure where it is has passed FundPressure it gives the city
// FundShare of its clean cash, up to what takes goodwill to 100, never a
// dollar of dirty; and it backs the reform ticket (#193) in every city
// when a campaign opens, with FundShare of its clean cash up to what
// buys the whole swing. It is the baseline for "a player who pays the
// town".
func Funded(cfg *content.Config, lieLowAt float64) Policy {
	laundered := Laundered(cfg, lieLowAt)
	return func(w *game.World) {
		laundered(w)
		backReform(cfg, w)
		payTown(cfg, w, w.Here())
	}
}

// payTown gives a city FundShare of the clean cash in hand, up to what
// takes its goodwill to 100, once a day and only while its pressure is
// over FundPressure.
func payTown(cfg *content.Config, w *game.World, c *game.City) {
	if c.Pressure <= FundPressure || w.FundedToday(c.ID) > 0 {
		return
	}
	need := int((100 - c.Goodwill) * float64(cfg.Law.Law.GoodwillCash))
	if amt := min(int(float64(w.Player.CleanCash)*FundShare), need); amt > 0 {
		_ = w.Fund(c.ID, amt)
	}
}

// backReform puts FundShare of the clean cash in hand behind the reform
// ticket in every city whose campaign is open and has none of its money
// yet (#193), up to what buys the whole swing: one donation a city an
// election, the day the campaign opens or the first day after with
// clean cash to give.
func backReform(cfg *content.Config, w *game.World) {
	if !w.Law.CampaignOpen {
		return
	}
	for _, cid := range w.CityOrder {
		if w.Cities[cid].Campaign.Cash > 0 {
			continue
		}
		if _, today := w.BackedToday(cid); today > 0 {
			continue
		}
		if amt := min(int(float64(w.Player.CleanCash)*FundShare), cfg.Law.Campaign.Fill()); amt > 0 {
			_ = w.Back(cid, "reform", amt)
		}
	}
}

// FundPressure is the pressure past which the funded policy pays, and
// FundShare the share of its clean cash it pays at a time.
const (
	FundPressure = 60
	FundShare    = 0.25
)

// Appoint fixes who the law is for a run: the chief's personality and
// the DA's stance, whichever are given, and stops the clock on both so
// they stay in office however long the run goes. It returns the config to
// run with; the world's actors are renamed in place. Either may be "" to
// take the seed's.
func Appoint(cfg *content.Config, w *game.World, chief, da string) *content.Config {
	fixed := *cfg
	if chief != "" {
		w.Law.Chief.Personality = chief
		fixed.Law.Law.ChiefTerm = 0
	}
	if da != "" {
		w.Law.DA.Stance = da
		fixed.Law.Law.TermDays = 0
	}
	return &fixed
}

// Elect holds a DA election on w now, on the given day's dice, and
// returns the winner: a test's way of asking the ballot box a question
// without playing to the term.
func Elect(cfg *content.Config, w *game.World, day int) events.DAElected {
	forced := *cfg
	forced.Law.Law.TermDays = 1
	w.Law.DA.ElectedDay = day - 1
	t := &game.Tick{Day: day, RNG: game.RNGFor(w.Seed, day), Seed: w.Seed}
	law.New(&forced).Step(w, t)
	for _, e := range t.Events() {
		if ev, ok := e.(events.DAElected); ok {
			return ev
		}
	}
	return events.DAElected{}
}

// Distributor plays like Laundered until the wholesaler will deal with it,
// then runs the route: on day one it turns the dial of the biggest route
// into home to normal (#61) and keeps its target at DistributorDays of
// home's demand for every product cheaper by the lot at the far end than
// on home's street, so the logistics sim buys the lots and sends them;
// once the wholesaler deals it moves to the city that sells by the lot
// and stays there, keeps runners on the corners at home and puts the
// rest on the corners there, buys at retail for the corners there, and
// sells everything that lands at home and everything stashed where it
// is. It is the baseline for "a player who runs a route".
func Distributor(cfg *content.Config, lieLowAt float64) Policy {
	return distribute(cfg, lieLowAt, false, false, "", BossMargin)
}

// DistributorDays is how many days of home's demand the distributor keeps
// the route's target at.
const DistributorDays = 4

// Corrupt plays like Distributor and buys the law (#42): whenever the
// heat where it stands or at home is over CorruptHeat and the chief is
// not already bought, it hands the chief chief_price (it knows a
// corrupt chief from a zealous one only the way the player does: once
// Observed, or once an envelope has come back, it leaves a zealous
// chief alone), and it keeps the
// checkpoint or customs agent on its route bought, renewing the day the
// deal runs out. Every envelope is dirty cash, and CorruptMargin times
// the price stays in hand. It is the baseline for "a player who pays
// the police".
func Corrupt(cfg *content.Config, lieLowAt float64) Policy {
	distributor := Distributor(cfg, lieLowAt)
	lg := logistics.New(cfg)
	tun := cfg.Law.Bribes
	home := cfg.City.Home().ID
	return func(w *game.World) {
		distributor(w)
		if w.Over != nil || w.Cold() {
			return
		}
		hot := math.Max(w.Here().Heat, w.Home().Heat)
		burned := w.Law.Backfired > 0 && w.Law.Chief.Since <= w.Law.Backfired // this chief sent one back already
		if hot > CorruptHeat && !w.Law.ChiefBoughtOn(w.Day) && w.BribedToday(game.BribeChief) == 0 && !burned && !(w.Law.Chief.Observed && w.Law.Chief.Personality == "zealous") && w.Player.DirtyCash >= CorruptMargin*tun.ChiefPrice {
			_ = w.Bribe(game.BribeChief, tun.ChiefPrice)
		}
		for _, r := range lg.Routes(home) {
			if !w.Route(r.ID).Dial.On() {
				continue
			}
			price := tun.CheckpointPrice
			if logistics.Customs(r) {
				price = tun.CustomsPrice
			}
			if _, live := w.Checkpoint(r.ID); !live && w.Player.DirtyCash >= CorruptMargin*price {
				_ = w.BuyCheckpoint(r.ID, price, tun.CheckpointDays)
			}
		}
	}
}

// CorruptHeat is the heat over which the corrupt player pays the chief,
// and CorruptMargin how many times the price it keeps in hand.
const (
	CorruptHeat   = 50.0
	CorruptMargin = 3
)

// Delegated plays like Distributor and hands home over: the first
// lieutenant who comes looking for work is hired ahead of anyone else and
// given the city the player is not in, and from then on the policy posts
// nothing and sells nothing there itself. It keeps HubCorners runners on
// the hub's corners, leaves the rest of the crew idle for the lieutenant
// to post, fills the roster the lieutenant's people make room for, and
// leaves the route to send everything home, where the lieutenant sells it
// at their dial and keeps their cut. personality, if set, is what the
// lieutenant turns out to be, so one seed can be played under each
// temper; "" takes them as they come. It is the tier-4 policy: the
// second city staffed by somebody who is not you.
func Delegated(cfg *content.Config, lieLowAt float64, personality string) Policy {
	return distribute(cfg, lieLowAt, true, false, personality, BossMargin)
}

// Boss plays the whole game (#60): Delegated, and it holds home as well
// as handing it over. Once the rival is about it keeps Territory's share
// of the roster as enforcers and, whenever a corner is contested, sends
// them against the rival's biggest corner at push when the odds (the
// ones the picker shows) clear BossOdds; otherwise it talks the way
// Diplomat does, proposing a truce whenever
// the rival has taken a corner off it lately and taking any truce on the
// table. It sells only what is worth the heat (BossWorth), spends at
// BossMargin, pays generous, fires a lieutenant who turns out violent and
// works every corner in the hub. It launders and buys the tree as
// Delegated does, and it invests its clean cash in the fronts' levels
// (#192): the cheapest next level of any front it owns, one a day, when
// clean cash is BossMargin times the price. It is the tier-3 and tier-4
// policy: the player who uses every screen.
func Boss(cfg *content.Config, lieLowAt float64, personality string) Policy {
	return BossAt(cfg, lieLowAt, personality, BossMargin)
}

// BossAt is Boss with the levels' margin given: how many times the next
// level's price it holds in clean cash before it buys one; the fronts
// and the nodes stay at BossMargin, so the road is not starved of the
// dirty they cost. cmd/balance's -margin; the greed curve (#192,
// TestInvestingEverythingFreezesTheFronts) reads it at 1: every clean
// dollar into levels, and one thin morning shuts the places that were
// paying.
func BossAt(cfg *content.Config, lieLowAt float64, personality string, margin float64) Policy {
	return distribute(cfg, lieLowAt, true, true, personality, margin)
}

// BossOdds is the strike odds under which the boss talks instead.
const BossOdds = 0.5

// BossMargin is how many times a front's or a node's price the boss has
// in hand before it buys.
const BossMargin = 1.5

// BossWorth is the share of the best unlocked product's price per point
// of heat under which the boss leaves a product alone: heat is the bind
// from tier 2 on, and a unit of weed draws a fifteenth of what a unit of
// designer does for a hundredth of the money. With the ladder to meth it
// keeps coke and up; once designer is on offer, heroin and up.
const BossWorth = 0.2

// worth reports whether a product is worth the heat to the boss: its base
// price per point of heat is at least BossWorth of the best on offer.
func worth(cfg *content.Config, w *game.World, id string) bool {
	value := func(id string) float64 {
		pc := cfg.Market.Product(id)
		if pc == nil || pc.Heat <= 0 {
			return 0
		}
		return pc.BasePrice / pc.Heat
	}
	best := 0.0
	for _, p := range w.Products {
		best = max(best, value(p))
	}
	return value(id) >= BossWorth*best
}

// HubCorners is how many corners the delegated player keeps working in
// the hub with runners of its own; the rest of the roster is the
// lieutenant's to post.
const HubCorners = 2

func distribute(cfg *content.Config, lieLowAt float64, delegate, fight bool, personality string, margin float64) Policy {
	laundered := Laundered(cfg, lieLowAt)
	ld := laundering.New(cfg)
	crewSim := crew.New(cfg)
	rv := rivals.New(cfg)
	dip := cfg.Rivals.Diplomacy
	lg := logistics.New(cfg)
	home := cfg.City.Home().ID
	// The route into home with the most room, from the city that sells
	// by the lot; the hub is where it starts.
	var route *content.RouteConfig
	for _, c := range cfg.City.Cities {
		if !c.Wholesale || c.ID == home {
			continue
		}
		for _, r := range lg.Routes(c.ID) {
			if r.From == c.ID && r.To == home && (route == nil || r.Capacity > route.Capacity) {
				rc := r
				route = &rc
			}
		}
	}
	hub := ""
	if route != nil {
		hub = route.From
	}
	tun := cfg.Crew.Crew
	homeCorners := max(1, tun.MaxCrew/2)
	guards := max(1, tun.MaxCrew/3)
	// The delegated player keeps HubCorners runners of its own in the hub
	// and leaves the rest to the lieutenant; the boss works every corner
	// there, the lieutenant's people being their own.
	hubCorners := HubCorners
	if fight && hub != "" {
		hubCorners = len(cfg.City.City(hub).Corners)
	}
	hot := TooHot(cfg, lieLowAt)
	return func(w *game.World) {
		if route == nil {
			laundered(w)
			return
		}
		if fight {
			defer war(w, rv, dip, hot)
		}
		// The dial, set once and left; the target, refreshed as home's
		// corners come and go. Whatever the lot does not undercut home's
		// street by DistributorMargin is not worth the road.
		if !w.Route(route.ID).Dial.On() {
			_ = w.SetRoute(route.ID, events.RouteNormal)
		}
		// The boss keeps the pipeline full: the road takes days, and a
		// target under that many days of demand starves home between
		// landings.
		days := float64(DistributorDays)
		if fight {
			days = max(days, float64(lg.Days(w, *route, w.Route(route.ID).Dial.Ship())+2))
		}
		wholesale := w.WholesaleSupplier(hub)
		for _, id := range w.Products {
			homeP := w.Product(home, id)
			target := 0
			if wholesale != nil && homeP != nil && wholesale.Price[id] > 0 && wholesale.Price[id] <= homeP.Price*DistributorMargin && (!fight || worth(cfg, w, id)) {
				target = int(days * w.Demand(home, id))
			}
			_ = w.SetRouteTarget(route.ID, id, target)
		}
		if wholesale == nil || wholesale.Locked(w) {
			laundered(w)
			return
		}
		if w.Player.Location != hub {
			_ = w.Travel(hub)
		}
		// The boss spends at a thinner margin (the pile is what draws the
		// police at this scale, and a front or a node is where it goes)
		// and pays generous: wages are noise against the takings, and
		// the loyalty is what keeps a ten-strong roster from firing
		// itself one a day.
		if fight {
			washUpAt(cfg, w, BossMargin)
			BuyUpgrades(cfg, w, BossMargin)
			w.SetPay(events.PayGenerous)
			// The boss pays the town (#193): the reform ticket in every
			// city when a campaign opens, and goodwill wherever the
			// pressure is up, as Funded does where it stands.
			backReform(cfg, w)
			for _, cid := range w.CityOrder {
				payTown(cfg, w, w.Cities[cid])
			}
			// Then the businesses (#192): the levels take what the town
			// left, over a campaign's worth kept in hand for the next
			// election, so the boss's civic spending is what it was.
			InvestOver(ld, w, margin, cfg.Law.Campaign.Fill())
			// And a lot a day offshore (#195) over the same reserve,
			// never over the line and never retiring: its numbers are
			// the horizon's.
			ReserveLot(ld, w, cfg.Law.Campaign.Fill())
		} else {
			washUp(cfg, w)
			BuyUpgrades(cfg, w, 3) // the stash spots are what a lot needs room for
			w.SetPay(events.PayFair)
		}

		// The crew: runners, and one enforcer for home once the rival is
		// about. Whoever has sunk to skimming goes, one a day. The
		// delegated player hires the first lieutenant looking for work
		// ahead of anyone else and hands them home; while one is on the
		// payroll it leaves home to them.
		for _, m := range w.Crew.Members {
			if m.Loyalty < tun.SkimThreshold {
				_, _ = w.Fire(m.ID)
				break
			}
		}
		var lt *game.CrewMember
		for i := range w.Crew.Members {
			if w.Crew.Members[i].Lieutenant() {
				lt = &w.Crew.Members[i]
			}
		}
		// The boss reads the market screen: a lieutenant whose standing
		// orders are at the aggressive dial is a violent one, and runs
		// the city into an arrest the first night a shipment lands, so
		// they go the morning the orders show, before those resolve. (A
		// careful one sells half of what the corners take, but firing
		// them costs more than they do: the loyalty, their people and
		// the wait for the next one looking for work.)
		if fight && lt != nil && len(w.Crew.FiredToday) == 0 {
			for _, id := range w.Products {
				if o, ok := w.StandingOrder(home, id); ok && o.Dial == events.DialAggressive {
					_, _ = w.Fire(lt.ID)
					lt = nil
					break
				}
			}
		}
		want := "runner"
		if w.Rival.Arrived > 0 && w.Crew.Role("enforcer") == 0 && w.Crew.Runners() >= 2 {
			want = "enforcer"
		}
		if fight && w.Rival.Arrived > 0 && w.Crew.Role("enforcer") < guards && w.Crew.Runners() >= 2 {
			want = "enforcer"
		}
		if delegate && lt == nil {
			for _, c := range w.Crew.Candidates {
				if c.Lieutenant() {
					want = game.RoleLieutenant
				}
			}
			// A full roster makes room for them: the least skilled
			// runner goes, and the lieutenant's people more than make
			// up for it.
			if want == game.RoleLieutenant && len(w.Crew.Members) >= crewSim.MaxCrew(w) && len(w.Crew.FiredToday) == 0 {
				worst := -1
				for i, m := range w.Crew.Members {
					if m.Role == "runner" && (worst < 0 || m.Skill < w.Crew.Members[worst].Skill) {
						worst = i
					}
				}
				if worst >= 0 {
					_, _ = w.Fire(w.Crew.Members[worst].ID)
				}
			}
		}
		best := -1
		for i, c := range w.Crew.Candidates {
			if c.Role == want && (best < 0 || c.Skill > w.Crew.Candidates[best].Skill) {
				best = i
			}
		}
		if best >= 0 && len(w.Crew.Members) < crewSim.MaxCrew(w) {
			if c := w.Crew.Candidates[best]; w.Player.DirtyCash >= c.Fee+cfg.Market.Market.StartCash {
				if m, err := w.Hire(c.ID, crewSim.MaxCrew(w)); err == nil && m.Lieutenant() {
					lt = w.Crew.Member(m.ID)
					if personality != "" {
						lt.Personality = personality
					}
				}
			}
		}
		if lt != nil && lt.City != home {
			_ = w.Assign(lt.ID, home)
		}
		// A lieutenant whose loyalty is sliding toward the flip line is
		// paid off before they get there: the player who reads the
		// roster keeps the one person who knows everything sweet.
		if lt != nil && lt.Loyalty < crewSim.FlipLine()+10 && len(w.Crew.PaidOffToday) == 0 {
			if cost := crewSim.PayoffCost(*lt); w.Player.DirtyCash >= cost+cfg.Market.Market.StartCash {
				_, _ = w.PayOff(lt.ID, cost, crewSim.PayoffLoyalty())
			}
		}
		delegated := lt != nil
		// Arriving with the whole crew posted at home, one runner comes
		// off the smallest home corner to work here: the route needs
		// somebody on this end, and a lieutenant only comes looking
		// once corners are held in both cities.
		if !delegated && w.WorkedIn(hub) == 0 && w.WorkedIn(home) > homeCorners {
			if c := pickCorner(w, func(c game.Corner) bool { return c.City == home && c.Worked() && c.Runner != game.You }, func(c game.Corner) float64 { return -c.Demand }); c != nil {
				w.Recall(c.Runner)
			}
		}
		// Idle runners take corners at home until homeCorners are worked,
		// then here; the enforcer guards the home corner the rival borders.
		// With home handed over, HubCorners runners work here, the rest
		// wait for the lieutenant, enforcer included, and whoever the
		// lieutenant had no corner for last night works here after all.
		for _, m := range w.Crew.Members {
			if w.PostOf(m.ID) != nil {
				continue
			}
			if delegated && (m.Role != "runner" || (w.WorkedIn(hub) >= hubCorners && m.Hired == w.Day)) {
				continue
			}
			switch m.Role {
			case "runner":
				// Home first, until homeCorners are worked or the rival
				// has left nothing to work; then here.
				for _, city := range []string{home, hub} {
					if city == home && (delegated || w.WorkedIn(home) >= homeCorners) {
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
			case "enforcer":
				score := func(c game.Corner) float64 {
					if w.Contested(c) {
						return 10 + c.Demand
					}
					return c.Risk
				}
				if c := pickCorner(w, func(c game.Corner) bool { return c.City == home && c.Worked() && c.Enforcer == 0 }, score); c != nil {
					_ = w.Post(c.ID, m.ID)
				}
			}
		}

		// The corners here: the supplier stocks them at retail, toward
		// what they sell, the way any trader restocks; the boss stocks
		// only what is worth the heat. The route feeds home on its own.
		if fight {
			restockOnly(cfg, w, func(id string) bool { return worth(cfg, w, id) })
		} else {
			restock(cfg, w)
		}

		// Sales: everything at home, and everything here. Heat anywhere
		// over the line is a day off.
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		for _, id := range w.Products {
			if q := w.Stock(home, id); q > 0 && !delegated {
				_ = w.PlaceSell(home, id, q, events.DialNormal)
			}
			if q := w.Stock(hub, id); q > 0 {
				_ = w.PlaceSell(hub, id, q, events.DialNormal)
			}
		}
	}
}

// DistributorMargin is the fraction of home's street price a lot must
// come under for the distributor to route the product at all.
const DistributorMargin = 0.7

// war is the boss's answer to the rival, after the day's trading is
// queued: the enforcers against the rival's biggest corner at push when
// a corner is contested, heat is under the line and the odds are over
// BossOdds; else the diplomat's table, a truce proposed whenever a
// corner was lost in the last DiplomatDays and any truce offered taken.
func war(w *game.World, rv *rivals.Sim, dip content.DiplomacyTuning, hot func(*game.World) bool) {
	if w.Rival.Arrived == 0 {
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
	if contested && !w.AtPeace() && !hot(w) && rv.Odds(w, events.ForcePush) >= BossOdds {
		if c := pickCorner(w, func(c game.Corner) bool { return c.Owner == game.OwnerRival }, size); c != nil {
			_ = w.SendEnforcers(c.ID, events.ForcePush)
			return
		}
	}
	if w.AtPeace() || w.Today.Proposal != nil || w.Rival.LastFlip == 0 || w.Day-w.Rival.LastFlip > DiplomatDays {
		return
	}
	_ = w.Propose(game.DealTruce, game.Terms{Days: dip.TruceDays[1]})
}

// Delegate puts a lieutenant on the payroll, free, running city with the
// given temper, so a test can measure what one does without waiting for
// one to come looking. The first candidate becomes them; it panics if
// nobody is looking for work.
func Delegate(cfg *content.Config, w *game.World, city, personality string) game.CrewMember {
	if len(w.Crew.Candidates) == 0 {
		panic("harness.Delegate: nobody looking for work")
	}
	c := w.Crew.Candidates[0]
	w.Player.DirtyCash += c.Fee
	m, err := w.Hire(c.ID, len(w.Crew.Members)+1)
	if err != nil {
		panic("harness.Delegate: " + err.Error())
	}
	w.Crew.HiredToday = nil // not a signing to report
	lt := w.Crew.Member(m.ID)
	rc := cfg.Crew.Role[game.RoleLieutenant]
	lt.Role, lt.Personality, lt.Units = game.RoleLieutenant, personality, 0
	lt.Wage = int(rc.WageBase + rc.WagePerSkill*float64(lt.Skill))
	if err := w.Assign(lt.ID, city); err != nil {
		panic("harness.Delegate: " + err.Error())
	}
	return *lt
}

// washUp is the Laundered policy's fronts: the cheapest one lacking when
// dirty cash is three times its price, the dial at normal and careful for
// a while after an audit.
func washUp(cfg *content.Config, w *game.World) {
	washUpAt(cfg, w, 3)
}

// Invest buys the cheapest next level of any front owned (#192), one a
// day, when clean cash is margin times its price: the boss's way of
// putting the pile to work. A front at its top is skipped. It reports
// whether a level was bought.
func Invest(ld *laundering.Sim, w *game.World, margin float64) bool {
	return InvestOver(ld, w, margin, 0)
}

// InvestOver is Invest with a reserve: only the clean cash over it
// counts toward the margin. The boss keeps a campaign's worth (#193,
// law.toml [campaign] Fill) for the next election.
func InvestOver(ld *laundering.Sim, w *game.World, margin float64, reserve int) bool {
	var pick *game.Front
	best := 0
	for i := range w.Fronts {
		f := &w.Fronts[i]
		if f.Level >= ld.MaxLevel(*f) {
			continue
		}
		if cost := ld.LevelCost(*f, 1); pick == nil || cost < best {
			pick, best = f, cost
		}
	}
	if pick == nil || float64(w.Player.CleanCash-reserve) < margin*float64(best) {
		return false
	}
	return ld.Invest(w, pick.ID, 1) == nil
}

// washUpAt is washUp with the margin given: the front is bought when
// dirty cash is margin times its price.
func washUpAt(cfg *content.Config, w *game.World, margin float64) {
	for _, o := range laundering.New(cfg).Offers() {
		if w.Front(o.ID) != nil {
			continue
		}
		if !o.Locked(w) && float64(w.Player.DirtyCash) >= margin*float64(o.Cost) {
			_, _ = w.BuyFront(o)
		}
		break // the cheapest one you lack, or nothing
	}
	dial := events.LaunderNormal
	for _, f := range w.Fronts {
		if f.Audited > 0 && w.Day-f.Audited < LaunderCarefulDays {
			dial = events.LaunderCareful
		}
	}
	w.SetLaunderDial(dial)
}

// pickCorner returns the corner in any city passing ok with the highest
// score, or nil.
func pickCorner(w *game.World, ok func(game.Corner) bool, score func(game.Corner) float64) *game.Corner {
	var best *game.Corner
	for _, cid := range w.CityOrder {
		cs := w.Cities[cid].Corners
		for i := range cs {
			c := &cs[i]
			if ok(*c) && (best == nil || score(*c) > score(*best)) {
				best = c
			}
		}
	}
	return best
}

// Horizon is how many days the harness plays a run for when it measures
// something. It is a ruler, not a run length: the game has no day cap and a
// run ends only through an ending, so a policy that is "still free at the
// horizon" is one the game never punished for playing on.
const Horizon = 200

// TierDays are the checkpoints the money curve is read at (#24): the day
// each progression tier is expected to have paid off by, read from
// progression.toml (#147: 30, 70, 120, 200) so the harness and the game
// name the same tiers. Like Horizon they are where the harness looks,
// not where the game stops.
var TierDays = content.MustLoad().Progression.Checkpoints()

// NoRival returns a copy of cfg in which the rival never arrives, so a
// run measures what the corners are worth with nobody contesting them
// (#60: the ceiling a policy plays against). Nothing else moves.
func NoRival(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Rivals.Rivals.ArriveDay = 1 << 30
	return &boxed
}

// FlatPace returns a copy of cfg in which the rival claims at the flat
// pace it had before #60: no scaling by what the player holds, no
// cooldown, no grace after it arrives and no fear in it, so a run
// measures what the pace alone moved.
func FlatPace(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Rivals.Pace = content.PaceTuning{}
	boxed.Reputation.Effects.RivalClaimCut = 0
	return &boxed
}

// PaceDays are the days cmd/balance reads the rival's corner count at.
var PaceDays = []int{30, 60, 120}

// NoHeat returns a copy of cfg in which nothing adds heat and the police
// never answer: every source the sims read is zeroed (sales, the cash
// pile, sloppy crews, informants, audits, tips, strikes, the crackdown, a
// seizure, fear's floor) and the ladder is lifted past 100, so a run
// measures what the street would move if it were never watched (#60: the
// ceiling a policy plays against). The DA's file still fills from what
// is not heat (an informant's pages, a fast seizure), so a run can still
// end.
func NoHeat(cfg *content.Config) *content.Config {
	boxed := *cfg
	h := &boxed.Heat.Heat
	h.SaleHeat, h.DirtyCashHeat, h.SloppyHeat, h.InformantHeat, h.AuditHeat = 0, 0, 0, 0, 0
	boxed.Heat.Responses = append([]content.ResponseConfig(nil), cfg.Heat.Responses...)
	for i := range boxed.Heat.Responses {
		boxed.Heat.Responses[i].Threshold += 1000
	}
	boxed.Rivals.Rivals.TipHeat, boxed.Rivals.Rivals.CrackdownHeat = 0, 0
	boxed.Rivals.Force = map[string]content.ForceConfig{}
	for k, f := range cfg.Rivals.Force {
		f.Heat = 0
		boxed.Rivals.Force[k] = f
	}
	boxed.Routes.Shipping.SeizureHeat = 0
	boxed.Reputation.Effects.FearHeatFloor = 0
	return &boxed
}
