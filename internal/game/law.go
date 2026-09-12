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
)

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
