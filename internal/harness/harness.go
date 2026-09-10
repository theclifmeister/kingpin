// Package harness runs the game headless under a scripted policy so balance
// can be measured and invariants tested without a terminal.
package harness

import (
	"sort"

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
// dilemma deck stays in the box: cards are choices, a scripted player has
// none to make, and the invariants the harness pins belong to the other
// sims. RunWith deals them.
func Run(cfg *content.Config, seed uint64, days int, policy Policy) (Result, error) {
	return RunFrom(cfg, sim.NewWorld(cfg, seed), days, policy)
}

// RunFrom plays up to days days on from w, which the caller may have set
// up (a cash pile, a crew) to test a situation a fresh run takes a while
// to reach. No cards are dealt; see Run.
func RunFrom(cfg *content.Config, w *game.World, days int, policy Policy) (Result, error) {
	boxed := *cfg
	boxed.Dilemmas.Cards = nil
	return run(&boxed, w, days, policy, nil)
}

// RunWith plays like RunFrom with the deck in play: every card is answered
// with pick before the policy acts, the way the UI shows the card before
// the day starts.
func RunWith(cfg *content.Config, w *game.World, days int, policy Policy, pick Chooser) (Result, error) {
	if pick == nil {
		pick = Decline
	}
	return run(cfg, w, days, policy, pick)
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
		total += city.Market[id].Demand
	}
	for _, id := range w.Products {
		m := city.Market[id]
		target := int(float64(w.Capacity(city.ID)) * m.Demand / total)
		afford := int(float64(w.Player.DirtyCash-reserve) / m.SupplierPrice)
		qty := min(target-w.Stock(city.ID, id), afford, w.Free(city.ID))
		if qty > 0 {
			_, _ = w.Buy(id, qty, pressure)
		}
	}
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
	hs := heat.New(cfg.Heat, cfg.Market, cfg.Routes.Shipping, cfg.Upgrades, cfg.Reputation.Effects, cfg.Crew.Lieutenant, cfg.Law)
	var sting *content.ResponseConfig
	for i := range cfg.Heat.Responses {
		if cfg.Heat.Responses[i].Level == "sting" {
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

// Upgraded plays like Managed and spends on the tree: whenever it can pay
// three times the price of the cheapest node it can buy, it buys it,
// Security first, then Operations, then Legal. It is the baseline for "a
// player who invests instead of reinvesting every dollar in stock".
func Upgraded(cfg *content.Config, lieLowAt float64) Policy {
	managed := Managed(cfg, lieLowAt)
	return func(w *game.World) {
		BuyUpgrades(cfg, w, 3)
		managed(w)
	}
}

// BuyUpgrades buys, in branch order Security, Operations, Legal, the
// cheapest node the player can buy from the right pool with margin times
// its cost in hand, one per call.
func BuyUpgrades(cfg *content.Config, w *game.World, margin float64) {
	for _, branch := range []string{"security", "operations", "legal"} {
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
	w.UpgradesToday = nil // a grant is not a purchase to report
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
// left where they are.
func staff(cfg *content.Config, w *game.World, city string, corners int) {
	tun := cfg.Crew.Crew
	if corners <= 0 {
		corners = len(cfg.City.City(city).Corners)
	}
	guards := max(1, tun.MaxCrew/3)
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
		if len(w.Crew.Members) >= tun.MaxCrew && len(w.Crew.FiredToday) == 0 {
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
	if best >= 0 && len(w.Crew.Members) < tun.MaxCrew {
		c := w.Crew.Candidates[best]
		if w.Player.DirtyCash >= c.Fee+cfg.Market.Market.StartCash {
			_, _ = w.Hire(c.ID, tun.MaxCrew)
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
		if w.Heat.Leaks >= 2 && w.Investigation == nil && len(w.Crew.FiredToday) == 0 {
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

// Diplomat plays like Territory and talks: whenever the rival has taken a
// corner off it in the last DiplomatDays it proposes a truce, and once
// the truce has been refused twice it offers tribute at the fair cut
// instead; it takes any truce the rival offers, and a tribute offer once
// it has been refused twice. It never sends the enforcers in. It is the
// baseline for "a player who buys peace".
func Diplomat(cfg *content.Config, lieLowAt float64, corners int) Policy {
	territory := Territory(cfg, lieLowAt, corners)
	rv := rivals.New(cfg.Rivals, cfg.Names, cfg.Reputation.Effects, cfg.Law.Effects)
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
		if w.AtPeace() || w.Proposal != nil || w.Rival.LastFlip == 0 || w.Day-w.Rival.LastFlip > DiplomatDays {
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

// Funded plays like Laundered and buys the city off (#41): whenever the
// pressure where it is has passed FundPressure it gives the city
// FundShare of its clean cash, up to what takes goodwill to 100, never a
// dollar of dirty. It is the baseline for "a player who pays the town".
func Funded(cfg *content.Config, lieLowAt float64) Policy {
	laundered := Laundered(cfg, lieLowAt)
	tun := cfg.Law.Law
	return func(w *game.World) {
		laundered(w)
		here := w.Here()
		if here.Pressure <= FundPressure || w.FundedToday(here.ID) > 0 {
			return
		}
		need := int((100 - here.Goodwill) * float64(tun.GoodwillCash))
		if amt := min(int(float64(w.Player.CleanCash)*FundShare), need); amt > 0 {
			_ = w.Fund(here.ID, amt)
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
	law.New(forced.Law, forced.Names).Step(w, t)
	for _, e := range t.Events() {
		if ev, ok := e.(events.DAElected); ok {
			return ev
		}
	}
	return events.DAElected{}
}

// Distributor plays like Laundered until the wholesaler will deal with it,
// then runs the route: it moves to the city that sells by the lot and
// stays there as the buyer, keeps runners on the corners at home and puts
// the rest on the corners there, ships home every unit of whatever is
// cheaper by the lot than at home (the normal dial, the biggest route
// with room) and buys the next lots behind it, sells the rest where it
// is, and sells everything that lands at home. It is the baseline for "a
// player who runs a route".
func Distributor(cfg *content.Config, lieLowAt float64) Policy {
	return distribute(cfg, lieLowAt, false, "")
}

// Delegated plays like Distributor and hands home over: the first
// lieutenant who comes looking for work is hired ahead of anyone else and
// given the city the player is not in, and from then on the policy posts
// nothing and sells nothing there itself. It keeps HubCorners runners on
// the hub's corners, leaves the rest of the crew idle for the lieutenant
// to post, fills the roster the lieutenant's people make room for, buys
// by the lot and ships everything home, where the lieutenant sells it at
// their dial and keeps their cut. personality, if set, is what the
// lieutenant turns out to be, so one seed can be played under each
// temper; "" takes them as they come. It is the tier-4 policy: the
// second city staffed by somebody who is not you.
func Delegated(cfg *content.Config, lieLowAt float64, personality string) Policy {
	return distribute(cfg, lieLowAt, true, personality)
}

// HubCorners is how many corners the delegated player keeps working in
// the hub with runners of its own; the rest of the roster is the
// lieutenant's to post.
const HubCorners = 2

func distribute(cfg *content.Config, lieLowAt float64, delegate bool, personality string) Policy {
	laundered := Laundered(cfg, lieLowAt)
	crewSim := crew.New(cfg.Crew, cfg.Names, cfg.Reputation.Effects)
	lg := logistics.New(cfg.Routes, cfg.City, cfg.Market)
	wholesale := lg.Wholesale()
	home := cfg.City.Home().ID
	hub := ""
	for _, c := range cfg.City.Cities {
		if c.Wholesale && c.ID != home {
			hub = c.ID
			break
		}
	}
	tun := cfg.Crew.Crew
	homeCorners := max(1, tun.MaxCrew/2)
	pressure := cfg.Market.Market.BuyPricePressure
	hot := TooHot(cfg, lieLowAt)
	return func(w *game.World) {
		if hub == "" || wholesale.Locked(w) || len(lg.Routes(hub)) == 0 {
			laundered(w)
			return
		}
		if w.Player.Location != hub {
			_ = w.Travel(hub)
		}
		washUp(cfg, w)
		BuyUpgrades(cfg, w, 3) // the stash spots are what a lot needs room for
		w.SetPay(events.PayFair)

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
		want := "runner"
		if w.Rival.Arrived > 0 && w.Crew.Role("enforcer") == 0 && w.Crew.Runners() >= 2 {
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
			if delegated && (m.Role != "runner" || (w.WorkedIn(hub) >= HubCorners && m.Hired == w.Day)) {
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

		// The route. Whatever sells for more at home than it costs here
		// goes home today, less what the corners here sell dearer than
		// home does, and the next few days of home's demand is bought
		// behind it: by the lot where a lot fits, at retail for the rest.
		fx := game.FoldEffects(w, cfg.Upgrades).BuyPressureMul
		keep := map[string]int{}
		for _, id := range w.Products {
			hubP, homeP := w.Product(hub, id), w.Product(home, id)
			if hubP == nil || homeP == nil {
				continue
			}
			if hubP.Price > homeP.Price {
				keep[id] = int(w.Demand(hub, id))
			}
			if hubP.SupplierPrice > homeP.Price*0.7 {
				continue // not worth the road
			}
			if q := w.Stock(hub, id) - keep[id]; q > 0 {
				ship(lg, w, hub, home, id, q)
			}
			want := int(4*w.Demand(home, id)) - w.Stock(home, id) - w.InTransit(id) + keep[id] - w.Stock(hub, id)
			lots := min((want+wholesale.Lot-1)/wholesale.Lot, int(float64(w.Player.DirtyCash)/(hubP.SupplierPrice*wholesale.Mul))/wholesale.Lot)
			if lots > 0 {
				_, _ = w.BuyWholesale(id, lots, wholesale, pressure*fx)
			}
		}

		// Sales: everything at home, and here whatever is not waiting
		// for a truck. Heat anywhere over the line is a day off.
		if hot(w) {
			w.SetLieLow(true)
			return
		}
		for _, id := range w.Products {
			if q := w.Stock(home, id); q > 0 && !delegated {
				_ = w.PlaceSell(home, id, q, events.DialNormal)
			}
			if q := min(w.Stock(hub, id), keep[id]); q > 0 {
				_ = w.PlaceSell(hub, id, q, events.DialNormal)
			}
		}
	}
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

// ship sends units of a product between two cities at the normal dial on
// the emptiest route that will take them today, splitting across routes
// if one will not.
func ship(lg *logistics.Sim, w *game.World, from, to, product string, units int) {
	routes := lg.Routes(from)
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].Capacity > routes[j].Capacity })
	for _, r := range routes {
		if units <= 0 {
			break
		}
		if r.Other(from) != to {
			continue
		}
		if _, err := lg.Ship(w, r.ID, from, to, product, min(units, r.Capacity), events.ShipNormal); err == nil {
			units -= min(units, r.Capacity)
		}
	}
}

// washUp is the Laundered policy's fronts: the cheapest one lacking when
// dirty cash is three times its price, the dial at normal and careful for
// a while after an audit.
func washUp(cfg *content.Config, w *game.World) {
	for _, o := range laundering.New(cfg.Laundering, cfg.Crew).Offers() {
		if w.Front(o.ID) != nil {
			continue
		}
		if !o.Locked(w) && w.Player.DirtyCash >= 3*o.Cost {
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
// each progression tier is expected to have paid off by. Like Horizon they
// are where the harness looks, not where the game stops.
var TierDays = []int{30, 70, 120, Horizon}
