package game

import (
	"errors"
	"fmt"
	"slices"
)

// LawState is the law's actors (#41): the police chief and the district
// attorney. Public pressure and goodwill are per city (City.Pressure,
// City.Goodwill). The law sim owns all of it; the heat sim and the rival
// sim read it through their own tuning.
type LawState struct {
	Chief Chief
	DA    DA

	// CampaignOpen says the next DA election is close enough for the
	// tickets to take money (#193, law.toml [campaign] open_days): the
	// law sim stamps it every step for the day to come, so Back can
	// refuse without the law's tuning. It is off between campaigns and
	// where there are no elections.
	CampaignOpen bool

	// SnapElection is the day of a snap election an incident called
	// (#44); 0 none. The law sim holds it when it comes before the
	// term's end and clears it after.
	SnapElection int

	// The bought law (#42), all of it the law sim's. ChiefBought and
	// DABought are the day a bribe runs out (0: none), ChiefShare the
	// bought chief's share of the effect (1 for a corrupt one, less for
	// a lazy one). Leads is what the DA's office has heard about
	// envelopes, LeadDay when it last heard. Backfired and Filed are the
	// day a bribe blew up and the day the leads became a file: the heat
	// sim, which steps before the law, reads them the next morning and
	// adds the pages and the heat. Cold is the day every deal ended
	// under a law-and-order DA (calls_stop_days after they took office;
	// 0: none), and nothing bought before it is live from it on.
	ChiefBought int
	ChiefShare  float64
	DABought    int
	Leads       int
	LeadDay     int
	Backfired   int
	Filed       int
	Cold        int
}

// Bribe targets (#42): the chief and the DA.
const (
	BribeChief = "chief"
	BribeDA    = "da"
)

// BribeOrder is a bribe paid today (#42), per-day scratch the law sim
// resolves tonight: who it was for and what was in the envelope.
type BribeOrder struct {
	Target string
	Amount int
}

// CheckpointOrder is a checkpoint or customs deal paid today (#42), for
// the logistics sim to report: the route and what it cost.
type CheckpointOrder struct {
	Route string
	Cost  int
}

// Chief is the police chief: a name, a personality (corrupt, zealous,
// lazy) that the heat sim multiplies its tuning by, and the day they took
// office. Observed says the player has seen enough of them for the report
// to name the personality.
type Chief struct {
	Name        string
	Personality string
	Since       int
	Observed    bool
}

// DA is the district attorney: a name, the ticket they ran on
// (law_and_order, moderate, reform) and the day they were elected. The
// stance does not move between elections.
type DA struct {
	Name       string
	Stance     string
	ElectedDay int
	Backed     bool // their ticket ran on your money (#193): the heat sim's sting line and #42's price read it
}

// Campaign is the money a city's voters saw put behind a DA ticket
// before the next election (#193): the ticket (reform or law_and_order,
// the two the vote's share separates), the clean cash so far, and
// Hedged once money went to both. The law sim folds the day's Backed
// scratch into it, spends it at the election and zeroes it.
type Campaign struct {
	Ticket string
	Cash   int
	Hedged bool
}

// Tickets are the DA tickets a campaign can back (#193), in the order
// the fund dialog turns through them: the two the vote's share moves
// between. The moderate takes a fixed share whatever the mood, so money
// cannot move them.
var Tickets = []string{"reform", "law_and_order"}

// Backing is clean cash the player put behind a DA ticket in a city
// today (#193), per-day scratch the law sim adds to the city's campaign.
type Backing struct {
	City   string
	Ticket string
	Amount int
}

// Funding is clean cash the player gave a city today, per-day scratch
// the law sim turns into goodwill and reports.
type Funding struct {
	City   string
	Amount int
}

var (
	// ErrNoCleanCash means the player tried to pay for something clean
	// money buys (goodwill, a campaign, a front's level) with money the
	// fronts have not washed yet: it is only ever bought clean.
	ErrNoCleanCash = errors.New("clean cash only, and the fronts have washed none")
	// ErrCampaignClosed means the next election is too far off for a
	// ticket to take money (#193): campaigns open open_days before it.
	ErrCampaignClosed = errors.New("no campaign is taking money yet")
	// ErrNoTicket means the ticket is not one a campaign can back.
	ErrNoTicket = errors.New("no such ticket")
	// ErrNoTarget means the bribe is for nobody the law knows (#42).
	ErrNoTarget = errors.New("nobody by that name takes envelopes")
	// ErrBribedToday means that official has had today's envelope.
	ErrBribedToday = errors.New("one envelope a day is as much as they will take")
	// ErrOfficialsCold means a law-and-order DA sits and nobody takes a
	// call (#42): the chief, the checkpoints and the customs agents.
	ErrOfficialsCold = errors.New("nobody is taking calls while a law-and-order DA sits")
	// ErrNoDirtyCash means a bribe or a checkpoint was offered clean
	// money: cash in a bag is dirty cash.
	ErrNoDirtyCash = errors.New("a bribe is dirty cash in a bag, and you have none")
)

// Cold reports whether the officials have stopped taking calls (#42): a
// law-and-order DA sits. The chief will not be bribed and no checkpoint
// or customs agent is for sale; the DA can still be tried, and bribing
// a law-and-order one backfires.
func (w *World) Cold() bool { return w.Law.DA.Stance == "law_and_order" }

// Bribe pays amount in dirty cash to the chief or the DA (#42), up
// front: the law sim resolves it tonight. One a day per target; the
// chief takes nothing while a law-and-order DA sits (ErrOfficialsCold).
func (w *World) Bribe(target string, amount int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if target != BribeChief && target != BribeDA {
		return ErrNoTarget
	}
	if amount <= 0 {
		return ErrBadQuantity
	}
	if target == BribeChief && w.Cold() {
		return ErrOfficialsCold
	}
	for _, b := range w.Today.Bribes {
		if b.Target == target {
			return ErrBribedToday
		}
	}
	if amount > w.Player.DirtyCash {
		if w.Player.DirtyCash <= 0 {
			return ErrNoDirtyCash
		}
		return fmt.Errorf("need $%d dirty, only have $%d dirty", amount, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= amount
	w.Stats.Bribes++
	w.Stats.Bribed += amount
	w.Today.Bribes = append(w.Today.Bribes, BribeOrder{Target: target, Amount: amount})
	return nil
}

// BribedToday is what went to a target today, in dirty cash.
func (w *World) BribedToday(target string) int {
	for _, b := range w.Today.Bribes {
		if b.Target == target {
			return b.Amount
		}
	}
	return 0
}

// BuyCheckpoint buys the checkpoint on a car or truck edge, or the
// customs agent on a boat edge (#42): cost in dirty cash, at once, for
// days more days on top of whatever the route already holds. Refused
// while a law-and-order DA sits. The logistics sim takes the cut off the
// edge's risk while the deal is live and reports the purchase.
func (w *World) BuyCheckpoint(route string, cost, days int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if days <= 0 || cost < 0 {
		return ErrBadQuantity
	}
	if w.Cold() {
		return ErrOfficialsCold
	}
	if cost > w.Player.DirtyCash {
		if w.Player.DirtyCash <= 0 {
			return ErrNoDirtyCash
		}
		return fmt.Errorf("need $%d dirty, only have $%d dirty", cost, w.Player.DirtyCash)
	}
	w.Player.DirtyCash -= cost
	rs := w.Route(route)
	until := w.Day
	if live, ok := w.Checkpoint(route); ok {
		until = live
	}
	rs.Bought = until + days
	if w.Routes == nil {
		w.Routes = map[string]RouteSetting{}
	}
	w.Routes[route] = rs
	w.Stats.Checkpoints++
	w.Stats.CheckpointCash += cost
	w.Today.Checkpoints = append(w.Today.Checkpoints, CheckpointOrder{Route: route, Cost: cost})
	return nil
}

// Checkpoint is the day a route's checkpoint or customs deal runs out
// and whether it is live today (#42): bought past today, and not ended
// by the cold (Law.Cold; nothing can be bought while it stands, so a
// deal on the books from before it is dead from that day on).
func (w *World) Checkpoint(route string) (until int, live bool) {
	until = w.Route(route).Bought
	return until, until > w.Day && !(w.Law.Cold > 0 && w.Day >= w.Law.Cold)
}

// CheckpointLive is Checkpoint on a given day: what the sims read.
func (w *World) CheckpointLive(route string, day int) bool {
	until := w.Route(route).Bought
	return until > day && !(w.Law.Cold > 0 && day >= w.Law.Cold)
}

// ChiefBought and DABought report whether the chief and the DA are
// bought on a day (#42): a bribe running, and not ended by the cold.
func (l LawState) ChiefBoughtOn(day int) bool {
	return l.ChiefBought > day && !(l.Cold > 0 && day >= l.Cold)
}

// DABoughtOn is ChiefBoughtOn for the DA.
func (l LawState) DABoughtOn(day int) bool {
	return l.DABought > day && !(l.Cold > 0 && day >= l.Cold)
}

// Fund gives a city amount in clean cash, at once: a community centre, a
// councillor's campaign, a police benevolent fund. Only clean cash pays;
// dirty cash is refused however much of it there is. The law sim turns
// it into goodwill at end of day and reports it.
func (w *World) Fund(city string, amount int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if amount <= 0 {
		return ErrBadQuantity
	}
	if amount > w.Player.CleanCash {
		if w.Player.CleanCash <= 0 {
			return ErrNoCleanCash
		}
		return fmt.Errorf("need $%d clean, only have $%d clean", amount, w.Player.CleanCash)
	}
	w.Player.CleanCash -= amount
	w.Stats.Funded += amount
	w.Today.Funded = append(w.Today.Funded, Funding{City: city, Amount: amount})
	return nil
}

// Back puts amount in clean cash behind a DA ticket's campaign in a
// city (#193): a donation the voters see. Only clean cash pays, as for
// Fund; only while a campaign is open (Law.CampaignOpen); only a ticket
// in Tickets. The law sim adds it to the city's campaign tonight, moves
// the vote by it at the election and reports it.
func (w *World) Back(city, ticket string, amount int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if !slices.Contains(Tickets, ticket) {
		return ErrNoTicket
	}
	if !w.Law.CampaignOpen {
		return ErrCampaignClosed
	}
	if amount <= 0 {
		return ErrBadQuantity
	}
	if amount > w.Player.CleanCash {
		if w.Player.CleanCash <= 0 {
			return ErrNoCleanCash
		}
		return fmt.Errorf("need $%d clean, only have $%d clean", amount, w.Player.CleanCash)
	}
	w.Player.CleanCash -= amount
	w.Stats.Backed += amount
	w.Today.Backed = append(w.Today.Backed, Backing{City: city, Ticket: ticket, Amount: amount})
	return nil
}

// Reserve moves amount of clean cash toward the offshore account (#195):
// out of the pile at once, into the account tonight, when the
// laundering sim moves it and takes the account's fee. Only clean cash
// goes, as for Fund; nothing comes back (an account is an exit, not a
// bank). Over the lot in a day is structuring, and the DA reads it the
// morning after.
func (w *World) Reserve(amount int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if amount <= 0 {
		return ErrBadQuantity
	}
	if amount > w.Player.CleanCash {
		if w.Player.CleanCash <= 0 {
			return ErrNoCleanCash
		}
		return fmt.Errorf("need $%d clean, only have $%d clean", amount, w.Player.CleanCash)
	}
	w.Player.CleanCash -= amount
	w.Today.Reserved += amount
	return nil
}

// ReservedToday is what the player has sent offshore today, before the
// fee: out of the pile, not yet in the account.
func (w *World) ReservedToday() int { return w.Today.Reserved }

// BackedToday is what the player has put behind a ticket in a city
// today, in clean cash, and which ticket got the last of it.
func (w *World) BackedToday(city string) (ticket string, amount int) {
	for _, b := range w.Today.Backed {
		if b.City == city {
			amount += b.Amount
			ticket = b.Ticket
		}
	}
	return ticket, amount
}

// Campaigning is the city's campaign with today's money folded in, as
// the law sim will see it tonight: what the LAW panel and the fund
// dialog show. Hedged is set if today's money went the other way.
func (w *World) Campaigning(city string) Campaign {
	c := w.Cities[city]
	if c == nil {
		return Campaign{}
	}
	camp := c.Campaign
	for _, b := range w.Today.Backed {
		if b.City != city {
			continue
		}
		if camp.Ticket != "" && camp.Ticket != b.Ticket {
			camp.Hedged = true
		}
		if camp.Ticket == "" {
			camp.Ticket = b.Ticket
		}
		camp.Cash += b.Amount
	}
	return camp
}

// FundedToday is what the player has given a city today, in clean cash.
func (w *World) FundedToday(city string) int {
	n := 0
	for _, f := range w.Today.Funded {
		if f.City == city {
			n += f.Amount
		}
	}
	return n
}

// MeanPressure is the average pressure across the cities: what an
// election is decided on.
func (w *World) MeanPressure() float64 {
	if len(w.CityOrder) == 0 {
		return 0
	}
	sum := 0.0
	for _, cid := range w.CityOrder {
		sum += w.Cities[cid].Pressure
	}
	return sum / float64(len(w.CityOrder))
}
